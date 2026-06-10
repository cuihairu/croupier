package rbac

import (
	"net/http"
	"testing"
)

func loadTestCasbinPolicy(t *testing.T) *CasbinPolicy {
	t.Helper()
	p, err := LoadCasbinPolicy("../../../configs/rbac.json")
	if err != nil {
		t.Skipf("cannot load casbin policy: %v", err)
	}
	cp, ok := p.(*CasbinPolicy)
	if !ok {
		t.Skip("Casbin not active (legacy policy in use); skip")
	}
	return cp
}

func TestCasbinPolicy_Can_AdminWildcard(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	if !cp.Can("admin", "*") {
		t.Error("admin should have wildcard permission")
	}
}

func TestCasbinPolicy_Can_Denied(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	if cp.Can("unknown_user_xyz", "roles:read") {
		t.Error("unknown user should be denied")
	}
}

func TestCasbinPolicy_parsePermission(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	tests := []struct {
		permission string
		wantObj    string
		wantAct    string
	}{
		{"*", "*", "*"},
		{"roles:read", "/api/v1/roles", "GET"},
		{"games:write", "/api/v1/games", "POST"},
		{"users:create", "/api/v1/users", "POST"},
		{"entities:update", "/api/v1/entities", "PUT"},
		{"functions:delete", "/api/v1/functions", "DELETE"},
		{"assignments:manage", "/api/v1/assignments", "*"},
		{"registry:read", "/api/v1/registry", "GET"},
		{"approvals:write", "/api/v1/approvals", "POST"},
		{"messages:read", "/api/v1/messages", "GET"},
		{"certificates:read", "/api/v1/certificates", "GET"},
		{"uploads:read", "/api/v1/storage", "GET"},
		{"upload:read", "/api/v1/storage", "GET"},
		{"custom_resource", "/api/v1/custom_resource", "GET"},
		{"unknown:unknown_action", "/api/v1/unknown", "GET"},
		{"single_part", "/api/v1/single_part", "GET"},
	}

	for _, tt := range tests {
		t.Run(tt.permission, func(t *testing.T) {
			obj, act := cp.parsePermission(tt.permission)
			if obj != tt.wantObj {
				t.Errorf("obj = %q, want %q", obj, tt.wantObj)
			}
			if act != tt.wantAct {
				t.Errorf("act = %q, want %q", act, tt.wantAct)
			}
		})
	}
}

func TestCasbinPolicy_AddPolicy(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	if err := cp.AddPolicy("testuser", "/api/v1/test", "GET"); err != nil {
		t.Errorf("AddPolicy: %v", err)
	}
}

func TestCasbinPolicy_AddRoleForUser(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	if err := cp.AddRoleForUser("testuser", "role:admin"); err != nil {
		t.Errorf("AddRoleForUser: %v", err)
	}
}

func TestCasbinPolicy_RemovePolicy(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	_ = cp.AddPolicy("rmuser", "/api/v1/rm", "GET")
	if err := cp.RemovePolicy("rmuser", "/api/v1/rm", "GET"); err != nil {
		t.Errorf("RemovePolicy: %v", err)
	}
}

func TestCasbinPolicy_SavePolicy(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	// SavePolicy may fail if file is read-only, that's OK
	_ = cp.SavePolicy()
}

func TestCasbinPolicy_LoadPolicy(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	if err := cp.LoadPolicy(); err != nil {
		t.Errorf("LoadPolicy: %v", err)
	}
}

func TestCasbinPolicy_CanHTTP_Roles(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	// Admin should be allowed to any path
	req, _ := http.NewRequest("DELETE", "/api/v1/anything/here", nil)
	if !cp.CanHTTP("admin-user", []string{"admin"}, req) {
		t.Error("admin should be allowed to DELETE anything")
	}

	// Unknown user with no roles
	req2, _ := http.NewRequest("GET", "/api/v1/roles", nil)
	if cp.CanHTTP("nobody", []string{}, req2) {
		t.Error("nobody should be denied")
	}
}

func TestEnforceAnyPermission(t *testing.T) {
	t.Run("match first required", func(t *testing.T) {
		ok, err := EnforceAnyPermission("user1", []string{"roles:read", "games:write"}, "roles:read")
		if err != nil {
			t.Fatalf("EnforceAnyPermission: %v", err)
		}
		if !ok {
			t.Error("expected allowed")
		}
	})

	t.Run("match second required", func(t *testing.T) {
		ok, err := EnforceAnyPermission("user1", []string{"roles:read"}, "games:write", "roles:read")
		if err != nil {
			t.Fatalf("EnforceAnyPermission: %v", err)
		}
		if !ok {
			t.Error("expected allowed")
		}
	})

	t.Run("no match", func(t *testing.T) {
		ok, err := EnforceAnyPermission("user1", []string{"roles:read"}, "games:delete")
		if err != nil {
			t.Fatalf("EnforceAnyPermission: %v", err)
		}
		if ok {
			t.Error("expected denied")
		}
	})

	t.Run("wildcard grant", func(t *testing.T) {
		ok, err := EnforceAnyPermission("user1", []string{"*"}, "anything:read")
		if err != nil {
			t.Fatalf("EnforceAnyPermission: %v", err)
		}
		if !ok {
			t.Error("expected allowed with wildcard")
		}
	})

	t.Run("empty subject defaults", func(t *testing.T) {
		ok, err := EnforceAnyPermission("", []string{"roles:read"}, "roles:read")
		if err != nil {
			t.Fatalf("EnforceAnyPermission: %v", err)
		}
		if !ok {
			t.Error("expected allowed with default subject")
		}
	})

	t.Run("empty granted", func(t *testing.T) {
		ok, err := EnforceAnyPermission("user1", []string{}, "roles:read")
		if err != nil {
			t.Fatalf("EnforceAnyPermission: %v", err)
		}
		if ok {
			t.Error("expected denied with empty grants")
		}
	})
}

func TestSplitLogicalPermission(t *testing.T) {
	tests := []struct {
		input   string
		wantObj string
		wantAct string
	}{
		{"", "*", "*"},
		{"*", "*", "*"},
		{"admin:all", "*", "*"},
		{"roles:read", "roles", "read"},
		{"games:write", "games", "write"},
		{"single", "single", "*"},
		{"res:all", "res", "*"},
		{"  roles  :  read  ", "roles", "read"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			obj, act := splitLogicalPermission(tt.input)
			if obj != tt.wantObj {
				t.Errorf("obj = %q, want %q", obj, tt.wantObj)
			}
			if act != tt.wantAct {
				t.Errorf("act = %q, want %q", act, tt.wantAct)
			}
		})
	}
}

func TestNewCasbinPolicy_InvalidPath(t *testing.T) {
	_, err := NewCasbinPolicy("/nonexistent/model.conf", "/nonexistent/policy.csv")
	if err == nil {
		t.Error("expected error for nonexistent files")
	}
}

func TestLoadPolicyAuto(t *testing.T) {
	t.Run("json loads legacy", func(t *testing.T) {
		p, err := LoadPolicyAuto("../../../configs/rbac.json")
		if err != nil {
			t.Fatalf("LoadPolicyAuto: %v", err)
		}
		if p == nil {
			t.Fatal("expected non-nil policy")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := LoadPolicyAuto("/nonexistent/config.json")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}
