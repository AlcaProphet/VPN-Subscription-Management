package assembly

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"vpn-sub/internal/config"
	"vpn-sub/internal/node"
	"vpn-sub/internal/proxygroup"
)

// nodeData 渲染用节点数据（凭据已解密）。
type nodeData struct {
	Name            string
	Source          string
	Protocol        string
	Host            string
	Port            int
	DisplayName     *string
	ProtocolJSON    map[string]any
	Enabled         bool
	Allocatable     bool
	Missing         bool
	InstanceEnabled bool
	RenderName      string
}

// groupData 渲染用代理组数据（节点引用仅来自本次装配 GroupNodeOrders）。
type groupData struct {
	Name    string
	Type    string // preset / custom
	Enabled bool
	proxygroup.Definition
}

// poolEntry 素材池条目。
type poolEntry struct {
	RuleType   string
	MatchValue string
}

// platformInfo 平台目标信息。
type platformInfo struct {
	ID              int64
	ProductType     string
	HasSubscription bool
}

// ruleInfo 规则目标信息。
type ruleInfo struct {
	ID int64
}

// loadedData 一次装配的只读上下文。
type loadedData struct {
	nodes     map[string]*nodeData
	groups    map[string]*groupData
	allGroups map[string]*groupData
	pools     map[int64][]poolEntry
	platform  *platformInfo
	rule      *ruleInfo
}

// loadData 加载本次装配所需只读数据。
func (s *Service) loadData(ctx context.Context, in GenerateInput) (*loadedData, error) {
	ld := &loadedData{
		nodes:     map[string]*nodeData{},
		groups:    map[string]*groupData{},
		allGroups: map[string]*groupData{},
		pools:     map[int64][]poolEntry{},
	}
	if err := s.loadNodes(ctx, ld, in.NodeNames); err != nil {
		return nil, err
	}
	if err := s.loadGroups(ctx, ld, in.GroupNames); err != nil {
		return nil, err
	}
	for _, p := range in.Pools {
		if _, ok := ld.pools[p.PoolID]; ok {
			continue
		}
		entries, err := s.loadPoolEntries(ctx, p.PoolID)
		if err != nil {
			return nil, err
		}
		ld.pools[p.PoolID] = entries
	}
	if in.PlatformID > 0 {
		pi, err := s.loadPlatform(ctx, in.PlatformID)
		if err != nil {
			return nil, err
		}
		ld.platform = pi
	}
	if in.RuleID > 0 {
		ri, err := s.loadRule(ctx, in.RuleID)
		if err != nil {
			return nil, err
		}
		ld.rule = ri
	}
	return ld, nil
}

func (s *Service) loadNodes(ctx context.Context, ld *loadedData, names []string) error {
	for _, name := range names {
		var nd nodeData
		var display sql.NullString
		var instanceEnabled sql.NullInt64
		var enabled, allocatable, missing int
		var protocolRaw string
		err := s.store.DB().QueryRowContext(ctx,
			`SELECT n.source, n.name, n.display_name, n.protocol, n.host, n.port, n.protocol_json,
			        n.enabled, n.allocatable, n.missing, COALESCE(i.enabled, 1)
			 FROM nodes n LEFT JOIN xray_instances i ON i.id = n.instance_id
			 WHERE n.name = ?`, name).
			Scan(&nd.Source, &nd.Name, &display, &nd.Protocol, &nd.Host, &nd.Port, &protocolRaw,
				&enabled, &allocatable, &missing, &instanceEnabled)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: 节点不存在: %s", ErrBadRequest, name)
		}
		if err != nil {
			return err
		}
		if display.Valid && display.String != "" {
			nd.DisplayName = &display.String
		}
		nd.Enabled = enabled == 1
		nd.Allocatable = allocatable == 1
		nd.Missing = missing == 1
		nd.InstanceEnabled = instanceEnabled.Int64 != 0
		if err := json.Unmarshal([]byte(protocolRaw), &nd.ProtocolJSON); err != nil {
			return fmt.Errorf("解析节点参数失败: %s", name)
		}
		if err := s.decryptNode(&nd); err != nil {
			return err
		}
		nd.RenderName = nodeRenderName(nd)
		ld.nodes[name] = &nd
	}
	return nil
}

func (s *Service) loadGroups(ctx context.Context, ld *loadedData, names []string) error {
	// 全部代理组用于校验未勾选子组是否存在；选中组放入 ld.groups。
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT name, type, COALESCE(preset_key,''), enabled, definition_json FROM proxy_groups`)
	if err != nil {
		return err
	}
	defer rows.Close()
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	for rows.Next() {
		var g groupData
		var gtype, presetKey string
		var enabled int
		var raw string
		if err := rows.Scan(&g.Name, &gtype, &presetKey, &enabled, &raw); err != nil {
			return err
		}
		g.Type = gtype
		g.Enabled = enabled == 1
		var def proxygroup.Definition
		if err := json.Unmarshal([]byte(raw), &def); err != nil {
			return fmt.Errorf("解析代理组定义失败: %s", g.Name)
		}
		g.Definition = def
		ld.allGroups[g.Name] = &g
		if want[g.Name] {
			copyG := g
			ld.groups[g.Name] = &copyG
		}
	}
	return rows.Err()
}

func (s *Service) loadPoolEntries(ctx context.Context, poolID int64) ([]poolEntry, error) {
	var exists int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rule_pools WHERE id = ?`, poolID).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, fmt.Errorf("%w: 素材池不存在: %d", ErrBadRequest, poolID)
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT rule_type, match_value FROM pool_entries WHERE pool_id = ? ORDER BY sort_order, id`, poolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]poolEntry, 0)
	for rows.Next() {
		var e poolEntry
		if err := rows.Scan(&e.RuleType, &e.MatchValue); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Service) loadPlatform(ctx context.Context, id int64) (*platformInfo, error) {
	var pi platformInfo
	var hasSub int
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT p.id, p.product_type,
		        (SELECT COUNT(*) FROM subscriptions s WHERE s.platform_id = p.id)
		 FROM platforms p WHERE p.id = ?`, id).Scan(&pi.ID, &pi.ProductType, &hasSub)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: 平台不存在", ErrBadRequest)
	}
	if err != nil {
		return nil, err
	}
	pi.HasSubscription = hasSub > 0
	return &pi, nil
}

func (s *Service) loadRule(ctx context.Context, id int64) (*ruleInfo, error) {
	var ri ruleInfo
	err := s.store.DB().QueryRowContext(ctx, `SELECT id FROM rules WHERE id = ?`, id).Scan(&ri.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: 规则不存在", ErrBadRequest)
	}
	return &ri, err
}

// decryptNode 将节点 protocol_json 中的敏感字段解密为明文。
func (s *Service) decryptNode(nd *nodeData) error {
	for _, path := range node.SensitiveFieldsOf(nd.Protocol) {
		v, ok := node.GetPath(nd.ProtocolJSON, path)
		if !ok {
			continue
		}
		str, ok := v.(string)
		if !ok || !strings.HasPrefix(str, encPrefix) {
			continue
		}
		key, err := s.cfg.Get(context.Background(), config.KeySigningKey)
		if err != nil || key == "" {
			return errors.New("签名密钥未配置，无法解密节点凭据")
		}
		plain, err := config.Decrypt(strings.TrimPrefix(str, encPrefix), []byte(key))
		if err != nil {
			return fmt.Errorf("解密节点凭据失败: %s", nd.Name)
		}
		node.SetPath(nd.ProtocolJSON, path, string(plain))
	}
	return nil
}

func nodeRenderName(nd nodeData) string {
	if nd.DisplayName != nil && *nd.DisplayName != "" {
		return *nd.DisplayName
	}
	return nd.Name
}
