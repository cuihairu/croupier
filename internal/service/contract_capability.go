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

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/function/schemadiff"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// backfillInputFromClassification 用既有 unbound 契约（OpenAPI 上传物料）
// 回补运行时注册入参中缺失的分类字段（D3/T6）：schema/分类以运行时与
// 上传物料中非空一方为准——运行时 schema 是真实 I/O 契约，OpenAPI 分类
// （resource/capability/risk/permission/approval）驱动页面生成与风控，
// 两边谁有值用谁，均空保持空。Summary 回补保证提案标题不退化。
func backfillInputFromClassification(input *spec.FunctionContractInput, existing *model.FunctionContract) {
	if input == nil || existing == nil {
		return
	}
	if strings.TrimSpace(input.Resource) == "" {
		input.Resource = strings.TrimSpace(existing.ResourceKey)
	}
	if strings.TrimSpace(input.Operation) == "" {
		input.Operation = strings.TrimSpace(existing.OperationKey)
	}
	if strings.TrimSpace(input.Capability) == "" {
		input.Capability = existing.Capability.String()
	}
	if strings.TrimSpace(input.Execution) == "" {
		input.Execution = strings.TrimSpace(existing.Execution)
	}
	if strings.TrimSpace(input.Risk) == "" {
		input.Risk = existing.Risk.String()
	}
	// SDK 重注册路径不携带 timeout（agentv1.FunctionDescriptor 无此字段），
	// 0 按「未声明」处理：保留 OpenAPI 上传等路径已声明的预算，避免静默清零。
	if input.TimeoutMs <= 0 {
		input.TimeoutMs = int(existing.TimeoutMs)
	}
	if strings.TrimSpace(input.Permission) == "" {
		input.Permission = strings.TrimSpace(existing.Permission)
	}
	if !input.ApprovalRequired {
		input.ApprovalRequired = existing.Approval["required"] == true
	}
	if strings.TrimSpace(input.ApprovalPolicyKey) == "" {
		if policyKey, ok := existing.Approval["policyKey"].(string); ok {
			input.ApprovalPolicyKey = strings.TrimSpace(policyKey)
		}
	}
	if strings.TrimSpace(input.Summary) == "" {
		input.Summary = localizedStringFromJSONMap(existing.Summary)
	}
	if strings.TrimSpace(input.InputSchema) == "" {
		input.InputSchema = strings.TrimSpace(string(existing.InputSchema))
	}
	if strings.TrimSpace(input.OutputSchema) == "" {
		input.OutputSchema = strings.TrimSpace(string(existing.OutputSchema))
	}
}

// localizedStringFromJSONMap 从契约行的 Summary/Description JSONMap
// （LocalizedText 形态）提取展示文案：优先系统默认语言，其次 en-US，
// 任意非空值兜底。
func localizedStringFromJSONMap(m datatypes.JSONMap) string {
	if len(m) == 0 {
		return ""
	}
	for _, locale := range []string{"zh-CN", "en-US"} {
		if v, ok := m[locale].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	for _, v := range m {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// logAutoBindingAudit D3/T6：unbound 契约被运行时注册自动置 bound 时写
// openapi_source.binding_auto 审计事件；audit 服务未注入或写入失败不影响
// 注册主流程。
func (s *ContractService) logAutoBindingAudit(ctx context.Context, gameID, env, source, functionID string, previous *model.FunctionContract) {
	if s.auditSvc == nil {
		return
	}
	details := map[string]interface{}{
		"gameId":         gameID,
		"env":            env,
		"functionId":     functionID,
		"source":         source,
		"previousState":  string(spec.ExecutionStateUnbound),
		"executionState": string(spec.ExecutionStateBound),
	}
	if previous != nil {
		details["materialSource"] = previous.Source
	}
	if _, err := s.auditSvc.Log(ctx, audit.EventOpenAPISourceBindingAuto,
		audit.WithResourceID("function_contract", functionID),
		audit.WithActorID("system", "system", "Contract Rebuild"),
		audit.WithDetails(details),
	); err != nil {
		// 副作用（契约绑定）已完成：呈现缺口而非静默，也不返回 5xx。
		s.auditSvc.ReportWriteGap(ctx, audit.EventOpenAPISourceBindingAuto, err)
	}
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
		// 副作用（契约更新）已完成：呈现缺口而非静默，也不返回 5xx。
		s.auditSvc.ReportWriteGap(ctx, audit.EventFunctionContractUpdated, err)
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
	// BUG-031 第三病灶：内容未变的重注册（agent 重连）不得 bump 版本，也不得
	// 追加语义版本历史行——否则每次重连灌一版历史+一版提案快照。
	changed, err := s.semanticsModel.UpsertSemantics(ctx, semantics)
	if err != nil {
		return fmt.Errorf("upsert capability semantics: %w", err)
	}

	// 4. Create version record only when semantics actually changed. This
	// rebuild is triggered by automated registration sync, not a human
	// operator, so the actor is "system".
	if changed {
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
	// 候选三键恒互异，原 duplicate 去重分支为死代码，已删（连带 seen map）：
	// "id" / resourceKey+"_id" / resourceKey+"Id" 两两比较——后两者同前缀不同
	// 后缀（"_id" != "Id"）恒不等；resourceKey+"_id"=="id" 需 resourceKey==""
	//（得 "_id"≠"id"），resourceKey+"Id"=="id" 同理（"Id"≠"id"，大小写不同）。
	for _, key := range candidateKeys {
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
