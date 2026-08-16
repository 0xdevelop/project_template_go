// Package api_auth_bridge 是 auth_type=bridge 形态的验签与本地令牌管理件：
// 令牌由外部 IdP 签发，本服务用 IdP 公钥本地验签（ES256，纯标准库实现，不引 JWT 库），
// 并在本地管理这批外部令牌（登出吊销，见 revocation.go）。
// 门禁分发在 api_auth_session.AuthenticateRequest，本包不感知协议层。
package api_auth_bridge

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/george012/gtbox/gtbox_time"
)

// VerifiedClaims 是验签结果的完整视图。IssuedAt 供本地吊销判定
// （令牌 iat 早于登出时刻即拒），ExpiresAt 供上层观测。
type VerifiedClaims struct {
	UserID    string
	Email     string
	TokenType string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type BridgeTokenVerifier struct {
	publicKey         *ecdsa.PublicKey
	allowedAlgorithms []string
	leeway            time.Duration
}

// NewBridgeTokenVerifier 加载 IdP 公钥（PKIX PEM，P-256）并构造验签器；
// 公钥路径错误/格式错误在此 fail-fast，不留到首个请求。
func NewBridgeTokenVerifier(publicKeyPath string, allowedAlgorithms []string, leeway time.Duration) (*BridgeTokenVerifier, error) {
	if strings.TrimSpace(publicKeyPath) == "" {
		return nil, fmt.Errorf("bridge public_key_path is required")
	}
	publicKey, err := loadBridgePublicKey(publicKeyPath)
	if err != nil {
		return nil, err
	}
	if leeway <= 0 {
		leeway = 60 * time.Second
	}
	return &BridgeTokenVerifier{
		publicKey:         publicKey,
		allowedAlgorithms: allowedAlgorithms,
		leeway:            leeway,
	}, nil
}

// VerifyToken 验签并返回完整 claims 视图。
func (v *BridgeTokenVerifier) VerifyToken(token string) (*VerifiedClaims, error) {
	if v == nil {
		return nil, fmt.Errorf("bridge token verifier is not configured")
	}
	return verifyBridgeJWT(token, v.publicKey, v.allowedAlgorithms, v.leeway, gtbox_time.NowUTC())
}

func loadBridgePublicKey(path string) (*ecdsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("decode PEM: no public key block found")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is %T, want *ecdsa.PublicKey", pub)
	}
	if ecdsaPub.Curve == nil || ecdsaPub.Params().Name != "P-256" {
		return nil, fmt.Errorf("public key curve must be P-256 for ES256")
	}
	return ecdsaPub, nil
}

func verifyBridgeJWT(token string, publicKey *ecdsa.PublicKey, allowedAlgorithms []string, leeway time.Duration, now time.Time) (*VerifiedClaims, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("bridge public key is required for token verification")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed JWT (%d parts)", len(parts))
	}

	headerBytes, err := decodeJWTPart(parts[0])
	if err != nil {
		return nil, fmt.Errorf("header base64 decode: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("header json unmarshal: %w", err)
	}
	if !jwtAlgAllowed(header.Alg, allowedAlgorithms) {
		return nil, fmt.Errorf("jwt alg %q is not allowed", header.Alg)
	}
	if header.Alg != "ES256" {
		return nil, fmt.Errorf("unsupported jwt alg %q", header.Alg)
	}
	gotSig, err := decodeJWTPart(parts[2])
	if err != nil {
		return nil, fmt.Errorf("signature base64 decode: %w", err)
	}
	if len(gotSig) != 64 {
		return nil, fmt.Errorf("invalid ES256 signature length %d", len(gotSig))
	}
	signingInput := parts[0] + "." + parts[1]
	digest := sha256.Sum256([]byte(signingInput))
	r := new(big.Int).SetBytes(gotSig[:32])
	s := new(big.Int).SetBytes(gotSig[32:])
	if !ecdsa.Verify(publicKey, digest[:], r, s) {
		return nil, fmt.Errorf("signature mismatch")
	}

	payload, err := decodeJWTPart(parts[1])
	if err != nil {
		return nil, fmt.Errorf("payload base64 decode: %w", err)
	}
	var claims struct {
		Sub   json.RawMessage `json:"sub"`
		Type  string          `json:"type"`
		Email string          `json:"email"`
		Exp   int64           `json:"exp"`
		Nbf   int64           `json:"nbf"`
		Iat   int64           `json:"iat"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("payload json unmarshal: %w", err)
	}
	if claims.Exp <= 0 {
		return nil, fmt.Errorf("exp is required")
	}
	if now.After(time.Unix(claims.Exp, 0).Add(leeway)) {
		return nil, fmt.Errorf("token expired")
	}
	if claims.Nbf > 0 && now.Add(leeway).Before(time.Unix(claims.Nbf, 0)) {
		return nil, fmt.Errorf("token not valid yet")
	}
	if claims.Iat > 0 && now.Add(leeway).Before(time.Unix(claims.Iat, 0)) {
		return nil, fmt.Errorf("token issued in the future")
	}
	userID, err := jwtSubjectString(claims.Sub)
	if err != nil {
		return nil, err
	}
	verified := &VerifiedClaims{UserID: userID, TokenType: claims.Type, Email: claims.Email}
	if claims.Iat > 0 {
		verified.IssuedAt = time.Unix(claims.Iat, 0).UTC()
	}
	verified.ExpiresAt = time.Unix(claims.Exp, 0).UTC()
	return verified, nil
}

func decodeJWTPart(s string) ([]byte, error) {
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err == nil {
		return data, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// jwtSubjectString 归一 sub：数字型转十进制字符串，字符串型去空白；
// 空值拒绝，长度上限 128（模板不强制数字 uid——那是具体 IdP 的实例约定）。
func jwtSubjectString(sub json.RawMessage) (string, error) {
	var id int64
	if err := json.Unmarshal(sub, &id); err == nil {
		if id <= 0 {
			return "", fmt.Errorf("sub must be positive")
		}
		return strconv.FormatInt(id, 10), nil
	}
	var subStr string
	if err := json.Unmarshal(sub, &subStr); err != nil {
		return "", fmt.Errorf("sub must be number or string")
	}
	subStr = strings.TrimSpace(subStr)
	if subStr == "" || len(subStr) > 128 {
		return "", fmt.Errorf("sub empty or too long")
	}
	return subStr, nil
}

func jwtAlgAllowed(alg string, allowed []string) bool {
	if alg == "" || alg == "none" {
		return false
	}
	if len(allowed) == 0 {
		allowed = []string{"ES256"}
	}
	for _, a := range allowed {
		if alg == strings.TrimSpace(a) {
			return true
		}
	}
	return false
}
