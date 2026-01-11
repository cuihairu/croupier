package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	commonv1 "github.com/cuihairu/croupier/pkg/pb/croupier/common/v1"
	optionsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/options/v1"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	pluginpb "google.golang.org/protobuf/types/pluginpb"
)

// CodeGeneratorRequest/Response are in descriptorpb since protoc >= v3.20 uses them via plugin proto.

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
	emitPack := params["emit_pack"] == "true" || params["pack"] == "true"
	emitManifest := params["emit_manifest"] == "true" || params["manifest"] == "true"
	emitFDS := params["emit_fds"] == "true" || params["fds"] == "true" || params["emit_fds"] == ""
	emitUI := params["emit_ui"] != "false" && params["ui"] != "false"

	resp := &pluginpb.CodeGeneratorResponse{}

	// Build a lookup for message types and services in files to generate
	fds := &descriptorpb.FileDescriptorSet{File: req.ProtoFile}
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

	// Manifest and output collections
	type FunctionSpec struct {
		ID          string            `json:"id"`
		Version     string            `json:"version,omitempty"`
		Category    string            `json:"category,omitempty"`
		Labels      map[string]string `json:"labels,omitempty"`
		Request     map[string]any    `json:"request,omitempty"`
		Response    map[string]any    `json:"response,omitempty"`
		Auth        map[string]any    `json:"auth,omitempty"`
		Semantics   map[string]any    `json:"semantics,omitempty"`
		Transport   map[string]any    `json:"transport,omitempty"`
		Routing     map[string]any    `json:"routing,omitempty"`
		UI          map[string]any    `json:"ui,omitempty"`
		DisplayName map[string]string `json:"display_name,omitempty"`
		Summary     map[string]string `json:"summary,omitempty"`
		Tags        []string          `json:"tags,omitempty"`
		Menu        map[string]any    `json:"menu,omitempty"`
		Permissions map[string]any    `json:"permissions,omitempty"`
	}

	type EntitySpec struct {
		ID          string            `json:"id"`
		Version     string            `json:"version,omitempty"`
		Name        string            `json:"name,omitempty"`
		Description string            `json:"description,omitempty"`
		Category    string            `json:"category,omitempty"`
		DisplayName map[string]string `json:"display_name,omitempty"`
		Summary     map[string]string `json:"summary,omitempty"`
		Tags        []string          `json:"tags,omitempty"`
		Menu        map[string]any    `json:"menu,omitempty"`
	}

	manifest := map[string]any{}
	manifestFunctions := make([]FunctionSpec, 0)
	manifestEntities := make([]EntitySpec, 0)

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
				// Derive function spec (basic defaults; custom options TODO)
				funID := defaultFunctionID(pkg, svc.GetName(), m.GetName())
				category := defaultCategory(pkg)
				version := "1.0.0"

				// Transport info
				inType := strings.TrimPrefix(m.GetInputType(), ".")
				outType := strings.TrimPrefix(m.GetOutputType(), ".")

				// Parse method-level custom options
				fo := parseFunctionOptions(m.GetOptions())

				// Make descriptor JSON (apply defaults, then override by options)
				desc := map[string]any{
					"id":       funID,
					"version":  version,
					"category": category,
					"transport": map[string]any{
						"request_type": "proto",
						"proto": map[string]any{
							"request_fqn":  inType,
							"response_fqn": outType,
							"encoding":     "pb-json",
						},
					},
					"semantics": map[string]any{
						"mode":            "query",
						"idempotency_key": false,
						"timeout":         "30s",
						"route":           "lb",
					},
					"auth": map[string]any{
						"permission":      funID,
						"two_person_rule": false,
					},
					"placement": "agent",
					"outputs": map[string]any{
						"views": []any{
							map[string]any{
								"id":       "json",
								"type":     "json",
								"renderer": "json.view",
							},
						},
					},
				}

				// Apply overrides from function options
				if fo.FunctionID != "" {
					desc["id"] = fo.FunctionID
					funID = fo.FunctionID
				}
				if fo.Version != "" {
					desc["version"] = fo.Version
					version = fo.Version
				}
				if fo.Category != "" {
					desc["category"] = fo.Category
				}
				if fo.Timeout != "" {
					desc["semantics"].(map[string]any)["timeout"] = fo.Timeout
				}
				if fo.Route != "" {
					desc["semantics"].(map[string]any)["route"] = strings.ToLower(fo.Route)
				}
				if fo.TwoPersonRuleSet {
					desc["auth"].(map[string]any)["two_person_rule"] = fo.TwoPersonRule
				}
				if fo.Placement != "" {
					desc["placement"] = strings.ToLower(fo.Placement)
				}
				if fo.Risk != "" {
					desc["risk"] = strings.ToLower(fo.Risk)
				}
				if fo.Mode != "" {
					desc["semantics"].(map[string]any)["mode"] = strings.ToLower(fo.Mode)
				}
				if fo.IdempotencyKeySet {
					desc["semantics"].(map[string]any)["idempotency_key"] = fo.IdempotencyKey
				}
				if len(fo.Labels) > 0 {
					desc["labels"] = fo.Labels
				}
				// --- UI/RBAC bridging from proto options into manifest ---
				if len(fo.DisplayName) > 0 {
					desc["display_name"] = fo.DisplayName
				}
				if len(fo.Summary) > 0 {
					desc["summary"] = fo.Summary
				}
				if len(fo.Tags) > 0 {
					desc["tags"] = fo.Tags
				}
				if len(fo.Menu) > 0 {
					desc["menu"] = fo.Menu
				}
				if len(fo.Permissions) > 0 {
					desc["permissions"] = fo.Permissions
				}

				// UI schema for input (with field-level UI options if any)
				if emitUI {
					if inMsg := globalMsgIndex[m.GetInputType()]; inMsg != nil {
						uiHints := collectUIFieldHints(inMsg)
						schema := buildJSONSchema(pkg, globalMsgIndex, globalEnumIndex, inMsg)
						uiSchema := buildUISchema(schema, uiHints)
						// Attach sensitive fields into descriptor (for audit masking)
						if len(uiHints.Sensitive) > 0 {
							desc["ui"] = map[string]any{"sensitive": uiHints.Sensitive}
						}
						addJSON(resp, &generatedFiles, filepath.Join("ui", sanitize(funID)+".schema.json"), schema)
						addJSON(resp, &generatedFiles, filepath.Join("ui", sanitize(funID)+".uischema.json"), uiSchema)
					}
				}
				addJSON(resp, &generatedFiles, filepath.Join("descriptors", sanitize(funID)+".json"), desc)

				fnSpec := FunctionSpec{ID: funID, Version: version, Category: category, Labels: fo.Labels}
				if len(fo.DisplayName) > 0 {
					fnSpec.DisplayName = fo.DisplayName
				}
				if len(fo.Summary) > 0 {
					fnSpec.Summary = fo.Summary
				}
				if len(fo.Tags) > 0 {
					fnSpec.Tags = fo.Tags
				}
				if len(fo.Menu) > 0 {
					fnSpec.Menu = fo.Menu
				}
				if len(fo.Permissions) > 0 {
					fnSpec.Permissions = fo.Permissions
				}

				if emitManifest {
					reqSchema := schemaFileForFQN(inType)
					respSchema := schemaFileForFQN(outType)
					fnSpec.Request = map[string]any{"proto_fqn": inType}
					fnSpec.Response = map[string]any{"proto_fqn": outType}
					fnSpec.Transport = map[string]any{"proto": map[string]any{"request_fqn": inType, "response_fqn": outType}}
					fnSpec.Semantics = map[string]any{"idempotent": fo.IdempotencyKey}
					fnSpec.Auth = map[string]any{"require": []string{funID}}
					fnSpec.UI = map[string]any{"category": category}
					if fo.Risk != "" {
						fnSpec.UI["risk"] = strings.ToLower(fo.Risk)
					}
					if len(fo.Tags) > 0 {
						fnSpec.UI["tags"] = fo.Tags
					}
					// Emit JSON schema files for request/response types when available; otherwise fall back to proto_fqn.
					if inMsg := globalMsgIndex[m.GetInputType()]; inMsg != nil {
						fnSpec.Request = map[string]any{"json_schema": reqSchema}
						if !emittedSchemas[reqSchema] {
							addJSON(resp, &generatedFiles, reqSchema, buildJSONSchema(pkg, globalMsgIndex, globalEnumIndex, inMsg))
							emittedSchemas[reqSchema] = true
						}
					}
					if outMsg := globalMsgIndex[m.GetOutputType()]; outMsg != nil {
						fnSpec.Response = map[string]any{"json_schema": respSchema}
						if !emittedSchemas[respSchema] {
							addJSON(resp, &generatedFiles, respSchema, buildJSONSchema(pkg, globalMsgIndex, globalEnumIndex, outMsg))
							emittedSchemas[respSchema] = true
						}
					}
				}

				manifestFunctions = append(manifestFunctions, fnSpec)
			}
		}

		// ========== Process entities (messages with (croupier.options.entity) option) ==========
		for _, msg := range fd.GetMessageType() {
			eo := parseEntityOptions(msg.GetOptions())
			if eo.EntityID == "" && !hasEntityExtension(msg.GetOptions()) {
				continue
			}

			// Default entity ID
			entityID := eo.EntityID
			if entityID == "" {
				entityID = defaultEntityID(pkg, msg.GetName())
			}
			version := eo.Version
			if version == "" {
				version = "1.0.0"
			}
			category := eo.Category
			if category == "" {
				category = defaultCategory(pkg)
			}
			name := eo.Name
			if name == "" {
				name = msg.GetName()
			}
			description := eo.Description
			if description == "" {
				description = fmt.Sprintf("%s entity definition", name)
			}

			// Build JSON Schema for the entity
			entitySchema := buildJSONSchema(pkg, globalMsgIndex, globalEnumIndex, msg)

			// Add entity-specific annotations to schema (primary_key, searchable, etc.)
			addEntityHintsToSchema(entitySchema, msg, eo)

			// Build entity descriptor
			entityDesc := map[string]any{
				"id":          entityID,
				"version":     version,
				"name":        name,
				"description": description,
				"type":        "entity",
				"category":    category,
				"schema":      entitySchema,
				"operations":  buildEntityOperations(eo),
				"ui":          buildEntityUI(eo),
			}

			// Add i18n display info
			if len(eo.DisplayName) > 0 {
				entityDesc["display_name"] = eo.DisplayName
			}
			if len(eo.Summary) > 0 {
				entityDesc["summary"] = eo.Summary
			}
			if len(eo.Tags) > 0 {
				entityDesc["tags"] = eo.Tags
			}
			if len(eo.Menu) > 0 {
				entityDesc["menu"] = eo.Menu
			}

			// Emit entity descriptor file
			addJSON(resp, &generatedFiles, filepath.Join("descriptors", sanitize(entityID)+".entity.json"), entityDesc)

			// Emit schema file separately for reuse
			schemaFile := schemaFileForFQN("." + pkg + "." + msg.GetName())
			if !emittedSchemas[schemaFile] {
				addJSON(resp, &generatedFiles, schemaFile, entitySchema)
				emittedSchemas[schemaFile] = true
			}

			// Add to manifest
			entSpec := EntitySpec{
				ID:          entityID,
				Version:     version,
				Name:        name,
				Description: description,
				Category:    category,
			}
			if len(eo.DisplayName) > 0 {
				entSpec.DisplayName = eo.DisplayName
			}
			if len(eo.Summary) > 0 {
				entSpec.Summary = eo.Summary
			}
			if len(eo.Tags) > 0 {
				entSpec.Tags = eo.Tags
			}
			if len(eo.Menu) > 0 {
				entSpec.Menu = eo.Menu
			}
			manifestEntities = append(manifestEntities, entSpec)
		}
	}

	// Emit manifest.json (pack manifest always includes functions; provider meta is optional)
	sort.Slice(manifestFunctions, func(i, j int) bool { return manifestFunctions[i].ID < manifestFunctions[j].ID })
	manifest["functions"] = manifestFunctions

	// Add entities to manifest if any were generated
	if len(manifestEntities) > 0 {
		sort.Slice(manifestEntities, func(i, j int) bool { return manifestEntities[i].ID < manifestEntities[j].ID })
		manifest["entities"] = manifestEntities
	}

	if emitManifest {
		manifest["provider"] = map[string]any{
			"id":          firstNonEmpty(params["provider_id"], params["provider"], params["id"], "provider"),
			"version":     firstNonEmpty(params["provider_version"], params["provider_ver"], params["version"], "1.0.0"),
			"lang":        firstNonEmpty(params["provider_lang"], params["lang"]),
			"sdk":         firstNonEmpty(params["provider_sdk"], params["sdk"]),
			"description": firstNonEmpty(params["provider_description"], params["description"]),
		}
	}
	addJSON(resp, &generatedFiles, "manifest.json", manifest)

	// Emit fds.pb (only filesToGen subset, but include deps to be safe -> full set)
	fdsBytes, _ := proto.Marshal(fds)
	resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String("fds.pb"),
		Content: proto.String(string(fdsBytes)),
	})
	generatedFiles = append(generatedFiles, generatedFile{Name: "fds.pb", Data: fdsBytes})

	if emitManifest && emitFDS {
		resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
			Name:    proto.String("descriptors.fds"),
			Content: proto.String(string(fdsBytes)),
		})
		generatedFiles = append(generatedFiles, generatedFile{Name: "descriptors.fds", Data: fdsBytes})
	}

	// Optionally emit pack.tgz
	if emitPack {
		pack, err := buildPackTarGz(generatedFiles)
		if err != nil {
			fatalf("build pack: %v", err)
		}
		resp.File = append(resp.File, &pluginpb.CodeGeneratorResponse_File{
			Name:    proto.String("pack.tgz"),
			Content: proto.String(string(pack)),
		})
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
			// optional: propertyNames constraints for key type not added (JSON Schema limitation)
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

type uiFieldHints struct {
	Fields    map[string]map[string]any // per-field ui config
	Sensitive []string
}

func buildUISchema(schema map[string]any, hints uiFieldHints) map[string]any {
	// Minimal UI schema with grid layout inferred from properties order
	props, _ := schema["properties"].(map[string]any)
	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	sort.Strings(names)
	groups := []map[string]any{
		{"title": "基本", "fields": names},
	}
	ui := map[string]any{
		"ui:layout": map[string]any{"type": "grid", "cols": 2},
		"ui:groups": groups,
	}
	if len(hints.Fields) > 0 {
		ui["ui:fields"] = hints.Fields
	}
	return ui
}

func defaultFunctionID(pkg, svc, method string) string {
	// default: <pkg>.<service>.<method> in lower snake for method
	id := pkg + "." + svc + "." + method
	// normalize: lower case, dots kept
	id = strings.ReplaceAll(id, " ", "")
	return strings.ToLower(id)
}

func defaultCategory(pkg string) string {
	parts := strings.Split(pkg, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "general"
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

// --- Options parsing (from UninterpretedOption.aggregate_value) ---

type funcOpts struct {
	FunctionID        string
	Version           string
	Category          string
	Risk              string
	Route             string
	Timeout           string
	TwoPersonRule     bool
	TwoPersonRuleSet  bool
	Placement         string
	Mode              string
	IdempotencyKey    bool
	IdempotencyKeySet bool
	Labels            map[string]string
	// Extended UI/RBAC
	DisplayName map[string]string
	Summary     map[string]string
	Tags        []string
	Menu        map[string]any
	Permissions map[string]any
}

func parseFunctionOptions(mo *descriptorpb.MethodOptions) funcOpts {
	var out funcOpts

	if mo == nil {
		return out
	}

	if proto.HasExtension(mo, optionsv1.E_Function) {
		ext := proto.GetExtension(mo, optionsv1.E_Function)
		if fn, ok := ext.(*optionsv1.FunctionOptions); ok && fn != nil {
			out.FunctionID = strings.TrimSpace(fn.GetFunctionId())
			out.Version = strings.TrimSpace(fn.GetVersion())
			out.Category = strings.TrimSpace(fn.GetCategory())
			out.Risk = strings.TrimSpace(fn.GetRisk())
			out.Route = strings.TrimSpace(fn.GetRoute())
			out.Timeout = strings.TrimSpace(fn.GetTimeout())
			out.TwoPersonRule = fn.GetTwoPersonRule()
			out.TwoPersonRuleSet = true
			out.Placement = strings.TrimSpace(fn.GetPlacement())
			out.Mode = strings.TrimSpace(fn.GetMode())
			out.IdempotencyKey = fn.GetIdempotencyKey()
			out.IdempotencyKeySet = true
			if len(fn.GetLabels()) > 0 {
				out.Labels = map[string]string{}
				for k, v := range fn.GetLabels() {
					out.Labels[k] = v
				}
			}
			if dn := i18nToMap(fn.GetDisplayName()); len(dn) > 0 {
				out.DisplayName = dn
			}
			if sm := i18nToMap(fn.GetSummary()); len(sm) > 0 {
				out.Summary = sm
			}
			if len(fn.GetTags()) > 0 {
				out.Tags = append([]string{}, fn.GetTags()...)
			}
			if menu := menuToMap(fn.GetMenu()); len(menu) > 0 {
				out.Menu = menu
			}
			if perms := permissionToMap(fn.GetPermissions()); len(perms) > 0 {
				out.Permissions = perms
			}
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
		if v := kv["category"]; v != "" {
			out.Category = trimQuotes(v)
		}
		if v := kv["risk"]; v != "" {
			out.Risk = trimQuotes(v)
		}
		if v := kv["route"]; v != "" {
			out.Route = trimQuotes(v)
		}
		if v := kv["timeout"]; v != "" {
			out.Timeout = trimQuotes(v)
		}
		if v := kv["two_person_rule"]; v != "" {
			out.TwoPersonRule, out.TwoPersonRuleSet = parseBool(v), true
		}
		if v := kv["placement"]; v != "" {
			out.Placement = trimQuotes(v)
		}
		if v := kv["mode"]; v != "" {
			out.Mode = trimQuotes(v)
		}
		if v := kv["idempotency_key"]; v != "" {
			out.IdempotencyKey, out.IdempotencyKeySet = parseBool(v), true
		}
		if m := parseOptionObjectMap(raw, "labels"); len(m) > 0 {
			out.Labels = m
		}
	}
	return out
}

func collectUIFieldHints(msg *descriptorpb.DescriptorProto) uiFieldHints {
	hints := uiFieldHints{Fields: map[string]map[string]any{}, Sensitive: []string{}}
	for _, f := range msg.GetField() {
		name := f.GetJsonName()
		if name == "" {
			name = f.GetName()
		}
		var fieldCfg map[string]any
		cfg := map[string]any{}
		if fo := f.GetOptions(); fo != nil {
			if proto.HasExtension(fo, optionsv1.E_Ui) {
				ext := proto.GetExtension(fo, optionsv1.E_Ui)
				if ui, ok := ext.(*optionsv1.UIFieldOptions); ok && ui != nil {
					if v := strings.TrimSpace(ui.GetWidget()); v != "" {
						cfg["widget"] = v
					}
					if v := strings.TrimSpace(ui.GetLabel()); v != "" {
						cfg["label"] = v
					}
					if v := strings.TrimSpace(ui.GetPlaceholder()); v != "" {
						cfg["placeholder"] = v
					}
					if v := strings.TrimSpace(ui.GetShowIf()); v != "" {
						cfg["show_if"] = v
					}
					if v := strings.TrimSpace(ui.GetRequiredIf()); v != "" {
						cfg["required_if"] = v
					}
					cfg["sensitive"] = ui.GetSensitive()
					if ui.GetSensitive() {
						hints.Sensitive = append(hints.Sensitive, name)
					}
					if len(ui.GetEnumMap()) > 0 {
						values := make([]string, 0, len(ui.GetEnumMap()))
						labels := map[string]string{}
						for k, v := range ui.GetEnumMap() {
							values = append(values, k)
							labels[k] = v
						}
						sort.Strings(values)
						cfg["enum"] = values
						cfg["x-enum-labels"] = labels
					}
				}
			} else {
				// Fallback: parse from UninterpretedOption aggregate_value
				for _, u := range fo.GetUninterpretedOption() {
					if joinOptionName(u) != "croupier.options.v1.ui" {
						continue
					}
					raw := u.GetAggregateValue()
					kv := parseAggregateKV(raw)
					if v := kv["widget"]; v != "" {
						cfg["widget"] = trimQuotes(v)
					}
					if v := kv["label"]; v != "" {
						cfg["label"] = trimQuotes(v)
					}
					if v := kv["placeholder"]; v != "" {
						cfg["placeholder"] = trimQuotes(v)
					}
					if v := kv["show_if"]; v != "" {
						cfg["show_if"] = trimQuotes(v)
					}
					if v := kv["required_if"]; v != "" {
						cfg["required_if"] = trimQuotes(v)
					}
					if v := kv["sensitive"]; v != "" {
						b := parseBool(v)
						cfg["sensitive"] = b
						if b {
							hints.Sensitive = append(hints.Sensitive, name)
						}
					}
					if m := parseOptionObjectMap(raw, "enum_map"); len(m) > 0 {
						values := make([]string, 0, len(m))
						labels := map[string]string{}
						for k, v := range m {
							values = append(values, k)
							labels[k] = v
						}
						sort.Strings(values)
						cfg["enum"] = values
						cfg["x-enum-labels"] = labels
					}
				}
			}
		}
		if len(cfg) > 0 {
			fieldCfg = cfg
		}
		if fieldCfg != nil {
			hints.Fields[name] = fieldCfg
		}
	}
	return hints
}

func i18nToMap(t *commonv1.I18NText) map[string]string {
	if t == nil {
		return nil
	}
	out := map[string]string{}
	if v := strings.TrimSpace(t.GetEn()); v != "" {
		out["en"] = v
	}
	if v := strings.TrimSpace(t.GetZh()); v != "" {
		out["zh"] = v
	}
	return out
}

func menuToMap(m *commonv1.Menu) map[string]any {
	if m == nil {
		return nil
	}
	out := map[string]any{}
	if v := strings.TrimSpace(m.GetSection()); v != "" {
		out["section"] = v
	}
	if v := strings.TrimSpace(m.GetGroup()); v != "" {
		out["group"] = v
	}
	if v := strings.TrimSpace(m.GetPath()); v != "" {
		out["path"] = v
	}
	if m.GetOrder() != 0 {
		out["order"] = int(m.GetOrder())
	}
	if v := strings.TrimSpace(m.GetIcon()); v != "" {
		out["icon"] = v
	}
	if v := strings.TrimSpace(m.GetBadge()); v != "" {
		out["badge"] = v
	}
	if m.GetHidden() {
		out["hidden"] = true
	}
	return out
}

func permissionToMap(p *commonv1.PermissionSpec) map[string]any {
	if p == nil {
		return nil
	}
	out := map[string]any{}
	if len(p.GetVerbs()) > 0 {
		out["verbs"] = append([]string{}, p.GetVerbs()...)
	}
	if len(p.GetScopes()) > 0 {
		out["scopes"] = append([]string{}, p.GetScopes()...)
	}
	if len(p.GetDefaults()) > 0 {
		defs := make([]map[string]any, 0, len(p.GetDefaults()))
		for _, rb := range p.GetDefaults() {
			if rb == nil || rb.GetRole() == "" || len(rb.GetVerbs()) == 0 {
				continue
			}
			defs = append(defs, map[string]any{"role": rb.GetRole(), "verbs": append([]string{}, rb.GetVerbs()...)})
		}
		if len(defs) > 0 {
			out["defaults"] = defs
		}
	}
	if len(p.GetI18NZh()) > 0 {
		m := map[string]string{}
		for k, v := range p.GetI18NZh() {
			m[k] = v
		}
		out["i18n_zh"] = m
	}
	return out
}

func schemaFileForFQN(fqn string) string {
	fqn = strings.TrimPrefix(strings.TrimSpace(fqn), ".")
	if fqn == "" {
		return "schema/unknown.json"
	}
	// Lowercase to keep filenames stable; caller must use the same when referencing.
	return filepath.ToSlash(filepath.Join("schema", sanitize(strings.ToLower(fqn))+".json"))
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

// parseOptionObjectMap extracts a simple object map for a given field name inside an aggregate_value string.
// Example: fieldName: { k1: "v1", k2: "v2" }
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

// parseOptionArray extracts a simple string array for "field: [a,b,c]" or "field: \"a,b,c\"" into []string
func parseOptionArray(s, field string) []string {
	out := []string{}
	if s == "" || field == "" {
		return out
	}
	i := 0
	for i < len(s) {
		idx := strings.Index(s[i:], field)
		if idx < 0 {
			break
		}
		i += idx + len(field)
		// skip spaces
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
		if i >= len(s) || s[i] != ':' {
			continue
		}
		i++ // skip colon
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
		if i >= len(s) {
			break
		}
		if s[i] == '[' {
			// parse until matching ]
			i++
			start := i
			depth := 1
			for i < len(s) && depth > 0 {
				if s[i] == '[' {
					depth++
				} else if s[i] == ']' {
					depth--
					if depth == 0 {
						break
					}
				}
				i++
			}
			end := i
			if end > start {
				inside := s[start:end]
				parts := strings.Split(inside, ",")
				for _, p := range parts {
					p = strings.TrimSpace(strings.Trim(p, "\"'"))
					if p != "" {
						out = append(out, p)
					}
				}
			}
			break
		} else if s[i] == '"' {
			// quoted csv
			i++
			start := i
			for i < len(s) && s[i] != '"' {
				i++
			}
			if i > start {
				inside := s[start:i]
				parts := strings.Split(inside, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						out = append(out, p)
					}
				}
			}
			break
		} else {
			// plain token or csv until comma/newline/}
			start := i
			for i < len(s) && s[i] != ',' && s[i] != '\n' && s[i] != '}' {
				i++
			}
			token := strings.TrimSpace(s[start:i])
			if token != "" {
				out = append(out, token)
			}
			break
		}
	}
	return out
}

func trimQuotes(s string) string {
	return strings.Trim(s, "\"")
}

// parseOptionI18n extracts an i18n message field like display_name { en: "xxx" zh: "xxx" }
func parseOptionI18n(s, field string) map[string]string {
	out := map[string]string{}
	if s == "" || field == "" {
		return out
	}
	fieldIdx := strings.Index(s, field+":")
	if fieldIdx < 0 {
		return out
	}
	i := fieldIdx + len(field)
	// skip spaces
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i >= len(s) || s[i] != ':' {
		return out
	}
	i++ // skip colon
	// skip spaces
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	if i >= len(s) || s[i] != '{' {
		return out
	}
	i++ // skip {
	// Find matching }
	depth := 1
	start := i
	for i < len(s) && depth > 0 {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				break
			}
		}
		i++
	}
	inside := s[start:i]
	// Parse key: value pairs
	for _, line := range strings.Split(inside, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(strings.Trim(parts[1], "\"'"))
			if key != "" && val != "" {
				out[key] = val
			}
		}
	}
	if len(out) > 0 {
		return out
	}
	return out
}

// parseOptionMenu extracts a menu nested message like menu { section: "xxx" group: "yyy" hidden: true }
func parseOptionMenu(s string) map[string]any {
	out := map[string]any{}
	if s == "" {
		return out
	}
	menuIdx := strings.Index(s, "menu:")
	if menuIdx < 0 {
		return out
	}
	i := menuIdx + len("menu")
	// skip spaces
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i >= len(s) || s[i] != ':' {
		return out
	}
	i++ // skip colon
	// skip spaces
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	if i >= len(s) || s[i] != '{' {
		return out
	}
	i++ // skip {
	// Find matching }
	depth := 1
	start := i
	for i < len(s) && depth > 0 {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				break
			}
		}
		i++
	}
	inside := s[start:i]
	// Parse key: value pairs
	kv := parseAggregateKV(inside)
	if v, ok := kv["section"]; ok && v != "" {
		out["section"] = trimQuotes(v)
	}
	if v, ok := kv["group"]; ok && v != "" {
		out["group"] = trimQuotes(v)
	}
	if v, ok := kv["path"]; ok && v != "" {
		out["path"] = trimQuotes(v)
	}
	if v, ok := kv["icon"]; ok && v != "" {
		out["icon"] = trimQuotes(v)
	}
	if v, ok := kv["badge"]; ok && v != "" {
		out["badge"] = trimQuotes(v)
	}
	if v, ok := kv["order"]; ok && v != "" {
		// Try to parse as int
		if order, err := parseOrderInt(v); err == nil {
			out["order"] = order
		}
	}
	if v, ok := kv["hidden"]; ok && v != "" {
		out["hidden"] = parseBool(v)
	}
	if len(out) > 0 {
		return out
	}
	return out
}

// parseOrderInt tries to parse order value as int
func parseOrderInt(s string) (int, error) {
	s = strings.TrimSpace(trimQuotes(s))
	var order int
	_, err := fmt.Sscanf(s, "%d", &order)
	return order, err
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(trimQuotes(s)))
	return s == "true" || s == "1" || s == "yes"
}

func parseIntMaybe(s string) int {
	ss := strings.TrimSpace(trimQuotes(s))
	n := 0
	for i := 0; i < len(ss); i++ {
		if ss[i] < '0' || ss[i] > '9' {
			return 0
		}
		n = n*10 + int(ss[i]-'0')
	}
	return n
}

func buildPackTarGz(files []generatedFile) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		hdr := &tar.Header{Name: filepath.ToSlash(f.Name), Mode: 0644, Size: int64(len(f.Data))}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(f.Data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

// ========== Entity helpers ==========

// entityOptions holds parsed entity options from proto extensions
type entityOptions struct {
	EntityID         string
	Version          string
	Name             string
	Description      string
	Category         string
	PrimaryKey       string
	DisplayField     string
	TitleTemplate    string
	AvatarField      string
	StatusField      string
	CreateFunctions  []string
	ReadFunctions    []string
	UpdateFunctions  []string
	DeleteFunctions  []string
	ListFunctions    []string
	CustomOperations map[string]string
	DisplayName      map[string]string
	Summary          map[string]string
	Tags             []string
	Menu             map[string]any
}

// parseEntityOptions parses entity options from message options
func parseEntityOptions(mo *descriptorpb.MessageOptions) entityOptions {
	var out entityOptions

	if mo == nil {
		return out
	}

	// For now, only parse from UninterpretedOption (proto-first workflow)
	// After the EntityOptions extension is generated and available in optionsv1,
	// we can add a check using proto.HasExtension(mo, optionsv1.E_Entity)
	for _, u := range mo.GetUninterpretedOption() {
		name := joinOptionName(u)
		if name != "croupier.options.v1.entity" {
			continue
		}
		raw := u.GetAggregateValue()
		kv := parseAggregateKV(raw)
		if v := kv["entity_id"]; v != "" {
			out.EntityID = trimQuotes(v)
		}
		if v := kv["version"]; v != "" {
			out.Version = trimQuotes(v)
		}
		if v := kv["name"]; v != "" {
			out.Name = trimQuotes(v)
		}
		if v := kv["description"]; v != "" {
			out.Description = trimQuotes(v)
		}
		if v := kv["category"]; v != "" {
			out.Category = trimQuotes(v)
		}
		if v := kv["primary_key"]; v != "" {
			out.PrimaryKey = trimQuotes(v)
		}
		if v := kv["display_field"]; v != "" {
			out.DisplayField = trimQuotes(v)
		}
		if v := kv["title_template"]; v != "" {
			out.TitleTemplate = trimQuotes(v)
		}
		if v := kv["avatar_field"]; v != "" {
			out.AvatarField = trimQuotes(v)
		}
		if v := kv["status_field"]; v != "" {
			out.StatusField = trimQuotes(v)
		}
		// Parse function arrays
		if arr := parseOptionArray(raw, "create_functions"); len(arr) > 0 {
			out.CreateFunctions = arr
		}
		if arr := parseOptionArray(raw, "read_functions"); len(arr) > 0 {
			out.ReadFunctions = arr
		}
		if arr := parseOptionArray(raw, "update_functions"); len(arr) > 0 {
			out.UpdateFunctions = arr
		}
		if arr := parseOptionArray(raw, "delete_functions"); len(arr) > 0 {
			out.DeleteFunctions = arr
		}
		if arr := parseOptionArray(raw, "list_functions"); len(arr) > 0 {
			out.ListFunctions = arr
		}
		if m := parseOptionObjectMap(raw, "custom_operations"); len(m) > 0 {
			out.CustomOperations = m
		}
		// Parse tags array
		if arr := parseOptionArray(raw, "tags"); len(arr) > 0 {
			out.Tags = arr
		}
		// Parse menu nested message
		if m := parseOptionMenu(raw); len(m) > 0 {
			out.Menu = m
		}
		// Parse display_name and summary (i18n)
		if dm := parseOptionI18n(raw, "display_name"); len(dm) > 0 {
			out.DisplayName = dm
		}
		if sm := parseOptionI18n(raw, "summary"); len(sm) > 0 {
			out.Summary = sm
		}
	}

	return out
}

// hasEntityExtension checks if message has entity extension
func hasEntityExtension(mo *descriptorpb.MessageOptions) bool {
	if mo == nil {
		return false
	}
	// Check uninterpreted options for entity extension
	for _, u := range mo.GetUninterpretedOption() {
		if joinOptionName(u) == "croupier.options.v1.entity" {
			return true
		}
	}
	return false
}

// defaultEntityID generates default entity ID from package and message name
func defaultEntityID(pkg, msg string) string {
	id := pkg + "." + msg
	id = strings.ReplaceAll(id, " ", "")
	return strings.ToLower(id)
}

// addEntityHintsToSchema adds entity-specific hints to JSON Schema
func addEntityHintsToSchema(schema map[string]any, msg *descriptorpb.DescriptorProto, eo entityOptions) {
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}

	// Determine primary key
	primaryKey := eo.PrimaryKey
	if primaryKey == "" {
		// Default to first string field
		for _, f := range msg.GetField() {
			if f.GetType() == descriptorpb.FieldDescriptorProto_TYPE_STRING {
				primaryKey = f.GetJsonName()
				if primaryKey == "" {
					primaryKey = f.GetName()
				}
				break
			}
		}
	}

	// Add hints to each property
	for _, f := range msg.GetField() {
		name := f.GetJsonName()
		if name == "" {
			name = f.GetName()
		}
		prop, ok := props[name]
		if !ok {
			continue
		}
		propMap, ok := prop.(map[string]any)
		if !ok {
			continue
		}

		// Mark primary key
		if name == primaryKey {
			propMap["primary_key"] = true
		}

		// Check for field-level entity hints via custom options
		if fo := f.GetOptions(); fo != nil {
			// Check UI hints for searchable/filterable/sortable
			if proto.HasExtension(fo, optionsv1.E_Ui) {
				ext := proto.GetExtension(fo, optionsv1.E_Ui)
				if ui, ok := ext.(*optionsv1.UIFieldOptions); ok && ui != nil {
					// These can be added as custom annotations
					if strings.TrimSpace(ui.GetPlaceholder()) != "" {
						propMap["placeholder"] = strings.TrimSpace(ui.GetPlaceholder())
					}
				}
			}
		}
	}
}

// buildEntityOperations builds operations mapping from entity options
func buildEntityOperations(eo entityOptions) map[string]any {
	ops := map[string]any{}

	if len(eo.CreateFunctions) > 0 {
		ops["create"] = eo.CreateFunctions
	}
	if len(eo.ReadFunctions) > 0 {
		ops["read"] = eo.ReadFunctions
	}
	if len(eo.UpdateFunctions) > 0 {
		ops["update"] = eo.UpdateFunctions
	}
	if len(eo.DeleteFunctions) > 0 {
		ops["delete"] = eo.DeleteFunctions
	}
	if len(eo.ListFunctions) > 0 {
		ops["list"] = eo.ListFunctions
	}

	// Add custom operations
	for k, v := range eo.CustomOperations {
		if v != "" {
			ops[k] = []string{v}
		}
	}

	return ops
}

// buildEntityUI builds UI configuration from entity options
func buildEntityUI(eo entityOptions) map[string]any {
	ui := map[string]any{}

	if eo.DisplayField != "" {
		ui["display_field"] = eo.DisplayField
	}
	if eo.TitleTemplate != "" {
		ui["title_template"] = eo.TitleTemplate
	}
	if eo.AvatarField != "" {
		ui["avatar_field"] = eo.AvatarField
	}
	if eo.StatusField != "" {
		ui["status_field"] = eo.StatusField
	}

	return ui
}
