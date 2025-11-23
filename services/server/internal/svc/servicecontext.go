// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"context"
	"net/http"

	"github.com/cuihairu/croupier/services/server/internal/config"
)

type ctxKey string

const actorKey ctxKey = "actor"

type ServiceContext struct {
	Config        config.Config
	authenticator Authenticator
	authorizer    Authorizer
}

func NewServiceContext(c config.Config) *ServiceContext {
	var auth Authenticator
	if c.Auth.JWTSecret != "" {
		var err error
		auth, err = newJWTAuthenticator(c.Auth.JWTSecret)
		if err != nil {
			auth = &noopAuthenticator{}
		}
	} else {
		auth = &noopAuthenticator{}
	}

	return &ServiceContext{
		Config:        c,
		authenticator: auth,
		authorizer:    newNoopRBAC(),
	}
}

// Authenticate validates the request and returns user, roles, and success status.
func (s *ServiceContext) Authenticate(r *http.Request) (string, []string, bool) {
	return s.authenticator.Authenticate(r)
}

// EnforcePermission checks if the user has the required permission.
func (s *ServiceContext) EnforcePermission(user string, roles []string, perm string) bool {
	return s.authorizer.Can(user, roles, perm)
}

// WithActor adds the actor (user) to the context.
func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

// ActorFromContext retrieves the actor from the context.
func ActorFromContext(ctx context.Context) string {
	if v := ctx.Value(actorKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
