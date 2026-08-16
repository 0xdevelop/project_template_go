package api_auth_bridge

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testKeyAndPEM(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pemPath := filepath.Join(t.TempDir(), "idp-public.pem")
	pemData := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	if err := os.WriteFile(pemPath, pemData, 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}
	return key, pemPath
}

func signTestJWT(t *testing.T, key *ecdsa.PrivateKey, alg string, claims map[string]any) string {
	t.Helper()
	headerJSON, _ := json.Marshal(map[string]string{"alg": alg, "typ": "JWT"})
	claimsJSON, _ := json.Marshal(claims)
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(signingInput))
	var signature []byte
	switch alg {
	case "ES256":
		r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		signature = make([]byte, 64)
		r.FillBytes(signature[:32])
		s.FillBytes(signature[32:])
	case "none":
		signature = nil
	default: // HS256 混淆
		mac := hmac.New(sha256.New, []byte("shared-secret"))
		mac.Write([]byte(signingInput))
		signature = mac.Sum(nil)
	}
	_ = big.NewInt(0)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func TestBridgeVerifier_AcceptsValidTokenAndReturnsClaims(t *testing.T) {
	key, pemPath := testKeyAndPEM(t)
	verifier, err := NewBridgeTokenVerifier(pemPath, nil, 0)
	if err != nil {
		t.Fatalf("NewBridgeTokenVerifier: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	token := signTestJWT(t, key, "ES256", map[string]any{
		"sub": "42", "type": "access", "email": "someone@example.com",
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	claims, err := verifier.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if claims.UserID != "42" || claims.TokenType != "access" ||
		claims.Email != "someone@example.com" ||
		!claims.IssuedAt.Equal(now.Add(-time.Minute)) ||
		!claims.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestBridgeVerifier_RejectsDowngradeAndBadSignature(t *testing.T) {
	key, pemPath := testKeyAndPEM(t)
	verifier, err := NewBridgeTokenVerifier(pemPath, nil, 0)
	if err != nil {
		t.Fatalf("NewBridgeTokenVerifier: %v", err)
	}
	exp := time.Now().UTC().Add(time.Hour).Unix()
	for name, token := range map[string]string{
		"alg=none": signTestJWT(t, key, "none", map[string]any{"sub": "42", "type": "access", "exp": exp}),
		"HS256":    signTestJWT(t, key, "HS256", map[string]any{"sub": "42", "type": "access", "exp": exp}),
		"tampered": signTestJWT(t, key, "ES256", map[string]any{"sub": "42", "type": "access", "exp": exp}) + "x",
	} {
		if _, err := verifier.VerifyToken(token); err == nil {
			t.Fatalf("%s 必须被拒绝", name)
		}
	}
}

func TestBridgeVerifier_LeewayBoundary(t *testing.T) {
	key, pemPath := testKeyAndPEM(t)
	verifier, err := NewBridgeTokenVerifier(pemPath, nil, 60*time.Second)
	if err != nil {
		t.Fatalf("NewBridgeTokenVerifier: %v", err)
	}
	now := time.Now().UTC()
	withinLeeway := signTestJWT(t, key, "ES256", map[string]any{
		"sub": "42", "type": "access", "exp": now.Add(-30 * time.Second).Unix(),
	})
	if _, err := verifier.VerifyToken(withinLeeway); err != nil {
		t.Fatalf("leeway 内过期应放行: %v", err)
	}
	beyondLeeway := signTestJWT(t, key, "ES256", map[string]any{
		"sub": "42", "type": "access", "exp": now.Add(-90 * time.Second).Unix(),
	})
	if _, err := verifier.VerifyToken(beyondLeeway); err == nil {
		t.Fatal("leeway 外过期必须拒绝")
	}
}

func TestBridgeVerifier_SubNumberOrString(t *testing.T) {
	key, pemPath := testKeyAndPEM(t)
	verifier, err := NewBridgeTokenVerifier(pemPath, nil, 0)
	if err != nil {
		t.Fatalf("NewBridgeTokenVerifier: %v", err)
	}
	exp := time.Now().UTC().Add(time.Hour).Unix()

	numeric := signTestJWT(t, key, "ES256", map[string]any{"sub": 90001, "type": "access", "exp": exp})
	claims, err := verifier.VerifyToken(numeric)
	if err != nil || claims.UserID != "90001" {
		t.Fatalf("数字 sub 应归一为十进制字符串: %v %#v", err, claims)
	}

	stringSub := signTestJWT(t, key, "ES256", map[string]any{"sub": "user-abc", "type": "access", "exp": exp})
	claims, err = verifier.VerifyToken(stringSub)
	if err != nil || claims.UserID != "user-abc" {
		t.Fatalf("字符串 sub 应通过(模板不强制数字 uid): %v %#v", err, claims)
	}

	empty := signTestJWT(t, key, "ES256", map[string]any{"sub": "  ", "type": "access", "exp": exp})
	if _, err := verifier.VerifyToken(empty); err == nil {
		t.Fatal("空 sub 必须拒绝")
	}
}

func TestMemoryRevocationStore_Semantics(t *testing.T) {
	previous := currentRevocation
	t.Cleanup(func() { currentRevocation = previous })
	store := NewMemoryRevocationStore()
	SetupRevocation(store)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := RevokeUserTokens(ctx, "42", time.Hour); err != nil {
		t.Fatalf("RevokeUserTokens: %v", err)
	}
	if revoked, err := Revoked(ctx, "42", now.Add(-time.Minute)); err != nil || !revoked {
		t.Fatalf("登出前签发的令牌应判已吊销: %v %v", revoked, err)
	}
	if revoked, err := Revoked(ctx, "42", now.Add(time.Minute)); err != nil || revoked {
		t.Fatalf("登出后新签令牌应放行: %v %v", revoked, err)
	}
	if revoked, err := Revoked(ctx, "42", time.Time{}); err != nil || !revoked {
		t.Fatalf("iat 缺席且有登出记录应按已吊销: %v %v", revoked, err)
	}
	if revoked, err := Revoked(ctx, "no-such-user", now); err != nil || revoked {
		t.Fatalf("无记录用户应放行: %v %v", revoked, err)
	}
}
