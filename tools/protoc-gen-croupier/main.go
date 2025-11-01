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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

// CodeGeneratorRequest/Response are in descriptorpb since protoc >= v3.20 uses them via plugin proto.

func main() {
	// Read request from stdin
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatalf("read stdin: %v", err)
	}
	var req descriptorpb.CodeGeneratorRequest
	if err := proto.Unmarshal(in, &req); err != nil {
		fatalf("unmarshal CodeGeneratorRequest: %v", err)
	}

	// Defaults and params
	params := parseParams(req.GetParameter())
	emitPack := params["emit_pack"] == "true" || params["pack"] == "true"

	resp := &descriptorpb.CodeGeneratorResponse{}

	// Build a lookup for message types and services in files to generate
	fds := &descriptorpb.FileDescriptorSet{File: req.ProtoFile}
	filesToGen := make(map[string]bool)
	for _, f := range req.GetFileToGenerate() {
		filesToGen[f] = true
	}

	// Manifest and output collections
	type FunctionSpec struct {
		ID       string            `json:"id"`
		Version  string            `json:"version"`
		Category string            `json:"category,omitempty"`
		Labels   map[string]string `json:"labels,omitempty"`
	}
	manifest := struct {
		Functions []FunctionSpec `json:"functions"`
	}{}

	var generatedFiles []generatedFile

	// Iterate files
	for _, fd := range req.GetProtoFile() {
		if !filesToGen[fd.GetName()] {
			continue
		}
		pkg := fd.GetPackage()
		// Index messages by FQN for JSON schema mapping
		msgIndex := indexMessages(fd)

		for _, svc := range fd.GetService() {
			for _, m := range svc.GetMethod() {
				// Derive function spec (basic defaults; custom options TODO)
				funID := defaultFunctionID(pkg, svc.GetName(), m.GetName())
				category := defaultCategory(pkg)
				version := "1.0.0"

				// Transport info
				inType := strings.TrimPrefix(m.GetInputType(), ".")
				outType := strings.TrimPrefix(m.GetOutputType(), ".")

				// Make descriptor JSON
				desc := map[string]any{
					"id":      funID,
					"version": version,
					"category": category,
					"transport": map[string]any{
						"request_type": "proto",
						"proto": map[string]any{
							"request_fqn": inType,
							"response_fqn": outType,
							"encoding":     "pb-json",
						},
					},
					"semantics": map[string]any{
						"mode":           "query",
						"idempotency_key": false,
						"timeout":        "30s",
						"route":          "lb",
					},
					"auth": map[string]any{
						"permission":     funID,
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
				// JSON schema for input
				if inMsg := msgIndex[m.GetInputType()]; inMsg != nil {
					schema := buildJSONSchema(pkg, msgIndex, inMsg)
					uiSchema := buildUISchema(schema)
					addJSON(resp, &generatedFiles, filepath.Join("ui", sanitize(funID)+".schema.json"), schema)
					addJSON(resp, &generatedFiles, filepath.Join("ui", sanitize(funID)+".uischema.json"), uiSchema)
				}
				addJSON(resp, &generatedFiles, filepath.Join("descriptors", sanitize(funID)+".json"), desc)

				manifest.Functions = append(manifest.Functions, FunctionSpec{ID: funID, Version: version, Category: category})
			}
		}
	}

	// Emit manifest.json
	addJSON(resp, &generatedFiles, "manifest.json", manifest)

	// Emit fds.pb (only filesToGen subset, but include deps to be safe -> full set)
	fdsBytes, _ := proto.Marshal(fds)
	resp.File = append(resp.File, &descriptorpb.CodeGeneratorResponse_File{
		Name:    proto.String("fds.pb"),
		Content: proto.String(string(fdsBytes)),
	})
	generatedFiles = append(generatedFiles, generatedFile{Name: "fds.pb", Data: fdsBytes})

	// Optionally emit pack.tgz
	if emitPack {
		pack, err := buildPackTarGz(generatedFiles)
		if err != nil {
			fatalf("build pack: %v", err)
		}
		resp.File = append(resp.File, &descriptorpb.CodeGeneratorResponse_File{
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

type generatedFile struct{ Name string; Data []byte }

func addJSON(resp *descriptorpb.CodeGeneratorResponse, files *[]generatedFile, name string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	resp.File = append(resp.File, &descriptorpb.CodeGeneratorResponse_File{ Name: proto.String(name), Content: proto.String(string(b)) })
	*files = append(*files, generatedFile{Name: name, Data: b})
}

func parseParams(p string) map[string]string {
	res := map[string]string{}
	for _, kv := range strings.Split(p, ",") {
		if kv == "" { continue }
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 1 { res[parts[0]] = "true"; continue }
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
			if len(m.NestedType) > 0 { walk(fqn, m.NestedType) }
		}
	}
	pkg := fd.GetPackage()
	walk(pkg, fd.GetMessageType())
	return idx
}

func buildJSONSchema(pkg string, idx map[string]*descriptorpb.DescriptorProto, m *descriptorpb.DescriptorProto) map[string]any {
	schema := map[string]any{
		"$schema":  "https://json-schema.org/draft/2020-12/schema",
		"type":     "object",
		"title":    m.GetName(),
		"properties": map[string]any{},
	}
	props := schema["properties"].(map[string]any)
	var required []string
	for _, f := range m.GetField() {
		name := f.GetJsonName()
		if name == "" { name = f.GetName() }
		typ, req := fieldToJSONSchema(pkg, idx, f)
		props[name] = typ
		if req { required = append(required, name) }
	}
	if len(required) > 0 { schema["required"] = required }
	return schema
}

func fieldToJSONSchema(pkg string, idx map[string]*descriptorpb.DescriptorProto, f *descriptorpb.FieldDescriptorProto) (map[string]any, bool) {
	required := false
	switch f.GetLabel() {
	case descriptorpb.FieldDescriptorProto_LABEL_REQUIRED:
		required = true
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
		return basic("string"), required
	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE:
		// Map or nested message
		t := f.GetTypeName()
		if strings.HasPrefix(t, ".") { t = t }
		// Detect google.protobuf.Timestamp/Duration → strings with format
		if t == ".google.protobuf.Timestamp" {
			return map[string]any{"type": "string", "format": "date-time"}, required
		}
		if t == ".google.protobuf.Duration" {
			return map[string]any{"type": "string", "pattern": "^\\d+[smhd]$"}, required
		}
		// Map type
		if f.GetTypeName() == "" && f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			// Should not happen since map has message type; keep fallback
			return map[string]any{"type": "array", "items": map[string]any{"type": "object"}}, required
		}
		// Repeated message as array
		if f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			item := map[string]any{"type": "object"}
			if sub := idx[f.GetTypeName()]; sub != nil {
				item = buildJSONSchema(pkg, idx, sub)
			}
			return map[string]any{"type": "array", "items": item}, required
		}
		// Nested object
		sub := idx[f.GetTypeName()]
		if sub != nil {
			return buildJSONSchema(pkg, idx, sub), required
		}
		return map[string]any{"type": "object"}, required
	default:
		return basic("string"), required
	}
}

func buildUISchema(schema map[string]any) map[string]any {
	// Minimal UI schema with grid layout inferred from properties order
	props, _ := schema["properties"].(map[string]any)
	names := make([]string, 0, len(props))
	for k := range props { names = append(names, k) }
	sort.Strings(names)
	groups := []map[string]any{
		{"title": "基本", "fields": names},
	}
	return map[string]any{
		"ui:layout": map[string]any{"type": "grid", "cols": 2},
		"ui:groups": groups,
	}
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
	if len(parts) >= 2 { return parts[len(parts)-2] }
	if len(parts) == 1 { return parts[0] }
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

func buildPackTarGz(files []generatedFile) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range files {
		hdr := &tar.Header{Name: filepath.ToSlash(f.Name), Mode: 0644, Size: int64(len(f.Data))}
		if err := tw.WriteHeader(hdr); err != nil { return nil, err }
		if _, err := tw.Write(f.Data); err != nil { return nil, err }
	}
	if err := tw.Close(); err != nil { return nil, err }
	if err := gz.Close(); err != nil { return nil, err }
	return buf.Bytes(), nil
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
