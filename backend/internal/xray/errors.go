package xray

import (
	"errors"
	"fmt"
	"strings"
)

// ErrQuotaExceeded 表示账号已超限，AddUser 类操作应跳过（对账时计为 skipped）。
var ErrQuotaExceeded = errors.New("已超限，请先重置配额")

// OpError 携带操作与实例上下文，便于日志定位。
type OpError struct {
	Op       string
	Instance string
	Tag      string
	Err      error
}

func (e *OpError) Error() string {
	if e.Tag != "" {
		return fmt.Sprintf("xray %s 失败 (instance=%s tag=%s): %v", e.Op, e.Instance, e.Tag, e.Err)
	}
	return fmt.Sprintf("xray %s 失败 (instance=%s): %v", e.Op, e.Instance, e.Err)
}

func (e *OpError) Unwrap() error { return e.Err }

// IsAlreadyExists 判断是否为 Xray 幂等「已存在」错误。
func IsAlreadyExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already exists.")
}

// IsNotFound 判断是否为 Xray 幂等「不存在」错误。
func IsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found.")
}

// IsOpError 便于测试/调用方判断包装类型。
func IsOpError(err error) bool {
	var oe *OpError
	return errors.As(err, &oe)
}
