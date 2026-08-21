package xray

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/curve25519"

	proxyman "github.com/xtls/xray-core/app/proxyman"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/vless"
	vlessinbound "github.com/xtls/xray-core/proxy/vless/inbound"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/reality"

	"vpn-sub/internal/node"
)

// NodeChangeKind 描述检测后的节点可见性变化。
type NodeChangeKind string

const (
	NodeChangeAdded       NodeChangeKind = "added"
	NodeChangeRecovered   NodeChangeKind = "recovered"
	NodeChangeMissing     NodeChangeKind = "missing"
	NodeChangeAllocatable NodeChangeKind = "allocatable"
)

// NodeChange 是节点可见性变化通知。
type NodeChange struct {
	NodeID int64
	Tag    string
	Kind   NodeChangeKind
}

// DetectResult 节点检测返回（HTTP 响应）。
type DetectResult struct {
	Added      int           `json:"added"`
	Updated    int           `json:"updated"`
	Missing    int           `json:"missing"`
	Skipped    []SkippedItem `json:"skipped"`
	AddedNodes []AddedNode   `json:"added_nodes"`
}

// SkippedItem 检测跳过项。
type SkippedItem struct {
	Tag    string `json:"tag"`
	Reason string `json:"reason"`
}

// AddedNode 新增节点信息。
type AddedNode struct {
	NodeID int64  `json:"node_id"`
	Tag    string `json:"tag"`
	Name   string `json:"name"`
}

// DetectNodes 对实例执行 ListInbounds 并 upsert 入库。
func (s *InstanceService) DetectNodes(ctx context.Context, instanceID int64) (*DetectResult, error) {
	inst, err := s.Get(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if !inst.Enabled {
		return nil, ErrDisabled
	}
	client, err := s.ClientFor(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, DialTimeout)
	defer cancel()
	resp, err := client.ListInbounds(probeCtx)
	if err != nil {
		return nil, fmt.Errorf("ListInbounds 失败: %w", err)
	}

	result := &DetectResult{Skipped: []SkippedItem{}, AddedNodes: []AddedNode{}}
	seen := map[string]bool{}
	allocatableChanged := []NodeChange{}
	recovered := []NodeChange{}
	var missingChanges []NodeChange

	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		for _, in := range resp.Inbounds {
			tag := in.GetTag()
			seen[tag] = true
			if in.GetProxySettings() == nil {
				result.Skipped = append(result.Skipped, SkippedItem{Tag: in.GetTag(), Reason: "缺少 ProxySettings"})
				continue
			}
			protocolName := protocolFromType(in.GetProxySettings().GetType())
			port := portFromReceiver(in.GetReceiverSettings())
			protocolJSON := buildProtocolJSON(in.GetProxySettings(), in.GetReceiverSettings())
			stableName := inst.Slug + "-" + tag
			if err := node.ValidateNodeName(stableName); err != nil {
				result.Skipped = append(result.Skipped, SkippedItem{Tag: tag, Reason: "节点名非法: " + err.Error()})
				continue
			}
			// 检测入库前校验命名空间（排除自身后由后续 upsert 处理）。
			existingID, err := existingNodeID(ctx, tx, inst.ID, tag)
			if err != nil {
				return err
			}
			oldMissing := false
			if existingID == 0 {
				if err := checkRenderNameForNewXray(ctx, tx, stableName); err != nil {
					result.Skipped = append(result.Skipped, SkippedItem{Tag: tag, Reason: err.Error()})
					continue
				}
			} else {
				oldMissing, err = nodeMissing(ctx, tx, existingID)
				if err != nil {
					return err
				}
			}
			allocatable := isAllocatable(protocolName)
			rawJSON, err := json.Marshal(protocolJSON)
			if err != nil {
				return fmt.Errorf("序列化节点参数失败: %w", err)
			}
			res, err := tx.ExecContext(ctx,
				`INSERT INTO nodes (source, name, display_name, instance_id, tag, protocol, host, port, protocol_json, is_public, enabled, allocatable, missing, last_seen_at)
				 VALUES ('xray', ?, NULL, ?, ?, ?, ?, ?, ?, 0, 1, ?, 0, CURRENT_TIMESTAMP)
				 ON CONFLICT(instance_id, tag) DO UPDATE SET
				   protocol = excluded.protocol,
				   host = excluded.host,
				   port = excluded.port,
				   protocol_json = excluded.protocol_json,
				   allocatable = excluded.allocatable,
				   missing = 0,
				   last_seen_at = CURRENT_TIMESTAMP`,
				stableName, inst.ID, tag, protocolName, hostFromAddr(inst.APIAddr), port, string(rawJSON), boolInt(allocatable))
			if err != nil {
				return fmt.Errorf("upsert 节点失败: %w", err)
			}
			id, err := res.LastInsertId()
			if err != nil {
				return err
			}
			if id == 0 {
				// SQLite UPSERT 的 LastInsertId 可能不返回已有行 id，单独查询。
				id, err = existingNodeID(ctx, tx, inst.ID, tag)
				if err != nil {
					return err
				}
			}
			if existingID == 0 {
				result.Added++
				result.AddedNodes = append(result.AddedNodes, AddedNode{NodeID: id, Tag: tag, Name: stableName})
				allocatableChanged = append(allocatableChanged, NodeChange{NodeID: id, Tag: tag, Kind: NodeChangeAdded})
			} else {
				result.Updated++
				// 检测 missing 恢复（1→0）与 allocatable 变化（1→0 或 0→1）在提交后回调。
				if oldMissing {
					recovered = append(recovered, NodeChange{NodeID: existingID, Tag: tag, Kind: NodeChangeRecovered})
				}
				oldAlloc, err := nodeAllocatable(ctx, tx, existingID)
				if err != nil {
					return err
				}
				if oldAlloc != allocatable {
					allocatableChanged = append(allocatableChanged, NodeChange{NodeID: existingID, Tag: tag, Kind: NodeChangeAllocatable})
				}
			}
		}
		// 本实例既有节点不在本次响应集合 → missing=1。
		rows, err := tx.QueryContext(ctx,
			`SELECT id, tag FROM nodes WHERE instance_id = ? AND missing = 0`, inst.ID)
		if err != nil {
			return err
		}
		var nowMissing []NodeChange
		for rows.Next() {
			var id int64
			var tag string
			if err := rows.Scan(&id, &tag); err != nil {
				_ = rows.Close()
				return err
			}
			if !seen[tag] {
				if _, err := tx.ExecContext(ctx,
					`UPDATE nodes SET missing = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
					_ = rows.Close()
					return err
				}
				nowMissing = append(nowMissing, NodeChange{NodeID: id, Tag: tag, Kind: NodeChangeMissing})
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
		result.Missing = len(nowMissing)
		missingChanges = nowMissing
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务提交后回调（Step3 接线；本 Step 可空）。
	if s.OnNodeVisibilityChanged != nil {
		all := append([]NodeChange{}, recovered...)
		all = append(all, missingChanges...)
		all = append(all, allocatableChanged...)
		if len(all) > 0 {
			s.OnNodeVisibilityChanged(ctx, all)
		}
	}
	return result, nil
}

// SetOnNodeVisibilityChanged 注入检测回调。
func (s *InstanceService) SetOnNodeVisibilityChanged(fn func(ctx context.Context, changes []NodeChange)) {
	s.OnNodeVisibilityChanged = fn
}

// protocolFromType 从 TypedMessage Type 提取协议名。
func protocolFromType(typeURL string) string {
	switch {
	case strings.Contains(typeURL, ".vless."):
		return "vless"
	case strings.Contains(typeURL, ".vmess."):
		return "vmess"
	case strings.Contains(typeURL, ".trojan."):
		return "trojan"
	case strings.Contains(typeURL, ".shadowsocks."):
		return "shadowsocks"
	}
	parts := strings.Split(typeURL, ".")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if last == "Config" {
		if len(parts) >= 3 && parts[len(parts)-2] == "inbound" {
			return parts[len(parts)-3]
		}
		if len(parts) >= 2 {
			return parts[len(parts)-2]
		}
	}
	return last
}

// portFromReceiver 从 ReceiverSettings 解析端口。
func portFromReceiver(tm *serial.TypedMessage) int {
	if tm == nil {
		return 0
	}
	inst, err := tm.GetInstance()
	if err != nil {
		return 0
	}
	rc, ok := inst.(*proxyman.ReceiverConfig)
	if !ok || rc.GetPortList() == nil || len(rc.GetPortList().GetRange()) == 0 {
		return 0
	}
	return int(rc.GetPortList().GetRange()[0].GetFrom())
}

// buildProtocolJSON 从 ProxySettings/StreamSettings 归一化渲染字段。
func buildProtocolJSON(proxyTM, receiverTM *serial.TypedMessage) map[string]any {
	out := map[string]any{}
	if proxyTM != nil {
		if inst, err := proxyTM.GetInstance(); err == nil {
			extractInboundProtocolParams(out, inst)
			mergeProtoMap(out, inst)
		}
	}
	// StreamSettings
	if receiverTM != nil {
		if inst, err := receiverTM.GetInstance(); err == nil {
			if rc, ok := inst.(*proxyman.ReceiverConfig); ok && rc.GetStreamSettings() != nil {
				mergeStreamSettings(out, rc.GetStreamSettings())
			}
		}
	}
	// 清理可能出现的私钥与用户列表等非渲染字段。
	delete(out, "private_key")
	delete(out, "privateKey")
	delete(out, "PrivateKey")
	delete(out, "users")
	delete(out, "clients")
	delete(out, "user")
	delete(out, "fallbacks")
	delete(out, "decryption")
	delete(out, "default")
	delete(out, "detour")
	return out
}

// extractInboundProtocolParams 在删除 users/clients 前提取渲染/推送必需的入站协议参数。
func extractInboundProtocolParams(out map[string]any, inst any) {
	switch cfg := inst.(type) {
	case *vlessinbound.Config:
		for _, u := range cfg.GetClients() {
			if u.GetAccount() == nil {
				continue
			}
			a, err := u.GetAccount().GetInstance()
			if err != nil {
				continue
			}
			if va, ok := a.(*vless.Account); ok && va.GetFlow() != "" {
				out["flow"] = va.GetFlow()
				return
			}
		}
	case *shadowsocks.ServerConfig:
		for _, u := range cfg.GetUsers() {
			if u.GetAccount() == nil {
				continue
			}
			a, err := u.GetAccount().GetInstance()
			if err != nil {
				continue
			}
			if sa, ok := a.(*shadowsocks.Account); ok {
				if name := cipherNameOf(sa.GetCipherType()); name != "" {
					out["cipher"] = name
					return
				}
			}
		}
	}
}

func mergeProtoMap(dst map[string]any, msg any) {
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return
	}
	for k, v := range m {
		dst[k] = v
	}
}

func mergeStreamSettings(dst map[string]any, sc *internet.StreamConfig) {
	if sc == nil {
		return
	}
	if sc.GetProtocolName() != "" {
		dst["network"] = sc.GetProtocolName()
	}
	if sc.GetSecurityType() != "" {
		dst["security"] = sc.GetSecurityType()
	}
	for _, tc := range sc.GetTransportSettings() {
		if tc.GetSettings() == nil {
			continue
		}
		inst, err := tc.GetSettings().GetInstance()
		if err != nil {
			continue
		}
		raw, _ := json.Marshal(inst)
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		for k, v := range m {
			if k == "header" {
				continue
			}
			dst[k] = v
		}
		switch tc.GetProtocolName() {
		case "websocket":
			dst["network"] = "ws"
			if host, ok := m["host"].(string); ok && host != "" {
				dst["ws-host"] = host
				dst["host"] = host
			}
			if path, ok := m["path"].(string); ok && path != "" {
				dst["ws-path"] = path
				dst["path"] = path
			}
		case "grpc":
			dst["network"] = "grpc"
			if sn, ok := m["serviceName"].(string); ok && sn != "" {
				dst["serviceName"] = sn
				dst["service_name"] = sn
			}
		case "httpupgrade":
			dst["network"] = "httpupgrade"
			if path, ok := m["path"].(string); ok && path != "" {
				dst["httpupgrade-path"] = path
				dst["path"] = path
			}
			if host, ok := m["host"].(string); ok && host != "" {
				dst["httpupgrade-host"] = host
				dst["host"] = host
			}
		}
	}
	for _, stm := range sc.GetSecuritySettings() {
		inst, err := stm.GetInstance()
		if err != nil {
			continue
		}
		raw, _ := json.Marshal(inst)
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		switch v := inst.(type) {
		case *reality.Config:
			dst["security"] = "reality"
			if len(v.GetServerNames()) > 0 {
				dst["servername"] = v.GetServerNames()[0]
				dst["server_name"] = v.GetServerNames()[0]
			}
			if v.GetFingerprint() != "" {
				dst["fingerprint"] = v.GetFingerprint()
			}
			if len(v.GetShortIds()) > 0 {
				dst["short_id"] = fmt.Sprintf("%x", v.GetShortIds()[0])
				dst["sid"] = dst["short_id"]
			}
			pub := v.GetPublicKey()
			if len(pub) == 0 && len(v.GetPrivateKey()) > 0 {
				if derived, err := curve25519.X25519(v.GetPrivateKey(), curve25519.Basepoint); err == nil {
					pub = derived
				}
			}
			if len(pub) > 0 {
				dst["public_key"] = base64RawURL(pub)
				dst["pbk"] = dst["public_key"]
			}
			// 私钥不落库。
			delete(dst, "private_key")
			delete(dst, "privateKey")
		default:
			if sc.GetSecurityType() == "tls" || strings.Contains(stm.GetType(), "tls") {
				dst["security"] = "tls"
				if sn, ok := m["serverName"].(string); ok && sn != "" {
					dst["servername"] = sn
					dst["server_name"] = sn
				}
				if fp, ok := m["fingerprint"].(string); ok && fp != "" {
					dst["fingerprint"] = fp
				}
				if alpn, ok := m["nextProtocol"].([]any); ok && len(alpn) > 0 {
					dst["alpn"] = alpn
				}
			}
		}
	}
	// shadowsocks cipher 从 proxy 配置中尝试提取；此处不虚构。
}

func base64RawURL(b []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var sb strings.Builder
	for i := 0; i < len(b); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], b[i:])
		sb.WriteByte(chars[chunk[0]>>2])
		sb.WriteByte(chars[(chunk[0]&0x03)<<4|chunk[1]>>4])
		if n > 1 {
			sb.WriteByte(chars[(chunk[1]&0x0f)<<2|chunk[2]>>6])
		} else {
			sb.WriteByte('=')
		}
		if n > 2 {
			sb.WriteByte(chars[chunk[2]&0x3f])
		} else {
			sb.WriteByte('=')
		}
	}
	return sb.String()
}

func isAllocatable(protocol string) bool {
	switch protocol {
	case "vless", "vmess", "trojan", "shadowsocks", "ss":
		return true
	}
	return false
}

func existingNodeID(ctx context.Context, tx *sql.Tx, instanceID int64, tag string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM nodes WHERE instance_id = ? AND tag = ?`, instanceID, tag).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

func nodeAllocatable(ctx context.Context, tx *sql.Tx, id int64) (bool, error) {
	var v int
	if err := tx.QueryRowContext(ctx, `SELECT allocatable FROM nodes WHERE id = ?`, id).Scan(&v); err != nil {
		return false, err
	}
	return v == 1, nil
}

func nodeMissing(ctx context.Context, tx *sql.Tx, id int64) (bool, error) {
	var v int
	if err := tx.QueryRowContext(ctx, `SELECT missing FROM nodes WHERE id = ?`, id).Scan(&v); err != nil {
		return false, err
	}
	return v == 1, nil
}

// checkRenderNameForNewXray 检查新 xray 节点稳定名是否与既有节点/代理组/保留名冲突。
func checkRenderNameForNewXray(ctx context.Context, tx *sql.Tx, name string) error {
	return node.CheckRenderNameNamespaceTx(ctx, tx, name)
}

// 供测试与 Step3 使用的可见性回调字段。
// 注意：此字段定义在 InstanceService 中，避免额外包级状态。
