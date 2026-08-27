package model

import (
	"gorm.io/gorm"
)

type ExtensionRelease struct {
	gorm.Model
	ExtensionID     string `gorm:"size:128;not null;index:idx_extension_release_version,priority:1" json:"extensionId"`
	Version         string `gorm:"size:64;not null;index:idx_extension_release_version,priority:2" json:"version"`
	ReleaseChannel  string `gorm:"size:32;not null;index" json:"releaseChannel"`
	ManifestJSON    JSON   `gorm:"type:json;not null" json:"manifestJson"`
	PackageRef      string `gorm:"size:512" json:"packageRef"`
	Checksum        string `gorm:"size:128" json:"checksum"`
	MinCoreVersion  string `gorm:"size:64" json:"minCoreVersion"`
	Changelog       string `gorm:"type:text" json:"changelog"`
	PublishedAtUnix int64  `gorm:"not null;default:0;index" json:"publishedAtUnix"`
}

func (ExtensionRelease) TableName() string {
	return "extension_releases"
}
