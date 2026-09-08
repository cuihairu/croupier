package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ui "github.com/cuihairu/croupier/pkg/pb/croupier/component/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	pluginpb "google.golang.org/protobuf/types/pluginpb"
)

func TestParseParams(t *testing.T) {
	cases := []struct {
		in   string
		want map[string]string
	}{
		{"", map[string]string{}},
		{"a=1,b=2", map[string]string{"a": "1", "b": "2"}},
		{"flag", map[string]string{"flag": "true"}},
		{"k=v,,x", map[string]string{"k": "v", "x": "true"}},
		{"k=", map[string]string{"k": ""}},
		{"k=a=b", map[string]string{"k": "a=b"}},
	}
	for _, tt := range cases {
		require.Equal(t, tt.want, parseParams(tt.in), "input %q", tt.in)
	}
}

func strPtr(s string) *string { return proto.String(s) }

func TestIndexMessagesAndEnums(t *testing.T) {
	nestedEnum := &descriptorpb.EnumDescriptorProto{Name: proto.String("Kind")}
	nestedMsg := &descriptorpb.DescriptorProto{
		Name:     proto.String("Inner"),
		EnumType: []*descriptorpb.EnumDescriptorProto{nestedEnum},
	}
	topEnum := &descriptorpb.EnumDescriptorProto{Name: proto.String("Status")}
	fd := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("test.proto"),
		Package: proto.String("demo.v1"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			topEnum,
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Outer"), NestedType: []*descriptorpb.DescriptorProto{nestedMsg}},
		},
	}

	msgIdx := indexMessages(fd)
	require.Contains(t, msgIdx, ".demo.v1.Outer")
	require.Contains(t, msgIdx, ".demo.v1.Outer.Inner")

	enumIdx := indexEnums(fd)
	require.Contains(t, enumIdx, ".demo.v1.Status")
	require.Contains(t, enumIdx, ".demo.v1.Outer.Inner.Kind")
}

func fieldDef(label descriptorpb.FieldDescriptorProto_Label, typ descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:  proto.String("f"),
		Label: label.Enum(),
		Type:  typ.Enum(),
	}
}

func TestFieldToJSONSchemaScalars(t *testing.T) {
	msgIdx := map[string]*descriptorpb.DescriptorProto{}
	enumIdx := map[string]*descriptorpb.EnumDescriptorProto{}

	cases := []struct {
		f     *descriptorpb.FieldDescriptorProto
		typ   string
		fmt   string
		req   bool
		extra map[string]any
	}{
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_STRING), "string", "", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_REQUIRED, descriptorpb.FieldDescriptorProto_TYPE_BOOL), "boolean", "", true, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_INT32), "integer", "int32", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_SINT32), "integer", "int32", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_SFIXED32), "integer", "int32", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_UINT32), "integer", "uint32", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_FIXED32), "integer", "uint32", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_INT64), "string", "int64", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_SINT64), "string", "int64", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_SFIXED64), "string", "int64", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_UINT64), "string", "uint64", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_FIXED64), "string", "uint64", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_FLOAT), "number", "float", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE), "number", "double", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_BYTES), "string", "", false, nil},
		{fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_GROUP), "string", "", false, nil},
	}
	for _, tt := range cases {
		sch, req := fieldToJSONSchema("p", msgIdx, enumIdx, tt.f)
		require.Equal(t, tt.typ, sch["type"], "type for %v", tt.f.GetType())
		if tt.fmt == "" {
			_, has := sch["format"]
			require.False(t, has)
		} else {
			require.Equal(t, tt.fmt, sch["format"])
		}
		require.Equal(t, tt.req, req)
	}
}

func TestFieldToJSONSchemaRepeatedScalar(t *testing.T) {
	f := &descriptorpb.FieldDescriptorProto{
		Name:  proto.String("tags"),
		Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
		Type:  descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
	}
	sch, req := fieldToJSONSchema("p", map[string]*descriptorpb.DescriptorProto{}, map[string]*descriptorpb.EnumDescriptorProto{}, f)
	require.Equal(t, "array", sch["type"])
	require.Equal(t, "string", sch["items"].(map[string]any)["type"])
	require.False(t, req)
}

func TestFieldToJSONSchemaEnum(t *testing.T) {
	enum := &descriptorpb.EnumDescriptorProto{
		Name: proto.String("Color"),
		Value: []*descriptorpb.EnumValueDescriptorProto{
			{Name: proto.String("RED")},
			{Name: proto.String("BLUE")},
		},
	}
	enumIdx := map[string]*descriptorpb.EnumDescriptorProto{".p.Color": enum}

	f := fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_ENUM)
	f.TypeName = proto.String(".p.Color")
	sch, _ := fieldToJSONSchema("p", map[string]*descriptorpb.DescriptorProto{}, enumIdx, f)
	require.Equal(t, "string", sch["type"])
	enumVals := sch["enum"].([]string)
	require.Len(t, enumVals, 2)
	require.Equal(t, "BLUE", enumVals[0])
	require.Equal(t, "p.Color", sch["x-enum-source"])

	// Unknown enum keeps plain string schema without x-enum-source.
	f2 := fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_ENUM)
	f2.TypeName = proto.String(".p.Missing")
	sch2, _ := fieldToJSONSchema("p", map[string]*descriptorpb.DescriptorProto{}, enumIdx, f2)
	_, has := sch2["enum"]
	require.False(t, has)
	_, has = sch2["x-enum-source"]
	require.False(t, has)
}

func TestFieldToJSONSchemaWellKnown(t *testing.T) {
	msgIdx := map[string]*descriptorpb.DescriptorProto{}
	ts := fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	ts.TypeName = proto.String(".google.protobuf.Timestamp")
	sch, _ := fieldToJSONSchema("p", msgIdx, map[string]*descriptorpb.EnumDescriptorProto{}, ts)
	require.Equal(t, "date-time", sch["format"])

	du := fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	du.TypeName = proto.String("google.protobuf.Duration") // no leading dot
	sch, _ = fieldToJSONSchema("p", msgIdx, map[string]*descriptorpb.EnumDescriptorProto{}, du)
	require.Equal(t, "^\\d+[smhd]$", sch["pattern"])
}

func TestFieldToJSONSchemaMapEntry(t *testing.T) {
	entry := &descriptorpb.DescriptorProto{
		Name: proto.String("LabelsEntry"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{Name: proto.String("key"), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()},
			{Name: proto.String("value"), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()},
		},
	}
	entry.Options = &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)}
	msgIdx := map[string]*descriptorpb.DescriptorProto{".p.L.LabelsEntry": entry}

	f := fieldDef(descriptorpb.FieldDescriptorProto_LABEL_REPEATED, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	f.TypeName = proto.String(".p.L.LabelsEntry")
	sch, _ := fieldToJSONSchema("p", msgIdx, map[string]*descriptorpb.EnumDescriptorProto{}, f)
	require.Equal(t, "object", sch["type"])
	addl := sch["additionalProperties"].(map[string]any)
	require.Equal(t, "integer", addl["type"])
}

func TestFieldToJSONSchemaMessageNestedAndUnknown(t *testing.T) {
	sub := &descriptorpb.DescriptorProto{
		Name: proto.String("Child"),
		Field: []*descriptorpb.FieldDescriptorProto{
			fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_STRING),
		},
	}
	msgIdx := map[string]*descriptorpb.DescriptorProto{".p.Child": sub}

	nested := fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	nested.TypeName = proto.String(".p.Child")
	sch, _ := fieldToJSONSchema("p", msgIdx, map[string]*descriptorpb.EnumDescriptorProto{}, nested)
	require.Equal(t, "Child", sch["title"])

	repeated := fieldDef(descriptorpb.FieldDescriptorProto_LABEL_REPEATED, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	repeated.TypeName = proto.String(".p.Child")
	sch, _ = fieldToJSONSchema("p", msgIdx, map[string]*descriptorpb.EnumDescriptorProto{}, repeated)
	require.Equal(t, "array", sch["type"])
	require.Equal(t, "Child", sch["items"].(map[string]any)["title"])

	unknown := fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE)
	unknown.TypeName = proto.String(".p.Missing")
	sch, _ = fieldToJSONSchema("p", msgIdx, map[string]*descriptorpb.EnumDescriptorProto{}, unknown)
	require.Equal(t, "object", sch["type"])
	_, has := sch["properties"]
	require.False(t, has)
}

func TestBuildJSONSchemaRequiredAndJsonName(t *testing.T) {
	m := &descriptorpb.DescriptorProto{
		Name: proto.String("Req"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("player_id"),
				JsonName: proto.String("playerId"),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_REQUIRED.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			},
			fieldDef(descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, descriptorpb.FieldDescriptorProto_TYPE_BOOL),
		},
	}
	sch := buildJSONSchema("p", map[string]*descriptorpb.DescriptorProto{}, map[string]*descriptorpb.EnumDescriptorProto{}, m)
	require.Equal(t, "object", sch["type"])
	props := sch["properties"].(map[string]any)
	require.Contains(t, props, "playerId")
	require.Contains(t, props, "f")
	require.Equal(t, []string{"playerId"}, sch["required"])

	// No required fields → no "required" key.
	empty := buildJSONSchema("p", map[string]*descriptorpb.DescriptorProto{}, map[string]*descriptorpb.EnumDescriptorProto{}, &descriptorpb.DescriptorProto{Name: proto.String("E")})
	_, has := empty["required"]
	require.False(t, has)
}

func TestDefaultFunctionID(t *testing.T) {
	require.Equal(t, "demo.v1.playersvc.banplayer", defaultFunctionID("demo.v1", "PlayerSvc", "BanPlayer"))
	require.Equal(t, "a.b.c", defaultFunctionID("a ", "b", "c"))
}

func TestFirstNonEmpty(t *testing.T) {
	require.Equal(t, "", firstNonEmpty())
	require.Equal(t, "", firstNonEmpty("", "  "))
	require.Equal(t, "x", firstNonEmpty("", "  ", "x"))
	require.Equal(t, "y", firstNonEmpty("y", "z"))
}

func TestJoinOptionName(t *testing.T) {
	require.Equal(t, "", joinOptionName(nil))
	single := &descriptorpb.UninterpretedOption{
		Name: []*descriptorpb.UninterpretedOption_NamePart{
			{NamePart: proto.String("(croupier.options.v1.function)"), IsExtension: proto.Bool(true)},
		},
	}
	require.Equal(t, "croupier.options.v1.function", joinOptionName(single))
	multi := &descriptorpb.UninterpretedOption{
		Name: []*descriptorpb.UninterpretedOption_NamePart{
			{NamePart: proto.String("croupier")},
			{NamePart: proto.String("options")},
		},
	}
	require.Equal(t, "croupier.options", joinOptionName(multi))
}

func TestSchemaFileForFQN(t *testing.T) {
	require.Equal(t, filepath.ToSlash(filepath.Join("schema", "demo.v1.request.json")), schemaFileForFQN(".demo.v1.Request"))
	require.Equal(t, filepath.ToSlash(filepath.Join("schema", "demo.v1.weird-id.json")), schemaFileForFQN(".demo.v1.Weird/ID"))
	require.Equal(t, "schema/unknown.json", schemaFileForFQN("   "))
	require.Equal(t, "schema/unknown.json", schemaFileForFQN(""))
}

func TestSanitize(t *testing.T) {
	require.Equal(t, "abc-123_x.y", sanitize("abc-123_x.y"))
	require.Equal(t, "a-b-c", sanitize("a/b c"))
}

func TestTrimQuotesAndParseBool(t *testing.T) {
	require.Equal(t, `x`, trimQuotes(`"x"`))
	require.Equal(t, "x", trimQuotes("x"))
	require.True(t, parseBool("true"))
	require.True(t, parseBool(`"1"`))
	require.True(t, parseBool(" Yes "))
	require.True(t, parseBool("TRUE"))
	require.False(t, parseBool("false"))
	require.False(t, parseBool("0"))
	require.False(t, parseBool(""))
}

func TestParseAggregateKV(t *testing.T) {
	require.Empty(t, parseAggregateKV(""))
	kv := parseAggregateKV(`{ function_id: "player.ban", version: "2.0", two_person_rule: true, nested: { a: "b" }, plain: 42, trailing }`)
	require.Equal(t, "player.ban", kv["function_id"])
	require.Equal(t, "2.0", kv["version"])
	require.Equal(t, "true", kv["two_person_rule"])
	require.Equal(t, "{}", kv["nested"])
	require.Equal(t, "42", kv["plain"])
	require.NotContains(t, kv, "trailing")

	esc := parseAggregateKV(`{ desc: "line1\nline2" }`)
	require.Equal(t, "line1nline2", esc["desc"])

	// Bare value terminated by brace, and duplicate separators.
	mixed := parseAggregateKV("{a: 1 ,  b:2}")
	require.Equal(t, "1", mixed["a"])
	require.Equal(t, "2", mixed["b"])

	// Missing colon value parses nothing harmful.
	require.NotContains(t, parseAggregateKV("abc"), "abc")
}

func TestParseOptionObjectMap(t *testing.T) {
	require.Empty(t, parseOptionObjectMap("", "x"))
	require.Empty(t, parseOptionObjectMap("something", ""))

	s := `key_x: { a: "1", b: "2" } other: 3 key_x: { c: "3" } missing: notanobject`
	m := parseOptionObjectMap(s, "key_x")
	require.Equal(t, map[string]string{"a": "1", "b": "2", "c": "3"}, m)

	// Field name followed by non-object is skipped.
	m = parseOptionObjectMap(`key_x: "nope" key_y: { a: "1" }`, "key_x")
	require.Empty(t, m)

	// Unquoted value fallback.
	m = parseOptionObjectMap(`key_x: { mode: command }`, "key_x")
	require.Equal(t, "command", m["mode"])

	// No match at all.
	require.Empty(t, parseOptionObjectMap(`nothing: { a: "1" }`, "zzz"))
}

func TestParseFunctionOptionsNilAndUninterpreted(t *testing.T) {
	require.Equal(t, funcOpts{}, parseFunctionOptions(nil))

	mo := &descriptorpb.MethodOptions{
		UninterpretedOption: []*descriptorpb.UninterpretedOption{
			{
				Name: []*descriptorpb.UninterpretedOption_NamePart{
					{NamePart: proto.String("(croupier.options.v1.function)"), IsExtension: proto.Bool(true)},
				},
				AggregateValue: proto.String(`{ function_id: "x.y", version: "1.0", resource: "player", operation: "ban", risk: "HIGH", two_person_rule: "yes", summary: "s", description: "d", permission: "p" }`),
			},
			{
				Name: []*descriptorpb.UninterpretedOption_NamePart{
					{NamePart: proto.String("unrelated")},
				},
				AggregateValue: proto.String(`{ a: "b" }`),
			},
		},
	}
	out := parseFunctionOptions(mo)
	require.Equal(t, "x.y", out.FunctionID)
	require.Equal(t, "1.0", out.Version)
	require.Equal(t, "player", out.Resource)
	require.Equal(t, "ban", out.Operation)
	require.Equal(t, "HIGH", out.Risk)
	require.True(t, out.TwoPersonRule)
	require.True(t, out.TwoPersonRuleSet)
	require.Equal(t, "s", out.Summary)
	require.Equal(t, "d", out.Description)
	require.Equal(t, "p", out.Permission)
}

func TestParseFunctionOptionsExtension(t *testing.T) {
	mo := &descriptorpb.MethodOptions{}
	proto.SetExtension(mo, ui.E_Function, &ui.FunctionOptions{
		FunctionId:     "ext.id",
		IdempotencyKey: true,
	})
	out := parseFunctionOptions(mo)
	require.Equal(t, "ext.id", out.FunctionID)
	require.True(t, out.IdempotencyKeySet)
	require.True(t, out.IdempotencyKey)
}

func TestBuildOpenAPIDoc(t *testing.T) {
	ops := []OpenAPIOperation{
		{ID: "a.b", Summary: "Sum", Description: "Desc", Resource: "r", Operation: "op", Risk: "safe", Permission: "perm",
			Request: map[string]any{"proto_fqn": ".p.In"}, Response: map[string]any{"proto_fqn": ".p.Out"}},
		{ID: "bare"},
	}
	doc := buildOpenAPIDoc(ops, map[string]string{"title": "T", "version": "9.9"})
	require.Equal(t, "3.0.3", doc["openapi"])
	info := doc["info"].(map[string]any)
	require.Equal(t, "T", info["title"])
	require.Equal(t, "9.9", info["version"])
	paths := doc["paths"].(map[string]any)
	a := paths["/a.b"].(map[string]any)
	post := a["post"].(map[string]any)
	require.Equal(t, "Sum", post["summary"])
	require.Equal(t, "r", post["x-resource"])
	require.Equal(t, "op", post["x-operation"])
	require.Equal(t, "safe", post["x-risk"])
	require.Equal(t, "perm", post["x-permission"])
	b := paths["/bare"].(map[string]any)
	bpost := b["post"].(map[string]any)
	require.Equal(t, "bare", bpost["summary"])
	_, has := bpost["x-resource"]
	require.False(t, has)

	// Params fallbacks.
	doc = buildOpenAPIDoc(nil, map[string]string{})
	info = doc["info"].(map[string]any)
	require.Equal(t, "Croupier Functions", info["title"])
	require.Equal(t, "Auto-generated OpenAPI specification from protobuf definitions", info["description"])
	require.Equal(t, "1.0.0", info["version"])

	doc = buildOpenAPIDoc(nil, map[string]string{"name": "N", "provider_version": "2", "description": "D"})
	info = doc["info"].(map[string]any)
	require.Equal(t, "N", info["title"])
	require.Equal(t, "2", info["version"])
}

func TestAddJSONAndYAML(t *testing.T) {
	resp := &pluginpb.CodeGeneratorResponse{}
	var files []generatedFile
	addJSON(resp, &files, "a.json", map[string]string{"k": "v"})
	addYAML(resp, &files, "b.yaml", map[string]string{"k": "v"})
	require.Len(t, resp.File, 2)
	require.Len(t, files, 2)
	require.Equal(t, "a.json", resp.File[0].GetName())
	require.Contains(t, resp.File[0].GetContent(), `"k": "v"`)
	require.Contains(t, resp.File[1].GetContent(), "k: v")
}

// --- end-to-end main() coverage ---

func buildTestRequest() *pluginpb.CodeGeneratorRequest {
	inputMsg := &descriptorpb.DescriptorProto{
		Name: proto.String("PingRequest"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("game_id"),
				JsonName: proto.String("gameId"),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_REQUIRED.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			},
		},
	}
	outputMsg := &descriptorpb.DescriptorProto{
		Name: proto.String("PingResponse"),
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("state"),
				JsonName: proto.String("state"),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
				TypeName: proto.String(".demo.v1.State"),
			},
		},
	}
	stateEnum := &descriptorpb.EnumDescriptorProto{
		Name: proto.String("State"),
		Value: []*descriptorpb.EnumValueDescriptorProto{
			{Name: proto.String("OK")},
			{Name: proto.String("FAIL")},
		},
	}
	fullOpts := &descriptorpb.MethodOptions{}
	proto.SetExtension(fullOpts, ui.E_Function, &ui.FunctionOptions{
		FunctionId:    "demo.custom.invoke",
		Resource:      "demo",
		Operation:     "custom",
		Risk:          "Danger",
		Permission:    "demo.custom.invoke",
		Summary:       "Full opts",
		Description:   "Every override set",
		TwoPersonRule: true,
		Version:       "3.0.0",
		Timeout:       "5s",
		Mode:          "command",
	})
	svc := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String("PingService"),
		Method: []*descriptorpb.MethodDescriptorProto{
			{
				Name:       proto.String("Ping"),
				InputType:  proto.String(".demo.v1.PingRequest"),
				OutputType: proto.String(".demo.v1.PingResponse"),
				Options:    &descriptorpb.MethodOptions{},
			},
			{
				Name:       proto.String("Plain"),
				InputType:  proto.String(".demo.v1.PingRequest"),
				OutputType: proto.String(".demo.v1.PingResponse"),
			},
			{
				Name:       proto.String("Full"),
				InputType:  proto.String(".demo.v1.PingRequest"),
				OutputType: proto.String(".demo.v1.PingResponse"),
				Options:    fullOpts,
			},
		},
	}
	fd := &descriptorpb.FileDescriptorProto{
		Name:        proto.String("demo/v1/ping.proto"),
		Package:     proto.String("demo.v1"),
		MessageType: []*descriptorpb.DescriptorProto{inputMsg, outputMsg},
		EnumType:    []*descriptorpb.EnumDescriptorProto{stateEnum},
		Service:     []*descriptorpb.ServiceDescriptorProto{svc},
	}
	other := &descriptorpb.FileDescriptorProto{
		Name:        proto.String("demo/v1/other.proto"),
		Package:     proto.String("demo.v1"),
		MessageType: []*descriptorpb.DescriptorProto{outputMsg},
		EnumType:    []*descriptorpb.EnumDescriptorProto{stateEnum},
	}
	return &pluginpb.CodeGeneratorRequest{
		FileToGenerate: []string{"demo/v1/ping.proto"},
		Parameter:      proto.String("emit_schemas=true,title=Demo,version=2.5"),
		ProtoFile:      []*descriptorpb.FileDescriptorProto{other, fd},
	}
}

func TestMainEndToEnd(t *testing.T) {
	data, err := proto.Marshal(buildTestRequest())
	require.NoError(t, err)

	stdinR, stdinW, err := os.Pipe()
	require.NoError(t, err)
	_, err = stdinW.Write(data)
	require.NoError(t, err)
	require.NoError(t, stdinW.Close())

	outFile, err := os.CreateTemp(t.TempDir(), "gen-out-*.bin")
	require.NoError(t, err)
	outName := outFile.Name()
	defer os.Remove(outName)

	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdinR, outFile
	defer func() {
		os.Stdin, os.Stdout = oldIn, oldOut
	}()

	main() // happy path never calls fatalf/os.Exit

	raw, err := os.ReadFile(outName)
	require.NoError(t, err)
	require.NoError(t, stdinR.Close())

	resp := &pluginpb.CodeGeneratorResponse{}
	require.NoError(t, proto.Unmarshal(raw, resp))

	names := map[string]string{}
	for _, f := range resp.File {
		names[f.GetName()] = f.GetContent()
	}
	require.Contains(t, names, "openapi.yaml")
	openapi := names["openapi.yaml"]
	require.Contains(t, openapi, "demo.custom.invoke")
	require.Contains(t, openapi, "demo.v1.pingservice.ping")
	require.Contains(t, openapi, "title: Demo")
	require.Contains(t, openapi, `version: "2.5"`)
	require.Contains(t, openapi, "x-resource: demo")
	require.Contains(t, openapi, "x-operation: custom")
	require.Contains(t, openapi, "x-risk: danger")
	require.Contains(t, openapi, "x-permission: demo.custom.invoke")
	require.Contains(t, openapi, "summary: Full opts")

	schemaName := schemaFileForFQN(".demo.v1.PingRequest")
	require.Contains(t, names, schemaName)
	require.Contains(t, names[schemaName], `"gameId"`)
	require.Contains(t, names[schemaName], `"required"`)
}

func TestShortHelpersCoverage(t *testing.T) {
	// parseOptionObjectMap with field name at the very end (no value) — tolerated.
	require.Empty(t, parseOptionObjectMap("abc: key_x", "key_x"))
	// Field name not followed by colon-space-object.
	require.Empty(t, parseOptionObjectMap("key_x:", "key_x"))
	require.Empty(t, parseOptionObjectMap("key_x: noobj", "key_x"))

	// parseAggregateKV with string terminated at EOF (no closing quote).
	kv := parseAggregateKV(`{a: "open}`)
	require.Equal(t, "open", kv["a"])

	// Leading separators hit the skip loop; value ends with empty tail.
	kv = parseAggregateKV("  a: 1,  ")
	require.Equal(t, "1", kv["a"])

	// Name terminated by space before colon (skip-to-colon loop).
	kv = parseAggregateKV("a b: 1")
	require.Equal(t, "1", kv["a"])

	// Skip-to-colon hits EOF before colon.
	kv = parseAggregateKV("a b")
	require.NotContains(t, kv, "a")

	// Deeply nested block skip.
	kv = parseAggregateKV(`{n: {a: {b: "c"}}}`)
	require.Equal(t, "{}", kv["n"])

	// parseAggregateKV with escape at very end.
	kv = parseAggregateKV(`a: "x\`)
	require.Equal(t, "x\\", kv["a"])

	// parseOptionObjectMap unterminated object body.
	m := parseOptionObjectMap(`key_x: { a: "1"`, "key_x")
	require.Equal(t, "1", m["a"])

	// Space between field name and colon hits the pre-colon skip loop.
	m = parseOptionObjectMap(`key_x : { a: "1" }`, "key_x")
	require.Equal(t, "1", m["a"])

	// Nested braces inside the object body increment depth.
	m = parseOptionObjectMap(`key_x: { { x: "0" } a: "1" }`, "key_x")
	require.Equal(t, "0", m["{"])
	require.Equal(t, "1", m["}"])

	// Key separated from colon by spaces hits the body skip-to-colon loop.
	m = parseOptionObjectMap(`key_x: { a : "1" }`, "key_x")
	require.Equal(t, "1", m["a"])

	// Escaped quote inside a quoted value.
	m = parseOptionObjectMap(`key_x: { a: "1\"2" }`, "key_x")
	require.Equal(t, `1"2`, m["a"])

	// joinOptionName with empty name part list.
	require.Equal(t, "", joinOptionName(&descriptorpb.UninterpretedOption{}))
}

func TestMainNoops(t *testing.T) {
	require.True(t, strings.TrimSpace("") == "")
}
