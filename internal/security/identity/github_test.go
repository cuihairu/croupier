package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGitHubFixture 起一个假 GitHub（token 端点 + REST API）并返回指向它的
// Provider——端点基址经 AuthBase/APIBase 注入，不访问真实 github.com。
func newGitHubFixture(t *testing.T, profile string, emails string) (*GitHubProvider, *[]*http.Request) {
	t.Helper()
	var hits []*http.Request
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok-1","token_type":"bearer"}`))
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(profile))
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emails))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p, err := NewGitHubProvider(GitHubConfig{
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURL:  "https://gm.example.com/api/v1/auth/github/callback",
		AuthBase:     srv.URL,
		APIBase:      srv.URL,
	})
	require.NoError(t, err)
	return p, &hits
}

func TestNewGitHubProvider_RequiresCredentials(t *testing.T) {
	_, err := NewGitHubProvider(GitHubConfig{ClientID: "", ClientSecret: ""})
	assert.Error(t, err)
}

func TestGitHubProvider_AuthCodeURL(t *testing.T) {
	p, _ := newGitHubFixture(t, `{}`, `[]`)
	u := p.AuthCodeURL("state-1")
	assert.Contains(t, u, "/login/oauth/authorize")
	assert.Contains(t, u, "client_id=cid")
	assert.Contains(t, u, "state=state-1")
	assert.Contains(t, u, "read%3Auser")
}

func TestGitHubProvider_Exchange_HappyPath(t *testing.T) {
	p, hits := newGitHubFixture(t,
		`{"login":"octocat","name":"The Octocat","email":"octo@example.com"}`, `[]`)
	ident, err := p.Exchange(context.Background(), "code-1")
	require.NoError(t, err)
	assert.Equal(t, KindGitHub, ident.Provider)
	assert.Equal(t, "octocat", ident.Username)
	assert.Equal(t, "The Octocat", ident.Nickname)
	assert.Equal(t, "octo@example.com", ident.Email)
	// profile 主邮箱存在时不查 /user/emails
	assert.Len(t, *hits, 2, "token + /user only")
}

func TestGitHubProvider_Exchange_PrivateEmailFallsBackToPrimary(t *testing.T) {
	// profile email 为空（GitHub 私密邮箱形态）→ 回退 /user/emails 的 primary
	p, hits := newGitHubFixture(t,
		`{"login":"octocat","name":""}`,
		`[{"email":"alt@example.com","primary":false},{"email":"pri@example.com","primary":true}]`)
	ident, err := p.Exchange(context.Background(), "code-1")
	require.NoError(t, err)
	assert.Equal(t, "octocat", ident.Username)
	assert.Equal(t, "octocat", ident.Nickname, "name 为空回退 login")
	assert.Equal(t, "pri@example.com", ident.Email)
	assert.Len(t, *hits, 3, "token + /user + /user/emails")
}

func TestGitHubProvider_Exchange_ProfileWithoutLogin(t *testing.T) {
	p, _ := newGitHubFixture(t, `{"name":"ghost"}`, `[]`)
	_, err := p.Exchange(context.Background(), "code-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no login")
}
