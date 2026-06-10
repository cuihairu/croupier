package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
)

func setupTestDBExtra(t *testing.T) *gorm.DB {
	// Use unique database name for each test to avoid sharing
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(&model.FunctionPolicy{})
	require.NoError(t, err)
	return db
}

func TestNewManager_WithValidConfigPath(t *testing.T) {
	// Create a temporary config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "policies.yaml")

	configContent := `
low:
  require_approval: false
  require_audit: false
  allowed_roles: ["user"]
medium:
  require_approval: false
  require_audit: true
  allowed_roles: ["operator"]
high:
  require_approval: true
  approval_workflow: "single_admin"
  require_audit: true
  allowed_roles: ["admin"]
danger:
  require_approval: true
  approval_workflow: "two_person"
  require_audit: true
  allowed_roles: ["super_admin"]
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	db := setupTestDBExtra(t)
	m, err := NewManager(db, configPath)
	require.NoError(t, err)
	assert.NotNil(t, m)

	// Verify config was loaded
	assert.Equal(t, []string{"user"}, m.config.Low.AllowedRoles)
	assert.Equal(t, []string{"operator"}, m.config.Medium.AllowedRoles)
}

func TestNewManager_InvalidConfigPath(t *testing.T) {
	db := setupTestDBExtra(t)

	// Create a file with invalid YAML
	dir := t.TempDir()
	configPath := filepath.Join(dir, "invalid.yaml")
	err := os.WriteFile(configPath, []byte("invalid: [yaml: content"), 0o644)
	require.NoError(t, err)

	_, err = NewManager(db, configPath)
	assert.Error(t, err)
}

func TestReloadConfig(t *testing.T) {
	// Create a temporary config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "policies.yaml")

	configContent := `
low:
  require_approval: false
  require_audit: false
  allowed_roles: ["user"]
medium:
  require_approval: false
  require_audit: true
  allowed_roles: ["operator"]
high:
  require_approval: true
  approval_workflow: "single_admin"
  require_audit: true
  allowed_roles: ["admin"]
danger:
  require_approval: true
  approval_workflow: "two_person"
  require_audit: true
  allowed_roles: ["super_admin"]
`
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	db := setupTestDBExtra(t)
	m, err := NewManager(db, configPath)
	require.NoError(t, err)

	// Modify the config file
	newConfigContent := `
low:
  require_approval: false
  require_audit: false
  allowed_roles: ["viewer"]
medium:
  require_approval: false
  require_audit: true
  allowed_roles: ["operator"]
high:
  require_approval: true
  approval_workflow: "single_admin"
  require_audit: true
  allowed_roles: ["admin"]
danger:
  require_approval: true
  approval_workflow: "two_person"
  require_audit: true
  allowed_roles: ["super_admin"]
`
	err = os.WriteFile(configPath, []byte(newConfigContent), 0o644)
	require.NoError(t, err)

	// Reload config
	err = m.ReloadConfig()
	require.NoError(t, err)

	// Verify new config was loaded
	assert.Equal(t, []string{"viewer"}, m.config.Low.AllowedRoles)
}

func TestGetPolicy_WithDefaultInDB(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Create a default policy in DB
	err = m.EnsureDefaultPolicy(ctx, "test.default.function", RiskHigh)
	require.NoError(t, err)

	// Get policy should return the default from DB
	got, err := m.GetPolicy(ctx, "test.default.function", RiskLow)
	require.NoError(t, err)
	assert.False(t, got.IsOverride)
	assert.Equal(t, "default", got.Source)
	assert.Equal(t, true, got.RequireApproval)
	assert.Equal(t, "single_admin", got.ApprovalWorkflow)
}

func TestGetPolicy_ManualOverridePrecedence(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Create a default policy in DB
	err = m.EnsureDefaultPolicy(ctx, "test.precedence", RiskLow)
	require.NoError(t, err)

	// Create a manual override
	override := &Policy{
		FunctionID:       "test.precedence",
		RequireApproval:  true,
		ApprovalWorkflow: "custom",
		RequireAudit:     true,
		AllowedRoles:     []string{"custom_role"},
	}
	err = m.SetOverride(ctx, "test.precedence", override)
	require.NoError(t, err)

	// Get policy should return the manual override
	got, err := m.GetPolicy(ctx, "test.precedence", RiskLow)
	require.NoError(t, err)
	assert.True(t, got.IsOverride)
	assert.Equal(t, "manual", got.Source)
	assert.Equal(t, "custom", got.ApprovalWorkflow)
}

func TestSetOverride_EmptyAllowedRoles(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	policy := &Policy{
		FunctionID:      "test.empty.roles",
		RequireApproval: true,
		AllowedRoles:    []string{},
	}

	err = m.SetOverride(ctx, "test.empty.roles", policy)
	require.NoError(t, err)

	// Verify
	got, err := m.GetPolicy(ctx, "test.empty.roles", RiskLow)
	require.NoError(t, err)
	assert.Empty(t, got.AllowedRoles)
}

func TestDeleteOverride_NonExistent(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Delete non-existent override should not error
	err = m.DeleteOverride(ctx, "nonexistent.function")
	require.NoError(t, err)
}

func TestEnsureDefaultPolicy_DifferentRiskLevels(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Create default policies for different risk levels
	err = m.EnsureDefaultPolicy(ctx, "func.low", RiskLow)
	require.NoError(t, err)

	err = m.EnsureDefaultPolicy(ctx, "func.medium", RiskMedium)
	require.NoError(t, err)

	err = m.EnsureDefaultPolicy(ctx, "func.high", RiskHigh)
	require.NoError(t, err)

	err = m.EnsureDefaultPolicy(ctx, "func.danger", RiskDanger)
	require.NoError(t, err)

	// Verify each has correct settings
	var count int64
	db.Model(&model.FunctionPolicy{}).Count(&count)
	assert.Equal(t, int64(4), count)
}

func TestListOverrides_Empty(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// No overrides
	got, err := m.ListOverrides(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestListOverrides_OnlyManual(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)
	ctx := context.Background()

	// Create manual overrides
	for i := 0; i < 3; i++ {
		policy := &Policy{
			FunctionID:      "func" + string(rune('a'+i)),
			RequireApproval: true,
			AllowedRoles:    []string{"admin"},
		}
		err = m.SetOverride(ctx, policy.FunctionID, policy)
		require.NoError(t, err)
	}

	// Create default policies (should not be listed)
	for i := 0; i < 2; i++ {
		err = m.EnsureDefaultPolicy(ctx, "default.func"+string(rune('a'+i)), RiskLow)
		require.NoError(t, err)
	}

	// List overrides should only return manual ones
	got, err := m.ListOverrides(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 3)

	for _, p := range got {
		assert.Equal(t, "manual", p.Source)
		assert.True(t, p.IsOverride)
	}
}

func TestDbPolicyToPolicy_NilAllowedRoles(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)

	dbPolicy := &model.FunctionPolicy{
		FunctionID:       "test",
		RequireApproval:  true,
		ApprovalWorkflow: "workflow",
		RequireAudit:     true,
		AllowedRoles:     nil,
		Source:           "manual",
	}

	policy := m.dbPolicyToPolicy(dbPolicy, true, "")
	assert.Equal(t, "test", policy.FunctionID)
	assert.True(t, policy.RequireApproval)
	assert.Equal(t, "workflow", policy.ApprovalWorkflow)
	assert.True(t, policy.RequireAudit)
	assert.Nil(t, policy.AllowedRoles)
	assert.Equal(t, "manual", policy.Source)
	assert.True(t, policy.IsOverride)
}

func TestDbPolicyToPolicy_WithDefaultRiskLevel(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)

	dbPolicy := &model.FunctionPolicy{
		FunctionID:      "test",
		RequireApproval: false,
		AllowedRoles:    []byte(`["user"]`),
		Source:          "default",
	}

	policy := m.dbPolicyToPolicy(dbPolicy, false, "low")
	assert.Equal(t, "low", policy.DefaultRiskLevel)
	assert.False(t, policy.IsOverride)
}

func TestGetDefaultPolicy_AllRiskLevels(t *testing.T) {
	db := setupTestDBExtra(t)
	m, err := NewManager(db, "")
	require.NoError(t, err)

	tests := []struct {
		name      string
		riskLevel RiskLevel
		wantRoles []string
	}{
		{"low", RiskLow, []string{"user", "operator"}},
		{"medium", RiskMedium, []string{"operator"}},
		{"high", RiskHigh, []string{"admin"}},
		{"danger", RiskDanger, []string{"super_admin"}},
		{"unknown", RiskUnknown, []string{"operator"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.GetDefaultPolicy(tt.riskLevel)
			assert.Equal(t, tt.wantRoles, got.AllowedRoles)
			assert.Equal(t, "default", got.Source)
			assert.False(t, got.IsOverride)
		})
	}
}

func TestLoadConfig_FallbackPaths(t *testing.T) {
	// Test that loadConfig tries multiple paths
	db := setupTestDBExtra(t)
	m := &Manager{
		db:         db,
		configPath: "/nonexistent/path",
	}

	err := m.loadConfig()
	// Should succeed with hardcoded defaults
	assert.NoError(t, err)
	assert.NotNil(t, m.config)
	assert.Equal(t, []string{"user", "operator"}, m.config.Low.AllowedRoles)
}
