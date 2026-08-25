package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/golang-jwt/jwt/v5"
)

// jwk 表示 OIDC JWKS 中的单个 JSON Web Key（仅实现验签所需字段）。
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// jwkSet 表示 OIDC JWKS 文档。
type jwkSet struct {
	Keys []jwk `json:"keys"`
}

var allowedSigningAlgs = []string{
	"RS256", "RS384", "RS512",
	"ES256", "ES384", "ES512",
	"PS256", "PS384", "PS512",
	"EdDSA",
}

// verifyIDToken 使用 JWKS 验签 id_token，并校验 iss/aud/exp、nonce、azp。
// 返回验签后的 claims 与原始 JSON 字符串，供上层提取业务字段。
// 首次验签失败时会刷新 JWKS 缓存重试一次，以支持密钥轮换。
func (s *Service) verifyIDToken(ctx context.Context, p *Params, disc *Discovery, rawIDToken, nonce string) (jwt.MapClaims, string, error) {
	if disc.JWKSURI == "" {
		return nil, "", errors.New("发现文档缺少 jwks_uri")
	}
	var outClaims jwt.MapClaims
	var outRaw string
	parse := func() error {
		set, err := s.getJWKS(ctx, disc.JWKSURI)
		if err != nil {
			return err
		}
		claims := jwt.MapClaims{}
		keyFunc := func(t *jwt.Token) (any, error) {
			if !containsAlg(allowedSigningAlgs, t.Method.Alg()) {
				return nil, fmt.Errorf("不支持的签名算法: %s", t.Method.Alg())
			}
			kid, _ := t.Header["kid"].(string)
			return findPublicKey(set, kid, t.Method.Alg())
		}
		parser := jwt.NewParser(
			jwt.WithValidMethods(allowedSigningAlgs),
			jwt.WithIssuer(disc.Issuer),
			jwt.WithAudience(p.ClientID),
			jwt.WithExpirationRequired(),
		)
		token, err := parser.ParseWithClaims(rawIDToken, claims, keyFunc)
		if err != nil || !token.Valid {
			return errors.New("id_token 验签或声明校验失败")
		}
		if got, _ := claims["nonce"].(string); got != nonce {
			return errors.New("id_token nonce 不匹配")
		}
		if azp, _ := claims["azp"].(string); azp != "" && azp != p.ClientID {
			return errors.New("id_token azp 不匹配")
		}
		sub, _ := claims["sub"].(string)
		if sub == "" {
			return errors.New("id_token 缺少 sub")
		}
		raw, err := json.Marshal(claims)
		if err != nil {
			return fmt.Errorf("序列化 id_token claims 失败: %w", err)
		}
		outClaims = claims
		outRaw = string(raw)
		return nil
	}
	if err := parse(); err != nil {
		// 可能是 JWKS 已轮换，刷新缓存后重试一次。
		s.refreshJWKS(disc.JWKSURI)
		if err2 := parse(); err2 != nil {
			return nil, "", err2
		}
	}
	return outClaims, outRaw, nil
}

func containsAlg(allowed []string, alg string) bool {
	for _, a := range allowed {
		if a == alg {
			return true
		}
	}
	return false
}

// findPublicKey 从 JWKS 中找到与 kid/alg 匹配的公钥。
func findPublicKey(set *jwkSet, kid, alg string) (any, error) {
	var matches []jwk
	for _, k := range set.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		if kid != "" && k.Kid != kid {
			continue
		}
		if k.Alg != "" && k.Alg != alg {
			continue
		}
		matches = append(matches, k)
	}
	if len(matches) == 0 {
		return nil, errors.New("未找到匹配的 JWKS 公钥")
	}
	if len(matches) > 1 {
		return nil, errors.New("JWKS 存在多个匹配公钥")
	}
	return publicKeyFromJWK(matches[0])
}

func publicKeyFromJWK(k jwk) (any, error) {
	switch k.Kty {
	case "RSA":
		n, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return nil, fmt.Errorf("RSA n 解码失败: %w", err)
		}
		e, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return nil, fmt.Errorf("RSA e 解码失败: %w", err)
		}
		eInt := new(big.Int).SetBytes(e)
		if !eInt.IsInt64() {
			return nil, errors.New("RSA e 超出 int 范围")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(eInt.Int64())}, nil
	case "EC":
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, fmt.Errorf("EC x 解码失败: %w", err)
		}
		y, err := base64.RawURLEncoding.DecodeString(k.Y)
		if err != nil {
			return nil, fmt.Errorf("EC y 解码失败: %w", err)
		}
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("不支持的 EC 曲线: %s", k.Crv)
		}
		return &ecdsa.PublicKey{Curve: curve, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
	case "OKP":
		if k.Crv != "Ed25519" {
			return nil, fmt.Errorf("不支持的 OKP 曲线: %s", k.Crv)
		}
		x, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, fmt.Errorf("Ed25519 x 解码失败: %w", err)
		}
		if len(x) != ed25519.PublicKeySize {
			return nil, errors.New("Ed25519 公钥长度非法")
		}
		return ed25519.PublicKey(x), nil
	default:
		return nil, fmt.Errorf("不支持的 JWK kty: %s", k.Kty)
	}
}
