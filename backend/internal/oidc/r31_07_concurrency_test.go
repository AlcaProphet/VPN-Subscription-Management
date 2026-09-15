package oidc

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/user"
)

// newTestOidcServiceForConcurrency 复用标准 Dev mock 测试服务（已启用 mock provider）。
func newTestOidcServiceForConcurrency(t *testing.T) (*Service, *user.Service) {
	t.Helper()
	_, svc, users := newTestOidcService(t)
	return svc, users
}

// r3107AdminOidcOps 将测试用 oidc.Service 适配为 config.OidcOps，供并发停用/流程写入串行化测试复用。
type r3107AdminOidcOps struct{ svc *Service }

func (a r3107AdminOidcOps) SaveParams(ctx context.Context, providerType, baseURL, realm, clientID, clientSecret string) error {
	return a.svc.SaveParams(ctx, providerType, Params{BaseURL: baseURL, Realm: realm, ClientID: clientID, ClientSecret: clientSecret})
}
func (a r3107AdminOidcOps) LoadParams(ctx context.Context, providerType string) (string, string, string, string, error) {
	p, err := a.svc.LoadParams(ctx, providerType)
	if err != nil {
		return "", "", "", "", err
	}
	return p.BaseURL, p.Realm, p.ClientID, p.ClientSecret, nil
}
func (a r3107AdminOidcOps) DescribeParams(ctx context.Context, providerType string) (config.OidcParamsState, error) {
	return a.svc.DescribeParams(ctx, providerType)
}
func (a r3107AdminOidcOps) SaveParamsTx(ctx context.Context, tx *sql.Tx, providerType, baseURL, realm, clientID, clientSecret string) error {
	return a.svc.SaveParamsTx(ctx, tx, providerType, Params{BaseURL: baseURL, Realm: realm, ClientID: clientID, ClientSecret: clientSecret})
}
func (a r3107AdminOidcOps) DescribeParamsTx(ctx context.Context, tx *sql.Tx, providerType string) (config.OidcParamsState, error) {
	return a.svc.DescribeParamsTx(ctx, tx, providerType)
}
func (a r3107AdminOidcOps) IsConfigured(ctx context.Context) bool { return a.svc.IsConfigured(ctx) }
func (a r3107AdminOidcOps) ClearDiscCache()                       { a.svc.ClearDiscCache() }

// r3107NewAdmin 为 OIDC 并发测试构造面板服务（共用同一个测试库/事务串行语义）。
func r3107NewAdmin(t *testing.T, svc *Service) *config.AdminService {
	t.Helper()
	return config.NewAdminService(svc.cfg, svc.store, r3107AdminOidcOps{svc: svc}, t.TempDir(), "dev", svc.log, new(slog.LevelVar))
}

func r3107CreateEventTriggers(t *testing.T, svc *Service) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.store.DB().ExecContext(ctx, `CREATE TABLE r3107_events (seq INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL)`); err != nil {
		t.Fatalf("创建事件表失败: %v", err)
	}
	if _, err := svc.store.DB().ExecContext(ctx, `
		CREATE TRIGGER r3107_disable_event AFTER UPDATE ON system_config
		WHEN NEW.key = 'oidc_configured' AND NEW.value = 'false'
		BEGIN
			INSERT INTO r3107_events (name) VALUES ('disable');
		END;`); err != nil {
		t.Fatalf("创建 disable 事件触发器失败: %v", err)
	}
	if _, err := svc.store.DB().ExecContext(ctx, `
		CREATE TRIGGER r3107_bind_event AFTER UPDATE ON users
		WHEN OLD.oidc_subject IS NULL AND NEW.oidc_subject IS NOT NULL
		BEGIN
			INSERT INTO r3107_events (name) VALUES ('bind');
		END;`); err != nil {
		t.Fatalf("创建 bind 事件触发器失败: %v", err)
	}
	if _, err := svc.store.DB().ExecContext(ctx, `
		CREATE TRIGGER r3107_ticket_event AFTER INSERT ON oidc_login_tickets
		BEGIN
			INSERT INTO r3107_events (name) VALUES ('ticket');
		END;`); err != nil {
		t.Fatalf("创建 ticket 事件触发器失败: %v", err)
	}
}

func r3107EventNames(t *testing.T, svc *Service) []string {
	t.Helper()
	rows, err := svc.store.DB().Query(`SELECT name FROM r3107_events ORDER BY seq`)
	if err != nil {
		t.Fatalf("读取事件表失败: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("扫描事件表失败: %v", err)
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历事件表失败: %v", err)
	}
	return out
}

func r3107AssertNoPostDisableResult(t *testing.T, events []string, resultName string) {
	t.Helper()
	disableIdx := -1
	resultIdx := -1
	for i, name := range events {
		if name == "disable" {
			disableIdx = i
		}
		if name == resultName {
			resultIdx = i
		}
	}
	if disableIdx < 0 {
		t.Fatalf("事件中缺少 disable，实际顺序: %v", events)
	}
	if resultIdx >= 0 && resultIdx > disableIdx {
		t.Fatalf("停用提交后仍出现 %s，事件顺序: %v", resultName, events)
	}
}

// TestR3107ConcurrentDisableVsBindSerialized 使用 SQLite 触发器记录事务内写入顺序：
// 绑定事件一旦出现，必须排在停用事件之前；不允许停用提交后再落库新绑定。
func TestR3107ConcurrentDisableVsBindSerialized(t *testing.T) {
	for i := 0; i < 8; i++ {
		svc, users := newTestOidcServiceForConcurrency(t)
		adminSvc := r3107NewAdmin(t, svc)
		u, err := users.Register(ctx, "r3107-bind-concurrent", "r3107-bind-concurrent@example.com", "password123")
		if err != nil {
			t.Fatalf("创建绑定目标失败: %v", err)
		}
		_, state, err := svc.StartFlow(ctx, "bind", u.ID)
		if err != nil {
			t.Fatalf("StartFlow 失败: %v", err)
		}
		rec, err := svc.ConsumeState(ctx, state)
		if err != nil {
			t.Fatalf("ConsumeState 失败: %v", err)
		}
		r3107CreateEventTriggers(t, svc)

		var wg sync.WaitGroup
		start := make(chan struct{})
		var bindErr, disableErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			bindErr = svc.ResolveBind(ctx, rec, &Identity{
				Subject:       "r3107-concurrent-subject",
				Email:         "r3107-concurrent-subject@example.com",
				EmailVerified: true,
			})
		}()
		go func() {
			defer wg.Done()
			<-start
			disableErr = adminSvc.DisableOidc(ctx)
		}()
		close(start)
		wg.Wait()

		if disableErr != nil {
			t.Fatalf("第 %d 轮停用失败: %v", i, disableErr)
		}
		events := r3107EventNames(t, svc)
		r3107AssertNoPostDisableResult(t, events, "bind")
		if bindErr != nil && !errors.Is(bindErr, ErrOidcDisabled) && !errors.Is(bindErr, ErrOidcFlowInvalid) {
			t.Fatalf("第 %d 轮绑定错误类型异常: %v", i, bindErr)
		}
	}
}

// TestR3107ConcurrentDisableVsTicketSerialized 会话 ticket 插入事件一旦出现，必须排在停用事件之前；
// 停用提交后不得再写入 ticket。
func TestR3107ConcurrentDisableVsTicketSerialized(t *testing.T) {
	for i := 0; i < 8; i++ {
		svc, users := newTestOidcServiceForConcurrency(t)
		adminSvc := r3107NewAdmin(t, svc)
		u, err := users.Register(ctx, "r3107-ticket-concurrent", "r3107-ticket-concurrent@example.com", "password123")
		if err != nil {
			t.Fatalf("创建会话用户失败: %v", err)
		}
		_, state, err := svc.StartFlow(ctx, "login", 0)
		if err != nil {
			t.Fatalf("StartFlow 失败: %v", err)
		}
		rec, err := svc.ConsumeState(ctx, state)
		if err != nil {
			t.Fatalf("ConsumeState 失败: %v", err)
		}
		r3107CreateEventTriggers(t, svc)

		var wg sync.WaitGroup
		start := make(chan struct{})
		var sessionErr, disableErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, _, sessionErr = svc.IssueLoginSessionForFlow(ctx, u.ID, u.CredentialVersion, rec.ConfigHash)
		}()
		go func() {
			defer wg.Done()
			<-start
			disableErr = adminSvc.DisableOidc(ctx)
		}()
		close(start)
		wg.Wait()

		if disableErr != nil {
			t.Fatalf("第 %d 轮停用失败: %v", i, disableErr)
		}
		events := r3107EventNames(t, svc)
		r3107AssertNoPostDisableResult(t, events, "ticket")
		if sessionErr != nil && !errors.Is(sessionErr, ErrOidcDisabled) && !errors.Is(sessionErr, ErrOidcFlowInvalid) {
			t.Fatalf("第 %d 轮会话签发错误类型异常: %v", i, sessionErr)
		}
	}
}
