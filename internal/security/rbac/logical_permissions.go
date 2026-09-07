package rbac

import (
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

const logicalPermissionModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`

// logicalPermissionEnforcer is the minimal enforcer surface needed for
// logical permission checks; *casbin.Enforcer satisfies it.
type logicalPermissionEnforcer interface {
	Enforce(params ...interface{}) (bool, error)
	AddPolicy(params ...interface{}) (bool, error)
}

// newLogicalModelFromString and newLogicalEnforcer are package seams that
// keep the real casbin construction while letting tests inject failures.
var newLogicalModelFromString = model.NewModelFromString

var newLogicalEnforcer = func(m model.Model) (logicalPermissionEnforcer, error) {
	return casbin.NewEnforcer(m)
}

// EnforceAnyPermission uses Casbin to evaluate logical permission IDs such as "user:read".
func EnforceAnyPermission(subject string, granted []string, required ...string) (bool, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "subject"
	}

	enforcer, err := newLogicalPermissionEnforcer(subject, granted)
	if err != nil {
		return false, err
	}

	for _, permission := range required {
		obj, act := splitLogicalPermission(permission)
		allowed, err := enforcer.Enforce(subject, obj, act)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}

	return false, nil
}

func newLogicalPermissionEnforcer(subject string, granted []string) (logicalPermissionEnforcer, error) {
	m, err := newLogicalModelFromString(logicalPermissionModel)
	if err != nil {
		return nil, err
	}

	enforcer, err := newLogicalEnforcer(m)
	if err != nil {
		return nil, err
	}

	for _, permission := range granted {
		obj, act := splitLogicalPermission(permission)
		if _, err := enforcer.AddPolicy(subject, obj, act); err != nil {
			return nil, err
		}
	}

	return enforcer, nil
}

func splitLogicalPermission(permission string) (string, string) {
	normalized := strings.ToLower(strings.TrimSpace(permission))
	switch normalized {
	case "", "*", "admin:all":
		return "*", "*"
	}

	parts := strings.SplitN(normalized, ":", 2)
	if len(parts) != 2 {
		return normalized, "*"
	}

	resource := strings.TrimSpace(parts[0])
	action := strings.TrimSpace(parts[1])
	if resource == "" {
		resource = "*"
	}
	if action == "" || action == "all" {
		action = "*"
	}
	return resource, action
}
