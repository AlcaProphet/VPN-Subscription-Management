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
