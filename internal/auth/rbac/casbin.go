package rbac

import (
	"log"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
)

// CasbinPolicy wraps Casbin enforcer
type CasbinPolicy struct {
	enforcer *casbin.Enforcer
}

// NewCasbinPolicy creates a new Casbin-based policy
func NewCasbinPolicy(modelPath, policyPath string) (*CasbinPolicy, error) {
	log.Printf("[RBAC] Loading Casbin policy - Model: %s, Policy: %s", modelPath, policyPath)

	enforcer, err := casbin.NewEnforcer(modelPath, policyPath)
	if err != nil {
		log.Printf("[RBAC] Failed to create Casbin enforcer: %v", err)
		return nil, err
	}

	// Enable logging
	enforcer.EnableLog(true)

	log.Printf("[RBAC] Casbin enforcer created successfully")

	return &CasbinPolicy{enforcer: enforcer}, nil
}

// Can checks if user has permission
func (p *CasbinPolicy) Can(user, permission string) bool {
	log.Printf("[RBAC-DEBUG] Can() called: user=%s, permission=%s", user, permission)

	// For HTTP permission checks, we need to parse permission into object and action
	obj, act := p.parsePermission(permission)
	log.Printf("[RBAC-DEBUG] Parsed permission: obj=%s, act=%s", obj, act)

	// Try different user formats
	userFormats := []string{
		user,           // direct user like "admin"
		"user:" + user, // prefixed user like "user:admin"
	}

	for _, userFormat := range userFormats {
		log.Printf("[RBAC-DEBUG] Checking permission for user format: %s", userFormat)
		allowed, err := p.enforcer.Enforce(userFormat, obj, act)
		if err != nil {
			log.Printf("[RBAC] Error checking permission for %s: %v", userFormat, err)
			continue
		}
		log.Printf("[RBAC-DEBUG] Enforce result for %s: %v", userFormat, allowed)
		if allowed {
			log.Printf("[RBAC] ALLOWED: %s -> %s:%s", userFormat, obj, act)
			return true
		}
	}

	log.Printf("[RBAC] DENIED: %s -> %s:%s", user, obj, act)
	return false
}

// CanHTTP checks HTTP request permission
func (p *CasbinPolicy) CanHTTP(user string, roles []string, r *http.Request) bool {
	path := r.URL.Path
	method := r.Method

	// Check direct user permissions
	if allowed, _ := p.enforcer.Enforce(user, path, method); allowed {
		log.Printf("[RBAC] ALLOWED: user %s -> %s %s", user, method, path)
		return true
	}

	// Check user with prefix
	userKey := "user:" + user
	if allowed, _ := p.enforcer.Enforce(userKey, path, method); allowed {
		log.Printf("[RBAC] ALLOWED: %s -> %s %s", userKey, method, path)
		return true
	}

	// Check role permissions
	for _, role := range roles {
		roleKey := "role:" + role
		if allowed, _ := p.enforcer.Enforce(roleKey, path, method); allowed {
			log.Printf("[RBAC] ALLOWED: %s (via %s) -> %s %s", user, roleKey, method, path)
			return true
		}
	}

	log.Printf("[RBAC] DENIED: user=%s, roles=%v -> %s %s", user, roles, method, path)
	return false
}

// parsePermission converts permission string to object and action
func (p *CasbinPolicy) parsePermission(permission string) (string, string) {
	// Handle wildcard
	if permission == "*" {
		return "*", "*"
	}

	// Convert permission like "roles:read" to object="/api/roles" and action="GET"
	parts := strings.Split(permission, ":")
	if len(parts) != 2 {
		// Fallback: treat as object with GET action
		return "/api/" + permission, "GET"
	}

	resource := parts[0]
	action := parts[1]

	// Map resource to API path
	var path string
	switch resource {
	case "roles":
		path = "/api/roles"
	case "games":
		path = "/api/games"
	case "users":
		path = "/api/users"
	case "entities":
		path = "/api/entities"
	case "functions":
		path = "/api/functions"
	case "assignments":
		path = "/api/assignments"
	case "registry":
		path = "/api/registry"
	case "approvals":
		path = "/api/approvals"
	case "messages":
		path = "/api/messages"
	case "certificates":
		path = "/api/certificates"
	case "components":
		path = "/api/components"
	case "uploads":
		path = "/api/uploads"
	default:
		path = "/api/" + resource
	}

	// Map action to HTTP method
	var method string
	switch action {
	case "read":
		method = "GET"
	case "write", "create", "manage":
		method = "*" // Allow all methods for write/manage
	case "update":
		method = "PUT"
	case "delete":
		method = "DELETE"
	default:
		method = "GET"
	}

	return path, method
}

// AddPolicy adds a new policy
func (p *CasbinPolicy) AddPolicy(sub, obj, act string) error {
	_, err := p.enforcer.AddPolicy(sub, obj, act)
	return err
}

// AddRoleForUser adds a role for user
func (p *CasbinPolicy) AddRoleForUser(user, role string) error {
	_, err := p.enforcer.AddRoleForUser(user, role)
	return err
}

// RemovePolicy removes a policy
func (p *CasbinPolicy) RemovePolicy(sub, obj, act string) error {
	_, err := p.enforcer.RemovePolicy(sub, obj, act)
	return err
}

// SavePolicy saves the policy to file
func (p *CasbinPolicy) SavePolicy() error {
	return p.enforcer.SavePolicy()
}

// LoadPolicy reloads the policy from file
func (p *CasbinPolicy) LoadPolicy() error {
	return p.enforcer.LoadPolicy()
}
