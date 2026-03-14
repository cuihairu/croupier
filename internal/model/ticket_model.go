package model

import (
	"context"

	"gorm.io/gorm"
)

// TicketModel manages ticket entities for the ticket module.
type TicketModel struct {
	db *gorm.DB
}

// NewTicketModel creates helper.
func NewTicketModel(db *gorm.DB) *TicketModel {
	return &TicketModel{db: db}
}

// ListTicketsOptions controls filtering.
type TicketQueryOptions struct {
	PaginationOptions
	Status   string
	Category string
	Priority string
	Assignee string
}

// Create inserts a ticket.
func (m *TicketModel) Create(ctx context.Context, ticket *Ticket) error {
	return m.db.WithContext(ctx).Create(ticket).Error
}

// Update modifies a ticket.
func (m *TicketModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&Ticket{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes a ticket.
func (m *TicketModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Ticket{}, id).Error
}

// FindOne loads a ticket.
func (m *TicketModel) FindOne(ctx context.Context, id uint) (*Ticket, error) {
	var ticket Ticket
	if err := m.db.WithContext(ctx).First(&ticket, id).Error; err != nil {
		return nil, err
	}
	return &ticket, nil
}

// List returns paginated tickets.
func (m *TicketModel) List(ctx context.Context, opts TicketQueryOptions) ([]Ticket, int64, error) {
	opts.PaginationOptions.Normalize()
	var (
		items []Ticket
		total int64
	)

	query := m.db.WithContext(ctx).Model(&Ticket{})
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Priority != "" {
		query = query.Where("priority = ?", opts.Priority)
	}
	if opts.Assignee != "" {
		query = query.Where("assignee = ?", opts.Assignee)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("updated_at DESC").
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// CreateComment inserts ticket comment.
func (m *TicketModel) CreateComment(ctx context.Context, comment *TicketComment) error {
	return m.db.WithContext(ctx).Create(comment).Error
}

// ListComments fetches ticket comments.
func (m *TicketModel) ListComments(ctx context.Context, ticketID uint) ([]TicketComment, error) {
	var comments []TicketComment
	err := m.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}
