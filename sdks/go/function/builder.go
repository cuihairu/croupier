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
			Tags:     []string{},
			Behavior: &FunctionBehavior{},
			Risk:     &FunctionRisk{Level: RiskMedium},
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

// SetResource sets the business resource/capability key.
func (b *MetadataBuilder) SetResource(resource string) *MetadataBuilder {
	b.metadata.Resource = resource
	return b
}

// SetOperation sets the business action key (e.g., "ban", "send", "list").
func (b *MetadataBuilder) SetOperation(operation string) *MetadataBuilder {
	b.metadata.Operation = operation
	return b
}

// SetPermission sets the optional permission identifier.
func (b *MetadataBuilder) SetPermission(permission string) *MetadataBuilder {
	b.metadata.Permission = permission
	return b
}

// SetApproval marks the function as requiring two-person approval.
// policyKey selects the approval policy; empty keeps the server default.
func (b *MetadataBuilder) SetApproval(policyKey string) *MetadataBuilder {
	b.metadata.ApprovalRequired = true
	b.metadata.ApprovalPolicyKey = policyKey
	return b
}

// SetCapability sets the capability kind.
// Valid values: collection_query, item_query, create, update, delete, action, task, report
func (b *MetadataBuilder) SetCapability(capability string) *MetadataBuilder {
	b.metadata.Capability = capability
	return b
}

// SetExecution sets the execution mode.
// Valid values: sync, task
func (b *MetadataBuilder) SetExecution(execution string) *MetadataBuilder {
	b.metadata.Execution = execution
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

// SetSummary sets the one-line summary.
func (b *MetadataBuilder) SetSummary(summary string) *MetadataBuilder {
	b.metadata.Summary = summary
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

// SetRisk sets the function risk.
func (b *MetadataBuilder) SetRisk(risk *FunctionRisk) *MetadataBuilder {
	b.metadata.Risk = risk
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
	if b.metadata.Risk == nil {
		b.metadata.Risk = &FunctionRisk{Level: RiskMedium}
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

// RiskBuilder builds FunctionRisk using the builder pattern.
type RiskBuilder struct {
	risk *FunctionRisk
}

// NewRiskBuilder creates a new RiskBuilder with default values.
func NewRiskBuilder() *RiskBuilder {
	return &RiskBuilder{
		risk: &FunctionRisk{
			Level: RiskMedium,
		},
	}
}

// SetLevel sets the risk level.
func (b *RiskBuilder) SetLevel(level RiskLevel) *RiskBuilder {
	b.risk.Level = level
	return b
}

// SetLow sets risk level to LOW.
func (b *RiskBuilder) SetLow() *RiskBuilder {
	b.risk.Level = RiskLow
	return b
}

// SetMedium sets risk level to MEDIUM.
func (b *RiskBuilder) SetMedium() *RiskBuilder {
	b.risk.Level = RiskMedium
	return b
}

// SetHigh sets risk level to HIGH.
func (b *RiskBuilder) SetHigh() *RiskBuilder {
	b.risk.Level = RiskHigh
	return b
}

// SetDanger sets risk level to DANGER.
func (b *RiskBuilder) SetDanger() *RiskBuilder {
	b.risk.Level = RiskDanger
	return b
}

// Build returns the FunctionRisk.
func (b *RiskBuilder) Build() *FunctionRisk {
	return b.risk
}

// ImportOptions controls OpenAPI import behavior.
type ImportOptions struct {
	// ResourcePrefix adds a prefix to all imported resources.
	ResourcePrefix string

	// TagPrefix adds a prefix to all imported tags
	TagPrefix string

	// DefaultTimeoutMs is the default timeout for imported functions
	DefaultTimeoutMs int32

	// ContinueOnError continues processing even if some functions fail
	ContinueOnError bool
}
