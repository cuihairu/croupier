package console

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/api/function"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/cuihairu/croupier/internal/validation"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) Menu(ctx context.Context, req *ConsoleMenuRequest) (*ConsoleMenuResponse, error) {
	if err := s.requireConsoleRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	publishedPages, err := s.svcCtx.PublishedPageSpecModel.ListLatestActiveByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	pages := parsePublishedPages(publishedPages)
	menu := generateMenuFromPages(pages, normalizeLanguage(req.Language))
	return &ConsoleMenuResponse{ConsoleMenuSpec: menu}, nil
}

func (s *Service) Pages(ctx context.Context, req *ConsolePagesRequest) (*ConsolePagesResponse, error) {
	if err := s.requireConsoleRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	publishedPages, err := s.svcCtx.PublishedPageSpecModel.ListLatestActiveByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	pages, err := s.attachBindingFreshness(ctx, parsePublishedPages(publishedPages))
	if err != nil {
		return nil, err
	}
	items := make([]spec.PublishedPageSpec, 0, len(pages))
	for _, page := range pages {
		if req.Category != "" && page.Category.Key != req.Category {
			continue
		}
		items = append(items, page)
	}
	return &ConsolePagesResponse{Items: items}, nil
}

func (s *Service) Page(ctx context.Context, req *ConsolePageRequest) (*ConsolePageResponse, error) {
	if err := s.requireConsoleRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	pp, err := s.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	pageSpec := parsePublishedPageSpec(*pp)
	if pageSpec == nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	if err := s.attachBindingFreshnessToPage(ctx, pageSpec); err != nil {
		return nil, err
	}
	return &ConsolePageResponse{Page: *pageSpec}, nil
}

func (s *Service) ExecuteBinding(ctx context.Context, req *ConsoleExecuteBindingRequest) (resp *ConsoleExecuteBindingResponse, err error) {
	if err := s.requireConsoleExecute(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	pp, err := s.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	published := parsePublishedPageSpec(*pp)
	if published == nil {
		return nil, ErrPageNotFound(req.PageKey)
	}

	binding, ok := findBinding(published.Bindings, req.BindingID)
	if !ok {
		return nil, errorx.NewNotFound("page binding not found")
	}
	contract, ok := findContract(published.BindingContracts, req.BindingID)
	if !ok {
		return nil, errorx.NewValidationError("binding contract snapshot missing")
	}
	functions, err := loadPublishedFunctionSpecs(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	requestID := uuid.NewString()
	actor := currentActor(ctx)
	var result spec.PageExecutionResult
	var target string
	ctx, finishSpan := s.startPageExecuteSpan(ctx, gameID, env, *published, binding, contract, requestID, actor)
	defer func() {
		finishSpan(err, result, target)
		s.auditPageExecute(ctx, gameID, env, *published, binding, contract, requestID, target, result, err)
	}()

	if err := s.ensureBindingFresh(binding, contract, functions); err != nil {
		return nil, err
	}
	if err := s.enforcePublishedBindingGovernance(ctx, contract); err != nil {
		return nil, err
	}

	payload, err := buildBindingPayloadFromSelectors(binding, req.Context)
	if err != nil {
		return nil, err
	}
	if err := validateBindingExecutePayload(payload, binding, functions); err != nil {
		return nil, err
	}
	mode := ""
	if binding.Execution.Mode == spec.PageExecutionModeTask {
		mode = "async"
	}
	metadata := publishedBindingExecutionMetadata(*published, binding, contract, requestID)
	if contract.Approval.Required {
		approvalID, err := s.createPageApproval(ctx, gameID, env, binding, contract, payload, mode, metadata)
		if err != nil {
			return nil, err
		}
		result = spec.PageExecutionResult{
			Kind:       spec.PageExecutionKindApproval,
			RequestID:  requestID,
			TraceID:    telemetry.TraceIDFromContext(ctx),
			ApprovalID: approvalID,
		}
		return &ConsoleExecuteBindingResponse{Result: result}, nil
	}

	functionResp, err := function.NewService(s.svcCtx).FunctionInvoke(ctx, &function.FunctionInvokeRequest{
		ID:       binding.FunctionID,
		Payload:  payload,
		GameID:   gameID,
		Env:      env,
		Mode:     mode,
		Metadata: metadata,
	})
	if err != nil {
		return nil, err
	}
	target = pageExecutionTarget(functionResp)

	result, err = buildExecutionResult(ctx, requestID, functionResp)
	if err != nil {
		return nil, err
	}
	return &ConsoleExecuteBindingResponse{Result: result}, nil
}

func (s *Service) ensureBindingFresh(binding spec.PageFunctionBinding, contract spec.BindingContractSnapshot, functions map[string]spec.FunctionSpec) error {
	diags := freshness.EvaluateBinding(binding, contract, functions)
	if len(diags) > 0 {
		return errorx.NewConflictWithDetails("binding_stale", map[string]any{
			"bindingId":  binding.ID,
			"functionId": binding.FunctionID,
			"statuses":   bindingFreshnessStatuses(diags),
		})
	}
	return nil
}

func (s *Service) enforcePublishedBindingGovernance(ctx context.Context, contract spec.BindingContractSnapshot) error {
	permission := strings.TrimSpace(contract.Permission)
	if permission == "" {
		return nil
	}
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权执行该页面操作", "admin:all", permission)
	return err
}

func publishedBindingExecutionMetadata(
	page spec.PublishedPageSpec,
	binding spec.PageFunctionBinding,
	contract spec.BindingContractSnapshot,
	requestID string,
) map[string]string {
	metadata := map[string]string{
		"page_key":                 page.PageKey,
		"page_category":            page.Category.Key,
		"publish_version":          strconv.Itoa(page.Version),
		"renderer_schema_version":  page.RendererSchemaVersion,
		"base_proposal_key":        page.BaseProposalKey,
		"function_digest":          page.FunctionDigest,
		"semantics_digest":         page.SemanticsDigest,
		"generator_version":        page.GeneratorVersion,
		"binding_id":               binding.ID,
		"binding_usage":            string(binding.Usage),
		"page_request_id":          requestID,
		"page_runtime_api":         "console.binding.execute",
		"page_snapshot_governance": "validated",
		"function_id":              binding.FunctionID,
		"snapshot_function_id":     contract.FunctionID,
		"snapshot_function_ver":    contract.FunctionVersion,
		"snapshot_execution_mode":  string(contract.ExecutionMode),
		"snapshot_risk":            string(contract.Risk),
		"snapshot_permission":      contract.Permission,
		"snapshot_approval":        strconv.FormatBool(contract.Approval.Required),
		"snapshot_approval_policy": contract.Approval.PolicyKey,
	}
	for key, value := range metadata {
		if strings.TrimSpace(value) == "" {
			delete(metadata, key)
		}
	}
	if page.BaseProposalVersion > 0 {
		metadata["base_proposal_version"] = strconv.Itoa(page.BaseProposalVersion)
	}
	return metadata
}

func (s *Service) createPageApproval(
	ctx context.Context,
	gameID string,
	env string,
	binding spec.PageFunctionBinding,
	contract spec.BindingContractSnapshot,
	payload json.RawMessage,
	mode string,
	metadata map[string]string,
) (string, error) {
	if s == nil || s.svcCtx == nil || s.svcCtx.ApprovalsStore == nil {
		return "", errorx.NewConflict("approval store unavailable")
	}
	functionID := strings.TrimSpace(firstNonEmpty(contract.FunctionID, binding.FunctionID))
	if functionID == "" {
		return "", errorx.NewValidationError("approval binding function is required")
	}
	approvalID := fmt.Sprintf("page_%s_%s", sanitizeApprovalID(functionID), strings.ReplaceAll(uuid.NewString(), "-", ""))
	approval := &approvals.Approval{
		ID:         approvalID,
		State:      "pending",
		FunctionID: functionID,
		GameID:     strings.TrimSpace(gameID),
		Env:        strings.TrimSpace(env),
		Actor:      currentActor(ctx),
		Mode:       strings.TrimSpace(mode),
		Payload:    []byte(payload),
		Metadata:   cloneStringMap(metadata),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if _, err := s.svcCtx.ApprovalsStore.Create(approval); err != nil {
		return "", fmt.Errorf("failed to create page approval: %w", err)
	}
	return approvalID, nil
}

func validateBindingExecutePayload(payload json.RawMessage, binding spec.PageFunctionBinding, functions map[string]spec.FunctionSpec) error {
	fn, ok := functions[strings.TrimSpace(binding.FunctionID)]
	if !ok || len(fn.InputSchema) == 0 {
		return nil
	}
	if err := validation.ValidateJSONRaw(json.RawMessage(fn.InputSchema), payload); err != nil {
		return errorx.NewValidationErrorWithDetails("binding payload does not match input schema", map[string]string{
			"bindingId":  binding.ID,
			"functionId": binding.FunctionID,
			"schema":     "inputSchema",
			"error":      err.Error(),
		})
	}
	return nil
}

func buildBindingPayloadFromSelectors(binding spec.PageFunctionBinding, execCtx ConsoleBindingExecutionContext) (json.RawMessage, error) {
	if binding.Selectors == nil {
		return nil, errorx.NewValidationError("binding selectors are required for console execution")
	}
	payload := map[string]json.RawMessage{}
	for _, assignment := range binding.Selectors.Input.Assignments {
		value, found, err := resolveSelectorValue(assignment.Source, execCtx)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if err := setJSONPointerValue(payload, assignment.Target, value); err != nil {
			return nil, errorx.NewValidationErrorWithDetails("binding selector target is invalid", map[string]string{
				"bindingId": binding.ID,
				"target":    assignment.Target,
				"error":     err.Error(),
			})
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func resolveSelectorValue(source spec.ValueSource, execCtx ConsoleBindingExecutionContext) (json.RawMessage, bool, error) {
	if source.Transform != nil && source.Transform.Type != spec.TransformPick {
		return nil, false, errorx.NewValidationError("binding selector transform is not supported: " + string(source.Transform.Type))
	}
	switch source.Kind {
	case spec.SourceLiteral:
		if len(source.Value) == 0 {
			return json.RawMessage("null"), true, nil
		}
		return validRawJSON(source.Value, "literal")
	case spec.SourceForm:
		return valueFromRawContext(execCtx.Form, source.Path, "form")
	case spec.SourceRow:
		return valueFromRawContext(execCtx.Row, source.Path, "row")
	case spec.SourceSelection:
		if source.Transform != nil && source.Transform.Type == spec.TransformPick {
			return pickSelectionValues(execCtx.Selection, source.Path)
		}
		return valueFromRawContext(execCtx.Selection, source.Path, "selection")
	case spec.SourceDetail:
		return valueFromRawContext(execCtx.Detail, source.Path, "detail")
	case spec.SourcePageState:
		key := strings.TrimSpace(source.Key)
		if key == "" {
			return nil, false, errorx.NewValidationError("page_state selector key is required")
		}
		if execCtx.PageState == nil {
			return nil, false, nil
		}
		raw, ok := execCtx.PageState[key]
		if !ok {
			return nil, false, nil
		}
		return valueFromRawContext(raw, source.Path, "page_state."+key)
	default:
		return nil, false, errorx.NewValidationError("unsupported binding selector source: " + string(source.Kind))
	}
}

func pickSelectionValues(raw json.RawMessage, path string) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	value, found, err := validRawJSON(raw, "selection")
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(value, &rows); err != nil {
		return nil, false, errorx.NewBadRequest("selection context must be an array")
	}
	values := make([]json.RawMessage, 0, len(rows))
	for _, row := range rows {
		selected, ok := getJSONPointerValue(row, path)
		if !ok {
			return nil, false, nil
		}
		values = append(values, selected)
	}
	result, err := json.Marshal(values)
	if err != nil {
		return nil, false, err
	}
	return result, true, nil
}

func valueFromRawContext(raw json.RawMessage, path string, sourceName string) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	value, found, err := validRawJSON(raw, sourceName)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	if path == "" {
		return value, true, nil
	}
	selected, ok := getJSONPointerValue(value, path)
	return selected, ok, nil
}

func validRawJSON(raw json.RawMessage, sourceName string) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	if !json.Valid(raw) {
		return nil, false, errorx.NewBadRequest(sourceName + " context must be valid JSON")
	}
	return append(json.RawMessage(nil), raw...), true, nil
}

func getJSONPointerValue(value json.RawMessage, path string) (json.RawMessage, bool) {
	if !isJSONPointer(path) {
		return nil, false
	}
	current := value
	for _, token := range jsonPointerTokens(path) {
		trimmed := bytes.TrimSpace(current)
		if len(trimmed) == 0 {
			return nil, false
		}
		switch trimmed[0] {
		case '{':
			var object map[string]json.RawMessage
			if err := json.Unmarshal(trimmed, &object); err != nil {
				return nil, false
			}
			next, ok := object[token]
			if !ok {
				return nil, false
			}
			current = next
		case '[':
			var array []json.RawMessage
			if err := json.Unmarshal(trimmed, &array); err != nil {
				return nil, false
			}
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(array) {
				return nil, false
			}
			current = array[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func setJSONPointerValue(payload map[string]json.RawMessage, path string, value json.RawMessage) error {
	if !isJSONPointer(path) || path == "" {
		return errorx.NewValidationError("target must be a non-empty JSON Pointer")
	}
	return setJSONObjectPointer(payload, jsonPointerTokens(path), value)
}

func setJSONObjectPointer(object map[string]json.RawMessage, tokens []string, value json.RawMessage) error {
	if len(tokens) == 0 || tokens[0] == "" {
		return errorx.NewValidationError("target contains an empty object key")
	}
	key := tokens[0]
	if len(tokens) == 1 {
		object[key] = value
		return nil
	}
	child := map[string]json.RawMessage{}
	if existing, ok := object[key]; ok {
		if err := json.Unmarshal(existing, &child); err != nil {
			return errorx.NewValidationError("target conflicts with a non-object parent")
		}
	}
	if err := setJSONObjectPointer(child, tokens[1:], value); err != nil {
		return err
	}
	raw, err := json.Marshal(child)
	if err != nil {
		return err
	}
	object[key] = raw
	return nil
}

func isJSONPointer(path string) bool {
	return path == "" || strings.HasPrefix(path, "/")
}

func jsonPointerTokens(path string) []string {
	if path == "" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts
}

func buildExecutionResult(ctx context.Context, requestID string, resp *function.FunctionInvokeResponse) (spec.PageExecutionResult, error) {
	if strings.TrimSpace(requestID) == "" {
		requestID = uuid.NewString()
	}
	traceID := telemetry.TraceIDFromContext(ctx)
	if resp == nil {
		return spec.PageExecutionResult{
			Kind:      spec.PageExecutionKindSync,
			RequestID: requestID,
			TraceID:   traceID,
		}, nil
	}
	if resp.ApprovalRequired {
		return spec.PageExecutionResult{
			Kind:       spec.PageExecutionKindApproval,
			RequestID:  requestID,
			TraceID:    traceID,
			ApprovalID: resp.ApprovalID,
		}, nil
	}
	if resp.TaskId != "" || resp.TaskID != "" {
		taskID := resp.TaskId
		if taskID == "" {
			taskID = resp.TaskID
		}
		return spec.PageExecutionResult{
			Kind:      spec.PageExecutionKindTask,
			RequestID: requestID,
			TraceID:   traceID,
			TaskID:    taskID,
		}, nil
	}
	return spec.PageExecutionResult{
		Kind:      spec.PageExecutionKindSync,
		RequestID: requestID,
		TraceID:   traceID,
		Data:      resp.Result,
	}, nil
}

func (s *Service) startPageExecuteSpan(
	ctx context.Context,
	gameID string,
	env string,
	page spec.PublishedPageSpec,
	binding spec.PageFunctionBinding,
	contract spec.BindingContractSnapshot,
	requestID string,
	actor string,
) (context.Context, func(error, spec.PageExecutionResult, string)) {
	if s == nil || s.svcCtx == nil || s.svcCtx.Telemetry == nil {
		return ctx, func(error, spec.PageExecutionResult, string) {}
	}
	startedAt := time.Now()
	nextCtx, span := s.svcCtx.Telemetry.StartSpan(ctx, "page.binding.execute",
		attribute.String("request.id", requestID),
		attribute.String("game.id", gameID),
		attribute.String("game.env", env),
		attribute.String("actor", actor),
		attribute.String("page.key", page.PageKey),
		attribute.Int("page.publish_version", page.Version),
		attribute.String("page.base_proposal_key", strings.TrimSpace(page.BaseProposalKey)),
		attribute.Int("page.base_proposal_version", page.BaseProposalVersion),
		attribute.String("page.function_digest", strings.TrimSpace(page.FunctionDigest)),
		attribute.String("page.semantics_digest", strings.TrimSpace(page.SemanticsDigest)),
		attribute.String("page.generator_version", strings.TrimSpace(page.GeneratorVersion)),
		attribute.String("page.binding_id", binding.ID),
		attribute.String("function.id", binding.FunctionID),
		attribute.String("page.binding_usage", string(binding.Usage)),
		attribute.String("page.execution_mode", string(binding.Execution.Mode)),
		attribute.String("page.snapshot_risk", string(contract.Risk)),
		attribute.String("page.snapshot_permission", strings.TrimSpace(contract.Permission)),
		attribute.Bool("page.snapshot_approval_required", contract.Approval.Required),
		attribute.String("page.snapshot_approval_policy", strings.TrimSpace(contract.Approval.PolicyKey)),
	)
	return nextCtx, func(err error, result spec.PageExecutionResult, target string) {
		attrs := []attribute.KeyValue{
			attribute.String("trace_id", telemetry.TraceIDFromContext(nextCtx)),
			attribute.String("page.result_kind", string(result.Kind)),
		}
		if target != "" {
			attrs = append(attrs, attribute.String("target", target))
		}
		if result.TaskID != "" {
			attrs = append(attrs, attribute.String("task.id", result.TaskID))
		}
		if result.ApprovalID != "" {
			attrs = append(attrs, attribute.String("approval.id", result.ApprovalID))
		}
		s.svcCtx.Telemetry.EndSpan(span, startedAt, err, attrs...)
	}
}

func (s *Service) auditPageExecute(
	ctx context.Context,
	gameID string,
	env string,
	page spec.PublishedPageSpec,
	binding spec.PageFunctionBinding,
	contract spec.BindingContractSnapshot,
	requestID string,
	target string,
	result spec.PageExecutionResult,
	executeErr error,
) {
	if s == nil || s.svcCtx == nil || s.svcCtx.AuditService == nil {
		return
	}
	actor := currentActor(ctx)
	outcome := "success"
	errorMessage := ""
	if executeErr != nil {
		outcome = "failure"
		errorMessage = executeErr.Error()
	}
	traceID := result.TraceID
	if traceID == "" {
		traceID = telemetry.TraceIDFromContext(ctx)
	}
	details := map[string]interface{}{
		"request_id":                 requestID,
		"trace_id":                   traceID,
		"game_id":                    gameID,
		"env":                        env,
		"page_key":                   page.PageKey,
		"publish_version":            page.Version,
		"base_proposal_key":          strings.TrimSpace(page.BaseProposalKey),
		"base_proposal_version":      page.BaseProposalVersion,
		"function_digest":            strings.TrimSpace(page.FunctionDigest),
		"semantics_digest":           strings.TrimSpace(page.SemanticsDigest),
		"generator_version":          strings.TrimSpace(page.GeneratorVersion),
		"binding_id":                 binding.ID,
		"binding_usage":              string(binding.Usage),
		"function_id":                binding.FunctionID,
		"target":                     target,
		"execution_mode":             string(binding.Execution.Mode),
		"snapshot_risk":              string(contract.Risk),
		"snapshot_permission":        strings.TrimSpace(contract.Permission),
		"snapshot_approval_required": contract.Approval.Required,
		"snapshot_approval_policy":   strings.TrimSpace(contract.Approval.PolicyKey),
		"result_kind":                string(result.Kind),
		"task_id":                    result.TaskID,
		"approval_id":                result.ApprovalID,
		"diagnostic_count":           len(result.Diagnostics),
	}
	_, err := s.svcCtx.AuditService.Log(ctx, audit.EventPageExecute,
		audit.WithActorID(actor, "user", actor),
		audit.WithResource(audit.ResourceInfo{
			Type:        "page",
			ID:          page.PageKey,
			Name:        page.PageKey,
			GameID:      gameID,
			Environment: env,
			Metadata: map[string]string{
				"binding_id":  binding.ID,
				"function_id": binding.FunctionID,
				"target":      target,
				"risk":        string(contract.Risk),
				"permission":  strings.TrimSpace(contract.Permission),
			},
		}),
		audit.WithDetails(details),
		audit.WithContext(audit.AuditContext{
			RequestID: requestID,
			TraceID:   traceID,
			Service:   "console",
			Tags: map[string]string{
				"page_key":    page.PageKey,
				"binding_id":  binding.ID,
				"function_id": binding.FunctionID,
				"target":      target,
			},
		}),
		audit.WithOutcome(outcome, errorMessage),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to write page execute audit event", "pageKey", page.PageKey, "bindingId", binding.ID, "error", err)
	}
}

func pageExecutionTarget(resp *function.FunctionInvokeResponse) string {
	if resp == nil {
		return ""
	}
	if agentID := strings.TrimSpace(resp.ExecutionMetadata["agent_id"]); agentID != "" {
		return agentID
	}
	if resp.Broadcast != nil {
		targets := make([]string, 0, len(resp.Broadcast.Results))
		for _, item := range resp.Broadcast.Results {
			agentID := strings.TrimSpace(item.AgentID)
			if agentID != "" {
				targets = append(targets, agentID)
			}
		}
		if len(targets) > 0 {
			sort.Strings(targets)
			return strings.Join(targets, ",")
		}
	}
	return ""
}

func currentActor(ctx context.Context) string {
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return "unknown"
	}
	return actor
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func sanitizeApprovalID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "binding"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.', r == '_', r == '-':
			builder.WriteRune('_')
		}
	}
	if builder.Len() == 0 {
		return "binding"
	}
	return builder.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parsePublishedPages(models []model.PublishedPageSpec) []spec.PublishedPageSpec {
	pages := make([]spec.PublishedPageSpec, 0, len(models))
	for _, pp := range models {
		pageSpec := parsePublishedPageSpec(pp)
		if pageSpec != nil {
			pages = append(pages, *pageSpec)
		}
	}
	return pages
}

func parsePublishedPageSpec(pp model.PublishedPageSpec) *spec.PublishedPageSpec {
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(pp.SpecJSON), &pageSpec); err != nil || pageSpec.PageKey == "" {
		return nil
	}
	var contracts []spec.BindingContractSnapshot
	if pp.BindingContractsJSON != "" {
		_ = json.Unmarshal([]byte(pp.BindingContractsJSON), &contracts)
	}
	return &spec.PublishedPageSpec{
		PageSpec:              pageSpec,
		GameID:                pp.GameID,
		Env:                   pp.Env,
		Version:               pp.Version,
		PublishedAt:           pp.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
		PublishedBy:           pp.PublishedBy,
		RendererSchemaVersion: pp.RendererSchemaVersion,
		BaseProposalKey:       pp.BaseProposalKey,
		BaseProposalVersion:   pp.BaseProposalVersion,
		FunctionDigest:        pp.FunctionDigest,
		SemanticsDigest:       pp.SemanticsDigest,
		GeneratorVersion:      pp.GeneratorVersion,
		BindingContracts:      contracts,
	}
}

func (s *Service) attachBindingFreshness(ctx context.Context, pages []spec.PublishedPageSpec) ([]spec.PublishedPageSpec, error) {
	if len(pages) == 0 {
		return pages, nil
	}
	functions, err := loadPublishedFunctionSpecs(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	for i := range pages {
		pages[i].BindingFreshness = freshness.EvaluatePublishedBindings(pages[i].Bindings, pages[i].BindingContracts, functions)
	}
	return pages, nil
}

func (s *Service) attachBindingFreshnessToPage(ctx context.Context, page *spec.PublishedPageSpec) error {
	if page == nil {
		return nil
	}
	functions, err := loadPublishedFunctionSpecs(ctx, s.svcCtx)
	if err != nil {
		return err
	}
	page.BindingFreshness = freshness.EvaluatePublishedBindings(page.Bindings, page.BindingContracts, functions)
	return nil
}

func bindingFreshnessStatuses(diags []spec.BindingFreshnessDiagnostic) []string {
	statuses := make([]string, 0, len(diags))
	for _, diag := range diags {
		if diag.Status != "" {
			statuses = append(statuses, string(diag.Status))
		}
	}
	sort.Strings(statuses)
	return statuses
}

func generateMenuFromPages(pages []spec.PublishedPageSpec, lang string) spec.ConsoleMenuSpec {
	categories := map[string]*categoryGroup{}
	for _, page := range pages {
		catKey := strings.TrimSpace(page.Category.Key)
		if catKey == "" {
			continue
		}
		if _, ok := categories[catKey]; !ok {
			categories[catKey] = &categoryGroup{
				key:    catKey,
				labels: page.Category.Labels,
				order:  page.Order,
			}
		}
		if page.Order < categories[catKey].order {
			categories[catKey].order = page.Order
		}
		categories[catKey].pages = append(categories[catKey].pages, pageEntry{
			key:   page.PageKey,
			title: page.Title,
			icon:  page.Icon,
			order: page.Order,
		})
	}

	items := make([]spec.ConsoleMenuItem, 0, len(categories))
	for _, cat := range categories {
		sort.Slice(cat.pages, func(i, j int) bool {
			if cat.pages[i].order != cat.pages[j].order {
				return cat.pages[i].order < cat.pages[j].order
			}
			left := getLocalizedText(cat.pages[i].title, lang, cat.pages[i].key)
			right := getLocalizedText(cat.pages[j].title, lang, cat.pages[j].key)
			if left != right {
				return left < right
			}
			return cat.pages[i].key < cat.pages[j].key
		})
		children := make([]spec.ConsoleMenuItem, 0, len(cat.pages))
		for _, p := range cat.pages {
			children = append(children, spec.ConsoleMenuItem{
				Key:    p.key,
				Path:   consolePagePath(cat.key, p.key),
				Title:  p.title,
				Locale: false,
				Icon:   p.icon,
				Order:  p.order,
			})
		}
		items = append(items, spec.ConsoleMenuItem{
			Key:      cat.key,
			Path:     consoleCategoryPath(cat.key),
			Title:    cat.labels,
			Locale:   false,
			Order:    cat.order,
			Children: children,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Order != items[j].Order {
			return items[i].Order < items[j].Order
		}
		left := getLocalizedText(items[i].Title, lang, items[i].Key)
		right := getLocalizedText(items[j].Title, lang, items[j].Key)
		if left != right {
			return left < right
		}
		return items[i].Key < items[j].Key
	})
	return spec.ConsoleMenuSpec{Items: items}
}

func consoleCategoryPath(categoryKey string) string {
	return "/console/" + url.PathEscape(categoryKey)
}

func consolePagePath(categoryKey string, pageKey string) string {
	return consoleCategoryPath(categoryKey) + "/" + url.PathEscape(pageKey)
}

func findBinding(bindings []spec.PageFunctionBinding, bindingID string) (spec.PageFunctionBinding, bool) {
	for _, binding := range bindings {
		if binding.ID == bindingID {
			return binding, true
		}
	}
	return spec.PageFunctionBinding{}, false
}

func findContract(contracts []spec.BindingContractSnapshot, bindingID string) (spec.BindingContractSnapshot, bool) {
	for _, contract := range contracts {
		if contract.BindingID == bindingID {
			return contract, true
		}
	}
	return spec.BindingContractSnapshot{}, false
}

func loadPublishedFunctionSpecs(ctx context.Context, svcCtx *svc.ServiceContext) (map[string]spec.FunctionSpec, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	if svcCtx == nil || svcCtx.DB == nil {
		return nil, errorx.NewInternalError("function contract database is not initialized")
	}
	return contractsvc.FunctionSpecsByScope(ctx, model.NewFunctionContractModel(svcCtx.DB), gameID, env)
}

func (s *Service) requireConsoleRead(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权查看运行控制台", "admin:all", "console:read", "pages:read", "function:invoke")
	return err
}

func (s *Service) requireConsoleExecute(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权执行运行控制台操作", "admin:all", "function:invoke")
	return err
}

func requireScope(ctx context.Context) (string, string, error) {
	scope := svc.GameScopeFromContext(ctx)
	gameID := strings.TrimSpace(scope.GameID)
	env := strings.TrimSpace(scope.Env)
	if gameID == "" {
		return "", "", errorx.NewBadRequest("X-Game-ID is required")
	}
	if env == "" {
		return "", "", errorx.NewBadRequest("X-Env is required")
	}
	return gameID, env, nil
}

func normalizeLanguage(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	switch lang {
	case "zh", "zh-cn", "zh_cn", "":
		return "zh-CN"
	case "en", "en-us", "en_us":
		return "en-US"
	default:
		return lang
	}
}

func getLocalizedText(labels spec.LocalizedText, lang, fallback string) string {
	if labels == nil {
		return fallback
	}
	if v, ok := labels[lang]; ok && v != "" {
		return v
	}
	if v, ok := labels["zh-CN"]; ok && v != "" {
		return v
	}
	for _, v := range labels {
		if v != "" {
			return v
		}
	}
	return fallback
}

type categoryGroup struct {
	key    string
	labels spec.LocalizedText
	order  int
	pages  []pageEntry
}

type pageEntry struct {
	key   string
	title spec.LocalizedText
	icon  string
	order int
}

func ErrPageNotFound(key string) error {
	return &PageNotFoundError{Key: key}
}

type PageNotFoundError struct {
	Key string
}

func (e *PageNotFoundError) Error() string {
	return "page not found: " + e.Key
}
