package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ExtensionRelease struct {
	gorm.Model
	ExtensionID     string         `gorm:"size:128;not null;index:idx_extension_release_version,priority:1" json:"extension_id"`
	Version         string         `gorm:"size:64;not null;index:idx_extension_release_version,priority:2" json:"version"`
	ReleaseChannel  string         `gorm:"size:32;not null;index" json:"release_channel"`
	ManifestJSON    datatypes.JSON `gorm:"type:json;not null" json:"manifest_json"`
	PackageRef      string         `gorm:"size:512" json:"package_ref"`
	Checksum        string         `gorm:"size:128" json:"checksum"`
	MinCoreVersion  string         `gorm:"size:64" json:"min_core_version"`
	Changelog       string         `gorm:"type:text" json:"changelog"`
	PublishedAtUnix int64          `gorm:"not null;default:0;index" json:"published_at_unix"`
}

func (ExtensionRelease) TableName() string {
	return "extension_releases"
}
