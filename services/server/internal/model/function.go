package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Function represents a registered callable unit.
type Function struct {
	gorm.Model
	FunctionID  string            `gorm:"size:64;uniqueIndex"`
	Name        string            `gorm:"size:128"`
	Description string            `gorm:"type:text"`
	Category    string            `gorm:"size:64;index"`
	GameID      string            `gorm:"size:64;index"`
	Status      int               `gorm:"index"`
	Version     string            `gorm:"size:32"`
	Instances   int               `gorm:"default:0"`
	Runtime     string            `gorm:"size:64"`
	Entry       string            `gorm:"size:128"`
	Schema      datatypes.JSONMap `gorm:"type:json"`     // Legacy descriptor format (deprecated)
	Metadata    datatypes.JSONMap `gorm:"type:json"`     // Additional metadata
	SpecFormat  string            `gorm:"size:32;index"` // Format: "legacy", "openapi3.0.3"
	OpenAPISpec datatypes.JSONMap `gorm:"type:json"`     // OpenAPI 3.0.3 Operation object
}

// FunctionDescriptor stores detailed descriptor versions.
type FunctionDescriptor struct {
	gorm.Model
	FunctionID string            `gorm:"size:64;index"`
	Version    string            `gorm:"size:32"`
	Input      datatypes.JSONMap `gorm:"type:json"`
	Output     datatypes.JSONMap `gorm:"type:json"`
	Schema     datatypes.JSONMap `gorm:"type:json"`
}

// Descriptor represents reusable descriptors independent of a function.
type Descriptor struct {
	gorm.Model
	DescriptorID string            `gorm:"size:64;uniqueIndex"`
	Name         string            `gorm:"size:128"`
	Description  string            `gorm:"type:text"`
	Category     string            `gorm:"size:64"`
	Schema       datatypes.JSONMap `gorm:"type:json"`
}

// FunctionInstance tracks runtime deployments.
type FunctionInstance struct {
	gorm.Model
	FunctionID string            `gorm:"size:64;index"`
	AgentID    string            `gorm:"size:64;index"`
	AgentName  string            `gorm:"size:128"`
	Status     string            `gorm:"size:32"`
	UpdatedAt  time.Time         `gorm:"autoUpdateTime"`
	Metadata   datatypes.JSONMap `gorm:"type:json"`
}

// FunctionPermission stores per-function permission mapping.
type FunctionPermission struct {
	gorm.Model
	FunctionID string         `gorm:"size:64;index"`
	GameID     string         `gorm:"size:64;index"`
	Env        string         `gorm:"size:64;index"`
	Resource   string         `gorm:"size:64"`
	Actions    datatypes.JSON `gorm:"type:json"`
	Roles      datatypes.JSON `gorm:"type:json"`
}

// PendingFunction stores functions awaiting approval.
type PendingFunction struct {
	gorm.Model
	FunctionID  string            `gorm:"size:64;uniqueIndex"`
	Payload     datatypes.JSONMap `gorm:"type:json"`
	RequestedBy string            `gorm:"size:64"`
	Status      string            `gorm:"size:32"`
}

func (Function) TableName() string {
	return "functions"
}

func (FunctionDescriptor) TableName() string {
	return "function_descriptors"
}

func (Descriptor) TableName() string {
	return "descriptors"
}

func (FunctionInstance) TableName() string {
	return "function_instances"
}

func (FunctionPermission) TableName() string {
	return "function_permissions"
}

func (PendingFunction) TableName() string {
	return "pending_functions"
}
