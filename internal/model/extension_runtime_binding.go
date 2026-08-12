package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ExtensionRuntimeBinding struct {
	gorm.Model
	InstallationID uint           `gorm:"not null;index;uniqueIndex:uk_extension_runtime_binding_installation_key,priority:1" json:"installationId"`
	BindingType    string         `gorm:"size:32;not null;index" json:"bindingType"`
	BindingKey     string         `gorm:"size:191;not null;uniqueIndex:uk_extension_runtime_binding_installation_key,priority:2" json:"bindingKey"`
	TargetRef      string         `gorm:"size:255" json:"targetRef"`
	SpecJSON       datatypes.JSON `gorm:"type:json" json:"specJson"`
	Status         string         `gorm:"size:32;not null;index" json:"status"`
	LastError      string         `gorm:"type:text" json:"lastError"`
}

func (ExtensionRuntimeBinding) TableName() string {
	return "extension_runtime_bindings"
}
