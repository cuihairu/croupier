package openapi

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/platform/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-008 验收：脱离 OpenAPI 文档后，嵌套本地引用仍解析；递归/缺失/外部引用
// 边界明确进入待审（proposal 记录诊断 openapi_schema_reference_unresolved，
// 绝不静默坍缩成匿名对象）。kin 能载入的文档（嵌套、递归）两个前端对拍同形；
// 悬空/外部引用在 kin loader 层被拒（"disallowed external reference"），生产
// 走 map 回落——回落侧同样记录诊断。

// refNestedDoc：嵌套非递归本地引用（Person → Address），kin 可载入。
const refNestedDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "refs", "version": "1.0.0"},
  "components": {"schemas": {
    "Address": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]},
    "Person": {"type": "object", "properties": {"name": {"type": "string"}, "address": {"$ref": "#/components/schemas/Address"}}}
  }},
  "paths": {
    "/people": {"post": {"operationId": "person.create", "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Person"}}}}, "responses": {"200": {"description": "ok"}}}}
  }
}`

// refCyclicDoc：自引用 Node（Node.next 指向 Node），kin 可载入；深度上限处
// 截断并记录循环诊断。
const refCyclicDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "refs", "version": "1.0.0"},
  "components": {"schemas": {
    "Node": {"type": "object", "properties": {"name": {"type": "string"}, "next": {"$ref": "#/components/schemas/Node"}}}
  }},
  "paths": {
    "/nodes": {"post": {"operationId": "node.create", "requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Node"}}}}, "responses": {"200": {"description": "ok"}}}}
  }
}`

// refMissingDoc：悬空本地引用（DoesNotExist 未声明），kin loader 拒载 → 回落。
const refMissingDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "refs", "version": "1.0.0"},
  "paths": {
    "/missing": {"post": {"operationId": "missing.create", "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/DoesNotExist"}}}}, "responses": {"200": {"description": "ok"}}}}
  }
}`

// refExternalDoc：外部引用（other.yaml），kin loader 拒载 → 回落。
const refExternalDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "refs", "version": "1.0.0"},
  "paths": {
    "/ext": {"post": {"operationId": "ext.create", "requestBody": {"content": {"application/json": {"schema": {"$ref": "other.yaml#/components/schemas/Thing"}}}}, "responses": {"200": {"description": "ok"}}}}
  }
}`

// refDupDoc：同一悬空引用出现在参数与 body 两个 schema——契约内去重为一条诊断。
const refDupDoc = `{
  "openapi": "3.0.3",
  "info": {"title": "refs", "version": "1.0.0"},
  "paths": {
    "/dup": {"post": {"operationId": "dup.create",
      "parameters": [{"name": "filter", "in": "query", "schema": {"$ref": "#/components/schemas/DoesNotExist"}}],
      "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/DoesNotExist"}}}},
      "responses": {"200": {"description": "ok"}}}}
  }
}`

func TestSchemaRef_NestedLocalRefsResolve(t *testing.T) {
	contracts := mediaContracts(t, refNestedDoc)
	for _, name := range []string{"/people.post", "/people.map.post"} {
		contract, ok := contracts[name]
		require.True(t, ok, "contract missing for %s", name)
		require.NotNil(t, contract.Body, "%s: nested local ref must yield a body slot", name)
		assert.Nil(t, diagnosticByCode(contract.Diagnostics, spec.DiagSchemaReferenceUnresolved),
			"%s: nested resolvable refs emit no unresolved-reference diagnostic", name)
		assert.Contains(t, string(contract.Body.Schema), "city",
			"%s: nested ref must be inlined (Address.city present)", name)
	}
	// 对拍：kin 与 map 输出同形。
	assert.JSONEq(t, string(contracts["/people.post"].Body.Schema), string(contracts["/people.map.post"].Body.Schema))
}

func TestSchemaRef_CyclicRefRecordsDiagnostic(t *testing.T) {
	contracts := mediaContracts(t, refCyclicDoc)
	for _, name := range []string{"/nodes.post", "/nodes.map.post"} {
		contract, ok := contracts[name]
		require.True(t, ok, "contract missing for %s", name)
		require.NotNil(t, contract.Body, "%s: cyclic-ref body must still yield a slot (truncated at depth cap)", name)
		diag := diagnosticByCode(contract.Diagnostics, spec.DiagSchemaReferenceUnresolved)
		require.NotNil(t, diag, "%s: recursive ref must be recorded as unresolved (pending review)", name)
		assert.Equal(t, "#/components/schemas/Node", diag.Field)
		assert.Equal(t, spec.SeverityWarning, diag.Severity)
		assert.Contains(t, diag.Message, "cyclic reference")
	}
	// 对拍：循环截断深度同机器（kin/map 同深），schema 与诊断同形——同文档
	// 两个前端产出同一契约（含 pending-review 诊断）。
	assert.JSONEq(t, string(contracts["/nodes.post"].Body.Schema), string(contracts["/nodes.map.post"].Body.Schema))
	assert.JSONEq(t, mustJSON(t, contracts["/nodes.post"]), mustJSON(t, contracts["/nodes.map.post"]))
}

func TestSchemaRef_MissingRefRecordsDiagnostic(t *testing.T) {
	// kin loader 拒载悬空引用（走回落），此处断言回落侧（map 前端）记录诊断。
	contract := ExtractOperationRequestContractMap(
		methodObjOf(t, refMissingDoc, "/missing", "post"), mustLoadRoot(t, refMissingDoc), nil)
	require.NotNil(t, contract.Body, "missing ref collapses to an empty object and keeps a body slot")
	assert.Equal(t, spec.JSONSchema(`{"type":"object"}`), contract.Body.Schema)
	diag := diagnosticByCode(contract.Diagnostics, spec.DiagSchemaReferenceUnresolved)
	require.NotNil(t, diag, "missing target must be recorded as unresolved")
	assert.Equal(t, "#/components/schemas/DoesNotExist", diag.Field)
	assert.Contains(t, diag.Message, "missing target")
}

func TestSchemaRef_ExternalRefRecordsDiagnostic(t *testing.T) {
	// kin loader 拒载外部引用（走回落），断言回落侧保留引用并记录诊断。
	contract := ExtractOperationRequestContractMap(
		methodObjOf(t, refExternalDoc, "/ext", "post"), mustLoadRoot(t, refExternalDoc), nil)
	require.NotNil(t, contract.Body, "external ref is preserved as $ref and keeps a body slot")
	assert.Contains(t, string(contract.Body.Schema), `"$ref"`)
	diag := diagnosticByCode(contract.Diagnostics, spec.DiagSchemaReferenceUnresolved)
	require.NotNil(t, diag, "external reference must be recorded as unresolved")
	assert.Equal(t, "other.yaml#/components/schemas/Thing", diag.Field)
	assert.Contains(t, diag.Message, "outside the document")
}

func TestSchemaRef_SameRefDedupedPerContract(t *testing.T) {
	// 同一悬空引用出现在参数与 body：契约内去重为一条诊断（ref+reason 键）。
	contract := ExtractOperationRequestContractMap(
		methodObjOf(t, refDupDoc, "/dup", "post"), mustLoadRoot(t, refDupDoc), nil)
	assert.Equal(t, 1, countDiagnostics(contract.Diagnostics, spec.DiagSchemaReferenceUnresolved),
		"the same unresolved reference must yield exactly one diagnostic per contract")
}

func TestSchemaRef_FallbackDiscoveryRecordsDiagnostic(t *testing.T) {
	// 生产发现路径：kin 拒载悬空/外部引用 → parseOpenAPISpec 落 map 遍历，
	// 发现到的 operation 契约携带 unresolved-reference 诊断（进入提案待审）。
	for _, doc := range []string{refMissingDoc, refExternalDoc} {
		p := NewProvider()
		p.config = provider.ProviderConfig{Enabled: true}
		p.openapiConfig = &Config{}
		p.methodMap = make(map[string]*APIMethod)
		require.NoError(t, p.parseOpenAPISpec([]byte(doc)), "map fallback must still discover the operation")
		found := false
		for _, m := range p.methodMap {
			if m.RequestContract == nil {
				continue
			}
			if diagnosticByCode(m.RequestContract.Diagnostics, spec.DiagSchemaReferenceUnresolved) != nil {
				found = true
			}
		}
		assert.True(t, found, "fallback discovery must surface the unresolved-reference diagnostic")
	}
}

func countDiagnostics(diags []spec.Diagnostic, code string) int {
	count := 0
	for _, d := range diags {
		if d.Code == code {
			count++
		}
	}
	return count
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	encoded, err := json.Marshal(v)
	require.NoError(t, err)
	return string(encoded)
}
