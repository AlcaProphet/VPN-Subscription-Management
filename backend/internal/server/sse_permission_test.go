package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/log"
)

type sequenceUserSource struct {
	mu    sync.Mutex
	snaps []*auth.UserSnapshot
	calls int
}

func (s *sequenceUserSource) SnapshotByID(_ context.Context, _ int64) (*auth.UserSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.snaps) == 0 {
		return nil, nil
	}
	idx := s.calls
	if idx >= len(s.snaps) {
		idx = len(s.snaps) - 1
	}
	s.calls++
	return s.snaps[idx], nil
}

func (s *sequenceUserSource) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func streamPermissionServer(t *testing.T, source *sequenceUserSource, interval time.Duration) (*httptest.Server, *log.StreamService) {
	t.Helper()
	streamSvc := log.NewStreamService(log.NewRingBuffer(), log.New("error", "console"))
	h := &LogHandler{streamSvc: streamSvc, users: source, permissionInterval: interval}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/stream", func(c *gin.Context) {
		c.Set(auth.CtxUserID, int64(7))
		h.stream(c)
	})
	ts := httptest.NewServer(engine)
	t.Cleanup(ts.Close)
	return ts, streamSvc
}

// TestStreamPermissionChangeCloses 流内每 15 秒（测试缩短为 20ms）重查；权限变化即关闭并清理订阅。
func TestStreamPermissionChangeCloses(t *testing.T) {
	admin := &auth.UserSnapshot{ID: 7, Role: "admin", Status: "active"}
	cases := []struct {
		name string
		next *auth.UserSnapshot
	}{
		{name: "admin-to-user", next: &auth.UserSnapshot{ID: 7, Role: "user", Status: "active"}},
		{name: "active-to-disabled", next: &auth.UserSnapshot{ID: 7, Role: "admin", Status: "disabled"}},
		{name: "deleted", next: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := &sequenceUserSource{snaps: []*auth.UserSnapshot{admin, tc.next}}
			ts, streamSvc := streamPermissionServer(t, source, 20*time.Millisecond)
			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get(ts.URL + "/stream")
			if err != nil {
				t.Fatalf("建立流失败: %v", err)
			}
			_, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				t.Fatalf("权限变化后流应正常结束: %v", readErr)
			}
			if source.callCount() < 2 {
				t.Fatalf("应完成至少两次权限重查，实际 %d", source.callCount())
			}
			ch, _, ok := streamSvc.Subscribe()
			if !ok {
				t.Fatal("流关闭后连接数未释放")
			}
			streamSvc.Unsubscribe(ch)
		})
	}
}

// TestStreamPermissionStableAdminConnection 权限不变时流保持打开，直到客户端取消。
func TestStreamPermissionStableAdminConnection(t *testing.T) {
	admin := &auth.UserSnapshot{ID: 7, Role: "admin", Status: "active"}
	source := &sequenceUserSource{snaps: []*auth.UserSnapshot{admin}}
	ts, _ := streamPermissionServer(t, source, 15*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("建立流失败: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if source.callCount() < 3 {
		t.Fatalf("权限不变时应持续重查，实际 %d", source.callCount())
	}
}
