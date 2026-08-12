// Package policy provides function policy management with default risk-based policies and database overrides.
package policy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cuihairu/croupier/internal/model"
)

// Policy represents the complete security policy for a function.
type Policy struct {
	FunctionID       string   `json:"functionId"`
	RequireApproval  bool     `json:"requireApproval"`
	ApprovalWorkflow string   `json:"approvalWorkflow"`
	RequireAudit     bool     `json:"requireAudit"`
	AllowedRoles     []string `json:"allowedRoles"`
	Source           string   `json:"source"`                     // 'default' or 'manual'
	IsOverride       bool     `json:"isOverride"`                 // true if overrides default risk policy
	DefaultRiskLevel string   `json:"defaultRiskLevel,omitempty"` // the risk level for default policy
}

// RiskLevel represents the risk level of a function.
type RiskLevel string

const (
	RiskLow     RiskLevel = "low"
	RiskMedium  RiskLevel = "medium"
	RiskHigh    RiskLevel = "high"
	RiskDanger  RiskLevel = "danger"
	RiskUnknown RiskLevel = "unknown"
)

// DefaultPolicyConfig defines the default policy for each risk level.
type DefaultPolicyConfig struct {
	Low    RiskPolicy `yaml:"low"`
	Medium RiskPolicy `yaml:"medium"`
	High   RiskPolicy `yaml:"high"`
	Danger RiskPolicy `yaml:"danger"`
}

// RiskPolicy defines the policy for a specific risk level.
type RiskPolicy struct {
	RequireApproval  bool     `yaml:"require_approval"`
	ApprovalWorkflow string   `yaml:"approval_workflow"`
	RequireAudit     bool     `yaml:"require_audit"`
	AllowedRoles     []string `yaml:"allowed_roles"`
	Description      string   `yaml:"description"`
}

// Manager manages function policies with default risk-based policies and database overrides.
type Manager struct {
	db         *gorm.DB
	config     *DefaultPolicyConfig
	configPath string
	mu         sync.RWMutex
}

// NewManager creates a new policy manager.
func NewManager(db *gorm.DB, configPath string) (*Manager, error) {
	m := &Manager{
		db:         db,
		configPath: configPath,
	}

	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	return m, nil
}

// loadConfig loads the default policy configuration from YAML file.
func (m *Manager) loadConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try multiple config paths
	paths := []string{
		m.configPath,
		filepath.Join("configs", "default-policies.yaml"),
		filepath.Join("..", "..", "configs", "default-policies.yaml"),
	}

	var configData []byte
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			configData = data
			break
		}
	}

	if len(configData) == 0 {
		// Return empty config if file not found (will use hardcoded defaults)
		m.config = &DefaultPolicyConfig{
			Low: RiskPolicy{
				RequireApproval: false,
				RequireAudit:    false,
				AllowedRoles:    []string{"user", "operator"},
			},
			Medium: RiskPolicy{
				RequireApproval: false,
				RequireAudit:    true,
				AllowedRoles:    []string{"operator"},
			},
			High: RiskPolicy{
				RequireApproval:  true,
				ApprovalWorkflow: "single_admin",
				RequireAudit:     true,
				AllowedRoles:     []string{"admin"},
			},
			Danger: RiskPolicy{
				RequireApproval:  true,
				ApprovalWorkflow: "two_person",
				RequireAudit:     true,
				AllowedRoles:     []string{"super_admin"},
			},
		}
		return nil
	}

	var config DefaultPolicyConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return err
	}

	m.config = &config
	return nil
}

// ReloadConfig reloads the default policy configuration.
func (m *Manager) ReloadConfig() error {
	return m.loadConfig()
}

// GetPolicy returns the effective policy for a function.
// It checks for database override first, then falls back to default risk-based policy.
func (m *Manager) GetPolicy(ctx context.Context, functionID string, riskLevel RiskLevel) (*Policy, error) {
	// Check for database override (manual)
	var dbPolicy model.FunctionPolicy
	err := m.db.WithContext(ctx).
		Where("function_id = ? AND source = ?", functionID, "manual").
		First(&dbPolicy).Error

	if err == nil {
		// Manual override exists
		return m.dbPolicyToPolicy(&dbPolicy, true, ""), nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Check for default policy in database
	err = m.db.WithContext(ctx).
		Where("function_id = ? AND source = ?", functionID, "default").
		First(&dbPolicy).Error

	if err == nil {
		// Default policy exists in database
		return m.dbPolicyToPolicy(&dbPolicy, false, string(riskLevel)), nil
	}

	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// No policy in database, return default policy for risk level
	return m.GetDefaultPolicy(riskLevel), nil
}

// GetDefaultPolicy returns the default policy for a given risk level.
func (m *Manager) GetDefaultPolicy(riskLevel RiskLevel) *Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var riskPolicy RiskPolicy
	switch riskLevel {
	case RiskLow:
		riskPolicy = m.config.Low
	case RiskMedium:
		riskPolicy = m.config.Medium
	case RiskHigh:
		riskPolicy = m.config.High
	case RiskDanger:
		riskPolicy = m.config.Danger
	default:
		riskPolicy = m.config.Medium // Default to medium
	}

	return &Policy{
		RequireApproval:  riskPolicy.RequireApproval,
		ApprovalWorkflow: riskPolicy.ApprovalWorkflow,
		RequireAudit:     riskPolicy.RequireAudit,
		AllowedRoles:     riskPolicy.AllowedRoles,
		Source:           "default",
		IsOverride:       false,
		DefaultRiskLevel: string(riskLevel),
	}
}

// SetOverride sets or updates a database override policy for a function.
func (m *Manager) SetOverride(ctx context.Context, functionID string, policy *Policy) error {
	var dbPolicy model.FunctionPolicy
	dbPolicy.FunctionID = functionID
	dbPolicy.RequireApproval = policy.RequireApproval
	dbPolicy.ApprovalWorkflow = policy.ApprovalWorkflow
	dbPolicy.RequireAudit = policy.RequireAudit

	rolesJSON, err := json.Marshal(policy.AllowedRoles)
	if err != nil {
		return err
	}
	dbPolicy.AllowedRoles = rolesJSON
	dbPolicy.Source = "manual"

	// Upsert
	return m.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "function_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"require_approval", "approval_workflow", "require_audit", "allowed_roles", "source"}),
		}).
		Create(&dbPolicy).Error
}

// DeleteOverride removes the database override for a function.
// After deletion, the function will use the default risk-based policy.
func (m *Manager) DeleteOverride(ctx context.Context, functionID string) error {
	return m.db.WithContext(ctx).
		Where("function_id = ?", functionID).
		Delete(&model.FunctionPolicy{}).Error
}

// EnsureDefaultPolicy creates a default policy for a function if no override exists.
// This should be called when a new function is registered.
// Uses upsert to handle concurrent calls safely.
func (m *Manager) EnsureDefaultPolicy(ctx context.Context, functionID string, riskLevel RiskLevel) error {
	// Create default policy
	defaultPolicy := m.GetDefaultPolicy(riskLevel)

	var dbPolicy model.FunctionPolicy
	dbPolicy.FunctionID = functionID
	dbPolicy.RequireApproval = defaultPolicy.RequireApproval
	dbPolicy.ApprovalWorkflow = defaultPolicy.ApprovalWorkflow
	dbPolicy.RequireAudit = defaultPolicy.RequireAudit

	rolesJSON, err := json.Marshal(defaultPolicy.AllowedRoles)
	if err != nil {
		return err
	}
	dbPolicy.AllowedRoles = rolesJSON
	dbPolicy.Source = "default"

	// Use upsert to avoid duplicate key errors
	return m.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "function_id"}},
			DoNothing: true, // Don't overwrite existing policies
		}).
		Create(&dbPolicy).Error
}

// dbPolicyToPolicy converts a database model to Policy.
func (m *Manager) dbPolicyToPolicy(dbPolicy *model.FunctionPolicy, isOverride bool, defaultRiskLevel string) *Policy {
	var allowedRoles []string
	if dbPolicy.AllowedRoles != nil {
		json.Unmarshal(dbPolicy.AllowedRoles, &allowedRoles)
	}

	return &Policy{
		FunctionID:       dbPolicy.FunctionID,
		RequireApproval:  dbPolicy.RequireApproval,
		ApprovalWorkflow: dbPolicy.ApprovalWorkflow,
		RequireAudit:     dbPolicy.RequireAudit,
		AllowedRoles:     allowedRoles,
		Source:           dbPolicy.Source,
		IsOverride:       isOverride,
		DefaultRiskLevel: defaultRiskLevel,
	}
}

// ListOverrides returns all manual policy overrides.
func (m *Manager) ListOverrides(ctx context.Context) ([]*Policy, error) {
	var dbPolicies []model.FunctionPolicy
	err := m.db.WithContext(ctx).
		Where("source = ?", "manual").
		Find(&dbPolicies).Error
	if err != nil {
		return nil, err
	}

	policies := make([]*Policy, len(dbPolicies))
	for i, dp := range dbPolicies {
		policies[i] = m.dbPolicyToPolicy(&dp, true, "")
	}
	return policies, nil
}
