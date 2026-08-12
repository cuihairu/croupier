package model

import "gorm.io/gorm"

type ExtensionCatalog struct {
	gorm.Model
	ExtensionID   string `gorm:"size:128;uniqueIndex;not null" json:"extensionId"`
	Name          string `gorm:"size:128;not null" json:"name"`
	DisplayName   string `gorm:"size:255;not null" json:"displayName"`
	Vendor        string `gorm:"size:128;not null" json:"vendor"`
	Kind          string `gorm:"size:32;not null;index" json:"kind"`
	Summary       string `gorm:"type:text" json:"summary"`
	IconURL       string `gorm:"size:512" json:"iconUrl"`
	HomepageURL   string `gorm:"size:512" json:"homepageUrl"`
	Status        string `gorm:"size:32;not null;index" json:"status"`
	LatestVersion string `gorm:"size:64" json:"latestVersion"`
}

func (ExtensionCatalog) TableName() string {
	return "extension_catalogs"
}
