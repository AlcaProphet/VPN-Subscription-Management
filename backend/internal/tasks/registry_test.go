package tasks

import "testing"

func TestRegistryLifecycle(t *testing.T) {
	r := NewRegistry()
	id := r.Register(KindInstanceDelete)
	if id == "" {
		t.Fatal("任务 ID 不能为空")
	}
	got := r.Get(id)
	if got.Status != StatusRunning || got.Kind != KindInstanceDelete {
		t.Fatalf("任务初始状态异常: %+v", got)
	}
	r.Succeed(id, map[string]any{"ok": true})
	got = r.Get(id)
	if got.Status != StatusSucceeded || got.Result == nil {
		t.Fatalf("成功终态异常: %+v", got)
	}
}

func TestRegistryUnknownReturnsFailed(t *testing.T) {
	r := NewRegistry()
	got := r.Get("missing")
	if got.Status != StatusFailed || got.Error == "" {
		t.Fatalf("未知任务应返回 failed: %+v", got)
	}
}

func TestRegistryFail(t *testing.T) {
	r := NewRegistry()
	id := r.Register(KindXrayInit)
	r.Fail(id, "boom")
	got := r.Get(id)
	if got.Status != StatusFailed || got.Error != "boom" {
		t.Fatalf("失败终态异常: %+v", got)
	}
}
