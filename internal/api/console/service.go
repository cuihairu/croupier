package console

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/api/function"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/descriptors"
	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
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
	pages := s.attachBindingFreshness(ctx, parsePublishedPages(publishedPages))
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
	s.attachBindingFreshnessToPage(ctx, pageSpec)
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
	functions := normalizedFunctions(ctx, s.svcCtx)
	requestID := uuid.NewString()
	actor := currentActor(ctx)
	var result spec.PageExecutionResult
	var target string
	ctx, finishSpan := s.startPageExecuteSpan(ctx, gameID, env, *published, binding, requestID, actor)
	defer func() {
		finishSpan(err, result, target)
		s.auditPageExecute(ctx, gameID, env, *published, binding, requestID, target, result, err)
	}()

	if err := s.ensureBindingFresh(binding, contract, functions); err != nil {
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

	functionResp, err := function.NewService(s.svcCtx).FunctionInvoke(ctx, &function.FunctionInvokeRequest{
		ID:      binding.FunctionID,
		Payload: payload,
		GameID:  gameID,
		Env:     env,
		Mode:    mode,
		Metadata: map[string]string{
			"page_key":         published.PageKey,
			"page_category":    published.Category.Key,
			"publish_version":  strconv.Itoa(published.Version),
			"binding_id":       binding.ID,
			"binding_usage":    string(binding.Usage),
			"page_request_id":  requestID,
			"page_runtime_api": "console.binding.execute",
		},
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
	payload := map[string]any{}
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

func resolveSelectorValue(source spec.ValueSource, execCtx ConsoleBindingExecutionContext) (any, bool, error) {
	if source.Transform != nil {
		return nil, false, errorx.NewValidationError("binding selector transforms are not supported by console execution")
	}
	switch source.Kind {
	case spec.SourceLiteral:
		if len(source.Value) == 0 {
			return nil, true, nil
		}
		value, err := decodeRawJSONValue(source.Value, "literal")
		return value, err == nil, err
	case spec.SourceForm:
		return valueFromRawContext(execCtx.Form, source.Path, "form")
	case spec.SourceRow:
		return valueFromRawContext(execCtx.Row, source.Path, "row")
	case spec.SourceSelection:
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

func valueFromRawContext(raw json.RawMessage, path string, sourceName string) (any, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	value, err := decodeRawJSONValue(raw, sourceName)
	if err != nil {
		return nil, false, err
	}
	if path == "" {
		return value, true, nil
	}
	selected, ok := getJSONPointerValue(value, path)
	return selected, ok, nil
}

func decodeRawJSONValue(raw json.RawMessage, sourceName string) (any, error) {
	if !json.Valid(raw) {
		return nil, errorx.NewBadRequest(sourceName + " context must be valid JSON")
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, errorx.NewBadRequest(sourceName + " context must be valid JSON")
	}
	return value, nil
}

func getJSONPointerValue(value any, path string) (any, bool) {
	if !isJSONPointer(path) {
		return nil, false
	}
	current := value
	for _, token := range jsonPointerTokens(path) {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[token]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func setJSONPointerValue(payload map[string]any, path string, value any) error {
	if !isJSONPointer(path) || path == "" {
		return errorx.NewValidationError("target must be a non-empty JSON Pointer")
	}
	current := payload
	tokens := jsonPointerTokens(path)
	for i, token := range tokens {
		if token == "" {
			return errorx.NewValidationError("target contains an empty object key")
		}
		if i == len(tokens)-1 {
			current[token] = value
			return nil
		}
		next, ok := current[token]
		if !ok {
			child := map[string]any{}
			current[token] = child
			current = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return errorx.NewValidationError("target conflicts with a non-object parent")
		}
		current = child
	}
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
		attribute.String("page.binding_id", binding.ID),
		attribute.String("function.id", binding.FunctionID),
		attribute.String("page.binding_usage", string(binding.Usage)),
		attribute.String("page.execution_mode", string(binding.Execution.Mode)),
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
		"request_id":       requestID,
		"trace_id":         traceID,
		"game_id":          gameID,
		"env":              env,
		"page_key":         page.PageKey,
		"publish_version":  page.Version,
		"binding_id":       binding.ID,
		"binding_usage":    string(binding.Usage),
		"function_id":      binding.FunctionID,
		"target":           target,
		"execution_mode":   string(binding.Execution.Mode),
		"result_kind":      string(result.Kind),
		"task_id":          result.TaskID,
		"approval_id":      result.ApprovalID,
		"diagnostic_count": len(result.Diagnostics),
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
		BindingContracts:      contracts,
	}
}

func (s *Service) attachBindingFreshness(ctx context.Context, pages []spec.PublishedPageSpec) []spec.PublishedPageSpec {
	if len(pages) == 0 {
		return pages
	}
	functions := normalizedFunctions(ctx, s.svcCtx)
	for i := range pages {
		pages[i].BindingFreshness = freshness.EvaluatePublishedBindings(pages[i].Bindings, pages[i].BindingContracts, functions)
	}
	return pages
}

func (s *Service) attachBindingFreshnessToPage(ctx context.Context, page *spec.PublishedPageSpec) {
	if page == nil {
		return
	}
	page.BindingFreshness = freshness.EvaluatePublishedBindings(page.Bindings, page.BindingContracts, normalizedFunctions(ctx, s.svcCtx))
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
				order:  page.Category.Order,
			}
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
				Path:   "/console/" + cat.key + "/" + p.key,
				Title:  p.title,
				Locale: false,
				Icon:   p.icon,
				Order:  p.order,
			})
		}
		items = append(items, spec.ConsoleMenuItem{
			Key:      cat.key,
			Path:     "/console/" + cat.key,
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

func normalizedFunctions(ctx context.Context, svcCtx *svc.ServiceContext) map[string]spec.FunctionSpec {
	inputs := descriptors.Collect(ctx, svcCtx)
	out := make(map[string]spec.FunctionSpec, len(inputs))
	for _, input := range inputs {
		result := normalizer.Normalize(input)
		if result.Function.ID != "" {
			out[result.Function.ID] = result.Function
		}
	}
	return out
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
	gameID, env := svc.GameScopeFromContext(ctx)
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
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
