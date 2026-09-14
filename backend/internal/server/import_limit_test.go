package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
)

func newImportTestServer(t *testing.T) *Server {
	t.Helper()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), downloadTestFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	lg := log.New("error", "console")
	cfg := config.NewService(st, lg)
	if err := cfg.Set(context.Background(), "debug_mode", "true"); err != nil {
		t.Fatalf("开启测试调试模式失败: %v", err)
	}
	users := user.NewService(st, cfg, lg)
	streamSvc := log.NewStreamService(log.NewRingBuffer(), lg)
	srv, err := New(st, cfg, users, log.NewRuntime("error", "console"), "prod", mustPolicy(t, "off"), "0", dataDir, streamSvc)
	if err != nil {
		t.Fatalf("装配 server 失败: %v", err)
	}
	return srv
}

func multipartImportBody(t *testing.T, fileSize, paddingSize int) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if paddingSize > 0 {
		fw, err := w.CreateFormField("padding")
		if err != nil {
			t.Fatalf("创建 padding 字段失败: %v", err)
		}
		if _, err := fw.Write(bytes.Repeat([]byte("x"), paddingSize)); err != nil {
			t.Fatalf("写入 padding 失败: %v", err)
		}
	}
	fw, err := w.CreateFormFile("file", "import.enc")
	if err != nil {
		t.Fatalf("创建 file 字段失败: %v", err)
	}
	if _, err := fw.Write(bytes.Repeat([]byte{0}, fileSize)); err != nil {
		t.Fatalf("写入文件内容失败: %v", err)
	}
	if err := w.WriteField("password", "password123"); err != nil {
		t.Fatalf("写入 password 失败: %v", err)
	}
	if err := w.WriteField("confirm_word", "IMPORT"); err != nil {
		t.Fatalf("写入 confirm_word 失败: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关闭 multipart 失败: %v", err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

func sendImportRequest(t *testing.T, srv *Server, path, token string, body []byte, contentType string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if mutate != nil {
		mutate(req)
	}
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	return w
}

// TestImportFileAndRequestBodyLimits 双入口：恰好 20 MiB 允许，20 MiB+1 与完整请求体 >21 MiB 均 413。
func TestImportFileAndRequestBodyLimits(t *testing.T) {
	srv := newImportTestServer(t)
	token := regUser(t, srv, "admin", "admin@x.com", "password123")
	exact, ct := multipartImportBody(t, MaxImportFileBytes, 0)
	for _, tc := range []struct{ path, token string }{
		{"/api/setup/import", ""},
		{"/api/admin/settings/import", token},
	} {
		w := sendImportRequest(t, srv, tc.path, tc.token, exact, ct, nil)
		if w.Code == http.StatusRequestEntityTooLarge || w.Code >= 500 {
			t.Fatalf("%s 恰好 20 MiB 不应被大小/读取失败拒绝: %d %s", tc.path, w.Code, w.Body.String())
		}
	}

	overFile, ct := multipartImportBody(t, MaxImportFileBytes+1, 0)
	for _, tc := range []struct{ path, token string }{
		{"/api/setup/import", ""},
		{"/api/admin/settings/import", token},
	} {
		w := sendImportRequest(t, srv, tc.path, tc.token, overFile, ct, nil)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s 20 MiB+1 应 413: %d %s", tc.path, w.Code, w.Body.String())
		}
	}

	overBody, ct := multipartImportBody(t, MaxImportFileBytes, 2<<20)
	for _, tc := range []struct{ path, token string }{
		{"/api/setup/import", ""},
		{"/api/admin/settings/import", token},
	} {
		w := sendImportRequest(t, srv, tc.path, tc.token, overBody, ct, nil)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s 完整请求体 >21 MiB 应 413: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
}

// TestImportChunkedAndForgedContentLength 无 Content-Length / 分块和伪造 Content-Length 仍受 21 MiB 上限。
func TestImportChunkedAndForgedContentLength(t *testing.T) {
	srv := newImportTestServer(t)
	body, ct := multipartImportBody(t, MaxImportFileBytes, 2<<20)
	// 分块：ContentLength=-1。
	w := sendImportRequest(t, srv, "/api/setup/import", "", body, ct, func(req *http.Request) {
		req.ContentLength = -1
		req.TransferEncoding = []string{"chunked"}
	})
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("分块超限请求应 413: %d %s", w.Code, w.Body.String())
	}
	// 伪造小的 Content-Length，实际 body 仍超 21 MiB。
	w = sendImportRequest(t, srv, "/api/setup/import", "", body, ct, func(req *http.Request) {
		req.ContentLength = 100
	})
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("伪造 Content-Length 超限请求应 413: %d %s", w.Code, w.Body.String())
	}
}

type errAfterReader struct {
	data     []byte
	err      error
	returned bool
}

func (r *errAfterReader) Read(p []byte) (int, error) {
	if r.returned {
		return 0, r.err
	}
	r.returned = true
	n := copy(p, r.data)
	return n, r.err
}

// TestImportReadErrorReturns500 中途读取真实错误不得被当作 EOF 正常结束，也不得创建任务。
func TestImportReadErrorReturns500(t *testing.T) {
	srv := newImportTestServer(t)
	body, ct := multipartImportBody(t, 1024, 0)
	reader := &errAfterReader{data: body[:len(body)/2], err: errors.New("read interrupted")}
	req := httptest.NewRequest(http.MethodPost, "/api/setup/import", io.NopCloser(reader))
	req.Header.Set("Content-Type", ct)
	req.ContentLength = -1
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("中途读取错误应 500: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "读取导入文件失败") {
		t.Fatalf("读取错误响应应给出通用读取失败提示: %s", w.Body.String())
	}
}
