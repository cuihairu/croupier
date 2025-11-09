package rbac

import (
	"context"
	"fmt"
	"time"
)

// AuthDescriptor represents the auth configuration from function descriptors
type AuthDescriptor struct {
	Permission    string               `json:"permission"`
	AllowIf       string               `json:"allow_if"`
	Risk          *RiskPolicy          `json:"risk"`
	TwoPersonRule *TwoPersonRulePolicy `json:"two_person_rule"`
}

// RiskPolicy defines risk-based authorization requirements
type RiskPolicy struct {
	Level       string   `json:"level"`        // low, medium, high, critical
	RequiresMFA bool     `json:"requires_mfa"` // Multi-factor authentication
	TimeWindow  string   `json:"time_window"`  // e.g., "business_hours"
	Conditions  []string `json:"conditions"`   // Additional risk conditions
}

// TwoPersonRulePolicy defines two-person authorization requirements
type TwoPersonRulePolicy struct {
	Required   bool     `json:"required"`
	Approvers  []string `json:"approvers"`   // List of required approver roles
	Threshold  int      `json:"threshold"`   // Minimum number of approvals needed
	ExpiryTime string   `json:"expiry_time"` // How long approval is valid
	Conditions []string `json:"conditions"`  // When two-person rule applies
}

// AuthorizationRequest represents a request for authorization
type AuthorizationRequest struct {
	User        string         `json:"user"`
	Function    string         `json:"function"`
	Parameters  map[string]any `json:"parameters"`
	Context     map[string]any `json:"context"`
	Approvals   []Approval     `json:"approvals"`
	RequestTime time.Time      `json:"request_time"`
}

// Approval represents an approval from another user
type Approval struct {
	ApproverID   string    `json:"approver_id"`
	ApproverRole string    `json:"approver_role"`
	Timestamp    time.Time `json:"timestamp"`
	Signature    string    `json:"signature"` // Optional cryptographic signature
}

// AuthorizationResult represents the result of authorization check
type AuthorizationResult struct {
	Allowed           bool       `json:"allowed"`
	RequiresApproval  bool       `json:"requires_approval"`
	RequiresMFA       bool       `json:"requires_mfa"`
	RiskLevel         string     `json:"risk_level"`
	Reason            string     `json:"reason"`
	RequiredApprovals int        `json:"required_approvals"`
	ExistingApprovals int        `json:"existing_approvals"`
	Conditions        []string   `json:"conditions"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
}

// UnifiedPolicyEngine provides centralized authorization decisions
type UnifiedPolicyEngine struct {
	evaluator      *EnhancedEvaluator
	riskAssessment *RiskAssessment
}

// RiskAssessment provides risk evaluation capabilities
type RiskAssessment struct {
	// Risk evaluation functions can be plugged in here
}

func NewUnifiedPolicyEngine(policy *Policy) *UnifiedPolicyEngine {
	return &UnifiedPolicyEngine{
		evaluator:      NewEnhancedEvaluator(policy),
		riskAssessment: &RiskAssessment{},
	}
}

// Authorize performs comprehensive authorization check based on descriptor
func (upe *UnifiedPolicyEngine) Authorize(ctx context.Context, authDesc *AuthDescriptor, request *AuthorizationRequest) (*AuthorizationResult, error) {
	result := &AuthorizationResult{
		Allowed:           false,
		RequiresApproval:  false,
		RequiresMFA:       false,
		RiskLevel:         "low",
		ExistingApprovals: len(request.Approvals),
	}

	// Build auth context
	authCtx := &AuthContext{
		User:     request.User,
		Resource: request.Parameters,
		Request:  request.Context,
		Now:      request.RequestTime,
	}

	// Step 1: Basic permission check
	if authDesc.Permission != "" {
		if !upe.evaluator.policy.Can(request.User, authDesc.Permission) {
			result.Reason = fmt.Sprintf("Missing required permission: %s", authDesc.Permission)
			return result, nil
		}
	}

	// Step 2: Evaluate allow_if conditions
	if authDesc.AllowIf != "" {
		if !upe.evaluator.EvaluateAllowIf(ctx, authCtx, authDesc.AllowIf) {
			result.Reason = "Access denied by allow_if condition"
			return result, nil
		}
	}

	// Step 3: Risk assessment
	if authDesc.Risk != nil {
		riskResult := upe.assessRisk(ctx, authDesc.Risk, request)
		result.RiskLevel = riskResult.Level
		result.RequiresMFA = riskResult.RequiresMFA

		if !riskResult.Allowed {
			result.Reason = riskResult.Reason
			return result, nil
		}

		// Apply risk-based conditions
		result.Conditions = append(result.Conditions, riskResult.Conditions...)
	}

	// Step 4: Two-person rule evaluation
	if authDesc.TwoPersonRule != nil && authDesc.TwoPersonRule.Required {
		twoPersonResult := upe.evaluateTwoPersonRule(ctx, authDesc.TwoPersonRule, request)
		result.RequiresApproval = twoPersonResult.RequiresApproval
		result.RequiredApprovals = twoPersonResult.RequiredApprovals

		if twoPersonResult.RequiresApproval {
			if result.ExistingApprovals < result.RequiredApprovals {
				result.Reason = fmt.Sprintf("Requires %d approval(s), have %d",
					result.RequiredApprovals, result.ExistingApprovals)
				return result, nil
			}

			// Validate existing approvals
			if !upe.validateApprovals(ctx, authDesc.TwoPersonRule, request.Approvals, request.RequestTime) {
				result.Reason = "Invalid or expired approvals"
				return result, nil
			}
		}

		// Set expiry time if specified
		if authDesc.TwoPersonRule.ExpiryTime != "" {
			if expiry := upe.parseExpiryTime(authDesc.TwoPersonRule.ExpiryTime, request.RequestTime); expiry != nil {
				result.ExpiresAt = expiry
			}
		}
	}

	// If we get here, authorization is granted
	result.Allowed = true
	result.Reason = "Authorization granted"

	return result, nil
}

type RiskAssessmentResult struct {
	Allowed     bool
	Level       string
	RequiresMFA bool
	Reason      string
	Conditions  []string
}

func (upe *UnifiedPolicyEngine) assessRisk(ctx context.Context, riskPolicy *RiskPolicy, request *AuthorizationRequest) *RiskAssessmentResult {
	result := &RiskAssessmentResult{
		Allowed:     true,
		Level:       riskPolicy.Level,
		RequiresMFA: riskPolicy.RequiresMFA,
	}

	// Evaluate risk level constraints
	switch riskPolicy.Level {
	case "critical":
		// Critical operations may have additional restrictions
		result.RequiresMFA = true
		result.Conditions = append(result.Conditions, "critical_operation_logged")

	case "high":
		// High risk operations during business hours only
		if riskPolicy.TimeWindow == "business_hours" {
			if !upe.isBusinessHours(request.RequestTime) {
				result.Allowed = false
				result.Reason = "High-risk operations only allowed during business hours"
				return result
			}
		}

	case "medium":
		// Medium risk may require additional validation
		result.Conditions = append(result.Conditions, "audit_logged")

	case "low":
		// Low risk, minimal constraints
	}

	// Evaluate custom risk conditions
	for _, condition := range riskPolicy.Conditions {
		authCtx := &AuthContext{
			User:     request.User,
			Resource: request.Parameters,
			Request:  request.Context,
			Now:      request.RequestTime,
		}

		if !upe.evaluator.EvaluateAllowIf(ctx, authCtx, condition) {
			result.Allowed = false
			result.Reason = fmt.Sprintf("Risk condition failed: %s", condition)
			return result
		}
	}

	return result
}

type TwoPersonRuleResult struct {
	RequiresApproval  bool
	RequiredApprovals int
}

func (upe *UnifiedPolicyEngine) evaluateTwoPersonRule(ctx context.Context, twoPersonRule *TwoPersonRulePolicy, request *AuthorizationRequest) *TwoPersonRuleResult {
	result := &TwoPersonRuleResult{
		RequiresApproval:  true,
		RequiredApprovals: twoPersonRule.Threshold,
	}

	if twoPersonRule.Threshold <= 0 {
		result.RequiredApprovals = 1 // Default to 1 approval
	}

	// Check if two-person rule conditions apply
	if len(twoPersonRule.Conditions) > 0 {
		authCtx := &AuthContext{
			User:     request.User,
			Resource: request.Parameters,
			Request:  request.Context,
			Now:      request.RequestTime,
		}

		conditionsMet := false
		for _, condition := range twoPersonRule.Conditions {
			if upe.evaluator.EvaluateAllowIf(ctx, authCtx, condition) {
				conditionsMet = true
				break
			}
		}

		if !conditionsMet {
			result.RequiresApproval = false
		}
	}

	return result
}

func (upe *UnifiedPolicyEngine) validateApprovals(ctx context.Context, twoPersonRule *TwoPersonRulePolicy, approvals []Approval, requestTime time.Time) bool {
	if len(approvals) == 0 {
		return false
	}

	validApprovals := 0
	expiryDuration := upe.parseExpiryDuration(twoPersonRule.ExpiryTime)

	for _, approval := range approvals {
		// Check if approval is not expired
		if expiryDuration > 0 {
			if requestTime.Sub(approval.Timestamp) > expiryDuration {
				continue // Expired approval
			}
		}

		// Check if approver has required role
		if len(twoPersonRule.Approvers) > 0 {
			hasRequiredRole := false
			for _, requiredRole := range twoPersonRule.Approvers {
				if approval.ApproverRole == requiredRole {
					hasRequiredRole = true
					break
				}
			}
			if !hasRequiredRole {
				continue // Approver doesn't have required role
			}
		}

		validApprovals++
	}

	return validApprovals >= twoPersonRule.Threshold
}

func (upe *UnifiedPolicyEngine) isBusinessHours(t time.Time) bool {
	// Business hours: Monday-Friday, 9 AM - 5 PM
	weekday := t.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	hour := t.Hour()
	return hour >= 9 && hour < 17
}

func (upe *UnifiedPolicyEngine) parseExpiryTime(expiryTime string, requestTime time.Time) *time.Time {
	duration := upe.parseExpiryDuration(expiryTime)
	if duration > 0 {
		expiry := requestTime.Add(duration)
		return &expiry
	}
	return nil
}

func (upe *UnifiedPolicyEngine) parseExpiryDuration(expiryTime string) time.Duration {
	switch expiryTime {
	case "1h", "1 hour":
		return time.Hour
	case "24h", "1 day":
		return 24 * time.Hour
	case "1w", "1 week":
		return 7 * 24 * time.Hour
	default:
		if duration, err := time.ParseDuration(expiryTime); err == nil {
			return duration
		}
		return 0
	}
}
