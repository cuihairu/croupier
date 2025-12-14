package approvals

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// SQLStore implements Store interface using GORM for PostgreSQL/SQLite
type SQLStore struct {
	db *gorm.DB
}

// NewSQLStore creates a new SQLStore with the given GORM database connection
func NewSQLStore(db *gorm.DB) (*SQLStore, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	// Auto migrate the schema
	if err := AutoMigrate(db); err != nil {
		return nil, err
	}

	return &SQLStore{db: db}, nil
}

// List returns approvals matching the filter with pagination
func (s *SQLStore) List(f Filter, p Page) ([]*Approval, int, error) {
	var models []ApprovalModel
	var total int64

	query := s.buildFilterQuery(f)

	// Count total records
	if err := query.Model(&ApprovalModel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and sorting
	if p.Size <= 0 {
		p.Size = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}

	offset := (p.Page - 1) * p.Size
	orderClause := s.buildOrderClause(p.Sort)

	if err := query.Offset(offset).Limit(p.Size).Order(orderClause).Find(&models).Error; err != nil {
		return nil, 0, err
	}

	// Convert to Approval
	approvals := make([]*Approval, len(models))
	for i, model := range models {
		approvals[i] = model.ToApproval()
	}

	return approvals, int(total), nil
}

// Get retrieves an approval by ID
func (s *SQLStore) Get(id string) (*Approval, error) {
	var model ApprovalModel
	if err := s.db.Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found")
		}
		return nil, err
	}
	return model.ToApproval(), nil
}

// Approve updates an approval state to approved
func (s *SQLStore) Approve(id string) (*Approval, error) {
	var model ApprovalModel
	if err := s.db.Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found")
		}
		return nil, err
	}

	// Update state
	model.State = "approved"
	if err := s.db.Save(&model).Error; err != nil {
		return nil, err
	}

	return model.ToApproval(), nil
}

// Reject updates an approval state to rejected with reason
func (s *SQLStore) Reject(id, reason string) (*Approval, error) {
	var model ApprovalModel
	if err := s.db.Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("not found")
		}
		return nil, err
	}

	// Update state and reason
	model.State = "rejected"
	model.Reason = reason
	if err := s.db.Save(&model).Error; err != nil {
		return nil, err
	}

	return model.ToApproval(), nil
}

// buildFilterQuery builds GORM query based on filter
func (s *SQLStore) buildFilterQuery(f Filter) *gorm.DB {
	query := s.db.Model(&ApprovalModel{})

	if f.State != "" {
		query = query.Where("state = ?", f.State)
	}
	if f.FunctionID != "" {
		query = query.Where("function_id = ?", f.FunctionID)
	}
	if f.GameID != "" {
		query = query.Where("game_id = ?", f.GameID)
	}
	if f.Env != "" {
		query = query.Where("env = ?", f.Env)
	}
	if f.Actor != "" {
		query = query.Where("actor = ?", f.Actor)
	}
	if f.Mode != "" {
		query = query.Where("mode = ?", f.Mode)
	}

	return query
}

// buildOrderClause builds SQL ORDER BY clause based on sort parameter
func (s *SQLStore) buildOrderClause(sort string) string {
	if sort == "" {
		return "updated_at DESC" // Default sort
	}

	// Parse sort format: "field asc|desc"
	parts := strings.Split(strings.ToLower(sort), " ")
	if len(parts) != 2 {
		return "updated_at DESC"
	}

	field, direction := parts[0], parts[1]

	// Validate field
	switch field {
	case "created_at", "updated_at", "id", "state", "actor", "function_id", "game_id":
		// Valid field
	default:
		field = "updated_at" // Default to updated_at for invalid field
	}

	// Validate direction
	if direction != "asc" && direction != "desc" {
		direction = "desc"
	}

	return field + " " + strings.ToUpper(direction)
}

// Create creates a new approval record
func (s *SQLStore) Create(approval *Approval) (*Approval, error) {
	if approval == nil {
		return nil, errors.New("approval is required")
	}

	model := FromApproval(approval)
	if err := s.db.Create(model).Error; err != nil {
		return nil, err
	}

	return model.ToApproval(), nil
}

// Update updates an existing approval
func (s *SQLStore) Update(approval *Approval) (*Approval, error) {
	if approval == nil {
		return nil, errors.New("approval is required")
	}

	model := FromApproval(approval)
	if err := s.db.Save(model).Error; err != nil {
		return nil, err
	}

	return model.ToApproval(), nil
}
