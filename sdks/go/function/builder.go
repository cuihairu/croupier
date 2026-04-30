// Package function provides builder patterns for constructing function metadata.
package function

import (
	"fmt"
)

// MetadataBuilder builds FunctionMetadata using the builder pattern.
type MetadataBuilder struct {
	metadata *FunctionMetadata
	errors   []error
}

// NewMetadataBuilder creates a new MetadataBuilder with default values.
func NewMetadataBuilder() *MetadataBuilder {
	return &MetadataBuilder{
		metadata: &FunctionMetadata{
			Tags:       []string{},
			Behavior:   &FunctionBehavior{},
			Security:   &FunctionSecurity{},
			Extensions: map[string]string{},
		},
		errors: []error{},
	}
}

// SetID sets the function ID.
func (b *MetadataBuilder) SetID(id string) *MetadataBuilder {
	b.metadata.ID = id
	return b
}

// SetVersion sets the function version.
func (b *MetadataBuilder) SetVersion(version string) *MetadataBuilder {
	b.metadata.Version = version
	return b
}

// SetCategory sets the function category.
func (b *MetadataBuilder) SetCategory(category string) *MetadataBuilder {
	b.metadata.Category = category
	return b
}

// SetTags sets the function tags.
func (b *MetadataBuilder) SetTags(tags ...string) *MetadataBuilder {
	b.metadata.Tags = tags
	return b
}

// AddTag adds a single tag.
func (b *MetadataBuilder) AddTag(tag string) *MetadataBuilder {
	b.metadata.Tags = append(b.metadata.Tags, tag)
	return b
}

// SetName sets the function display name.
func (b *MetadataBuilder) SetName(name string) *MetadataBuilder {
	b.metadata.Name = name
	return b
}

// SetDescription sets the function description.
func (b *MetadataBuilder) SetDescription(description string) *MetadataBuilder {
	b.metadata.Description = description
	return b
}

// SetInputSchema sets the input JSON Schema.
func (b *MetadataBuilder) SetInputSchema(schema string) *MetadataBuilder {
	b.metadata.InputSchema = schema
	return b
}

// SetOutputSchema sets the output JSON Schema.
func (b *MetadataBuilder) SetOutputSchema(schema string) *MetadataBuilder {
	b.metadata.OutputSchema = schema
	return b
}

// SetBehavior sets the function behavior.
func (b *MetadataBuilder) SetBehavior(behavior *FunctionBehavior) *MetadataBuilder {
	b.metadata.Behavior = behavior
	return b
}

// SetSecurity sets the function security.
func (b *MetadataBuilder) SetSecurity(security *FunctionSecurity) *MetadataBuilder {
	b.metadata.Security = security
	return b
}

// SetExtension sets an extension value.
func (b *MetadataBuilder) SetExtension(key, value string) *MetadataBuilder {
	if b.metadata.Extensions == nil {
		b.metadata.Extensions = make(map[string]string)
	}
	b.metadata.Extensions[key] = value
	return b
}

// Build validates and returns the FunctionMetadata.
func (b *MetadataBuilder) Build() (*FunctionMetadata, error) {
	// Validate required fields
	if b.metadata.ID == "" {
		b.errors = append(b.errors, fmt.Errorf("ID is required"))
	}
	if b.metadata.Behavior == nil {
		b.metadata.Behavior = &FunctionBehavior{}
	}
	if b.metadata.Security == nil {
		b.metadata.Security = &FunctionSecurity{}
	}

	if len(b.errors) > 0 {
		return nil, fmt.Errorf("validation failed: %v", b.errors)
	}

	return b.metadata, nil
}

// MustBuild returns the FunctionMetadata or panics if validation fails.
func (b *MetadataBuilder) MustBuild() *FunctionMetadata {
	metadata, err := b.Build()
	if err != nil {
		panic(err)
	}
	return metadata
}

// BehaviorBuilder builds FunctionBehavior using the builder pattern.
type BehaviorBuilder struct {
	behavior *FunctionBehavior
}

// NewBehaviorBuilder creates a new BehaviorBuilder with default values.
func NewBehaviorBuilder() *BehaviorBuilder {
	return &BehaviorBuilder{
		behavior: &FunctionBehavior{
			Mode:          ModeQuery,
			Idempotent:    false,
			TimeoutMs:     30000,
			RouteStrategy: RouteLB,
			Cacheable:     false,
		},
	}
}

// SetMode sets the execution mode.
func (b *BehaviorBuilder) SetMode(mode Mode) *BehaviorBuilder {
	b.behavior.Mode = mode
	return b
}

// SetQuery sets the mode to QUERY (read-only).
func (b *BehaviorBuilder) SetQuery() *BehaviorBuilder {
	b.behavior.Mode = ModeQuery
	return b
}

// SetCommand sets the mode to COMMAND (write operation).
func (b *BehaviorBuilder) SetCommand() *BehaviorBuilder {
	b.behavior.Mode = ModeCommand
	return b
}

// SetIdempotent marks the function as idempotent.
func (b *BehaviorBuilder) SetIdempotent(idempotent bool) *BehaviorBuilder {
	b.behavior.Idempotent = idempotent
	return b
}

// SetTimeoutMs sets the timeout in milliseconds.
func (b *BehaviorBuilder) SetTimeoutMs(timeoutMs int32) *BehaviorBuilder {
	b.behavior.TimeoutMs = timeoutMs
	return b
}

// SetRouteStrategy sets the routing strategy.
func (b *BehaviorBuilder) SetRouteStrategy(strategy RouteStrategy) *BehaviorBuilder {
	b.behavior.RouteStrategy = strategy
	return b
}

// SetCacheable enables caching with the given TTL.
func (b *BehaviorBuilder) SetCacheable(cacheTtlSeconds int32) *BehaviorBuilder {
	b.behavior.Cacheable = true
	b.behavior.CacheTtlSeconds = cacheTtlSeconds
	return b
}

// Build returns the FunctionBehavior.
func (b *BehaviorBuilder) Build() *FunctionBehavior {
	return b.behavior
}

// SecurityBuilder builds FunctionSecurity using the builder pattern.
type SecurityBuilder struct {
	security *FunctionSecurity
}

// NewSecurityBuilder creates a new SecurityBuilder with default values.
func NewSecurityBuilder() *SecurityBuilder {
	return &SecurityBuilder{
		security: &FunctionSecurity{
			RiskLevel:         RiskMedium,
			RequiresApproval:  false,
			AuditLog:          true,
			MaskSensitiveData: false,
		},
	}
}

// SetRiskLevel sets the risk level.
func (b *SecurityBuilder) SetRiskLevel(level RiskLevel) *SecurityBuilder {
	b.security.RiskLevel = level
	return b
}

// SetLowRisk sets risk level to LOW.
func (b *SecurityBuilder) SetLowRisk() *SecurityBuilder {
	b.security.RiskLevel = RiskLow
	return b
}

// SetMediumRisk sets risk level to MEDIUM.
func (b *SecurityBuilder) SetMediumRisk() *SecurityBuilder {
	b.security.RiskLevel = RiskMedium
	return b
}

// SetHighRisk sets risk level to HIGH.
func (b *SecurityBuilder) SetHighRisk() *SecurityBuilder {
	b.security.RiskLevel = RiskHigh
	return b
}

// SetDangerRisk sets risk level to DANGER.
func (b *SecurityBuilder) SetDangerRisk() *SecurityBuilder {
	b.security.RiskLevel = RiskDanger
	return b
}

// SetPermission sets the required permission.
func (b *SecurityBuilder) SetPermission(permission string) *SecurityBuilder {
	b.security.Permission = permission
	return b
}

// SetRequiresApproval enables or disables approval requirement.
func (b *SecurityBuilder) SetRequiresApproval(requires bool) *SecurityBuilder {
	b.security.RequiresApproval = requires
	return b
}

// SetApprovalType sets the approval type.
func (b *SecurityBuilder) SetApprovalType(approvalType ApprovalType) *SecurityBuilder {
	b.security.ApprovalType = approvalType
	if approvalType != ApprovalNone {
		b.security.RequiresApproval = true
	}
	return b
}

// SetAllowedRoles sets the allowed roles.
func (b *SecurityBuilder) SetAllowedRoles(roles ...string) *SecurityBuilder {
	b.security.AllowedRoles = roles
	return b
}

// SetAuditLog enables or disables audit logging.
func (b *SecurityBuilder) SetAuditLog(enabled bool) *SecurityBuilder {
	b.security.AuditLog = enabled
	return b
}

// SetMaskSensitiveData enables or disables sensitive data masking.
func (b *SecurityBuilder) SetMaskSensitiveData(enabled bool) *SecurityBuilder {
	b.security.MaskSensitiveData = enabled
	return b
}

// Build returns the FunctionSecurity.
func (b *SecurityBuilder) Build() *FunctionSecurity {
	return b.security
}

// ImportOptions controls OpenAPI import behavior.
type ImportOptions struct {
	// CategoryPrefix adds a prefix to all imported categories
	CategoryPrefix string

	// TagPrefix adds a prefix to all imported tags
	TagPrefix string

	// DefaultTimeoutMs is the default timeout for imported functions
	DefaultTimeoutMs int32

	// ContinueOnError continues processing even if some functions fail
	ContinueOnError bool
}
