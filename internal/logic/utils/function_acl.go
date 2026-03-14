package utils

import (
	"strings"

	"github.com/cuihairu/croupier/internal/model"
)

// FunctionActionAllowed checks whether a user with the given role names can perform
// an action on a function, based on function_permissions records.
//
// Backward compatibility rules:
// - If there is no matching rule for the action, this returns (false, false).
// - The caller can decide a default policy (e.g. only allow functions:manage).
func FunctionActionAllowed(roleNames []string, perms []model.FunctionPermission, action string, gameID string, env string) (allowed bool, hasRule bool) {
	if HasAdminRole(roleNames) {
		return true, true
	}

	want := strings.ToLower(strings.TrimSpace(action))
	if want == "" {
		return false, false
	}
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)

	roleSet := make(map[string]struct{}, len(roleNames))
	for _, r := range roleNames {
		key := strings.ToLower(strings.TrimSpace(r))
		if key == "" {
			continue
		}
		roleSet[key] = struct{}{}
	}

	matchesAction := func(values []string) bool {
		for _, v := range values {
			a := strings.ToLower(strings.TrimSpace(v))
			if a == "" {
				continue
			}
			if a == "*" || a == want {
				return true
			}
			// Small synonym set to tolerate legacy naming in configs.
			switch want {
			case "invoke":
				if a == "execute" || a == "call" || a == "run" {
					return true
				}
			case "read":
				if a == "list" || a == "view" {
					return true
				}
			}
		}
		return false
	}

	matchesRole := func(values []string) bool {
		for _, v := range values {
			r := strings.ToLower(strings.TrimSpace(v))
			if r == "" {
				continue
			}
			if r == "*" || r == "admin" || r == "super_admin" {
				return true
			}
			if _, ok := roleSet[r]; ok {
				return true
			}
		}
		return false
	}

	for i := range perms {
		p := perms[i]
		if scoped := strings.TrimSpace(p.GameID); scoped != "" && gameID != "" && !strings.EqualFold(scoped, gameID) {
			continue
		} else if strings.TrimSpace(p.GameID) != "" && gameID == "" {
			// When caller didn't specify game, only match unscoped rules.
			continue
		}
		if scoped := strings.TrimSpace(p.Env); scoped != "" && env != "" && !strings.EqualFold(scoped, env) {
			continue
		} else if strings.TrimSpace(p.Env) != "" && env == "" {
			continue
		}
		acts := DecodeStringSlice(p.Actions)
		if !matchesAction(acts) {
			continue
		}
		hasRule = true

		roles := DecodeStringSlice(p.Roles)
		if matchesRole(roles) {
			return true, true
		}
	}

	return false, hasRule
}
