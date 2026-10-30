package approvals

import (
	"errors"
	"strings"
	"time"

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

// Approve updates an approval state to approved.
//
// ARCH-001 D1：pending → 终态用单条条件 UPDATE 原子转换（`WHERE id=? AND
// state='pending'`），RowsAffected==0 说明记录已不在 pending（已批准/已拒绝，
// 或重复点击）——此时原样返回当前记录 + ErrAlreadyReviewed，调用方据此
// 返回 409 approval_already_reviewed，且不再触发续跑/审计/通知副作用。
// 该写法在 sqlite/mysql/postgres 上均可移植，无需 SELECT ... FOR UPDATE。
func (s *SQLStore) Approve(id, operator string) (*Approval, error) {
	now := time.Now()
	res := s.db.Model(&ApprovalModel{}).
		Where("id = ? AND state = ?", id, "pending").
		Updates(map[string]any{
			"state":       "approved",
			"approver":    strings.TrimSpace(operator),
			"reviewed_at": now,
			"updated_at":  now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, s.alreadyReviewedOrNotFound(id)
	}
	model, err := s.getByID(id)
	if err != nil {
		return nil, err
	}
	return model.ToApproval(), nil
}

// Reject updates an approval state to rejected with reason.
//
// ARCH-001 D1：同 Approve，单条条件 UPDATE + RowsAffected 检查。
func (s *SQLStore) Reject(id, reason, operator string) (*Approval, error) {
	now := time.Now()
	res := s.db.Model(&ApprovalModel{}).
		Where("id = ? AND state = ?", id, "pending").
		Updates(map[string]any{
			"state":       "rejected",
			"reason":      reason,
			"approver":    strings.TrimSpace(operator),
			"reviewed_at": now,
			"updated_at":  now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, s.alreadyReviewedOrNotFound(id)
	}
	model, err := s.getByID(id)
	if err != nil {
		return nil, err
	}
	return model.ToApproval(), nil
}

// alreadyReviewedOrNotFound 区分「记录存在但已处理」与「记录不存在」。
// 前者返回 ErrAlreadyReviewed 携带当前记录（D1 语义），后者返回 not found。
func (s *SQLStore) alreadyReviewedOrNotFound(id string) error {
	model, err := s.getByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("not found")
		}
		return err
	}
	return wrapAlreadyReviewed(model.ToApproval())
}

// getByID 读取单条审批记录；不存在时返回 gorm.ErrRecordNotFound。
func (s *SQLStore) getByID(id string) (*ApprovalModel, error) {
	var model ApprovalModel
	if err := s.db.Where("id = ?", id).First(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
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

// ClaimForDispatch claims the single approved continuation for an approval
// (ARCH-001 D7). The conditional update persists dispatch intent before the
// continuation runs. Existing claims never succeed again.
func (s *SQLStore) ClaimForDispatch(id, token, payloadDigest string) (*Approval, error) {
	if token == "" {
		return nil, errors.New("claim token is required")
	}
	now := time.Now()
	res := s.db.Model(&ApprovalModel{}).
		Where("id = ? AND state = ? AND claim_token = ''", id, "approved").
		Updates(map[string]any{
			"claim_status":   "claimed",
			"claim_token":    token,
			"claimed_at":     now,
			"payload_digest": payloadDigest,
			"updated_at":     now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		model, err := s.getByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("not found")
			}
			return nil, err
		}
		if model.State != "approved" {
			return nil, errors.New("approval is not approved")
		}
		return model.ToApproval(), wrapAlreadyReviewed(model.ToApproval())
	}
	model, err := s.getByID(id)
	if err != nil {
		return nil, err
	}
	return model.ToApproval(), nil
}
