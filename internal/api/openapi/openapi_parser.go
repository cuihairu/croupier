package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	driveropenapi "github.com/cuihairu/croupier/internal/drivers/openapi"
	"github.com/cuihairu/croupier/internal/function/registrationguard"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

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
		// 设计债清理：trimmed 已通过 json.Valid 校验，Compact 对合法 JSON
		// 恒成功，原 err 分支不可达已删除。
		_ = json.Compact(&compacted, trimmed)
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
	parsed := parseOpenAPISource(raw)
	if hasErrorDiagnostic(parsed.Diagnostics) {
		return nil, "", errorx.NewBadRequestWithDetails("OpenAPI source is invalid", map[string]any{
			"diagnostics": parsed.Diagnostics,
		})
	}
	return parsed, format, nil
}

// parseOpenAPISource 解析并校验 OpenAPI 源，所有失败（扫描/解析/校验/版本）
// 一律以 Diagnostics 承载而非 error。
// 设计债清理：原实现的全部出口均返回 nil error，error 返回值已随签名收紧删除。
func parseOpenAPISource(raw []byte) *parsedOpenAPISource {
	parsed := &parsedOpenAPISource{Spec: json.RawMessage(raw)}
	parsed.Diagnostics = append(parsed.Diagnostics, scanOpenAPISourceRaw(raw)...)
	if hasErrorDiagnostic(parsed.Diagnostics) {
		return parsed
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
		return parsed
	}
	normalizeOpenAPIDoc(doc)
	if err := doc.Validate(loader.Context); err != nil {
		parsed.Diagnostics = append(parsed.Diagnostics, sourceDiagnostic(
			"openapi_validation_failed",
			spec.SeverityError,
			err.Error(),
			"$",
		))
		return parsed
	}
	parsed.OpenAPIVersion = strings.TrimSpace(doc.OpenAPI)
	if !strings.HasPrefix(parsed.OpenAPIVersion, "3.") {
		parsed.Diagnostics = append(parsed.Diagnostics, sourceDiagnostic(
			"openapi_version_unsupported",
			spec.SeverityError,
			"only OpenAPI 3.x sources are supported",
			"$.openapi",
		))
		return parsed
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
	return parsed
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
	// 设计债清理：openAPIMethodOperations 按 (path, method) 唯一产出候选
	// （pathItem 的每方法字段单值），items 不存在 (Path,Method) 重复对，
	// 原第三级 OperationID tie-break 恒不可达已删除，两级已构成全序。
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		return items[i].Method < items[j].Method
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
	fieldPrefix := fmt.Sprintf("$.paths.%s.%s", candidate.path, strings.ToLower(candidate.method))
	risk := spec.RiskLevel(extensionString(extensions, "x-risk"))
	if risk != "" && !isValidRisk(risk) {
		*diags = append(*diags, sourceDiagnostic(
			"openapi_risk_invalid",
			spec.SeverityWarning,
			"invalid x-risk ignored",
			fieldPrefix+".x-risk",
		))
		risk = ""
	}
	capability := spec.CapabilityKind(extensionString(extensions, "x-capability"))
	if capability != "" && !spec.IsValidCapabilityKind(capability) {
		*diags = append(*diags, sourceDiagnostic(
			"openapi_capability_invalid",
			spec.SeverityError,
			"x-capability must be one of collection_query, item_query, create, update, delete, action, task, report",
			fieldPrefix+".x-capability",
		))
		capability = ""
	}

	// Use new REST classifier for more comprehensive classification
	classifier := driveropenapi.NewRESTCapabilityClassifier()
	var classificationResult *driveropenapi.ClassificationResult
	inferredCapability := spec.CapabilityKind("")

	// If capability is not provided, try to infer from REST method/path
	if capability == "" {
		hasRequestBody := candidate.op.RequestBody != nil && candidate.op.RequestBody.Value != nil
		responseIsArray := false // TODO: check response schema if needed

		result := classifier.ClassifyOperation(
			candidate.method,
			candidate.path,
			hasRequestBody,
			responseIsArray,
		)

		if result.Capability != "" {
			classificationResult = &result
			inferredCapability = result.Capability
			capability = result.Capability

			// Add classification diagnostic with confidence info
			*diags = append(*diags, restClassificationDiagnosticWithConfidence(
				candidate.method, candidate.path, result.Capability, result.Confidence,
			))

			// Add any additional diagnostics from classification
			*diags = append(*diags, result.Diagnostics...)
		}
	}

	execution := spec.FunctionExecution(extensionString(extensions, "x-execution"))
	if execution != "" && !spec.IsValidFunctionExecution(execution) {
		*diags = append(*diags, sourceDiagnostic(
			"openapi_execution_invalid",
			spec.SeverityError,
			"x-execution must be one of sync, task",
			fieldPrefix+".x-execution",
		))
		execution = ""
	}
	approval := approvalPolicyFromExtensions(extensions, candidate, diags)
	resourceKey := extensionString(extensions, "x-resource")
	if resourceKey == "" && classificationResult != nil && classificationResult.ResourceKey != "" {
		resourceKey = classificationResult.ResourceKey
	}
	operationKey := extensionString(extensions, "x-operation")
	if operationKey == "" && inferredCapability != "" {
		operationKey = operationKeyForCapability(inferredCapability, candidate.path)
	}
	appendStableSourceKeyDiagnostic(diags, "openapi_resource_key_invalid", resourceKey, fieldPrefix+".x-resource")
	appendStableSourceKeyDiagnostic(diags, "openapi_operation_key_invalid", operationKey, fieldPrefix+".x-operation")
	timeoutMs := extensionInt(extensions, "x-timeout-ms", fieldPrefix, diags)
	return OpenAPISourceOperation{
		OperationID: operationID,
		Method:      candidate.method,
		Path:        candidate.path,
		Summary:     strings.TrimSpace(candidate.op.Summary),
		Description: strings.TrimSpace(candidate.op.Description),
		Tags:        append([]string(nil), candidate.op.Tags...),
		Operation:   operationKey,
		Resource:    resourceKey,
		Capability:  capability,
		Execution:   execution,
		Approval:    approval,
		Risk:        risk,
		Permission:  extensionString(extensions, "x-permission"),
		TimeoutMs:   timeoutMs,
	}
}

func appendStableSourceKeyDiagnostic(diags *[]spec.Diagnostic, code, value, field string) {
	if strings.TrimSpace(value) == "" || isStableSourceKey(value) {
		return
	}
	*diags = append(*diags, sourceDiagnostic(
		code,
		spec.SeverityError,
		"key must match [a-z0-9][a-z0-9._-]*",
		field,
	))
}

func isStableSourceKey(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for i, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !valid {
			return false
		}
		if i == 0 && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func operationKeyForCapability(capability spec.CapabilityKind, path string) string {
	switch capability {
	case spec.CapabilityCollectionQuery:
		return "list"
	case spec.CapabilityItemQuery:
		return "get"
	case spec.CapabilityCreate:
		return "create"
	case spec.CapabilityUpdate:
		return "update"
	case spec.CapabilityDelete:
		return "delete"
	case spec.CapabilityAction:
		if key := lastStaticPathSegment(path); key != "" {
			return key
		}
		return "action"
	default:
		return ""
	}
}

func lastStaticPathSegment(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" || strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			continue
		}
		return part
	}
	return ""
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
	scanOpenAPIValue(value, "$", openAPIScanRoot, &diags)
	return diags
}

type openAPIScanContext uint8

const (
	openAPIScanRoot openAPIScanContext = iota
	openAPIScanPaths
	openAPIScanPathItem
	openAPIScanOperation
)

func scanOpenAPIValue(value interface{}, path string, context openAPIScanContext, diags *[]spec.Diagnostic) {
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
			if forbiddenOpenAPIPresentationField(key, context == openAPIScanOperation) {
				*diags = append(*diags, sourceDiagnostic(
					"openapi_presentation_field_forbidden",
					spec.SeverityError,
					fmt.Sprintf("presentation field %q is not allowed in OpenAPI source", key),
					childPath,
				))
			}
			scanOpenAPIValue(child, childPath, nextOpenAPIScanContext(context, key), diags)
		}
	case []interface{}:
		for i, child := range v {
			scanOpenAPIValue(child, fmt.Sprintf("%s[%d]", path, i), openAPIScanRoot, diags)
		}
	}
}

func nextOpenAPIScanContext(context openAPIScanContext, key string) openAPIScanContext {
	switch context {
	case openAPIScanRoot:
		if key == "paths" {
			return openAPIScanPaths
		}
	case openAPIScanPaths:
		return openAPIScanPathItem
	case openAPIScanPathItem:
		if isOpenAPIMethod(key) {
			return openAPIScanOperation
		}
	}
	return openAPIScanRoot
}

func isOpenAPIMethod(key string) bool {
	switch strings.ToLower(key) {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return true
	default:
		return false
	}
}

func forbiddenOpenAPIPresentationField(key string, operationObject bool) bool {
	normalized := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(key, "_", "-")))
	if strings.HasPrefix(normalized, "x-") {
		_, forbidden := registrationguard.ForbiddenPresentationField(normalized)
		return forbidden
	}
	if normalized == "formily" {
		_, forbidden := registrationguard.ForbiddenPresentationField(normalized)
		return forbidden
	}
	if !operationObject {
		return false
	}
	_, forbidden := registrationguard.ForbiddenRegistrationExtensionField(normalized)
	return forbidden
}

func hasErrorDiagnostic(diags []spec.Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity == spec.SeverityError {
			return true
		}
	}
	return false
}

func openAPIOperationFromSource(source *model.OpenAPISource, operationID string) *openapi3.Operation {
	if source == nil {
		return nil
	}
	return openAPIOperationsByID(source.GetSpec())[strings.TrimSpace(operationID)]
}

func openAPIOperationsByID(raw json.RawMessage) map[string]*openapi3.Operation {
	out := map[string]*openapi3.Operation{}
	if len(raw) == 0 {
		return out
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(raw)
	if err != nil || doc == nil || doc.Paths == nil {
		return out
	}
	normalizeOpenAPIDoc(doc)
	for _, candidate := range openAPIMethodOperations(doc.Paths) {
		if candidate.op == nil || strings.TrimSpace(candidate.op.OperationID) == "" {
			continue
		}
		mergePathLevelParameters(doc.Paths.Find(candidate.path), candidate.op)
		out[strings.TrimSpace(candidate.op.OperationID)] = candidate.op
	}
	return out
}

// mergePathLevelParameters appends path-item level parameters that the
// operation does not override so request schemas see shared path params.
func mergePathLevelParameters(pathItem *openapi3.PathItem, op *openapi3.Operation) {
	if pathItem == nil || op == nil {
		return
	}
	seen := map[string]bool{}
	for _, paramRef := range op.Parameters {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		seen[strings.ToLower(paramRef.Value.In+":"+paramRef.Value.Name)] = true
	}
	for _, paramRef := range pathItem.Parameters {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		key := strings.ToLower(paramRef.Value.In + ":" + paramRef.Value.Name)
		if seen[key] {
			continue
		}
		op.Parameters = append(op.Parameters, paramRef)
	}
}

func openAPIRequestSchema(op *openapi3.Operation) string {
	if op == nil {
		return ""
	}
	properties := map[string]json.RawMessage{}
	required := make([]string, 0)
	for _, paramRef := range op.Parameters {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := paramRef.Value
		if param.In != "path" && param.In != "query" {
			continue
		}
		name := strings.TrimSpace(param.Name)
		if name == "" {
			continue
		}
		schemaRef := param.Schema
		if schemaRef == nil {
			schemaRef = &openapi3.SchemaRef{Value: openapi3.NewStringSchema()}
		}
		schemaJSON := json.RawMessage(openAPISchemaRefJSON(schemaRef))
		if len(schemaJSON) == 0 {
			continue
		}
		properties[name] = schemaJSON
		if param.Required {
			required = append(required, name)
		}
	}
	bodySchema := ""
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		bodySchema = openAPIMediaSchemaJSON(op.RequestBody.Value.Content)
	}
	if len(properties) == 0 {
		return bodySchema
	}
	if bodySchema != "" {
		var body map[string]interface{}
		if err := json.Unmarshal([]byte(bodySchema), &body); err == nil {
			if bodyProps, ok := body["properties"].(map[string]interface{}); ok {
				for name, prop := range bodyProps {
					raw, err := json.Marshal(prop)
					if err == nil {
						properties[name] = raw
					}
				}
			}
			if bodyRequired, ok := body["required"].([]interface{}); ok {
				for _, item := range bodyRequired {
					if name, ok := item.(string); ok {
						required = append(required, name)
					}
				}
			}
		}
	}
	root := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	seen := map[string]bool{}
	deduped := make([]string, 0, len(required))
	for _, name := range required {
		if seen[name] {
			continue
		}
		seen[name] = true
		deduped = append(deduped, name)
	}
	sort.Strings(deduped)
	if len(deduped) > 0 {
		root["required"] = deduped
	}
	// 设计债清理：root 的 properties 值均为 json.Marshal 产物（合法 JSON
	// 子串），对 map[string]json.RawMessage 再 Marshal 恒成功，
	// 原 err 分支不可达已删除。
	raw, _ := json.Marshal(root)
	return string(raw)
}

func openAPIResponseSchema(op *openapi3.Operation) string {
	if op == nil || op.Responses == nil {
		return ""
	}
	for _, code := range []string{"200", "201", "default"} {
		if ref := op.Responses.Value(code); ref != nil && ref.Value != nil {
			if schema := openAPIMediaSchemaJSON(ref.Value.Content); schema != "" {
				return schema
			}
		}
	}
	for _, ref := range op.Responses.Map() {
		if ref != nil && ref.Value != nil {
			if schema := openAPIMediaSchemaJSON(ref.Value.Content); schema != "" {
				return schema
			}
		}
	}
	return ""
}

func openAPIMediaSchemaJSON(content openapi3.Content) string {
	if len(content) == 0 {
		return ""
	}
	if media := content.Get("application/json"); media != nil && media.Schema != nil {
		return openAPISchemaRefJSON(media.Schema)
	}
	for _, media := range content {
		if media != nil && media.Schema != nil {
			return openAPISchemaRefJSON(media.Schema)
		}
	}
	return ""
}

func openAPISchemaRefJSON(ref *openapi3.SchemaRef) string {
	if ref == nil {
		return ""
	}
	var raw []byte
	var err error
	if ref.Value != nil {
		raw, err = json.Marshal(ref.Value)
	} else if strings.TrimSpace(ref.Ref) != "" {
		raw, err = json.Marshal(map[string]string{"$ref": ref.Ref})
	} else {
		return ""
	}
	if err != nil {
		return ""
	}
	return string(raw)
}
