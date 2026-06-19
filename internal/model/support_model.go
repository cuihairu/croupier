package model

import (
	"context"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
)

// SupportModel wraps support data operations.
type SupportModel struct {
	db *gorm.DB
}

// NewSupportModel creates helper.
func NewSupportModel(db *gorm.DB) *SupportModel {
	return &SupportModel{db: db}
}

// ListTicketsOptions controls filtering.
type ListTicketsOptions struct {
	PaginationOptions
	Status string
}

// CreateTicket inserts ticket.
func (m *SupportModel) CreateTicket(ctx context.Context, ticket *SupportTicket) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(ticket).Error
}

// UpdateTicket updates ticket fields.
func (m *SupportModel) UpdateTicket(ctx context.Context, id uint, updates map[string]interface{}) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&SupportTicket{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteTicket deletes ticket.
func (m *SupportModel) DeleteTicket(ctx context.Context, id uint) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Delete(&SupportTicket{}, id).Error
}

// ListTickets returns paginated tickets.
func (m *SupportModel) ListTickets(ctx context.Context, opts ListTicketsOptions) ([]SupportTicket, int64, error) {
	opts.PaginationOptions.Normalize()

	var (
		items []SupportTicket
		total int64
	)

	query := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&SupportTicket{})
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
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
func (m *SupportModel) CreateComment(ctx context.Context, comment *SupportComment) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(comment).Error
}

// ListComments fetches ticket comments.
func (m *SupportModel) ListComments(ctx context.Context, ticketID uint) ([]SupportComment, error) {
	var comments []SupportComment
	err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}

// CreateFAQ inserts FAQ.
func (m *SupportModel) CreateFAQ(ctx context.Context, faq *SupportFAQ) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(faq).Error
}

// ListFAQs returns FAQs.
func (m *SupportModel) ListFAQs(ctx context.Context) ([]SupportFAQ, error) {
	var faqs []SupportFAQ
	err := dbctx.Resolve(ctx, m.db).WithContext(ctx).Order("sort DESC, id DESC").Find(&faqs).Error
	return faqs, err
}

// CreateFeedback inserts support feedback.
func (m *SupportModel) CreateFeedback(ctx context.Context, feedback *SupportFeedback) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(feedback).Error
}

// ListFeedback returns support feedback entries.
func (m *SupportModel) ListFeedback(ctx context.Context) ([]SupportFeedback, error) {
	var entries []SupportFeedback
	err := dbctx.Resolve(ctx, m.db).WithContext(ctx).Order("created_at DESC").Find(&entries).Error
	return entries, err
}
