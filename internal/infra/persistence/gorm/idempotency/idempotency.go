package idempotency

import (
	"crypto/md5"
	"fmt"
	"gorm.io/gorm"
	"time"
)

// IdempotencyRecord represents an idempotency key record
type IdempotencyRecord struct {
	ID           uint   `gorm:"primaryKey"`
	Key          string `gorm:"column:idempotency_key;uniqueIndex;size:255"`
	UserID       string `gorm:"column:user_id;index;size:100"`
	FunctionID   string `gorm:"column:function_id;index;size:100"`
	RequestHash  string `gorm:"column:request_hash;size:32"` // MD5 hash of request parameters
	ResponseBody string `gorm:"column:response_body;type:text"`
	StatusCode   int    `gorm:"column:status_code"`
	ExpiresAt    time.Time `gorm:"column:expires_at;index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (IdempotencyRecord) TableName() string {
	return "idempotency_records"
}

// Manager handles idempotency key operations
type Manager struct {
	db  *gorm.DB
	ttl time.Duration
}

func NewManager(db *gorm.DB, ttl time.Duration) *Manager {
	if ttl == 0 {
		ttl = 24 * time.Hour // Default 24 hour TTL
	}
	return &Manager{db: db, ttl: ttl}
}

// AutoMigrate creates the idempotency table
func (m *Manager) AutoMigrate() error {
	return m.db.AutoMigrate(&IdempotencyRecord{})
}

// ComputeRequestHash computes MD5 hash of request parameters for deduplication
func (m *Manager) ComputeRequestHash(params map[string]interface{}) string {
	// Simple serialization for hashing - in production you might want more sophisticated approach
	str := fmt.Sprintf("%v", params)
	return fmt.Sprintf("%x", md5.Sum([]byte(str)))
}

// Get retrieves an existing idempotency record if it exists and hasn't expired
func (m *Manager) Get(key, userID, functionID string, requestHash string) (*IdempotencyRecord, error) {
	var record IdempotencyRecord
	err := m.db.Where("idempotency_key = ? AND user_id = ? AND function_id = ? AND expires_at > ?",
		key, userID, functionID, time.Now()).First(&record).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found, not an error
		}
		return nil, err
	}

	// Verify request hash matches for additional safety
	if record.RequestHash != requestHash {
		return nil, nil // Different request, treat as new
	}

	return &record, nil
}

// Store saves a new idempotency record
func (m *Manager) Store(key, userID, functionID, requestHash, responseBody string, statusCode int) error {
	record := IdempotencyRecord{
		Key:          key,
		UserID:       userID,
		FunctionID:   functionID,
		RequestHash:  requestHash,
		ResponseBody: responseBody,
		StatusCode:   statusCode,
		ExpiresAt:    time.Now().Add(m.ttl),
	}

	return m.db.Create(&record).Error
}

// CleanExpired removes expired idempotency records
func (m *Manager) CleanExpired() error {
	return m.db.Where("expires_at < ?", time.Now()).Delete(&IdempotencyRecord{}).Error
}

// GenerateKey generates a default idempotency key based on user, function, and request
func (m *Manager) GenerateKey(userID, functionID string, params map[string]interface{}) string {
	requestHash := m.ComputeRequestHash(params)
	return fmt.Sprintf("%s:%s:%s", userID, functionID, requestHash[:8])
}