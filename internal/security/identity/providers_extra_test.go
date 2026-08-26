package identity

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// ---------- LDAP 边界路径 ----------

// erroringConn 按调用点返回错误，覆盖拨号后各失败分支。
type erroringConn struct {
	startTLSErr  error
	bindErr      error
	searchErr    error
	entries      []*ldap.Entry
	gotStartTLS  bool
	gotBindCalls int
	gotClosed    bool
}

func (c *erroringConn) Bind(username, password string) error {
	c.gotBindCalls++
	if c.bindErr != nil {
		return c.bindErr
	}
	return nil
}

func (c *erroringConn) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	if c.searchErr != nil {
		return nil, c.searchErr
	}
	return &ldap.SearchResult{Entries: c.entries}, nil
}

func (c *erroringConn) StartTLS(config *tls.Config) error {
	c.gotStartTLS = true
	if c.startTLSErr != nil {
		return c.startTLSErr
	}
	return nil
}

func (c *erroringConn) Close() error { c.gotClosed = true; return nil }

func newLDAPWithConn(cfg LDAPConfig, conn ldapConn) *LDAPProvider {
	p := NewLDAPProvider(cfg)
	p.dial = func(addr string) (ldapConn, error) { return conn, nil }
	return p
}

var ldapBaseCfg = LDAPConfig{Addr: "ldap://x:389", BaseDN: "dc=e,dc=c", BindDN: "svc"}

func TestLDAPProvider_Kind(t *testing.T) {
	if NewLDAPProvider(LDAPConfig{}).Kind() != KindLDAP {
		t.Fatal("kind mismatch")
	}
	if NewLocalProvider(nil).Kind() != KindLocal {
		t.Fatal("local kind mismatch")
	}
}

func TestLDAPProvider_DialError(t *testing.T) {
	p := NewLDAPProvider(ldapBaseCfg)
	p.dial = func(addr string) (ldapConn, error) { return nil, errors.New("dial tcp: refused") }

	_, err := p.Authenticate(context.Background(), "alice", "pw")
	if err == nil || !strings.Contains(err.Error(), "ldap dial") {
		t.Fatalf("want dial error, got %v", err)
	}
}

func TestLDAPProvider_StartTLSUpgrade(t *testing.T) {
	conn := &erroringConn{
		entries: []*ldap.Entry{
			ldap.NewEntry("uid=alice,ou=p,dc=e,dc=c", map[string][]string{"cn": {"Alice"}}),
		},
	}
	cfg := ldapBaseCfg
	cfg.StartTLS = true
	p := newLDAPWithConn(cfg, conn)

	ident, err := p.Authenticate(context.Background(), "alice", "pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !conn.gotStartTLS {
		t.Fatal("expected StartTLS upgrade on plain ldap:// addr")
	}
	if ident.Username != "alice" || ident.Nickname != "Alice" {
		t.Fatalf("identity mismatch: %+v", ident)
	}
}

func TestLDAPProvider_StartTLSOnLDAPSIgnored(t *testing.T) {
	conn := &erroringConn{
		entries: []*ldap.Entry{ldap.NewEntry("uid=a,dc=e,dc=c", nil)},
	}
	cfg := ldapBaseCfg
	cfg.Addr = "ldaps://x:636"
	cfg.StartTLS = true
	p := newLDAPWithConn(cfg, conn)

	if _, err := p.Authenticate(context.Background(), "a", "pw"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.gotStartTLS {
		t.Fatal("ldaps:// must not upgrade StartTLS")
	}
}

func TestLDAPProvider_StartTLSError(t *testing.T) {
	conn := &erroringConn{startTLSErr: errors.New("tls handshake failed")}
	cfg := ldapBaseCfg
	cfg.StartTLS = true
	p := newLDAPWithConn(cfg, conn)

	_, err := p.Authenticate(context.Background(), "a", "pw")
	if err == nil || !strings.Contains(err.Error(), "starttls") {
		t.Fatalf("want starttls error, got %v", err)
	}
	if !conn.gotClosed {
		t.Fatal("connection must be closed on failure")
	}
}

func TestLDAPProvider_ServiceBindError(t *testing.T) {
	conn := &erroringConn{bindErr: errors.New("invalid service credentials")}
	p := newLDAPWithConn(ldapBaseCfg, conn)

	_, err := p.Authenticate(context.Background(), "a", "pw")
	if err == nil || !strings.Contains(err.Error(), "service bind") {
		t.Fatalf("want service bind error, got %v", err)
	}
}

func TestLDAPProvider_SearchError(t *testing.T) {
	conn := &erroringConn{searchErr: errors.New("size limit exceeded")}
	p := newLDAPWithConn(ldapBaseCfg, conn)

	_, err := p.Authenticate(context.Background(), "a", "pw")
	if err == nil || !strings.Contains(err.Error(), "ldap search") {
		t.Fatalf("want search error, got %v", err)
	}
}

func TestLDAPProvider_MultipleEntries(t *testing.T) {
	conn := &erroringConn{
		entries: []*ldap.Entry{
			ldap.NewEntry("uid=a,ou=1,dc=e,dc=c", nil),
			ldap.NewEntry("uid=a,ou=2,dc=e,dc=c", nil),
		},
	}
	p := newLDAPWithConn(ldapBaseCfg, conn)

	_, err := p.Authenticate(context.Background(), "a", "pw")
	if err == nil || !strings.Contains(err.Error(), "multiple entries") {
		t.Fatalf("want multiple entries error, got %v", err)
	}
}

func TestLDAPProvider_AnonymousSearch(t *testing.T) {
	conn := &erroringConn{
		entries: []*ldap.Entry{
			ldap.NewEntry("uid=anon,dc=e,dc=c", map[string][]string{"mail": {"anon@example.com"}}),
		},
	}
	cfg := ldapBaseCfg
	cfg.BindDN = ""
	p := newLDAPWithConn(cfg, conn)

	ident, err := p.Authenticate(context.Background(), "anon", "pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 无 BindDN：匿名搜索后仅一次用户 DN 绑定。
	if conn.gotBindCalls != 1 {
		t.Fatalf("expected 1 bind call (user only), got %d", conn.gotBindCalls)
	}
	if ident.Nickname != "anon" || ident.Email != "anon@example.com" {
		t.Fatalf("identity mismatch: %+v", ident)
	}
}

func TestLDAPProvider_EmptyCredentials(t *testing.T) {
	p := NewLDAPProvider(ldapBaseCfg)
	p.dial = func(addr string) (ldapConn, error) {
		t.Fatal("must not dial for empty credentials")
		return nil, nil
	}

	if _, err := p.Authenticate(context.Background(), "", "pw"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
	if _, err := p.Authenticate(context.Background(), "alice", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLDAPProvider_ContextCancelled(t *testing.T) {
	p := NewLDAPProvider(ldapBaseCfg)
	p.dial = func(addr string) (ldapConn, error) {
		t.Fatal("must not dial after ctx cancel")
		return nil, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.Authenticate(ctx, "alice", "pw")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

// ---------- OIDC Exchange 完整验签流程 ----------

// signRS256 手工构造 RS256 JWT。
func signRS256(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]interface{}) string {
	t.Helper()
	header := map[string]interface{}{"alg": "RS256", "typ": "JWT", "kid": kid}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	signingInput := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func jwksJSON(t *testing.T, key *rsa.PublicKey, kid string) string {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
	return `{"keys":[{"kty":"RSA","kid":"` + kid + `","use":"sig","alg":"RS256","n":"` + n + `","e":"` + e + `"}]}`
}

// newSignedIDP 启动带 RSA 验签能力的假身份源；sign 用服务器私钥签发
// id_token 并更新 /token 响应（issuer 自动填服务器地址），setToken 直接
// 注入裸 token（供篡改场景）。
func newSignedIDP(t *testing.T) (*httptest.Server, OIDCConfig, func(map[string]interface{}) string, func(string)) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	currentToken := "placeholder"
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issuer": "` + srv.URL + `",
			"authorization_endpoint": "` + srv.URL + `/auth",
			"token_endpoint": "` + srv.URL + `/token",
			"jwks_uri": "` + srv.URL + `/jwks",
			"response_types_supported": ["code"],
			"subject_types_supported": ["public"],
			"id_token_signing_alg_values_supported": ["RS256"]
		}`))
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jwksJSON(t, &key.PublicKey, "k1")))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","token_type":"Bearer","expires_in":3600,"id_token":"` + currentToken + `"}`))
	})

	sign := func(claims map[string]interface{}) string {
		claims["iss"] = srv.URL
		currentToken = signRS256(t, key, "k1", claims)
		return currentToken
	}
	setToken := func(tok string) { currentToken = tok }

	cfg := OIDCConfig{
		Issuer:       srv.URL,
		ClientID:     "croupier",
		ClientSecret: "cs",
		RedirectURL:  "http://localhost:18780/api/auth/oidc/callback",
	}
	return srv, cfg, sign, setToken
}

func mustProvider(t *testing.T, cfg OIDCConfig) *OIDCProvider {
	t.Helper()
	p, err := NewOIDCProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewOIDCProvider: %v", err)
	}
	return p
}

func TestOIDCProvider_Exchange_HappyPath(t *testing.T) {
	_, cfg, sign, _ := newSignedIDP(t)
	sign(map[string]interface{}{
		"aud":                "croupier",
		"sub":                "user-123",
		"preferred_username": "alice",
		"name":               "Alice Doe",
		"email":              "alice@example.com",
		"exp":                time.Now().Add(time.Hour).Unix(),
	})
	p := mustProvider(t, cfg)

	ident, err := p.Exchange(context.Background(), "code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Provider != KindOIDC || ident.Username != "alice" ||
		ident.Nickname != "Alice Doe" || ident.Email != "alice@example.com" {
		t.Fatalf("identity mismatch: %+v", ident)
	}
}

func TestOIDCProvider_Exchange_FallbackClaims(t *testing.T) {
	_, cfg, sign, _ := newSignedIDP(t)
	// 无 preferred_username → 回退 sub；无 name → 回退 given_name。
	sign(map[string]interface{}{
		"aud":        "croupier",
		"sub":        "subj-1",
		"given_name": "Given",
		"exp":        time.Now().Add(time.Hour).Unix(),
	})
	p := mustProvider(t, cfg)

	ident, err := p.Exchange(context.Background(), "code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Username != "subj-1" || ident.Nickname != "Given" || ident.Email != "" {
		t.Fatalf("identity mismatch: %+v", ident)
	}
}

func TestOIDCProvider_Exchange_CustomUsernameClaim(t *testing.T) {
	_, cfg, sign, _ := newSignedIDP(t)
	sign(map[string]interface{}{
		"aud":   "croupier",
		"sub":   "subj-1",
		"login": "dave",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	cfg.UsernameClaim = "login"
	p := mustProvider(t, cfg)

	ident, err := p.Exchange(context.Background(), "code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Username != "dave" {
		t.Fatalf("custom claim not honored: %+v", ident)
	}
}

func TestOIDCProvider_Exchange_NoUsableClaim(t *testing.T) {
	_, cfg, sign, _ := newSignedIDP(t)
	sign(map[string]interface{}{
		"aud": "croupier",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	p := mustProvider(t, cfg)

	if _, err := p.Exchange(context.Background(), "code"); err == nil ||
		!strings.Contains(err.Error(), "no usable username claim") {
		t.Fatalf("want no-claim error, got %v", err)
	}
}

func TestOIDCProvider_Exchange_TamperedToken(t *testing.T) {
	_, cfg, sign, setToken := newSignedIDP(t)
	valid := sign(map[string]interface{}{
		"aud": "croupier",
		"sub": "subj-1",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	// 复用原签名，替换 payload 为恶意 claims，注入 token 端点。
	parts := strings.Split(valid, ".")
	tampered := parts[0] + "." +
		base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"`+cfg.Issuer+`","aud":"croupier","sub":"evil","exp":9999999999}`)) +
		"." + parts[2]
	setToken(tampered)

	p := mustProvider(t, cfg)
	if _, err := p.Exchange(context.Background(), "code"); err == nil {
		t.Fatal("tampered token must fail signature verification")
	}
}

func TestClaimString_Direct(t *testing.T) {
	claims := map[string]interface{}{
		"s":   "  padded  ",
		"n":   json.Number("42"),
		"i":   7,
		"nil": nil,
	}
	if got := claimString(claims, "s"); got != "padded" {
		t.Fatalf("string trim mismatch: %q", got)
	}
	if got := claimString(claims, "n"); got != "42" {
		t.Fatalf("json.Number mismatch: %q", got)
	}
	if got := claimString(claims, "i"); got != "" {
		t.Fatalf("non-string must be empty, got %q", got)
	}
	if got := claimString(claims, "nil"); got != "" {
		t.Fatalf("nil must be empty, got %q", got)
	}
	if got := claimString(claims, "missing"); got != "" {
		t.Fatalf("missing must be empty, got %q", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "x", "y"); got != "x" {
		t.Fatalf("mismatch: %q", got)
	}
	if got := firstNonEmpty(); got != "" {
		t.Fatalf("empty args must be empty, got %q", got)
	}
}
