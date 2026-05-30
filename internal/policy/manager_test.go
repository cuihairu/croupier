package policy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.FunctionPolicy{})
	require.NoError(t, err)
	return db
}

func TestNewManager(t *testing.T) {
	db := setupTestDB(t)

	// Test with non-existent config path
	m, err := NewManager(db, "/nonexistent/path/default-policies.yaml")
	assert.NoError(t, err)
	assert.NotNil(t, m)

	// Should have default policies
	assert.NotNil(t, m.config)
	assert.Equal(t, false, m.config.Low.RequireApproval)
	assert.Equal(t, true, m.config.High.RequireApproval)
	assert.Equal(t, true, m.config.Danger.RequireApproval)
}

func TestGetDefaultPolicy(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)

	tests := []struct {
		name      string
		riskLevel RiskLevel
		want      *Policy
	}{
		{
			name:      "low risk",
			riskLevel: RiskLow,
			want: &Policy{
				RequireApproval:  false,
				ApprovalWorkflow: "",
				RequireAudit:     false,
				AllowedRoles:     []string{"user", "operator"},
				Source:           "default",
				IsOverride:       false,
				DefaultRiskLevel: "low",
			},
		},
		{
			name:      "medium risk",
			riskLevel: RiskMedium,
			want: &Policy{
				RequireApproval:  false,
				ApprovalWorkflow: "",
				RequireAudit:     true,
				AllowedRoles:     []string{"operator"},
				Source:           "default",
				IsOverride:       false,
				DefaultRiskLevel: "medium",
			},
		},
		{
			name:      "high risk",
			riskLevel: RiskHigh,
			want: &Policy{
				RequireApproval:  true,
				ApprovalWorkflow: "single_admin",
				RequireAudit:     true,
				AllowedRoles:     []string{"admin"},
				Source:           "default",
				IsOverride:       false,
				DefaultRiskLevel: "high",
			},
		},
		{
			name:      "danger risk",
			riskLevel: RiskDanger,
			want: &Policy{
				RequireApproval:  true,
				ApprovalWorkflow: "two_person",
				RequireAudit:     true,
				AllowedRoles:     []string{"super_admin"},
				Source:           "default",
				IsOverride:       false,
				DefaultRiskLevel: "danger",
			},
		},
		{
			name:      "unknown risk defaults to medium",
			riskLevel: RiskUnknown,
			want: &Policy{
				RequireApproval:  false,
				ApprovalWorkflow: "",
				RequireAudit:     true,
				AllowedRoles:     []string{"operator"},
				Source:           "default",
				IsOverride:       false,
				DefaultRiskLevel: "medium",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.GetDefaultPolicy(tt.riskLevel)
			assert.Equal(t, tt.want.RequireApproval, got.RequireApproval)
			assert.Equal(t, tt.want.ApprovalWorkflow, got.ApprovalWorkflow)
			assert.Equal(t, tt.want.RequireAudit, got.RequireAudit)
			assert.Equal(t, tt.want.AllowedRoles, got.AllowedRoles)
			assert.Equal(t, tt.want.Source, got.Source)
			assert.Equal(t, tt.want.IsOverride, got.IsOverride)
		})
	}
}

func TestGetPolicy_WithOverride(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Set an override policy
	override := &Policy{
		FunctionID:       "test.function",
		RequireApproval:  true,
		ApprovalWorkflow: "custom_workflow",
		RequireAudit:     true,
		AllowedRoles:     []string{"custom_role"},
	}
	err = m.SetOverride(ctx, "test.function", override)
	require.NoError(t, err)

	// Get policy should return override
	got, err := m.GetPolicy(ctx, "test.function", RiskLow)
	require.NoError(t, err)
	assert.True(t, got.IsOverride)
	assert.Equal(t, "manual", got.Source)
	assert.Equal(t, true, got.RequireApproval)
	assert.Equal(t, "custom_workflow", got.ApprovalWorkflow)
	assert.Equal(t, []string{"custom_role"}, got.AllowedRoles)
}

func TestGetPolicy_NoOverride(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// No override set, should return default
	got, err := m.GetPolicy(ctx, "no.override.function", RiskHigh)
	require.NoError(t, err)
	assert.False(t, got.IsOverride)
	assert.Equal(t, "default", got.Source)
	assert.Equal(t, true, got.RequireApproval)
	assert.Equal(t, "single_admin", got.ApprovalWorkflow)
}

func TestSetOverride(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	policy := &Policy{
		FunctionID:       "test.function",
		RequireApproval:  true,
		ApprovalWorkflow: "test_workflow",
		RequireAudit:     true,
		AllowedRoles:     []string{"role1", "role2"},
	}

	err = m.SetOverride(ctx, "test.function", policy)
	require.NoError(t, err)

	// Verify in database
	var dbPolicy model.FunctionPolicy
	err = db.Where("function_id = ?", "test.function").First(&dbPolicy).Error
	require.NoError(t, err)
	assert.Equal(t, "test.function", dbPolicy.FunctionID)
	assert.Equal(t, true, dbPolicy.RequireApproval)
	assert.Equal(t, "test_workflow", dbPolicy.ApprovalWorkflow)
	assert.Equal(t, "manual", dbPolicy.Source)
}

func TestSetOverride_Update(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Create initial override
	policy1 := &Policy{
		FunctionID:       "test.function",
		RequireApproval:  true,
		ApprovalWorkflow: "workflow1",
		RequireAudit:     true,
		AllowedRoles:     []string{"role1"},
	}
	err = m.SetOverride(ctx, "test.function", policy1)
	require.NoError(t, err)

	// Update with new values
	policy2 := &Policy{
		FunctionID:       "test.function",
		RequireApproval:  false,
		ApprovalWorkflow: "workflow2",
		RequireAudit:     false,
		AllowedRoles:     []string{"role2"},
	}
	err = m.SetOverride(ctx, "test.function", policy2)
	require.NoError(t, err)

	// Verify updated values
	got, err := m.GetPolicy(ctx, "test.function", RiskLow)
	require.NoError(t, err)
	assert.Equal(t, false, got.RequireApproval)
	assert.Equal(t, "workflow2", got.ApprovalWorkflow)
	assert.Equal(t, []string{"role2"}, got.AllowedRoles)
}

func TestDeleteOverride(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Set an override
	policy := &Policy{
		FunctionID:      "test.function",
		RequireApproval: true,
		AllowedRoles:    []string{"role1"},
	}
	err = m.SetOverride(ctx, "test.function", policy)
	require.NoError(t, err)

	// Verify it exists
	var count int64
	db.Model(&model.FunctionPolicy{}).Where("function_id = ?", "test.function").Count(&count)
	assert.Equal(t, int64(1), count)

	// Delete override
	err = m.DeleteOverride(ctx, "test.function")
	require.NoError(t, err)

	// Verify it's gone
	db.Model(&model.FunctionPolicy{}).Where("function_id = ?", "test.function").Count(&count)
	assert.Equal(t, int64(0), count)

	// GetPolicy should now return default
	got, err := m.GetPolicy(ctx, "test.function", RiskLow)
	require.NoError(t, err)
	assert.False(t, got.IsOverride)
	assert.Equal(t, "default", got.Source)
}

func TestEnsureDefaultPolicy(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Create default policy with unique ID
	err = m.EnsureDefaultPolicy(ctx, "ensure.default.function", RiskHigh)
	require.NoError(t, err)

	// Verify it exists
	var dbPolicy model.FunctionPolicy
	err = db.Where("function_id = ?", "ensure.default.function").First(&dbPolicy).Error
	require.NoError(t, err)
	assert.Equal(t, "ensure.default.function", dbPolicy.FunctionID)
	assert.Equal(t, "default", dbPolicy.Source)
	assert.Equal(t, true, dbPolicy.RequireApproval)
	assert.Equal(t, "single_admin", dbPolicy.ApprovalWorkflow)

	// Call again should not error (already exists)
	err = m.EnsureDefaultPolicy(ctx, "ensure.default.function", RiskHigh)
	require.NoError(t, err)

	// Should still have same values
	var dbPolicy2 model.FunctionPolicy
	err = db.Where("function_id = ?", "ensure.default.function").First(&dbPolicy2).Error
	require.NoError(t, err)
	assert.Equal(t, dbPolicy.RequireApproval, dbPolicy2.RequireApproval)
}

func TestListOverrides(t *testing.T) {
	db := setupTestDB(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Set some overrides
	policies := []*Policy{
		{FunctionID: "func1", RequireApproval: true, AllowedRoles: []string{"admin"}},
		{FunctionID: "func2", RequireApproval: false, AllowedRoles: []string{"user"}},
		{FunctionID: "func3", RequireApproval: true, AllowedRoles: []string{"super_admin"}},
	}
	for _, p := range policies {
		err = m.SetOverride(ctx, p.FunctionID, p)
		require.NoError(t, err)
	}

	// Also create a default policy (should not be listed)
	err = m.EnsureDefaultPolicy(ctx, "func_default", RiskLow)
	require.NoError(t, err)

	// List overrides
	got, err := m.ListOverrides(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 3)

	functionIDs := make(map[string]bool)
	for _, p := range got {
		functionIDs[p.FunctionID] = true
		assert.True(t, p.IsOverride)
		assert.Equal(t, "manual", p.Source)
	}

	assert.True(t, functionIDs["func1"])
	assert.True(t, functionIDs["func2"])
	assert.True(t, functionIDs["func3"])
	assert.False(t, functionIDs["func_default"])
}

func TestGetPolicy_DBError(t *testing.T) {
	// Use a closed DB to simulate error
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	// Get underlying SQL DB and close it
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.Close()

	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Should return error when DB fails
	_, err = m.GetPolicy(ctx, "test.function", RiskLow)
	assert.Error(t, err)
}
