package model

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"github.com/cuihairu/croupier/internal/dbenum"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MessageModel exposes CRUD helpers for messages.
type MessageModel struct {
	db *gorm.DB
}

// NewMessageModel creates a message model.
func NewMessageModel(db *gorm.DB) *MessageModel {
	return &MessageModel{db: db}
}

// ListMessagesOptions controls pagination/filtering.
type ListMessagesOptions struct {
	Page     int
	PageSize int
	Type     string
	Status   dbenum.MessageStatus // -1 = no filter; 0 (unread) IS a valid filter
	To       string
}

// NewListMessagesOptions returns options with status unfiltered.
func NewListMessagesOptions() ListMessagesOptions {
	return ListMessagesOptions{Status: -1}
}

// List returns paginated messages with filters applied.
func (m *MessageModel) List(ctx context.Context, opts ListMessagesOptions) ([]Message, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}

	query := m.db.WithContext(ctx).Model(&Message{})
	if opts.Type != "" {
		query = query.Where("type = ?", opts.Type)
	}
	if opts.Status >= 0 {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.To != "" {
		query = query.Where("recipient = ?", opts.To)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var messages []Message
	offset := (opts.Page - 1) * opts.PageSize
	if err := query.Order("id DESC").Offset(offset).Limit(opts.PageSize).Find(&messages).Error; err != nil {
		return nil, 0, err
	}
	return messages, total, nil
}

// Create inserts a new message.
func (m *MessageModel) Create(ctx context.Context, msg *Message) error {
	return m.db.WithContext(ctx).Create(msg).Error
}

// FindOne fetches a message by ID.
func (m *MessageModel) FindOne(ctx context.Context, id uint) (*Message, error) {
	var msg Message
	if err := m.db.WithContext(ctx).First(&msg, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("message not found")
		}
		return nil, err
	}
	return &msg, nil
}

// MarkRead marks a message as read.
func (m *MessageModel) MarkRead(ctx context.Context, id uint) error {
	now := time.Now().UTC()
	return m.db.WithContext(ctx).
		Model(&Message{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":  dbenum.MessageStatusRead,
			"read_at": now,
		}).Error
}

// CountUnread returns number of unread messages (optionally filtered by recipient).
func (m *MessageModel) CountUnread(ctx context.Context, to string) (int64, error) {
	query := m.db.WithContext(ctx).Model(&Message{}).Where("status = ?", dbenum.MessageStatusUnread)
	if to != "" {
		query = query.Where("recipient = ?", to)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Recent returns last N messages.
func (m *MessageModel) Recent(ctx context.Context, limit int, to string) ([]Message, error) {
	if limit <= 0 {
		limit = 20
	}
	query := m.db.WithContext(ctx).Model(&Message{}).Order("id DESC").Limit(limit)
	if to != "" {
		query = query.Where("recipient = ?", to)
	}
	var messages []Message
	if err := query.Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

// EncodeData converts arbitrary payload into datatypes.JSON.
func EncodeData(data interface{}) (datatypes.JSON, error) {
	if data == nil {
		return datatypes.JSON([]byte("null")), nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(bytes), nil
}
