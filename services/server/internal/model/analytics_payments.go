package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// PaymentTransaction records a single payment event.
type PaymentTransaction struct {
	gorm.Model
	TransactionID string `gorm:"size:64;uniqueIndex"`
	GameID        string `gorm:"size:64;index"`
	Env           string `gorm:"size:32;index"`
	UserID        string `gorm:"size:64;index"`
	ProductID     string `gorm:"size:64;index"`
	ProductName   string `gorm:"size:128"`
	Amount        float64
	Currency      string            `gorm:"size:16"`
	Status        string            `gorm:"size:32;index"`
	PaymentMethod string            `gorm:"size:32"`
	Metadata      datatypes.JSONMap `gorm:"type:json"`
	OccurredAt    time.Time         `gorm:"index"`
}

// ProductTrend stores aggregated metrics per product.
type ProductTrend struct {
	gorm.Model
	GameID      string `gorm:"size:64;index"`
	Env         string `gorm:"size:32;index"`
	ProductID   string `gorm:"size:64;index:idx_product_window,unique"`
	ProductName string `gorm:"size:128"`
	Revenue     float64
	Sales       int
	Growth      float64
	WindowStart time.Time `gorm:"index:idx_product_window,unique"`
	WindowEnd   time.Time
}

func (PaymentTransaction) TableName() string {
	return "payment_transactions"
}

func (ProductTrend) TableName() string {
	return "payment_product_trends"
}
