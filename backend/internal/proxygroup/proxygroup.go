// Package proxygroup 提供代理组全局定义服务：预设/自建组 CRUD、DAG 与内容约束校验。
package proxygroup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"vpn-sub/internal/node"
	"vpn-sub/internal/store"
)

// ValidGroupTypes 是 mihomo 支持的代理组类型共享元数据。
var ValidGroupTypes = map[string]bool{"select": true, "url-test": true, "fallback": true, "load-balance": true, "relay": true}

// 业务错误。
var (
	ErrNotFound   = errors.New("代理组不存在")
	ErrConflict   = errors.New("代理组名称冲突")
	ErrBadRequest = errors.New("参数错误")
	ErrForbidden  = errors.New("操作不允许")
)

// Definition 代理组定义（仅子组引用；节点引用已改为装配时按组选择/排序）。
type Definition struct {
	GroupType           string   `json:"type"`
	Nodes               []string `json:"-"` // 兼容旧代码/测试结构体，不再序列化，不再参与校验与渲染
	Groups              []string `json:"groups"`
	Use                 []string `json:"use,omitempty"`
	URL                 string   `json:"url,omitempty"`
	ExpectedStatus      string   `json:"expected-status,omitempty"`
	Interval            int      `json:"interval,omitempty"`
	Timeout             int      `json:"timeout,omitempty"`
	MaxFailedTimes      int      `json:"max-failed-times,omitempty"`
	Lazy                bool     `json:"lazy,omitempty"`
	DisableUDP          bool     `json:"disable-udp,omitempty"`
	InterfaceName       string   `json:"interface-name,omitempty"`
	RoutingMark         int      `json:"routing-mark,omitempty"`
	Filter              string   `json:"filter,omitempty"`
	ExcludeFilter       string   `json:"exclude-filter,omitempty"`
	ExcludeType         string   `json:"exclude-type,omitempty"`
	IncludeAll          bool     `json:"include-all,omitempty"`
	IncludeAllProxies   bool     `json:"include-all-proxies,omitempty"`
	IncludeAllProviders bool     `json:"include-all-providers,omitempty"`
	Hidden              bool     `json:"hidden,omitempty"`
	Icon                string   `json:"icon,omitempty"`
}

// Group 代理组行。
type Group struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"` // preset / custom
	PresetKey  string     `json:"preset_key,omitempty"`
	Enabled    bool       `json:"enabled"`
	Definition Definition `json:"definition"`
}

// Service 代理组服务。
type Service struct {
	store *store.Store
	log   *slog.Logger
}

// NewService 构造代理组服务。
func NewService(st *store.Store, lg *slog.Logger) *Service {
	return &Service{store: st, log: lg}
}

// List 代理组列表（含预设组）。
func (s *Service) List(ctx context.Context) ([]Group, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, name, type, COALESCE(preset_key,''), enabled, definition_json FROM proxy_groups ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("读取代理组列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]Group, 0)
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Get 单个代理组。
func (s *Service) Get(ctx context.Context, id int64) (*Group, error) {
	g, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// CreateCustom 创建自建组。
func (s *Service) CreateCustom(ctx context.Context, name, groupType string, def Definition) (*Group, error) {
	if err := node.ValidateProxyGroupName(name); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if !ValidGroupTypes[groupType] {
		return nil, fmt.Errorf("%w: 非法代理组类型", ErrBadRequest)
	}
	def.GroupType = groupType
	if err := s.validateDefinition(ctx, def); err != nil {
		return nil, err
	}
	var created *Group
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := node.CheckRenderNameNamespaceTx(ctx, tx, name); err != nil {
			return fmt.Errorf("%w: %v", ErrConflict, err)
		}
		raw, err := json.Marshal(def)
		if err != nil {
			return fmt.Errorf("序列化代理组定义失败: %w", err)
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO proxy_groups (name, type, preset_key, enabled, definition_json) VALUES (?, 'custom', '', 1, ?)`,
			name, string(raw))
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("%w: 代理组名称已存在", ErrConflict)
			}
			return fmt.Errorf("创建代理组失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &Group{ID: id, Name: name, Type: "custom", Enabled: true, Definition: def}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("自建代理组已创建", "id", created.ID, "name", created.Name)
	return created, nil
}

// Update 更新代理组成员/类型；name/preset_key 不可改。
func (s *Service) Update(ctx context.Context, id int64, groupType string, def Definition) (*Group, error) {
	existing, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ValidGroupTypes[groupType] {
		return nil, fmt.Errorf("%w: 非法代理组类型", ErrBadRequest)
	}
	def.GroupType = groupType
	if err := s.validateDefinitionWithDAG(ctx, existing, def); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(def)
	if err != nil {
		return nil, fmt.Errorf("序列化代理组定义失败: %w", err)
	}
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE proxy_groups SET definition_json = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, string(raw), id); err != nil {
			return fmt.Errorf("更新代理组失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// SetPresetEnabled 切换预设组启用状态。
func (s *Service) SetPresetEnabled(ctx context.Context, id int64, enabled bool) (*Group, error) {
	existing, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Type != "preset" {
		return nil, fmt.Errorf("%w: 仅预设组可切换启用状态", ErrForbidden)
	}
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`UPDATE proxy_groups SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, boolInt(enabled), id); err != nil {
			return fmt.Errorf("更新预设组启用状态失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Delete 删除自建组；预设组不可删。
func (s *Service) Delete(ctx context.Context, id int64) error {
	existing, err := s.getRaw(ctx, id)
	if err != nil {
		return err
	}
	if existing.Type == "preset" {
		return fmt.Errorf("%w: 预设组不可删除", ErrForbidden)
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM proxy_groups WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除代理组失败: %w", err)
		}
		return nil
	})
}

// validateDefinition 校验节点/子组引用与内容约束（不加载全量 DAG）。
func (s *Service) validateDefinition(ctx context.Context, def Definition) error {
	return s.validateDefinitionWithDAG(ctx, Group{}, def)
}

// validateDefinitionWithDAG 校验引用存在、内容约束与全量 DAG。
func (s *Service) validateDefinitionWithDAG(ctx context.Context, existing Group, def Definition) error {
	if !ValidGroupTypes[def.GroupType] {
		return fmt.Errorf("%w: 非法代理组类型", ErrBadRequest)
	}
	// 子组引用：允许 🚀直接连接 / 🌎国外流量 或已存在代理组；🛟无法归属的流量不允许作为子组。
	for _, name := range def.Groups {
		if name == node.ForceDirect || name == node.ForceOverseas {
			continue
		}
		var n int
		if err := s.store.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM proxy_groups WHERE name = ?`, name).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("%w: 子组不存在: %s", ErrBadRequest, name)
		}
		if existing.ID > 0 && name == existing.Name {
			return fmt.Errorf("%w: 代理组不能引用自身", ErrBadRequest)
		}
	}
	if def.GroupType == "select" && len(def.Groups) == 0 && len(def.Use) == 0 &&
		!def.IncludeAll && !def.IncludeAllProxies && !def.IncludeAllProviders {
		return fmt.Errorf("%w: select 组至少需要 groups/use/include-all 之一", ErrBadRequest)
	}
	if def.URL != "" {
		if def.GroupType != "url-test" && def.GroupType != "fallback" && def.GroupType != "load-balance" {
			return fmt.Errorf("%w: 当前组类型不支持健康检查 URL", ErrBadRequest)
		}
		parsed, err := url.Parse(def.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("%w: 健康检查 URL 仅支持 http/https", ErrBadRequest)
		}
	}
	if def.Interval < 0 || def.Timeout < 0 || def.MaxFailedTimes < 0 || def.RoutingMark < 0 {
		return fmt.Errorf("%w: 数值字段不能为负数", ErrBadRequest)
	}
	if err := validateExcludeTypes(def.ExcludeType); err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	// 全量 DAG 校验（含本次变更后的组）
	return s.validateDAG(ctx, existing, def)
}

var excludeTypes = map[string]bool{
	"direct": true, "reject": true, "rejectdrop": true, "compatible": true, "pass": true, "dns": true,
	"shadowsocks": true, "shadowsocksr": true, "snell": true, "socks5": true, "http": true,
	"vmess": true, "vless": true, "trojan": true, "hysteria": true, "hysteria2": true, "wireguard": true,
	"tuic": true, "mieru": true, "masque": true, "anytls": true, "sudoku": true, "relay": true,
	"selector": true, "fallback": true, "urltest": true, "loadbalance": true, "ssh": true,
}

func validateExcludeTypes(value string) error {
	for _, item := range strings.Split(value, "|") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !excludeTypes[strings.ToLower(item)] {
			return fmt.Errorf("未知 exclude-type: %s", item)
		}
	}
	return nil
}

// validateDAG 加载全部代理组，替换当前编辑组后做三色 DFS。
func (s *Service) validateDAG(ctx context.Context, existing Group, def Definition) error {
	all, err := s.List(ctx)
	if err != nil {
		return err
	}
	groups := make([]Group, 0, len(all)+1)
	replaced := false
	for _, g := range all {
		if existing.ID > 0 && g.ID == existing.ID {
			g.Definition = def
			replaced = true
		}
		groups = append(groups, g)
	}
	if existing.ID == 0 && !replaced {
		// 新建：加入当前组
		groups = append(groups, Group{Name: "（新建）", Definition: def})
	}
	adj := make(map[string][]string, len(groups))
	for _, g := range groups {
		adj[g.Name] = append([]string(nil), g.Definition.Groups...)
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(groups))
	var dfs func(name string) error
	dfs = func(name string) error {
		color[name] = gray
		for _, next := range adj[name] {
			if next == node.ForceDirect || next == node.ForceOverseas || next == node.ForceFallback {
				continue // 强制组无定义，视为叶子
			}
			if _, ok := adj[next]; !ok {
				continue // 引用外部组时已由存在性校验拦截，这里仅防御
			}
			switch color[next] {
			case gray:
				return fmt.Errorf("%w: 代理组存在环: %s → %s", ErrBadRequest, name, next)
			case white:
				if err := dfs(next); err != nil {
					return err
				}
			}
		}
		color[name] = black
		return nil
	}
	for _, g := range groups {
		if color[g.Name] == white {
			if err := dfs(g.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) getRaw(ctx context.Context, id int64) (Group, error) {
	row := s.store.DB().QueryRowContext(ctx,
		`SELECT id, name, type, COALESCE(preset_key,''), enabled, definition_json FROM proxy_groups WHERE id = ?`, id)
	g, err := scanGroup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	if err != nil {
		return Group{}, err
	}
	return g, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanGroup(row rowScanner) (Group, error) {
	var g Group
	var raw string
	var enabled int
	if err := row.Scan(&g.ID, &g.Name, &g.Type, &g.PresetKey, &enabled, &raw); err != nil {
		return Group{}, err
	}
	g.Enabled = enabled == 1
	if err := json.Unmarshal([]byte(raw), &g.Definition); err != nil {
		return Group{}, fmt.Errorf("解析代理组定义失败: %w", err)
	}
	return g, nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isUniqueViolation(err error) bool {
	return err != nil && (containsString(err.Error(), "UNIQUE constraint failed") || containsString(err.Error(), "constraint failed"))
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && indexOf(s, sub) >= 0))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
