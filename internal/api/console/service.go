package console

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/api/function"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/descriptors"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/google/uuid"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

func (s *Service) Menu(ctx context.Context, req *ConsoleMenuRequest) (*ConsoleMenuResponse, error) {
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
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	publishedPages, err := s.svcCtx.PublishedPageSpecModel.ListLatestActiveByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	items := make([]spec.PublishedPageSpec, 0, len(publishedPages))
	for _, page := range parsePublishedPages(publishedPages) {
		if req.Category != "" && page.Category.Key != req.Category {
			continue
		}
		items = append(items, page)
	}
	return &ConsolePagesResponse{Items: items}, nil
}

func (s *Service) Page(ctx context.Context, req *ConsolePageRequest) (*ConsolePageResponse, error) {
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
	return &ConsolePageResponse{Page: *pageSpec}, nil
}

func (s *Service) ExecuteBinding(ctx context.Context, req *ConsoleExecuteBindingRequest) (*ConsoleExecuteBindingResponse, error) {
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
	if err := s.ensureBindingFresh(ctx, binding, contract); err != nil {
		return nil, err
	}

	var payload any = map[string]any{}
	if len(req.Payload) > 0 {
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			return nil, errorx.NewBadRequest("payload must be valid JSON")
		}
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
	})
	if err != nil {
		return nil, err
	}

	result, err := buildExecutionResult(ctx, functionResp)
	if err != nil {
		return nil, err
	}
	return &ConsoleExecuteBindingResponse{Result: result}, nil
}

func (s *Service) ensureBindingFresh(ctx context.Context, binding spec.PageFunctionBinding, contract spec.BindingContractSnapshot) error {
	functions := normalizedFunctions(ctx, s.svcCtx)
	fn, ok := functions[binding.FunctionID]
	if !ok {
		return errorx.NewValidationError("binding_stale: function no longer exists")
	}
	stale := strings.TrimSpace(fn.Version) != strings.TrimSpace(contract.FunctionVersion) ||
		digestRaw(fn.InputSchema) != contract.InputSchemaDigest ||
		digestRaw(fn.OutputSchema) != contract.OutputSchemaDigest ||
		binding.Execution.Mode != contract.ExecutionMode
	if stale {
		return errorx.NewConflictWithDetails("binding_stale", map[string]any{
			"bindingId":  binding.ID,
			"functionId": binding.FunctionID,
		})
	}
	return nil
}

func buildExecutionResult(ctx context.Context, resp *function.FunctionInvokeResponse) (spec.PageExecutionResult, error) {
	requestID := uuid.NewString()
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
	data, err := json.Marshal(resp.Result)
	if err != nil {
		return spec.PageExecutionResult{}, fmt.Errorf("encode page execution result: %w", err)
	}
	return spec.PageExecutionResult{
		Kind:      spec.PageExecutionKindSync,
		RequestID: requestID,
		TraceID:   traceID,
		Data:      data,
	}, nil
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

func digestRaw(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
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
