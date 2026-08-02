package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ContractService manages FunctionContract persistence and semantic rebuilding.
type ContractService struct {
	db              *gorm.DB
	contractModel   *model.FunctionContractModel
	capabilityModel *model.ResourceCapabilityModel
	semanticsModel  *model.CapabilitySemanticsModel
	versionModel    *model.CapabilitySemanticVersionModel
}

// NewContractService creates the service.
func NewContractService(db *gorm.DB) *ContractService {
	return &ContractService{
		db:              db,
		contractModel:   model.NewFunctionContractModel(db),
		capabilityModel: model.NewResourceCapabilityModel(db),
		semanticsModel:  model.NewCapabilitySemanticsModel(db),
		versionModel:    model.NewCapabilitySemanticVersionModel(db),
	}
}

// RebuildContractFromFunctionMeta rebuilds a FunctionContract from raw function metadata.
// This is called when a function is registered or updated.
// meta can be FunctionMetaInput or any struct with matching JSON tags.
func (s *ContractService) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, meta interface{}) error {
	// Convert meta to FunctionMetaInput via JSON round-trip to support
	// anonymous structs from the registry store.
	var input FunctionMetaInput
	if m, ok := meta.(FunctionMetaInput); ok {
		input = m
	} else {
		b, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("marshal meta: %w", err)
		}
		if err := json.Unmarshal(b, &input); err != nil {
			return fmt.Errorf("unmarshal meta: %w", err)
		}
	}

	// 1. Normalize the descriptor
	normInput := normalizer.DescriptorInput{
		ID:           input.ID,
		Version:      input.Version,
		Summary:      input.Summary,
		Description:  input.Description,
		InputSchema:  input.InputSchema,
		OutputSchema: input.OutputSchema,
		Resource:     input.Resource,
		Operation:    input.Operation,
		Capability:   input.Capability,
		Execution:    input.Execution,
		Risk:         input.Risk,
		Permission:   input.Permission,
		Enabled:      input.Enabled,
		Tags:         input.Tags,
	}
	result := normalizer.Normalize(normInput)

	// 2. Compute source digest
	digest := computeDigest(input)

	// 3. Build FunctionContract
	contract := &model.FunctionContract{
		GameID:       gameID,
		Env:          env,
		FunctionID:   input.ID,
		Version:      input.Version,
		Enabled:      input.Enabled,
		Deprecated:   input.Deprecated,
		ResourceKey:  input.Resource,
		OperationKey: input.Operation,
		Capability:   input.Capability,
		Execution:    input.Execution,
		Risk:         input.Risk,
		Permission:   input.Permission,
		InputSchema:  datatypes.JSON(input.InputSchema),
		OutputSchema: datatypes.JSON(input.OutputSchema),
		Summary:      toJSONMap(result.Function.Summary),
		Description:  toJSONMap(result.Function.Description),
		Tags:         toJSON(input.Tags),
		Source:       source,
		SourceDigest: digest,
		Diagnostics:  toJSON(result.Diagnostics),
	}

	// 4. Upsert contract
	if err := s.contractModel.UpsertContract(ctx, contract); err != nil {
		return fmt.Errorf("upsert function contract: %w", err)
	}

	slog.Info("rebuilt function contract",
		"game_id", gameID,
		"env", env,
		"function_id", input.ID,
		"resource", input.Resource,
		"capability", input.Capability)

	return nil
}

// RebuildResourceCapability rebuilds a ResourceCapability from existing contracts.
func (s *ContractService) RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	// 1. Get all contracts for this resource
	contracts, err := s.contractModel.ListByResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		return fmt.Errorf("list contracts: %w", err)
	}
	if len(contracts) == 0 {
		return nil
	}

	// 2. Build capability aggregation
	cap := &model.ResourceCapability{
		GameID:      gameID,
		Env:         env,
		ResourceKey: resourceKey,
		Labels:      toJSONMap(spec.LocalizedText{"zh-CN": resourceKey}),
	}

	if err := s.capabilityModel.UpsertCapability(ctx, cap); err != nil {
		return fmt.Errorf("upsert resource capability: %w", err)
	}

	// 3. Build capability semantics
	semantics := s.buildSemantics(gameID, env, resourceKey, contracts)
	if err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
		return fmt.Errorf("upsert capability semantics: %w", err)
	}

	// 4. Create version record
	version := &model.CapabilitySemanticVersion{
		SemanticsID:  semantics.ID,
		Version:      semantics.Version,
		Semantics:    toJSON(semantics),
		SourceDigest: computeDigest(contracts),
		ChangeReason: "rebuild from function registration",
	}
	if err := s.versionModel.CreateVersion(ctx, version); err != nil {
		return fmt.Errorf("create semantic version: %w", err)
	}

	return nil
}

// buildSemantics constructs CapabilitySemantics from a list of contracts.
func (s *ContractService) buildSemantics(gameID, env, resourceKey string, contracts []*model.FunctionContract) *model.CapabilitySemantics {
	sem := &model.CapabilitySemantics{
		GameID:      gameID,
		Env:         env,
		ResourceKey: resourceKey,
		Source:      "sdk_explicit",
	}

	for _, c := range contracts {
		switch c.Capability {
		case "collection_query":
			sem.CollectionQueryID = c.ID
		case "item_query":
			sem.ItemQueryID = c.ID
		case "create":
			sem.CreateID = c.ID
		case "update":
			sem.UpdateID = c.ID
		case "delete":
			sem.DeleteID = c.ID
		}
	}

	return sem
}

// FunctionMetaInput is the input for contract rebuilding.
type FunctionMetaInput struct {
	ID           string
	Version      string
	Enabled      bool
	Deprecated   bool
	Summary      string
	Description  string
	InputSchema  string
	OutputSchema string
	Resource     string
	Operation    string
	Capability   string
	Execution    string
	Risk         string
	Permission   string
	Tags         []string
}

func computeDigest(v interface{}) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}

func toJSON(v interface{}) datatypes.JSON {
	b, _ := json.Marshal(v)
	return datatypes.JSON(b)
}

func toJSONMap(m spec.LocalizedText) datatypes.JSONMap {
	if m == nil {
		return datatypes.JSONMap{}
	}
	b, _ := json.Marshal(m)
	var result datatypes.JSONMap
	json.Unmarshal(b, &result)
	return result
}

// ListContracts lists all contracts in a scope.
func (s *ContractService) ListContracts(ctx context.Context, gameID, env string) ([]*model.FunctionContract, error) {
	return s.contractModel.ListByScope(ctx, gameID, env)
}

// GetContract gets a contract by function ID.
func (s *ContractService) GetContract(ctx context.Context, gameID, env, functionID string) (*model.FunctionContract, error) {
	return s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
}

// ListResourceCapabilities lists all resource capabilities in a scope.
func (s *ContractService) ListResourceCapabilities(ctx context.Context, gameID, env string) ([]*model.ResourceCapability, error) {
	return s.capabilityModel.ListByScope(ctx, gameID, env)
}

// RebuildProposalsForResource triggers incremental proposal recalculation
// for all proposals affected by changes to a resource.
// This should be called after:
// - Function registration/update
// - OpenAPI Source update
// - Catalog semantic update
func (s *ContractService) RebuildProposalsForResource(ctx context.Context, gameID, env, resourceKey string) error {
	// Get current semantics
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		// No semantics yet, skip
		return nil
	}

	// Get contracts for this resource
	contracts, err := s.contractModel.ListByResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		return fmt.Errorf("list contracts: %w", err)
	}

	// Compute new source digest
	newDigest := computeDigest(contracts)

	// Check if semantics have changed
	if semantics.SourceDigest == newDigest {
		// No change, skip rebuild
		return nil
	}

	slog.Info("triggering proposal rebuild",
		"game_id", gameID,
		"env", env,
		"resource_key", resourceKey,
		"old_digest", semantics.SourceDigest,
		"new_digest", newDigest)

	// Update semantics source digest
	semantics.SourceDigest = newDigest
	if err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
		return fmt.Errorf("update semantics digest: %w", err)
	}

	return nil
}

// RebuildAllProposals triggers proposal recalculation for all resources in a scope.
func (s *ContractService) RebuildAllProposals(ctx context.Context, gameID, env string) error {
	// Get all resource capabilities
	capabilities, err := s.capabilityModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return fmt.Errorf("list capabilities: %w", err)
	}

	for _, cap := range capabilities {
		if err := s.RebuildProposalsForResource(ctx, gameID, env, cap.ResourceKey); err != nil {
			slog.Error("failed to rebuild proposals for resource",
				"game_id", gameID,
				"env", env,
				"resource_key", cap.ResourceKey,
				"error", err)
			// Continue with other resources
		}
	}

	return nil
}
