// Package descriptors collects raw function descriptor data from runtime,
// OpenAPI, and persisted DB records before normalization.
package descriptors

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/getkin/kin-openapi/openapi3"
)

// Collect gathers descriptor inputs for the dashboard Resource/Page model.
func Collect(ctx context.Context, svcCtx *svc.ServiceContext) []normalizer.DescriptorInput {
	if svcCtx == nil {
		return nil
	}

	byID := map[string]*normalizer.DescriptorInput{}
	gameID, env := svc.GameScopeFromContext(ctx)
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	ensure := func(fid string) *normalizer.DescriptorInput {
		fid = strings.TrimSpace(fid)
		if fid == "" {
			return nil
		}
		if existing := byID[fid]; existing != nil {
			return existing
		}
		input := &normalizer.DescriptorInput{
			ID:      fid,
			Enabled: true,
		}
		byID[fid] = input
		return input
	}

	if store := svcCtx.RegistryStore; store != nil {
		store.Mu().RLock()
		for _, sess := range store.AgentsUnsafe() {
			if sess == nil {
				continue
			}
			if gameID != "" && strings.TrimSpace(sess.GameID) != gameID {
				continue
			}
			if env != "" && strings.TrimSpace(sess.Env) != env {
				continue
			}
			for fid, meta := range sess.Functions {
				input := ensure(fid)
				if input == nil {
					continue
				}
				if meta.Version != "" {
					input.Version = meta.Version
				}
				input.Enabled = meta.Enabled
				mergeRuntimeFunctionMetaInput(input, meta)
			}
		}
		store.Mu().RUnlock()

		for fid, op := range store.ListOpenAPIOperations() {
			input := ensure(fid)
			if input != nil {
				mergeOpenAPIOperationInput(input, op)
			}
		}
	}

	if svcCtx.FunctionModel != nil {
		templates, err := svcCtx.FunctionModel.ListDescriptorTemplates(ctx, "")
		if err == nil {
			for _, template := range templates {
				input := ensure(template.DescriptorID)
				if input == nil {
					continue
				}
				mergeDescriptorTemplateInput(input, template)
			}
		}

		functions, _, err := svcCtx.FunctionModel.List(ctx, modelListAllFunctions())
		if err == nil {
			for _, fn := range functions {
				input := ensure(fn.FunctionID)
				if input == nil {
					continue
				}
				mergeFunctionRecordInput(input, fn)
			}
		}
	}
	mergeOpenAPISourceBindings(ctx, svcCtx, byID)

	inputs := make([]normalizer.DescriptorInput, 0, len(byID))
	for _, input := range byID {
		if input != nil {
			inputs = append(inputs, *input)
		}
	}
	sort.Slice(inputs, func(i, j int) bool {
		return inputs[i].ID < inputs[j].ID
	})
	return inputs
}

type sourceOperationInput struct {
	OperationID string            `json:"operationId"`
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Operation   string            `json:"operation,omitempty"`
	Resource    string            `json:"resource,omitempty"`
	Capability  string            `json:"capability,omitempty"`
	Execution   string            `json:"execution,omitempty"`
	Approval    specApprovalInput `json:"approval,omitempty"`
	Risk        string            `json:"risk,omitempty"`
	Permission  string            `json:"permission,omitempty"`
}

type specApprovalInput struct {
	Required  bool   `json:"required,omitempty"`
	PolicyKey string `json:"policyKey,omitempty"`
}

func mergeOpenAPISourceBindings(ctx context.Context, svcCtx *svc.ServiceContext, byID map[string]*normalizer.DescriptorInput) {
	if svcCtx == nil || svcCtx.OpenAPISourceModel == nil || svcCtx.OpenAPISourceBindingModel == nil {
		return
	}
	gameID, env := svc.GameScopeFromContext(ctx)
	gameID = strings.TrimSpace(gameID)
	env = strings.TrimSpace(env)
	if gameID == "" || env == "" {
		return
	}
	sources, err := svcCtx.OpenAPISourceModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return
	}
	for i := range sources {
		source := &sources[i]
		bindings, err := svcCtx.OpenAPISourceBindingModel.ListBySource(ctx, gameID, env, source.SourceID)
		if err != nil {
			continue
		}
		var operations []sourceOperationInput
		if err := source.GetOperations(&operations); err != nil {
			continue
		}
		operationsByID := make(map[string]sourceOperationInput, len(operations))
		for _, operation := range operations {
			operationID := strings.TrimSpace(operation.OperationID)
			if operationID != "" {
				operationsByID[operationID] = operation
			}
		}
		openAPIOperations := sourceOpenAPIOperationsByID(source.GetSpec())
		for _, binding := range bindings {
			if strings.TrimSpace(binding.Kind) != "provider" {
				continue
			}
			functionID := strings.TrimSpace(binding.FunctionID)
			if functionID == "" {
				continue
			}
			input := byID[functionID]
			if input == nil {
				continue
			}
			operationID := strings.TrimSpace(binding.OperationID)
			sourceOp, ok := operationsByID[operationID]
			if !ok {
				continue
			}
			mergeOpenAPISourceOperationInput(input, sourceOp)
			if op := openAPIOperations[operationID]; op != nil {
				mergeOpenAPIOperationInput(input, op)
			}
		}
	}
}

func mergeOpenAPISourceOperationInput(input *normalizer.DescriptorInput, op sourceOperationInput) {
	if input == nil {
		return
	}
	if input.Summary == "" {
		input.Summary = strings.TrimSpace(op.Summary)
	}
	if input.Description == "" {
		input.Description = strings.TrimSpace(op.Description)
	}
	if input.Tags == nil {
		input.Tags = append([]string(nil), op.Tags...)
	}
	if input.Resource == "" {
		input.Resource = strings.TrimSpace(op.Resource)
	}
	if input.Operation == "" {
		input.Operation = strings.TrimSpace(op.Operation)
	}
	if input.Capability == "" {
		input.Capability = strings.TrimSpace(op.Capability)
	}
	if input.Execution == "" {
		input.Execution = strings.TrimSpace(op.Execution)
	}
	if !input.ApprovalRequired {
		input.ApprovalRequired = op.Approval.Required
	}
	if input.ApprovalPolicyKey == "" {
		input.ApprovalPolicyKey = strings.TrimSpace(op.Approval.PolicyKey)
	}
	if input.Risk == "" {
		input.Risk = strings.TrimSpace(op.Risk)
	}
	if input.Permission == "" {
		input.Permission = strings.TrimSpace(op.Permission)
	}
}

func sourceOpenAPIOperationsByID(raw json.RawMessage) map[string]*openapi3.Operation {
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
	for _, candidate := range methodOperations(doc.Paths) {
		if candidate == nil || strings.TrimSpace(candidate.OperationID) == "" {
			continue
		}
		out[strings.TrimSpace(candidate.OperationID)] = candidate
	}
	return out
}

func methodOperations(paths *openapi3.Paths) []*openapi3.Operation {
	if paths == nil {
		return nil
	}
	out := make([]*openapi3.Operation, 0)
	for _, pathItem := range paths.Map() {
		if pathItem == nil {
			continue
		}
		out = append(out,
			pathItem.Get,
			pathItem.Post,
			pathItem.Put,
			pathItem.Patch,
			pathItem.Delete,
			pathItem.Options,
			pathItem.Head,
			pathItem.Trace,
		)
	}
	return out
}

func mergeDescriptorTemplateInput(input *normalizer.DescriptorInput, template model.Descriptor) {
	if input == nil {
		return
	}
	if input.Summary == "" {
		input.Summary = strings.TrimSpace(firstNonEmpty(template.Name, template.Description))
	}
	if input.Description == "" {
		input.Description = strings.TrimSpace(template.Description)
	}
	if input.InputSchema == "" && len(template.Schema) > 0 {
		if raw, err := json.Marshal(template.Schema); err == nil {
			input.InputSchema = string(raw)
		}
	}
}

func mergeRuntimeFunctionMetaInput(input *normalizer.DescriptorInput, meta reg.FunctionMeta) {
	if input == nil {
		return
	}
	if input.Summary == "" {
		input.Summary = strings.TrimSpace(meta.Summary)
	}
	if input.Description == "" {
		input.Description = strings.TrimSpace(meta.Description)
	}
	if input.InputSchema == "" {
		input.InputSchema = strings.TrimSpace(meta.InputSchema)
	}
	if input.OutputSchema == "" {
		input.OutputSchema = strings.TrimSpace(meta.OutputSchema)
	}
	if input.Resource == "" {
		input.Resource = strings.TrimSpace(meta.Resource)
	}
	if input.Operation == "" {
		input.Operation = strings.TrimSpace(meta.Operation)
	}
	if input.Capability == "" {
		input.Capability = strings.TrimSpace(meta.Capability)
	}
	if input.Execution == "" {
		input.Execution = strings.TrimSpace(meta.Execution)
	}
	if input.Risk == "" {
		input.Risk = strings.TrimSpace(meta.Risk)
	}
	if input.Permission == "" {
		input.Permission = strings.TrimSpace(meta.Permission)
	}
	if input.Tags == nil {
		input.Tags = append([]string(nil), meta.Tags...)
	}
}

func modelListAllFunctions() model.ListFunctionsOptions {
	return model.ListFunctionsOptions{
		PaginationOptions: model.PaginationOptions{Page: 1, PageSize: 10000},
	}
}

func mergeFunctionRecordInput(input *normalizer.DescriptorInput, fn model.Function) {
	input.Enabled = fn.Status != 0
	if fn.Version != "" && input.Version == "" {
		input.Version = fn.Version
	}
	if fn.Description != "" {
		if input.Summary == "" {
			input.Summary = fn.Description
		}
		if input.Description == "" {
			input.Description = fn.Description
		}
	}
	if len(fn.OpenAPISpec) > 0 {
		var op openapi3.Operation
		if raw, err := json.Marshal(fn.OpenAPISpec); err == nil && op.UnmarshalJSON(raw) == nil {
			mergeOpenAPIOperationInput(input, &op)
		}
	}
	if len(fn.Metadata) > 0 {
		if raw, err := json.Marshal(fn.Metadata); err == nil {
			mergeMetadataInput(input, rawMessageMapFromJSON(raw))
		}
	}
}

func mergeOpenAPIOperationInput(input *normalizer.DescriptorInput, op *openapi3.Operation) {
	if input == nil || op == nil {
		return
	}
	if input.Summary == "" {
		input.Summary = strings.TrimSpace(op.Summary)
	}
	if input.Description == "" {
		input.Description = strings.TrimSpace(op.Description)
	}
	if schema := openAPIRequestSchema(op); schema != "" && input.InputSchema == "" {
		input.InputSchema = schema
	}
	if schema := openAPIResponseSchema(op); schema != "" && input.OutputSchema == "" {
		input.OutputSchema = schema
	}
	extRaw, _ := json.Marshal(op.Extensions)
	ext := rawMessageMapFromJSON(extRaw)
	if input.Resource == "" {
		input.Resource = stringExtension(ext, "x-resource")
	}
	if input.Operation == "" {
		input.Operation = stringExtension(ext, "x-operation")
	}
	if input.Capability == "" {
		input.Capability = stringExtension(ext, "x-capability")
	}
	if input.Execution == "" {
		input.Execution = stringExtension(ext, "x-execution")
	}
	mergeApprovalInputFromMetadata(input, ext)
	if input.Risk == "" {
		input.Risk = stringExtension(ext, "x-risk")
	}
	if input.Permission == "" {
		input.Permission = stringExtension(ext, "x-permission")
	}
}

func mergeMetadataInput(input *normalizer.DescriptorInput, metadata map[string]json.RawMessage) {
	if input == nil || len(metadata) == 0 {
		return
	}
	if input.Resource == "" {
		input.Resource = stringExtension(metadata, "resource")
	}
	if input.Operation == "" {
		input.Operation = stringExtension(metadata, "operation")
	}
	if input.Capability == "" {
		input.Capability = stringExtension(metadata, "capability")
	}
	if input.Execution == "" {
		input.Execution = stringExtension(metadata, "execution")
	}
	mergeApprovalInputFromMetadata(input, metadata)
	if input.Risk == "" {
		input.Risk = stringExtension(metadata, "risk")
	}
	if input.Permission == "" {
		input.Permission = stringExtension(metadata, "permission")
	}
}

func openAPIRequestSchema(op *openapi3.Operation) string {
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return ""
	}
	return mediaSchemaJSON(op.RequestBody.Value.Content)
}

func openAPIResponseSchema(op *openapi3.Operation) string {
	if op == nil || op.Responses == nil {
		return ""
	}
	for _, code := range []string{"200", "201", "default"} {
		if ref := op.Responses.Value(code); ref != nil && ref.Value != nil {
			if schema := mediaSchemaJSON(ref.Value.Content); schema != "" {
				return schema
			}
		}
	}
	for _, ref := range op.Responses.Map() {
		if ref != nil && ref.Value != nil {
			if schema := mediaSchemaJSON(ref.Value.Content); schema != "" {
				return schema
			}
		}
	}
	return ""
}

func mediaSchemaJSON(content openapi3.Content) string {
	if len(content) == 0 {
		return ""
	}
	if media := content.Get("application/json"); media != nil && media.Schema != nil {
		return schemaRefJSON(media.Schema)
	}
	for _, media := range content {
		if media != nil && media.Schema != nil {
			return schemaRefJSON(media.Schema)
		}
	}
	return ""
}

func schemaRefJSON(ref *openapi3.SchemaRef) string {
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

func stringExtension(extensions map[string]json.RawMessage, key string) string {
	if len(extensions) == 0 || key == "" {
		return ""
	}
	candidates := []string{key}
	if strings.HasPrefix(key, "x-") {
		candidates = append(candidates, strings.TrimPrefix(key, "x-"))
	} else {
		candidates = append(candidates, "x-"+key)
	}
	for _, candidate := range candidates {
		if raw, ok := extensions[candidate]; ok {
			var value string
			if err := json.Unmarshal(raw, &value); err == nil {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func mergeApprovalInputFromMetadata(input *normalizer.DescriptorInput, metadata map[string]json.RawMessage) {
	if input == nil || len(metadata) == 0 {
		return
	}
	if required, ok := boolExtension(metadata, "approval_required"); ok && !input.ApprovalRequired {
		input.ApprovalRequired = required
	}
	if required, ok := boolExtension(metadata, "requires_approval"); ok && !input.ApprovalRequired {
		input.ApprovalRequired = required
	}
	if policyKey := stringExtension(metadata, "approval_policy_key"); input.ApprovalPolicyKey == "" && policyKey != "" {
		input.ApprovalPolicyKey = policyKey
	}
	raw, ok := firstRawExtension(metadata, "approval")
	if !ok {
		return
	}
	var required bool
	if err := json.Unmarshal(raw, &required); err == nil {
		if !input.ApprovalRequired {
			input.ApprovalRequired = required
		}
		return
	}
	var object struct {
		Required     bool   `json:"required"`
		PolicyKey    string `json:"policyKey"`
		PolicyKeyAlt string `json:"policy_key"`
	}
	if err := json.Unmarshal(raw, &object); err == nil {
		if !input.ApprovalRequired {
			input.ApprovalRequired = object.Required
		}
		if input.ApprovalPolicyKey == "" {
			input.ApprovalPolicyKey = strings.TrimSpace(firstNonEmpty(object.PolicyKey, object.PolicyKeyAlt))
		}
	}
}

func boolExtension(extensions map[string]json.RawMessage, key string) (bool, bool) {
	raw, ok := firstRawExtension(extensions, key)
	if !ok {
		return false, false
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, false
	}
	return value, true
}

func firstRawExtension(extensions map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	candidates := []string{key}
	if strings.HasPrefix(key, "x-") {
		candidates = append(candidates, strings.TrimPrefix(key, "x-"))
	} else {
		candidates = append(candidates, "x-"+key)
	}
	for _, candidate := range candidates {
		if raw, ok := extensions[candidate]; ok {
			return raw, true
		}
	}
	return nil, false
}

func rawMessageMapFromJSON(raw []byte) map[string]json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			return v
		}
	}
	return ""
}
