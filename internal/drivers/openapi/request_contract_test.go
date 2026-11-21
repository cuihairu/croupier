package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustLoadKin 用 kin 解析 spec 并返回 (doc, root)。
func mustLoadKin(t *testing.T, doc string) (*openapi3.T, map[string]interface{}) {
	t.Helper()
	loader := openapi3.NewLoader()
	loaded, err := loader.LoadFromData([]byte(doc))
	require.NoError(t, err)
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(doc), &root))
	return loaded, root
}

// SUI-002：path-item 级参数并入（operation 级按 (in,name) 覆盖）。
func TestRequestContract_KinPathItemParamsMergedWithOverride(t *testing.T) {
	doc := `{"openapi":"3.0.3","paths":{"/pets/{petId}":{"parameters":[{"name":"petId","in":"path","required":true,"schema":{"type":"string"}},{"name":"shared","in":"query","schema":{"type":"string"}}],"get":{"operationId":"pet.get","parameters":[{"name":"petId","in":"path","required":true,"schema":{"type":"integer"}},{"name":"detail","in":"query","schema":{"type":"boolean"}}]}}}}`
	loaded, root := mustLoadKin(t, doc)
	item := loaded.Paths.Find("/pets/{petId}")
	contract := ExtractOperationRequestContractKin(item.Get, item.Parameters, root)

	require.Len(t, contract.Parameters, 3)
	// operation 级在前；同名 (path,petId) 被 operation 级覆盖为 integer。
	assert.Equal(t, "petId", contract.Parameters[0].Name)
	assert.Equal(t, "path", string(contract.Parameters[0].In))
	assert.Equal(t, "integer", schemaTypeOf(t, contract.Parameters[0].Schema))
	assert.Equal(t, "detail", contract.Parameters[1].Name)
	assert.Equal(t, "query", string(contract.Parameters[1].In))
	// path-item 级未被覆盖的参数在后。
	assert.Equal(t, "shared", contract.Parameters[2].Name)
	assert.Equal(t, "query", string(contract.Parameters[2].In))
}

// SUI-002：header/cookie 参数被捕获（旧上传路径静默丢弃），序列化默认按位置。
func TestRequestContract_KinHeaderCookieCaptured(t *testing.T) {
	doc := `{"openapi":"3.0.3","paths":{"/h/{ver}":{"get":{"operationId":"h.call","parameters":[{"name":"ver","in":"path","required":true,"schema":{"type":"integer"}},{"name":"X-Trace","in":"header","schema":{"type":"string"}},{"name":"session","in":"cookie","required":true,"schema":{"type":"string","format":"uuid"}}]}}}}`
	loaded, root := mustLoadKin(t, doc)
	contract := ExtractOperationRequestContractKin(loaded.Paths.Find("/h/{ver}").Get, nil, root)

	require.Len(t, contract.Parameters, 3)
	byName := map[string]spec.RequestParameterSpec{}
	for _, p := range contract.Parameters {
		byName[p.Name] = p
	}
	assert.Equal(t, "header", string(byName["X-Trace"].In))
	assert.Equal(t, "simple", byName["X-Trace"].Serialization.Style)
	assert.False(t, byName["X-Trace"].Required)
	assert.Equal(t, "cookie", string(byName["session"].In))
	assert.Equal(t, "form", byName["session"].Serialization.Style)
	assert.True(t, byName["session"].Required)
	assert.Equal(t, "path", string(byName["ver"].In))
	assert.Equal(t, "simple", byName["ver"].Serialization.Style)
}

// SUI-002：完整 body schema 保留（含根数组与根级约束）。
func TestRequestContract_KinRootArrayBodyPreserved(t *testing.T) {
	doc := `{"openapi":"3.0.3","paths":{"/batch":{"post":{"operationId":"batch.import","requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"array","items":{"type":"string"},"maxItems":100}}}}}}}}`
	loaded, root := mustLoadKin(t, doc)
	contract := ExtractOperationRequestContractKin(loaded.Paths.Find("/batch").Post, nil, root)
	require.NotNil(t, contract.Body)
	assert.Equal(t, "application/json", contract.Body.MediaType)
	assert.True(t, contract.Body.Required)
	bodySchema := decodeMap(t, contract.Body.Schema)
	assert.Equal(t, "array", bodySchema["type"])
	assert.EqualValues(t, 100, bodySchema["maxItems"])

	// 无寻址参数 → body_only 投影：根数组原样。
	schema, mode := ProjectOperationInputSchema(contract)
	assert.Equal(t, "body_only", string(mode))
	assert.Equal(t, "array", decodeMap(t, spec.JSONSchema(schema))["type"])
}

// SUI-002/004：body 根级不可保留约束（minProperties）→ 诊断 + structured。
func TestRequestContract_BodyRootConstraintForcesStructured(t *testing.T) {
	doc := `{"openapi":"3.0.3","paths":{"/c":{"post":{"operationId":"c.create","parameters":[{"name":"q","in":"query","schema":{"type":"string"}}],"requestBody":{"content":{"application/json":{"schema":{"type":"object","minProperties":1,"properties":{"a":{"type":"string"}}}}}}}}}}`
	loaded, root := mustLoadKin(t, doc)
	contract := ExtractOperationRequestContractKin(loaded.Paths.Find("/c").Post, nil, root)
	diag := diagnosticByCode(contract.Diagnostics, spec.DiagRequestBodyShapeNonFlat)
	if diag == nil {
		t.Fatal("minProperties is not flat-preservable → expected nonflat diagnostic")
	}
	assert.Equal(t, "body", diag.Field, "shape diagnostic is field-level on the body root")

	schema, mode := ProjectOperationInputSchema(contract)
	assert.Equal(t, "structured", string(mode))
	rootSchema := decodeMap(t, spec.JSONSchema(schema))
	props := decodeMap(t, rootSchema["properties"].(map[string]interface{})["body"])
	assert.EqualValues(t, 1, props["minProperties"], "structured body slot keeps the root constraint")
}

// SUI-002/004：query/body 同名 → 冲突诊断 + structured（不覆盖不丢弃）。
func TestRequestContract_QueryBodySameNameCollision(t *testing.T) {
	doc := `{"openapi":"3.0.3","paths":{"/u":{"post":{"operationId":"u.create","parameters":[{"name":"name","in":"query","required":true,"schema":{"type":"string"}}],"requestBody":{"content":{"application/json":{"schema":{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}}}}}}}}}}`
	loaded, root := mustLoadKin(t, doc)
	contract := ExtractOperationRequestContractKin(loaded.Paths.Find("/u").Post, nil, root)
	diag := diagnosticByCode(contract.Diagnostics, spec.DiagRequestParameterCollision)
	if diag == nil {
		t.Fatal("query/body same-name collision must emit collision diagnostic")
	}
	assert.Equal(t, "name", diag.Field, "collision diagnostic is field-level")

	schema, mode := ProjectOperationInputSchema(contract)
	assert.Equal(t, "structured", string(mode))
	rootSchema := decodeMap(t, spec.JSONSchema(schema))
	paramsObj := decodeMap(t, rootSchema["properties"].(map[string]interface{})["params"])
	paramsProps := decodeMap(t, paramsObj["properties"])
	assert.Contains(t, paramsProps, "name")
	bodyObj := decodeMap(t, rootSchema["properties"].(map[string]interface{})["body"])
	bodyProps := decodeMap(t, bodyObj["properties"])
	assert.Contains(t, bodyProps, "name")
	assert.Contains(t, bodyProps, "age")
}

// SUI-004：跨位置同名参数 → 冲突诊断 + structured（平铺只能留一个）。
func TestRequestContract_CrossPositionSameNameCollision(t *testing.T) {
	doc := `{"openapi":"3.0.3","paths":{"/p/{id}":{"get":{"operationId":"p.get","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"id","in":"query","schema":{"type":"integer"}}]}}}}`
	loaded, root := mustLoadKin(t, doc)
	contract := ExtractOperationRequestContractKin(loaded.Paths.Find("/p/{id}").Get, nil, root)
	diag := diagnosticByCode(contract.Diagnostics, spec.DiagRequestParameterCollision)
	if diag == nil {
		t.Fatal("cross-position same-name collision must emit collision diagnostic")
	}
	assert.Equal(t, "id", diag.Field, "cross-position collision diagnostic names the field")

	_, mode := ProjectOperationInputSchema(contract)
	assert.Equal(t, "structured", string(mode))
}

// SUI-003：同一 3.x 文档 kin 前端与 map 前端产出相同契约（逐 op JSONEq）。
func TestRequestContract_KinMapParity(t *testing.T) {
	docs := []string{
		`{"openapi":"3.0.3","paths":{"/u/{id}":{"put":{"operationId":"u.update","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"dry_run","in":"query","schema":{"type":"boolean"}},{"name":"X-Key","in":"header","schema":{"type":"string"}}],"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}}}}}}}}`,
		`{"openapi":"3.0.3","components":{"schemas":{"Payload":{"type":"array","items":{"type":"string"}}}},"paths":{"/batch":{"post":{"operationId":"batch.send","requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Payload"}}}}}}}}`,
		`{"openapi":"3.0.3","paths":{"/p/{id}":{"get":{"operationId":"p.get","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}]}}}}`,
	}
	for i, doc := range docs {
		loaded, root := mustLoadKin(t, doc)
		rootMap := decodeMap(t, spec.JSONSchema(doc))
		pathsRaw, ok := rootMap["paths"].(map[string]interface{})
		if !ok {
			t.Fatalf("doc %d: paths is not an object", i)
		}
		// 与生产同形：map 前端遍历原始文档（method 键小写），kin 前端遍历
		// PathItem.Operations()（method 键大写）——按大写归一后配对对拍。
		for path, pathItemRaw := range pathsRaw {
			pathObjRaw, ok := pathItemRaw.(map[string]interface{})
			if !ok {
				t.Fatalf("doc %d: path %q not an object (got %T)", i, path, pathItemRaw)
			}
			pathItemParams, _ := pathObjRaw["parameters"].([]interface{})
			item, hasItem := loaded.Paths.Map()[path]
			if !hasItem {
				t.Fatalf("doc %d: kin could not load path %q", i, path)
			}
			for methodRaw, methodItem := range pathObjRaw {
				if _, isMethod := methodItem.(map[string]interface{}); !isMethod {
					continue
				}
				op := item.Operations()[strings.ToUpper(methodRaw)]
				if op == nil {
					t.Fatalf("doc %d: kin missing operation %s.%s", i, methodRaw, path)
				}
				kinContract := ExtractOperationRequestContractKin(op, item.Parameters, root)
				mapContract := ExtractOperationRequestContractMap(methodItem.(map[string]interface{}), root, pathItemParams)

				kinJSON, _ := json.Marshal(kinContract)
				mapJSON, _ := json.Marshal(mapContract)
				assert.JSONEq(t, string(kinJSON), string(mapJSON),
					"doc %d %s.%s: kin and map front-ends must extract identical contracts", i, methodRaw, path)
			}
		}
	}
}

// SUI-004：平铺投影对无歧义对象输入字节稳定（键序与旧实现一致）。
func TestRequestContract_FlatProjectionByteStable(t *testing.T) {
	doc := `{"openapi":"3.0.3","paths":{"/players/{id}":{"put":{"operationId":"player.update","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"dry_run","in":"query","schema":{"type":"boolean"}}],"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"name":{"type":"string"},"level":{"type":"integer"}},"required":["name"]}}}}}}}}`
	loaded, root := mustLoadKin(t, doc)
	contract := ExtractOperationRequestContractKin(loaded.Paths.Find("/players/{id}").Put, nil, root)
	schema, mode := ProjectOperationInputSchema(contract)
	assert.Equal(t, "flat", string(mode))
	// 逐字节锁定旧平铺形态：json.Marshal 按键名排序（properties 内 name/level、
	// 顶层 properties/required/type），required 去重后排序 [id name]。
	assert.Equal(t,
		`{"properties":{"dry_run":{"type":"boolean"},"id":{"type":"string"},"level":{"type":"integer"},"name":{"type":"string"}},"required":["id","name"],"type":"object"}`,
		schema)
}

// SUI-003：Swagger 2.0 兼容（map 前端）。in: body → body 槽位；in: formData
// 提升进隐式 object body 属性；mediaType 从 consumes（method 优先、doc 兜底、
// 缺省 application/json）。
func TestRequestContract_Swagger20Compat(t *testing.T) {
	t.Run("body param with method-level consumes", func(t *testing.T) {
		doc := `{"swagger":"2.0","consumes":["application/xml"],"paths":{"/x":{"post":{"operationId":"x.create","consumes":["application/json"],"parameters":[{"name":"payload","in":"body","required":true,"schema":{"type":"object","properties":{"a":{"type":"string"}}}}]}}}}`
		contract := ExtractOperationRequestContractMap(methodObjOf(t, doc, "/x", "post"), mustLoadRoot(t, doc), nil)
		require.NotNil(t, contract.Body)
		assert.Equal(t, "application/json", contract.Body.MediaType, "method-level consumes wins over doc-level")
		assert.True(t, contract.Body.Required)
		props := decodeMap(t, contract.Body.Schema)["properties"].(map[string]interface{})
		assert.Contains(t, props, "a")
	})

	t.Run("formData lifted into implicit object body", func(t *testing.T) {
		doc := `{"swagger":"2.0","paths":{"/f":{"post":{"operationId":"f.upload","parameters":[{"name":"nickname","in":"formData","type":"string","required":true},{"name":"age","in":"formData","type":"integer"}]}}}}`
		contract := ExtractOperationRequestContractMap(methodObjOf(t, doc, "/f", "post"), mustLoadRoot(t, doc), nil)
		require.NotNil(t, contract.Body)
		assert.Equal(t, "application/json", contract.Body.MediaType, "default media type when consumes absent")
		bodySchema := decodeMap(t, contract.Body.Schema)
		assert.Equal(t, "object", bodySchema["type"])
		props := bodySchema["properties"].(map[string]interface{})
		assert.Contains(t, props, "nickname")
		assert.Contains(t, props, "age")
		required, _ := bodySchema["required"].([]interface{})
		assert.ElementsMatch(t, []interface{}{"nickname"}, required)

		// 无寻址参数 → body_only 投影，body 原样（formData 提升后的对象）。
		schema, mode := ProjectOperationInputSchema(contract)
		assert.Equal(t, "body_only", string(mode))
		assert.Equal(t, "object", decodeMap(t, spec.JSONSchema(schema))["type"])
	})

	t.Run("no body param and no formData", func(t *testing.T) {
		doc := `{"swagger":"2.0","consumes":["application/xml"],"paths":{"/y":{"get":{"operationId":"y.get","parameters":[]}}}}`
		contract := ExtractOperationRequestContractMap(methodObjOf(t, doc, "/y", "get"), mustLoadRoot(t, doc), nil)
		assert.Nil(t, contract.Body, "no in:body / in:formData → no body slot")
	})
}

func mustLoadRoot(t *testing.T, doc string) map[string]interface{} {
	t.Helper()
	var root map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(doc), &root))
	return root
}

func methodObjOf(t *testing.T, doc, path, method string) map[string]interface{} {
	t.Helper()
	root := mustLoadRoot(t, doc)
	paths := root["paths"].(map[string]interface{})
	pathObj := paths[path].(map[string]interface{})
	methodObj, ok := pathObj[method].(map[string]interface{})
	require.True(t, ok)
	return methodObj
}

func schemaTypeOf(t *testing.T, schema spec.JSONSchema) string {
	t.Helper()
	return decodeMap(t, schema)["type"].(string)
}

func decodeMap(t *testing.T, raw interface{}) map[string]interface{} {
	t.Helper()
	switch v := raw.(type) {
	case spec.JSONSchema:
		var out map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(v), &out))
		return out
	case string:
		var out map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(v), &out))
		return out
	case map[string]interface{}:
		return v
	default:
		t.Fatalf("cannot decode %T", raw)
		return nil
	}
}

func hasDiagnostic(diags []spec.Diagnostic, code string) bool {
	return diagnosticByCode(diags, code) != nil
}

func diagnosticByCode(diags []spec.Diagnostic, code string) *spec.Diagnostic {
	for i := range diags {
		if diags[i].Code == code {
			return &diags[i]
		}
	}
	return nil
}
