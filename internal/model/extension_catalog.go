package model

import "gorm.io/gorm"

type ExtensionCatalog struct {
	gorm.Model
	ExtensionID   string `gorm:"size:128;uniqueIndex;not null" json:"extension_id"`
	Name          string `gorm:"size:128;not null" json:"name"`
	DisplayName   string `gorm:"size:255;not null" json:"display_name"`
	Vendor        string `gorm:"size:128;not null" json:"vendor"`
	Kind          string `gorm:"size:32;not null;index" json:"kind"`
	Summary       string `gorm:"type:text" json:"summary"`
	IconURL       string `gorm:"size:512" json:"icon_url"`
	HomepageURL   string `gorm:"size:512" json:"homepage_url"`
	Status        string `gorm:"size:32;not null;index" json:"status"`
	LatestVersion string `gorm:"size:64" json:"latest_version"`
}

func (ExtensionCatalog) TableName() string {
	return "extension_catalogs"
}
