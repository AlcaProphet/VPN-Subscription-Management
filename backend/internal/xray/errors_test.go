package xray

import (
	"errors"
	"testing"
)

func TestIsAlreadyExists(t *testing.T) {
	if !IsAlreadyExists(errors.New("user already exists.")) {
		t.Fatal("应识别 already exists.")
	}
	if IsAlreadyExists(errors.New("not found.")) {
		t.Fatal("不应识别 not found.")
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(errors.New("user not found.")) {
		t.Fatal("应识别 not found.")
	}
	if IsNotFound(errors.New("already exists.")) {
		t.Fatal("不应识别 already exists.")
	}
}

func TestOpError(t *testing.T) {
	err := &OpError{Op: "AddUser", Instance: "127.0.0.1:10086", Tag: "inbound", Err: errors.New("boom")}
	if !IsOpError(err) {
		t.Fatal("IsOpError 应返回 true")
	}
	if err.Error() == "" {
		t.Fatal("OpError 应有消息")
	}
	if !errors.Is(err, err.Err) {
		t.Fatal("Unwrap 应可被 errors.Is 识别")
	}
}
