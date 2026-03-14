package provider

import (
	"encoding/json"
	"strings"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
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

func openAPIDocEntities(doc *openapi3.T) []map[string]interface{} {
	if doc == nil {
		return nil
	}

	// Extract entities from OpenAPI extensions
	entities := make([]map[string]interface{}, 0)

	// Check if OpenAPI doc has x-entities extension
	if doc.Extensions != nil {
		if entitiesExt, exists := doc.Extensions["x-entities"]; exists {
			if entitiesArr, ok := entitiesExt.([]interface{}); ok {
				for _, item := range entitiesArr {
					if entity, ok := item.(map[string]interface{}); ok {
						entities = append(entities, entity)
					}
				}
			}
		}
	}

	// Also extract entities from operation extensions (x-entity)
	entitySet := make(map[string]map[string]interface{})
	for _, pathItem := range doc.Paths.Map() {
		for _, op := range pathItem.Operations() {
			if op.Extensions != nil {
				if entityExt, exists := op.Extensions["x-entity"]; exists {
					if entityName, ok := entityExt.(string); ok && entityName != "" {
						if _, exists := entitySet[entityName]; !exists {
							entitySet[entityName] = map[string]interface{}{
								"name": entityName,
							}
						}
					}
				}
			}
		}
	}

	// Merge unique entities
	for _, entity := range entitySet {
		entities = append(entities, entity)
	}

	return entities
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
				if cat, exists := op.Extensions["x-category"]; exists {
					fn["category"] = cat
				}
				if risk, exists := op.Extensions["x-risk"]; exists {
					fn["risk"] = risk
				}
				if entity, exists := op.Extensions["x-entity"]; exists {
					fn["entity"] = entity
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
		entities := openAPIDocEntities(doc)
		meta["functions"] = len(functions)
		meta["entities"] = len(entities)
		if includeDoc {
			meta["openapi"] = doc
		}
	} else if includeDoc {
		meta["docError"] = err.Error()
	}
	return meta
}

func aggregateEntities(store *reg.Store) []map[string]interface{} {
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
		if entities := openAPIDocEntities(doc); len(entities) > 0 {
			for _, entity := range entities {
				entityCopy := map[string]interface{}{
					"provider_id": item.ID,
				}
				for k, v := range entity {
					entityCopy[k] = v
				}
				out = append(out, entityCopy)
			}
		}
	}
	return out
}

func aggregateEntitiesForProvider(store *reg.Store, id string) ([]map[string]interface{}, error) {
	caps, err := getProviderCaps(store, id)
	if err != nil {
		return nil, err
	}
	doc, err := decodeOpenAPIDoc(caps.OpenAPIDoc)
	if err != nil {
		return nil, err
	}
	entities := openAPIDocEntities(doc)
	result := make([]map[string]interface{}, 0, len(entities))
	for _, entity := range entities {
		entry := map[string]interface{}{
			"provider_id": caps.ID,
		}
		for k, v := range entity {
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
