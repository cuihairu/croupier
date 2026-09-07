package rbac

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/casbin/casbin/v2/model"
)

// ---- casbin.go keyMatch2Func：直接驱动参数校验与匹配逻辑 ----

func TestKeyMatch2Func_ArityAndMatching(t *testing.T) {
	t.Run("no args yields false without error", func(t *testing.T) {
		got, err := keyMatch2Func()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != false {
			t.Fatalf("expected false, got %v", got)
		}
	})

	t.Run("single arg yields false without error", func(t *testing.T) {
		got, err := keyMatch2Func("/api/v1/roles")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != false {
			t.Fatalf("expected false, got %v", got)
		}
	})

	t.Run("matching wildcard path", func(t *testing.T) {
		got, err := keyMatch2Func("/api/v1/roles/42", "/api/v1/roles/:id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != true {
			t.Fatalf("expected true, got %v", got)
		}
	})

	t.Run("non-matching path", func(t *testing.T) {
		got, err := keyMatch2Func("/api/v1/games", "/api/v1/roles/:id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != false {
			t.Fatalf("expected false, got %v", got)
		}
	})

	t.Run("non-string args fall back to empty strings", func(t *testing.T) {
		got, err := keyMatch2Func(42, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != true {
			t.Fatalf("expected true for empty-vs-empty pattern, got %v", got)
		}
	})
}

// ---- enhanced.go splitComparisonTerm：畸形切分时的 continue 分支 ----

func TestEvaluateComparison_SplitMismatchContinues(t *testing.T) {
	orig := splitComparisonTerm
	splitComparisonTerm = func(term, op string) []string {
		return []string{term}
	}
	t.Cleanup(func() { splitComparisonTerm = orig })

	evaluator := NewEnhancedEvaluator(NewPolicy())
	authCtx := &AuthContext{User: "u1", Now: time.Now()}

	if evaluator.evaluateComparison(context.Background(), authCtx, "1>=2") {
		t.Error("comparison with malformed split must evaluate false")
	}
}

// ---- logical_permissions.go：注入 enforcer 构造/使用失败 ----

type fakeLogicalEnforcer struct {
	addErr     error
	enforceErr error
}

func (f *fakeLogicalEnforcer) Enforce(params ...interface{}) (bool, error) {
	return false, f.enforceErr
}

func (f *fakeLogicalEnforcer) AddPolicy(params ...interface{}) (bool, error) {
	return f.addErr == nil, f.addErr
}

func swapLogicalSeams(t *testing.T, modelFn func(string) (model.Model, error), enfFn func(model.Model) (logicalPermissionEnforcer, error)) {
	t.Helper()
	origModel, origEnf := newLogicalModelFromString, newLogicalEnforcer
	if modelFn != nil {
		newLogicalModelFromString = modelFn
	}
	if enfFn != nil {
		newLogicalEnforcer = enfFn
	}
	t.Cleanup(func() {
		newLogicalModelFromString = origModel
		newLogicalEnforcer = origEnf
	})
}

func TestEnforceAnyPermission_ModelParseError(t *testing.T) {
	errModelBoom := errors.New("logical model boom")
	swapLogicalSeams(t, func(string) (model.Model, error) {
		return nil, errModelBoom
	}, nil)

	allowed, err := EnforceAnyPermission("u1", []string{"user:read"}, "user:read")
	if !errors.Is(err, errModelBoom) {
		t.Fatalf("expected model parse error, got %v", err)
	}
	if allowed {
		t.Error("must not allow when enforcer cannot be built")
	}
}

func TestEnforceAnyPermission_EnforcerCreateError(t *testing.T) {
	errEnforcerBoom := errors.New("enforcer boom")
	swapLogicalSeams(t, model.NewModelFromString, func(model.Model) (logicalPermissionEnforcer, error) {
		return nil, errEnforcerBoom
	})

	allowed, err := EnforceAnyPermission("u1", []string{"user:read"}, "user:read")
	if !errors.Is(err, errEnforcerBoom) {
		t.Fatalf("expected enforcer create error, got %v", err)
	}
	if allowed {
		t.Error("must not allow when enforcer cannot be built")
	}
}

func TestEnforceAnyPermission_AddPolicyError(t *testing.T) {
	errAddBoom := errors.New("add policy boom")
	swapLogicalSeams(t, model.NewModelFromString, func(model.Model) (logicalPermissionEnforcer, error) {
		return &fakeLogicalEnforcer{addErr: errAddBoom}, nil
	})

	allowed, err := EnforceAnyPermission("u1", []string{"user:read"}, "user:read")
	if !errors.Is(err, errAddBoom) {
		t.Fatalf("expected add policy error, got %v", err)
	}
	if allowed {
		t.Error("must not allow when granted policy cannot be loaded")
	}
}

func TestEnforceAnyPermission_EnforceError(t *testing.T) {
	errEnforceBoom := errors.New("enforce boom")
	swapLogicalSeams(t, model.NewModelFromString, func(model.Model) (logicalPermissionEnforcer, error) {
		return &fakeLogicalEnforcer{enforceErr: errEnforceBoom}, nil
	})

	allowed, err := EnforceAnyPermission("u1", []string{"user:read"}, "user:read")
	if !errors.Is(err, errEnforceBoom) {
		t.Fatalf("expected enforce error, got %v", err)
	}
	if allowed {
		t.Error("must not allow when enforce fails")
	}
}
