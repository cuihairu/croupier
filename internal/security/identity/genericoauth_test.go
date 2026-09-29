package identity

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenericOAuthProvider_AuthCodeURL(t *testing.T) {
	p, err := NewGenericOAuthProvider(GenericOAuthConfig{
		ClientID:     "cid",
		ClientSecret: "csecret",
		AuthURL:      "https://sso.example.com/authorize",
		TokenURL:     "https://sso.example.com/token",
		UserInfoURL:  "https://sso.example.com/userinfo",
		Scopes:       []string{"openid", "profile"},
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	got := p.AuthCodeURL("st-1")
	for _, want := range []string{"https://sso.example.com/authorize?", "client_id=cid", "state=st-1", "scope=openid+profile"} {
		if !strings.Contains(got, want) {
			t.Fatalf("auth url missing %q: %q", want, got)
		}
	}
}

func TestGenericOAuthProvider_ExchangeFieldMapping(t *testing.T) {
	var gotAuth string
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("token endpoint expects POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "code=code-9") {
			t.Errorf("token body missing code: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-9","token_type":"bearer"}`))
	}))
	defer tokenSrv.Close()

	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		// 非默认属性名 + 昵称缺失回退 username
		_, _ = w.Write([]byte(`{"login":"zhang.san","full_name":"张三","mail":"z@ex.com"}`))
	}))
	defer userSrv.Close()

	p, err := NewGenericOAuthProvider(GenericOAuthConfig{
		ClientID:      "cid",
		ClientSecret:  "csecret",
		AuthURL:       "https://sso.example.com/authorize",
		TokenURL:      tokenSrv.URL,
		UserInfoURL:   userSrv.URL,
		UsernameField: "login",
		NicknameField: "full_name",
		EmailField:    "mail",
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	ident, err := p.Exchange(context.Background(), "code-9")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if ident.Provider != KindGenericOAuth || ident.Username != "zhang.san" || ident.Nickname != "张三" || ident.Email != "z@ex.com" {
		t.Fatalf("unexpected identity: %+v", ident)
	}
	if gotAuth != "Bearer at-9" {
		t.Fatalf("userinfo auth header = %q", gotAuth)
	}
}

func TestGenericOAuthProvider_ExchangeDefaultsAndMissingUsername(t *testing.T) {
	// 默认字段名 username/name/email；缺 username 视为失败
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-8"}`))
	}))
	defer tokenSrv.Close()
	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"仅昵称"}`))
	}))
	defer userSrv.Close()

	p, err := NewGenericOAuthProvider(GenericOAuthConfig{
		ClientID:     "cid",
		ClientSecret: "csecret",
		AuthURL:      "https://sso.example.com/authorize",
		TokenURL:     tokenSrv.URL,
		UserInfoURL:  userSrv.URL,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	_, err = p.Exchange(context.Background(), "code")
	if err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("expected missing username field error, got: %v", err)
	}
}

func TestGenericOAuthProvider_ConfigValidation(t *testing.T) {
	base := GenericOAuthConfig{
		ClientID: "cid", ClientSecret: "cs",
		AuthURL: "https://sso.example.com/authorize", TokenURL: "https://sso.example.com/token",
		UserInfoURL: "https://sso.example.com/userinfo",
	}
	if _, err := NewGenericOAuthProvider(GenericOAuthConfig{}); err == nil {
		t.Fatal("expected empty config rejected")
	}
	noToken := base
	noToken.TokenURL = ""
	if _, err := NewGenericOAuthProvider(noToken); err == nil {
		t.Fatal("expected missing tokenUrl rejected")
	}
	noInfo := base
	noInfo.UserInfoURL = ""
	if _, err := NewGenericOAuthProvider(noInfo); err == nil {
		t.Fatal("expected missing userInfoUrl rejected")
	}
}
