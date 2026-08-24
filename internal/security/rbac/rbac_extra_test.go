package rbac

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Policy.CanHTTP ------------------------------------------------------

func TestExtra_Policy_CanHTTP(t *testing.T) {
	policy := NewPolicy()
	policy.Grant("alice", "GET:/api/v1/things")
	policy.Grant("super", "*")

	req := func(method, path string) *http.Request {
		r, err := http.NewRequest(method, path, nil)
		require.NoError(t, err)
		return r
	}

	assert.True(t, policy.CanHTTP("alice", nil, req(http.MethodGet, "/api/v1/things")))
	assert.False(t, policy.CanHTTP("alice", nil, req(http.MethodPost, "/api/v1/things")))
	assert.True(t, policy.CanHTTP("super", nil, req(http.MethodDelete, "/anything")))
	assert.False(t, policy.CanHTTP("nobody", nil, req(http.MethodGet, "/api/v1/things")))
}

// --- CasbinPolicy extras --------------------------------------------------

func TestExtra_CasbinPolicy_CanHTTP_UserPolicies(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	require.NoError(t, cp.AddPolicy("plainuser", "/api/v1/plain", "GET"))
	require.NoError(t, cp.AddPolicy("user:pfx", "/api/v1/prefixed", "GET"))

	req1, _ := http.NewRequest(http.MethodGet, "/api/v1/plain", nil)
	assert.True(t, cp.CanHTTP("plainuser", nil, req1), "direct user policy should match")

	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/prefixed", nil)
	assert.True(t, cp.CanHTTP("pfx", nil, req2), "user: prefixed policy should match")

	req3, _ := http.NewRequest(http.MethodPost, "/api/v1/plain", nil)
	assert.False(t, cp.CanHTTP("plainuser", nil, req3))
}

func TestExtra_CasbinPolicy_parsePermission_UpdateAction(t *testing.T) {
	cp := loadTestCasbinPolicy(t)

	obj, act := cp.parsePermission("resources:update")
	assert.Equal(t, "/api/v1/resources", obj)
	assert.Equal(t, "PUT", act)

	obj, act = cp.parsePermission("widgets:update")
	assert.Equal(t, "/api/v1/widgets", obj)
	assert.Equal(t, "PUT", act)
}

// --- EnhancedEvaluator extras ---------------------------------------------

func newExtraEvaluator() (*EnhancedEvaluator, *AuthContext) {
	policy := NewPolicy()
	policy.Grant("alice", "item:read")
	evaluator := NewEnhancedEvaluator(policy)
	authCtx := &AuthContext{
		User:        "alice",
		Permissions: []string{"item:read"},
		Roles:       []string{"gm"},
		Resource: map[string]any{
			"price": 42.0,
			"name":  "sword",
			"owner": "bob",
			"count": 7,
		},
		Request: map[string]any{"amount": 5},
		Now:     time.Date(2024, 11, 13, 10, 30, 0, 0, time.UTC), // Wednesday
	}
	return evaluator, authCtx
}

func TestExtra_EnhancedEvaluator_TimeWindowExpressions(t *testing.T) {
	ev, ctx := newExtraEvaluator()
	c := context.Background()

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{"bare time range inside window", "09:00-17:00", true},
		{"bare time range outside window", "11:00-12:00", false},
		{"overnight range spans midnight", "18:00-08:00", false},
		{"garbage time falls back to truthy literal", "99:99-aa:00", true},
		{"day short name matches", "Wed", true},
		{"day full name matches", "Wednesday", true},
		{"day full name no match", "Monday", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ev.EvaluateAllowIf(c, ctx, tt.expression))
		})
	}
}

func TestExtra_EnhancedEvaluator_FunctionArgumentValidation(t *testing.T) {
	ev, ctx := newExtraEvaluator()
	c := context.Background()

	tests := []struct {
		name       string
		expression string
	}{
		{"has_role no args", "has_role()"},
		{"has_role too many args", "has_role('a', 'b')"},
		{"is_owner no ownership fields", "is_owner('unknown_field')"},
		{"is_owner two args", "is_owner('a', 'b')"},
		{"time_between no args", "time_between()"},
		{"time_between one arg", "time_between('09:00')"},
		{"time_between bad format", "time_between('abc', 'def')"},
		{"day_of_week no args", "day_of_week()"},
		{"hour_between one arg", "hour_between('9')"},
		{"hour_between bad numbers", "hour_between('x', 'y')"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, ev.EvaluateAllowIf(c, ctx, tt.expression))
		})
	}
}

func TestExtra_EnhancedEvaluator_IsOwnerNonStringAndMissing(t *testing.T) {
	ev := NewEnhancedEvaluator(NewPolicy())
	c := context.Background()

	ctx := &AuthContext{
		User: "alice",
		Resource: map[string]any{
			"owner_id": 12345, // non-string owner value is ignored
		},
		Now: time.Now(),
	}
	assert.False(t, ev.EvaluateAllowIf(c, ctx, "is_owner()"))
	assert.False(t, ev.EvaluateAllowIf(c, ctx, "is_owner('user_id')"))
}

func TestExtra_EnhancedEvaluator_SpecialVariablesAndLiterals(t *testing.T) {
	ev, ctx := newExtraEvaluator()
	c := context.Background()

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{"user_id equality", `user_id == "alice"`, true},
		{"user_id inequality", `user_id != "bob"`, true},
		{"numeric truthy literal", "5", true},
		{"zero literal falsy", "0", false},
		{"now variable non-empty", "now", true},
		{"request field reference", "request.amount == 5", true},
		{"string compare less than", `resource.name < "zzz"`, true},
		{"string compare greater equal", `resource.name >= "aaa"`, true},
		{"string compare less equal", `resource.name <= "sword"`, true},
		{"string compare greater than", `resource.name > "aaa"`, true},
		{"mixed numeric string compare", `resource.price < "1000"`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ev.EvaluateAllowIf(c, ctx, tt.expression))
		})
	}
}

func TestExtra_EnhancedEvaluator_NegationOfTimeWindow(t *testing.T) {
	ev, ctx := newExtraEvaluator()

	// Negation wraps a parenthesised time-window term.
	assert.True(t, ev.EvaluateAllowIf(context.Background(), ctx, "!(Mon)"))
	assert.False(t, ev.EvaluateAllowIf(context.Background(), ctx, "!(Wed)"))
}

// --- Loader extras ---------------------------------------------------------

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestExtra_LoadPolicy_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "policy.json", "{not json")

	_, err := LoadPolicy(path)
	assert.Error(t, err)
}

func TestExtra_LoadPolicy_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "policy.json", `{"allow":{"carol":["games:read"]}}`)

	p, err := LoadPolicy(path)
	require.NoError(t, err)
	assert.True(t, p.Can("carol", "games:read"))
}

func TestExtra_LoadCasbinPolicy_FallsBackToLegacy(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "rbac.json", `{"allow":{"dave":["*"]}}`)

	p, err := LoadCasbinPolicy(path)
	require.NoError(t, err)
	_, isCasbin := p.(*CasbinPolicy)
	assert.False(t, isCasbin, "should fall back to legacy policy without model files")
	assert.True(t, p.Can("dave", "whatever"))
}

func TestExtra_LoadPolicyAuto_YamlSuffixUsesCasbinDetection(t *testing.T) {
	dir := t.TempDir()
	// YAML-named file containing legacy JSON content.
	path := writeTempFile(t, dir, "rbac.yaml", `{"allow":{"erin":["roles:read"]}}`)

	p, err := LoadPolicyAuto(path)
	require.NoError(t, err)
	assert.True(t, p.Can("erin", "roles:read"))

	// Missing file with non-json suffix also errors.
	_, err = LoadPolicyAuto(filepath.Join(dir, "missing.yaml"))
	assert.Error(t, err)
}

// --- splitLogicalPermission extras ----------------------------------------

func TestExtra_SplitLogicalPermission_EmptyResourceAndAction(t *testing.T) {
	obj, act := splitLogicalPermission(":read")
	assert.Equal(t, "*", obj)
	assert.Equal(t, "read", act)

	obj, act = splitLogicalPermission("roles:")
	assert.Equal(t, "roles", obj)
	assert.Equal(t, "*", act)
}

// --- UnifiedPolicyEngine extras --------------------------------------------

// newGrantedEngine returns an engine whose policy grants wildcard access to
// user "u", so tests can focus on risk / two-person-rule branches.
func newGrantedEngine() *UnifiedPolicyEngine {
	policy := NewPolicy()
	policy.Grant("u", "*")
	return NewUnifiedPolicyEngine(policy)
}

func TestExtra_Unified_Authorize_PermissionDenied(t *testing.T) {
	engine := NewUnifiedPolicyEngine(NewPolicy())

	result, err := engine.Authorize(context.Background(),
		&AuthDescriptor{Permission: "item:delete"},
		&AuthorizationRequest{User: "mallory"})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.Reason, "Missing required permission")
}

func TestExtra_Unified_Authorize_AllowIfDenied(t *testing.T) {
	engine := newGrantedEngine()

	result, err := engine.Authorize(context.Background(),
		&AuthDescriptor{Permission: "games:read", AllowIf: "resource.amount > 100"},
		&AuthorizationRequest{
			User:       "u",
			Parameters: map[string]any{"amount": 10.0},
		})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.Reason, "allow_if")
}

func TestExtra_Unified_RiskLevelMedium(t *testing.T) {
	engine := newGrantedEngine()
	now := time.Date(2024, 11, 13, 10, 0, 0, 0, time.UTC)

	result, err := engine.Authorize(context.Background(), &AuthDescriptor{
		Permission: "games:read",
		Risk:       &RiskPolicy{Level: "medium"},
	}, &AuthorizationRequest{User: "u", RequestTime: now})
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, "medium", result.RiskLevel)
	assert.Contains(t, result.Conditions, "audit_logged")
}

func TestExtra_Unified_RiskConditionFailure(t *testing.T) {
	engine := newGrantedEngine()
	now := time.Date(2024, 11, 13, 10, 0, 0, 0, time.UTC)

	result, err := engine.Authorize(context.Background(), &AuthDescriptor{
		Permission: "games:read",
		Risk: &RiskPolicy{
			Level:      "low",
			Conditions: []string{"request.amount > 100"},
		},
	}, &AuthorizationRequest{
		User:        "u",
		Context:     map[string]any{"amount": 5.0},
		RequestTime: now,
	})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.Reason, "Risk condition failed")
}

func TestExtra_Unified_TwoPersonRule_DefaultThresholdAndInvalidApprovals(t *testing.T) {
	engine := newGrantedEngine()
	now := time.Date(2024, 11, 13, 10, 0, 0, 0, time.UTC)

	t.Run("threshold defaults to 1 and missing approval blocks", func(t *testing.T) {
		result, err := engine.Authorize(context.Background(), &AuthDescriptor{
			Permission:    "*",
			TwoPersonRule: &TwoPersonRulePolicy{Required: true},
		}, &AuthorizationRequest{User: "u", RequestTime: now})
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.True(t, result.RequiresApproval)
		assert.Equal(t, 1, result.RequiredApprovals)
	})

	t.Run("wrong approver role rejected", func(t *testing.T) {
		result, err := engine.Authorize(context.Background(), &AuthDescriptor{
			Permission: "games:read",
			TwoPersonRule: &TwoPersonRulePolicy{
				Required:   true,
				Threshold:  1,
				Approvers:  []string{"senior_admin"},
				ExpiryTime: "24h",
			},
		}, &AuthorizationRequest{
			User: "u",
			Approvals: []Approval{{
				ApproverID:   "helper",
				ApproverRole: "viewer",
				Timestamp:    now,
			}},
			RequestTime: now,
		})
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Contains(t, result.Reason, "Invalid or expired approvals")
	})

	t.Run("expired approval rejected", func(t *testing.T) {
		result, err := engine.Authorize(context.Background(), &AuthDescriptor{
			Permission: "games:read",
			TwoPersonRule: &TwoPersonRulePolicy{
				Required:   true,
				Threshold:  1,
				ExpiryTime: "1h",
			},
		}, &AuthorizationRequest{
			User: "u",
			Approvals: []Approval{{
				ApproverID:   "helper",
				ApproverRole: "admin",
				Timestamp:    now.Add(-2 * time.Hour),
			}},
			RequestTime: now,
		})
		require.NoError(t, err)
		assert.False(t, result.Allowed)
	})

	t.Run("valid approval grants and sets expiry", func(t *testing.T) {
		result, err := engine.Authorize(context.Background(), &AuthDescriptor{
			Permission: "games:read",
			TwoPersonRule: &TwoPersonRulePolicy{
				Required:   true,
				Threshold:  1,
				ExpiryTime: "1h",
			},
		}, &AuthorizationRequest{
			User: "u",
			Approvals: []Approval{{
				ApproverID:   "helper",
				ApproverRole: "admin",
				Timestamp:    now,
			}},
			RequestTime: now,
		})
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.NotNil(t, result.ExpiresAt)
	})

	t.Run("bogus expiry yields nil expiry", func(t *testing.T) {
		result, err := engine.Authorize(context.Background(), &AuthDescriptor{
			Permission: "games:read",
			TwoPersonRule: &TwoPersonRulePolicy{
				Required:   false,
				ExpiryTime: "not-a-duration",
			},
		}, &AuthorizationRequest{User: "u", RequestTime: now})
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Nil(t, result.ExpiresAt)
	})
}

func TestExtra_Unified_HighRiskOutsideBusinessHours(t *testing.T) {
	engine := newGrantedEngine()
	sundayNight := time.Date(2024, 11, 17, 22, 0, 0, 0, time.UTC) // Sunday

	result, err := engine.Authorize(context.Background(), &AuthDescriptor{
		Permission: "games:read",
		Risk:       &RiskPolicy{Level: "high", TimeWindow: "business_hours"},
	}, &AuthorizationRequest{User: "u", RequestTime: sundayNight})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Contains(t, result.Reason, "business hours")
}
