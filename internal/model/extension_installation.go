package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ExtensionInstallation struct {
	gorm.Model
	InstallationKey string         `gorm:"size:191;uniqueIndex;not null" json:"installation_key"`
	ExtensionID     string         `gorm:"size:128;not null;index" json:"extension_id"`
	ReleaseVersion  string         `gorm:"size:64;not null" json:"release_version"`
	ScopeType       string         `gorm:"size:32;not null;index" json:"scope_type"`
	ScopeID         string         `gorm:"size:128;not null;index" json:"scope_id"`
	TargetType      string         `gorm:"size:32;not null;index" json:"target_type"`
	TargetID        string         `gorm:"size:128;index" json:"target_id"`
	Status          string         `gorm:"size:32;not null;index" json:"status"`
	DesiredState    string         `gorm:"size:32;not null;index" json:"desired_state"`
	Enabled         bool           `gorm:"not null;default:false;index" json:"enabled"`
	ConfigJSON      datatypes.JSON `gorm:"type:json" json:"config_json"`
	SecretRefsJSON  datatypes.JSON `gorm:"type:json" json:"secret_refs_json"`
	LastError       string         `gorm:"type:text" json:"last_error"`
	InstalledBy     string         `gorm:"size:128" json:"installed_by"`
	InstalledAtUnix int64          `gorm:"not null;default:0;index" json:"installed_at_unix"`
}

func (ExtensionInstallation) TableName() string {
	return "extension_installations"
}
