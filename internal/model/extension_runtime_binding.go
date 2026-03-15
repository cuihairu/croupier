package model

import "gorm.io/gorm"

type ExtensionRuntimeBinding struct {
	gorm.Model
	InstallationID uint   `gorm:"not null;index;uniqueIndex:uk_extension_runtime_binding_installation_key,priority:1" json:"installation_id"`
	BindingType    string `gorm:"size:32;not null;index" json:"binding_type"`
	BindingKey     string `gorm:"size:191;not null;uniqueIndex:uk_extension_runtime_binding_installation_key,priority:2" json:"binding_key"`
	TargetRef      string `gorm:"size:255" json:"target_ref"`
	SpecJSON       string `gorm:"type:longtext" json:"spec_json"`
	Status         string `gorm:"size:32;not null;index" json:"status"`
	LastError      string `gorm:"type:text" json:"last_error"`
}

func (ExtensionRuntimeBinding) TableName() string {
	return "extension_runtime_bindings"
}
