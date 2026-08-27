package model

import (
	"gorm.io/gorm"
)

type ExtensionInstallation struct {
	gorm.Model
	InstallationKey string `gorm:"size:191;uniqueIndex;not null" json:"installationKey"`
	ExtensionID     string `gorm:"size:128;not null;index" json:"extensionId"`
	ReleaseVersion  string `gorm:"size:64;not null" json:"releaseVersion"`
	ScopeType       string `gorm:"size:32;not null;index" json:"scopeType"`
	ScopeID         string `gorm:"size:128;not null;index" json:"scopeId"`
	TargetType      string `gorm:"size:32;not null;index" json:"targetType"`
	TargetID        string `gorm:"size:128;index" json:"targetId"`
	Status          string `gorm:"size:32;not null;index" json:"status"`
	DesiredState    string `gorm:"size:32;not null;index" json:"desiredState"`
	Enabled         bool   `gorm:"not null;default:false;index" json:"enabled"`
	ConfigJSON      JSON   `gorm:"type:json" json:"configJson"`
	SecretRefsJSON  JSON   `gorm:"type:json" json:"secretRefsJson"`
	LastError       string `gorm:"type:text" json:"lastError"`
	InstalledBy     string `gorm:"size:128" json:"installedBy"`
	InstalledAtUnix int64  `gorm:"not null;default:0;index" json:"installedAtUnix"`
}

func (ExtensionInstallation) TableName() string {
	return "extension_installations"
}
