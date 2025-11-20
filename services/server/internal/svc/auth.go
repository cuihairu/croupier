package svc

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cuihairu/croupier/internal/security/token"
)

// Authenticator validates requests and returns user + roles info.
type Authenticator interface {
	Authenticate(r *http.Request) (string, []string, bool)
}

type Authorizer interface {
	Can(user string, roles []string, perm string) bool
}

type noopAuthenticator struct{}

func (n *noopAuthenticator) Authenticate(r *http.Request) (string, []string, bool) {
	return "dev", []string{"admin"}, true
}

func newJWTAuthenticator(secret string) (Authenticator, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("jwt secret empty")
	}
	return &jwtAuthenticator{manager: token.NewManager(secret)}, nil
}

type noopAuthorizer struct{}

func newNoopRBAC() Authorizer { return &noopAuthorizer{} }

func (n *noopAuthorizer) Can(user string, roles []string, perm string) bool { return true }

type jwtAuthenticator struct {
	manager *token.Manager
}

func (j *jwtAuthenticator) Authenticate(r *http.Request) (string, []string, bool) {
	authz := r.Header.Get("Authorization")
	if strings.HasPrefix(authz, "Bearer ") && j.manager != nil {
		tokenStr := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		if tokenStr == "" {
			return "", nil, false
		}
		user, roles, err := j.manager.Verify(tokenStr)
		if err == nil {
			return user, roles, true
		}
	}
	return "", nil, false
}
