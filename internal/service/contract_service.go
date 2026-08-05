package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/generator"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const pageProposalGeneratorVersion = "dashboard-vnext-1"

// ContractService manages FunctionContract persistence and semantic rebuilding.
type ContractService struct {
	db               *gorm.DB
	contractModel    *model.FunctionContractModel
	capabilityModel  *model.ResourceCapabilityModel
	semanticsModel   *model.CapabilitySemanticsModel
	versionModel     *model.CapabilitySemanticVersionModel
	proposalModel    *model.PageProposalModel
	proposalVersions *model.PageProposalVersionModel
}

// NewContractService creates the service.
func NewContractService(db *gorm.DB) *ContractService {
	return &ContractService{
		db:               db,
		contractModel:    model.NewFunctionContractModel(db),
		capabilityModel:  model.NewResourceCapabilityModel(db),
		semanticsModel:   model.NewCapabilitySemanticsModel(db),
		versionModel:     model.NewCapabilitySemanticVersionModel(db),
		proposalModel:    model.NewPageProposalModel(db),
		proposalVersions: model.NewPageProposalVersionModel(db),
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
		ID:                input.ID,
		Version:           input.Version,
		Summary:           input.Summary,
		Description:       input.Description,
		InputSchema:       input.InputSchema,
		OutputSchema:      input.OutputSchema,
		Resource:          input.Resource,
		Operation:         input.Operation,
		Capability:        input.Capability,
		Execution:         input.Execution,
		ApprovalRequired:  input.ApprovalRequired,
		ApprovalPolicyKey: input.ApprovalPolicyKey,
		Risk:              input.Risk,
		Permission:        input.Permission,
		Enabled:           input.Enabled,
		Tags:              input.Tags,
	}
	result := normalizer.Normalize(normInput)

	// 2. Compute source digest from the canonical normalized contract.
	digest := computeDigest(result.Function)

	// 3. Build FunctionContract
	contract := &model.FunctionContract{
		GameID:       gameID,
		Env:          env,
		FunctionID:   result.Function.ID,
		Version:      result.Function.Version,
		Enabled:      result.Function.Enabled,
		Deprecated:   input.Deprecated,
		ResourceKey:  result.Function.Resource,
		OperationKey: result.Function.Operation,
		Capability:   string(result.Function.Capability),
		Execution:    string(result.Function.Execution),
		Approval:     approvalPolicyToJSONMap(result.Function.Approval),
		Risk:         string(result.Function.Risk),
		Permission:   result.Function.Permission,
		InputSchema:  datatypes.JSON(result.Function.InputSchema),
		OutputSchema: datatypes.JSON(result.Function.OutputSchema),
		Summary:      toJSONMap(result.Function.Summary),
		Description:  toJSONMap(result.Function.Description),
		Tags:         toJSON(result.Function.Tags),
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
		Labels:      datatypes.JSONMap{},
	}

	if err := s.capabilityModel.UpsertCapability(ctx, cap); err != nil {
		return fmt.Errorf("upsert resource capability: %w", err)
	}

	// 3. Build capability semantics
	semantics := s.buildSemantics(gameID, env, resourceKey, contracts)
	semantics.SourceDigest = computeDigest(contracts)
	if err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
		return fmt.Errorf("upsert capability semantics: %w", err)
	}

	// 4. Create version record
	version := &model.CapabilitySemanticVersion{
		SemanticsID:  semantics.ID,
		Version:      semantics.Version,
		Semantics:    toJSON(semantics),
		SourceDigest: semantics.SourceDigest,
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
		GameID:            gameID,
		Env:               env,
		ResourceKey:       resourceKey,
		Source:            "sdk_explicit",
		PageFieldName:     "page",
		PageSizeFieldName: "page_size",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
	}

	for _, c := range contracts {
		if c == nil {
			continue
		}
		switch c.Capability {
		case "collection_query":
			sem.CollectionQueryID = c.ID
			inferCollectionFields(sem, c)
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
	inferIdentityField(sem, contracts)

	return sem
}

func inferCollectionFields(sem *model.CapabilitySemantics, contract *model.FunctionContract) {
	if sem == nil || contract == nil {
		return
	}
	root := parseJSONSchema(contract.OutputSchema)
	properties := schemaObjectProperty(root, "properties")
	if len(properties) == 0 {
		return
	}
	for _, key := range []string{"items", "list", "rows", "data"} {
		prop := schemaObjectProperty(properties, key)
		if len(prop) == 0 || schemaString(prop["type"]) != "array" {
			continue
		}
		sem.ItemsFieldName = key
		break
	}
	for _, key := range []string{"total", "count", "total_count", "totalCount"} {
		if _, ok := properties[key]; ok {
			sem.TotalFieldName = key
			break
		}
	}
}

func inferIdentityField(sem *model.CapabilitySemantics, contracts []*model.FunctionContract) {
	if sem == nil || strings.TrimSpace(sem.IdentityField) != "" {
		return
	}
	itemSchema := collectionItemSchema(sem, contracts)
	props := schemaObjectProperty(itemSchema, "properties")
	if len(props) == 0 {
		return
	}
	candidates := []string{"id", sem.ResourceKey + "_id", sem.ResourceKey + "Id"}
	for _, key := range candidates {
		if raw, ok := props[key]; ok {
			sem.IdentityField = key
			sem.IdentityFieldType = schemaScalarType(parseJSONSchema(raw))
			sem.IdentityPath = key
			return
		}
	}
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		prop := parseJSONSchema(props[key])
		if typ := schemaScalarType(prop); typ != "" {
			sem.IdentityField = key
			sem.IdentityFieldType = typ
			sem.IdentityPath = key
			return
		}
	}
}

func collectionItemSchema(sem *model.CapabilitySemantics, contracts []*model.FunctionContract) map[string]json.RawMessage {
	contract := findContractByModelID(contracts, sem.CollectionQueryID)
	if contract == nil {
		return nil
	}
	root := parseJSONSchema(contract.OutputSchema)
	if schemaString(root["type"]) == "array" {
		return schemaObjectProperty(root, "items")
	}
	properties := schemaObjectProperty(root, "properties")
	if len(properties) == 0 {
		return nil
	}
	for _, key := range collectionSchemaKeys(sem) {
		prop := schemaObjectProperty(properties, key)
		if len(prop) == 0 || schemaString(prop["type"]) != "array" {
			continue
		}
		items := schemaObjectProperty(prop, "items")
		if len(items) > 0 {
			return items
		}
	}
	return nil
}

func findContractByModelID(contracts []*model.FunctionContract, id uint) *model.FunctionContract {
	for _, contract := range contracts {
		if contract != nil && contract.ID == id {
			return contract
		}
	}
	return nil
}

func collectionSchemaKeys(sem *model.CapabilitySemantics) []string {
	keys := []string{}
	if sem != nil {
		if key := strings.TrimSpace(sem.ItemsFieldName); key != "" {
			keys = append(keys, key)
		}
	}
	return append(keys, "items", "list", "rows", "data")
}

func schemaScalarType(obj map[string]json.RawMessage) string {
	switch schemaString(obj["type"]) {
	case "string", "number", "integer", "boolean":
		return schemaString(obj["type"])
	default:
		return ""
	}
}

func parseJSONSchema(raw []byte) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func schemaObjectProperty(obj map[string]json.RawMessage, key string) map[string]json.RawMessage {
	if len(obj) == 0 {
		return nil
	}
	return parseJSONSchema(obj[key])
}

func schemaString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var out string
	if err := json.Unmarshal(raw, &out); err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// FunctionMetaInput is the input for contract rebuilding.
type FunctionMetaInput struct {
	ID                string
	Version           string
	Enabled           bool
	Deprecated        bool
	Summary           string
	Description       string
	InputSchema       string
	OutputSchema      string
	Resource          string
	Operation         string
	Capability        string
	Execution         string
	ApprovalRequired  bool
	ApprovalPolicyKey string
	Risk              string
	Permission        string
	Tags              []string
}

func computeDigest(v interface{}) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
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

func approvalPolicyToJSONMap(policy spec.ApprovalPolicy) datatypes.JSONMap {
	return datatypes.JSONMap{
		"required":  policy.Required,
		"policyKey": policy.PolicyKey,
	}
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

	slog.Info("rebuilding page proposals",
		"game_id", gameID,
		"env", env,
		"resource_key", resourceKey,
		"old_digest", semantics.SourceDigest,
		"new_digest", newDigest)

	if semantics.SourceDigest != newDigest {
		semantics.SourceDigest = newDigest
		if err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
			return fmt.Errorf("update semantics digest: %w", err)
		}
	}

	consumedActions, err := s.upsertResourceProposal(ctx, gameID, env, semantics, contracts)
	if err != nil {
		return err
	}
	if err := s.upsertStandaloneProposals(ctx, gameID, env, contracts, consumedActions); err != nil {
		return err
	}
	return nil
}

func (s *ContractService) upsertResourceProposal(
	ctx context.Context,
	gameID string,
	env string,
	semantics *model.CapabilitySemantics,
	contracts []*model.FunctionContract,
) (map[string]struct{}, error) {
	generated, ok := generator.GenerateResourcePageProposal(semantics, contracts, generator.DefaultGenerateOptions())
	if !ok {
		return map[string]struct{}{}, nil
	}
	consumed := consumedStandaloneBindings(generated.Bindings, contracts)
	return consumed, s.upsertGeneratedProposal(ctx, gameID, env, resourceProposalKey(semantics.ResourceKey), semantics, contracts, generated)
}

func (s *ContractService) upsertStandaloneProposals(
	ctx context.Context,
	gameID string,
	env string,
	contracts []*model.FunctionContract,
	consumed map[string]struct{},
) error {
	for _, contract := range contracts {
		if contract == nil || isCRUDCapability(contract.Capability) {
			continue
		}
		if _, ok := consumed[strings.TrimSpace(contract.FunctionID)]; ok {
			continue
		}
		generated := generator.GenerateForOperation(operationSpecFromContract(contract), generator.GenerateOptions{
			DefaultLocale: "zh-CN",
			Functions: map[string]spec.FunctionSpec{
				contract.FunctionID: functionSpecFromContract(contract),
			},
		})
		proposalKey := standaloneProposalKey(generated.Type, contract.FunctionID)
		if err := s.upsertGeneratedProposal(ctx, gameID, env, proposalKey, nil, []*model.FunctionContract{contract}, generated); err != nil {
			return err
		}
	}
	return nil
}

func consumedStandaloneBindings(bindings []spec.PageFunctionBinding, contracts []*model.FunctionContract) map[string]struct{} {
	consumed := map[string]struct{}{}
	for _, binding := range bindings {
		contract := findContractByFunctionID(contracts, binding.FunctionID)
		if contract == nil || isCRUDCapability(contract.Capability) {
			continue
		}
		consumed[strings.TrimSpace(contract.FunctionID)] = struct{}{}
	}
	return consumed
}

func findContractByFunctionID(contracts []*model.FunctionContract, functionID string) *model.FunctionContract {
	functionID = strings.TrimSpace(functionID)
	for _, contract := range contracts {
		if contract != nil && strings.TrimSpace(contract.FunctionID) == functionID {
			return contract
		}
	}
	return nil
}

func (s *ContractService) upsertGeneratedProposal(
	ctx context.Context,
	gameID string,
	env string,
	proposalKey string,
	semantics *model.CapabilitySemantics,
	contracts []*model.FunctionContract,
	generated spec.GeneratedPageSpec,
) error {
	if strings.TrimSpace(proposalKey) == "" || strings.TrimSpace(generated.PageKey) == "" {
		return nil
	}
	pageJSON, err := json.Marshal(generated.PageSpec)
	if err != nil {
		return fmt.Errorf("marshal generated page spec: %w", err)
	}
	proposal := &model.PageProposal{
		GameID:           gameID,
		Env:              env,
		ProposalKey:      proposalKey,
		PageKey:          generated.PageKey,
		PageType:         string(generated.Type),
		ResourceKey:      generated.ResourceKey,
		Quality:          string(generated.Quality),
		GeneratorVersion: pageProposalGeneratorVersion,
		FunctionDigest:   computeDigest(contracts),
		SemanticsDigest:  computeDigest(semantics),
		Title:            toJSONMap(generated.Title),
		Description:      toJSONMap(generated.Description),
		CategoryKey:      generated.Category.Key,
		PageSpec:         datatypes.JSON(pageJSON),
		Diagnostics:      toJSON(generated.Diagnostics),
		Status:           "pending",
		UpdatedBy:        "system",
	}
	changed := true
	if existing, err := s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey); err == nil {
		changed = generatedProposalChanged(existing, proposal)
		if !changed {
			proposal.Status = preserveGeneratedProposalStatus(existing.Status)
			if existing.UpdatedBy != "" {
				proposal.UpdatedBy = existing.UpdatedBy
			}
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find existing page proposal %s: %w", proposalKey, err)
	}
	if err := s.proposalModel.UpsertProposal(ctx, proposal); err != nil {
		return fmt.Errorf("upsert page proposal %s: %w", proposalKey, err)
	}
	if changed {
		if _, err := createProposalVersionSnapshot(ctx, s.proposalVersions, proposal, "generate proposal from latest contracts", "system"); err != nil {
			return fmt.Errorf("snapshot page proposal %s: %w", proposalKey, err)
		}
	}
	return nil
}

func generatedProposalChanged(existing *model.PageProposal, next *model.PageProposal) bool {
	if existing == nil || next == nil {
		return true
	}
	return proposalComparableDigest(existing) != proposalComparableDigest(next)
}

func proposalComparableDigest(proposal *model.PageProposal) string {
	if proposal == nil {
		return ""
	}
	return computeDigest(struct {
		ProposalKey      string
		PageKey          string
		PageType         string
		ResourceKey      string
		Quality          string
		GeneratorVersion string
		FunctionDigest   string
		SemanticsDigest  string
		Title            datatypes.JSONMap
		Description      datatypes.JSONMap
		CategoryKey      string
		PageSpec         datatypes.JSON
		Diagnostics      datatypes.JSON
	}{
		ProposalKey:      proposal.ProposalKey,
		PageKey:          proposal.PageKey,
		PageType:         proposal.PageType,
		ResourceKey:      proposal.ResourceKey,
		Quality:          proposal.Quality,
		GeneratorVersion: proposal.GeneratorVersion,
		FunctionDigest:   proposal.FunctionDigest,
		SemanticsDigest:  proposal.SemanticsDigest,
		Title:            proposal.Title,
		Description:      proposal.Description,
		CategoryKey:      proposal.CategoryKey,
		PageSpec:         proposal.PageSpec,
		Diagnostics:      proposal.Diagnostics,
	})
}

func preserveGeneratedProposalStatus(status string) string {
	switch status {
	case "accepted", "rejected":
		return status
	default:
		return "pending"
	}
}

func operationSpecFromContract(contract *model.FunctionContract) spec.OperationSpec {
	if contract == nil {
		return spec.OperationSpec{}
	}
	return spec.OperationSpec{
		FunctionID:  contract.FunctionID,
		ResourceKey: contract.ResourceKey,
		Operation:   contract.OperationKey,
		Capability:  spec.CapabilityKind(contract.Capability),
		Execution:   spec.FunctionExecution(contract.Execution),
		Approval:    jsonMapToApprovalPolicy(contract.Approval),
		Risk:        spec.RiskLevel(contract.Risk),
		Permission:  contract.Permission,
		Enabled:     contract.Enabled,
	}
}

func functionSpecFromContract(contract *model.FunctionContract) spec.FunctionSpec {
	if contract == nil {
		return spec.FunctionSpec{}
	}
	return spec.FunctionSpec{
		ID:           contract.FunctionID,
		Version:      contract.Version,
		Enabled:      contract.Enabled,
		Deprecated:   contract.Deprecated,
		InputSchema:  spec.JSONSchema(contract.InputSchema),
		OutputSchema: spec.JSONSchema(contract.OutputSchema),
		Summary:      jsonMapToLocalizedText(contract.Summary),
		Description:  jsonMapToLocalizedText(contract.Description),
		Resource:     contract.ResourceKey,
		Operation:    contract.OperationKey,
		Capability:   spec.CapabilityKind(contract.Capability),
		Execution:    spec.FunctionExecution(contract.Execution),
		Approval:     jsonMapToApprovalPolicy(contract.Approval),
		Risk:         spec.RiskLevel(contract.Risk),
		Permission:   contract.Permission,
	}
}

func jsonMapToApprovalPolicy(values map[string]interface{}) spec.ApprovalPolicy {
	if len(values) == 0 {
		return spec.ApprovalPolicy{}
	}
	required, _ := values["required"].(bool)
	policyKey, _ := values["policyKey"].(string)
	if policyKey == "" {
		policyKey, _ = values["policy_key"].(string)
	}
	return spec.ApprovalPolicy{
		Required:  required,
		PolicyKey: strings.TrimSpace(policyKey),
	}
}

func jsonMapToLocalizedText(values map[string]interface{}) spec.LocalizedText {
	if len(values) == 0 {
		return nil
	}
	out := make(spec.LocalizedText, len(values))
	for key, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			out[key] = text
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func resourceProposalKey(resourceKey string) string {
	key := sanitizeSourceKey(resourceKey)
	if key == "" {
		return ""
	}
	return "resource:" + key
}

func standaloneProposalKey(pageType spec.PageType, functionID string) string {
	kind := string(pageType)
	switch pageType {
	case spec.PageTypeTask, spec.PageTypeReport:
	default:
		kind = string(spec.PageTypeOperation)
	}
	key := sanitizeSourceKey(functionID)
	if key == "" {
		return ""
	}
	return kind + ":" + key
}

func isCRUDCapability(capability string) bool {
	switch spec.CapabilityKind(capability) {
	case spec.CapabilityCollectionQuery, spec.CapabilityItemQuery, spec.CapabilityCreate, spec.CapabilityUpdate, spec.CapabilityDelete:
		return true
	default:
		return false
	}
}

func sanitizeSourceKey(value string) string {
	return strings.Trim(strings.TrimSpace(value), ".")
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
