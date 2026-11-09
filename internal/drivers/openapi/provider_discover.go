package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// discoverMethodsFromSpec auto-discovers methods from a single OpenAPI specification.
// Supports both HTTP URLs and local file paths (YAML or JSON format).
func (p *Provider) discoverMethodsFromSpec(ctx context.Context, specURL string) error {
	var spec []byte
	var err error

	if strings.HasPrefix(specURL, "http://") || strings.HasPrefix(specURL, "https://") {
		// HTTP URL
		req, err := http.NewRequestWithContext(ctx, "GET", specURL, nil)
		if err != nil {
			return err
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to fetch OpenAPI spec: %s", resp.Status)
		}

		spec, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	} else {
		// Local file path (supports both JSON and YAML)
		spec, err = os.ReadFile(specURL)
		if err != nil {
			return fmt.Errorf("failed to read OpenAPI spec file: %w", err)
		}

		// Convert YAML to JSON if needed
		if !looksLikeJSON(spec) {
			spec, err = yamlToJSON(spec)
			if err != nil {
				return fmt.Errorf("failed to convert YAML to JSON: %w", err)
			}
		}
	}

	// Parse OpenAPI spec and generate method definitions
	return p.parseOpenAPISpec(spec)
}

// looksLikeJSON checks if the data appears to be JSON format.
func looksLikeJSON(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}
	return trimmed[0] == '{' || trimmed[0] == '['
}

// yamlToJSON converts YAML bytes to JSON bytes.
func yamlToJSON(yamlData []byte) ([]byte, error) {
	var yamlObj interface{}
	if err := yaml.Unmarshal(yamlData, &yamlObj); err != nil {
		return nil, err
	}
	return json.Marshal(yamlObj)
}

// parseOpenAPISpec parses OpenAPI/Swagger specification.
func (p *Provider) parseOpenAPISpec(spec []byte) error {
	// Store raw OpenAPI document for later retrieval
	if p.openapiDoc == nil {
		p.openapiDoc = spec
	} else {
		// Merge with existing doc (if multiple specs)
		var existing map[string]interface{}
		var newDoc map[string]interface{}
		if err := json.Unmarshal(p.openapiDoc, &existing); err != nil {
			return err
		}
		if err := json.Unmarshal(spec, &newDoc); err != nil {
			return err
		}

		// Merge paths
		if existingPaths, ok := existing["paths"].(map[string]interface{}); ok {
			if newPaths, ok := newDoc["paths"].(map[string]interface{}); ok {
				for k, v := range newPaths {
					existingPaths[k] = v
				}
			}
		}

		// existing 是 json.Unmarshal 的产物（仅含 JSON 基础类型），
		// 再 Marshal 恒成功，err 分支为死代码已删。
		merged, _ := json.Marshal(existing)
		p.openapiDoc = merged
	}

	var openapi map[string]interface{}
	if err := json.Unmarshal(spec, &openapi); err != nil {
		return err
	}

	// Document-level version (info.version). OpenAPI spec allows any string
	// here; validity is decided by the consumer (versionutil), we only carry
	// the raw value. First non-empty document wins on multi-spec merge.
	if p.infoVersion == "" {
		if info, ok := openapi["info"].(map[string]interface{}); ok {
			if v, ok := info["version"].(string); ok {
				p.infoVersion = strings.TrimSpace(v)
			}
		}
	}

	// Support both OpenAPI 3.0 and Swagger 2.0
	paths := make(map[string]interface{})
	if v, ok := openapi["paths"].(map[string]interface{}); ok {
		// OpenAPI 3.0
		paths = v
	} else if v, ok := openapi["apis"].(map[string]interface{}); ok {
		// Swagger 2.0 (sometimes uses "apis")
		paths = v
	}

	// 批次 D：inputSchema 推导优先走 kin-openapi 解析层（对拍锁定同形输出）；
	// kin 解析失败（Swagger 2.0/悬空 $ref 等）回落下方 map 遍历旧推导。
	kinDerived := kinInputSchemas(spec, openapi)

	for apiPath, pathItem := range paths {
		pathObj, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}

		// Iterate over HTTP methods
		for httpMethod, methodItem := range pathObj {
			httpMethod = strings.ToUpper(httpMethod)
			if httpMethod == "PARAMETERS" || httpMethod == "$REF" {
				continue
			}

			methodObj, ok := methodItem.(map[string]interface{})
			if !ok {
				continue
			}

			// Extract operation ID or generate from path
			var methodName string
			if opID, ok := methodObj["operationId"].(string); ok {
				methodName = opID
			} else {
				methodName = pathToMethodName(apiPath, httpMethod)
			}

			// Extract description
			desc := ""
			if summary, ok := methodObj["summary"].(string); ok {
				desc = summary
			} else if description, ok := methodObj["description"].(string); ok {
				desc = description
			}

			// Extract tags
			tags := p.extractTags(methodObj)

			// Extract operationId
			operationID, _ := methodObj["operationId"].(string)

			// Extract summary (prefer summary over description for short text)
			summary, _ := methodObj["summary"].(string)

			// Extract deprecated flag
			deprecated, _ := methodObj["deprecated"].(bool)

			// Extract OpenAPI extension fields (x-*)
			resource, _ := methodObj["x-resource"].(string)
			risk, _ := methodObj["x-risk"].(string)
			operation, _ := methodObj["x-operation"].(string)
			capability, _ := methodObj["x-capability"].(string)
			execution, _ := methodObj["x-execution"].(string)
			enabled, _ := methodObj["x-enabled"].(bool)
			permission, _ := methodObj["x-permission"].(string)
			versionOverride, _ := methodObj["x-version"].(string)

			// Create APIMethod
			inputSchema := extractInputSchema(methodObj, openapi)
			if kinSchema, ok := kinDerived[methodName]; ok {
				inputSchema = kinSchema
			}
			apiMethod := &APIMethod{
				Name:         methodName,
				OperationID:  operationID,
				Summary:      summary,
				Description:  desc,
				Path:         apiPath,
				Method:       httpMethod,
				Tags:         tags,
				Deprecated:   deprecated,
				Parameters:   p.extractParameters(methodObj),
				Resource:     resource,
				Risk:         risk,
				Operation:    operation,
				Capability:   capability,
				Execution:    execution,
				Enabled:      enabled,
				Permission:   permission,
				Version:      strings.TrimSpace(versionOverride),
				InputSchema:  inputSchema,
				OutputSchema: extractOutputSchema(methodObj, openapi),
			}

			p.methodMap[methodName] = apiMethod
			p.methods = append(p.methods, methodName)
		}
	}

	return nil
}

// extractTags extracts tags from OpenAPI method definition.
func (p *Provider) extractTags(methodObj map[string]interface{}) []string {
	tagList, ok := methodObj["tags"].([]interface{})
	if !ok {
		return nil
	}
	tags := make([]string, 0, len(tagList))
	for _, t := range tagList {
		if tag, ok := t.(string); ok {
			tags = append(tags, tag)
		}
	}
	return tags
}

// extractParameters extracts parameters from OpenAPI method definition.
func (p *Provider) extractParameters(methodObj map[string]interface{}) []ParameterMapping {
	params := make([]ParameterMapping, 0)

	paramList, ok := methodObj["parameters"].([]interface{})
	if !ok {
		return params
	}

	for _, param := range paramList {
		pObj, ok := param.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract basic parameter info
		name, _ := pObj["name"].(string)
		in, _ := pObj["in"].(string) // "path", "query", "header", "cookie"
		required, _ := pObj["required"].(bool)
		desc, _ := pObj["description"].(string)
		deprecated, _ := pObj["deprecated"].(bool)

		// Build parameter mapping
		paramMapping := ParameterMapping{
			Name:        name,
			In:          in,
			Description: desc,
			From:        name, // Default to using same name
			Required:    required,
			Deprecated:  deprecated,
		}

		// Extract default value
		if defaultValue, ok := pObj["default"]; ok {
			paramMapping.Default = defaultValue
		}

		// Extract schema information (type, format, enum)
		// In OpenAPI 3.0, parameter schema is in the "schema" field
		if schema, ok := pObj["schema"].(map[string]interface{}); ok {
			if typ, ok := schema["type"].(string); ok {
				paramMapping.Type = typ
			}
			if format, ok := schema["format"].(string); ok {
				paramMapping.Format = format
			}
			if enum, ok := schema["enum"].([]interface{}); ok {
				paramMapping.Enum = enum
			}
		}

		params = append(params, paramMapping)
	}

	return params
}
