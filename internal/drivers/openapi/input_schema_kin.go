package openapi

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/getkin/kin-openapi/openapi3"
)

// kinInputSchemas 用 kin-openapi（getkin，官方生态事实标准）解析 spec 并按
// 统一请求契约推导每个 operation 的 inputSchema（能力矩阵 #3，批次 D +
// SUI-003：输出形状由 input_schema_parity_test 与 request_contract_test 锁定）。
// 返回 methodName → schema 文本。
//
// kin 无法解析（Swagger 2.0、悬空/外链 $ref 等旧实现可容忍的形态）时返回
// nil，调用方回落 map 遍历旧路径——旧推导保留为兼容回落而非主路径。
func kinInputSchemas(specBytes []byte, root map[string]interface{}) map[string]string {
	contracts := kinOperationContracts(specBytes, root)
	if contracts == nil {
		return nil
	}
	out := make(map[string]string, len(contracts))
	for name, contract := range contracts {
		out[name] = extractContractInputSchema(contract)
	}
	return out
}

// kinOperationContracts 用 kin 解析 spec 并提取每个 operation 的统一请求
// 契约（SUI-002/003）。返回 methodName → 契约；kin 拒载或非 3.x 时返回 nil。
func kinOperationContracts(specBytes []byte, root map[string]interface{}) map[string]spec.OperationRequestContract {
	doc, err := openapi3.NewLoader().LoadFromData(specBytes)
	if err != nil {
		return nil
	}
	// kin 的 LoadFromData 不校验版本字段、对 Swagger 2.0 文档也能加载——但 2.0
	// 语义（definitions/produces/formData）kin 不保真，主路径只对 3.x 负责，
	// 其余回落旧 map 遍历（对拍裁定记录：非 3.x 一律走兼容路径）。
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		return nil
	}
	out := make(map[string]spec.OperationRequestContract)
	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			httpMethod := strings.ToUpper(method)
			name := op.OperationID
			if name == "" {
				name = pathToMethodName(path, httpMethod)
			}
			out[name] = ExtractOperationRequestContractKin(op, item.Parameters, root)
		}
	}
	return out
}

// extractContractInputSchema 把统一请求契约投影为驱动侧输入 schema 文本；
// 空输入（无参数无 body）回落到旧的空对象形态。
func extractContractInputSchema(contract spec.OperationRequestContract) string {
	schema, _ := ProjectOperationInputSchema(contract)
	if schema == "" {
		encoded, _ := json.Marshal(map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		})
		return string(encoded)
	}
	return schema
}

// extractInputSchemaKin 从 kin 解析出的 operation 推导调用方输入 schema，
// 经统一请求契约投影（与 extractInputSchema 完全一致，对拍锁定）。
func extractInputSchemaKin(op *openapi3.Operation, root map[string]interface{}) string {
	return extractContractInputSchema(ExtractOperationRequestContractKin(op, nil, root))
}

// schemaRefToMap 把 kin SchemaRef 序列化回通用 map，再复用既有 $ref 内联
// 机器：kin 对内部引用回吐 {"$ref":"#/..."}（悬空/外链在 loader 层已失败、
// 走回落），resolveSchemaRefs 对根文档内联后与旧实现的输出逐键同形。无法
// 内联的引用记入 issues（SUI-008）。
func schemaRefToMap(ref *openapi3.SchemaRef, root map[string]interface{}, issues *[]refIssue) (map[string]interface{}, bool) {
	encoded, err := json.Marshal(ref)
	if err != nil {
		return nil, false
	}
	var raw interface{}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return nil, false
	}
	resolved, ok := resolveSchemaRefs(raw, root, 0, issues).(map[string]interface{})
	return resolved, ok
}
