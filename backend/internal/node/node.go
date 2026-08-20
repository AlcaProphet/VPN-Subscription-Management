package node

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

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

// 业务错误。
var (
	ErrNotFound   = errors.New("节点不存在")
	ErrConflict   = errors.New("名称冲突")
	ErrBadRequest = errors.New("参数错误")
	ErrForbidden  = errors.New("操作不允许")
)

// Node 统一节点行。
type Node struct {
	ID           int64          `json:"id"`
	Source       string         `json:"source"`
	Name         string         `json:"name"`
	DisplayName  *string        `json:"display_name,omitempty"`
	InstanceID   *int64         `json:"instance_id,omitempty"`
	Tag          string         `json:"tag,omitempty"`
	Protocol     string         `json:"protocol"`
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	ProtocolJSON map[string]any `json:"protocol_json"`
	IsPublic     bool           `json:"is_public"`
	Enabled      bool           `json:"enabled"`
	Allocatable  bool           `json:"allocatable"`
	Missing      bool           `json:"missing"`
	RenderName   string         `json:"render_name"`
	InstanceSlug string         `json:"instance_slug,omitempty"`
}

// CreateManualInput 手工节点创建入参。
type CreateManualInput struct {
	Name         string         `json:"name"`
	Protocol     string         `json:"protocol"`
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	ProtocolJSON map[string]any `json:"protocol_json"`
}

// UpdateManualInput 手工节点编辑入参（name 只读，协议允许变更）。
type UpdateManualInput struct {
	Name         string         `json:"name"`
	Protocol     string         `json:"protocol"`
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	ProtocolJSON map[string]any `json:"protocol_json"`
}

// XrayChangedFunc 节点启停/公共标记变更后的副作用钩子（Build6 注入 Xray 推送）。
type XrayChangedFunc func(ctx context.Context, node Node, oldEnabled, oldPublic bool)

// Service 节点服务。
type Service struct {
	store         *store.Store
	cfg           *config.Service
	log           *slog.Logger
	onXrayChanged XrayChangedFunc
}

// NewService 构造节点服务。
func NewService(st *store.Store, cfg *config.Service, lg *slog.Logger) *Service {
	return &Service{store: st, cfg: cfg, log: lg}
}

// SetOnXrayChanged 注入 Xray 副作用钩子（本 Build 可为 nil，Build6 由装配侧注入）。
func (s *Service) SetOnXrayChanged(fn XrayChangedFunc) {
	s.onXrayChanged = fn
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

// CreateManual 创建手工节点：名称/协议/凭据加密/跨命名空间校验。
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
	encrypted, err := s.encryptProtocolJSON(ctx, in.ProtocolJSON, proto.SensitiveFields)
	if err != nil {
		return nil, err
	}
	var created *Node
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
			`INSERT INTO nodes (source, name, display_name, instance_id, tag, protocol, host, port, protocol_json, is_public, enabled, allocatable, missing)
			 VALUES ('manual', ?, NULL, NULL, '', ?, ?, ?, ?, 0, 1, 1, 0)`,
			in.Name, in.Protocol, in.Host, in.Port, string(raw))
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
		created = &Node{
			ID: id, Source: "manual", Name: in.Name, Protocol: in.Protocol,
			Host: in.Host, Port: in.Port, ProtocolJSON: encrypted,
			Enabled: true, Allocatable: true, RenderName: in.Name,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("manual 节点已创建", "id", created.ID, "name", created.Name)
	s.redactSensitive(created)
	return created, nil
}

// UpdateManual 编辑手工节点：名称只读；协议可变更；敏感字段留空保留原凭据。
func (s *Service) UpdateManual(ctx context.Context, id int64, in UpdateManualInput) (*Node, error) {
	existing, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Source != "manual" {
		return nil, fmt.Errorf("%w: 节点信息由实例检测维护", ErrForbidden)
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
	// 敏感字段：空值保留原密文（可能协议变更后同名字段仍保留）
	merged, err := s.mergeSensitive(ctx, existing, proto, in.ProtocolJSON)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("序列化节点参数失败: %w", err)
	}
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 名称不变时无需重复跨命名空间校验；但协议/参数更新仍可能影响渲染名？不涉及。
		if _, err := tx.ExecContext(ctx,
			`UPDATE nodes SET protocol = ?, host = ?, port = ?, protocol_json = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			in.Protocol, in.Host, in.Port, string(raw), id); err != nil {
			return fmt.Errorf("更新节点失败: %w", err)
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
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM nodes WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除节点失败: %w", err)
		}
		return nil
	})
}

// List 节点列表，支持 ?source=manual|xray。
func (s *Service) List(ctx context.Context, source string) ([]Node, error) {
	query := `SELECT n.id, n.source, n.name, n.display_name, n.instance_id, n.tag, n.protocol, n.host, n.port,
		n.protocol_json, n.is_public, n.enabled, n.allocatable, n.missing, COALESCE(i.slug,'')
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
			n.protocol_json, n.is_public, n.enabled, n.allocatable, n.missing, COALESCE(i.slug,'')
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
	var instanceSlug sql.NullString
	if err := row.Scan(&n.ID, &n.Source, &n.Name, &display, &instance, &tag, &n.Protocol, &n.Host, &n.Port,
		&protocolRaw, &isPublic, &enabled, &allocatable, &missing, &instanceSlug); err != nil {
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
	return n, nil
}

// redactSensitive 列表/详情返回时敏感字段置空，避免泄露凭据明文。
func (s *Service) redactSensitive(n *Node) {
	for _, field := range SensitiveFieldsOf(n.Protocol) {
		if _, ok := n.ProtocolJSON[field]; ok {
			n.ProtocolJSON[field] = ""
		}
	}
}

// encryptProtocolJSON 将敏感字段明文加密后写入副本。
func (s *Service) encryptProtocolJSON(ctx context.Context, in map[string]any, sensitive []string) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	for _, field := range sensitive {
		v, ok := out[field]
		if !ok {
			continue
		}
		str, ok := v.(string)
		if !ok || str == "" {
			continue
		}
		enc, err := s.encryptSecret(ctx, str)
		if err != nil {
			return nil, err
		}
		out[field] = enc
	}
	return out, nil
}

// mergeSensitive 编辑时敏感字段留空保留原密文；非空替换为新密文。
// 协议变更时只保留新协议 schema 内的字段，旧协议不兼容的敏感字段一律丢弃。
func (s *Service) mergeSensitive(ctx context.Context, existing Node, proto Protocol, in map[string]any) (map[string]any, error) {
	// 仅保留新协议表单中声明的字段，避免旧协议残留字段进入渲染。
	out := make(map[string]any, len(proto.FormSchema))
	for _, f := range proto.FormSchema {
		if v, ok := in[f.Name]; ok {
			out[f.Name] = v
		}
	}
	newSensitive := map[string]bool{}
	for _, f := range proto.SensitiveFields {
		newSensitive[f] = true
	}
	// 旧协议中与新协议同名的敏感字段，在输入缺失或为空时沿用旧密文。
	existingSensitive := SensitiveFieldsOf(existing.Protocol)
	for _, field := range existingSensitive {
		if !newSensitive[field] {
			continue
		}
		if v, ok := existing.ProtocolJSON[field]; ok {
			if _, exists := out[field]; !exists {
				out[field] = v
			} else if s, ok := out[field].(string); ok && s == "" {
				out[field] = v
			}
		}
	}
	// 再加密新协议敏感字段（非空替换）
	return s.encryptProtocolJSON(ctx, out, proto.SensitiveFields)
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
		if !f.Required {
			continue
		}
		v, ok := m[f.Name]
		if !ok || v == nil || v == "" {
			if allowEmptySensitive && sensitive[f.Name] {
				continue
			}
			return fmt.Errorf("字段 %s 必填", f.Name)
		}
	}
	return nil
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
