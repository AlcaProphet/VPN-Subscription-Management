package node

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
)

// 强制组名与 Clash/mihomo 内建保留代理名（Design2 §3.3，逐字一致）。
const (
	ForceDirect        = "🚀直接连接"
	ForceOverseas      = "🌎国外流量"
	ForceFallback      = "🛟无法归属的流量"
	ReservedDirect     = "DIRECT"
	ReservedReject     = "REJECT"
	ReservedRejectDrop = "REJECT-DROP"
	ReservedPass       = "PASS"
	ReservedCompatible = "COMPATIBLE"
)

// encPrefix 节点凭据密文前缀。
const encPrefix = "enc:v1:"

// extEncPrefix 节点未知扩展负载密文前缀。
const extEncPrefix = "enc:ext:v1:"

const currentStateFormatVersion = 1

// 业务错误。
var (
	ErrNotFound         = errors.New("节点不存在")
	ErrConflict         = errors.New("名称冲突")
	ErrBadRequest       = errors.New("参数错误")
	ErrForbidden        = errors.New("操作不允许")
	ErrRevisionConflict = errors.New("节点已被其他编辑更新，请重新加载后重试")
)

// revisionConflictError 携带冲突发生时数据库中的最新修订号。
type revisionConflictError struct {
	current int64
}

func (e *revisionConflictError) Error() string {
	return fmt.Sprintf("%s: current_revision=%d", ErrRevisionConflict, e.current)
}

func (e *revisionConflictError) Unwrap() error { return ErrRevisionConflict }

// CurrentRevisionFromError 从修订冲突错误中提取当前修订号，供 HTTP 层生成 409 响应。
func CurrentRevisionFromError(err error) (int64, bool) {
	var conflict *revisionConflictError
	if !errors.As(err, &conflict) {
		return 0, false
	}
	return conflict.current, true
}

// Node 统一节点行。
type Node struct {
	ID                 int64              `json:"id"`
	Source             string             `json:"source"`
	Name               string             `json:"name"`
	DisplayName        *string            `json:"display_name,omitempty"`
	InstanceID         *int64             `json:"instance_id,omitempty"`
	Tag                string             `json:"tag,omitempty"`
	Protocol           string             `json:"protocol"`
	Host               string             `json:"host"`
	Port               int                `json:"port"`
	ProtocolJSON       map[string]any     `json:"protocol_json"`
	IsPublic           bool               `json:"is_public"`
	Enabled            bool               `json:"enabled"`
	Allocatable        bool               `json:"allocatable"`
	Missing            bool               `json:"missing"`
	RenderName         string             `json:"render_name"`
	InstanceSlug       string             `json:"instance_slug,omitempty"`
	EditRevision       int64              `json:"edit_revision"`
	StateFormatVersion int                `json:"state_format_version"`
	CurrentState       CurrentState       `json:"current_state"`
	Extensions         []ExtensionSummary `json:"extensions"`

	// extensionRecords 仅供节点服务在更新时保留已有密文，禁止序列化到响应。
	extensionRecords []ExtensionRecord `json:"-"`
}

// CurrentState 保存当前激活的传输、安全、插件和功能选择。
type CurrentState struct {
	Network  string   `json:"network,omitempty"`
	Security string   `json:"security,omitempty"`
	Plugin   *string  `json:"plugin"`
	Features []string `json:"features,omitempty"`
}

// CredentialOp 表示一次明确的凭据保留或清除操作。
type CredentialOp struct {
	Path string `json:"path"`
	Op   string `json:"op"` // keep|clear
}

// ExtensionInput 是创建节点时提交的未知扩展明文。
type ExtensionInput struct {
	ID      string   `json:"id,omitempty"`
	Scope   string   `json:"scope"`
	Targets []string `json:"targets"`
	Label   string   `json:"label"`
	Payload string   `json:"payload"`
}

// ExtensionOp 表示更新时对未知扩展执行的操作。
type ExtensionOp struct {
	Op      string   `json:"op"` // keep|replace|clear|add
	ID      string   `json:"id,omitempty"`
	Scope   string   `json:"scope,omitempty"`
	Targets []string `json:"targets,omitempty"`
	Label   string   `json:"label,omitempty"`
	Payload string   `json:"payload,omitempty"`
}

// ExtensionSummary 是返回给前端的扩展脱敏摘要。
type ExtensionSummary struct {
	ID         string   `json:"id"`
	Scope      string   `json:"scope"`
	Targets    []string `json:"targets,omitempty"`
	Label      string   `json:"label,omitempty"`
	Configured bool     `json:"configured"`
}

// ExtensionRecord 是 nodes.extensions_json 中的内部记录，负载始终为密文。
type ExtensionRecord struct {
	ID         string   `json:"id"`
	Scope      string   `json:"scope"`
	Targets    []string `json:"targets,omitempty"`
	Label      string   `json:"label,omitempty"`
	Status     string   `json:"status"`
	PayloadEnc string   `json:"payload_encrypted,omitempty"`
}

type extensionEnvelope struct {
	Entries []ExtensionRecord `json:"entries"`
}

// CreateManualInput 手工节点创建入参。
type CreateManualInput struct {
	Name         string           `json:"name"`
	Protocol     string           `json:"protocol"`
	Host         string           `json:"host"`
	Port         int              `json:"port"`
	ProtocolJSON map[string]any   `json:"protocol_json"`
	CurrentState *CurrentState    `json:"current_state,omitempty"`
	Extensions   []ExtensionInput `json:"extensions,omitempty"`
}

// UpdateManualInput 手工节点编辑入参（name 只读，协议允许变更）。
type UpdateManualInput struct {
	Name          string         `json:"name"`
	Protocol      string         `json:"protocol"`
	Host          string         `json:"host"`
	Port          int            `json:"port"`
	ProtocolJSON  map[string]any `json:"protocol_json"`
	CurrentState  *CurrentState  `json:"current_state,omitempty"`
	BaseRevision  int64          `json:"base_revision"`
	ResetScopes   []string       `json:"reset_scopes,omitempty"`
	CredentialOps []CredentialOp `json:"credential_ops,omitempty"`
	ExtensionOps  []ExtensionOp  `json:"extension_ops,omitempty"`
}

// XrayChangedFunc 节点启停/公共标记变更后的副作用钩子（Build6 注入 Xray 推送）。
type XrayChangedFunc func(ctx context.Context, node Node, oldEnabled, oldPublic bool)

// XrayDeleteTarget 节点删除后清理 Xray 用户所需的最小连接信息。
type XrayDeleteTarget struct {
	Email   string
	Tag     string
	APIAddr string
}

// XrayNodeDeletedFunc 节点删除后的 Xray 清理回调。
type XrayNodeDeletedFunc func(ctx context.Context, targets []XrayDeleteTarget)

// Service 节点服务。
type Service struct {
	store             *store.Store
	cfg               *config.Service
	log               *slog.Logger
	onXrayChanged     XrayChangedFunc
	onXrayNodeDeleted XrayNodeDeletedFunc
}

// NewService 构造节点服务。
func NewService(st *store.Store, cfg *config.Service, lg *slog.Logger) *Service {
	return &Service{store: st, cfg: cfg, log: lg}
}

// SetOnXrayChanged 注入 Xray 副作用钩子（本 Build 可为 nil，Build6 由装配侧注入）。
func (s *Service) SetOnXrayChanged(fn XrayChangedFunc) {
	s.onXrayChanged = fn
}

// SetOnXrayNodeDeleted 注入节点删除后的 Xray 清理回调（Build6 Step1 由装配侧注入）。
func (s *Service) SetOnXrayNodeDeleted(fn XrayNodeDeletedFunc) {
	s.onXrayNodeDeleted = fn
}

// ValidateNodeName 节点名（manual 录入名与 xray 系统名），禁止空格。
// validateName 共享基础字符集校验：禁止控制字符、逗号、首尾空白；
// 是否禁止空格由调用侧参数化（节点名禁空格 / 代理组名允许空格）。
func validateName(name string, allowSpace bool) error {
	if name == "" || utf8.RuneCountInString(name) > 128 {
		return errors.New("名称不能为空且不超过 128 字符")
	}
	if name != strings.TrimSpace(name) {
		return errors.New("名称禁止首尾空白")
	}
	if strings.Contains(name, ",") {
		return errors.New("名称禁止逗号")
	}
	if !allowSpace && strings.Contains(name, " ") {
		return errors.New("名称禁止空格")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errors.New("名称禁止控制字符")
		}
	}
	return nil
}

// ValidateNodeName 节点名（manual 录入名与 xray 系统名），禁止空格。
func ValidateNodeName(name string) error {
	return validateName(name, false)
}

// ValidateProxyGroupName 代理组名，允许空格，其余同节点名。
func ValidateProxyGroupName(name string) error {
	return validateName(name, true)
}

// RenderName 返回有效渲染名：display_name 非空则用之，否则 name。
func RenderName(n Node) string {
	if n.DisplayName != nil && *n.DisplayName != "" {
		return *n.DisplayName
	}
	return n.Name
}

// CheckRenderNameNamespaceTx 跨命名空间校验：节点有效渲染名/代理组名不得与
// 强制组名、Clash/mihomo 内建保留代理名、proxy_groups.name 或节点有效渲染名重复。
// 供代理组与节点创建共用；display_name 更新请使用内部带排除自身的 checkRenderNameTx。
func CheckRenderNameNamespaceTx(ctx context.Context, tx *sql.Tx, name string) error {
	if err := checkReservedNames(name); err != nil {
		return err
	}
	// proxy_groups.name
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM proxy_groups WHERE name = ? LIMIT 1`, name).Scan(&id)
	if err == nil {
		return errors.New("名称不得与代理组名重复")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// 节点有效渲染名
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM nodes WHERE COALESCE(NULLIF(display_name,''), name) = ? LIMIT 1`, name).Scan(&id)
	if err == nil {
		return errors.New("名称不得与节点有效渲染名重复")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// checkReservedNames 检查强制组名与内建保留代理名。
func checkReservedNames(name string) error {
	switch name {
	case ForceDirect, ForceOverseas, ForceFallback,
		ReservedDirect, ReservedReject, ReservedRejectDrop, ReservedPass, ReservedCompatible:
		return errors.New("名称不得与代理组/强制组/内建保留代理名重复")
	}
	return nil
}

// checkRenderNameTx 带排除自身节点 ID 的有效渲染名唯一 + 跨命名空间校验。
func checkRenderNameTx(ctx context.Context, tx *sql.Tx, name string, excludeID int64) error {
	if err := checkReservedNames(name); err != nil {
		return err
	}
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM proxy_groups WHERE name = ? LIMIT 1`, name).Scan(&id)
	if err == nil {
		return errors.New("名称不得与代理组名重复")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM nodes WHERE COALESCE(NULLIF(display_name,''), name) = ? AND id != ? LIMIT 1`, name, excludeID).Scan(&id)
	if err == nil {
		return errors.New("名称不得与节点有效渲染名重复")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// GetProtocols 返回协议注册表列表（接入层直接透传）。
func (s *Service) GetProtocols() []Protocol {
	return ManualProtocols()
}

// CreateManual 创建手工节点：名称/协议/当前状态/凭据与扩展均在同一保存契约内处理。
func (s *Service) CreateManual(ctx context.Context, in CreateManualInput) (*Node, error) {
	if err := ValidateNodeName(in.Name); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := validateHostPort(in.Host, in.Port); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	proto, err := GetProtocol(in.Protocol)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := validateProtocolFields(proto, in.ProtocolJSON, false); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	params := in.ProtocolJSON
	if params == nil {
		params = map[string]any{}
	}
	state, err := resolveCurrentState(proto, in.CurrentState, params)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	encrypted, err := s.encryptProtocolJSON(ctx, params, proto.SensitiveFields)
	if err != nil {
		return nil, err
	}
	records, err := s.prepareExtensionInputs(ctx, state, in.Extensions)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	stateRaw, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("序列化节点当前状态失败: %w", err)
	}
	extensionsRaw, err := json.Marshal(extensionEnvelope{Entries: records})
	if err != nil {
		return nil, fmt.Errorf("序列化节点扩展失败: %w", err)
	}
	var createdID int64
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 节点 name 全局唯一（数据库 UNIQUE 兜底）
		if err := CheckRenderNameNamespaceTx(ctx, tx, in.Name); err != nil {
			return fmt.Errorf("%w: %v", ErrConflict, err)
		}
		raw, err := json.Marshal(encrypted)
		if err != nil {
			return fmt.Errorf("序列化节点参数失败: %w", err)
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO nodes (source, name, display_name, instance_id, tag, protocol, host, port,
			 protocol_json, current_state_json, extensions_json, edit_revision, state_format_version,
			 is_public, enabled, allocatable, missing)
			 VALUES ('manual', ?, NULL, NULL, '', ?, ?, ?, ?, ?, ?, 1, 1, 0, 1, 1, 0)`,
			in.Name, in.Protocol, in.Host, in.Port, string(raw), string(stateRaw), string(extensionsRaw))
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: 节点名称已存在", ErrConflict)
			}
			return fmt.Errorf("创建节点失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		createdID = id
		return nil
	})
	if err != nil {
		return nil, err
	}
	created, err := s.Get(ctx, createdID)
	if err != nil {
		return nil, err
	}
	s.log.Info("manual 节点已创建", "id", created.ID, "name", created.Name)
	return created, nil
}

// UpdateManual 编辑手工节点：按修订号原子更新当前状态、活动参数和扩展。
func (s *Service) UpdateManual(ctx context.Context, id int64, in UpdateManualInput) (*Node, error) {
	existing, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Source != "manual" {
		return nil, fmt.Errorf("%w: 节点信息由实例检测维护", ErrForbidden)
	}
	if in.BaseRevision != existing.EditRevision {
		return nil, &revisionConflictError{current: existing.EditRevision}
	}
	if in.Name != "" && in.Name != existing.Name {
		return nil, fmt.Errorf("%w: 节点名称创建后不可修改", ErrBadRequest)
	}
	if err := validateHostPort(in.Host, in.Port); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	proto, err := GetProtocol(in.Protocol)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := validateProtocolFields(proto, in.ProtocolJSON, true); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	resetScopes, err := normalizeResetScopes(in.ResetScopes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	// 跨协议切换的清空边界由服务端强制补齐，避免旧协议凭据通过同名路径复活。
	if existing.Protocol != in.Protocol && !containsString(resetScopes, "protocol") {
		resetScopes = append(resetScopes, "protocol")
	}
	merged := mergeProtocolJSON(existing.ProtocolJSON, in.ProtocolJSON, proto, resetScopes)
	state, err := resolveCurrentState(proto, in.CurrentState, merged)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	merged, err = s.mergeSensitiveWithOps(ctx, existing, proto, merged, resetScopes, in.CredentialOps)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := validateProtocolFields(proto, merged, false); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	encrypted, err := s.encryptProtocolJSON(ctx, merged, proto.SensitiveFields)
	if err != nil {
		return nil, err
	}
	records, err := s.prepareExtensionOps(ctx, existing.extensionRecords, state, resetScopes, in.ExtensionOps)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	raw, err := json.Marshal(encrypted)
	if err != nil {
		return nil, fmt.Errorf("序列化节点参数失败: %w", err)
	}
	stateRaw, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("序列化节点当前状态失败: %w", err)
	}
	extensionsRaw, err := json.Marshal(extensionEnvelope{Entries: records})
	if err != nil {
		return nil, fmt.Errorf("序列化节点扩展失败: %w", err)
	}
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE nodes
			 SET protocol = ?, host = ?, port = ?, protocol_json = ?, current_state_json = ?,
			     extensions_json = ?, edit_revision = edit_revision + 1,
			     state_format_version = ?, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ? AND source = 'manual' AND edit_revision = ?`,
			in.Protocol, in.Host, in.Port, string(raw), string(stateRaw), string(extensionsRaw),
			currentStateFormatVersion, id, in.BaseRevision)
		if err != nil {
			return fmt.Errorf("更新节点失败: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("读取节点更新结果失败: %w", err)
		}
		if affected == 0 {
			var current int64
			if err := tx.QueryRowContext(ctx, `SELECT edit_revision FROM nodes WHERE id = ?`, id).Scan(&current); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return ErrNotFound
				}
				return err
			}
			return &revisionConflictError{current: current}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	n, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	s.redactSensitive(&n)
	return &n, nil
}

// SetDisplayName 仅 xray 来源可设置显示名；空串清空回退系统名。
func (s *Service) SetDisplayName(ctx context.Context, id int64, displayName string) (*Node, error) {
	existing, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Source != "xray" {
		return nil, fmt.Errorf("%w: 仅 Xray 节点可设置显示名", ErrForbidden)
	}
	var newDisplay *string
	if displayName != "" {
		if err := ValidateNodeName(displayName); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		newDisplay = &displayName
	}
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if newDisplay != nil {
			if err := checkRenderNameTx(ctx, tx, *newDisplay, id); err != nil {
				return fmt.Errorf("%w: %v", ErrConflict, err)
			}
		}
		var display any
		if newDisplay != nil {
			display = *newDisplay
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE nodes SET display_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, display, id); err != nil {
			return fmt.Errorf("更新显示名失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	n, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	s.redactSensitive(&n)
	return &n, nil
}

// SetEnabled 切换节点启用状态（manual/xray 均可用）。
func (s *Service) SetEnabled(ctx context.Context, id int64, enabled bool) (*Node, error) {
	existing, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE nodes SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, boolInt(enabled), id); err != nil {
			return fmt.Errorf("更新节点启用状态失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	n, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.onXrayChanged != nil {
		s.onXrayChanged(ctx, n, existing.Enabled, existing.IsPublic)
	}
	s.redactSensitive(&n)
	return &n, nil
}

// SetPublic 切换 xray 节点公共标记：仅 xray 且 allocatable=1 / missing=0 可切换。
func (s *Service) SetPublic(ctx context.Context, id int64, isPublic bool) (*Node, error) {
	existing, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Source != "xray" {
		return nil, fmt.Errorf("%w: 仅 Xray 节点可设置公共标记", ErrForbidden)
	}
	if isPublic && (!existing.Allocatable || existing.Missing) {
		return nil, fmt.Errorf("%w: 仅可分配且未缺失的 Xray 节点可设为公共", ErrBadRequest)
	}
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE nodes SET is_public = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, boolInt(isPublic), id); err != nil {
			return fmt.Errorf("更新公共标记失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	n, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.onXrayChanged != nil {
		s.onXrayChanged(ctx, n, existing.Enabled, existing.IsPublic)
	}
	s.redactSensitive(&n)
	return &n, nil
}

// Delete 删除节点：xray 非 missing 拒绝；manual 可直接删除。
func (s *Service) Delete(ctx context.Context, id int64) error {
	existing, err := s.getRaw(ctx, id)
	if err != nil {
		return err
	}
	if existing.Source == "xray" && !existing.Missing {
		return fmt.Errorf("%w: 请先删除 Xray 入站并刷新节点检测", ErrForbidden)
	}
	var targets []XrayDeleteTarget
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if existing.Source == "xray" {
			rows, err := tx.QueryContext(ctx,
				`SELECT xu.email, xu.inbound_tag, i.api_addr
				 FROM xray_users xu JOIN xray_instances i ON i.id = xu.instance_id
				 WHERE xu.node_id = ?
				 UNION ALL
				 SELECT a.email, xu.inbound_tag, i.api_addr
				 FROM xray_ext_users xu
				 JOIN xray_ext_accounts a ON a.id = xu.ext_account_id
				 JOIN xray_instances i ON i.id = xu.instance_id
				 WHERE xu.node_id = ?`, id, id)
			if err != nil {
				return err
			}
			for rows.Next() {
				var t XrayDeleteTarget
				if err := rows.Scan(&t.Email, &t.Tag, &t.APIAddr); err != nil {
					_ = rows.Close()
					return err
				}
				targets = append(targets, t)
			}
			if err := rows.Close(); err != nil {
				return err
			}
			// Build6-2 补强：额外收集“受影响 active 用户 × 该节点”期望集，
			// 即使 xray_users 尚未落记录也尝试清理。
			rows, err = tx.QueryContext(ctx,
				`SELECT 'user-' || u.id || '@vpn.local', n.tag, i.api_addr
				 FROM users u
				 JOIN group_nodes gn ON gn.group_id = u.group_id
				 JOIN nodes n ON n.id = gn.node_id
				 JOIN xray_instances i ON i.id = n.instance_id
				 WHERE u.status = 'active' AND n.id = ?
				 UNION
				 SELECT 'user-' || u.id || '@vpn.local', n.tag, i.api_addr
				 FROM users u
				 JOIN nodes n ON n.id = ? AND n.is_public = 1
				 JOIN xray_instances i ON i.id = n.instance_id
				 WHERE u.status = 'active'`, id, id)
			if err != nil {
				return err
			}
			for rows.Next() {
				var t XrayDeleteTarget
				if err := rows.Scan(&t.Email, &t.Tag, &t.APIAddr); err != nil {
					_ = rows.Close()
					return err
				}
				targets = append(targets, t)
			}
			if err := rows.Close(); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除节点失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(targets) > 0 && s.onXrayNodeDeleted != nil {
		s.onXrayNodeDeleted(ctx, targets)
	}
	return nil
}

// List 节点列表，支持 ?source=manual|xray。
func (s *Service) List(ctx context.Context, source string) ([]Node, error) {
	query := `SELECT n.id, n.source, n.name, n.display_name, n.instance_id, n.tag, n.protocol, n.host, n.port,
		n.protocol_json, n.edit_revision, n.state_format_version, n.current_state_json, n.extensions_json,
		n.is_public, n.enabled, n.allocatable, n.missing, COALESCE(i.slug,'')
		FROM nodes n LEFT JOIN xray_instances i ON i.id = n.instance_id`
	args := []any{}
	if source == "manual" || source == "xray" {
		query += ` WHERE n.source = ?`
		args = append(args, source)
	}
	query += ` ORDER BY n.id`
	rows, err := s.store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("读取节点列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]Node, 0)
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		n.RenderName = RenderName(n)
		s.redactSensitive(&n)
		out = append(out, n)
	}
	return out, rows.Err()
}

// Get 单个节点（列表脱敏口径）。
func (s *Service) Get(ctx context.Context, id int64) (*Node, error) {
	n, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	n.RenderName = RenderName(n)
	s.redactSensitive(&n)
	return &n, nil
}

// getRaw 读取节点原始数据（含密文，仅供内部使用）。
func (s *Service) getRaw(ctx context.Context, id int64) (Node, error) {
	row := s.store.DB().QueryRowContext(ctx,
		`SELECT n.id, n.source, n.name, n.display_name, n.instance_id, n.tag, n.protocol, n.host, n.port,
			n.protocol_json, n.edit_revision, n.state_format_version, n.current_state_json, n.extensions_json,
			n.is_public, n.enabled, n.allocatable, n.missing, COALESCE(i.slug,'')
			 FROM nodes n LEFT JOIN xray_instances i ON i.id = n.instance_id WHERE n.id = ?`, id)
	n, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Node{}, ErrNotFound
	}
	if err != nil {
		return Node{}, err
	}
	n.RenderName = RenderName(n)
	return n, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNode(row rowScanner) (Node, error) {
	var n Node
	var display sql.NullString
	var instance sql.NullInt64
	var tag sql.NullString
	var isPublic, enabled, allocatable, missing int
	var protocolRaw string
	var currentStateRaw, extensionsRaw string
	var instanceSlug sql.NullString
	if err := row.Scan(&n.ID, &n.Source, &n.Name, &display, &instance, &tag, &n.Protocol, &n.Host, &n.Port,
		&protocolRaw, &n.EditRevision, &n.StateFormatVersion, &currentStateRaw, &extensionsRaw,
		&isPublic, &enabled, &allocatable, &missing, &instanceSlug); err != nil {
		return Node{}, err
	}
	if display.Valid && display.String != "" {
		n.DisplayName = &display.String
	}
	if instance.Valid {
		n.InstanceID = &instance.Int64
	}
	if tag.Valid {
		n.Tag = tag.String
	}
	if instanceSlug.Valid {
		n.InstanceSlug = instanceSlug.String
	}
	n.IsPublic = isPublic == 1
	n.Enabled = enabled == 1
	n.Allocatable = allocatable == 1
	n.Missing = missing == 1
	if err := json.Unmarshal([]byte(protocolRaw), &n.ProtocolJSON); err != nil {
		return Node{}, fmt.Errorf("解析节点参数失败: %w", err)
	}
	if currentStateRaw == "" {
		currentStateRaw = "{}"
	}
	if err := json.Unmarshal([]byte(currentStateRaw), &n.CurrentState); err != nil {
		return Node{}, fmt.Errorf("解析节点当前状态失败: %w", err)
	}
	records, err := decodeExtensionRecords(extensionsRaw)
	if err != nil {
		return Node{}, err
	}
	n.extensionRecords = records
	n.Extensions = extensionSummaries(records)
	return n, nil
}

// redactSensitive 列表/详情返回时敏感字段置空，避免泄露凭据明文。
func (s *Service) redactSensitive(n *Node) {
	for _, path := range SensitiveFieldsOf(n.Protocol) {
		if _, ok := GetPath(n.ProtocolJSON, path); ok {
			SetPath(n.ProtocolJSON, path, "")
		}
	}
	redactExtensions(n)
}

// redactExtensions 只保留未知扩展摘要，禁止响应中出现扩展密文。
func redactExtensions(n *Node) {
	n.Extensions = extensionSummaries(n.extensionRecords)
}

// GetPath 从嵌套 JSON 映射读取点路径。
func GetPath(m map[string]any, path string) (any, bool) {
	var current any = m
	parts := strings.Split(path, ".")
	for _, part := range parts {
		next, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = next[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// SetPath 向嵌套 JSON 映射写入点路径，不存在的中间映射会自动创建。
func SetPath(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := m
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func cloneJSONValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = cloneJSONValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneJSONValue(item)
		}
		return out
	default:
		return value
	}
}

func cloneJSONMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	cloned, _ := cloneJSONValue(value).(map[string]any)
	return cloned
}

// mergeJSONValues 合并对象字段；显式提交空对象表示清空该对象，数组和标量直接替换。
func mergeJSONValues(oldValue, newValue any) any {
	newObject, newIsObject := newValue.(map[string]any)
	if !newIsObject {
		return cloneJSONValue(newValue)
	}
	if len(newObject) == 0 {
		return cloneJSONValue(newObject)
	}
	oldObject, oldIsObject := oldValue.(map[string]any)
	if !oldIsObject {
		return cloneJSONValue(newObject)
	}
	out := cloneJSONMap(oldObject)
	for key, value := range newObject {
		if old, ok := out[key]; ok {
			out[key] = mergeJSONValues(old, value)
		} else {
			out[key] = cloneJSONValue(value)
		}
	}
	return out
}

// mergeProtocolJSON 从旧节点构造更新基底，先清除 reset_scopes，再合并新协议声明的字段。
func mergeProtocolJSON(existing, incoming map[string]any, proto Protocol, resetScopes []string) map[string]any {
	base := cloneJSONMap(existing)
	for _, scope := range resetScopes {
		base = clearScope(base, scope, proto.FormSchema)
	}

	allowed := make(map[string]bool, len(proto.FormSchema))
	out := make(map[string]any, len(proto.FormSchema))
	for _, field := range proto.FormSchema {
		allowed[field.Name] = true
		if value, ok := base[field.Name]; ok {
			out[field.Name] = cloneJSONValue(value)
		}
	}
	for key, value := range incoming {
		if !allowed[key] {
			continue
		}
		if old, ok := out[key]; ok {
			out[key] = mergeJSONValues(old, value)
		} else {
			out[key] = cloneJSONValue(value)
		}
	}
	return out
}

// normalizeResetScopes 校验并去重本次编辑的清空作用域。
func normalizeResetScopes(scopes []string) ([]string, error) {
	out := make([]string, 0, len(scopes))
	seen := make(map[string]bool, len(scopes))
	for _, raw := range scopes {
		scope := strings.TrimSpace(raw)
		if scope == "" {
			return nil, errors.New("reset scope 不能为空")
		}
		valid := scope == "protocol" || scope == "network" || scope == "security" || scope == "plugin"
		if strings.HasPrefix(scope, "feature.") && len(strings.TrimPrefix(scope, "feature.")) > 0 {
			valid = true
		}
		if !valid {
			return nil, fmt.Errorf("不支持的 reset scope: %s", scope)
		}
		if !seen[scope] {
			seen[scope] = true
			out = append(out, scope)
		}
	}
	return out, nil
}

// clearScope 清除指定作用域的协议参数。Build18 增加 ResetOn 元数据后，
// 可在本函数的字段判断中直接替换启发式映射；当前实现保持现有注册表兼容。
func clearScope(m map[string]any, scope string, schema []FieldSchema) map[string]any {
	out := cloneJSONMap(m)
	if scope == "protocol" {
		return map[string]any{}
	}
	for _, field := range schema {
		if scopeResetsField(scope, field.Name) {
			delete(out, field.Name)
		}
	}
	// 这些兼容别名可能不在当前协议 schema 中，也必须随所属分支清除。
	for _, field := range scopeResetFields(scope) {
		delete(out, field)
	}
	return out
}

func scopeResetsField(scope, field string) bool {
	if scope == "network" {
		switch field {
		case "ws-opts", "grpc-opts", "h2-opts", "http-opts", "xhttp-opts", "ws-path", "ws-headers", "packet-encoding", "flow":
			return true
		}
	}
	if scope == "security" {
		switch field {
		case "tls", "security", "sni", "servername", "alpn", "fingerprint", "client-fingerprint", "skip-cert-verify", "certificate", "ca", "ca-str", "reality-opts", "ech-opts":
			return true
		}
	}
	if scope == "plugin" {
		return field == "plugin" || field == "plugin-opts"
	}
	if strings.HasPrefix(scope, "feature.") {
		feature := strings.TrimPrefix(scope, "feature.")
		switch feature {
		case "smux":
			return field == "smux"
		case "udp-over-tcp":
			return field == "udp-over-tcp" || field == "udp-over-tcp-version"
		case "udp-over-stream":
			return field == "udp-over-stream" || field == "udp-over-stream-version"
		case "multiplexing":
			return field == "multiplexing"
		default:
			return field == feature
		}
	}
	return false
}

func scopeResetFields(scope string) []string {
	switch scope {
	case "network":
		return []string{"ws-opts", "grpc-opts", "h2-opts", "http-opts", "xhttp-opts", "ws-path", "ws-headers", "packet-encoding", "flow"}
	case "security":
		return []string{"tls", "security", "sni", "servername", "alpn", "fingerprint", "client-fingerprint", "skip-cert-verify", "certificate", "ca", "ca-str", "reality-opts", "ech-opts"}
	case "plugin":
		return []string{"plugin", "plugin-opts"}
	case "feature.smux":
		return []string{"smux"}
	case "feature.udp-over-tcp":
		return []string{"udp-over-tcp", "udp-over-tcp-version"}
	case "feature.udp-over-stream":
		return []string{"udp-over-stream", "udp-over-stream-version"}
	case "feature.multiplexing":
		return []string{"multiplexing"}
	default:
		if strings.HasPrefix(scope, "feature.") {
			return []string{strings.TrimPrefix(scope, "feature.")}
		}
		return nil
	}
}

func pathInResetScope(path, scope string) bool {
	if scope == "protocol" {
		return true
	}
	top := strings.SplitN(path, ".", 2)[0]
	if scope == "network" || scope == "security" || scope == "plugin" {
		return scopeResetsField(scope, top)
	}
	if strings.HasPrefix(scope, "feature.") {
		return scopeResetsField(scope, top)
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// DeriveCurrentState 从当前协议参数派生最小当前选择。
func DeriveCurrentState(proto Protocol, params map[string]any) CurrentState {
	state := CurrentState{}
	if value, ok := params["network"].(string); ok && value != "" {
		state.Network = value
	} else if value, ok := params["transport"].(string); ok && value != "" {
		state.Network = value
	} else if hasSchemaField(proto.FormSchema, "network") {
		// 注册表中的 network 默认值为 tcp，属于当前默认组合而非待恢复分支。
		state.Network = "tcp"
	}

	if value, ok := params["security"].(string); ok && value != "" {
		state.Security = value
	} else if tls, ok := params["tls"].(bool); ok && tls {
		if configuredObject(params["reality-opts"]) {
			state.Security = "reality"
		} else {
			state.Security = "tls"
		}
	} else if proto.Protocol == "trojan" {
		state.Security = "tls"
	} else {
		state.Security = "none"
	}

	if value, ok := params["plugin"].(string); ok && value != "" {
		plugin := value
		state.Plugin = &plugin
	}
	if value, ok := params["udp-over-tcp"].(bool); ok && value {
		state.Features = append(state.Features, "udp-over-tcp")
	}
	if value, ok := params["udp-over-stream"].(bool); ok && value {
		state.Features = append(state.Features, "udp-over-stream")
	}
	if value, ok := params["xudp"].(bool); ok && value {
		state.Features = append(state.Features, "xudp")
	}
	if value, ok := params["smux"].(map[string]any); ok {
		if enabled, _ := value["enabled"].(bool); enabled {
			state.Features = append(state.Features, "smux")
		}
	}
	if value, ok := params["multiplexing"].(string); ok && value != "" && value != "MULTIPLEXING_OFF" {
		state.Features = append(state.Features, "multiplexing")
	}
	state.Features = uniqueSortedStrings(state.Features)
	return state
}

func hasSchemaField(schema []FieldSchema, name string) bool {
	for _, field := range schema {
		if field.Name == name {
			return true
		}
	}
	return false
}

func configuredObject(value any) bool {
	object, ok := value.(map[string]any)
	return ok && len(object) > 0
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sameStringSet(left, right []string) bool {
	left = uniqueSortedStrings(left)
	right = uniqueSortedStrings(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// resolveCurrentState 在保存前校验请求状态与 protocol_json 的基本一致性，并补齐合法缺省。
func resolveCurrentState(proto Protocol, requested *CurrentState, params map[string]any) (CurrentState, error) {
	derived := DeriveCurrentState(proto, params)
	if requested == nil {
		return derived, nil
	}
	state := *requested
	if state.Network == "" {
		state.Network = derived.Network
	} else if state.Network != derived.Network {
		return CurrentState{}, fmt.Errorf("current_state.network 与 protocol_json.network 不一致")
	}
	if state.Security == "" {
		state.Security = derived.Security
	} else if state.Security != derived.Security {
		return CurrentState{}, fmt.Errorf("current_state.security 与 protocol_json 安全参数不一致")
	}
	if state.Plugin == nil {
		if derived.Plugin != nil {
			return CurrentState{}, errors.New("current_state.plugin 与 protocol_json.plugin 不一致")
		}
	} else if derived.Plugin == nil || *state.Plugin != *derived.Plugin {
		return CurrentState{}, errors.New("current_state.plugin 与 protocol_json.plugin 不一致")
	}
	if len(state.Features) == 0 {
		if len(derived.Features) > 0 {
			return CurrentState{}, errors.New("current_state.features 与 protocol_json 功能开关不一致")
		}
		state.Features = nil
	} else if !sameStringSet(state.Features, derived.Features) {
		return CurrentState{}, errors.New("current_state.features 与 protocol_json 功能开关不一致")
	} else {
		state.Features = uniqueSortedStrings(state.Features)
	}
	return state, nil
}

// mergeSensitive 编辑时执行凭据默认保留，保留旧函数名供同包旧调用方兼容。
func (s *Service) mergeSensitive(ctx context.Context, existing Node, proto Protocol, in map[string]any) (map[string]any, error) {
	merged := mergeProtocolJSON(existing.ProtocolJSON, in, proto, nil)
	return s.mergeSensitiveWithOps(ctx, existing, proto, merged, nil, nil)
}

// mergeSensitiveWithOps 在重置基底上应用 keep/clear，并对未声明操作的空值保留旧密文。
func (s *Service) mergeSensitiveWithOps(_ context.Context, existing Node, proto Protocol, base map[string]any, resetScopes []string, ops []CredentialOp) (map[string]any, error) {
	opByPath := make(map[string]string, len(ops))
	sensitive := make(map[string]bool, len(proto.SensitiveFields))
	for _, path := range proto.SensitiveFields {
		sensitive[path] = true
	}
	for _, op := range ops {
		if !sensitive[op.Path] {
			return nil, fmt.Errorf("凭据路径 %s 不属于当前协议", op.Path)
		}
		if op.Op != "keep" && op.Op != "clear" {
			return nil, fmt.Errorf("不支持的凭据操作: %s", op.Op)
		}
		if _, exists := opByPath[op.Path]; exists {
			return nil, fmt.Errorf("凭据路径 %s 重复操作", op.Path)
		}
		opByPath[op.Path] = op.Op
	}

	out := cloneJSONMap(base)
	for _, path := range proto.SensitiveFields {
		oldValue, oldExists := GetPath(existing.ProtocolJSON, path)
		reset := false
		for _, scope := range resetScopes {
			if pathInResetScope(path, scope) {
				reset = true
				break
			}
		}
		switch opByPath[path] {
		case "keep":
			if reset {
				deletePath(out, path)
			} else if oldExists {
				SetPath(out, path, cloneJSONValue(oldValue))
			}
		case "clear":
			deletePath(out, path)
		default:
			if !reset && oldExists {
				incoming, incomingExists := GetPath(out, path)
				if !incomingExists || emptyJSONString(incoming) {
					SetPath(out, path, cloneJSONValue(oldValue))
				}
			}
		}
	}
	return out, nil
}

func emptyJSONString(value any) bool {
	text, ok := value.(string)
	return ok && text == ""
}

func deletePath(m map[string]any, path string) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return
	}
	deletePathParts(m, parts)
}

func deletePathParts(m map[string]any, parts []string) bool {
	if len(parts) == 1 {
		delete(m, parts[0])
		return len(m) == 0
	}
	next, ok := m[parts[0]].(map[string]any)
	if !ok {
		return false
	}
	if deletePathParts(next, parts[1:]) {
		delete(m, parts[0])
	}
	return len(m) == 0
}

// encryptProtocolJSON 将敏感字段明文加密后写入副本。
func (s *Service) encryptProtocolJSON(ctx context.Context, in map[string]any, sensitive []string) (map[string]any, error) {
	out := cloneJSONMap(in)
	for _, path := range sensitive {
		v, ok := GetPath(out, path)
		if !ok {
			continue
		}
		str, ok := v.(string)
		if !ok || str == "" {
			continue
		}
		if strings.HasPrefix(str, encPrefix) {
			continue
		}
		enc, err := s.encryptSecret(ctx, str)
		if err != nil {
			return nil, err
		}
		SetPath(out, path, enc)
	}
	return out, nil
}

// encryptSecret 使用签名密钥派生 AES-256-GCM 加密。
func (s *Service) encryptSecret(ctx context.Context, plain string) (string, error) {
	key, err := s.cfg.Get(ctx, config.KeySigningKey)
	if err != nil || key == "" {
		return "", errors.New("签名密钥未配置，无法加密节点凭据")
	}
	b, err := config.Encrypt([]byte(plain), []byte(key))
	if err != nil {
		return "", err
	}
	return encPrefix + b, nil
}

// decryptSecret 解密节点凭据。
func (s *Service) decryptSecret(ctx context.Context, v string) (string, error) {
	if !strings.HasPrefix(v, encPrefix) {
		return "", errors.New("非法密文格式")
	}
	key, err := s.cfg.Get(ctx, config.KeySigningKey)
	if err != nil || key == "" {
		return "", errors.New("签名密钥未配置")
	}
	b, err := config.Decrypt(strings.TrimPrefix(v, encPrefix), []byte(key))
	return string(b), err
}

func decodeExtensionRecords(raw string) ([]ExtensionRecord, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" || strings.TrimSpace(raw) == "null" {
		return nil, nil
	}
	var envelope extensionEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, fmt.Errorf("解析节点扩展失败: %w", err)
	}
	seen := make(map[string]bool, len(envelope.Entries))
	records := make([]ExtensionRecord, 0, len(envelope.Entries))
	for i, record := range envelope.Entries {
		if strings.TrimSpace(record.ID) == "" {
			return nil, fmt.Errorf("节点扩展第 %d 项缺少 id", i+1)
		}
		if seen[record.ID] {
			return nil, fmt.Errorf("节点扩展 id 重复: %s", record.ID)
		}
		seen[record.ID] = true
		if err := validateExtensionScope(record.Scope); err != nil {
			return nil, fmt.Errorf("节点扩展 %s: %v", record.ID, err)
		}
		if record.Status == "" {
			record.Status = "encrypted"
		}
		if record.Status != "encrypted" {
			return nil, fmt.Errorf("节点扩展 %s 状态非法: %s", record.ID, record.Status)
		}
		if !strings.HasPrefix(record.PayloadEnc, extEncPrefix) {
			return nil, fmt.Errorf("节点扩展 %s 负载未加密", record.ID)
		}
		record.Targets = normalizeTargets(record.Targets)
		records = append(records, record)
	}
	return records, nil
}

func extensionSummaries(records []ExtensionRecord) []ExtensionSummary {
	out := make([]ExtensionSummary, 0, len(records))
	for _, record := range records {
		out = append(out, ExtensionSummary{
			ID:         record.ID,
			Scope:      record.Scope,
			Targets:    append([]string(nil), record.Targets...),
			Label:      record.Label,
			Configured: record.Status == "encrypted" && strings.HasPrefix(record.PayloadEnc, extEncPrefix),
		})
	}
	return out
}

func cloneExtensionRecords(records []ExtensionRecord) []ExtensionRecord {
	out := make([]ExtensionRecord, 0, len(records))
	for _, record := range records {
		record.Targets = append([]string(nil), record.Targets...)
		out = append(out, record)
	}
	return out
}

func validateExtensionScope(scope string) error {
	scope = strings.TrimSpace(scope)
	if scope == "node" {
		return nil
	}
	parts := strings.SplitN(scope, ".", 2)
	if len(parts) != 2 || parts[1] == "" {
		return errors.New("scope 必须为 node、transport.<network>、security.<security>、plugin.<plugin> 或 feature.<feature>")
	}
	switch parts[0] {
	case "transport", "security", "plugin", "feature":
		return nil
	default:
		return fmt.Errorf("不支持的 scope: %s", scope)
	}
}

func extensionScopeActive(scope string, state CurrentState) bool {
	if scope == "node" {
		return true
	}
	parts := strings.SplitN(scope, ".", 2)
	if len(parts) != 2 {
		return false
	}
	switch parts[0] {
	case "transport":
		return state.Network != "" && state.Network == parts[1]
	case "security":
		return state.Security != "" && state.Security == parts[1]
	case "plugin":
		return state.Plugin != nil && *state.Plugin == parts[1]
	case "feature":
		return containsString(state.Features, parts[1])
	default:
		return false
	}
}

func extensionScopeReset(scope string, resetScopes []string) bool {
	for _, reset := range resetScopes {
		switch reset {
		case "protocol":
			return true
		case "network":
			if strings.HasPrefix(scope, "transport.") {
				return true
			}
		case "security":
			if strings.HasPrefix(scope, "security.") {
				return true
			}
		case "plugin":
			if strings.HasPrefix(scope, "plugin.") {
				return true
			}
		default:
			if strings.HasPrefix(reset, "feature.") && scope == reset {
				return true
			}
		}
	}
	return false
}

func normalizeTargets(targets []string) []string {
	seen := make(map[string]bool, len(targets))
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

func extensionID(id string) string {
	if id != "" {
		return id
	}
	return "ext-" + uuid.NewString()
}

func (s *Service) encryptExtensionPayload(ctx context.Context, plain string) (string, error) {
	key, err := s.cfg.Get(ctx, config.KeySigningKey)
	if err != nil || key == "" {
		return "", errors.New("签名密钥未配置，无法加密节点扩展")
	}
	b, err := config.Encrypt([]byte(plain), []byte(key))
	if err != nil {
		return "", err
	}
	return extEncPrefix + b, nil
}

func (s *Service) prepareExtensionInputs(ctx context.Context, state CurrentState, inputs []ExtensionInput) ([]ExtensionRecord, error) {
	records := make([]ExtensionRecord, 0, len(inputs))
	seen := make(map[string]bool, len(inputs))
	for _, input := range inputs {
		scope := strings.TrimSpace(input.Scope)
		if err := validateExtensionScope(scope); err != nil {
			return nil, err
		}
		if !extensionScopeActive(scope, state) {
			return nil, fmt.Errorf("扩展 scope %s 不属于当前分支", scope)
		}
		if input.Payload == "" {
			return nil, errors.New("扩展负载不能为空")
		}
		id := extensionID(strings.TrimSpace(input.ID))
		if seen[id] {
			return nil, fmt.Errorf("节点扩展 id 重复: %s", id)
		}
		seen[id] = true
		payload, err := s.encryptExtensionPayload(ctx, input.Payload)
		if err != nil {
			return nil, err
		}
		records = append(records, ExtensionRecord{
			ID:         id,
			Scope:      scope,
			Targets:    normalizeTargets(input.Targets),
			Label:      input.Label,
			Status:     "encrypted",
			PayloadEnc: payload,
		})
	}
	return records, nil
}

func (s *Service) prepareExtensionOps(ctx context.Context, existing []ExtensionRecord, state CurrentState, resetScopes []string, ops []ExtensionOp) ([]ExtensionRecord, error) {
	records := make([]ExtensionRecord, 0, len(existing)+len(ops))
	byID := make(map[string]int, len(existing)+len(ops))
	for _, record := range existing {
		if extensionScopeReset(record.Scope, resetScopes) || !extensionScopeActive(record.Scope, state) {
			continue
		}
		record.Targets = normalizeTargets(record.Targets)
		byID[record.ID] = len(records)
		records = append(records, record)
	}
	for _, op := range ops {
		switch op.Op {
		case "keep":
			if op.ID == "" {
				return nil, errors.New("keep 扩展操作缺少 id")
			}
			// 不存在、已重置或不属于当前分支的扩展均不得被 keep 复活。
			continue
		case "clear":
			if op.ID == "" {
				return nil, errors.New("clear 扩展操作缺少 id")
			}
			if index, ok := byID[op.ID]; ok {
				records = append(records[:index], records[index+1:]...)
				byID = extensionIndex(records)
			}
		case "replace":
			if op.ID == "" {
				return nil, errors.New("replace 扩展操作缺少 id")
			}
			index, ok := byID[op.ID]
			if !ok {
				return nil, fmt.Errorf("待替换扩展不存在: %s", op.ID)
			}
			if op.Payload == "" {
				return nil, errors.New("replace 扩展负载不能为空")
			}
			record := records[index]
			if op.Scope != "" {
				record.Scope = strings.TrimSpace(op.Scope)
			}
			if err := validateExtensionScope(record.Scope); err != nil {
				return nil, err
			}
			if !extensionScopeActive(record.Scope, state) {
				return nil, fmt.Errorf("扩展 scope %s 不属于当前分支", record.Scope)
			}
			if op.Targets != nil {
				record.Targets = normalizeTargets(op.Targets)
			}
			if op.Label != "" {
				record.Label = op.Label
			}
			payload, err := s.encryptExtensionPayload(ctx, op.Payload)
			if err != nil {
				return nil, err
			}
			record.Status = "encrypted"
			record.PayloadEnc = payload
			records[index] = record
		case "add":
			scope := strings.TrimSpace(op.Scope)
			if err := validateExtensionScope(scope); err != nil {
				return nil, err
			}
			if !extensionScopeActive(scope, state) {
				return nil, fmt.Errorf("扩展 scope %s 不属于当前分支", scope)
			}
			if op.Payload == "" {
				return nil, errors.New("add 扩展负载不能为空")
			}
			id := extensionID(strings.TrimSpace(op.ID))
			if _, exists := byID[id]; exists {
				return nil, fmt.Errorf("节点扩展 id 重复: %s", id)
			}
			payload, err := s.encryptExtensionPayload(ctx, op.Payload)
			if err != nil {
				return nil, err
			}
			byID[id] = len(records)
			records = append(records, ExtensionRecord{
				ID:         id,
				Scope:      scope,
				Targets:    normalizeTargets(op.Targets),
				Label:      op.Label,
				Status:     "encrypted",
				PayloadEnc: payload,
			})
		default:
			return nil, fmt.Errorf("不支持的扩展操作: %s", op.Op)
		}
	}
	return records, nil
}

func extensionIndex(records []ExtensionRecord) map[string]int {
	index := make(map[string]int, len(records))
	for i, record := range records {
		index[record.ID] = i
	}
	return index
}

// validateHostPort 校验 host/port 基本合法性。
func validateHostPort(host string, port int) error {
	if strings.TrimSpace(host) == "" {
		return errors.New("服务器地址不能为空")
	}
	if port < 1 || port > 65535 {
		return errors.New("端口须在 1-65535 之间")
	}
	return nil
}

// validateProtocolFields 按注册表 schema 校验必填字段与基本类型。
// allowEmptySensitive=true 时敏感字段允许空值（编辑留空=保留原凭据）。
func validateProtocolFields(proto Protocol, m map[string]any, allowEmptySensitive bool) error {
	if m == nil {
		m = map[string]any{}
	}
	sensitive := map[string]bool{}
	for _, f := range proto.SensitiveFields {
		sensitive[f] = true
	}
	for _, f := range proto.FormSchema {
		v, ok := m[f.Name]
		if f.Required && (!ok || v == nil || v == "") {
			if !(allowEmptySensitive && sensitive[f.Name]) {
				return fmt.Errorf("字段 %s 必填", f.Name)
			}
			continue
		}
		if !ok || v == nil || v == "" {
			continue
		}
		if err := validateFieldType(f, v); err != nil {
			return err
		}
	}
	return nil
}

func validateFieldType(field FieldSchema, value any) error {
	return validateFieldValue(field, value, field.Name)
}

func validateFieldValue(field FieldSchema, value any, path string) error {
	valid := false
	switch field.Type {
	case "text", "password", "select", "text-list", "int-list":
		_, valid = value.(string)
		if field.Type == "text-list" {
			valid = valid || stringList(value)
		} else if field.Type == "int-list" {
			valid = valid || numberList(value)
		}
	case "number":
		switch value.(type) {
		case int, int32, int64, float32, float64, json.Number:
			valid = true
		}
	case "bool":
		_, valid = value.(bool)
	case "object":
		switch field.ObjectKind {
		case "fields", "map":
			object, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("字段 %s 类型应为 object", path)
			}
			if field.ObjectKind == "fields" {
				if err := validateObjectProperties(field, object, path); err != nil {
					return err
				}
			}
			valid = true
		case "list":
			items, ok := value.([]any)
			if !ok {
				return fmt.Errorf("字段 %s 类型应为 object list", path)
			}
			for i, item := range items {
				object, ok := item.(map[string]any)
				if !ok {
					return fmt.Errorf("字段 %s[%d] 类型应为 object", path, i)
				}
				if err := validateObjectProperties(field, object, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
			valid = true
		default:
			switch value.(type) {
			case map[string]any, []any:
				valid = true
			}
		}
	default:
		return fmt.Errorf("字段 %s 使用未知类型 %s", field.Name, field.Type)
	}
	if !valid {
		return fmt.Errorf("字段 %s 类型应为 %s", path, field.Type)
	}
	return nil
}

func validateObjectProperties(field FieldSchema, object map[string]any, path string) error {
	known := make(map[string]FieldSchema, len(field.Properties))
	for _, property := range field.Properties {
		known[property.Name] = property
		value, ok := object[property.Name]
		if property.Required && (!ok || value == nil || value == "") {
			return fmt.Errorf("字段 %s.%s 必填", path, property.Name)
		}
		if !ok || value == nil || value == "" {
			continue
		}
		if err := validateFieldValue(property, value, path+"."+property.Name); err != nil {
			return err
		}
	}
	if field.AllowUnknown {
		return nil
	}
	for key := range object {
		if _, ok := known[key]; !ok {
			return fmt.Errorf("字段 %s.%s 未在协议注册表中声明", path, key)
		}
	}
	return nil
}

func stringList(value any) bool {
	items, ok := value.([]any)
	if !ok {
		_, ok = value.([]string)
		return ok
	}
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}

func numberList(value any) bool {
	items, ok := value.([]any)
	if !ok {
		_, ok = value.([]int)
		return ok
	}
	for _, item := range items {
		switch item.(type) {
		case int, int32, int64, float32, float64, json.Number:
		default:
			return false
		}
	}
	return true
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isUniqueViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "constraint failed")
}
