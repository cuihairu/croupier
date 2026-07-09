package approvals

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Delegation related errors
var (
	ErrDelegationNotFound     = errors.New("delegation not found")
	ErrDelegationExpired      = errors.New("delegation has expired")
	ErrDelegationNotActive    = errors.New("delegation is not active")
	ErrCannotDelegateToSelf   = errors.New("cannot delegate to yourself")
	ErrCircularDelegation     = errors.New("circular delegation detected")
	ErrDelegationNotPermitted = errors.New("delegation not permitted for this approval")
)

// DelegationState represents the state of a delegation
type DelegationState string

const (
	DelegationStateActive    DelegationState = "active"
	DelegationStateRevoked   DelegationState = "revoked"
	DelegationStateExpired   DelegationState = "expired"
	DelegationStateCompleted DelegationState = "completed"
)

// DelegationScope defines the scope of delegation authority
type DelegationScope string

const (
	ScopeAll      DelegationScope = "all"         // All approvals
	ScopeFunction DelegationScope = "function"    // Specific function
	ScopeGame     DelegationScope = "game"        // Specific game
	ScopeEnv      DelegationScope = "environment" // Specific environment
	ScopeWorkflow DelegationScope = "workflow"    // Specific workflow type
	ScopeAmount   DelegationScope = "amount"      // Up to certain amount/limit
)

// DelegationPermission defines what actions a delegate can perform
type DelegationPermission string

const (
	PermApprove  DelegationPermission = "approve"
	PermReject   DelegationPermission = "reject"
	PermView     DelegationPermission = "view"
	PermDelegate DelegationPermission = "delegate" // Can further delegate
)

// Delegation represents an approval delegation from one user to another
type Delegation struct {
	ID            string                 `json:"id"`
	Delegator     string                 `json:"delegator"` // User who delegates
	Delegate      string                 `json:"delegate"`  // User who receives delegation
	Scope         DelegationScope        `json:"scope"`
	ScopeValue    string                 `json:"scope_value"` // Value for the scope (e.g., function_id, game_id)
	Permissions   []DelegationPermission `json:"permissions"`
	State         DelegationState        `json:"state"`
	Reason        string                 `json:"reason"`
	StartAt       time.Time              `json:"start_at"`
	EndAt         *time.Time             `json:"end_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	RevokedAt     *time.Time             `json:"revoked_at,omitempty"`
	RevokedBy     string                 `json:"revoked_by,omitempty"`
	RevokedReason string                 `json:"revoked_reason,omitempty"`
	MaxUsages     int                    `json:"max_usages"`  // Maximum number of times this can be used (0 = unlimited)
	UsageCount    int                    `json:"usage_count"` // Current usage count
	Constraints   []DelegationConstraint `json:"constraints,omitempty"`
}

// DelegationConstraint defines additional constraints on delegations
type DelegationConstraint struct {
	Type     string      `json:"type"`     // e.g., "time_restriction", "amount_limit", "requires_mfa"
	Value    interface{} `json:"value"`    // Constraint-specific value
	Enforced bool        `json:"enforced"` // Whether the constraint is enforced
}

// TimeRestrictionConstraint value for time-based restrictions
type TimeRestrictionConstraint struct {
	AllowedDays  []int  `json:"allowed_days"`  // 0=Sunday, 1=Monday, etc.
	AllowedStart string `json:"allowed_start"` // e.g., "09:00"
	AllowedEnd   string `json:"allowed_end"`   // e.g., "17:00"
	Timezone     string `json:"timezone"`
}

// AmountLimitConstraint value for amount-based restrictions
type AmountLimitConstraint struct {
	MaxAmount  float64 `json:"max_amount"`
	Currency   string  `json:"currency"`
	PeriodDays int     `json:"period_days"` // Rolling period in days
}

// DelegationRequest represents a request to create a delegation
type DelegationRequest struct {
	Delegator   string                 `json:"delegator"`
	Delegate    string                 `json:"delegate"`
	Scope       DelegationScope        `json:"scope"`
	ScopeValue  string                 `json:"scope_value"`
	Permissions []DelegationPermission `json:"permissions"`
	Reason      string                 `json:"reason"`
	Duration    time.Duration          `json:"duration"`
	MaxUsages   int                    `json:"max_usages"`
	Constraints []DelegationConstraint `json:"constraints"`
}

// DelegationFilter for filtering delegations
type DelegationFilter struct {
	Delegator string
	Delegate  string
	Scope     DelegationScope
	State     DelegationState
}

// DelegationStore interface for delegation persistence
type DelegationStore interface {
	Create(delegation *Delegation) (*Delegation, error)
	Get(id string) (*Delegation, error)
	Update(delegation *Delegation) (*Delegation, error)
	Delete(id string) error
	List(filter DelegationFilter, page Page) ([]*Delegation, int, error)
	GetActiveDelegationsForUser(userID string) ([]*Delegation, error)
	GetActiveDelegationsByUser(userID string) ([]*Delegation, error)
	IncrementUsage(id string) error
}

// DelegationService manages approval delegations
type DelegationService struct {
	store         DelegationStore
	workflowStore WorkflowStore
	notifier      Notifier

	// approvalStore resolves live Approval records when evaluating delegation
	// scope. Optional for backward compatibility — when nil, getApproval
	// returns an explicit error so callers know scope cannot be enforced.
	approvalStore Store
	mu            sync.RWMutex
}

// NewDelegationService creates a new delegation service
func NewDelegationService(store DelegationStore, workflowStore WorkflowStore, notifier Notifier) *DelegationService {
	return &DelegationService{
		store:         store,
		workflowStore: workflowStore,
		notifier:      notifier,
	}
}

// SetApprovalStore wires the optional Approval store used to resolve scope
// when checking whether a delegate can act on a specific approval. Without
// it, CanDelegate returns an error rather than silently treating scope as
// "all-matches".
func (s *DelegationService) SetApprovalStore(store Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvalStore = store
}

// CreateDelegation creates a new delegation
func (s *DelegationService) CreateDelegation(ctx context.Context, req *DelegationRequest) (*Delegation, error) {
	// Validate request
	if err := s.validateDelegationRequest(req); err != nil {
		return nil, err
	}

	// Check for circular delegation
	if err := s.checkCircularDelegation(req.Delegator, req.Delegate); err != nil {
		return nil, err
	}

	now := time.Now()
	delegation := &Delegation{
		ID:          generateID("del"),
		Delegator:   req.Delegator,
		Delegate:    req.Delegate,
		Scope:       req.Scope,
		ScopeValue:  req.ScopeValue,
		Permissions: req.Permissions,
		State:       DelegationStateActive,
		Reason:      req.Reason,
		StartAt:     now,
		MaxUsages:   req.MaxUsages,
		UsageCount:  0,
		Constraints: req.Constraints,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Set end time if duration is specified
	if req.Duration > 0 {
		endAt := now.Add(req.Duration)
		delegation.EndAt = &endAt
	}

	created, err := s.store.Create(delegation)
	if err != nil {
		return nil, fmt.Errorf("failed to create delegation: %w", err)
	}

	// Notify the delegate
	if s.notifier != nil {
		s.notifier.Notify(ctx, []string{req.Delegate}, NotificationEvent{
			Type:    "delegation_created",
			Title:   "Approval Delegation Received",
			Message: fmt.Sprintf("You have received approval delegation from %s", req.Delegator),
			Data: map[string]interface{}{
				"delegation_id": created.ID,
				"scope":         req.Scope,
				"permissions":   req.Permissions,
			},
		})
	}

	return created, nil
}

// validateDelegationRequest validates a delegation request
func (s *DelegationService) validateDelegationRequest(req *DelegationRequest) error {
	if req.Delegator == "" {
		return errors.New("delegator is required")
	}
	if req.Delegate == "" {
		return errors.New("delegate is required")
	}
	if req.Delegator == req.Delegate {
		return ErrCannotDelegateToSelf
	}
	if len(req.Permissions) == 0 {
		return errors.New("at least one permission is required")
	}
	return nil
}

// checkCircularDelegation checks for circular delegation chains
func (s *DelegationService) checkCircularDelegation(delegator, delegate string) error {
	// Get all active delegations where the delegate is a delegator
	delegations, err := s.store.GetActiveDelegationsByUser(delegate)
	if err != nil {
		return nil // If we can't check, allow it
	}

	visited := make(map[string]bool)
	return s.checkCircularRecursive(delegator, delegations, visited)
}

func (s *DelegationService) checkCircularRecursive(target string, delegations []*Delegation, visited map[string]bool) error {
	for _, d := range delegations {
		if visited[d.ID] {
			continue
		}
		visited[d.ID] = true

		if d.Delegate == target {
			return ErrCircularDelegation
		}

		// Check further delegations
		subDelegations, err := s.store.GetActiveDelegationsByUser(d.Delegate)
		if err != nil {
			continue
		}
		if err := s.checkCircularRecursive(target, subDelegations, visited); err != nil {
			return err
		}
	}
	return nil
}

// RevokeDelegation revokes a delegation
func (s *DelegationService) RevokeDelegation(ctx context.Context, id, revokedBy, reason string) (*Delegation, error) {
	delegation, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}

	if delegation.State != DelegationStateActive {
		return nil, ErrDelegationNotActive
	}

	// Only the original delegator or admin can revoke
	if delegation.Delegator != revokedBy {
		// Check if revokedBy has admin rights (would integrate with RBAC)
		// For now, only allow delegator to revoke
		return nil, errors.New("only the delegator can revoke this delegation")
	}

	now := time.Now()
	delegation.State = DelegationStateRevoked
	delegation.RevokedAt = &now
	delegation.RevokedBy = revokedBy
	delegation.RevokedReason = reason
	delegation.UpdatedAt = now

	updated, err := s.store.Update(delegation)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke delegation: %w", err)
	}

	// Notify the delegate
	if s.notifier != nil {
		s.notifier.Notify(ctx, []string{delegation.Delegate}, NotificationEvent{
			Type:    "delegation_revoked",
			Title:   "Approval Delegation Revoked",
			Message: fmt.Sprintf("Your delegation from %s has been revoked", delegation.Delegator),
			Data: map[string]interface{}{
				"delegation_id": id,
				"reason":        reason,
			},
		})
	}

	return updated, nil
}

// GetActiveDelegation checks if a user has active delegation from another user
func (s *DelegationService) GetActiveDelegation(delegator, delegate string, scope DelegationScope, scopeValue string) (*Delegation, error) {
	delegations, err := s.store.GetActiveDelegationsForUser(delegate)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	for _, d := range delegations {
		if d.Delegator != delegator {
			continue
		}
		if d.State != DelegationStateActive {
			continue
		}
		if d.EndAt != nil && now.After(*d.EndAt) {
			continue
		}
		if d.MaxUsages > 0 && d.UsageCount >= d.MaxUsages {
			continue
		}

		// Check scope match
		if !s.matchesScope(d, scope, scopeValue) {
			continue
		}

		// Check constraints
		if !s.checkConstraints(d, now) {
			continue
		}

		return d, nil
	}

	return nil, ErrDelegationNotFound
}

// CanDelegate checks if a user can delegate for a specific approval
func (s *DelegationService) CanDelegate(userID, approvalID string, permission DelegationPermission) (bool, *Delegation, error) {
	// Get the approval to check scope
	approval, err := s.getApproval(approvalID)
	if err != nil {
		return false, nil, err
	}

	// Get active delegations for the user
	delegations, err := s.store.GetActiveDelegationsForUser(userID)
	if err != nil {
		return false, nil, err
	}

	now := time.Now()

	for _, d := range delegations {
		if d.State != DelegationStateActive {
			continue
		}
		if d.EndAt != nil && now.After(*d.EndAt) {
			continue
		}
		if d.MaxUsages > 0 && d.UsageCount >= d.MaxUsages {
			continue
		}

		// Check if user has the required permission
		if !s.hasPermission(d, permission) {
			continue
		}

		// Check if scope matches
		if !s.matchesApprovalScope(d, approval) {
			continue
		}

		// Check constraints
		if !s.checkConstraints(d, now) {
			continue
		}

		return true, d, nil
	}

	return false, nil, nil
}

// UseDelegation records usage of a delegation
func (s *DelegationService) UseDelegation(ctx context.Context, id string) error {
	delegation, err := s.store.Get(id)
	if err != nil {
		return err
	}

	if delegation.State != DelegationStateActive {
		return ErrDelegationNotActive
	}

	now := time.Now()
	if delegation.EndAt != nil && now.After(*delegation.EndAt) {
		return ErrDelegationExpired
	}

	if delegation.MaxUsages > 0 && delegation.UsageCount >= delegation.MaxUsages {
		return errors.New("delegation usage limit reached")
	}

	if err := s.store.IncrementUsage(id); err != nil {
		return fmt.Errorf("failed to increment usage: %w", err)
	}

	return nil
}

// matchesScope checks if a delegation matches the given scope
func (s *DelegationService) matchesScope(d *Delegation, scope DelegationScope, scopeValue string) bool {
	if d.Scope == ScopeAll {
		return true
	}
	if d.Scope != scope {
		return false
	}
	if d.ScopeValue == "" || d.ScopeValue == "*" {
		return true
	}
	return d.ScopeValue == scopeValue
}

// matchesApprovalScope checks if a delegation matches an approval's scope
func (s *DelegationService) matchesApprovalScope(d *Delegation, approval *Approval) bool {
	switch d.Scope {
	case ScopeAll:
		return true
	case ScopeFunction:
		return d.ScopeValue == "" || d.ScopeValue == approval.FunctionID
	case ScopeGame:
		return d.ScopeValue == "" || d.ScopeValue == approval.GameID
	case ScopeEnv:
		return d.ScopeValue == "" || d.ScopeValue == approval.Env
	default:
		return false
	}
}

// hasPermission checks if a delegation has a specific permission
func (s *DelegationService) hasPermission(d *Delegation, permission DelegationPermission) bool {
	for _, p := range d.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// checkConstraints checks if all constraints are satisfied
func (s *DelegationService) checkConstraints(d *Delegation, now time.Time) bool {
	for _, c := range d.Constraints {
		if !c.Enforced {
			continue
		}

		switch c.Type {
		case "time_restriction":
			if !s.checkTimeRestriction(c.Value, now) {
				return false
			}
			// Add more constraint types as needed
		}
	}
	return true
}

// checkTimeRestriction checks if the current time is within allowed hours
func (s *DelegationService) checkTimeRestriction(value interface{}, now time.Time) bool {
	data, ok := value.(map[string]interface{})
	if !ok {
		return true // If we can't parse, allow it
	}

	// Check allowed days
	if days, ok := data["allowed_days"].([]interface{}); ok {
		weekday := int(now.Weekday())
		dayAllowed := false
		for _, d := range days {
			if int(d.(float64)) == weekday {
				dayAllowed = true
				break
			}
		}
		if !dayAllowed {
			return false
		}
	}

	// Check allowed time range
	startTime, hasStart := data["allowed_start"].(string)
	endTime, hasEnd := data["allowed_end"].(string)

	if hasStart && hasEnd {
		// Simple time comparison (would be more robust with proper parsing)
		currentTime := now.Format("15:04")
		if currentTime < startTime || currentTime > endTime {
			return false
		}
	}

	return true
}

// getApproval resolves an approval by ID via the wired Approval store. When
// no store has been configured the call fails explicitly — silently returning
// a stub record previously caused matchesApprovalScope to treat every
// game/function/env scope as "match nothing", which is worse than failing.
func (s *DelegationService) getApproval(id string) (*Approval, error) {
	s.mu.RLock()
	store := s.approvalStore
	s.mu.RUnlock()

	if store == nil {
		return nil, errors.New("approval store not configured; cannot resolve approval scope for delegation")
	}
	return store.Get(id)
}

// ListDelegations lists delegations with filtering
func (s *DelegationService) ListDelegations(filter DelegationFilter, page Page) ([]*Delegation, int, error) {
	return s.store.List(filter, page)
}

// GetUserDelegations gets all delegations for a user (both as delegator and delegate)
func (s *DelegationService) GetUserDelegations(userID string) (delegated []*Delegation, received []*Delegation, err error) {
	delegated, err = s.store.GetActiveDelegationsByUser(userID)
	if err != nil {
		return nil, nil, err
	}

	received, err = s.store.GetActiveDelegationsForUser(userID)
	if err != nil {
		return nil, nil, err
	}

	return delegated, received, nil
}

// CleanupExpiredDelegations marks expired delegations as expired
func (s *DelegationService) CleanupExpiredDelegations(ctx context.Context) (int, error) {
	filter := DelegationFilter{State: DelegationStateActive}
	delegations, _, err := s.store.List(filter, Page{Size: 10000})
	if err != nil {
		return 0, err
	}

	now := time.Now()
	count := 0

	for _, d := range delegations {
		if d.EndAt != nil && now.After(*d.EndAt) {
			d.State = DelegationStateExpired
			d.UpdatedAt = now
			if _, err := s.store.Update(d); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// DelegationChain represents a chain of delegations
type DelegationChain struct {
	OriginalDelegator string           `json:"original_delegator"`
	CurrentDelegate   string           `json:"current_delegate"`
	Chain             []DelegationLink `json:"chain"`
	TotalDepth        int              `json:"total_depth"`
}

// DelegationLink represents a link in the delegation chain
type DelegationLink struct {
	Delegator string    `json:"delegator"`
	Delegate  string    `json:"delegate"`
	At        time.Time `json:"at"`
	Reason    string    `json:"reason"`
}

// GetDelegationChain gets the full delegation chain for a user
func (s *DelegationService) GetDelegationChain(userID string) (*DelegationChain, error) {
	chain := &DelegationChain{
		CurrentDelegate: userID,
		Chain:           []DelegationLink{},
	}

	// Walk up the delegation chain
	currentUser := userID
	visited := make(map[string]bool)
	maxDepth := 10 // Prevent infinite loops

	for i := 0; i < maxDepth; i++ {
		if visited[currentUser] {
			break
		}
		visited[currentUser] = true

		delegations, err := s.store.GetActiveDelegationsForUser(currentUser)
		if err != nil || len(delegations) == 0 {
			break
		}

		// Find the primary delegation (could have multiple)
		var primaryDelegation *Delegation
		for _, d := range delegations {
			if primaryDelegation == nil || d.CreatedAt.After(primaryDelegation.CreatedAt) {
				primaryDelegation = d
			}
		}

		if primaryDelegation != nil {
			chain.Chain = append([]DelegationLink{{
				Delegator: primaryDelegation.Delegator,
				Delegate:  primaryDelegation.Delegate,
				At:        primaryDelegation.CreatedAt,
				Reason:    primaryDelegation.Reason,
			}}, chain.Chain...)

			chain.OriginalDelegator = primaryDelegation.Delegator
			currentUser = primaryDelegation.Delegator
		}
	}

	chain.TotalDepth = len(chain.Chain)
	return chain, nil
}
