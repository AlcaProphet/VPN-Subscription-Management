package node

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
)

func TestImportURIsBatch(t *testing.T) {
	svc, st, _ := newTestService(t)
	one := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:password1")) + "@a.example.com:8388#Alpha"
	two := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:password2")) + "@b.example.com:8388#Beta"
	text := "ss://" + one + "\nss://" + two + "\nnot-a-uri"
	results, err := svc.ImportURIs(context.Background(), text)
	if err != nil {
		t.Fatalf("ImportURIs 失败: %v", err)
	}
	var okCount, skipCount int
	for _, r := range results {
		if r.OK {
			okCount++
		} else {
			skipCount++
		}
	}
	if okCount != 2 || skipCount != 1 {
		t.Fatalf("导入回执异常: %+v", results)
	}
	var count int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes WHERE source='manual'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("应导入 2 个节点，实际 %d", count)
	}
}

func TestImportURIsDuplicateSkipped(t *testing.T) {
	svc, st, _ := newTestService(t)
	one := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:password1")) + "@a.example.com:8388#Same"
	text := "ss://" + one + "\nss://" + one
	results, err := svc.ImportURIs(context.Background(), text)
	if err != nil {
		t.Fatalf("ImportURIs 失败: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("应有 2 条回执，实际 %d: %+v", len(results), results)
	}
	var count int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes WHERE source='manual'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("重复名称应只导入 1 个，实际 %d", count)
	}
	if !strings.Contains(results[1].Reason, "重复") {
		t.Fatalf("第二条应回执重复: %+v", results[1])
	}
}
