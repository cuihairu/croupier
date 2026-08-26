package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newFakeIDP 启动一个最小 OIDC 发现服务器（无签名校验，仅覆盖装配与授权 URL）。
func newFakeIDP(t *testing.T, tokenHandler http.HandlerFunc) (*httptest.Server, OIDCConfig) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	discovery := map[string]interface{}{
		"issuer":                                srv.URL,
		"authorization_endpoint":                srv.URL + "/auth",
		"token_endpoint":                        srv.URL + "/token",
		"jwks_uri":                              srv.URL + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(discovery)
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})
	if tokenHandler != nil {
		mux.HandleFunc("/token", tokenHandler)
	}

	cfg := OIDCConfig{
		Issuer:       srv.URL,
		ClientID:     "croupier",
		ClientSecret: "s3cret",
		RedirectURL:  "http://localhost:18780/api/auth/oidc/callback",
	}
	return srv, cfg
}

func TestOIDCProvider_DiscoveryAndAuthCodeURL(t *testing.T) {
	_, cfg := newFakeIDP(t, nil)

	p, err := NewOIDCProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Kind() != KindOIDC {
		t.Fatalf("kind mismatch: %s", p.Kind())
	}

	url := p.AuthCodeURL("st4te")
	if !strings.Contains(url, cfg.Issuer+"/auth") {
		t.Fatalf("auth url should hit fake idp: %s", url)
	}
	if !strings.Contains(url, "client_id=croupier") ||
		!strings.Contains(url, "redirect_uri=http%3A%2F%2Flocalhost%3A18780%2Fapi%2Fauth%2Foidc%2Fcallback") ||
		!strings.Contains(url, "state=st4te") ||
		!strings.Contains(url, "scope=openid") {
		t.Fatalf("auth url missing expected params: %s", url)
	}
}

func TestOIDCProvider_RequiresIssuer(t *testing.T) {
	if _, err := NewOIDCProvider(context.Background(), OIDCConfig{}); err == nil {
		t.Fatal("expected error for empty issuer")
	}
}

func TestOIDCProvider_ExchangeMissingIDToken(t *testing.T) {
	_, cfg := newFakeIDP(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 合法 token 响应但缺 id_token。
		_, _ = w.Write([]byte(`{"access_token":"at","token_type":"Bearer","expires_in":3600}`))
	})
	p, err := NewOIDCProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := p.Exchange(context.Background(), "code"); err == nil || !strings.Contains(err.Error(), "id_token") {
		t.Fatalf("expected missing id_token error, got %v", err)
	}
}

func TestOIDCProvider_ExchangeBadCode(t *testing.T) {
	_, cfg := newFakeIDP(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	})
	p, err := NewOIDCProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := p.Exchange(context.Background(), "bad"); err == nil {
		t.Fatal("expected exchange error for invalid grant")
	}
}
