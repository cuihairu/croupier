package approvals

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

// WorkflowDefinitionModel is the GORM model for workflow definitions
type WorkflowDefinitionModel struct {
	ID          string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Version     string         `gorm:"type:varchar(50);not null" json:"version"`
	Active      bool           `gorm:"default:true" json:"active"`
	StepsJSON   []byte         `gorm:"type:json;not null" json:"steps_json"`
	CreatedAt   time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"not null" json:"updated_at"`
	CreatedBy   string         `gorm:"type:varchar(255);not null" json:"created_by"`
}

// TableName returns the table name
func (WorkflowDefinitionModel) TableName() string {
	return "workflow_definitions"
}

// ToDefinition converts model to domain type
func (m *WorkflowDefinitionModel) ToDefinition() (*WorkflowDefinition, error) {
	var steps []ApprovalStep
	if err := json.Unmarshal(m.StepsJSON, &steps); err != nil {
		return nil, err
	}

	return &WorkflowDefinition{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Version:     m.Version,
		Active:      m.Active,
		Steps:       steps,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
		CreatedBy:   m.CreatedBy,
	}, nil
}

// FromDefinition creates model from domain type
func FromDefinition(d *WorkflowDefinition) (*WorkflowDefinitionModel, error) {
	stepsJSON, err := json.Marshal(d.Steps)
	if err != nil {
		return nil, err
	}

	return &WorkflowDefinitionModel{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		Version:     d.Version,
		Active:      d.Active,
		StepsJSON:   stepsJSON,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
		CreatedBy:   d.CreatedBy,
	}, nil
}

// WorkflowInstanceModel is the GORM model for workflow instances
type WorkflowInstanceModel struct {
	ID            string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	DefinitionID  string         `gorm:"type:varchar(255);not null;index" json:"definition_id"`
	State         string         `gorm:"type:varchar(50);not null;index" json:"state"`
	CurrentStep   int            `gorm:"not null" json:"current_step"`
	ContextJSON   []byte         `gorm:"type:json" json:"context_json"`
	ApprovalID    string         `gorm:"type:varchar(255);not null;index" json:"approval_id"`
	Initiator     string         `gorm:"type:varchar(255);not null;index" json:"initiator"`
	StartedAt     time.Time      `gorm:"not null;index" json:"started_at"`
	CompletedAt   *time.Time     `json:"completed_at"`
	ExpiresAt     *time.Time     `json:"expires_at"`
	HistoryJSON   []byte         `gorm:"type:json" json:"history_json"`
	CreatedAt     time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"not null" json:"updated_at"`
}

// TableName returns the table name
func (WorkflowInstanceModel) TableName() string {
	return "workflow_instances"
}

// ToInstance converts model to domain type
func (m *WorkflowInstanceModel) ToInstance() (*WorkflowInstance, error) {
	var context map[string]interface{}
	if m.ContextJSON != nil {
		if err := json.Unmarshal(m.ContextJSON, &context); err != nil {
			return nil, err
		}
	}

	var history []WorkflowHistoryEntry
	if m.HistoryJSON != nil {
		if err := json.Unmarshal(m.HistoryJSON, &history); err != nil {
			return nil, err
		}
	}

	return &WorkflowInstance{
		ID:           m.ID,
		DefinitionID: m.DefinitionID,
		State:        WorkflowState(m.State),
		CurrentStep:  m.CurrentStep,
		Context:      context,
		ApprovalID:   m.ApprovalID,
		Initiator:    m.Initiator,
		StartedAt:    m.StartedAt,
		CompletedAt:  m.CompletedAt,
		ExpiresAt:    m.ExpiresAt,
		History:      history,
	}, nil
}

// FromInstance creates model from domain type
func FromInstance(i *WorkflowInstance) (*WorkflowInstanceModel, error) {
	var contextJSON []byte
	if i.Context != nil {
		var err error
		contextJSON, err = json.Marshal(i.Context)
		if err != nil {
			return nil, err
		}
	}

	var historyJSON []byte
	if i.History != nil {
		var err error
		historyJSON, err = json.Marshal(i.History)
		if err != nil {
			return nil, err
		}
	}

	return &WorkflowInstanceModel{
		ID:           i.ID,
		DefinitionID: i.DefinitionID,
		State:        string(i.State),
		CurrentStep:  i.CurrentStep,
		ContextJSON:  contextJSON,
		ApprovalID:   i.ApprovalID,
		Initiator:    i.Initiator,
		StartedAt:    i.StartedAt,
		CompletedAt:  i.CompletedAt,
		ExpiresAt:    i.ExpiresAt,
		HistoryJSON:  historyJSON,
	}, nil
}

// StepApprovalModel is the GORM model for step approvals
type StepApprovalModel struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	InstanceID  string    `gorm:"type:varchar(255);not null;index" json:"instance_id"`
	StepID      string    `gorm:"type:varchar(255);not null;index" json:"step_id"`
	Approver    string    `gorm:"type:varchar(255);not null;index" json:"approver"`
	DelegatedBy string    `gorm:"type:varchar(255)" json:"delegated_by"`
	Decision    string    `gorm:"type:varchar(50);not null" json:"decision"`
	Comment     string    `gorm:"type:text" json:"comment"`
	DecidedAt   time.Time `gorm:"not null" json:"decided_at"`
	IPAddress   string    `gorm:"type:varchar(50)" json:"ip_address"`
	UserAgent   string    `gorm:"type:varchar(500)" json:"user_agent"`
	CreatedAt   time.Time `gorm:"not null" json:"created_at"`
}

// TableName returns the table name
func (StepApprovalModel) TableName() string {
	return "workflow_step_approvals"
}

// ToStepApproval converts model to domain type
func (m *StepApprovalModel) ToStepApproval() StepApproval {
	return StepApproval{
		StepID:      m.StepID,
		Approver:    m.Approver,
		DelegatedBy: m.DelegatedBy,
		Decision:    m.Decision,
		Comment:     m.Comment,
		DecidedAt:   m.DecidedAt,
		IPAddress:   m.IPAddress,
		UserAgent:   m.UserAgent,
	}
}

// DelegationModel is the GORM model for delegations
type DelegationModel struct {
	ID            string         `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Delegator     string         `gorm:"type:varchar(255);not null;index" json:"delegator"`
	Delegate      string         `gorm:"type:varchar(255);not null;index" json:"delegate"`
	Scope         string         `gorm:"type:varchar(50);not null" json:"scope"`
	ScopeValue    string         `gorm:"type:varchar(255)" json:"scope_value"`
	Permissions   []byte         `gorm:"type:json;not null" json:"permissions"`
	State         string         `gorm:"type:varchar(50);not null;index" json:"state"`
	Reason        string         `gorm:"type:text" json:"reason"`
	StartAt       time.Time      `gorm:"not null" json:"start_at"`
	EndAt         *time.Time     `json:"end_at"`
	MaxUsages     int            `gorm:"default:0" json:"max_usages"`
	UsageCount    int            `gorm:"default:0" json:"usage_count"`
	Constraints   []byte         `gorm:"type:json" json:"constraints"`
	RevokedAt     *time.Time     `json:"revoked_at"`
	RevokedBy     string         `gorm:"type:varchar(255)" json:"revoked_by"`
	RevokedReason string         `gorm:"type:text" json:"revoked_reason"`
	CreatedAt     time.Time      `gorm:"not null" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"not null" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// TableName returns the table name
func (DelegationModel) TableName() string {
	return "approval_delegations"
}

// ToDelegation converts model to domain type
func (m *DelegationModel) ToDelegation() (*Delegation, error) {
	var permissions []DelegationPermission
	if err := json.Unmarshal(m.Permissions, &permissions); err != nil {
		return nil, err
	}

	var constraints []DelegationConstraint
	if m.Constraints != nil {
		if err := json.Unmarshal(m.Constraints, &constraints); err != nil {
			return nil, err
		}
	}

	return &Delegation{
		ID:             m.ID,
		Delegator:      m.Delegator,
		Delegate:       m.Delegate,
		Scope:          DelegationScope(m.Scope),
		ScopeValue:     m.ScopeValue,
		Permissions:    permissions,
		State:          DelegationState(m.State),
		Reason:         m.Reason,
		StartAt:        m.StartAt,
		EndAt:          m.EndAt,
		MaxUsages:      m.MaxUsages,
		UsageCount:     m.UsageCount,
		Constraints:    constraints,
		RevokedAt:      m.RevokedAt,
		RevokedBy:      m.RevokedBy,
		RevokedReason:  m.RevokedReason,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}, nil
}

// FromDelegation creates model from domain type
func FromDelegation(d *Delegation) (*DelegationModel, error) {
	permissions, err := json.Marshal(d.Permissions)
	if err != nil {
		return nil, err
	}

	var constraints []byte
	if d.Constraints != nil {
		constraints, err = json.Marshal(d.Constraints)
		if err != nil {
			return nil, err
		}
	}

	return &DelegationModel{
		ID:             d.ID,
		Delegator:      d.Delegator,
		Delegate:       d.Delegate,
		Scope:          string(d.Scope),
		ScopeValue:     d.ScopeValue,
		Permissions:    permissions,
		State:          string(d.State),
		Reason:         d.Reason,
		StartAt:        d.StartAt,
		EndAt:          d.EndAt,
		MaxUsages:      d.MaxUsages,
		UsageCount:     d.UsageCount,
		Constraints:    constraints,
		RevokedAt:      d.RevokedAt,
		RevokedBy:      d.RevokedBy,
		RevokedReason:  d.RevokedReason,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}, nil
}

// NotificationModel is the GORM model for notifications
type NotificationModel struct {
	ID         uint                `gorm:"primaryKey;autoIncrement" json:"id"`
	Recipient  string              `gorm:"type:varchar(255);not null;index" json:"recipient"`
	Channel    string              `gorm:"type:varchar(50);not null;index" json:"channel"`
	EventJSON  []byte              `gorm:"type:json;not null" json:"event_json"`
	Read       bool                `gorm:"default:false;index" json:"read"`
	ReadAt     *time.Time          `json:"read_at"`
	CreatedAt  time.Time           `gorm:"not null;index" json:"created_at"`
}

// TableName returns the table name
func (NotificationModel) TableName() string {
	return "notifications"
}

// SQLWorkflowStore implements WorkflowStore using SQL
type SQLWorkflowStore struct {
	db *gorm.DB
}

// NewSQLWorkflowStore creates a new SQL workflow store
func NewSQLWorkflowStore(db *gorm.DB) (*SQLWorkflowStore, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	// Auto migrate
	if err := db.AutoMigrate(
		&WorkflowDefinitionModel{},
		&WorkflowInstanceModel{},
		&StepApprovalModel{},
		&DelegationModel{},
		&NotificationModel{},
	); err != nil {
		return nil, err
	}

	return &SQLWorkflowStore{db: db}, nil
}

// CreateDefinition creates a workflow definition
func (s *SQLWorkflowStore) CreateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error) {
	model, err := FromDefinition(def)
	if err != nil {
		return nil, err
	}

	if err := s.db.Create(model).Error; err != nil {
		return nil, err
	}

	return def, nil
}

// UpdateDefinition updates a workflow definition
func (s *SQLWorkflowStore) UpdateDefinition(def *WorkflowDefinition) (*WorkflowDefinition, error) {
	model, err := FromDefinition(def)
	if err != nil {
		return nil, err
	}

	if err := s.db.Save(model).Error; err != nil {
		return nil, err
	}

	return def, nil
}

// GetDefinition gets a workflow definition by ID
func (s *SQLWorkflowStore) GetDefinition(id string) (*WorkflowDefinition, error) {
	var model WorkflowDefinitionModel
	if err := s.db.Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}

	return model.ToDefinition()
}

// ListDefinitions lists workflow definitions
func (s *SQLWorkflowStore) ListDefinitions(activeOnly bool) ([]*WorkflowDefinition, error) {
	query := s.db.Model(&WorkflowDefinitionModel{})
	if activeOnly {
		query = query.Where("active = ?", true)
	}

	var models []WorkflowDefinitionModel
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	definitions := make([]*WorkflowDefinition, len(models))
	for i, model := range models {
		def, err := model.ToDefinition()
		if err != nil {
			return nil, err
		}
		definitions[i] = def
	}

	return definitions, nil
}

// DeleteDefinition deletes a workflow definition
func (s *SQLWorkflowStore) DeleteDefinition(id string) error {
	return s.db.Delete(&WorkflowDefinitionModel{}, "id = ?", id).Error
}

// CreateInstance creates a workflow instance
func (s *SQLWorkflowStore) CreateInstance(inst *WorkflowInstance) (*WorkflowInstance, error) {
	model, err := FromInstance(inst)
	if err != nil {
		return nil, err
	}

	model.CreatedAt = time.Now()
	model.UpdatedAt = time.Now()

	if err := s.db.Create(model).Error; err != nil {
		return nil, err
	}

	return inst, nil
}

// GetInstance gets a workflow instance by ID
func (s *SQLWorkflowStore) GetInstance(id string) (*WorkflowInstance, error) {
	var model WorkflowInstanceModel
	if err := s.db.Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}

	return model.ToInstance()
}

// GetInstanceByApprovalID gets a workflow instance by approval ID
func (s *SQLWorkflowStore) GetInstanceByApprovalID(approvalID string) (*WorkflowInstance, error) {
	var model WorkflowInstanceModel
	if err := s.db.Where("approval_id = ?", approvalID).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, err
	}

	return model.ToInstance()
}

// UpdateInstance updates a workflow instance
func (s *SQLWorkflowStore) UpdateInstance(inst *WorkflowInstance) (*WorkflowInstance, error) {
	model, err := FromInstance(inst)
	if err != nil {
		return nil, err
	}

	model.UpdatedAt = time.Now()

	if err := s.db.Save(model).Error; err != nil {
		return nil, err
	}

	return inst, nil
}

// ListInstances lists workflow instances with filtering
func (s *SQLWorkflowStore) ListInstances(filter WorkflowInstanceFilter, page Page) ([]*WorkflowInstance, int, error) {
	query := s.db.Model(&WorkflowInstanceModel{})

	if filter.State != "" {
		query = query.Where("state = ?", filter.State)
	}
	if filter.DefinitionID != "" {
		query = query.Where("definition_id = ?", filter.DefinitionID)
	}
	if filter.Initiator != "" {
		query = query.Where("initiator = ?", filter.Initiator)
	}
	if filter.ApprovalID != "" {
		query = query.Where("approval_id = ?", filter.ApprovalID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page.Size <= 0 {
		page.Size = 50
	}
	if page.Page <= 0 {
		page.Page = 1
	}

	offset := (page.Page - 1) * page.Size

	var models []WorkflowInstanceModel
	if err := query.Offset(offset).Limit(page.Size).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, 0, err
	}

	instances := make([]*WorkflowInstance, len(models))
	for i, model := range models {
		inst, err := model.ToInstance()
		if err != nil {
			return nil, 0, err
		}
		instances[i] = inst
	}

	return instances, int(total), nil
}

// AddStepApproval adds a step approval
func (s *SQLWorkflowStore) AddStepApproval(instanceID string, approval *StepApproval) error {
	model := &StepApprovalModel{
		InstanceID:  instanceID,
		StepID:      approval.StepID,
		Approver:    approval.Approver,
		DelegatedBy: approval.DelegatedBy,
		Decision:    approval.Decision,
		Comment:     approval.Comment,
		DecidedAt:   approval.DecidedAt,
		IPAddress:   approval.IPAddress,
		UserAgent:   approval.UserAgent,
		CreatedAt:   time.Now(),
	}

	return s.db.Create(model).Error
}

// GetStepApprovals gets all step approvals for an instance
func (s *SQLWorkflowStore) GetStepApprovals(instanceID string) ([]StepApproval, error) {
	var models []StepApprovalModel
	if err := s.db.Where("instance_id = ?", instanceID).Order("decided_at ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	approvals := make([]StepApproval, len(models))
	for i, model := range models {
		approvals[i] = model.ToStepApproval()
	}

	return approvals, nil
}

// SQLDelegationStore implements DelegationStore using SQL
type SQLDelegationStore struct {
	db *gorm.DB
}

// NewSQLDelegationStore creates a new SQL delegation store
func NewSQLDelegationStore(db *gorm.DB) (*SQLDelegationStore, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	return &SQLDelegationStore{db: db}, nil
}

// Create creates a delegation
func (s *SQLDelegationStore) Create(d *Delegation) (*Delegation, error) {
	model, err := FromDelegation(d)
	if err != nil {
		return nil, err
	}

	if err := s.db.Create(model).Error; err != nil {
		return nil, err
	}

	return d, nil
}

// Get gets a delegation by ID
func (s *SQLDelegationStore) Get(id string) (*Delegation, error) {
	var model DelegationModel
	if err := s.db.Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDelegationNotFound
		}
		return nil, err
	}

	return model.ToDelegation()
}

// Update updates a delegation
func (s *SQLDelegationStore) Update(d *Delegation) (*Delegation, error) {
	model, err := FromDelegation(d)
	if err != nil {
		return nil, err
	}

	if err := s.db.Save(model).Error; err != nil {
		return nil, err
	}

	return d, nil
}

// Delete deletes a delegation
func (s *SQLDelegationStore) Delete(id string) error {
	return s.db.Delete(&DelegationModel{}, "id = ?", id).Error
}

// List lists delegations with filtering
func (s *SQLDelegationStore) List(filter DelegationFilter, page Page) ([]*Delegation, int, error) {
	query := s.db.Model(&DelegationModel{})

	if filter.Delegator != "" {
		query = query.Where("delegator = ?", filter.Delegator)
	}
	if filter.Delegate != "" {
		query = query.Where("delegate = ?", filter.Delegate)
	}
	if filter.Scope != "" {
		query = query.Where("scope = ?", filter.Scope)
	}
	if filter.State != "" {
		query = query.Where("state = ?", filter.State)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page.Size <= 0 {
		page.Size = 50
	}
	if page.Page <= 0 {
		page.Page = 1
	}

	offset := (page.Page - 1) * page.Size

	var models []DelegationModel
	if err := query.Offset(offset).Limit(page.Size).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, 0, err
	}

	delegations := make([]*Delegation, len(models))
	for i, model := range models {
		d, err := model.ToDelegation()
		if err != nil {
			return nil, 0, err
		}
		delegations[i] = d
	}

	return delegations, int(total), nil
}

// GetActiveDelegationsForUser gets active delegations where user is delegate
func (s *SQLDelegationStore) GetActiveDelegationsForUser(userID string) ([]*Delegation, error) {
	var models []DelegationModel
	if err := s.db.Where("delegate = ? AND state = ?", userID, DelegationStateActive).Find(&models).Error; err != nil {
		return nil, err
	}

	delegations := make([]*Delegation, len(models))
	for i, model := range models {
		d, err := model.ToDelegation()
		if err != nil {
			return nil, err
		}
		delegations[i] = d
	}

	return delegations, nil
}

// GetActiveDelegationsByUser gets active delegations where user is delegator
func (s *SQLDelegationStore) GetActiveDelegationsByUser(userID string) ([]*Delegation, error) {
	var models []DelegationModel
	if err := s.db.Where("delegator = ? AND state = ?", userID, DelegationStateActive).Find(&models).Error; err != nil {
		return nil, err
	}

	delegations := make([]*Delegation, len(models))
	for i, model := range models {
		d, err := model.ToDelegation()
		if err != nil {
			return nil, err
		}
		delegations[i] = d
	}

	return delegations, nil
}

// IncrementUsage increments the usage count of a delegation
func (s *SQLDelegationStore) IncrementUsage(id string) error {
	return s.db.Model(&DelegationModel{}).Where("id = ?", id).
		UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error
}

// SQLNotificationStore implements NotificationStore using SQL
type SQLNotificationStore struct {
	db *gorm.DB
	mu  sync.Mutex
}

// NewSQLNotificationStore creates a new SQL notification store
func NewSQLNotificationStore(db *gorm.DB) (*SQLNotificationStore, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	return &SQLNotificationStore{db: db}, nil
}

// RecordNotification records a notification
func (s *SQLNotificationStore) RecordNotification(recipient string, channel NotificationChannel, event NotificationEvent) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	model := &NotificationModel{
		Recipient: recipient,
		Channel:   string(channel),
		EventJSON: eventJSON,
		Read:      false,
		CreatedAt: time.Now(),
	}

	return s.db.Create(model).Error
}

// GetNotificationCount gets notification count for rate limiting
func (s *SQLNotificationStore) GetNotificationCount(recipient string, channel NotificationChannel, duration time.Duration) (int, error) {
	since := time.Now().Add(-duration)
	var count int64
	err := s.db.Model(&NotificationModel{}).
		Where("recipient = ? AND channel = ? AND created_at > ?", recipient, channel, since).
		Count(&count).Error
	return int(count), err
}

// GetNotifications gets notifications for a recipient
func (s *SQLNotificationStore) GetNotifications(recipient string, limit int) ([]*NotificationRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	var models []NotificationModel
	if err := s.db.Where("recipient = ?", recipient).
		Order("created_at DESC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, err
	}

	records := make([]*NotificationRecord, len(models))
	for i, model := range models {
		var event NotificationEvent
		if err := json.Unmarshal(model.EventJSON, &event); err != nil {
			continue
		}

		records[i] = &NotificationRecord{
			ID:        fmt.Sprintf("%d", model.ID),
			Recipient: model.Recipient,
			Channel:   NotificationChannel(model.Channel),
			Event:     event,
			Read:      model.Read,
			ReadAt:    model.ReadAt,
			CreatedAt: model.CreatedAt,
		}
	}

	return records, nil
}

// MarkAsRead marks a notification as read
func (s *SQLNotificationStore) MarkAsRead(id string) error {
	now := time.Now()
	return s.db.Model(&NotificationModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"read":    true,
			"read_at": &now,
		}).Error
}
