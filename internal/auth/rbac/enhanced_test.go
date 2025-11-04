package rbac

import (
	"context"
	"testing"
	"time"
)

func TestEnhancedEvaluator_HasPermission(t *testing.T) {
	policy := NewPolicy()
	policy.Grant("user123", "item:read")
	evaluator := NewEnhancedEvaluator(policy)

	authCtx := &AuthContext{
		User:        "user123",
		Permissions: []string{"item:write"},
		Now:         time.Now(),
	}

	tests := []struct {
		expression string
		expected   bool
	}{
		{"has_permission('item:read')", true},
		{"has_permission('item:write')", true},
		{"has_permission('item:delete')", false},
		{"has_permission('admin:all')", false},
	}

	for _, test := range tests {
		result := evaluator.EvaluateAllowIf(context.Background(), authCtx, test.expression)
		if result != test.expected {
			t.Errorf("Expression %s: expected %v, got %v", test.expression, test.expected, result)
		}
	}
}

func TestEnhancedEvaluator_IsOwner(t *testing.T) {
	policy := NewPolicy()
	evaluator := NewEnhancedEvaluator(policy)

	authCtx := &AuthContext{
		User: "user123",
		Resource: map[string]any{
			"owner":      "user123",
			"created_by": "user456",
			"item_id":    "item789",
		},
		Now: time.Now(),
	}

	tests := []struct {
		expression string
		expected   bool
	}{
		{"is_owner()", true},                   // Checks owner field
		{"is_owner('owner')", true},            // Explicit owner field
		{"is_owner('created_by')", false},      // Different user
		{"is_owner('nonexistent')", false},     // Field doesn't exist
	}

	for _, test := range tests {
		result := evaluator.EvaluateAllowIf(context.Background(), authCtx, test.expression)
		if result != test.expected {
			t.Errorf("Expression %s: expected %v, got %v", test.expression, test.expected, result)
		}
	}
}

func TestEnhancedEvaluator_NumericComparisons(t *testing.T) {
	policy := NewPolicy()
	evaluator := NewEnhancedEvaluator(policy)

	authCtx := &AuthContext{
		User: "user123",
		Resource: map[string]any{
			"price":    100.50,
			"quantity": 5,
		},
		Request: map[string]any{
			"amount": 50.0,
		},
		Now: time.Now(),
	}

	tests := []struct {
		expression string
		expected   bool
	}{
		{"resource.price > 100", true},
		{"resource.price < 200", true},
		{"resource.price >= 100.50", true},
		{"resource.quantity == 5", true},
		{"resource.quantity != 10", true},
		{"request.amount <= resource.price", true},
		{"request.amount > resource.price", false},
	}

	for _, test := range tests {
		result := evaluator.EvaluateAllowIf(context.Background(), authCtx, test.expression)
		if result != test.expected {
			t.Errorf("Expression %s: expected %v, got %v", test.expression, test.expected, result)
		}
	}
}

func TestEnhancedEvaluator_TimeWindows(t *testing.T) {
	policy := NewPolicy()
	evaluator := NewEnhancedEvaluator(policy)

	// Test during business hours (10:30 AM on a Wednesday)
	testTime := time.Date(2024, 11, 13, 10, 30, 0, 0, time.UTC) // Wednesday
	authCtx := &AuthContext{
		User: "user123",
		Now:  testTime,
	}

	tests := []struct {
		expression string
		expected   bool
	}{
		{"time_between('09:00', '17:00')", true},
		{"time_between('18:00', '08:00')", false}, // Night shift
		{"hour_between('9', '17')", true},
		{"hour_between('18', '8')", false},
		{"day_of_week('Wednesday')", true},
		{"day_of_week('Monday')", false},
		{"day_of_week('Wed')", true},
	}

	for _, test := range tests {
		result := evaluator.EvaluateAllowIf(context.Background(), authCtx, test.expression)
		if result != test.expected {
			t.Errorf("Expression %s: expected %v, got %v", test.expression, test.expected, result)
		}
	}
}

func TestEnhancedEvaluator_ComplexExpressions(t *testing.T) {
	policy := NewPolicy()
	policy.Grant("user123", "item:read")
	evaluator := NewEnhancedEvaluator(policy)

	authCtx := &AuthContext{
		User:        "user123",
		Permissions: []string{"item:write"},
		Roles:       []string{"gm", "moderator"},
		Resource: map[string]any{
			"owner": "user123",
			"price": 100.0,
		},
		Request: map[string]any{
			"action": "update",
		},
		Now: time.Date(2024, 11, 13, 14, 30, 0, 0, time.UTC), // Wednesday 2:30 PM
	}

	tests := []struct {
		expression string
		expected   bool
	}{
		// AND conditions
		{"has_permission('item:read') && is_owner()", true},
		{"has_permission('item:delete') && is_owner()", false},

		// OR conditions
		{"has_permission('item:delete') || is_owner()", true},
		{"has_permission('item:delete') || has_role('admin')", false},

		// Complex combinations
		{"(has_role('gm') || has_role('admin')) && time_between('09:00', '17:00')", true},
		{"has_permission('item:write') && resource.price > 50 && day_of_week('Wednesday')", true},

		// Negation
		{"!has_role('admin')", true},
		{"!has_permission('item:read')", false},

		// Mixed conditions
		{"has_role('gm') && (resource.price < 200 || is_owner())", true},
	}

	for _, test := range tests {
		result := evaluator.EvaluateAllowIf(context.Background(), authCtx, test.expression)
		if result != test.expected {
			t.Errorf("Expression %s: expected %v, got %v", test.expression, test.expected, result)
		}
	}
}

func TestEnhancedEvaluator_EdgeCases(t *testing.T) {
	policy := NewPolicy()
	evaluator := NewEnhancedEvaluator(policy)

	authCtx := &AuthContext{
		User: "user123",
		Now:  time.Now(),
	}

	tests := []struct {
		expression string
		expected   bool
	}{
		{"", true},                           // Empty expression should allow
		{"true", true},                       // Literal true
		{"false", false},                     // Literal false
		{"invalid_function()", false},        // Unknown function
		{"has_permission()", false},          // Missing argument
		{"resource.nonexistent", false},      // Nonexistent field
	}

	for _, test := range tests {
		result := evaluator.EvaluateAllowIf(context.Background(), authCtx, test.expression)
		if result != test.expected {
			t.Errorf("Expression %s: expected %v, got %v", test.expression, test.expected, result)
		}
	}
}