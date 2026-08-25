// Package reqinfo extracts client identity (IP, User-Agent) from gin requests
// into the request context so non-handler layers (audit, telemetry) can read
// it without plumbing gin.Context through service signatures.
package reqinfo

import (
	"context"

	"github.com/gin-gonic/gin"
)

type contextKey struct{}

// Info carries the client identity of the current request.
type Info struct {
	IP        string
	UserAgent string
}

// Middleware stores the gin client identity into the request context.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		info := Info{
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		}
		c.Request = c.Request.WithContext(WithContext(c.Request.Context(), info))
		c.Next()
	}
}

// WithContext attaches info to ctx.
func WithContext(ctx context.Context, info Info) context.Context {
	return context.WithValue(ctx, contextKey{}, info)
}

// FromContext returns the stored info, if any.
func FromContext(ctx context.Context) (Info, bool) {
	if ctx == nil {
		return Info{}, false
	}
	info, ok := ctx.Value(contextKey{}).(Info)
	return info, ok
}
