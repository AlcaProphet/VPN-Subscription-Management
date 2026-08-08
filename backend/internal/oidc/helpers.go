package oidc

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// randRead 加密安全随机数填充
func randRead(b []byte) (int, error) { return rand.Read(b) }

// sha256Sum SHA-256 摘要
func sha256Sum(b []byte) [32]byte { return sha256.Sum256(b) }

// stringsNewReader 字符串读取器
func stringsNewReader(s string) *strings.Reader { return strings.NewReader(s) }

// stringsSplitN 按分隔符拆分
func stringsSplitN(s, sep string, n int) []string { return strings.SplitN(s, sep, n) }

// decodeJWTPayload 解析 JWT payload 段（base64url JSON，不验签）
func decodeJWTPayload(token string) ([]byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("id_token 格式无效")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("id_token payload 解码失败")
	}
	if !json.Valid(payload) {
		return nil, errors.New("id_token payload 非 JSON")
	}
	return payload, nil
}

// claimPathValues 从 claims JSON 中按点分路径取值（如 realm_access.roles）；
// 支持字符串数组与单字符串；路径不存在返回 false
func claimPathValues(rawClaims, path string) ([]string, bool) {
	if rawClaims == "" || path == "" {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rawClaims), &m); err != nil {
		return nil, false
	}
	var cur any = m
	for _, p := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = obj[p]
		if !ok {
			return nil, false
		}
	}
	switch v := cur.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out, len(out) > 0
	case string:
		return []string{v}, true
	}
	return nil, false
}

// intersectAny 两个字符串集合是否有交集（白名单命中判定）
func intersectAny(a, b []string) bool {
	set := make(map[string]bool, len(b))
	for _, v := range b {
		set[v] = true
	}
	for _, v := range a {
		if set[v] {
			return true
		}
	}
	return false
}
