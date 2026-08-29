package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/assembly"
	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
	"vpn-sub/internal/platform"
	"vpn-sub/internal/pool"
	"vpn-sub/internal/proxygroup"
	"vpn-sub/internal/rule"
	"vpn-sub/internal/store"
	"vpn-sub/internal/subscription"
	"vpn-sub/internal/token"
	"vpn-sub/internal/version"
	"vpn-sub/migrations"
)

func newAssemblyTestEnv(t *testing.T) (*gin.Engine, *store.Store, *config.Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	lg := log.New("error", "console")
	cfg := config.NewService(st, lg)
	if err := cfg.Set(context.Background(), config.KeySigningKey, "test-signing-key-0123456789abcdef"); err != nil {
		t.Fatalf("写入签名密钥失败: %v", err)
	}
	dataDir := t.TempDir()
	versionSvc := version.NewService(st, dataDir, lg)
	nodeSvc := node.NewService(st, cfg, lg)
	proxyGroupSvc := proxygroup.NewService(st, lg)
	poolSvc := pool.NewService(st, lg)
	platformSvc := platform.NewService(st, dataDir, versionSvc, lg)
	subSvc := subscription.NewService(st, versionSvc, lg)
	tokenSvc := token.NewService(st, lg)
	ruleSvc := rule.NewService(st, versionSvc, tokenSvc, subSvc, lg)
	assemblySvc := assembly.NewService(st, cfg, lg)
	h := &AssemblyHandler{
		assemblySvc: assemblySvc, nodeSvc: nodeSvc, proxyGroupSvc: proxyGroupSvc,
		poolSvc: poolSvc, platformSvc: platformSvc, ruleSvc: ruleSvc,
		versionSvc: versionSvc,
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	noop := func(c *gin.Context) { c.Next() }
	RegisterAssemblyRoutes(engine, h, noop, noop)
	return engine, st, cfg
}

func insertAssemblyBase(t *testing.T, st *store.Store) int64 {
	t.Helper()
	ctx := context.Background()
	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO platforms (slug, name, product_type) VALUES ('platform-assembly', '装配平台', 'yaml')`)
	if err != nil {
		t.Fatalf("插入平台失败: %v", err)
	}
	pid, _ := res.LastInsertId()
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO subscriptions (slug, name, platform_id, product_type) VALUES ('sub-assembly','装配订阅',?,'yaml')`, pid); err != nil {
		t.Fatalf("插入订阅失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO nodes (source, name, protocol, host, port, protocol_json) VALUES ('manual','节点A','vless','example.com',443,'{"uuid":"11111111-2222-3333-4444-555555555555"}')`); err != nil {
		t.Fatalf("插入节点失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO proxy_groups (name, type, preset_key, enabled, definition_json) VALUES ('组A','custom','',1,'{"type":"select","nodes":["节点A"],"groups":[]}')`); err != nil {
		t.Fatalf("插入代理组失败: %v", err)
	}
	return pid
}

func assemblyBody(pid int64) map[string]any {
	return map[string]any{
		"target_syntax":     "clash-yaml",
		"platform_id":       pid,
		"node_names":        []string{"节点A"},
		"group_names":       []string{"组A"},
		"group_node_orders": map[string][]string{"组A": {"节点A"}},
		"overseas_members":  []string{"节点A"},
		"fallback_group_members": []string{
			"🚀直接连接",
			"🌎国外流量",
		},
		"fixed_params": map[string]any{"port": 7890},
		"pools":        []any{},
	}
}

func doJSON(t *testing.T, engine *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestAssemblyGenerateAndBlueprint(t *testing.T) {
	engine, st, _ := newAssemblyTestEnv(t)
	pid := insertAssemblyBase(t, st)
	ctx := context.Background()
	// preview 不落库
	body := assemblyBody(pid)
	w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/preview", body)
	if w.Code != http.StatusOK {
		t.Fatalf("preview 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	var before int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM versions`).Scan(&before); err != nil {
		t.Fatalf("查询版本数失败: %v", err)
	}
	if before != 0 {
		t.Fatalf("preview 不应产生版本: %d", before)
	}
	var previewResp struct {
		Data struct {
			PreviewHash string `json:"preview_hash"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &previewResp); err != nil || previewResp.Data.PreviewHash == "" {
		t.Fatalf("preview 应返回内容摘要: err=%v body=%s", err, w.Body.String())
	}
	body["preview_hash"] = previewResp.Data.PreviewHash
	// 首次 generate 自动激活
	w = doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", body)
	if w.Code != http.StatusOK {
		t.Fatalf("generate 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	var genResp struct {
		Data struct {
			VersionID     int64 `json:"version_id"`
			AutoActivated bool  `json:"auto_activated"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &genResp); err != nil {
		t.Fatalf("解析 generate 响应失败: %v", err)
	}
	if genResp.Data.VersionID <= 0 || !genResp.Data.AutoActivated {
		t.Fatalf("首版应自动激活: %+v", genResp.Data)
	}
	var blueprintCount int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM assembly_blueprints WHERE version_id = ?`, genResp.Data.VersionID).Scan(&blueprintCount); err != nil {
		t.Fatalf("查询蓝图失败: %v", err)
	}
	if blueprintCount != 1 {
		t.Fatalf("蓝图应 1:1，实际 %d", blueprintCount)
	}
	// 第二次 generate 不自动激活
	w = doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", body)
	if w.Code != http.StatusOK {
		t.Fatalf("第二次 generate 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &genResp); err != nil {
		t.Fatalf("解析第二次响应失败: %v", err)
	}
	if genResp.Data.AutoActivated {
		t.Fatal("第二版不应自动激活")
	}
	// blueprint 端点
	path := "/api/admin/versions/" + jsonInt(genResp.Data.VersionID) + "/blueprint"
	w = doJSON(t, engine, http.MethodGet, path, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("blueprint 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	var bpResp struct {
		Data struct {
			Blueprint map[string]any `json:"blueprint"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &bpResp); err != nil {
		t.Fatalf("解析 blueprint 响应失败: %v", err)
	}
	if bpResp.Data.Blueprint["target_syntax"] != "clash-yaml" {
		t.Fatalf("blueprint 内容异常: %+v", bpResp.Data.Blueprint)
	}
}

func TestAssemblyGenerateRejectsChangedPreviewContent(t *testing.T) {
	engine, st, _ := newAssemblyTestEnv(t)
	pid := insertAssemblyBase(t, st)
	ctx := context.Background()
	res, err := st.DB().ExecContext(ctx, `INSERT INTO rule_pools (name, urls_json) VALUES ('摘要校验池','[]')`)
	if err != nil {
		t.Fatalf("插入素材池失败: %v", err)
	}
	poolID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("读取素材池 ID 失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order) VALUES (?, 'DOMAIN-SUFFIX', 'before.example', 'manual', 1)`, poolID); err != nil {
		t.Fatalf("插入素材池条目失败: %v", err)
	}
	body := assemblyBody(pid)
	body["pools"] = []map[string]any{{"pool_id": poolID, "target": "组A"}}
	w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/preview", body)
	if w.Code != http.StatusOK {
		t.Fatalf("preview 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	var previewResp struct {
		Data struct {
			PreviewHash string `json:"preview_hash"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &previewResp); err != nil || previewResp.Data.PreviewHash == "" {
		t.Fatalf("解析 preview 摘要失败: err=%v body=%s", err, w.Body.String())
	}
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE pool_entries SET match_value = 'after.example' WHERE pool_id = ?`, poolID); err != nil {
		t.Fatalf("更新素材池条目失败: %v", err)
	}
	body["preview_hash"] = previewResp.Data.PreviewHash
	w = doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", body)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "装配依赖已变化，请重新预览") {
		t.Fatalf("素材池内容变化后应拒绝生成: code=%d body=%s", w.Code, w.Body.String())
	}
	var versions int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM versions`).Scan(&versions); err != nil {
		t.Fatalf("查询版本数失败: %v", err)
	}
	if versions != 0 {
		t.Fatalf("摘要冲突不应创建版本: %d", versions)
	}
}

func TestAssemblySelfCheckPreviewWarnsAndGenerateRejects(t *testing.T) {
	engine, st, _ := newAssemblyTestEnv(t)
	pid := insertAssemblyBase(t, st)
	if _, err := st.DB().Exec(`UPDATE nodes SET protocol_json = '{}' WHERE name = '节点A'`); err != nil {
		t.Fatalf("破坏节点测试数据失败: %v", err)
	}
	w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/preview", assemblyBody(pid))
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("产物自检[error]")) {
		t.Fatalf("preview 应返回自检告警: code=%d body=%s", w.Code, w.Body.String())
	}
	w = doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", assemblyBody(pid))
	if w.Code != http.StatusBadRequest || !bytes.Contains(w.Body.Bytes(), []byte("产物自检未通过")) {
		t.Fatalf("generate 应被自检阻断: code=%d body=%s", w.Code, w.Body.String())
	}
}

func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestAssemblyGenerateSrConf(t *testing.T) {
	engine, st, _ := newAssemblyTestEnv(t)
	ctx := context.Background()
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO rules (slug, name, client_type) VALUES ('rule-assembly', '规则', 'shadowrocket')`); err != nil {
		t.Fatalf("插入规则失败: %v", err)
	}
	var ruleID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM rules WHERE slug='rule-assembly'`).Scan(&ruleID); err != nil {
		t.Fatalf("查询规则失败: %v", err)
	}
	body := map[string]any{
		"target_syntax":   "sr-conf",
		"rule_id":         ruleID,
		"fixed_params":    map[string]any{"loglevel": "warning"},
		"pools":           []any{},
		"final_direction": "DIRECT",
	}
	w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", body)
	if w.Code != http.StatusOK {
		t.Fatalf("sr-conf generate 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	var genResp struct {
		Data struct {
			VersionID int64 `json:"version_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &genResp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	var owner string
	if err := st.DB().QueryRowContext(ctx, `SELECT owner_type FROM versions WHERE id = ?`, genResp.Data.VersionID).Scan(&owner); err != nil {
		t.Fatalf("查询版本失败: %v", err)
	}
	if owner != "rule" {
		t.Fatalf("sr-conf 版本应属于 rule，实际 %s", owner)
	}
}

func TestAssemblyGenerateSrConfAutoCreateRule(t *testing.T) {
	engine, st, _ := newAssemblyTestEnv(t)
	ctx := context.Background()
	body := map[string]any{
		"target_syntax":   "sr-conf",
		"rule_name":       "自动新建规则",
		"fixed_params":    map[string]any{"loglevel": "warning"},
		"pools":           []any{},
		"final_direction": "DIRECT",
	}
	// 无预建规则时 Preview 也应成功
	w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/preview", body)
	if w.Code != http.StatusOK {
		t.Fatalf("sr-conf 无预建规则 preview 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	// Generate 自动创建规则
	w = doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", body)
	if w.Code != http.StatusOK {
		t.Fatalf("sr-conf 自动建规则 generate 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	var genResp struct {
		Data struct {
			VersionID int64 `json:"version_id"`
			RuleID    int64 `json:"rule_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &genResp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if genResp.Data.RuleID <= 0 {
		t.Fatalf("应返回自动创建的新规则 id: %+v", genResp.Data)
	}
	var ruleCount int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM rules WHERE id = ?`, genResp.Data.RuleID).Scan(&ruleCount); err != nil {
		t.Fatalf("查询自动创建规则失败: %v", err)
	}
	if ruleCount != 1 {
		t.Fatalf("自动创建的规则应存在: %d", ruleCount)
	}
	var owner string
	if err := st.DB().QueryRowContext(ctx, `SELECT owner_type FROM versions WHERE id = ?`, genResp.Data.VersionID).Scan(&owner); err != nil {
		t.Fatalf("查询版本失败: %v", err)
	}
	if owner != "rule" {
		t.Fatalf("自动创建的 sr-conf 版本应属于 rule，实际 %s", owner)
	}
	var bpRuleID int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COALESCE(rule_id,0) FROM assembly_blueprints WHERE version_id = ?`, genResp.Data.VersionID).Scan(&bpRuleID); err != nil {
		t.Fatalf("查询蓝图 rule_id 失败: %v", err)
	}
	if bpRuleID != genResp.Data.RuleID {
		t.Fatalf("蓝图 rule_id 应为自动创建规则 %d，实际 %d", genResp.Data.RuleID, bpRuleID)
	}
}
