package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRulePreviewNoVersion 空规则实体/无激活版本预览：HTTP 200 纯文本注释块；有版本后回归正文
func TestRulePreviewNoVersion(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "admin", "admin@x.com", "password123")
	ctx := context.Background()
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO rules (id, slug, name) VALUES (1, 'rule-x', '测试规则')`); err != nil {
		t.Fatalf("创建空规则失败: %v", err)
	}

	w := profileReq(t, srv, http.MethodGet, "/api/rules/1/preview", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("无激活版本预览应 200: %d %s", w.Code, w.Body.String())
	}
	if strings.TrimSpace(w.Body.String()) != "# error: no active version" {
		t.Fatalf("无激活版本注释块异常: %s", w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("预览响应应带 no-store: %s", cc)
	}

	// 补建激活版本后预览回归正文
	root := filepath.Dir(srv.store.DBPath())
	dir := filepath.Join(root, "contents", "rule", "1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建版本目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "v1"), []byte("[Rule]\n"), 0o644); err != nil {
		t.Fatalf("写版本文件失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path, file_name) VALUES ('rule',1,1,'rule/1/v1','rule.conf')`); err != nil {
		t.Fatalf("插入版本失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `UPDATE rules SET current_version = 1 WHERE id = 1`); err != nil {
		t.Fatalf("设置当前版本失败: %v", err)
	}
	w2 := profileReq(t, srv, http.MethodGet, "/api/rules/1/preview", token, nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("有版本预览应 200: %d %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "[Rule]") {
		t.Errorf("预览内容异常: %s", w2.Body.String())
	}
}
