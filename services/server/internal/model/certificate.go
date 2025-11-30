package model

import (
	"time"

	"gorm.io/gorm"
)

// Certificate stores TLS certificate metadata.
type Certificate struct {
	gorm.Model
	Domain         string `gorm:"size:255;uniqueIndex"`
	CertificatePEM string `gorm:"type:text"`
	PrivateKeyPEM  string `gorm:"type:text"`
	Issuer         string `gorm:"size:255"`
	ExpiresAt      time.Time
	Status         string `gorm:"size:32;index"` // active, expiring, expired
	LastCheckedAt  *time.Time
	ErrorMessage   string `gorm:"type:text"`
}

func (Certificate) TableName() string {
	return "certificates"
}

// CertificateAlert stores alert thresholds for certificates.
type CertificateAlert struct {
	gorm.Model
	Domain          string `gorm:"size:255;index"`
	ThresholdDays   int    `gorm:"default:30"`
	Active          bool   `gorm:"default:true"`
	LastTriggeredAt *time.Time
}

func (CertificateAlert) TableName() string {
	return "certificate_alerts"
}
