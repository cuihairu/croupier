package openapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/function/uicontract"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const maxOpenAPISourceBytes = 2 << 20

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// GetSpec returns the OpenAPI spec for a function
func (s *Service) GetSpec(ctx context.Context, req *GetSpecRequest) (*GetSpecResponse, error) {
	spec, err := s.svcCtx.RegistryStore.GetOpenAPI(req.ID)
	if err != nil {
		if hasRegisteredFunction(s.svcCtx, req.ID) {
			spec = logicfunction.BuildFallbackOpenAPIOperation(req.ID)
		} else {
			return nil, err
		}
	}
	return &GetSpecResponse{Spec: mustMarshalRaw(spec)}, nil
}

func operationUIRegistrationKey(op *openapi3.Operation) (string, bool) {
	if op == nil {
		return "", false
	}
	for key := range op.Extensions {
		if forbiddenKey, ok := uicontract.ForbiddenRegistrationKey(key); ok {
			return forbiddenKey, true
		}
	}
	return "", false
}

func (s *Service) CreateSource(ctx context.Context, req *OpenAPISourceCreateRequest) (*OpenAPISourceGetResponse, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	raw, format, err := normalizeRawSource(req.Spec)
	if err != nil {
		return nil, err
	}
	parsed, err := parseOpenAPISource(raw)
	if err != nil {
		return nil, err
	}
	if hasErrorDiagnostic(parsed.Diagnostics) {
		return nil, errorx.NewBadRequestWithDetails("OpenAPI source is invalid", map[string]any{
			"diagnostics": parsed.Diagnostics,
		})
	}
	if name == "" {
		name = firstNonEmpty(parsed.InfoTitle, "OpenAPI Source")
	}
	modelSource := &model.OpenAPISource{
		GameID:         gameID,
		Env:            env,
		SourceID:       uuid.NewString(),
		Name:           name,
		Revision:       1,
		Format:         format,
		OpenAPIVersion: parsed.OpenAPIVersion,
		InfoTitle:      parsed.InfoTitle,
		InfoVersion:    parsed.InfoVersion,
		ContentHash:    sha256Hex(raw),
	}
	modelSource.SetSpec(parsed.Spec)
	if err := modelSource.SetOperations(parsed.Operations); err != nil {
		return nil, err
	}
	if err := modelSource.SetDiagnostics(parsed.Diagnostics); err != nil {
		return nil, err
	}
	if err := s.svcCtx.OpenAPISourceModel.Create(ctx, modelSource); err != nil {
		return nil, err
	}
	return &OpenAPISourceGetResponse{Source: sourceDetailFromModel(modelSource, nil)}, nil
}

func (s *Service) CreateSourceFromMultipart(ctx context.Context, name string, file multipart.File) (*OpenAPISourceGetResponse, error) {
	if file == nil {
		return nil, errorx.NewBadRequest("file is required")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxOpenAPISourceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxOpenAPISourceBytes {
		return nil, errorx.NewBadRequest("OpenAPI source exceeds 2 MiB limit")
	}
	return s.CreateSource(ctx, &OpenAPISourceCreateRequest{
		Name: name,
		Spec: json.RawMessage(data),
	})
}

func (s *Service) ListSources(ctx context.Context, req *OpenAPISourceListRequest) (*OpenAPISourceListResponse, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.svcCtx.OpenAPISourceModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	resp := make([]OpenAPISourceSummary, 0, len(items))
	for i := range items {
		resp = append(resp, sourceSummaryFromModel(&items[i]))
	}
	return &OpenAPISourceListResponse{Items: resp}, nil
}

func (s *Service) GetSource(ctx context.Context, req *OpenAPISourceGetRequest) (*OpenAPISourceGetResponse, error) {
	source, bindings, err := s.loadSourceWithBindings(ctx, req.SourceID)
	if err != nil {
		return nil, err
	}
	return &OpenAPISourceGetResponse{Source: sourceDetailFromModel(source, bindings)}, nil
}

func (s *Service) SourceDiagnostics(ctx context.Context, req *OpenAPISourceGetRequest) (*OpenAPISourceDiagnosticsResponse, error) {
	source, _, err := s.loadSourceWithBindings(ctx, req.SourceID)
	if err != nil {
		return nil, err
	}
	var diags []spec.Diagnostic
	if err := source.GetDiagnostics(&diags); err != nil {
		return nil, err
	}
	return &OpenAPISourceDiagnosticsResponse{
		SourceID:    source.SourceID,
		Diagnostics: diags,
	}, nil
}

func (s *Service) CreateBinding(ctx context.Context, req *OpenAPISourceBindingCreateRequest) (*OpenAPISourceBindingResponse, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(req.SourceID))
	if err != nil {
		return nil, errorx.NewNotFound("OpenAPI source not found")
	}
	var operations []OpenAPISourceOperation
	if err := source.GetOperations(&operations); err != nil {
		return nil, err
	}
	if !sourceHasOperation(operations, req.OperationID) {
		return nil, errorx.NewBadRequest("operationId is not part of this OpenAPI source")
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		return nil, errorx.NewBadRequest("kind is required")
	}
	if kind != "provider" {
		return nil, errorx.NewBadRequest("only provider execution binding is enabled; httpConnector requires allowlist and SecretRef policy")
	}
	functionID := strings.TrimSpace(req.FunctionID)
	if functionID == "" {
		return nil, errorx.NewBadRequest("functionId is required for provider binding")
	}
	if !hasRegisteredFunction(s.svcCtx, functionID) {
		return nil, errorx.NewBadRequest("functionId is not registered in current runtime")
	}
	bindingID := strings.TrimSpace(req.BindingID)
	if bindingID == "" {
		bindingID = sanitizeBindingID(req.OperationID)
	}
	binding := &model.OpenAPISourceBinding{
		GameID:      gameID,
		Env:         env,
		SourceID:    source.SourceID,
		BindingID:   bindingID,
		OperationID: strings.TrimSpace(req.OperationID),
		Kind:        kind,
		FunctionID:  functionID,
		ProviderID:  strings.TrimSpace(req.ProviderID),
	}
	if err := s.svcCtx.OpenAPISourceBindingModel.Upsert(ctx, binding); err != nil {
		return nil, err
	}
	return &OpenAPISourceBindingResponse{Binding: bindingDTOFromModel(*binding)}, nil
}

func (s *Service) DeleteBinding(ctx context.Context, req *OpenAPISourceBindingDeleteRequest) (*OpenAPISourceBindingResponse, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(req.SourceID)); err != nil {
		return nil, errorx.NewNotFound("OpenAPI source not found")
	}
	if err := s.svcCtx.OpenAPISourceBindingModel.Delete(ctx, gameID, env, strings.TrimSpace(req.SourceID), strings.TrimSpace(req.BindingID)); err != nil {
		return nil, err
	}
	return &OpenAPISourceBindingResponse{Binding: OpenAPISourceBindingDTO{
		BindingID: strings.TrimSpace(req.BindingID),
	}}, nil
}

func (s *Service) loadSourceWithBindings(ctx context.Context, sourceID string) (*model.OpenAPISource, []model.OpenAPISourceBinding, error) {
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, nil, err
	}
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(sourceID))
	if err != nil {
		return nil, nil, errorx.NewNotFound("OpenAPI source not found")
	}
	bindings, err := s.svcCtx.OpenAPISourceBindingModel.ListBySource(ctx, gameID, env, source.SourceID)
	if err != nil {
		return nil, nil, err
	}
	return source, bindings, nil
}

type parsedOpenAPISource struct {
	Spec           json.RawMessage
	OpenAPIVersion string
	InfoTitle      string
	InfoVersion    string
	Operations     []OpenAPISourceOperation
	Diagnostics    []spec.Diagnostic
}

type methodOperation struct {
	method string
	path   string
	op     *openapi3.Operation
}

func normalizeRawSource(raw json.RawMessage) ([]byte, string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, "", errorx.NewBadRequest("spec is required")
	}
	if len(trimmed) > maxOpenAPISourceBytes {
		return nil, "", errorx.NewBadRequest("OpenAPI source exceeds 2 MiB limit")
	}
	if json.Valid(trimmed) {
		var compacted bytes.Buffer
		if err := json.Compact(&compacted, trimmed); err != nil {
			return nil, "", err
		}
		return compacted.Bytes(), "json", nil
	}
	var value interface{}
	if err := yaml.Unmarshal(trimmed, &value); err != nil {
		return nil, "", errorx.NewBadRequest("spec must be valid OpenAPI JSON or YAML")
	}
	data, err := json.Marshal(normalizeYAMLValue(value))
	if err != nil {
		return nil, "", err
	}
	return data, "yaml", nil
}

func normalizeYAMLValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(v))
		for key, val := range v {
			out[key] = normalizeYAMLValue(val)
		}
		return out
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(v))
		for key, val := range v {
			out[fmt.Sprint(key)] = normalizeYAMLValue(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(v))
		for _, val := range v {
			out = append(out, normalizeYAMLValue(val))
		}
		return out
	default:
		return v
	}
}

func parseOpenAPISource(raw []byte) (*parsedOpenAPISource, error) {
	parsed := &parsedOpenAPISource{Spec: json.RawMessage(raw)}
	parsed.Diagnostics = append(parsed.Diagnostics, scanOpenAPISourceRaw(raw)...)
	if hasErrorDiagnostic(parsed.Diagnostics) {
		return parsed, nil
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(raw)
	if err != nil {
		parsed.Diagnostics = append(parsed.Diagnostics, sourceDiagnostic(
			"openapi_parse_failed",
			spec.SeverityError,
			err.Error(),
			"$",
		))
		return parsed, nil
	}
	normalizeOpenAPIDoc(doc)
	if err := doc.Validate(loader.Context); err != nil {
		parsed.Diagnostics = append(parsed.Diagnostics, sourceDiagnostic(
			"openapi_validation_failed",
			spec.SeverityError,
			err.Error(),
			"$",
		))
		return parsed, nil
	}
	parsed.OpenAPIVersion = strings.TrimSpace(doc.OpenAPI)
	if !strings.HasPrefix(parsed.OpenAPIVersion, "3.") {
		parsed.Diagnostics = append(parsed.Diagnostics, sourceDiagnostic(
			"openapi_version_unsupported",
			spec.SeverityError,
			"only OpenAPI 3.x sources are supported",
			"$.openapi",
		))
		return parsed, nil
	}
	if doc.Info != nil {
		parsed.InfoTitle = strings.TrimSpace(doc.Info.Title)
		parsed.InfoVersion = strings.TrimSpace(doc.Info.Version)
	}
	parsed.Operations, parsed.Diagnostics = extractSourceOperations(doc, parsed.Diagnostics)
	if len(parsed.Operations) == 0 && !hasErrorDiagnostic(parsed.Diagnostics) {
		parsed.Diagnostics = append(parsed.Diagnostics, sourceDiagnostic(
			"openapi_no_operations",
			spec.SeverityWarning,
			"OpenAPI source contains no operations",
			"$.paths",
		))
	}
	return parsed, nil
}

func extractSourceOperations(doc *openapi3.T, diags []spec.Diagnostic) ([]OpenAPISourceOperation, []spec.Diagnostic) {
	if doc == nil || doc.Paths == nil {
		return nil, append(diags, sourceDiagnostic("openapi_paths_missing", spec.SeverityError, "paths is required", "$.paths"))
	}
	seen := map[string]string{}
	items := make([]OpenAPISourceOperation, 0)
	for _, candidate := range openAPIMethodOperations(doc.Paths) {
		if candidate.op == nil {
			continue
		}
		fieldPath := fmt.Sprintf("$.paths.%s.%s", candidate.path, strings.ToLower(candidate.method))
		if forbiddenKey, ok := operationUIRegistrationKey(candidate.op); ok {
			diags = append(diags, sourceDiagnostic(
				"openapi_ui_field_forbidden",
				spec.SeverityError,
				fmt.Sprintf("UI field %q is not allowed in OpenAPI source", forbiddenKey),
				fieldPath,
			))
			continue
		}
		operationID := strings.TrimSpace(candidate.op.OperationID)
		if operationID == "" {
			diags = append(diags, sourceDiagnostic(
				"openapi_operation_id_missing",
				spec.SeverityError,
				"operationId is required for every OpenAPI source operation",
				fieldPath+".operationId",
			))
			continue
		}
		if existing, ok := seen[operationID]; ok {
			diags = append(diags, sourceDiagnostic(
				"openapi_operation_id_duplicate",
				spec.SeverityError,
				fmt.Sprintf("operationId %q duplicates %s", operationID, existing),
				fieldPath+".operationId",
			))
			continue
		}
		seen[operationID] = fieldPath
		items = append(items, operationDTOFromOpenAPI(candidate, operationID, &diags))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		if items[i].Method != items[j].Method {
			return items[i].Method < items[j].Method
		}
		return items[i].OperationID < items[j].OperationID
	})
	return items, diags
}

func openAPIMethodOperations(paths *openapi3.Paths) []methodOperation {
	out := make([]methodOperation, 0)
	for path, pathItem := range paths.Map() {
		if pathItem == nil {
			continue
		}
		out = append(out,
			methodOperation{method: "GET", path: path, op: pathItem.Get},
			methodOperation{method: "POST", path: path, op: pathItem.Post},
			methodOperation{method: "PUT", path: path, op: pathItem.Put},
			methodOperation{method: "PATCH", path: path, op: pathItem.Patch},
			methodOperation{method: "DELETE", path: path, op: pathItem.Delete},
			methodOperation{method: "OPTIONS", path: path, op: pathItem.Options},
			methodOperation{method: "HEAD", path: path, op: pathItem.Head},
			methodOperation{method: "TRACE", path: path, op: pathItem.Trace},
		)
	}
	return out
}

func operationDTOFromOpenAPI(candidate methodOperation, operationID string, diags *[]spec.Diagnostic) OpenAPISourceOperation {
	extensions := candidate.op.Extensions
	kind := spec.OperationKind(extensionString(extensions, "x-operation-kind"))
	if kind != "" && !isValidOperationKind(kind) {
		*diags = append(*diags, sourceDiagnostic(
			"openapi_operation_kind_invalid",
			spec.SeverityWarning,
			"invalid x-operation-kind ignored",
			fmt.Sprintf("$.paths.%s.%s.x-operation-kind", candidate.path, strings.ToLower(candidate.method)),
		))
		kind = ""
	}
	placement := spec.OperationPlacement(extensionString(extensions, "x-placement"))
	if placement != "" && !isValidOperationPlacement(placement) {
		*diags = append(*diags, sourceDiagnostic(
			"openapi_placement_invalid",
			spec.SeverityWarning,
			"invalid x-placement ignored",
			fmt.Sprintf("$.paths.%s.%s.x-placement", candidate.path, strings.ToLower(candidate.method)),
		))
		placement = ""
	}
	risk := spec.RiskLevel(extensionString(extensions, "x-risk"))
	if risk != "" && !isValidRisk(risk) {
		*diags = append(*diags, sourceDiagnostic(
			"openapi_risk_invalid",
			spec.SeverityWarning,
			"invalid x-risk ignored",
			fmt.Sprintf("$.paths.%s.%s.x-risk", candidate.path, strings.ToLower(candidate.method)),
		))
		risk = ""
	}
	return OpenAPISourceOperation{
		OperationID:      operationID,
		Method:           candidate.method,
		Path:             candidate.path,
		Summary:          strings.TrimSpace(candidate.op.Summary),
		Description:      strings.TrimSpace(candidate.op.Description),
		Tags:             append([]string(nil), candidate.op.Tags...),
		Category:         extensionString(extensions, "x-category"),
		CategoryDisplay:  extensionLocalized(extensions, "x-category-display"),
		Entity:           extensionString(extensions, "x-entity"),
		EntityDisplay:    extensionLocalized(extensions, "x-entity-display"),
		Operation:        extensionString(extensions, "x-operation"),
		OperationDisplay: extensionLocalized(extensions, "x-operation-display"),
		OperationKind:    kind,
		Placement:        placement,
		PageHint:         extensionString(extensions, "x-page-hint"),
		PageContract:     extensionPageContract(extensions, "x-page-contract"),
		Risk:             risk,
	}
}

func scanOpenAPISourceRaw(raw []byte) []spec.Diagnostic {
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return []spec.Diagnostic{sourceDiagnostic("openapi_json_decode_failed", spec.SeverityError, err.Error(), "$")}
	}
	var diags []spec.Diagnostic
	scanOpenAPIValue(value, "$", &diags)
	return diags
}

func scanOpenAPIValue(value interface{}, path string, diags *[]spec.Diagnostic) {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, child := range v {
			childPath := path + "." + key
			if key == "$ref" {
				ref, _ := child.(string)
				if strings.TrimSpace(ref) != "" && !strings.HasPrefix(strings.TrimSpace(ref), "#") {
					*diags = append(*diags, sourceDiagnostic(
						"openapi_external_ref_forbidden",
						spec.SeverityError,
						"external $ref is not allowed in uploaded OpenAPI sources",
						childPath,
					))
				}
			}
			if forbiddenOpenAPIUIKey(key) {
				*diags = append(*diags, sourceDiagnostic(
					"openapi_ui_field_forbidden",
					spec.SeverityError,
					fmt.Sprintf("UI field %q is not allowed in OpenAPI source", key),
					childPath,
				))
			}
			scanOpenAPIValue(child, childPath, diags)
		}
	case []interface{}:
		for i, child := range v {
			scanOpenAPIValue(child, fmt.Sprintf("%s[%d]", path, i), diags)
		}
	}
}

func forbiddenOpenAPIUIKey(key string) bool {
	normalized := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(key, "_", "-")))
	if _, ok := uicontract.ForbiddenRegistrationKey(normalized); ok {
		return ok
	}
	switch normalized {
	case "x-menu", "x-route", "x-routes", "x-page", "x-page-schema", "x-table-columns", "x-columns":
		return true
	default:
		return false
	}
}

func hasErrorDiagnostic(diags []spec.Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity == spec.SeverityError {
			return true
		}
	}
	return false
}

func sourceSummaryFromModel(source *model.OpenAPISource) OpenAPISourceSummary {
	var operations []OpenAPISourceOperation
	_ = source.GetOperations(&operations)
	var diags []spec.Diagnostic
	_ = source.GetDiagnostics(&diags)
	return OpenAPISourceSummary{
		SourceID:        source.SourceID,
		GameID:          source.GameID,
		Env:             source.Env,
		Name:            source.Name,
		Revision:        source.Revision,
		Format:          source.Format,
		OpenAPIVersion:  source.OpenAPIVersion,
		InfoTitle:       source.InfoTitle,
		InfoVersion:     source.InfoVersion,
		ContentHash:     source.ContentHash,
		OperationCount:  len(operations),
		DiagnosticCount: len(diags),
		CreatedAt:       formatTime(source.CreatedAt),
		UpdatedAt:       formatTime(source.UpdatedAt),
		Diagnostics:     diags,
	}
}

func sourceDetailFromModel(source *model.OpenAPISource, bindings []model.OpenAPISourceBinding) OpenAPISourceDetail {
	var operations []OpenAPISourceOperation
	_ = source.GetOperations(&operations)
	bindingDTOs := make([]OpenAPISourceBindingDTO, 0, len(bindings))
	bindingByOperation := make(map[string]OpenAPISourceBindingDTO, len(bindings))
	for _, binding := range bindings {
		dto := bindingDTOFromModel(binding)
		bindingDTOs = append(bindingDTOs, dto)
		bindingByOperation[dto.OperationID] = dto
	}
	for i := range operations {
		if binding, ok := bindingByOperation[operations[i].OperationID]; ok {
			operations[i].Bound = true
			operations[i].BindingID = binding.BindingID
			operations[i].FunctionID = binding.FunctionID
		}
	}
	return OpenAPISourceDetail{
		OpenAPISourceSummary: sourceSummaryFromModel(source),
		Spec:                 source.GetSpec(),
		Operations:           operations,
		Bindings:             bindingDTOs,
	}
}

func bindingDTOFromModel(binding model.OpenAPISourceBinding) OpenAPISourceBindingDTO {
	return OpenAPISourceBindingDTO{
		BindingID:   binding.BindingID,
		OperationID: binding.OperationID,
		Kind:        binding.Kind,
		FunctionID:  binding.FunctionID,
		ProviderID:  binding.ProviderID,
		CreatedAt:   formatTime(binding.CreatedAt),
		UpdatedAt:   formatTime(binding.UpdatedAt),
	}
}

func sourceHasOperation(operations []OpenAPISourceOperation, operationID string) bool {
	operationID = strings.TrimSpace(operationID)
	for _, operation := range operations {
		if operation.OperationID == operationID {
			return true
		}
	}
	return false
}

func sanitizeBindingID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.NewString()
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return uuid.NewString()
	}
	return out
}

func extensionString(extensions map[string]interface{}, key string) string {
	if extensions == nil {
		return ""
	}
	value, ok := extensions[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func extensionLocalized(extensions map[string]interface{}, key string) spec.LocalizedText {
	if extensions == nil {
		return nil
	}
	value, ok := extensions[key]
	if !ok {
		return nil
	}
	out := spec.LocalizedText{}
	switch typed := value.(type) {
	case map[string]string:
		for locale, text := range typed {
			if strings.TrimSpace(locale) != "" && strings.TrimSpace(text) != "" {
				out[strings.TrimSpace(locale)] = strings.TrimSpace(text)
			}
		}
	case map[string]interface{}:
		for locale, rawText := range typed {
			text, ok := rawText.(string)
			if ok && strings.TrimSpace(locale) != "" && strings.TrimSpace(text) != "" {
				out[strings.TrimSpace(locale)] = strings.TrimSpace(text)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func extensionPageContract(extensions map[string]interface{}, key string) *spec.PageContract {
	if extensions == nil {
		return nil
	}
	value, ok := extensions[key]
	if !ok {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var contract spec.PageContract
	if err := json.Unmarshal(data, &contract); err != nil {
		return nil
	}
	if strings.TrimSpace(contract.Version) == "" {
		return nil
	}
	return &contract
}

func isValidOperationKind(kind spec.OperationKind) bool {
	switch kind {
	case spec.OperationKindList, spec.OperationKindGet, spec.OperationKindCreate, spec.OperationKindUpdate,
		spec.OperationKindDelete, spec.OperationKindAction, spec.OperationKindTask, spec.OperationKindReport:
		return true
	default:
		return false
	}
}

func isValidOperationPlacement(placement spec.OperationPlacement) bool {
	switch placement {
	case spec.PlacementQuery, spec.PlacementTableData, spec.PlacementDetailData, spec.PlacementRowAction,
		spec.PlacementDetailAction, spec.PlacementToolbarAction, spec.PlacementBatchAction, spec.PlacementStandalone:
		return true
	default:
		return false
	}
}

func isValidRisk(risk spec.RiskLevel) bool {
	switch risk {
	case spec.RiskSafe, spec.RiskWarning, spec.RiskHigh, spec.RiskDanger:
		return true
	default:
		return false
	}
}

func sourceDiagnostic(code string, severity spec.DiagnosticSeverity, message, field string) spec.Diagnostic {
	return spec.Diagnostic{
		Code:     code,
		Severity: severity,
		Message:  message,
		Field:    field,
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func mustMarshalRaw(value interface{}) json.RawMessage {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return json.RawMessage(data)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
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

// GetDocument returns aggregated OpenAPI document
func (s *Service) GetDocument(ctx context.Context, req *GetDocumentRequest) (*GetDocumentResponse, error) {
	spec, err := s.svcCtx.RegistryStore.BuildOpenAPISpec()
	if err != nil {
		return nil, err
	}
	return &GetDocumentResponse{
		Spec: mustMarshalRaw(spec),
	}, nil
}

func (s *Service) BatchGetSpec(ctx context.Context, req *BatchGetSpecRequest) (BatchGetSpecResponse, error) {
	resp := make(BatchGetSpecResponse, len(req.FunctionIDs))
	for _, id := range req.FunctionIDs {
		functionID := strings.TrimSpace(id)
		if functionID == "" {
			continue
		}
		spec, err := s.svcCtx.RegistryStore.GetOpenAPI(functionID)
		if err != nil {
			if hasRegisteredFunction(s.svcCtx, functionID) {
				resp[functionID] = mustMarshalRaw(logicfunction.BuildFallbackOpenAPIOperation(functionID))
				continue
			}
			resp[functionID] = nil
			continue
		}
		resp[functionID] = mustMarshalRaw(spec)
	}
	return resp, nil
}

func hasRegisteredFunction(svcCtx *svc.ServiceContext, functionID string) bool {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return false
	}
	if svcCtx == nil {
		return false
	}
	if svcCtx.RegistryStore != nil {
		svcCtx.RegistryStore.Mu().RLock()
		defer svcCtx.RegistryStore.Mu().RUnlock()
		for _, sess := range svcCtx.RegistryStore.AgentsUnsafe() {
			if sess == nil {
				continue
			}
			if _, ok := sess.Functions[functionID]; ok {
				return true
			}
		}
	}
	if svcCtx.FunctionModel != nil {
		if _, err := svcCtx.FunctionModel.FindByFunctionID(context.Background(), functionID); err == nil {
			return true
		}
	}
	return false
}

// normalizeOpenAPIDoc patches common non-critical gaps from external OpenAPI docs
// so import stays resilient (e.g. response description omitted by third-party generators).
func normalizeOpenAPIDoc(doc *openapi3.T) {
	if doc == nil || doc.Paths == nil {
		return
	}
	for _, pathItem := range doc.Paths.Map() {
		if pathItem == nil {
			continue
		}
		operations := []*openapi3.Operation{
			pathItem.Get,
			pathItem.Post,
			pathItem.Put,
			pathItem.Patch,
			pathItem.Delete,
			pathItem.Options,
			pathItem.Head,
			pathItem.Trace,
		}
		for _, op := range operations {
			if op == nil || op.Responses == nil {
				continue
			}
			for statusCode, responseRef := range op.Responses.Map() {
				if responseRef == nil {
					continue
				}
				if responseRef.Value == nil {
					responseRef.Value = &openapi3.Response{}
				}
				if responseRef.Value.Description == nil || strings.TrimSpace(*responseRef.Value.Description) == "" {
					desc := fmt.Sprintf("Auto-generated response description for status %s", statusCode)
					responseRef.Value.Description = &desc
				}
			}
		}
	}
}
