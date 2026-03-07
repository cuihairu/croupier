package approvals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Workflow related errors
var (
	ErrWorkflowNotFound      = errors.New("workflow not found")
	ErrWorkflowNotActive     = errors.New("workflow not active")
	ErrInvalidTransition     = errors.New("invalid state transition")
	ErrNoApprovalRequired    = errors.New("no approval required")
	ErrApprovalAlreadyExists = errors.New("approval already exists for this step")
	ErrNotAuthorizedApprover = errors.New("user not authorized to approve this step")
)

// WorkflowState represents the state of a workflow
type WorkflowState string

const (
	WorkflowStateDraft     WorkflowState = "draft"
	WorkflowStatePending   WorkflowState = "pending"
	WorkflowStateApproved  WorkflowState = "approved"
	WorkflowStateRejected  WorkflowState = "rejected"
	WorkflowStateCancelled WorkflowState = "cancelled"
	WorkflowStateExpired   WorkflowState = "expired"
)

// StepType defines the type of approval step
type StepType string

const (
	StepTypeSequential StepType = "sequential" // One approver at a time, in order
	StepTypeParallel   StepType = "parallel"   // All approvers can approve simultaneously
	StepTypeAny        StepType = "any"        // Any one approver is sufficient
	StepTypePercentage StepType = "percentage" // Requires percentage of approvers
)

// ConditionOperator defines condition operators
type ConditionOperator string

const (
	CondOpEquals      ConditionOperator = "equals"
	CondOpNotEquals   ConditionOperator = "not_equals"
	CondOpContains    ConditionOperator = "contains"
	CondOpGreaterThan ConditionOperator = "greater_than"
	CondOpLessThan    ConditionOperator = "less_than"
	CondOpIn          ConditionOperator = "in"
	CondOpNotIn       ConditionOperator = "not_in"
)

// Condition represents a condition for conditional approval
type Condition struct {
	Field    string            `json:"field"`
	Operator ConditionOperator `json:"operator"`
	Value    interface{}       `json:"value"`
}

// ConditionGroup represents a group of conditions with AND/OR logic
type ConditionGroup struct {
	Conditions []Condition `json:"conditions"`
	Logic      string      `json:"logic"` // "and" or "or"
}

// ApprovalStep represents a single step in the approval workflow
type ApprovalStep struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	Type           StepType         `json:"type"`
	Approvers      []string         `json:"approvers"`      // User IDs or role IDs
	RequiredCount  int              `json:"required_count"` // For percentage type
	Conditions     []ConditionGroup `json:"conditions"`     // Conditions to enter this step
	Timeout        time.Duration    `json:"timeout"`
	TimeoutAction  string           `json:"timeout_action"` // "approve", "reject", "escalate"
	EscalateTo     string           `json:"escalate_to"`    // Step ID to escalate to
	Order          int              `json:"order"`
	AllowDelegate  bool             `json:"allow_delegate"`
	AllowComment   bool             `json:"allow_comment"`
	RequireComment bool             `json:"require_comment"`
}

// WorkflowDefinition defines an approval workflow template
type WorkflowDefinition struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Active      bool           `json:"active"`
	Steps       []ApprovalStep `json:"steps"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CreatedBy   string         `json:"created_by"`
}

// WorkflowInstance represents a running workflow instance
type WorkflowInstance struct {
	ID            string                 `json:"id"`
	DefinitionID  string                 `json:"definition_id"`
	Definition    *WorkflowDefinition    `json:"definition,omitempty"`
	State         WorkflowState          `json:"state"`
	CurrentStep   int                    `json:"current_step"`
	Context       map[string]interface{} `json:"context"`     // Workflow context data
	ApprovalID    string                 `json:"approval_id"` // Reference to original approval
	Initiator     string                 `json:"initiator"`
	StartedAt     time.Time              `json:"started_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	ExpiresAt     *time.Time             `json:"expires_at,omitempty"`
	StepApprovals []StepApproval         `json:"step_approvals"`
	History       []WorkflowHistoryEntry `json:"history"`
}

// StepApproval represents an approval at a specific step
type StepApproval struct {
	StepID      string    `json:"step_id"`
	Approver    string    `json:"approver"`
	DelegatedBy string    `json:"delegated_by,omitempty"`
	Decision    string    `json:"decision"` // "approved", "rejected"
	Comment     string    `json:"comment,omitempty"`
	DecidedAt   time.Time `json:"decided_at"`
	IPAddress   string    `json:"ip_address,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
}

// WorkflowHistoryEntry represents a history entry for workflow state changes
type WorkflowHistoryEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Actor     string    `json:"actor"`
	Details   string    `json:"details"`
}

// WorkflowStore interface for workflow persistence
type WorkflowStore interface {
	// Definition operations
	CreateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error)
	UpdateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error)
	GetDefinition(id string) (*WorkflowDefinition, error)
	ListDefinitions(activeOnly bool) ([]*WorkflowDefinition, error)
	DeleteDefinition(id string) error

	// Instance operations
	CreateInstance(inst *WorkflowInstance) (*WorkflowInstance, error)
	GetInstance(id string) (*WorkflowInstance, error)
	GetInstanceByApprovalID(approvalID string) (*WorkflowInstance, error)
	UpdateInstance(inst *WorkflowInstance) (*WorkflowInstance, error)
	ListInstances(filter WorkflowInstanceFilter, page Page) ([]*WorkflowInstance, int, error)

	// Step approval operations
	AddStepApproval(instanceID string, approval *StepApproval) error
	GetStepApprovals(instanceID string) ([]StepApproval, error)
}

// WorkflowInstanceFilter for filtering workflow instances
type WorkflowInstanceFilter struct {
	State        WorkflowState
	DefinitionID string
	Initiator    string
	ApprovalID   string
}

// WorkflowEngine manages approval workflows
type WorkflowEngine struct {
	store         WorkflowStore
	approvalStore Store
	notifier      Notifier
	timer         Timer
}

// NewWorkflowEngine creates a new workflow engine
func NewWorkflowEngine(store WorkflowStore, approvalStore Store, notifier Notifier) *WorkflowEngine {
	return &WorkflowEngine{
		store:         store,
		approvalStore: approvalStore,
		notifier:      notifier,
		timer:         &realTimer{},
	}
}

// SetTimer sets a custom timer (for testing)
func (e *WorkflowEngine) SetTimer(timer Timer) {
	e.timer = timer
}

// CreateDefinition creates a new workflow definition
func (e *WorkflowEngine) CreateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error) {
	if err := e.validateDefinition(def); err != nil {
		return nil, fmt.Errorf("invalid workflow definition: %w", err)
	}

	def.CreatedAt = time.Now()
	def.UpdatedAt = time.Now()
	return e.store.CreateDefinition(def)
}

// UpdateDefinition updates an existing workflow definition
func (e *WorkflowEngine) UpdateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error) {
	if err := e.validateDefinition(def); err != nil {
		return nil, fmt.Errorf("invalid workflow definition: %w", err)
	}

	def.UpdatedAt = time.Now()
	return e.store.UpdateDefinition(def)
}

// validateDefinition validates a workflow definition
func (e *WorkflowEngine) validateDefinition(def *WorkflowDefinition) error {
	if def.ID == "" {
		return errors.New("workflow ID is required")
	}
	if def.Name == "" {
		return errors.New("workflow name is required")
	}
	if len(def.Steps) == 0 {
		return errors.New("workflow must have at least one step")
	}

	// Validate step order and references
	stepIDs := make(map[string]bool)
	for i, step := range def.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d: ID is required", i)
		}
		if stepIDs[step.ID] {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
		stepIDs[step.ID] = true

		if step.EscalateTo != "" && !stepIDs[step.EscalateTo] {
			return fmt.Errorf("step %s: invalid escalate_to reference: %s", step.ID, step.EscalateTo)
		}

		if step.Type == StepTypePercentage && (step.RequiredCount <= 0 || step.RequiredCount > 100) {
			return fmt.Errorf("step %s: percentage type requires required_count between 1 and 100", step.ID)
		}
	}

	return nil
}

// StartWorkflow starts a new workflow instance for an approval
func (e *WorkflowEngine) StartWorkflow(ctx context.Context, defID string, approval *Approval) (*WorkflowInstance, error) {
	// Get workflow definition
	def, err := e.store.GetDefinition(defID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow definition: %w", err)
	}

	if !def.Active {
		return nil, ErrWorkflowNotActive
	}

	// Check if workflow instance already exists for this approval
	existing, _ := e.store.GetInstanceByApprovalID(approval.ID)
	if existing != nil {
		return nil, ErrApprovalAlreadyExists
	}

	// Determine starting step based on conditions
	startStep := 0
	for i, step := range def.Steps {
		if e.evaluateConditions(step.Conditions, approval) {
			startStep = i
			break
		}
	}

	// Create workflow instance
	now := time.Now()
	instance := &WorkflowInstance{
		ID:            generateID("wf"),
		DefinitionID:  defID,
		Definition:    def,
		State:         WorkflowStatePending,
		CurrentStep:   startStep,
		Context:       make(map[string]interface{}),
		ApprovalID:    approval.ID,
		Initiator:     approval.Actor,
		StartedAt:     now,
		StepApprovals: []StepApproval{},
		History: []WorkflowHistoryEntry{
			{
				Timestamp: now,
				Action:    "workflow_started",
				Actor:     approval.Actor,
				Details:   fmt.Sprintf("Workflow %s started at step %d", def.Name, startStep),
			},
		},
	}

	// Set expiration if timeout is configured
	if step := def.Steps[startStep]; step.Timeout > 0 {
		exp := now.Add(step.Timeout)
		instance.ExpiresAt = &exp
	}

	// Save instance
	instance, err = e.store.CreateInstance(instance)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow instance: %w", err)
	}

	// Send notifications to approvers
	if e.notifier != nil {
		step := def.Steps[startStep]
		e.notifyApprovers(ctx, instance, &step, "approval_required")
	}

	return instance, nil
}

// ApproveStep approves the current step in a workflow
func (e *WorkflowEngine) ApproveStep(ctx context.Context, instanceID, approver, comment, ipAddress, userAgent string) (*WorkflowInstance, error) {
	instance, err := e.store.GetInstance(instanceID)
	if err != nil {
		return nil, err
	}

	if instance.State != WorkflowStatePending {
		return nil, ErrInvalidTransition
	}

	def := instance.Definition
	if def == nil {
		def, err = e.store.GetDefinition(instance.DefinitionID)
		if err != nil {
			return nil, err
		}
		instance.Definition = def
	}

	currentStep := def.Steps[instance.CurrentStep]

	// Check if user is authorized approver
	if !e.isAuthorizedApprover(currentStep, approver, instance) {
		return nil, ErrNotAuthorizedApprover
	}

	// Check if already approved by this user
	existingApprovals, err := e.store.GetStepApprovals(instance.ID)
	if err != nil {
		return nil, err
	}

	for _, a := range existingApprovals {
		if a.StepID == currentStep.ID && a.Approver == approver && a.Decision == "approved" {
			return nil, ErrApprovalAlreadyExists
		}
	}

	// Record the approval
	now := time.Now()
	stepApproval := &StepApproval{
		StepID:    currentStep.ID,
		Approver:  approver,
		Decision:  "approved",
		Comment:   comment,
		DecidedAt: now,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	if err := e.store.AddStepApproval(instance.ID, stepApproval); err != nil {
		return nil, err
	}

	instance.StepApprovals = append(instance.StepApprovals, *stepApproval)
	instance.History = append(instance.History, WorkflowHistoryEntry{
		Timestamp: now,
		Action:    "step_approved",
		Actor:     approver,
		Details:   fmt.Sprintf("Step %s approved: %s", currentStep.Name, comment),
	})

	// Check if step is complete
	if e.isStepComplete(currentStep, instance) {
		// Move to next step or complete workflow
		if instance.CurrentStep < len(def.Steps)-1 {
			instance.CurrentStep++
			newStep := def.Steps[instance.CurrentStep]

			// Set new expiration
			if newStep.Timeout > 0 {
				exp := now.Add(newStep.Timeout)
				instance.ExpiresAt = &exp
			} else {
				instance.ExpiresAt = nil
			}

			instance.History = append(instance.History, WorkflowHistoryEntry{
				Timestamp: now,
				Action:    "step_advanced",
				Actor:     approver,
				Details:   fmt.Sprintf("Advanced to step %s", newStep.Name),
			})

			// Notify approvers of new step
			if e.notifier != nil {
				e.notifyApprovers(ctx, instance, &newStep, "approval_required")
			}
		} else {
			// Workflow complete
			instance.State = WorkflowStateApproved
			instance.CompletedAt = &now
			instance.ExpiresAt = nil

			// Approve the original approval record
			if _, err := e.approvalStore.Approve(instance.ApprovalID); err != nil {
				// Log error but don't fail
			}

			instance.History = append(instance.History, WorkflowHistoryEntry{
				Timestamp: now,
				Action:    "workflow_completed",
				Actor:     approver,
				Details:   "Workflow approved",
			})

			// Notify initiator
			if e.notifier != nil {
				e.notifier.Notify(ctx, []string{instance.Initiator}, NotificationEvent{
					Type:       "workflow_approved",
					Title:      "Workflow Approved",
					Message:    fmt.Sprintf("Your approval request has been approved"),
					InstanceID: instance.ID,
					ApprovalID: instance.ApprovalID,
				})
			}
		}
	}

	return e.store.UpdateInstance(instance)
}

// RejectStep rejects the workflow at the current step
func (e *WorkflowEngine) RejectStep(ctx context.Context, instanceID, approver, reason, ipAddress, userAgent string) (*WorkflowInstance, error) {
	instance, err := e.store.GetInstance(instanceID)
	if err != nil {
		return nil, err
	}

	if instance.State != WorkflowStatePending {
		return nil, ErrInvalidTransition
	}

	def := instance.Definition
	if def == nil {
		def, err = e.store.GetDefinition(instance.DefinitionID)
		if err != nil {
			return nil, err
		}
		instance.Definition = def
	}

	currentStep := def.Steps[instance.CurrentStep]

	// Check if user is authorized approver
	if !e.isAuthorizedApprover(currentStep, approver, instance) {
		return nil, ErrNotAuthorizedApprover
	}

	// Record the rejection
	now := time.Now()
	stepApproval := &StepApproval{
		StepID:    currentStep.ID,
		Approver:  approver,
		Decision:  "rejected",
		Comment:   reason,
		DecidedAt: now,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	if err := e.store.AddStepApproval(instance.ID, stepApproval); err != nil {
		return nil, err
	}

	instance.StepApprovals = append(instance.StepApprovals, *stepApproval)
	instance.State = WorkflowStateRejected
	instance.CompletedAt = &now
	instance.ExpiresAt = nil

	instance.History = append(instance.History, WorkflowHistoryEntry{
		Timestamp: now,
		Action:    "workflow_rejected",
		Actor:     approver,
		Details:   fmt.Sprintf("Rejected at step %s: %s", currentStep.Name, reason),
	})

	// Reject the original approval record
	if _, err := e.approvalStore.Reject(instance.ApprovalID, reason); err != nil {
		// Log error but don't fail
	}

	// Notify initiator
	if e.notifier != nil {
		e.notifier.Notify(ctx, []string{instance.Initiator}, NotificationEvent{
			Type:       "workflow_rejected",
			Title:      "Workflow Rejected",
			Message:    fmt.Sprintf("Your approval request has been rejected: %s", reason),
			InstanceID: instance.ID,
			ApprovalID: instance.ApprovalID,
		})
	}

	return e.store.UpdateInstance(instance)
}

// CancelWorkflow cancels a workflow instance
func (e *WorkflowEngine) CancelWorkflow(ctx context.Context, instanceID, actor, reason string) (*WorkflowInstance, error) {
	instance, err := e.store.GetInstance(instanceID)
	if err != nil {
		return nil, err
	}

	if instance.State != WorkflowStatePending {
		return nil, ErrInvalidTransition
	}

	now := time.Now()
	instance.State = WorkflowStateCancelled
	instance.CompletedAt = &now
	instance.ExpiresAt = nil

	instance.History = append(instance.History, WorkflowHistoryEntry{
		Timestamp: now,
		Action:    "workflow_cancelled",
		Actor:     actor,
		Details:   reason,
	})

	// Reject the original approval record
	if _, err := e.approvalStore.Reject(instance.ApprovalID, "Cancelled: "+reason); err != nil {
		// Log error but don't fail
	}

	return e.store.UpdateInstance(instance)
}

// ProcessTimeouts processes expired workflows
func (e *WorkflowEngine) ProcessTimeouts(ctx context.Context) ([]*WorkflowInstance, error) {
	// This would typically be called by a background job
	filter := WorkflowInstanceFilter{State: WorkflowStatePending}
	instances, _, err := e.store.ListInstances(filter, Page{Size: 1000})
	if err != nil {
		return nil, err
	}

	var processed []*WorkflowInstance
	now := time.Now()

	for _, instance := range instances {
		if instance.ExpiresAt != nil && now.After(*instance.ExpiresAt) {
			def := instance.Definition
			if def == nil {
				def, _ = e.store.GetDefinition(instance.DefinitionID)
				if def == nil {
					continue
				}
			}

			currentStep := def.Steps[instance.CurrentStep]

			switch currentStep.TimeoutAction {
			case "approve":
				instance, err = e.timeoutApprove(ctx, instance)
			case "reject":
				instance, err = e.timeoutReject(ctx, instance)
			case "escalate":
				instance, err = e.timeoutEscalate(ctx, instance, &currentStep)
			default:
				instance, err = e.timeoutReject(ctx, instance)
			}

			if err == nil {
				processed = append(processed, instance)
			}
		}
	}

	return processed, nil
}

func (e *WorkflowEngine) timeoutApprove(ctx context.Context, instance *WorkflowInstance) (*WorkflowInstance, error) {
	now := time.Now()
	instance.State = WorkflowStateApproved
	instance.CompletedAt = &now
	instance.ExpiresAt = nil

	instance.History = append(instance.History, WorkflowHistoryEntry{
		Timestamp: now,
		Action:    "timeout_approved",
		Actor:     "system",
		Details:   "Auto-approved due to timeout",
	})

	if _, err := e.approvalStore.Approve(instance.ApprovalID); err != nil {
		// Log error
	}

	return e.store.UpdateInstance(instance)
}

func (e *WorkflowEngine) timeoutReject(ctx context.Context, instance *WorkflowInstance) (*WorkflowInstance, error) {
	now := time.Now()
	instance.State = WorkflowStateExpired
	instance.CompletedAt = &now
	instance.ExpiresAt = nil

	instance.History = append(instance.History, WorkflowHistoryEntry{
		Timestamp: now,
		Action:    "timeout_rejected",
		Actor:     "system",
		Details:   "Rejected due to timeout",
	})

	if _, err := e.approvalStore.Reject(instance.ApprovalID, "Expired due to timeout"); err != nil {
		// Log error
	}

	return e.store.UpdateInstance(instance)
}

func (e *WorkflowEngine) timeoutEscalate(ctx context.Context, instance *WorkflowInstance, currentStep *ApprovalStep) (*WorkflowInstance, error) {
	if currentStep.EscalateTo == "" {
		return e.timeoutReject(ctx, instance)
	}

	def := instance.Definition
	if def == nil {
		return e.timeoutReject(ctx, instance)
	}

	// Find the escalate step
	for i, step := range def.Steps {
		if step.ID == currentStep.EscalateTo {
			now := time.Now()
			instance.CurrentStep = i

			if step.Timeout > 0 {
				exp := now.Add(step.Timeout)
				instance.ExpiresAt = &exp
			}

			instance.History = append(instance.History, WorkflowHistoryEntry{
				Timestamp: now,
				Action:    "escalated",
				Actor:     "system",
				Details:   fmt.Sprintf("Escalated from %s to %s due to timeout", currentStep.Name, step.Name),
			})

			// Notify new approvers
			if e.notifier != nil {
				e.notifyApprovers(ctx, instance, &step, "approval_escalated")
			}

			return e.store.UpdateInstance(instance)
		}
	}

	return e.timeoutReject(ctx, instance)
}

// isAuthorizedApprover checks if a user can approve at a step
func (e *WorkflowEngine) isAuthorizedApprover(step ApprovalStep, user string, instance *WorkflowInstance) bool {
	// Check direct approvers
	for _, approver := range step.Approvers {
		if approver == user {
			return true
		}
	}

	// Check for delegation
	approvals, err := e.store.GetStepApprovals(instance.ID)
	if err != nil {
		return false
	}

	for _, approval := range approvals {
		if approval.StepID == step.ID && approval.DelegatedBy == user {
			return true
		}
	}

	return false
}

// isStepComplete checks if a step has received sufficient approvals
func (e *WorkflowEngine) isStepComplete(step ApprovalStep, instance *WorkflowInstance) bool {
	approvals, err := e.store.GetStepApprovals(instance.ID)
	if err != nil {
		return false
	}

	var stepApprovals []StepApproval
	for _, a := range approvals {
		if a.StepID == step.ID && a.Decision == "approved" {
			stepApprovals = append(stepApprovals, a)
		}
	}

	switch step.Type {
	case StepTypeSequential:
		// In sequential, each approver approves in order
		return len(stepApprovals) >= len(step.Approvers)

	case StepTypeParallel:
		// All approvers must approve
		return len(stepApprovals) >= len(step.Approvers)

	case StepTypeAny:
		// Any one approver is sufficient
		return len(stepApprovals) >= 1

	case StepTypePercentage:
		// Required percentage of approvers
		required := len(step.Approvers) * step.RequiredCount / 100
		if required < 1 {
			required = 1
		}
		return len(stepApprovals) >= required

	default:
		return len(stepApprovals) >= 1
	}
}

// evaluateConditions evaluates condition groups against approval data
func (e *WorkflowEngine) evaluateConditions(groups []ConditionGroup, approval *Approval) bool {
	if len(groups) == 0 {
		return true
	}

	// All condition groups must be satisfied (AND logic between groups)
	for _, group := range groups {
		if !e.evaluateConditionGroup(group, approval) {
			return false
		}
	}

	return true
}

func (e *WorkflowEngine) evaluateConditionGroup(group ConditionGroup, approval *Approval) bool {
	if len(group.Conditions) == 0 {
		return true
	}

	approvalMap := approvalToMap(approval)

	if group.Logic == "or" {
		// Any condition must be satisfied
		for _, cond := range group.Conditions {
			if e.evaluateCondition(cond, approvalMap) {
				return true
			}
		}
		return false
	}

	// All conditions must be satisfied (AND logic)
	for _, cond := range group.Conditions {
		if !e.evaluateCondition(cond, approvalMap) {
			return false
		}
	}
	return true
}

func (e *WorkflowEngine) evaluateCondition(cond Condition, data map[string]interface{}) bool {
	value, exists := data[cond.Field]
	if !exists {
		return false
	}

	switch cond.Operator {
	case CondOpEquals:
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", cond.Value)
	case CondOpNotEquals:
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", cond.Value)
	case CondOpContains:
		return containsString(fmt.Sprintf("%v", value), fmt.Sprintf("%v", cond.Value))
	case CondOpGreaterThan:
		return compareNumbers(value, cond.Value) > 0
	case CondOpLessThan:
		return compareNumbers(value, cond.Value) < 0
	case CondOpIn:
		return inList(value, cond.Value)
	case CondOpNotIn:
		return !inList(value, cond.Value)
	default:
		return false
	}
}

// notifyApprovers sends notifications to approvers
func (e *WorkflowEngine) notifyApprovers(ctx context.Context, instance *WorkflowInstance, step *ApprovalStep, eventType string) {
	if e.notifier == nil {
		return
	}

	event := NotificationEvent{
		Type:       eventType,
		InstanceID: instance.ID,
		ApprovalID: instance.ApprovalID,
	}

	switch eventType {
	case "approval_required":
		event.Title = "Approval Required"
		event.Message = fmt.Sprintf("Your approval is requested for step: %s", step.Name)
	case "approval_escalated":
		event.Title = "Approval Escalated"
		event.Message = fmt.Sprintf("An approval has been escalated to you: %s", step.Name)
	case "reminder":
		event.Title = "Approval Reminder"
		event.Message = fmt.Sprintf("Reminder: Approval pending for step: %s", step.Name)
	}

	e.notifier.Notify(ctx, step.Approvers, event)
}

// Helper functions

func approvalToMap(a *Approval) map[string]interface{} {
	return map[string]interface{}{
		"id":                a.ID,
		"state":             a.State,
		"function_id":       a.FunctionID,
		"game_id":           a.GameID,
		"env":               a.Env,
		"actor":             a.Actor,
		"mode":              a.Mode,
		"idempotency_key":   a.IdempotencyKey,
		"route":             a.Route,
		"target_service_id": a.TargetServiceID,
		"reason":            a.Reason,
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

func compareNumbers(a, b interface{}) int {
	af := toFloat64(a)
	bf := toFloat64(b)
	if af < bf {
		return -1
	} else if af > bf {
		return 1
	}
	return 0
}

func toFloat64(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float32:
		return float64(n)
	case float64:
		return n
	default:
		return 0
	}
}

func inList(value, list interface{}) bool {
	listSlice, ok := list.([]interface{})
	if !ok {
		return false
	}
	for _, item := range listSlice {
		if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", item) {
			return true
		}
	}
	return false
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// Timer interface for time-related operations (for testing)
type Timer interface {
	Now() time.Time
}

type realTimer struct{}

func (t *realTimer) Now() time.Time {
	return time.Now()
}

// MarshalJSON custom marshaling for WorkflowInstance
func (i *WorkflowInstance) MarshalJSON() ([]byte, error) {
	type Alias WorkflowInstance
	return json.Marshal(&struct {
		*Alias
		StartedAt   string  `json:"started_at"`
		CompletedAt *string `json:"completed_at,omitempty"`
		ExpiresAt   *string `json:"expires_at,omitempty"`
	}{
		Alias:     (*Alias)(i),
		StartedAt: i.StartedAt.Format(time.RFC3339),
		CompletedAt: func() *string {
			if i.CompletedAt != nil {
				s := i.CompletedAt.Format(time.RFC3339)
				return &s
			}
			return nil
		}(),
		ExpiresAt: func() *string {
			if i.ExpiresAt != nil {
				s := i.ExpiresAt.Format(time.RFC3339)
				return &s
			}
			return nil
		}(),
	})
}
