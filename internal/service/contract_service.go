package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/generator"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/cuihairu/croupier/internal/function/schemadiff"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// PageProposalGeneratorVersion identifies the page proposal generator
// revision. It participates in proposalComparableDigest, so bumping it makes
// every stored proposal compare as changed on the next rebuild pass and get
// regenerated from the current generator (e.g. after label-fallback changes).
// page-generator:3：humanize 兜底上线（裸 key → 可读标题），存量提案
// 在下一轮 RebuildAllProposals 时按版本差异自动重生成。
const PageProposalGeneratorVersion = "page-generator:3"

// normalizeSchemaToJSON ensures schema is stored as a native JSON object,
// not as a JSON string value. This prevents the API from returning schemas
// as strings instead of objects.
func normalizeSchemaToJSON(raw json.RawMessage) model.JSON {
	if len(raw) == 0 {
		return nil
	}
	// If it starts with a quote, it's a JSON string — unwrap it
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return model.JSON(s)
		}
	}
	return model.JSON(raw)
}

// ContractService manages FunctionContract persistence and semantic rebuilding.
type ContractService struct {
	db               *gorm.DB
	contractModel    *model.FunctionContractModel
	capabilityModel  *model.ResourceCapabilityModel
	semanticsModel   *model.CapabilitySemanticsModel
	versionModel     *model.CapabilitySemanticVersionModel
	proposalModel    *model.PageProposalModel
	proposalVersions *model.PageProposalVersionModel
	blockedIssues    *model.BlockedProposalIssueModel
	auditSvc         *audit.AuditService
	alertModel       *model.AlertModel
}

// WithAlertModel 注入告警模型（可选）：schema 破坏性变更写告警中心。
func (s *ContractService) WithAlertModel(m *model.AlertModel) *ContractService {
	s.alertModel = m
	return s
}

// WithAuditService 注入审计服务（可选）：契约更新时写 function.contract_updated
// 审计事件（含 schema 兼容性 diff 摘要）。
func (s *ContractService) WithAuditService(auditSvc *audit.AuditService) *ContractService {
	s.auditSvc = auditSvc
	return s
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
		blockedIssues:    model.NewBlockedProposalIssueModel(db),
		alertModel:       model.NewAlertModel(db),
	}
}

// RebuildContractFromFunctionMeta rebuilds a FunctionContract from the
// explicit registration contract. This is called when a function is
// registered or updated.
func (s *ContractService) RebuildContractFromFunctionMeta(ctx context.Context, gameID, env, source string, input spec.FunctionContractInput) error {
	if violation, ok := registrationguard.FindPresentationViolation(nil, input.InputSchema, input.OutputSchema); ok {
		return fmt.Errorf("function contract contains forbidden presentation field %q at %s", violation.Field, violation.Location)
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
		TimeoutMs:         input.TimeoutMs,
	}
	result := normalizer.Normalize(normInput)
	if validationErr := contractValidationError(result.Diagnostics); validationErr != nil {
		return validationErr
	}

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
		Capability:   mustParseCapability(string(result.Function.Capability)),
		Execution:    string(result.Function.Execution),
		TimeoutMs:    timeoutMsToInt32(result.Function.TimeoutMs),
		Approval:     approvalPolicyToJSONMap(result.Function.Approval),
		Risk:         mustParseRisk(string(result.Function.Risk)),
		Permission:   result.Function.Permission,
		InputSchema:  normalizeSchemaToJSON(json.RawMessage(result.Function.InputSchema)),
		OutputSchema: normalizeSchemaToJSON(json.RawMessage(result.Function.OutputSchema)),
		Summary:      toJSONMap(result.Function.Summary),
		Description:  toJSONMap(result.Function.Description),
		Tags:         toJSON(result.Function.Tags),
		Source:       source,
		SourceDigest: digest,
		Diagnostics:  toJSON(result.Diagnostics),
	}

	// 4. Schema 兼容性 diff（F12）：与库中现有契约对比，破坏性变更
	// 写入 Diagnostics 告警（不阻断注册，由告警/可视化层消费）。
	diagnostics, diffFindings, isUpdate := mergeSchemaDiffDiagnostics(ctx, s.contractModel, gameID, env, input.ID, contract, toJSON(result.Diagnostics))

	// 5. Upsert contract
	contract.Diagnostics = diagnostics
	if err := s.contractModel.UpsertContract(ctx, contract); err != nil {
		return fmt.Errorf("upsert function contract: %w", err)
	}

	// F13：契约更新审计（function.contract_updated，含 diff 摘要）。
	s.logContractUpdateAudit(ctx, gameID, env, source, input.ID, isUpdate, diffFindings)

	// F：schema 破坏性变更写告警中心（主动提醒运营；按内容去重防骚扰）。
	s.warnBreakingSchemaChanges(ctx, gameID, env, source, input.ID, diffFindings)

	slog.Info("rebuilt function contract",
		"game_id", gameID,
		"env", env,
		"function_id", input.ID,
		"resource", input.Resource,
		"capability", input.Capability)

	return nil
}

// logContractUpdateAudit 在契约更新（非首次注册）时写审计事件；audit 服务
// 未注入或写入失败不影响注册主流程。
func (s *ContractService) logContractUpdateAudit(
	ctx context.Context,
	gameID, env, source, functionID string,
	isUpdate bool,
	findings []schemadiff.Finding,
) {
	if !isUpdate || s.auditSvc == nil {
		return
	}
	breaking := make([]string, 0, len(findings))
	for _, finding := range findings {
		if finding.Severity == schemadiff.SeverityBreaking {
			breaking = append(breaking, fmt.Sprintf("%s%s: %s", finding.Source, finding.Path, finding.Reason))
		}
	}
	details := map[string]interface{}{
		"gameId":       gameID,
		"env":          env,
		"functionId":   functionID,
		"source":       source,
		"diffFindings": len(findings),
		"breaking":     breaking,
	}
	if _, err := s.auditSvc.Log(ctx, audit.EventFunctionContractUpdated,
		audit.WithResourceID("function_contract", functionID),
		audit.WithActorID("system", "system", "Contract Rebuild"),
		audit.WithDetails(details),
	); err != nil {
		slog.Warn("failed to write function.contract_updated audit",
			"game_id", gameID, "env", env, "function_id", functionID, "error", err)
	}
}

// warnBreakingSchemaChanges 将 schema 破坏性变更写入告警中心。
// AlertID 含变更内容指纹：相同变更不重复告警；修复后再次出现会重新告警。
func (s *ContractService) warnBreakingSchemaChanges(
	ctx context.Context, gameID, env, source, functionID string, findings []schemadiff.Finding,
) {
	var breaking []schemadiff.Finding
	for _, finding := range findings {
		if finding.Severity == schemadiff.SeverityBreaking {
			breaking = append(breaking, finding)
		}
	}
	if len(breaking) == 0 || s.alertModel == nil {
		return
	}
	fingerprint := fmt.Sprintf("%s-%s-%s", gameID, env, functionID)
	details := map[string]interface{}{
		"gameId":     gameID,
		"env":        env,
		"functionId": functionID,
		"source":     source,
		"findings":   breaking,
	}
	raw, _ := json.Marshal(breaking)
	fingerprint += "-" + fmt.Sprintf("%x", sha256.Sum256(raw))[:12]
	alertID := "schema-diff-" + fmt.Sprintf("%x", sha256.Sum256([]byte(fingerprint)))[:16]

	if existing, err := s.alertModel.FindByAlertID(ctx, alertID); err == nil && existing != nil {
		return // 相同变更已告警过
	}
	message := fmt.Sprintf("函数 %s 的 schema 存在 %d 处破坏性变更（来源 %s）", functionID, len(breaking), source)
	if err := s.alertModel.Create(ctx, &model.Alert{
		AlertID:   alertID,
		Type:      "schema_diff",
		Level:     "warning",
		Message:   message,
		Source:    "contract",
		Status:    "firing",
		Details:   details,
		CreatedBy: "system",
	}); err != nil {
		slog.Warn("failed to create schema diff alert",
			"game_id", gameID, "env", env, "function_id", functionID, "error", err)
	}
}

// RebuildResourceCapability rebuilds a ResourceCapability from existing contracts.
func (s *ContractService) RebuildResourceCapability(ctx context.Context, gameID, env, resourceKey string) error {
	// 1. Get all contracts for this resource
	contracts, err := s.contractModel.ListByResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		return fmt.Errorf("list contracts: %w", err)
	}
	if len(contracts) == 0 {
		return s.removeResourceDerivedState(ctx, gameID, env, resourceKey)
	}

	slog.Info("RebuildResourceCapability: found contracts",
		"game_id", gameID,
		"env", env,
		"resource_key", resourceKey,
		"contract_count", len(contracts))

	// 2. Build capability aggregation. Registration is an automated source, so
	// reviewed presentation fields (labels, description, category, tags) are
	// preserved from the existing capability instead of being wiped on every
	// re-registration.
	cap := &model.ResourceCapability{
		GameID:      gameID,
		Env:         env,
		ResourceKey: resourceKey,
		Labels:      datatypes.JSONMap{},
	}
	if existing, err := s.capabilityModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey); err == nil {
		preserveReviewedCapability(cap, existing)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find existing resource capability: %w", err)
	}

	if err := s.capabilityModel.UpsertCapability(ctx, cap); err != nil {
		return fmt.Errorf("upsert resource capability: %w", err)
	}

	slog.Info("RebuildResourceCapability: upserted capability",
		"game_id", gameID,
		"env", env,
		"resource_key", resourceKey)

	// 3. Build capability semantics. Existing platform_review fields are
	// preserved because registration only provides capability hints, not the
	// final reviewed semantics.
	semantics := s.buildSemantics(gameID, env, resourceKey, contracts)
	if existing, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey); err == nil {
		liveIDs := make(map[uint]struct{}, len(contracts))
		for _, c := range contracts {
			if c != nil {
				liveIDs[c.ID] = struct{}{}
			}
		}
		preserveReviewedSemantics(semantics, existing, liveIDs)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find existing capability semantics: %w", err)
	}
	semantics.SourceDigest = computeDigest(contracts)
	if err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
		return fmt.Errorf("upsert capability semantics: %w", err)
	}

	// 4. Create version record. This rebuild is triggered by automated
	// registration sync, not a human operator, so the actor is "system".
	version := &model.CapabilitySemanticVersion{
		SemanticsID:  semantics.ID,
		Version:      semantics.Version,
		Semantics:    toJSON(semantics),
		SourceDigest: semantics.SourceDigest,
		ChangeReason: "rebuild from function registration",
		CreatedBy:    "system",
	}
	if err := s.versionModel.CreateVersion(ctx, version); err != nil {
		return fmt.Errorf("create semantic version: %w", err)
	}

	return nil
}

func (s *ContractService) removeResourceDerivedState(ctx context.Context, gameID, env, resourceKey string) error {
	resourceKey = strings.TrimSpace(resourceKey)
	if resourceKey == "" {
		return nil
	}
	if err := s.capabilityModel.DeleteByScopeAndResourceKey(ctx, gameID, env, resourceKey); err != nil {
		return fmt.Errorf("delete resource capability %s: %w", resourceKey, err)
	}
	if err := s.semanticsModel.DeleteByScopeAndResourceKey(ctx, gameID, env, resourceKey); err != nil {
		return fmt.Errorf("delete capability semantics %s: %w", resourceKey, err)
	}
	return nil
}

// buildSemantics constructs CapabilitySemantics from a list of contracts.
func (s *ContractService) buildSemantics(gameID, env, resourceKey string, contracts []*model.FunctionContract) *model.CapabilitySemantics {
	sem := &model.CapabilitySemantics{
		GameID:            gameID,
		Env:               env,
		ResourceKey:       resourceKey,
		Source:            semanticSourceForContracts(contracts),
		PageFieldName:     "page",
		PageSizeFieldName: "page_size",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
	}
	tracker := normalizer.NewSemanticProvenanceTracker()

	for _, c := range contracts {
		if c == nil {
			continue
		}
		switch c.Capability {
		case dbenum.CapabilityCollectionQuery:
			if trackSemanticBinding(tracker, "collectionQueryID", c) {
				sem.CollectionQueryID = c.ID
				inferCollectionFields(sem, c)
			}
		case dbenum.CapabilityItemQuery:
			if trackSemanticBinding(tracker, "itemQueryID", c) {
				sem.ItemQueryID = c.ID
			}
		case dbenum.CapabilityCreate:
			if trackSemanticBinding(tracker, "createID", c) {
				sem.CreateID = c.ID
			}
		case dbenum.CapabilityUpdate:
			if trackSemanticBinding(tracker, "updateID", c) {
				sem.UpdateID = c.ID
			}
		case dbenum.CapabilityDelete:
			if trackSemanticBinding(tracker, "deleteID", c) {
				sem.DeleteID = c.ID
			}
		}
	}
	inferIdentityField(sem, contracts)
	inferActionSemantics(sem, contracts)
	if provenance, err := json.Marshal(tracker.GetProvenance()); err == nil {
		sem.Provenance = provenance
	}
	if conflicts, err := json.Marshal(tracker.GetConflicts()); err == nil {
		sem.Conflicts = conflicts
	}

	return sem
}

// inferActionSemantics derives default action subjects from action contract
// input schemas so registered row/batch/toolbar actions can be inlined into
// the generated ResourcePage without manual Resource Catalog edits. Actions
// whose identity input cannot be statically verified stay standalone; the
// generator re-validates before inlining.
func inferActionSemantics(sem *model.CapabilitySemantics, contracts []*model.FunctionContract) {
	if sem == nil {
		return
	}
	type inferredAction struct {
		FunctionID    string `json:"functionId"`
		Subject       string `json:"subject"`
		IdentityInput string `json:"identityInput,omitempty"`
	}
	actions := make([]inferredAction, 0)
	for _, contract := range contracts {
		if contract == nil || contract.Capability != dbenum.CapabilityAction {
			continue
		}
		functionID := strings.TrimSpace(contract.FunctionID)
		if functionID == "" {
			continue
		}
		root := parseJSONSchema(contract.InputSchema)
		props := schemaObjectProperty(root, "properties")
		required := requiredSchemaFields(root)
		if len(props) == 0 {
			actions = append(actions, inferredAction{FunctionID: functionID, Subject: "none"})
			continue
		}
		if len(required) == 1 {
			field := required[0]
			prop := parseJSONSchema(props[field])
			if schemaScalarType(prop) != "" && field == sem.IdentityField {
				actions = append(actions, inferredAction{
					FunctionID:    functionID,
					Subject:       "resource_item",
					IdentityInput: "/" + escapeJSONPointerToken(field),
				})
				continue
			}
			if schemaString(prop["type"]) == "array" {
				actions = append(actions, inferredAction{
					FunctionID:    functionID,
					Subject:       "resource_selection",
					IdentityInput: "/" + escapeJSONPointerToken(field),
				})
				continue
			}
		}
		// Identity input cannot be statically verified: keep the action as a
		// standalone operation by not registering an action semantic.
	}
	if len(actions) == 0 {
		return
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].FunctionID < actions[j].FunctionID })
	if raw, err := json.Marshal(actions); err == nil {
		sem.Actions = raw
	}
}

func requiredSchemaFields(root map[string]json.RawMessage) []string {
	if root == nil || len(root["required"]) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(root["required"], &out); err != nil {
		return nil
	}
	filtered := make([]string, 0, len(out))
	for _, name := range out {
		if strings.TrimSpace(name) != "" {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

func trackSemanticBinding(tracker *normalizer.SemanticProvenanceTracker, field string, contract *model.FunctionContract) bool {
	if tracker == nil || contract == nil {
		return false
	}
	return tracker.TrackUint(field, contract.ID, semanticSourceForContract(contract), contract.SourceDigest, "system")
}

func semanticSourceForContract(contract *model.FunctionContract) spec.SemanticSource {
	if contract != nil && strings.TrimSpace(contract.Source) == "openapi" {
		return spec.SemanticSourceOpenAPIRest
	}
	return spec.SemanticSourceSDKExplicit
}

func semanticSourceForContracts(contracts []*model.FunctionContract) string {
	hasSDK := false
	hasOpenAPI := false
	for _, contract := range contracts {
		if contract == nil {
			continue
		}
		switch strings.TrimSpace(contract.Source) {
		case "sdk":
			hasSDK = true
		case "openapi":
			hasOpenAPI = true
		}
	}
	switch {
	case hasSDK:
		return string(spec.SemanticSourceSDKExplicit)
	case hasOpenAPI:
		return string(spec.SemanticSourceOpenAPIRest)
	default:
		return string(spec.SemanticSourceSDKExplicit)
	}
}

// preserveReviewedCapability keeps human-maintained presentation fields on
// rebuilds triggered by function registration. Registration data never
// carries resource-level presentation, so any stored value is reviewed data
// and must survive automated re-registration.
func preserveReviewedCapability(next *model.ResourceCapability, existing *model.ResourceCapability) {
	if next == nil || existing == nil {
		return
	}
	if len(existing.Labels) > 0 {
		next.Labels = existing.Labels
	}
	if len(existing.Description) > 0 {
		next.Description = existing.Description
	}
	if strings.TrimSpace(existing.CategoryKey) != "" {
		next.CategoryKey = existing.CategoryKey
	}
	if len(existing.Tags) > 0 {
		next.Tags = existing.Tags
	}
	if strings.TrimSpace(existing.UpdatedBy) != "" {
		next.UpdatedBy = existing.UpdatedBy
	}
}

func idLive(live map[uint]struct{}, id uint) bool {
	_, ok := live[id]
	return ok
}

func preserveReviewedSemantics(next *model.CapabilitySemantics, existing *model.CapabilitySemantics, liveContractIDs map[uint]struct{}) {
	if next == nil || existing == nil {
		return
	}
	if strings.TrimSpace(existing.Source) != string(spec.SemanticSourcePlatformReview) {
		return
	}
	next.Source = existing.Source
	next.UpdatedBy = existing.UpdatedBy
	next.Provenance = existing.Provenance
	next.Conflicts = existing.Conflicts
	next.Diagnostics = existing.Diagnostics
	if strings.TrimSpace(existing.IdentityField) != "" {
		next.IdentityField = existing.IdentityField
		next.IdentityFieldType = existing.IdentityFieldType
		next.IdentityPath = existing.IdentityPath
	}
	if existing.CollectionQueryID > 0 && next.CollectionQueryID == 0 && idLive(liveContractIDs, existing.CollectionQueryID) {
		next.CollectionQueryID = existing.CollectionQueryID
	}
	if strings.TrimSpace(existing.CollectionPath) != "" {
		next.CollectionPath = existing.CollectionPath
	}
	if strings.TrimSpace(existing.PageFieldName) != "" {
		next.PageFieldName = existing.PageFieldName
	}
	if strings.TrimSpace(existing.PageSizeFieldName) != "" {
		next.PageSizeFieldName = existing.PageSizeFieldName
	}
	if strings.TrimSpace(existing.ItemsFieldName) != "" {
		next.ItemsFieldName = existing.ItemsFieldName
	}
	if strings.TrimSpace(existing.TotalFieldName) != "" {
		next.TotalFieldName = existing.TotalFieldName
	}
	if existing.ItemQueryID > 0 && next.ItemQueryID == 0 && idLive(liveContractIDs, existing.ItemQueryID) {
		next.ItemQueryID = existing.ItemQueryID
	}
	if strings.TrimSpace(existing.ItemPath) != "" {
		next.ItemPath = existing.ItemPath
	}
	if existing.CreateID > 0 && next.CreateID == 0 && idLive(liveContractIDs, existing.CreateID) {
		next.CreateID = existing.CreateID
	}
	if existing.UpdateID > 0 && next.UpdateID == 0 && idLive(liveContractIDs, existing.UpdateID) {
		next.UpdateID = existing.UpdateID
	}
	if existing.DeleteID > 0 && next.DeleteID == 0 && idLive(liveContractIDs, existing.DeleteID) {
		next.DeleteID = existing.DeleteID
	}
	if len(existing.Actions) > 0 {
		next.Actions = existing.Actions
	}
	if len(existing.Tasks) > 0 {
		next.Tasks = existing.Tasks
	}
	if len(existing.Reports) > 0 {
		next.Reports = existing.Reports
	}
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
		setIdentityDiagnostic(sem, "resource_identity_not_verifiable", "collection item schema does not expose a verifiable resource identity")
		return
	}
	candidateKeys := []string{"id", sem.ResourceKey + "_id", sem.ResourceKey + "Id"}
	type identityCandidate struct {
		field     string
		valueType string
	}
	candidates := make([]identityCandidate, 0, len(candidateKeys))
	seen := make(map[string]struct{}, len(candidateKeys))
	for _, key := range candidateKeys {
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		raw, ok := props[key]
		if !ok {
			continue
		}
		valueType := schemaScalarType(parseJSONSchema(raw))
		if valueType == "" {
			continue
		}
		candidates = append(candidates, identityCandidate{field: key, valueType: valueType})
	}
	if len(candidates) == 0 {
		setIdentityDiagnostic(sem, "resource_identity_not_verifiable", "collection item schema has no supported explicit identity field")
		return
	}
	if len(candidates) > 1 {
		setIdentityDiagnostic(sem, "resource_identity_ambiguous", "collection item schema exposes multiple identity candidates")
		return
	}
	sem.IdentityField = candidates[0].field
	sem.IdentityFieldType = candidates[0].valueType
	sem.IdentityPath = "/" + escapeJSONPointerToken(candidates[0].field)
}

func setIdentityDiagnostic(sem *model.CapabilitySemantics, code, message string) {
	if sem == nil {
		return
	}
	sem.Diagnostics = toJSON([]spec.Diagnostic{{
		Code:     code,
		Severity: spec.SeverityWarning,
		Message:  message,
		Field:    "identityField",
	}})
}

func escapeJSONPointerToken(token string) string {
	token = strings.ReplaceAll(token, "~", "~0")
	return strings.ReplaceAll(token, "/", "~1")
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

// FunctionMetaInput remains a concise service-level alias for the canonical
// registration contract. It does not create a second DTO or conversion path.
type FunctionMetaInput = spec.FunctionContractInput

// timeoutMsToInt32 把归一后的契约预算（int）安全收窄为 int32。
// 归一层已 clamp [1000, 60000]，这里再设防御上界——收窄转换必须显式
// 有界（CodeQL go/incorrect-integer-conversion），负值按未声明归零。
func timeoutMsToInt32(v int) int32 {
	if v < 0 {
		return 0
	}
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(v)
}

func computeDigest(v interface{}) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:])
}

func toJSON(v interface{}) model.JSON {
	b, _ := json.Marshal(v)
	return model.JSON(b)
}

// mergeSchemaDiffDiagnostics 在契约 upsert 前对比库中现有契约的
// input/output schema：破坏性变更（F12）作为 Diagnostic 追加到现有
// diagnostics 之后；无现有契约（首次注册）或非破坏性差异时原样返回。
// 同时返回全部 findings 与是否为更新（供 F13 审计消费）。
func mergeSchemaDiffDiagnostics(
	ctx context.Context,
	contractModel *model.FunctionContractModel,
	gameID, env, functionID string,
	contract *model.FunctionContract,
	base model.JSON,
) (model.JSON, []schemadiff.Finding, bool) {
	existing, err := contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
	if err != nil {
		// 首次注册（not found）或查询失败都不阻断；查询异常记录后跳过
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Warn("schema diff: failed to load existing contract",
				"game_id", gameID, "env", env, "function_id", functionID, "error", err)
		}
		return base, nil, false
	}

	findings := schemadiff.DiffSchemas("inputSchema", json.RawMessage(existing.InputSchema), json.RawMessage(contract.InputSchema))
	findings = append(findings,
		schemadiff.DiffSchemas("outputSchema", json.RawMessage(existing.OutputSchema), json.RawMessage(contract.OutputSchema))...)
	if !schemadiff.HasBreaking(findings) {
		return base, findings, true
	}

	// 现有 diagnostics（数组）之上追加 schema diff 条目
	var diags []interface{}
	if len(base) > 0 {
		_ = json.Unmarshal(base, &diags)
	}
	for _, finding := range findings {
		if finding.Severity != schemadiff.SeverityBreaking {
			continue
		}
		diags = append(diags, map[string]interface{}{
			"code":     "schema_breaking_change",
			"severity": "warning",
			"message":  fmt.Sprintf("%s%s: %s", finding.Source, finding.Path, finding.Reason),
			"field":    finding.Source,
		})
	}
	return toJSON(diags), findings, true
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

func contractValidationError(diags []spec.Diagnostic) error {
	for _, diag := range diags {
		if diag.Severity != spec.SeverityError {
			continue
		}
		field := strings.TrimSpace(diag.Field)
		if field == "" {
			field = "descriptor"
		}
		return fmt.Errorf("function contract validation failed: %s: %s", field, diag.Message)
	}
	return nil
}

// ListContracts lists all contracts in a scope.
func (s *ContractService) ListContracts(ctx context.Context, gameID, env string) ([]*model.FunctionContract, error) {
	return s.contractModel.ListByScope(ctx, gameID, env)
}

// GetContract gets a contract by function ID.
func (s *ContractService) GetContract(ctx context.Context, gameID, env, functionID string) (*model.FunctionContract, error) {
	return s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
}

// RemoveFunctionContract removes an executable registration contract and its
// standalone generated proposal. Published snapshots are intentionally kept:
// freshness then reports binding_function_missing and rejects execution.
func (s *ContractService) RemoveFunctionContract(ctx context.Context, gameID, env, functionID string) (string, error) {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return "", nil
	}
	contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find function contract %s: %w", functionID, err)
	}
	resourceKey := strings.TrimSpace(contract.ResourceKey)
	if err := s.contractModel.DeleteByScopeAndFunctionID(ctx, gameID, env, functionID); err != nil {
		return "", fmt.Errorf("delete function contract %s: %w", functionID, err)
	}
	if err := s.removeStandaloneProposalsForFunction(ctx, gameID, env, functionID); err != nil {
		return "", err
	}
	if err := s.resolveBlockedIssue(ctx, gameID, env, resourceKey, functionID); err != nil {
		return "", err
	}
	return resourceKey, nil
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.removeResourceProposal(ctx, gameID, env, resourceKey)
		}
		return fmt.Errorf("find capability semantics: %w", err)
	}

	// Get contracts for this resource
	contracts, err := s.contractModel.ListByResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		return fmt.Errorf("list contracts: %w", err)
	}
	if len(contracts) == 0 {
		return s.removeResourceProposal(ctx, gameID, env, resourceKey)
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

	consumedBindings, err := s.upsertResourceProposal(ctx, gameID, env, semantics, contracts)
	if err != nil {
		return err
	}
	if err := s.upsertStandaloneProposals(ctx, gameID, env, contracts, semantics, consumedBindings); err != nil {
		return err
	}
	return nil
}

func (s *ContractService) removeResourceProposal(ctx context.Context, gameID, env, resourceKey string) error {
	resourceKey = strings.TrimSpace(resourceKey)
	if resourceKey == "" {
		return nil
	}
	if err := s.proposalModel.DeleteByScopeAndKey(ctx, gameID, env, resourceProposalKey(resourceKey)); err != nil {
		return fmt.Errorf("delete resource proposal %s: %w", resourceKey, err)
	}
	return s.resolveBlockedIssue(ctx, gameID, env, resourceKey, "")
}

// RebuildProposalForFunction creates or refreshes the standalone page proposal
// for a function that cannot be safely grouped into a ResourcePage.
// loadTermDictionary loads the platform term dictionary for generated
// category labels and title fallbacks. A nil/empty result keeps generation
// on the humanize fallback path.
func (s *ContractService) loadTermDictionary(ctx context.Context) generator.TermDictionary {
	if s == nil || s.db == nil {
		return nil
	}
	items, err := model.NewTermDictionaryModel(s.db).List(ctx, "")
	if err != nil {
		return nil
	}
	out := make(generator.TermDictionary, len(items))
	for _, item := range items {
		text := spec.LocalizedText(model.NormalizeTermDisplay(item.Display))
		if len(text) == 0 {
			continue
		}
		out[strings.ToLower(strings.TrimSpace(item.Domain))+"/"+strings.ToLower(strings.TrimSpace(item.Alias))] = text
	}
	return out
}

func (s *ContractService) RebuildProposalForFunction(ctx context.Context, gameID, env, functionID string) error {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return nil
	}
	contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("find function contract %s: %w", functionID, err)
	}
	if contract == nil {
		return nil
	}
	if isCRUDCapability(contract.Capability.String()) && strings.TrimSpace(contract.ResourceKey) != "" {
		return nil
	}
	functions := map[string]spec.FunctionSpec{
		functionID: FunctionSpecFromContract(contract),
	}
	var taskSemantics map[string]spec.TaskSemantic
	var reportSemantics map[string]spec.ReportSemantic
	if resourceKey := strings.TrimSpace(contract.ResourceKey); resourceKey != "" {
		if semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey); err == nil {
			taskSemantics = taskSemanticsByStartFunction(semantics)
			reportSemantics = reportSemanticsByQueryFunction(semantics)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find capability semantics for %s: %w", resourceKey, err)
		}
	}
	generated := generator.GenerateForOperation(OperationSpecFromContract(contract), generator.GenerateOptions{
		DefaultLocale:   "zh-CN",
		Functions:       functions,
		TaskSemantics:   taskSemantics,
		ReportSemantics: reportSemantics,
		Terms:           s.loadTermDictionary(ctx),
	})
	if generator.ShouldBlockProposal(generated.Diagnostics) {
		return s.upsertBlockedIssue(ctx, gameID, env, contract.ResourceKey, contract.FunctionID, generated.Diagnostics, contractsForIssue(contract))
	}
	proposalKey := standaloneProposalKey(generated.Type, contract.FunctionID)
	if err := s.upsertGeneratedProposal(ctx, gameID, env, proposalKey, nil, []*model.FunctionContract{contract}, generated); err != nil {
		return err
	}
	return s.resolveBlockedIssue(ctx, gameID, env, contract.ResourceKey, contract.FunctionID)
}

func (s *ContractService) upsertResourceProposal(
	ctx context.Context,
	gameID string,
	env string,
	semantics *model.CapabilitySemantics,
	contracts []*model.FunctionContract,
) (map[string]struct{}, error) {
	resourceOptions := generator.DefaultGenerateOptions()
	resourceOptions.Terms = s.loadTermDictionary(ctx)
	generated, ok := generator.GenerateResourcePageProposal(semantics, contracts, resourceOptions)
	if !ok {
		if err := s.removeResourceProposal(ctx, gameID, env, semantics.ResourceKey); err != nil {
			return nil, err
		}
		return map[string]struct{}{}, nil
	}
	if generator.ShouldBlockProposal(generated.Diagnostics) {
		if err := s.removeResourceProposal(ctx, gameID, env, semantics.ResourceKey); err != nil {
			return nil, err
		}
		if err := s.upsertBlockedIssue(ctx, gameID, env, semantics.ResourceKey, "", generated.Diagnostics, contracts); err != nil {
			return nil, err
		}
		return map[string]struct{}{}, nil
	}
	consumed := consumedPageBindings(generated.Bindings, contracts)
	if err := s.upsertGeneratedProposal(ctx, gameID, env, resourceProposalKey(semantics.ResourceKey), semantics, contracts, generated); err != nil {
		return nil, err
	}
	if err := s.resolveBlockedIssue(ctx, gameID, env, semantics.ResourceKey, ""); err != nil {
		return nil, err
	}
	return consumed, nil
}

func (s *ContractService) upsertStandaloneProposals(
	ctx context.Context,
	gameID string,
	env string,
	contracts []*model.FunctionContract,
	semantics *model.CapabilitySemantics,
	consumed map[string]struct{},
) error {
	taskSemantics := taskSemanticsByStartFunction(semantics)
	reportSemantics := reportSemanticsByQueryFunction(semantics)
	functions := make(map[string]spec.FunctionSpec, len(contracts))
	for _, contract := range contracts {
		if contract == nil || strings.TrimSpace(contract.FunctionID) == "" {
			continue
		}
		functions[strings.TrimSpace(contract.FunctionID)] = FunctionSpecFromContract(contract)
	}
	for _, contract := range contracts {
		if contract == nil || strings.TrimSpace(contract.FunctionID) == "" {
			continue
		}
		functionID := strings.TrimSpace(contract.FunctionID)
		if _, ok := consumed[functionID]; ok {
			if err := s.removeStandaloneProposalsForFunction(ctx, gameID, env, functionID); err != nil {
				return err
			}
			continue
		}
		generated := generator.GenerateForOperation(OperationSpecFromContract(contract), generator.GenerateOptions{
			DefaultLocale:   "zh-CN",
			Functions:       functions,
			TaskSemantics:   taskSemantics,
			ReportSemantics: reportSemantics,
			Terms:           s.loadTermDictionary(ctx),
		})
		if generator.ShouldBlockProposal(generated.Diagnostics) {
			if err := s.upsertBlockedIssue(ctx, gameID, env, contract.ResourceKey, functionID, generated.Diagnostics, contractsForIssue(contract)); err != nil {
				return err
			}
			continue
		}
		proposalKey := standaloneProposalKey(generated.Type, contract.FunctionID)
		if err := s.upsertGeneratedProposal(ctx, gameID, env, proposalKey, nil, []*model.FunctionContract{contract}, generated); err != nil {
			return err
		}
	}
	return nil
}

func (s *ContractService) removeStandaloneProposalsForFunction(ctx context.Context, gameID, env, functionID string) error {
	for _, pageType := range []spec.PageType{spec.PageTypeOperation, spec.PageTypeTask, spec.PageTypeReport} {
		if err := s.proposalModel.DeleteByScopeAndKey(ctx, gameID, env, standaloneProposalKey(pageType, functionID)); err != nil {
			return fmt.Errorf("delete standalone proposal for %s: %w", functionID, err)
		}
	}
	return nil
}

func contractsForIssue(contract *model.FunctionContract) []*model.FunctionContract {
	if contract == nil {
		return nil
	}
	return []*model.FunctionContract{contract}
}

func (s *ContractService) upsertBlockedIssue(ctx context.Context, gameID, env, resourceKey, functionID string, diagnostics []spec.Diagnostic, contracts []*model.FunctionContract) error {
	issue := generator.CreateBlockedProposalIssue(gameID, env, resourceKey, functionID, diagnostics, "zh-CN")
	if err := s.blockedIssues.Upsert(ctx, &model.BlockedProposalIssue{
		GameID:        issue.GameID,
		Env:           issue.Env,
		ResourceKey:   issue.ResourceKey,
		FunctionID:    issue.FunctionID,
		SourceDigests: toJSON([]string{computeDigest(contracts)}),
		Diagnostics:   toJSON(issue.Diagnostics),
		RepairHint:    toJSONMap(issue.RepairHint),
		Status:        issue.Status,
		UpdatedBy:     "system",
	}); err != nil {
		return fmt.Errorf("upsert blocked proposal issue: %w", err)
	}
	return nil
}

func (s *ContractService) resolveBlockedIssue(ctx context.Context, gameID, env, resourceKey, functionID string) error {
	if err := s.blockedIssues.Resolve(ctx, gameID, env, resourceKey, functionID, "system"); err != nil {
		return fmt.Errorf("resolve blocked proposal issue: %w", err)
	}
	return nil
}

func consumedPageBindings(bindings []spec.PageFunctionBinding, contracts []*model.FunctionContract) map[string]struct{} {
	consumed := map[string]struct{}{}
	for _, binding := range bindings {
		contract := findContractByFunctionID(contracts, binding.FunctionID)
		if contract == nil {
			continue
		}
		functionID := strings.TrimSpace(contract.FunctionID)
		if functionID != "" {
			consumed[functionID] = struct{}{}
		}
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

func taskSemanticsByStartFunction(semantics *model.CapabilitySemantics) map[string]spec.TaskSemantic {
	out := map[string]spec.TaskSemantic{}
	if semantics == nil || len(semantics.Tasks) == 0 {
		return out
	}
	var items []spec.TaskSemantic
	if err := json.Unmarshal(semantics.Tasks, &items); err != nil {
		return out
	}
	for _, item := range items {
		functionID := strings.TrimSpace(item.Start.FunctionID)
		if functionID == "" {
			continue
		}
		out[functionID] = item
	}
	return out
}

func reportSemanticsByQueryFunction(semantics *model.CapabilitySemantics) map[string]spec.ReportSemantic {
	out := map[string]spec.ReportSemantic{}
	if semantics == nil || len(semantics.Reports) == 0 {
		return out
	}
	var items []spec.ReportSemantic
	if err := json.Unmarshal(semantics.Reports, &items); err != nil {
		return out
	}
	for _, item := range items {
		functionID := strings.TrimSpace(item.Query.FunctionID)
		if functionID == "" {
			continue
		}
		out[functionID] = item
	}
	return out
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
		GeneratorVersion: PageProposalGeneratorVersion,
		FunctionDigest:   computeDigest(contracts),
		SemanticsDigest:  computeDigest(semantics),
		Title:            toJSONMap(generated.Title),
		Description:      toJSONMap(generated.Description),
		CategoryKey:      generated.Category.Key,
		PageSpec:         model.JSON(pageJSON),
		Diagnostics:      toJSON(generated.Diagnostics),
		Status:           dbenum.ProposalStatusPending,
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
		PageSpec         model.JSON
		Diagnostics      model.JSON
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

func preserveGeneratedProposalStatus(status dbenum.ProposalStatus) dbenum.ProposalStatus {
	switch status {
	case dbenum.ProposalStatusAccepted, dbenum.ProposalStatusRejected:
		return status
	default:
		return dbenum.ProposalStatusPending
	}
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

// mustParseCapability converts a normalized wire capability into the DB
// enum. The normalizer only emits controlled values, so a parse failure is a
// programming error and gets surfaced instead of silently stored.
func mustParseCapability(wire string) dbenum.Capability {
	parsed, err := dbenum.ParseCapability(wire)
	if err != nil {
		return dbenum.CapabilityUnknown
	}
	return parsed
}

func mustParseRisk(wire string) dbenum.Risk {
	parsed, err := dbenum.ParseRisk(wire)
	if err != nil {
		return dbenum.RiskSafe
	}
	return parsed
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

// CreateCompositeProposal 手动创建组合页提案：每区块选一个函数
// （任意资源）+ 视图形态 + 联动声明，生成自由组合页提案进
// ProposalInbox——区块间经 page_state 联动（选玩家→联动刷新
// 订单/背包等）。
func (s *ContractService) CreateCompositeProposal(
	ctx context.Context,
	gameID, env, pageKey string,
	sections []CompositeSectionRequest,
) (proposal *model.PageProposal, err error) {
	// 组合页创建链路新近重构——panic 显式转为错误（含栈）返回前端，
	// 避免 500 空响应无诊断。
	defer func() {
		if r := recover(); r != nil {
			slog.Error("CreateCompositeProposal panic", "recover", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("internal error creating composite page: %v", r)
		}
	}()
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	pageKey = strings.TrimSpace(pageKey)
	if pageKey == "" || len(sections) < 2 {
		return nil, fmt.Errorf("pageKey and 2+ sections are required")
	}

	var contracts []*model.FunctionContract
	inputs := make([]generator.CompositeSectionInput, 0, len(sections))
	seenKey := map[string]bool{}
	contractCache := map[string]*model.FunctionContract{}
	for _, sec := range sections {
		fid := strings.TrimSpace(sec.FunctionID)
		if fid == "" {
			continue
		}
		// 同函数多实例：不再丢弃，靠区块 key 唯一区分
		//（同一 player.list 可拖两个表格分别配列/联动）。
		key := strings.TrimSpace(sec.Key)
		if key == "" {
			key = fid
		}
		if seenKey[key] {
			return nil, fmt.Errorf("duplicate section key %q (同函数多实例请提供不同 key)", key)
		}
		seenKey[key] = true
		contract, ok := contractCache[fid]
		if !ok {
			var err error
			contract, err = s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, fid)
			if err != nil {
				return nil, fmt.Errorf("function %s contract not found: %w", fid, err)
			}
			contractCache[fid] = contract
		}
		contracts = append(contracts, contract)
		in := generator.CompositeSectionInput{
			Key:        key,
			FunctionID: fid,
			View:       sec.View,
			Title:      sec.Title,
			Span:       sec.Span,
			AutoRun:    sec.AutoRun,
			RefreshOn:  sec.RefreshOn,
			Display:    sec.Display,
			OnSuccess:  sec.OnSuccessRefresh,
			Events:     convEvents(sec.Events),
		}
		for _, ra := range sec.RowActions {
			in.RowActions = append(in.RowActions, generator.CompositeRowActionInput{
				Label:         ra.Label,
				TargetSection: ra.TargetSection,
				Params:        ra.Params,
				Danger:        ra.Danger,
				Chain:         convChain(ra.Chain),
			})
		}
		for _, ta := range sec.ToolbarActions {
			in.Toolbar = append(in.Toolbar, generator.CompositeToolbarActionInput{
				Label:         ta.Label,
				TargetSection: ta.TargetSection,
				Params:        ta.Params,
				Danger:        ta.Danger,
				Chain:         convChain(ta.Chain),
			})
		}
		inputs = append(inputs, in)
	}

	opts := generator.DefaultGenerateOptions()
	opts.Terms = s.loadTermDictionary(ctx)
	generated, ok := generator.GenerateCompositePage(pageKey, inputs, contracts, opts)
	if !ok {
		return nil, fmt.Errorf("composite page cannot be generated (see diagnostics)")
	}
	proposalKey := compositeProposalKey(pageKey)
	if err := s.upsertGeneratedProposal(ctx, gameID, env, proposalKey, nil, contracts, generated); err != nil {
		return nil, err
	}
	return s.proposalModel.FindByScopeAndKey(ctx, gameID, env, proposalKey)
}

// CompositeSectionRequest 组合页区块输入。
type CompositeSectionRequest struct {
	// Key 区块唯一标识（同函数多实例时必填以区分；缺省=函数 id）。
	Key        string   `json:"key,omitempty"`
	FunctionID string   `json:"functionId"`
	View       string   `json:"view"` // table|fields|form|actions|toolbar（空=按能力推导）
	Title      string   `json:"title,omitempty"`
	Span       int      `json:"span,omitempty"`
	AutoRun    bool     `json:"autoRun,omitempty"`
	RefreshOn  []string `json:"refreshOn,omitempty"`
	// Display inline（默认）| dialog（弹窗表单，按钮触发）。
	Display string `json:"display,omitempty"`
	// RowActions 表格行操作（view=table）。
	RowActions []CompositeRowActionRequest `json:"rowActions,omitempty"`
	// ToolbarActions 工具栏按钮（view=toolbar）。
	ToolbarActions []CompositeToolbarActionRequest `json:"toolbarActions,omitempty"`
	// OnSuccessRefresh 操作成功后自动重跑的区块 key。
	OnSuccessRefresh []string `json:"onSuccessRefresh,omitempty"`
	// Events 通用事件绑定（发布触发点）。
	Events []EventBindingReq `json:"events,omitempty"`
}

// EventBindingReq 事件绑定请求。
type EventBindingReq struct {
	Event  string          `json:"event"`
	Action ActionStepReq   `json:"action"`
	Chain  []ActionStepReq `json:"chain,omitempty"`
}

// CompositeRowActionRequest 表格行操作输入。
type CompositeRowActionRequest struct {
	Label         string            `json:"label"`
	TargetSection string            `json:"targetSection,omitempty"`
	Params        map[string]string `json:"params,omitempty"`
	Danger        bool              `json:"danger,omitempty"`
	Chain         []ActionStepReq   `json:"chain,omitempty"`
}

// CompositeToolbarActionRequest 工具栏按钮输入。
type CompositeToolbarActionRequest struct {
	Label         string            `json:"label"`
	TargetSection string            `json:"targetSection,omitempty"`
	Params        map[string]string `json:"params,omitempty"`
	Danger        bool              `json:"danger,omitempty"`
	Chain         []ActionStepReq   `json:"chain,omitempty"`
}

// ActionStepReq 动作链单步。
type ActionStepReq struct {
	Kind   string            `json:"kind"`
	Target string            `json:"target"`
	Params map[string]string `json:"params,omitempty"`
}

// convChain 请求动作链 → generator 输入。
func convChain(steps []ActionStepReq) []spec.CompositeActionStep {
	if len(steps) == 0 {
		return nil
	}
	out := make([]spec.CompositeActionStep, 0, len(steps))
	for _, st := range steps {
		out = append(out, spec.CompositeActionStep{Kind: st.Kind, Target: st.Target, Params: st.Params})
	}
	return out
}

// convEvents 事件绑定请求 → generator 输入。
func convEvents(evs []EventBindingReq) []spec.CompositeEventBinding {
	if len(evs) == 0 {
		return nil
	}
	out := make([]spec.CompositeEventBinding, 0, len(evs))
	for _, ev := range evs {
		out = append(out, spec.CompositeEventBinding{
			Event:  ev.Event,
			Action: spec.CompositeActionStep{Kind: ev.Action.Kind, Target: ev.Action.Target, Params: ev.Action.Params},
			Chain:  convChain(ev.Chain),
		})
	}
	return out
}

func compositeProposalKey(pageKey string) string {
	return "composite--" + strings.TrimSpace(pageKey)
}
