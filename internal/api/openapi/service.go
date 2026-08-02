package openapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	logicfunction "github.com/cuihairu/croupier/internal/logic/function"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
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

func operationPresentationField(op *openapi3.Operation) (string, bool) {
	if op == nil {
		return "", false
	}
	for key := range op.Extensions {
		if forbiddenKey, ok := registrationguard.ForbiddenPresentationField(key); ok {
			return forbiddenKey, true
		}
	}
	return "", false
}

func (s *Service) CreateSource(ctx context.Context, req *OpenAPISourceCreateRequest) (*OpenAPISourceGetResponse, error) {
	if err := s.requireSourceWrite(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	ctx, finishSpan := s.startSourceSpan(ctx, "openapi.source.create", gameID, env,
		attribute.String("openapi_source.name", strings.TrimSpace(req.Name)),
	)
	var spanErr error
	defer func() { finishSpan(spanErr) }()
	name := strings.TrimSpace(req.Name)
	parsed, format, err := s.parseValidSource(req.Spec)
	if err != nil {
		spanErr = err
		return nil, err
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
		ContentHash:    sha256Hex(parsed.Spec),
	}
	modelSource.SetSpec(parsed.Spec)
	if err := modelSource.SetOperations(parsed.Operations); err != nil {
		spanErr = err
		return nil, err
	}
	if err := modelSource.SetDiagnostics(parsed.Diagnostics); err != nil {
		spanErr = err
		return nil, err
	}
	if err := s.svcCtx.OpenAPISourceModel.Create(ctx, modelSource); err != nil {
		spanErr = err
		return nil, err
	}
	finishSpan(nil,
		attribute.String("openapi_source.id", modelSource.SourceID),
		attribute.Int("openapi_source.revision", modelSource.Revision),
		attribute.String("openapi_source.format", modelSource.Format),
		attribute.Int("openapi_source.operation_count", len(parsed.Operations)),
		attribute.Int("openapi_source.diagnostic_count", len(parsed.Diagnostics)),
	)
	finishSpan = func(error, ...attribute.KeyValue) {}
	s.auditSourceEvent(ctx, audit.EventOpenAPISourceCreate, gameID, env, modelSource.SourceID, modelSource.Name, map[string]interface{}{
		"revision":         modelSource.Revision,
		"format":           modelSource.Format,
		"openapi_version":  modelSource.OpenAPIVersion,
		"operation_count":  len(parsed.Operations),
		"diagnostic_count": len(parsed.Diagnostics),
		"content_hash":     modelSource.ContentHash,
	})
	return &OpenAPISourceGetResponse{Source: sourceDetailFromModel(modelSource, nil)}, nil
}

func (s *Service) UpdateSource(ctx context.Context, req *OpenAPISourceUpdateRequest) (*OpenAPISourceGetResponse, error) {
	if err := s.requireSourceWrite(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(req.SourceID))
	if err != nil {
		return nil, errorx.NewNotFound("OpenAPI source not found")
	}
	ctx, finishSpan := s.startSourceSpan(ctx, "openapi.source.update", gameID, env,
		attribute.String("openapi_source.id", source.SourceID),
		attribute.Int("openapi_source.previous_revision", source.Revision),
	)
	var spanErr error
	defer func() { finishSpan(spanErr) }()
	parsed, format, err := s.parseValidSource(req.Spec)
	if err != nil {
		spanErr = err
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = firstNonEmpty(parsed.InfoTitle, source.Name, "OpenAPI Source")
	}
	previousRevision := source.Revision
	source.Name = name
	source.Revision++
	source.Format = format
	source.OpenAPIVersion = parsed.OpenAPIVersion
	source.InfoTitle = parsed.InfoTitle
	source.InfoVersion = parsed.InfoVersion
	source.ContentHash = sha256Hex(parsed.Spec)
	source.SetSpec(parsed.Spec)
	if err := source.SetOperations(parsed.Operations); err != nil {
		spanErr = err
		return nil, err
	}
	if err := source.SetDiagnostics(parsed.Diagnostics); err != nil {
		spanErr = err
		return nil, err
	}
	if err := s.svcCtx.OpenAPISourceModel.Update(ctx, source); err != nil {
		spanErr = err
		return nil, err
	}
	bindings, err := s.svcCtx.OpenAPISourceBindingModel.ListBySource(ctx, gameID, env, source.SourceID)
	if err != nil {
		spanErr = err
		return nil, err
	}
	finishSpan(nil,
		attribute.Int("openapi_source.revision", source.Revision),
		attribute.String("openapi_source.format", source.Format),
		attribute.Int("openapi_source.operation_count", len(parsed.Operations)),
		attribute.Int("openapi_source.diagnostic_count", len(parsed.Diagnostics)),
	)
	finishSpan = func(error, ...attribute.KeyValue) {}
	s.auditSourceEvent(ctx, audit.EventOpenAPISourceUpdate, gameID, env, source.SourceID, source.Name, map[string]interface{}{
		"previous_revision": previousRevision,
		"revision":          source.Revision,
		"format":            source.Format,
		"openapi_version":   source.OpenAPIVersion,
		"operation_count":   len(parsed.Operations),
		"diagnostic_count":  len(parsed.Diagnostics),
		"content_hash":      source.ContentHash,
	})
	return &OpenAPISourceGetResponse{Source: sourceDetailFromModel(source, bindings)}, nil
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
	if err := s.requireSourceRead(ctx); err != nil {
		return nil, err
	}
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
	if err := s.requireSourceRead(ctx); err != nil {
		return nil, err
	}
	source, bindings, err := s.loadSourceWithBindings(ctx, req.SourceID)
	if err != nil {
		return nil, err
	}
	return &OpenAPISourceGetResponse{Source: sourceDetailFromModel(source, bindings)}, nil
}

func (s *Service) SourceDiagnostics(ctx context.Context, req *OpenAPISourceGetRequest) (*OpenAPISourceDiagnosticsResponse, error) {
	if err := s.requireSourceRead(ctx); err != nil {
		return nil, err
	}
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
	if err := s.requireSourceWrite(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	ctx, finishSpan := s.startSourceSpan(ctx, "openapi.source.binding.create", gameID, env,
		attribute.String("openapi_source.id", strings.TrimSpace(req.SourceID)),
		attribute.String("openapi_source.operation_id", strings.TrimSpace(req.OperationID)),
		attribute.String("openapi_source.binding_id", strings.TrimSpace(req.BindingID)),
		attribute.String("function.id", strings.TrimSpace(req.FunctionID)),
		attribute.String("provider.id", strings.TrimSpace(req.ProviderID)),
	)
	var spanErr error
	defer func() { finishSpan(spanErr) }()
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(req.SourceID))
	if err != nil {
		spanErr = err
		return nil, errorx.NewNotFound("OpenAPI source not found")
	}
	var operations []OpenAPISourceOperation
	if err := source.GetOperations(&operations); err != nil {
		spanErr = err
		return nil, err
	}
	if !sourceHasOperation(operations, req.OperationID) {
		err := errorx.NewBadRequest("operationId is not part of this OpenAPI source")
		spanErr = err
		return nil, err
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		err := errorx.NewBadRequest("kind is required")
		spanErr = err
		return nil, err
	}
	if kind != "provider" {
		err := errorx.NewBadRequest("only provider execution binding is enabled; httpConnector requires allowlist and SecretRef policy")
		spanErr = err
		return nil, err
	}
	functionID := strings.TrimSpace(req.FunctionID)
	if functionID == "" {
		err := errorx.NewBadRequest("functionId is required for provider binding")
		spanErr = err
		return nil, err
	}
	if !hasRegisteredFunction(s.svcCtx, functionID) {
		err := errorx.NewBadRequest("functionId is not registered in current runtime")
		spanErr = err
		return nil, err
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
		spanErr = err
		return nil, err
	}
	finishSpan(nil,
		attribute.String("openapi_source.id", source.SourceID),
		attribute.Int("openapi_source.revision", source.Revision),
		attribute.String("openapi_source.binding_id", binding.BindingID),
	)
	finishSpan = func(error, ...attribute.KeyValue) {}
	s.auditSourceEvent(ctx, audit.EventOpenAPISourceBindingCreate, gameID, env, source.SourceID, source.Name, map[string]interface{}{
		"binding_id":   binding.BindingID,
		"operation_id": binding.OperationID,
		"kind":         binding.Kind,
		"function_id":  binding.FunctionID,
		"provider_id":  binding.ProviderID,
		"revision":     source.Revision,
	})
	return &OpenAPISourceBindingResponse{Binding: bindingDTOFromModel(*binding)}, nil
}

func (s *Service) DeleteBinding(ctx context.Context, req *OpenAPISourceBindingDeleteRequest) (*OpenAPISourceBindingResponse, error) {
	if err := s.requireSourceWrite(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	ctx, finishSpan := s.startSourceSpan(ctx, "openapi.source.binding.delete", gameID, env,
		attribute.String("openapi_source.id", strings.TrimSpace(req.SourceID)),
		attribute.String("openapi_source.binding_id", strings.TrimSpace(req.BindingID)),
	)
	var spanErr error
	defer func() { finishSpan(spanErr) }()
	source, err := s.svcCtx.OpenAPISourceModel.FindByScopeAndSourceID(ctx, gameID, env, strings.TrimSpace(req.SourceID))
	if err != nil {
		spanErr = err
		return nil, errorx.NewNotFound("OpenAPI source not found")
	}
	if err := s.svcCtx.OpenAPISourceBindingModel.Delete(ctx, gameID, env, strings.TrimSpace(req.SourceID), strings.TrimSpace(req.BindingID)); err != nil {
		spanErr = err
		return nil, err
	}
	finishSpan(nil, attribute.Int("openapi_source.revision", source.Revision))
	finishSpan = func(error, ...attribute.KeyValue) {}
	s.auditSourceEvent(ctx, audit.EventOpenAPISourceBindingDelete, gameID, env, source.SourceID, source.Name, map[string]interface{}{
		"binding_id": strings.TrimSpace(req.BindingID),
		"revision":   source.Revision,
	})
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

func (s *Service) parseValidSource(spec json.RawMessage) (*parsedOpenAPISource, string, error) {
	raw, format, err := normalizeRawSource(spec)
	if err != nil {
		return nil, "", err
	}
	parsed, err := parseOpenAPISource(raw)
	if err != nil {
		return nil, "", err
	}
	if hasErrorDiagnostic(parsed.Diagnostics) {
		return nil, "", errorx.NewBadRequestWithDetails("OpenAPI source is invalid", map[string]any{
			"diagnostics": parsed.Diagnostics,
		})
	}
	return parsed, format, nil
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
		if forbiddenKey, ok := operationPresentationField(candidate.op); ok {
			diags = append(diags, sourceDiagnostic(
				"openapi_presentation_field_forbidden",
				spec.SeverityError,
				fmt.Sprintf("presentation field %q is not allowed in OpenAPI source", forbiddenKey),
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
	capability := spec.CapabilityKind(extensionString(extensions, "x-capability"))
	if capability != "" && !spec.IsValidCapabilityKind(capability) {
		*diags = append(*diags, sourceDiagnostic(
			"openapi_capability_invalid",
			spec.SeverityError,
			"x-capability must be one of collection_query, item_query, create, update, delete, action, task, report",
			fmt.Sprintf("$.paths.%s.%s.x-capability", candidate.path, strings.ToLower(candidate.method)),
		))
		capability = ""
	}

	// If capability is not provided, try to infer from REST method/path
	if capability == "" {
		inferred := classifyRESTCapability(candidate.method, candidate.path)
		if inferred != "" {
			capability = inferred
			*diags = append(*diags, restClassificationDiagnostic(candidate.method, candidate.path, inferred))
		}
	}

	execution := spec.FunctionExecution(extensionString(extensions, "x-execution"))
	if execution != "" && !spec.IsValidFunctionExecution(execution) {
		*diags = append(*diags, sourceDiagnostic(
			"openapi_execution_invalid",
			spec.SeverityError,
			"x-execution must be one of sync, task",
			fmt.Sprintf("$.paths.%s.%s.x-execution", candidate.path, strings.ToLower(candidate.method)),
		))
		execution = ""
	}
	approval := approvalPolicyFromExtensions(extensions, candidate, diags)
	return OpenAPISourceOperation{
		OperationID: operationID,
		Method:      candidate.method,
		Path:        candidate.path,
		Summary:     strings.TrimSpace(candidate.op.Summary),
		Description: strings.TrimSpace(candidate.op.Description),
		Tags:        append([]string(nil), candidate.op.Tags...),
		Operation:   extensionString(extensions, "x-operation"),
		Resource:    extensionString(extensions, "x-resource"),
		Capability:  capability,
		Execution:   execution,
		Approval:    approval,
		Risk:        risk,
		Permission:  extensionString(extensions, "x-permission"),
	}
}

func approvalPolicyFromExtensions(
	extensions map[string]interface{},
	candidate methodOperation,
	diags *[]spec.Diagnostic,
) spec.ApprovalPolicy {
	raw, ok := extensions["x-approval"]
	if !ok {
		return spec.ApprovalPolicy{}
	}
	fieldPath := fmt.Sprintf("$.paths.%s.%s.x-approval", candidate.path, strings.ToLower(candidate.method))
	switch value := raw.(type) {
	case bool:
		return spec.ApprovalPolicy{Required: value}
	case map[string]interface{}:
		required, _ := value["required"].(bool)
		policyKey, _ := value["policyKey"].(string)
		if policyKey == "" {
			policyKey, _ = value["policy_key"].(string)
		}
		return spec.ApprovalPolicy{
			Required:  required,
			PolicyKey: strings.TrimSpace(policyKey),
		}
	default:
		*diags = append(*diags, sourceDiagnostic(
			"openapi_approval_invalid",
			spec.SeverityError,
			"x-approval must be a boolean or an object with required/policyKey",
			fieldPath,
		))
		return spec.ApprovalPolicy{}
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
			if forbiddenOpenAPIPresentationField(key) {
				*diags = append(*diags, sourceDiagnostic(
					"openapi_presentation_field_forbidden",
					spec.SeverityError,
					fmt.Sprintf("presentation field %q is not allowed in OpenAPI source", key),
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

func forbiddenOpenAPIPresentationField(key string) bool {
	normalized := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(key, "_", "-")))
	if _, ok := registrationguard.ForbiddenPresentationField(normalized); ok {
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

func (s *Service) requireSourceRead(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(
		ctx,
		s.svcCtx,
		"无权查看 OpenAPI Source",
		"admin:all",
		"openapi_sources:read",
		"openapi_sources:write",
		"resources:read",
		"resources:diagnose",
		"functions:read",
		"functions:manage",
		"pages:read",
		"pages:edit",
	)
	return err
}

func (s *Service) requireSourceWrite(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(
		ctx,
		s.svcCtx,
		"无权管理 OpenAPI Source",
		"admin:all",
		"openapi_sources:write",
		"resources:diagnose",
		"functions:manage",
		"pages:edit",
	)
	return err
}

func (s *Service) auditSourceEvent(ctx context.Context, eventType audit.AuditEventType, gameID, env, sourceID, sourceName string, details map[string]interface{}) {
	if s == nil || s.svcCtx == nil || s.svcCtx.AuditService == nil {
		return
	}
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		actor = "unknown"
	}
	if details == nil {
		details = map[string]interface{}{}
	}
	details["game_id"] = gameID
	details["env"] = env
	details["source_id"] = sourceID
	_, err = s.svcCtx.AuditService.Log(ctx, eventType,
		audit.WithActorID(actor, "user", actor),
		audit.WithResource(audit.ResourceInfo{
			Type:        "openapi_source",
			ID:          sourceID,
			Name:        sourceName,
			GameID:      gameID,
			Environment: env,
		}),
		audit.WithDetails(details),
		audit.WithOutcome("success", ""),
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to write OpenAPI source audit event", "event", eventType, "sourceID", sourceID, "error", err)
	}
}

func (s *Service) startSourceSpan(ctx context.Context, name, gameID, env string, attrs ...attribute.KeyValue) (context.Context, func(error, ...attribute.KeyValue)) {
	if s == nil || s.svcCtx == nil || s.svcCtx.Telemetry == nil {
		return ctx, func(error, ...attribute.KeyValue) {}
	}
	startedAt := time.Now()
	baseAttrs := []attribute.KeyValue{
		attribute.String("game.id", gameID),
		attribute.String("game.env", env),
	}
	baseAttrs = append(baseAttrs, attrs...)
	nextCtx, span := s.svcCtx.Telemetry.StartSpan(ctx, name, baseAttrs...)
	return nextCtx, func(err error, extraAttrs ...attribute.KeyValue) {
		s.svcCtx.Telemetry.EndSpan(span, startedAt, err, extraAttrs...)
	}
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
