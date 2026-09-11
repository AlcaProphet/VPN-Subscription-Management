package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func exchangeWithTicket(srv *Server, ticket string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/oidc/exchange", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_login_ticket", Value: ticket})
	w := httptest.NewRecorder()
	srv.Engine().ServeHTTP(w, req)
	return w
}

// TestOidcExchangeTicketHTTP 换票成功 200 且严格一次性；过期/未知 401；清理失败按 500 暴露。
func TestOidcExchangeTicketHTTP(t *testing.T) {
	srv := newDownloadTestServer(t)
	if _, err := srv.store.DB().Exec(`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES ('t-valid','session-1',?)`, time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("插入有效 ticket 失败: %v", err)
	}
	w := exchangeWithTicket(srv, "t-valid")
	if w.Code != http.StatusOK {
		t.Fatalf("换票应 200: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Data.Token != "session-1" {
		t.Fatalf("换票响应异常: %v %s", err, w.Body.String())
	}
	if w2 := exchangeWithTicket(srv, "t-valid"); w2.Code != http.StatusUnauthorized {
		t.Fatalf("二次换票应 401: %d", w2.Code)
	}

	if _, err := srv.store.DB().Exec(`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES ('t-expired','session-2',?)`, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("插入过期 ticket 失败: %v", err)
	}
	if w := exchangeWithTicket(srv, "t-expired"); w.Code != http.StatusUnauthorized {
		t.Fatalf("过期 ticket 应 401: %d", w.Code)
	}

	if _, err := srv.store.DB().Exec(`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES ('t-fail','session-3',?)`, time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("插入失败 ticket 失败: %v", err)
	}
	if _, err := srv.store.DB().Exec(`
		CREATE TRIGGER fail_ticket_delete_http BEFORE DELETE ON oidc_login_tickets
		BEGIN
			SELECT RAISE(ABORT, 'fail ticket delete');
		END;`); err != nil {
		t.Fatalf("创建删除失败触发器失败: %v", err)
	}
	if w := exchangeWithTicket(srv, "t-fail"); w.Code != http.StatusInternalServerError {
		t.Fatalf("删除清理失败应 500 而非 401: %d %s", w.Code, w.Body.String())
	}
}
