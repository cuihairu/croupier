package provider

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/getkin/kin-openapi/openapi3"
)

func ensureRegistryStore(store *reg.Store) (*reg.Store, error) {
	if store == nil {
		return nil, errorx.NewInternalError("registry store unavailable")
	}
	return store, nil
}

func getProviderCaps(store *reg.Store, id string) (reg.OpenAPIProviderCaps, error) {
	store, err := ensureRegistryStore(store)
	if err != nil {
		return reg.OpenAPIProviderCaps{}, err
	}
	if strings.TrimSpace(id) == "" {
		return reg.OpenAPIProviderCaps{}, errorx.NewBadRequest("provider ID 不能为空")
	}
	caps, err := store.GetOpenAPIProvider(strings.TrimSpace(id))
	if err != nil {
		return reg.OpenAPIProviderCaps{}, errorx.NewNotFound("provider 不存在: " + id)
	}
	return *caps, nil
}

func deleteProviderCaps(store *reg.Store, id string) error {
	store, err := ensureRegistryStore(store)
	if err != nil {
		return err
	}
	if err := store.DeleteOpenAPIProvider(strings.TrimSpace(id)); err != nil {
		return errorx.NewInternalError("删除provider失败")
	}
	return nil
}

func decodeOpenAPIDoc(doc []byte) (*openapi3.T, error) {
	if len(doc) == 0 {
		return nil, nil
	}
	var out openapi3.T
	if err := json.Unmarshal(doc, &out); err != nil {
		return nil, errorx.NewBadRequest("解析OpenAPI文档失败")
	}
	return &out, nil
}

func openAPIDocResources(doc *openapi3.T) []map[string]interface{} {
	if doc == nil {
		return nil
	}
	resourceSet := make(map[string]map[string]interface{})
	for _, pathItem := range doc.Paths.Map() {
		for _, op := range pathItem.Operations() {
			if op.Extensions != nil {
				if resourceExt, exists := op.Extensions["x-resource"]; exists {
					if resourceName, ok := resourceExt.(string); ok && resourceName != "" {
						if _, exists := resourceSet[resourceName]; !exists {
							resourceSet[resourceName] = map[string]interface{}{
								"name": resourceName,
							}
						}
					}
				}
			}
		}
	}

	resources := make([]map[string]interface{}, 0, len(resourceSet))
	for _, resource := range resourceSet {
		resources = append(resources, resource)
	}
	return resources
}

func openAPIDocFunctions(doc *openapi3.T) []map[string]interface{} {
	if doc == nil {
		return nil
	}

	functions := make([]map[string]interface{}, 0)

	for path, pathItem := range doc.Paths.Map() {
		for method, op := range pathItem.Operations() {
			if op == nil {
				continue
			}

			fn := map[string]interface{}{
				"operationId": op.OperationID,
				"method":      strings.ToUpper(method),
				"path":        path,
				"summary":     op.Summary,
			}

			// Extract extensions
			if op.Extensions != nil {
				if resource, exists := op.Extensions["x-resource"]; exists {
					fn["resource"] = resource
				}
				if risk, exists := op.Extensions["x-risk"]; exists {
					fn["risk"] = risk
				}
				if operation, exists := op.Extensions["x-operation"]; exists {
					fn["operation"] = operation
				}
			}

			functions = append(functions, fn)
		}
	}

	return functions
}

func buildProviderMeta(caps reg.OpenAPIProviderCaps, includeDoc bool) map[string]interface{} {
	meta := map[string]interface{}{
		"id":        caps.ID,
		"version":   caps.Version,
		"lang":      caps.Lang,
		"sdk":       caps.SDK,
		"updatedAt": utils.FormatTimestamp(caps.UpdatedAt),
	}
	doc, err := decodeOpenAPIDoc(caps.OpenAPIDoc)
	if err == nil {
		functions := openAPIDocFunctions(doc)
		resources := openAPIDocResources(doc)
		meta["functions"] = len(functions)
		meta["resources"] = len(resources)
		if includeDoc {
			meta["openapi"] = doc
		}
	} else if includeDoc {
		meta["docError"] = err.Error()
	}
	return meta
}

func aggregateResources(store *reg.Store) []map[string]interface{} {
	if store == nil {
		return nil
	}
	providers := store.ListOpenAPIProviders()
	out := make([]map[string]interface{}, 0)
	for _, item := range providers {
		doc, err := decodeOpenAPIDoc(item.OpenAPIDoc)
		if err != nil {
			continue
		}
		if resources := openAPIDocResources(doc); len(resources) > 0 {
			for _, resource := range resources {
				resourceCopy := map[string]interface{}{
					"provider_id": item.ID,
				}
				for k, v := range resource {
					resourceCopy[k] = v
				}
				out = append(out, resourceCopy)
			}
		}
	}
	return out
}

func aggregateResourcesForProvider(store *reg.Store, id string) ([]map[string]interface{}, error) {
	caps, err := getProviderCaps(store, id)
	if err != nil {
		return nil, err
	}
	doc, err := decodeOpenAPIDoc(caps.OpenAPIDoc)
	if err != nil {
		return nil, err
	}
	resources := openAPIDocResources(doc)
	result := make([]map[string]interface{}, 0, len(resources))
	for _, resource := range resources {
		entry := map[string]interface{}{
			"provider_id": caps.ID,
		}
		for k, v := range resource {
			entry[k] = v
		}
		result = append(result, entry)
	}
	return result, nil
}

func refreshProviderTimestamp(store *reg.Store, caps reg.OpenAPIProviderCaps) {
	if store == nil {
		return
	}
	caps.UpdatedAt = time.Now()
	store.UpsertOpenAPIProvider(caps)
}
