// Package tasks 提供进程内全局长任务注册表（中性包，不依赖 server）。
package tasks

import (
	"encoding/json"
	"sync"
)

// Kind 任务类型枚举。
type Kind string

const (
	KindInstanceDelete Kind = "instance_delete"
	KindXrayInit       Kind = "xray_init"
	KindReconcileExec  Kind = "reconcile_exec"
	KindOffClear       Kind = "off_clear"
	KindImport         Kind = "import"
)

// Status 任务状态。
type Status string

const (
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// Task 任务视图。
type Task struct {
	ID     string          `json:"id"`
	Kind   Kind            `json:"kind"`
	Status Status          `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Registry 进程内任务注册表。业务包通过构造注入使用，禁止包级全局变量。
type Registry struct {
	mu     sync.Mutex
	nextID int64
	tasks  map[string]*Task
}

// NewRegistry 创建任务注册表。
func NewRegistry() *Registry {
	return &Registry{tasks: map[string]*Task{}}
}

// Register 登记一个新任务并返回任务 ID。
func (r *Registry) Register(kind Kind) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	id := formatID(r.nextID)
	r.tasks[id] = &Task{ID: id, Kind: kind, Status: StatusRunning}
	return id
}

// Succeed 写回成功终态。
func (r *Registry) Succeed(id string, result any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Status = StatusFailed
		t.Error = "序列化任务结果失败: " + err.Error()
		return
	}
	t.Status = StatusSucceeded
	t.Result = raw
	t.Error = ""
}

// Fail 写回失败终态。
func (r *Registry) Fail(id, msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[id]; ok {
		t.Status = StatusFailed
		t.Error = msg
	}
}

// Get 查询任务；未知 ID 或服务重启后（内存无记录）返回 failed「服务重启，任务中断」。
func (r *Registry) Get(id string) *Task {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[id]; ok {
		cp := *t
		return &cp
	}
	return &Task{ID: id, Status: StatusFailed, Error: "服务重启，任务中断"}
}

func formatID(n int64) string {
	if n == 0 {
		return "task-0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [24]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return "task-" + string(buf[i:])
}
