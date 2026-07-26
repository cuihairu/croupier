package function

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestMetadataHelpers(t *testing.T) {
	t.Parallel()

	meta := map[string]interface{}{
		"name":   "demo",
		"count":  "12",
		"flag":   "true",
		"nodes":  []interface{}{"a", "b"},
		"object": map[string]interface{}{"a": 1},
	}

	if got := getStringFromMetadata(meta, "name"); got != "demo" {
		t.Fatalf("unexpected string metadata: %q", got)
	}
	if got := getIntFromMetadata(meta, "count"); got != 12 {
		t.Fatalf("unexpected int metadata: %d", got)
	}
	if !getBoolFromMetadata(meta, "flag") {
		t.Fatal("expected bool metadata true")
	}
	nodes := getStringSliceFromMetadata(meta, "nodes")
	if len(nodes) != 2 || nodes[0] != "a" {
		t.Fatalf("unexpected string slice metadata: %#v", nodes)
	}
	obj := jsonValueFromMetadata(meta, "object")
	if obj == nil {
		t.Fatal("expected interface metadata")
	}
}

func TestParseRolesAndActionsFromJSON(t *testing.T) {
	t.Parallel()

	if got := parseRolesFromJSON(datatypes.JSON([]byte(`["admin","viewer"]`))); len(got) != 2 {
		t.Fatalf("unexpected roles array parse: %#v", got)
	}
	if got := parseRolesFromJSON(datatypes.JSON([]byte(`"admin,viewer"`))); len(got) != 2 {
		t.Fatalf("unexpected roles string parse: %#v", got)
	}
	if got := parseActionsFromJSON(datatypes.JSON([]byte(`["read","write"]`))); len(got) != 2 {
		t.Fatalf("unexpected actions array parse: %#v", got)
	}
	if got := parseActionsFromJSON(datatypes.JSON([]byte(`"read,write"`))); len(got) != 2 {
		t.Fatalf("unexpected actions string parse: %#v", got)
	}
}

func TestEnforceInvokePermission(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	functionModel := model.NewFunctionModel(db)
	ctx := context.Background()
	fn := &model.Function{FunctionID: "f1", Name: "demo"}
	if err := db.WithContext(ctx).Create(fn).Error; err != nil {
		t.Fatalf("create function failed: %v", err)
	}
	if err := db.WithContext(ctx).Create(&model.FunctionPermission{
		FunctionID: "f1",
		Resource:   "function",
		Actions:    datatypes.JSON([]byte(`["invoke"]`)),
		Roles:      datatypes.JSON([]byte(`["viewer"]`)),
	}).Error; err != nil {
		t.Fatalf("create permission failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{FunctionModel: functionModel}

	if err := enforceInvokePermission(svcCtx, []string{"admin"}, nil, "f1", "", ""); err != nil {
		t.Fatalf("expected admin bypass, got %v", err)
	}
	if err := enforceInvokePermission(svcCtx, []string{"viewer"}, nil, "f1", "", ""); err != nil {
		t.Fatalf("expected role-based invoke allowed, got %v", err)
	}
	fn2 := &model.Function{FunctionID: "f2", Name: "demo2"}
	if err := db.WithContext(ctx).Create(fn2).Error; err != nil {
		t.Fatalf("create function 2 failed: %v", err)
	}

	if err := enforceInvokePermission(svcCtx, []string{"guest"}, []string{"function:invoke"}, "f2", "", ""); err != nil {
		t.Fatalf("expected perm id fallback allowed, got %v", err)
	}
	if err := enforceInvokePermission(svcCtx, []string{"guest"}, nil, "f1", "", ""); err == nil {
		t.Fatal("expected invoke forbidden without role or perm")
	}
}

// Policy system integration tests

func TestFunctionPolicy_EnforceFunctionPolicy(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	// Migrate required tables
	if err := db.AutoMigrate(
		&model.Function{},
		&model.FunctionPolicy{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	ctx := context.Background()

	// Create policy manager
	policyManager, err := policymgr.NewManager(db, "")
	if err != nil {
		t.Fatalf("create policy manager failed: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		DB:            db,
		PolicyManager: policyManager,
	}

	// Test case 1: No restriction when no roles specified
	t.Run("no restriction when no roles", func(t *testing.T) {
		// First, set a policy with no role restriction
		override := &policymgr.Policy{
			FunctionID:   "test.function",
			AllowedRoles: []string{}, // Empty means no restriction
		}
		if err := policyManager.SetOverride(ctx, "test.function", override); err != nil {
			t.Fatalf("set policy failed: %v", err)
		}

		result, err := enforceFunctionPolicy(ctx, svcCtx, "test.function", []string{"any"})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if result == nil {
			t.Fatal("expected policy result")
		}
		if len(result.AllowedRoles) != 0 {
			t.Errorf("expected no role restriction, got %d roles", len(result.AllowedRoles))
		}
	})

	// Test case 2: Set policy with role restriction
	t.Run("role restriction enforced", func(t *testing.T) {
		// Set policy requiring admin role
		override := &policymgr.Policy{
			FunctionID:   "test.secure",
			AllowedRoles: []string{"admin"},
		}
		if err := policyManager.SetOverride(ctx, "test.secure", override); err != nil {
			t.Fatalf("set policy failed: %v", err)
		}

		// Test with allowed role
		result, err := enforceFunctionPolicy(ctx, svcCtx, "test.secure", []string{"admin"})
		if err != nil {
			t.Fatalf("expected no error for admin, got %v", err)
		}
		if result == nil {
			t.Fatal("expected policy result")
		}

		// Test with denied role
		_, err = enforceFunctionPolicy(ctx, svcCtx, "test.secure", []string{"user"})
		if err == nil {
			t.Fatal("expected error for user role")
		}
	})

	// Test case 3: Case-insensitive role matching
	t.Run("case insensitive role matching", func(t *testing.T) {
		override := &policymgr.Policy{
			FunctionID:   "test.case",
			AllowedRoles: []string{"Admin", "Operator"},
		}
		if err := policyManager.SetOverride(ctx, "test.case", override); err != nil {
			t.Fatalf("set policy failed: %v", err)
		}

		// Test with lowercase role name
		result, err := enforceFunctionPolicy(ctx, svcCtx, "test.case", []string{"admin"})
		if err != nil {
			t.Fatalf("expected no error with lowercase admin, got %v", err)
		}
		if result == nil {
			t.Fatal("expected policy result")
		}
	})
}

func TestFunctionPolicy_RiskLevelDefaults(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	policyManager, err := policymgr.NewManager(db, "")
	if err != nil {
		t.Fatalf("create policy manager failed: %v", err)
	}

	tests := []struct {
		name             string
		riskLevel        policymgr.RiskLevel
		requireApproval  bool
		requireAudit     bool
		approvalWorkflow string
		allowedRoles     []string
	}{
		{
			name:             "low risk",
			riskLevel:        policymgr.RiskLow,
			requireApproval:  false,
			requireAudit:     false,
			approvalWorkflow: "",
			allowedRoles:     []string{"user", "operator"},
		},
		{
			name:             "medium risk",
			riskLevel:        policymgr.RiskMedium,
			requireApproval:  false,
			requireAudit:     true,
			approvalWorkflow: "",
			allowedRoles:     []string{"operator"},
		},
		{
			name:             "high risk",
			riskLevel:        policymgr.RiskHigh,
			requireApproval:  true,
			requireAudit:     true,
			approvalWorkflow: "single_admin",
			allowedRoles:     []string{"admin"},
		},
		{
			name:             "danger risk",
			riskLevel:        policymgr.RiskDanger,
			requireApproval:  true,
			requireAudit:     true,
			approvalWorkflow: "two_person",
			allowedRoles:     []string{"super_admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := policyManager.GetDefaultPolicy(tt.riskLevel)
			if policy.RequireApproval != tt.requireApproval {
				t.Errorf("RequireApproval: got %v, want %v", policy.RequireApproval, tt.requireApproval)
			}
			if policy.RequireAudit != tt.requireAudit {
				t.Errorf("RequireAudit: got %v, want %v", policy.RequireAudit, tt.requireAudit)
			}
			if policy.ApprovalWorkflow != tt.approvalWorkflow {
				t.Errorf("ApprovalWorkflow: got %q, want %q", policy.ApprovalWorkflow, tt.approvalWorkflow)
			}
			if len(policy.AllowedRoles) != len(tt.allowedRoles) {
				t.Errorf("AllowedRoles length: got %d, want %d", len(policy.AllowedRoles), len(tt.allowedRoles))
			}
		})
	}
}
