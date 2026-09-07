package identity

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLDAPProvider_DefaultDial_ConnectionRefused 走 NewLDAPProvider 内置的
// 默认 dial 闭包（ldap.DialURL），连向必然拒绝的本地端口，覆盖其错误返回路径。
func TestLDAPProvider_DefaultDial_ConnectionRefused(t *testing.T) {
	p := NewLDAPProvider(LDAPConfig{Addr: "ldap://127.0.0.1:1"})

	ident, err := p.Authenticate(context.Background(), "alice", "s3cret")
	if err == nil {
		t.Fatalf("expected dial error, got identity %+v", ident)
	}
	if !strings.Contains(err.Error(), "ldap dial") {
		t.Fatalf("error should wrap dial failure, got: %v", err)
	}
}

// TestLDAPProvider_DefaultDial_Success 覆盖默认 dial 闭包的成功返回路径：
// TCP 连接本地 listener 成功后，由服务端立即断开使后续 Bind 失败。
func TestLDAPProvider_DefaultDial_Success(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// 接受后立刻断开，客户端 Bind 将得到连接错误。
			_ = conn.Close()
		}
	}()

	p := NewLDAPProvider(LDAPConfig{
		Addr:               "ldap://" + listener.Addr().String(),
		UserDNTemplate:     "uid=%s,ou=people,dc=example,dc=com",
		InsecureSkipVerify: true,
	})

	// dial 本身成功；Bind 因服务端断开失败 => ErrInvalidCredentials。
	_, err = p.Authenticate(context.Background(), "alice", "s3cret")
	if err == nil {
		t.Fatal("expected bind failure after successful dial")
	}
}

// TestOIDCProvider_DiscoveryFailure 覆盖 NewOIDCProvider 中
// 发现端点请求失败的分支（oidc.go:45）。
func TestOIDCProvider_DiscoveryFailure(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	p, err := NewOIDCProvider(context.Background(), OIDCConfig{Issuer: srv.URL})
	if err == nil {
		t.Fatal("expected discovery error")
	}
	if p != nil {
		t.Fatalf("expected nil provider on error, got %+v", p)
	}
	if !strings.Contains(err.Error(), "oidc discovery") {
		t.Fatalf("error should wrap discovery failure, got: %v", err)
	}
}

// TestOIDCProvider_ExchangeClaimsUnmarshalFailure 覆盖 Exchange 中
// idToken.Claims 反序列化失败的分支（oidc.go:95）。
//
// 思路：Verify 阶段 go-oidc 先把 payload 反序列化进内部 struct（未知字段
// "big" 被跳过、不校验数值范围），验签与 issuer/aud/exp 校验全部通过；
// 随后 Claims 解进 map[string]interface{} 时 "big":1e999 因超出 float64
// 表示范围而失败。
func TestOIDCProvider_ExchangeClaimsUnmarshalFailure(t *testing.T) {
	_, cfg, sign, _ := newSignedIDP(t)
	sign(map[string]interface{}{
		"aud": "croupier",
		"sub": "u1",
		"exp": json.RawMessage("9999999999"),
		"iat": json.RawMessage("0"),
		// go-oidc 的内部 struct 跳过未知字段（不触发数值范围检查），
		// 而 map[string]interface{} 解码 float64 时溢出报错。
		"big": json.RawMessage("1e999"),
	})

	p := mustProvider(t, cfg)

	ident, err := p.Exchange(context.Background(), "code")
	if err == nil {
		t.Fatalf("expected claims unmarshal error, got identity %+v", ident)
	}
	if !strings.Contains(err.Error(), "oidc parse claims") {
		t.Fatalf("error should wrap claims parse failure, got: %v", err)
	}
}
