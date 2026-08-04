package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// FunctionContract represents the normalized executable capability contract.
// It is the single source of truth for function capability, input/output schema,
// governance, and execution mode. It does NOT contain UI or page configuration.
type FunctionContract struct {
	gorm.Model
	GameID       string            `gorm:"size:64;uniqueIndex:idx_function_contract_scope"`
	Env          string            `gorm:"size:64;uniqueIndex:idx_function_contract_scope"`
	FunctionID   string            `gorm:"size:128;uniqueIndex:idx_function_contract_scope"`
	Version      string            `gorm:"size:32"`
	Enabled      bool              `gorm:"default:true"`
	Deprecated   bool              `gorm:"default:false"`
	ResourceKey  string            `gorm:"size:64;index"`
	OperationKey string            `gorm:"size:64"`
	Capability   string            `gorm:"size:32"`   // collection_query|item_query|create|update|delete|action|task|report
	Execution    string            `gorm:"size:32"`   // sync|task
	Approval     datatypes.JSONMap `gorm:"type:json"` // ApprovalPolicy
	Risk         string            `gorm:"size:32"`   // safe|warning|high|danger
	Permission   string            `gorm:"size:128"`
	InputSchema  datatypes.JSON    `gorm:"type:json"`
	OutputSchema datatypes.JSON    `gorm:"type:json"`
	Summary      datatypes.JSONMap `gorm:"type:json"` // LocalizedText map
	Description  datatypes.JSONMap `gorm:"type:json"` // LocalizedText map
	Tags         datatypes.JSON    `gorm:"type:json"` // string array
	Source       string            `gorm:"size:32"`   // sdk|openapi|catalog
	SourceDigest string            `gorm:"size:64"`   // SHA256 of source descriptor
	Diagnostics  datatypes.JSON    `gorm:"type:json"` // Diagnostic array
	UpdatedAt    time.Time
	UpdatedBy    string `gorm:"size:64"` // user or system
}

// ResourceCapability aggregates function capabilities around a business resource.
// It is NOT a database table and NOT a direct business data API.
type ResourceCapability struct {
	gorm.Model
	GameID      string            `gorm:"size:64;uniqueIndex:idx_resource_capability_scope"`
	Env         string            `gorm:"size:64;uniqueIndex:idx_resource_capability_scope"`
	ResourceKey string            `gorm:"size:64;uniqueIndex:idx_resource_capability_scope"`
	Labels      datatypes.JSONMap `gorm:"type:json"` // LocalizedText map
	Description datatypes.JSONMap `gorm:"type:json"` // LocalizedText map
	CategoryKey string            `gorm:"size:64;index"`
	Tags        datatypes.JSON    `gorm:"type:json"` // string array
	SemanticsID uint              `gorm:"index"`     // FK to CapabilitySemantics
	UpdatedAt   time.Time
	UpdatedBy   string `gorm:"size:64"`
}

// CapabilitySemantics holds the verified business semantics for a resource.
// It describes identity, collection, CRUD lifecycle, actions, tasks, and reports.
// It does NOT describe columns, button positions, page layout, or mapping.
type CapabilitySemantics struct {
	gorm.Model
	GameID      string `gorm:"size:64;uniqueIndex:idx_capability_semantics_scope"`
	Env         string `gorm:"size:64;uniqueIndex:idx_capability_semantics_scope"`
	ResourceKey string `gorm:"size:64;uniqueIndex:idx_capability_semantics_scope"`
	Version     int    `gorm:"default:1"`

	// Identity semantics
	IdentityField     string `gorm:"size:64"`  // e.g., "player_id", "id"
	IdentityFieldType string `gorm:"size:32"`  // string|number|integer
	IdentityPath      string `gorm:"size:128"` // JSON path for nested identity

	// Collection semantics
	CollectionQueryID uint   `gorm:"index"`    // FK to FunctionContract for list query
	CollectionPath    string `gorm:"size:128"` // API path, e.g., "/players"
	PageFieldName     string `gorm:"size:64"`  // pagination field name, default "page"
	PageSizeFieldName string `gorm:"size:64"`  // page size field name, default "page_size"
	ItemsFieldName    string `gorm:"size:64"`  // items array field, default "items"
	TotalFieldName    string `gorm:"size:64"`  // total count field, default "total"

	// Item query semantics
	ItemQueryID uint   `gorm:"index"`    // FK to FunctionContract for item query
	ItemPath    string `gorm:"size:128"` // API path template, e.g., "/players/{player_id}"

	// Lifecycle semantics (CRUD)
	CreateID uint `gorm:"index"` // FK to FunctionContract for create
	UpdateID uint `gorm:"index"` // FK to FunctionContract for update
	DeleteID uint `gorm:"index"` // FK to FunctionContract for delete

	// Action semantics
	Actions datatypes.JSON `gorm:"type:json"` // ActionSemantic array

	// Task semantics
	Tasks datatypes.JSON `gorm:"type:json"` // TaskSemantic array

	// Report semantics
	Reports datatypes.JSON `gorm:"type:json"` // ReportSemantic array

	// Source and diagnostics
	Source       string         `gorm:"size:32"`   // openapi_rest|sdk_explicit|platform_review
	SourceDigest string         `gorm:"size:64"`   // SHA256 of source
	Diagnostics  datatypes.JSON `gorm:"type:json"` // Diagnostic array

	// Field-level provenance tracking
	// Each semantic field can have its own provenance record
	Provenance datatypes.JSON `gorm:"type:json"` // map[string]SemanticProvenance

	// Unresolved conflicts
	Conflicts datatypes.JSON `gorm:"type:json"` // []SemanticConflict

	UpdatedAt time.Time
	UpdatedBy string `gorm:"size:64"`
}

// CapabilitySemanticVersion stores version history of CapabilitySemantics.
// Each semantic change creates a new version for audit and rollback.
type CapabilitySemanticVersion struct {
	gorm.Model
	SemanticsID  uint `gorm:"index"`
	Version      int
	Semantics    datatypes.JSON `gorm:"type:json"` // Snapshot of CapabilitySemantics
	SourceDigest string         `gorm:"size:64"`
	ChangeReason string         `gorm:"size:256"`
	CreatedBy    string         `gorm:"size:64"`
}
