package openapi

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serializationCase is one SUI-006 matrix cell: given position/style/explode
// and value, assert the wire product (path segment / query pairs / header /
// cookie) or the error.
type serializationCase struct {
	name    string
	in      string
	style   string
	explode bool
	value   any
	wantSeg string
	wantQ   []queryPair
	wantHdr string
	wantCk  string
	wantErr string // substring match; empty = no error
}

// qp is a queryPair shorthand.
func qp(name, value string) queryPair { return queryPair{name: name, value: value} }

func runSerializationCase(t *testing.T, c serializationCase) {
	t.Helper()
	wire, err := serializeParamValue(c.in, "p", c.style, c.explode, c.value)
	if c.wantErr != "" {
		require.Error(t, err, "case %s: want error", c.name)
		assert.Contains(t, err.Error(), c.wantErr, "case %s", c.name)
		return
	}
	require.NoError(t, err, "case %s", c.name)
	assert.Equal(t, c.wantSeg, wire.pathSegment, "case %s path segment", c.name)
	assert.Equal(t, c.wantQ, wire.queryPairs, "case %s query pairs", c.name)
	assert.Equal(t, c.wantHdr, wire.headerValue, "case %s header", c.name)
	assert.Equal(t, c.wantCk, wire.cookieValue, "case %s cookie", c.name)
}

func TestSerializeParamValue_Matrix(t *testing.T) {
	cases := []serializationCase{
		// path simple
		{name: "path simple primitive", in: "path", style: "simple", value: "5", wantSeg: "5"},
		{name: "path simple array", in: "path", style: "simple", value: []any{"a", "b"}, wantSeg: "a,b"},
		{name: "path simple object noexplode", in: "path", style: "simple", value: map[string]any{"id": "1", "tag": "alice"}, wantSeg: "id,1,tag,alice"},
		{name: "path simple object explode", in: "path", style: "simple", explode: true, value: map[string]any{"id": "1", "tag": "alice"}, wantSeg: "id=1,tag=alice"},
		{name: "path simple slash escaped", in: "path", style: "simple", value: "a/b", wantSeg: "a%2Fb"},
		{name: "path simple number", in: "path", style: "simple", value: float64(5), wantSeg: "5"},

		// path label
		{name: "path label primitive", in: "path", style: "label", value: "1.2", wantSeg: ".1.2"},
		{name: "path label array noexplode", in: "path", style: "label", value: []any{1, 2}, wantSeg: ".1,2"},
		{name: "path label array explode", in: "path", style: "label", explode: true, value: []any{1, 2, 3}, wantSeg: ".1.2.3"},
		{name: "path label object explode", in: "path", style: "label", explode: true, value: map[string]any{"id": "1", "tag": "alice"}, wantSeg: ".id=1.tag=alice"},

		// path matrix
		{name: "path matrix primitive", in: "path", style: "matrix", value: float64(5), wantSeg: ";p=5"},
		{name: "path matrix array noexplode", in: "path", style: "matrix", value: []any{1, 2, 3}, wantSeg: ";p=1,2,3"},
		{name: "path matrix array explode", in: "path", style: "matrix", explode: true, value: []any{1, 2}, wantSeg: ";p=1;p=2"},
		{name: "path matrix object explode", in: "path", style: "matrix", explode: true, value: map[string]any{"id": "1", "tag": "alice"}, wantSeg: ";id=1;tag=alice"},

		// query form
		{name: "query form primitive", in: "query", style: "form", value: "x", wantQ: []queryPair{qp("p", "x")}},
		{name: "query form array noexplode", in: "query", style: "form", value: []any{"a", "b"}, wantQ: []queryPair{qp("p", "a,b")}},
		{name: "query form array explode", in: "query", style: "form", explode: true, value: []any{"a", "b", "c"}, wantQ: []queryPair{qp("p", "a"), qp("p", "b"), qp("p", "c")}},
		{name: "query form object explode", in: "query", style: "form", explode: true, value: map[string]any{"id": "1", "tag": "alice"}, wantQ: []queryPair{qp("id", "1"), qp("tag", "alice")}},
		{name: "query form space escaped", in: "query", style: "form", value: "a b&c", wantQ: []queryPair{qp("p", "a+b%26c")}},

		// query spaceDelimited / pipeDelimited
		{name: "query spaced array", in: "query", style: "spaceDelimited", value: []any{"a b", "c"}, wantQ: []queryPair{qp("p", "a+b+c")}},
		{name: "query pipe array", in: "query", style: "pipeDelimited", value: []any{"x", "y"}, wantQ: []queryPair{qp("p", "x|y")}},
		{name: "query pipe object", in: "query", style: "pipeDelimited", value: map[string]any{"id": "1", "tag": "alice"}, wantQ: []queryPair{qp("p", "id|1|tag|alice")}},

		// query deepObject
		{name: "query deepObject object", in: "query", style: "deepObject", explode: true, value: map[string]any{"a": "1", "b": "2"}, wantQ: []queryPair{qp("p%5Ba%5D", "1"), qp("p%5Bb%5D", "2")}},
		{name: "query deepObject array rejected", in: "query", style: "deepObject", explode: true, value: []any{1, 2}, wantErr: "deepObject is only executable for object values"},
		{name: "query deepObject needs explode", in: "query", style: "deepObject", value: map[string]any{"a": "1"}, wantErr: "deepObject requires explode=true"},

		// header simple
		{name: "header primitive", in: "header", style: "simple", value: "ops", wantHdr: "ops"},
		{name: "header array", in: "header", style: "simple", value: []any{"p", "q"}, wantHdr: "p,q"},
		{name: "header wrong style", in: "header", style: "form", value: "x", wantErr: "not valid for header"},

		// cookie form (primitive only)
		{name: "cookie primitive", in: "cookie", style: "form", value: "8", wantCk: "8"},
		{name: "cookie escaped", in: "cookie", style: "form", value: "a=b", wantCk: "a%3Db"},
		{name: "cookie array rejected", in: "cookie", style: "form", value: []any{"a"}, wantErr: "cookie parameters only support primitive values"},
		{name: "cookie object rejected", in: "cookie", style: "form", value: map[string]any{"a": "1"}, wantErr: "cookie parameters only support primitive values"},

		// wrong-position styles and unknown positions
		{name: "path form rejected", in: "path", style: "form", value: "x", wantErr: "not valid for path"},
		{name: "query simple rejected", in: "query", style: "simple", value: "x", wantErr: "not valid for query"},
		{name: "unknown position rejected", in: "body", style: "", value: "x", wantErr: "not serializable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { runSerializationCase(t, c) })
	}
}

func TestDefaultStyleAndExplode(t *testing.T) {
	assert.Equal(t, "simple", defaultStyleForPosition("path"))
	assert.Equal(t, "simple", defaultStyleForPosition("header"))
	assert.Equal(t, "form", defaultStyleForPosition("query"))
	assert.Equal(t, "form", defaultStyleForPosition("cookie"))
	assert.True(t, defaultExplodeForStyle("form"))
	assert.False(t, defaultExplodeForStyle("simple"))
	assert.True(t, isStyleValidForPosition("matrix", "path"))
	assert.False(t, isStyleValidForPosition("form", "path"))
	assert.True(t, isStyleValidForPosition("deepObject", "query"))
	assert.False(t, isStyleValidForPosition("simple", "query"))
}

func TestSchemaValueShape(t *testing.T) {
	shape := func(s string) valueShape { return schemaValueShape(json.RawMessage(s)) }
	assert.Equal(t, shapeArray, shape(`{"type":"array"}`))
	assert.Equal(t, shapeObject, shape(`{"type":"object"}`))
	assert.Equal(t, shapePrimitive, shape(`{"type":"string"}`))
	assert.Equal(t, shapeObject, shape(`{"properties":{"a":{"type":"string"}}}`))
	assert.Equal(t, shapeUnknown, shape(`{}`))
	assert.Equal(t, shapeUnknown, shape(""))
	assert.Equal(t, shapeUnknown, shape(`not json`))
}

func TestSerializationUnsupportedReason(t *testing.T) {
	explodeTrue := true
	param := func(in, style string, schema string) RequestParameter {
		p := RequestParameter{In: in, Name: "p", Style: style}
		if schema != "" {
			p.Schema = json.RawMessage(schema)
		}
		return p
	}
	// invalid style at position
	assert.Contains(t, serializationUnsupportedReason(param("path", "form", `{"type":"string"}`), shapeUnknown), "not valid for path parameters")
	// deepObject without explode
	assert.Equal(t, "deepObject requires explode=true", serializationUnsupportedReason(param("query", "deepObject", `{"type":"object"}`), shapeUnknown))
	// deepObject on array schema (explode=true so the explode check passes)
	deepArray := RequestParameter{In: "query", Name: "p", Style: "deepObject", Explode: &explodeTrue, Schema: json.RawMessage(`{"type":"array"}`)}
	assert.Equal(t, "deepObject is only executable for object values", serializationUnsupportedReason(deepArray, shapeUnknown))
	// cookie array schema
	assert.Equal(t, "cookie parameters only support primitive values", serializationUnsupportedReason(param("cookie", "form", `{"type":"array"}`), shapeUnknown))
	// executable combos
	assert.Empty(t, serializationUnsupportedReason(param("path", "", `{"type":"string"}`), shapeUnknown))
	assert.Empty(t, serializationUnsupportedReason(param("query", "", `{"type":"array","items":{"type":"string"}}`), shapeUnknown))
}

func TestValidateSerialization_Diagnostic(t *testing.T) {
	explode := true
	p := RequestParameter{In: "cookie", Name: "bad", Style: "form", Schema: json.RawMessage(`{"type":"array","items":{"type":"string"}}`)}
	diags := validateSerialization(p)
	require.Len(t, diags, 1)
	assert.Equal(t, diagParameterSerializationUnsupported, diags[0].Code)
	assert.Equal(t, spec.SeverityError, diags[0].Severity)
	assert.Equal(t, "bad", diags[0].Field)
	assert.Contains(t, diags[0].Message, "cannot be serialized")

	assert.Empty(t, validateSerialization(RequestParameter{In: "query", Name: "ok", Explode: &explode}))
}

func TestSchemaDefault_Helper(t *testing.T) {
	v, ok := schemaDefault(json.RawMessage(`{"type":"string","default":"x"}`))
	require.True(t, ok)
	assert.Equal(t, "x", v)
	_, ok = schemaDefault(json.RawMessage(`{"type":"string"}`))
	assert.False(t, ok)
	_, ok = schemaDefault(nil)
	assert.False(t, ok)
}
