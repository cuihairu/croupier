package rbac

import "net/http"

// PolicyInterface defines the interface for authorization policies
type PolicyInterface interface {
	Can(user, permission string) bool
	CanHTTP(user string, roles []string, r *http.Request) bool
}

// Minimal RBAC policy stub (legacy implementation).
type Policy struct {
	// allow[user][permission] = true
	allow map[string]map[string]bool
}

func NewPolicy() *Policy { return &Policy{allow: map[string]map[string]bool{}} }

func (p *Policy) Grant(user, perm string) {
	m := p.allow[user]
	if m == nil {
		m = map[string]bool{}
		p.allow[user] = m
	}
	m[perm] = true
}

func (p *Policy) Can(user, perm string) bool {
	if m := p.allow[user]; m != nil {
		if m[perm] {
			return true
		}
		if m["*"] {
			return true
		}
	}
	return false
}

// CanHTTP implements PolicyInterface for legacy compatibility
func (p *Policy) CanHTTP(user string, roles []string, r *http.Request) bool {
	// This is a simple implementation for the legacy policy
	// In practice, you would convert HTTP request to permission check
	path := r.URL.Path
	method := r.Method
	permission := method + ":" + path
	return p.Can(user, permission) || p.Can(user, "*")
}
