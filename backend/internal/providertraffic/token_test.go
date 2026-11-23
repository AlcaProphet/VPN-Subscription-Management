// 本文件使用合成 JWT 样本，不包含真实凭据。
package providertraffic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// syntheticJWTValue 生成只用于解析到期时间的合成 JWT；签名段不参与校验。
func syntheticJWTValue(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("序列化合成 JWT 载荷失败: %v", err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".synthetic-signature"
}

func TestAccessTokenExpiry(t *testing.T) {
	base := time.Unix(1_800_000_000, 0).UTC()
	cases := []struct {
		name    string
		token   string
		wantOK  bool
		wantExp time.Time
	}{
		{"未来整数", syntheticJWTValue(t, map[string]any{"exp": base.Add(time.Hour).Unix()}), true, base.Add(time.Hour)},
		{"未来小数", syntheticJWTValue(t, map[string]any{"exp": float64(base.Unix()) + 0.5}), true, base.Add(500 * time.Millisecond)},
		{"已过期仍可解析", syntheticJWTValue(t, map[string]any{"exp": base.Add(-time.Minute).Unix()}), true, base.Add(-time.Minute)},
		{"零", syntheticJWTValue(t, map[string]any{"exp": 0}), false, time.Time{}},
		{"负数", syntheticJWTValue(t, map[string]any{"exp": -1}), false, time.Time{}},
		{"字符串", syntheticJWTValue(t, map[string]any{"exp": "1800003600"}), true, base.Add(time.Hour)},
		{"错误类型", syntheticJWTValue(t, map[string]any{"exp": "not-a-number"}), false, time.Time{}},
		{"布尔", syntheticJWTValue(t, map[string]any{"exp": true}), false, time.Time{}},
		{"null", syntheticJWTValue(t, map[string]any{"exp": nil}), false, time.Time{}},
		{"缺失", syntheticJWTValue(t, map[string]any{"sub": "synthetic"}), false, time.Time{}},
		{"过大", syntheticJWTValue(t, map[string]any{"exp": 1e13}), false, time.Time{}},
		{"溢出", "a.b.c", false, time.Time{}},
		{"段数不足", "only.two", false, time.Time{}},
		{"载荷非 Base64", "a.@@@.c", false, time.Time{}},
		{"载荷非对象", "a." + base64.RawURLEncoding.EncodeToString([]byte(`[]`)) + ".c", false, time.Time{}},
		{"非 JWT", "synthetic-opaque-token", false, time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := accessTokenExpiry(tc.token)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v，期望 %v", ok, tc.wantOK)
			}
			if ok && !got.Equal(tc.wantExp) {
				t.Fatalf("到期时间 = %v，期望 %v", got, tc.wantExp)
			}
		})
	}
}

func TestRotationCommitContextDetachesCancellationAndBoundsTime(t *testing.T) {
	type contextKey struct{}
	parent, cancelParent := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "synthetic"))
	cancelParent()

	commitCtx, cancel := rotationCommitContext(parent)
	defer cancel()

	if err := commitCtx.Err(); err != nil {
		t.Fatalf("提交上下文不应继承父上下文取消: %v", err)
	}
	if got := commitCtx.Value(contextKey{}); got != "synthetic" {
		t.Fatalf("提交上下文应保留父上下文值，实际 %v", got)
	}
	deadline, ok := commitCtx.Deadline()
	if !ok {
		t.Fatal("提交上下文必须有截止时间")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > rotationCommitTimeout+time.Second {
		t.Fatalf("提交上下文剩余时间 = %v，期望不超过 %v", remaining, rotationCommitTimeout)
	}
}
