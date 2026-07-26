package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ui "github.com/cuihairu/croupier/pkg/pb/croupier/component/v1"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	pluginpb "google.golang.org/protobuf/types/pluginpb"
	"gopkg.in/yaml.v3"
)

// OpenAPI types for specification generation
type OpenAPIOperation struct {
	ID          string         `json:"operationId"`
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Resource    string         `json:"x-resource,omitempty"`
	Operation   string         `json:"x-operation,omitempty"`
	Risk        string         `json:"x-risk,omitempty"`
	Permission  string         `json:"x-permission,omitempty"`
	Request     map[string]any `json:"request,omitempty"`
	Response    map[string]any `json:"response,omitempty"`
}

func main() {
	// Read request from stdin
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatalf("read stdin: %v", err)
	}
	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(in, &req); err != nil {
		fatalf("unmarshal CodeGeneratorRequest: %v", err)
	}

	// Defaults and params
	params := parseParams(req.GetParameter())
	emitSchemas := params["emit_schemas"] == "true" || params["schemas"] == "true"

	resp := &pluginpb.CodeGeneratorResponse{}

	// Build a lookup for message types and services in files to generate
	filesToGen := make(map[string]bool)
	for _, f := range req.GetFileToGenerate() {
		filesToGen[f] = true
	}

	// Build global indexes for message/enum resolution across all proto files in the request.
	globalMsgIndex := map[string]*descriptorpb.DescriptorProto{}
	globalEnumIndex := map[string]*descriptorpb.EnumDescriptorProto{}
	for _, fd := range req.GetProtoFile() {
		for k, v := range indexMessages(fd) {
			globalMsgIndex[k] = v
		}
		for k, v := range indexEnums(fd) {
			globalEnumIndex[k] = v
		}
	}

	// OpenAPI output collection
	openapiOps := make([]OpenAPIOperation, 0)

	var generatedFiles []generatedFile
	emittedSchemas := map[string]bool{}

	// Iterate files
	for _, fd := range req.GetProtoFile() {
		if !filesToGen[fd.GetName()] {
			continue
		}
		pkg := fd.GetPackage()

		for _, svc := range fd.GetService() {
			for _, m := range svc.GetMethod() {
				// Derive function spec.
				funID := defaultFunctionID(pkg, svc.GetName(), m.GetName())

				// Transport info
				inType := strings.TrimPrefix(m.GetInputType(), ".")
				outType := strings.TrimPrefix(m.GetOutputType(), ".")

				// Parse method-level custom options
				fo := parseFunctionOptions(m.GetOptions())

				// Build OpenAPI operation
				op := OpenAPIOperation{
					ID:          funID,
					Description: fmt.Sprintf("gRPC method: %s.%s/%s", pkg, svc.GetName(), m.GetName()),
				}

				// Apply overrides from function options
				if fo.FunctionID != "" {
					op.ID = fo.FunctionID
					funID = fo.FunctionID
				}
				if fo.Resource != "" {
					op.Resource = fo.Resource
				}
				if fo.Operation != "" {
					op.Operation = fo.Operation
				}
				if fo.Risk != "" {
					op.Risk = strings.ToLower(fo.Risk)
				}
				if fo.Permission != "" {
					op.Permission = fo.Permission
				}
				if fo.Summary != "" {
					op.Summary = fo.Summary
				}
				if fo.Description != "" {
					op.Description = fo.Description
				}
				// Add request/response schema references
				op.Request = map[string]any{"proto_fqn": inType}
				op.Response = map[string]any{"proto_fqn": outType}

				// Optionally emit JSON schemas
				if emitSchemas {
					if inMsg := globalMsgIndex[m.GetInputType()]; inMsg != nil {
						schema := buildJSONSchema(pkg, globalMsgIndex, globalEnumIndex, inMsg)
						schemaFile := schemaFileForFQN(m.GetInputType())
						if !emittedSchemas[schemaFile] {
							addJSON(resp, &generatedFiles, schemaFile, schema)
							emittedSchemas[schemaFile] = true
						}
					}

					if outMsg := globalMsgIndex[m.GetOutputType()]; outMsg != nil {
						schema := buildJSONSchema(pkg, globalMsgIndex, globalEnumIndex, outMsg)
						schemaFile := schemaFileForFQN(m.GetOutputType())
						if !emittedSchemas[schemaFile] {
							addJSON(resp, &generatedFiles, schemaFile, schema)
							emittedSchemas[schemaFile] = true
						}
					}
				}

				openapiOps = append(openapiOps, op)
			}
		}
	}

	// Generate OpenAPI 3.0.3 spec
	if len(openapiOps) > 0 {
		openapiDoc := buildOpenAPIDoc(openapiOps, params)
		addYAML(resp, &generatedFiles, "openapi.yaml", openapiDoc)
	}

	// Write response
	out, err := proto.Marshal(resp)
	if err != nil {
		fatalf("marshal CodeGeneratorResponse: %v", err)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fatalf("write stdout: %v", err)
	}
}

// Helpers

type generatedFile struct {
	Name string
	Data []byte
}

func addJSON(resp *pluginpb.CodeGeneratorResponse, files *[]generatedFile, name string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{Name: proto.String(name), Content: proto.String(string(b))})
	*files = append(*files, generatedFile{Name: name, Data: b})
}

func addYAML(resp *pluginpb.CodeGeneratorResponse, files *[]generatedFile, name string, v any) {
	b, _ := yaml.Marshal(v)
	resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{Name: proto.String(name), Content: proto.String(string(b))})
	*files = append(*files, generatedFile{Name: name, Data: b})
}

func buildOpenAPIDoc(ops []OpenAPIOperation, params map[string]string) map[string]any {
	doc := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       firstNonEmpty(params["title"], params["name"], "Croupier Functions"),
			"description": firstNonEmpty(params["description"], "Auto-generated OpenAPI specification from protobuf definitions"),
			"version":     firstNonEmpty(params["version"], params["provider_version"], "1.0.0"),
		},
		"paths": map[string]any{},
	}

	// Add paths from operations
	paths := doc["paths"].(map[string]any)
	for _, op := range ops {
		path := fmt.Sprintf("/%s", op.ID)
		pathItem := map[string]any{
			"post": map[string]any{
				"operationId": op.ID,
				"summary":     firstNonEmpty(op.Summary, op.ID),
				"description": op.Description,
				"requestBody": map[string]any{
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"type":                 "object",
								"additionalProperties": true,
							},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "Success",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type":                 "object",
									"additionalProperties": true,
								},
							},
						},
					},
				},
				"x-request": op.Request,
				"x-response": op.Response,
			},
		}

		// Add optional extensions
		if op.Resource != "" {
			pathItem["post"].(map[string]any)["x-resource"] = op.Resource
		}
		if op.Operation != "" {
			pathItem["post"].(map[string]any)["x-operation"] = op.Operation
		}
		if op.Risk != "" {
			pathItem["post"].(map[string]any)["x-risk"] = op.Risk
		}
		if op.Permission != "" {
			pathItem["post"].(map[string]any)["x-permission"] = op.Permission
		}
		paths[path] = pathItem
	}

	return doc
}

func parseParams(p string) map[string]string {
	res := map[string]string{}
	for _, kv := range strings.Split(p, ",") {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 1 {
			res[parts[0]] = "true"
			continue
		}
		res[parts[0]] = parts[1]
	}
	return res
}

func indexMessages(fd *descriptorpb.FileDescriptorProto) map[string]*descriptorpb.DescriptorProto {
	idx := map[string]*descriptorpb.DescriptorProto{}
	var walk func(prefix string, msgs []*descriptorpb.DescriptorProto)
	walk = func(prefix string, msgs []*descriptorpb.DescriptorProto) {
		for _, m := range msgs {
			fqn := prefix + "." + m.GetName()
			idx["."+fqn] = m
			// nested
			if len(m.NestedType) > 0 {
				walk(fqn, m.NestedType)
			}
		}
	}
	pkg := fd.GetPackage()
	walk(pkg, fd.GetMessageType())
	return idx
}

func indexEnums(fd *descriptorpb.FileDescriptorProto) map[string]*descriptorpb.EnumDescriptorProto {
	idx := map[string]*descriptorpb.EnumDescriptorProto{}
	var walk func(prefix string, msgs []*descriptorpb.DescriptorProto)
	walk = func(prefix string, msgs []*descriptorpb.DescriptorProto) {
		for _, m := range msgs {
			fqn := prefix + "." + m.GetName()
			for _, e := range m.GetEnumType() {
				idx["."+fqn+"."+e.GetName()] = e
			}
			if len(m.NestedType) > 0 {
				walk(fqn, m.NestedType)
			}
		}
	}
	pkg := fd.GetPackage()
	// top-level enums
	for _, e := range fd.GetEnumType() {
		idx["."+pkg+"."+e.GetName()] = e
	}
	// nested enums
	walk(pkg, fd.GetMessageType())
	return idx
}

func buildJSONSchema(pkg string, msgIdx map[string]*descriptorpb.DescriptorProto, enumIdx map[string]*descriptorpb.EnumDescriptorProto, m *descriptorpb.DescriptorProto) map[string]any {
	schema := map[string]any{
		"$schema":    "https://json-schema.org/draft/2020-12/schema",
		"type":       "object",
		"title":      m.GetName(),
		"properties": map[string]any{},
	}
	props := schema["properties"].(map[string]any)
	var required []string
	for _, f := range m.GetField() {
		name := f.GetJsonName()
		if name == "" {
			name = f.GetName()
		}
		typ, req := fieldToJSONSchema(pkg, msgIdx, enumIdx, f)
		props[name] = typ
		if req {
			required = append(required, name)
		}
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func fieldToJSONSchema(pkg string, msgIdx map[string]*descriptorpb.DescriptorProto, enumIdx map[string]*descriptorpb.EnumDescriptorProto, f *descriptorpb.FieldDescriptorProto) (map[string]any, bool) {
	required := false
	switch f.GetLabel() {
	case descriptorpb.FieldDescriptorProto_LABEL_REQUIRED:
		required = true
	}

	// Repeated fields are arrays in JSON, except protobuf maps (handled below).
	if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED && f.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
		item, _ := fieldToJSONSchema(pkg, msgIdx, enumIdx, &descriptorpb.FieldDescriptorProto{
			Type:     f.Type,
			TypeName: f.TypeName,
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		})
		return map[string]any{"type": "array", "items": item}, required
	}

	basic := func(t string) map[string]any { return map[string]any{"type": t} }
	format := func(t, fmt string) map[string]any { return map[string]any{"type": t, "format": fmt} }

	switch f.GetType() {
	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return basic("string"), required
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return basic("boolean"), required
	case descriptorpb.FieldDescriptorProto_TYPE_INT32, descriptorpb.FieldDescriptorProto_TYPE_SINT32, descriptorpb.FieldDescriptorProto_TYPE_SFIXED32:
		return format("integer", "int32"), required
	case descriptorpb.FieldDescriptorProto_TYPE_UINT32, descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return format("integer", "uint32"), required
	case descriptorpb.FieldDescriptorProto_TYPE_INT64, descriptorpb.FieldDescriptorProto_TYPE_SINT64, descriptorpb.FieldDescriptorProto_TYPE_SFIXED64:
		return format("string", "int64"), required // use string to avoid JS precision loss
	case descriptorpb.FieldDescriptorProto_TYPE_UINT64, descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return format("string", "uint64"), required
	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT:
		return format("number", "float"), required
	case descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return format("number", "double"), required
	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return basic("string"), required
	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		// enums use string names in proto JSON
		sch := basic("string")
		if e := enumIdx[f.GetTypeName()]; e != nil {
			vals := make([]string, 0, len(e.GetValue()))
			for _, v := range e.GetValue() {
				vals = append(vals, v.GetName())
			}
			sort.Strings(vals)
			sch["enum"] = vals
			sch["x-enum-source"] = strings.TrimPrefix(f.GetTypeName(), ".")
		}
		return sch, required
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		// Map or nested message
		t := f.GetTypeName()
		// Normalize to a leading dot for fully-qualified type comparisons
		if !strings.HasPrefix(t, ".") {
			t = "." + t
		}
		// Detect google.protobuf.Timestamp/Duration → strings with format
		if t == ".google.protobuf.Timestamp" {
			return map[string]any{"type": "string", "format": "date-time"}, required
		}
		if t == ".google.protobuf.Duration" {
			return map[string]any{"type": "string", "pattern": "^\\d+[smhd]$"}, required
		}
		// Map type (detect map_entry)
		if sub := msgIdx[t]; sub != nil && sub.GetOptions().GetMapEntry() {
			// map<key, value> represented by a message with key/value fields
			var _, valType map[string]any
			for _, sf := range sub.GetField() {
				if sf.GetName() == "key" {
					_, _ = fieldToJSONSchema(pkg, msgIdx, enumIdx, sf)
				} else if sf.GetName() == "value" {
					valType, _ = fieldToJSONSchema(pkg, msgIdx, enumIdx, sf)
				}
			}
			sch := map[string]any{"type": "object"}
			if valType != nil {
				sch["additionalProperties"] = valType
			}
			return sch, required
		}
		// Repeated message as array
		if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			item := map[string]any{"type": "object"}
			if sub := msgIdx[t]; sub != nil {
				item = buildJSONSchema(pkg, msgIdx, enumIdx, sub)
			}
			return map[string]any{"type": "array", "items": item}, required
		}
		// Nested object
		sub := msgIdx[t]
		if sub != nil {
			return buildJSONSchema(pkg, msgIdx, enumIdx, sub), required
		}
		return map[string]any{"type": "object"}, required
	default:
		return basic("string"), required
	}
}

func defaultFunctionID(pkg, svc, method string) string {
	// default: <pkg>.<service>.<method> in lower snake for method
	id := pkg + "." + svc + "." + method
	// normalize: lower case, dots kept
	id = strings.ReplaceAll(id, " ", "")
	return strings.ToLower(id)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func joinOptionName(u *descriptorpb.UninterpretedOption) string {
	if u == nil {
		return ""
	}
	parts := make([]string, 0, len(u.GetName()))
	for _, np := range u.GetName() {
		s := np.GetNamePart()
		// extension parts may come as "croupier.options.function" or "(croupier.options.function)"
		s = strings.TrimPrefix(s, "(")
		s = strings.TrimSuffix(s, ")")
		parts = append(parts, s)
	}
	// For extension options usually it's a single part with fully-qualified path
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts, ".")
}

func schemaFileForFQN(fqn string) string {
	fqn = strings.TrimPrefix(strings.TrimSpace(fqn), ".")
	if fqn == "" {
		return "schema/unknown.json"
	}
	// Lowercase to keep filenames stable; caller must use the same when referencing.
	return filepath.ToSlash(filepath.Join("schema", sanitize(strings.ToLower(fqn))+".json"))
}

func sanitize(id string) string {
	// replace non-filename chars
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.' || r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, id)
	return out
}

func trimQuotes(s string) string {
	return strings.Trim(s, "\"")
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(trimQuotes(s)))
	return s == "true" || s == "1" || s == "yes"
}

// ========== Options parsing ==========

type funcOpts struct {
	FunctionID        string
	Version           string
	Resource          string
	Operation         string
	Risk              string
	Route             string
	Timeout           string
	TwoPersonRule     bool
	TwoPersonRuleSet  bool
	Mode              string
	IdempotencyKey    bool
	IdempotencyKeySet bool
	Summary           string
	Description       string
	Tags              []string
	Permission        string
}

func parseFunctionOptions(mo *descriptorpb.MethodOptions) funcOpts {
	var out funcOpts

	if mo == nil {
		return out
	}

	if proto.HasExtension(mo, ui.E_Function) {
		ext := proto.GetExtension(mo, ui.E_Function)
		if fn, ok := ext.(*ui.FunctionOptions); ok && fn != nil {
			out.FunctionID = strings.TrimSpace(fn.GetFunctionId())
			out.Version = strings.TrimSpace(fn.GetVersion())
			out.Resource = strings.TrimSpace(fn.GetResource())
			out.Operation = strings.TrimSpace(fn.GetOperation())
			out.Risk = strings.TrimSpace(fn.GetRisk())
			out.Route = strings.TrimSpace(fn.GetRoute())
			out.Timeout = strings.TrimSpace(fn.GetTimeout())
			out.TwoPersonRule = fn.GetTwoPersonRule()
			out.TwoPersonRuleSet = true
			out.Mode = strings.TrimSpace(fn.GetMode())
			out.IdempotencyKey = fn.GetIdempotencyKey()
			out.IdempotencyKeySet = true
			out.Summary = strings.TrimSpace(fn.GetSummary())
			out.Description = strings.TrimSpace(fn.GetDescription())
			if len(fn.GetTags()) > 0 {
				out.Tags = append([]string{}, fn.GetTags()...)
			}
			out.Permission = strings.TrimSpace(fn.GetPermission())
			return out
		}
	}

	// Fallback: parse method-level custom options from UninterpretedOption aggregate_value
	for _, u := range mo.GetUninterpretedOption() {
		name := joinOptionName(u)
		if name != "croupier.options.v1.function" {
			continue
		}
		raw := u.GetAggregateValue()
		kv := parseAggregateKV(raw)
		if v := kv["function_id"]; v != "" {
			out.FunctionID = trimQuotes(v)
		}
		if v := kv["version"]; v != "" {
			out.Version = trimQuotes(v)
		}
		if v := kv["resource"]; v != "" {
			out.Resource = trimQuotes(v)
		}
		if v := kv["operation"]; v != "" {
			out.Operation = trimQuotes(v)
		}
		if v := kv["risk"]; v != "" {
			out.Risk = trimQuotes(v)
		}
		if v := kv["two_person_rule"]; v != "" {
			out.TwoPersonRule, out.TwoPersonRuleSet = parseBool(v), true
		}
		if v := kv["summary"]; v != "" {
			out.Summary = trimQuotes(v)
		}
		if v := kv["description"]; v != "" {
			out.Description = trimQuotes(v)
		}
		if v := kv["permission"]; v != "" {
			out.Permission = trimQuotes(v)
		}
	}
	return out
}

func parseAggregateKV(s string) map[string]string {
	// very small tolerant parser for key: value pairs inside {...}
	res := map[string]string{}
	if s == "" {
		return res
	}
	// strip outer braces if present
	src := strings.TrimSpace(s)
	if strings.HasPrefix(src, "{") && strings.HasSuffix(src, "}") {
		src = strings.TrimSpace(src[1 : len(src)-1])
	}
	i := 0
	for i < len(src) {
		// skip spaces/commas/newlines
		for i < len(src) && (src[i] == ' ' || src[i] == '\n' || src[i] == '\t' || src[i] == ',') {
			i++
		}
		if i >= len(src) {
			break
		}
		// field name
		start := i
		for i < len(src) {
			c := src[i]
			if c == ':' || c == ' ' || c == '\t' || c == '\n' {
				break
			}
			i++
		}
		name := strings.TrimSpace(src[start:i])
		// skip to colon
		for i < len(src) && src[i] != ':' {
			i++
		}
		if i < len(src) && src[i] == ':' {
			i++
		}
		// skip spaces
		for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n') {
			i++
		}
		// parse value
		if i >= len(src) {
			break
		}
		var val string
		switch src[i] {
		case '"': // string literal
			i++
			var b strings.Builder
			for i < len(src) {
				if src[i] == '\\' && i+1 < len(src) {
					// escape next
					b.WriteByte(src[i+1])
					i += 2
					continue
				}
				if src[i] == '"' {
					i++
					break
				}
				b.WriteByte(src[i])
				i++
			}
			val = b.String()
		case '{': // nested block (map/object) -> skip
			depth := 1
			i++
			for i < len(src) && depth > 0 {
				if src[i] == '{' {
					depth++
				} else if src[i] == '}' {
					depth--
				}
				i++
			}
			// ignore nested content
			val = "{}"
		default:
			start := i
			for i < len(src) {
				c := src[i]
				if c == ',' || c == '\n' || c == ' ' || c == '}' {
					break
				}
				i++
			}
			val = strings.TrimSpace(src[start:i])
		}
		if name != "" {
			res[name] = val
		}
		// skip trailing separators
		for i < len(src) && (src[i] == ',' || src[i] == ' ' || src[i] == '\n' || src[i] == '\t') {
			i++
		}
	}
	return res
}

func parseOptionObjectMap(s, fieldName string) map[string]string {
	res := map[string]string{}
	if s == "" || fieldName == "" {
		return res
	}
	i := 0
	for i < len(s) {
		idx := strings.Index(s[i:], fieldName)
		if idx < 0 {
			break
		}
		i += idx
		// ensure it's a full field name followed by ':'
		j := i + len(fieldName)
		// skip spaces
		for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n') {
			j++
		}
		if j >= len(s) || s[j] != ':' {
			i = j
			continue
		}
		j++
		for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n') {
			j++
		}
		if j >= len(s) || s[j] != '{' {
			i = j
			continue
		}
		// parse object body until matching }
		j++
		depth := 1
		start := j
		for j < len(s) && depth > 0 {
			if s[j] == '{' {
				depth++
			} else if s[j] == '}' {
				depth--
			}
			j++
		}
		body := s[start : j-1]
		// parse simple k: "v" pairs
		k := 0
		for k < len(body) {
			// skip separators
			for k < len(body) && (body[k] == ' ' || body[k] == '\n' || body[k] == '\t' || body[k] == ',') {
				k++
			}
			if k >= len(body) {
				break
			}
			// key
			ks := k
			for k < len(body) {
				c := body[k]
				if c == ':' || c == ' ' || c == '\n' || c == '\t' {
					break
				}
				k++
			}
			key := strings.TrimSpace(body[ks:k])
			for k < len(body) && body[k] != ':' {
				k++
			}
			if k < len(body) && body[k] == ':' {
				k++
			}
			for k < len(body) && (body[k] == ' ' || body[k] == '\n' || body[k] == '\t') {
				k++
			}
			// value (expect quoted)
			val := ""
			if k < len(body) && body[k] == '"' {
				k++
				var b strings.Builder
				for k < len(body) {
					if body[k] == '\\' && k+1 < len(body) {
						b.WriteByte(body[k+1])
						k += 2
						continue
					}
					if body[k] == '"' {
						k++
						break
					}
					b.WriteByte(body[k])
					k++
				}
				val = b.String()
			} else {
				vs := k
				for k < len(body) {
					c := body[k]
					if c == ',' || c == '\n' || c == ' ' {
						break
					}
					k++
				}
				val = strings.TrimSpace(body[vs:k])
			}
			if key != "" {
				res[key] = val
			}
		}
		i = j
	}
	return res
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
