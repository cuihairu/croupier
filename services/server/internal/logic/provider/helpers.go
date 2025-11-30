package provider

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
)

func ensureRegistryStore(store *reg.Store) (*reg.Store, error) {
	if store == nil {
		return nil, fmt.Errorf("registry store unavailable")
	}
	return store, nil
}

func getProviderCaps(store *reg.Store, id string) (reg.ProviderCaps, error) {
	store, err := ensureRegistryStore(store)
	if err != nil {
		return reg.ProviderCaps{}, err
	}
	if strings.TrimSpace(id) == "" {
		return reg.ProviderCaps{}, fmt.Errorf("provider ID 不能为空")
	}
	if caps, ok := store.GetProviderCaps(strings.TrimSpace(id)); ok {
		return caps, nil
	}
	return reg.ProviderCaps{}, fmt.Errorf("provider %s 不存在", id)
}

func deleteProviderCaps(store *reg.Store, id string) error {
	store, err := ensureRegistryStore(store)
	if err != nil {
		return err
	}
	if !store.DeleteProviderCaps(strings.TrimSpace(id)) {
		return fmt.Errorf("provider %s 不存在", id)
	}
	return nil
}

func decodeManifest(manifest []byte) (map[string]interface{}, error) {
	if len(manifest) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(manifest, &out); err != nil {
		return nil, fmt.Errorf("解析manifest失败: %w", err)
	}
	return out, nil
}

func manifestArray(man map[string]interface{}, key string) []interface{} {
	if man == nil {
		return nil
	}
	raw, ok := man[key]
	if !ok {
		return nil
	}
	arr, _ := raw.([]interface{})
	return arr
}

func manifestEntities(man map[string]interface{}) []map[string]interface{} {
	raw := manifestArray(man, "entities")
	if len(raw) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if entity, ok := item.(map[string]interface{}); ok {
			out = append(out, entity)
		}
	}
	return out
}

func manifestFunctions(man map[string]interface{}) []map[string]interface{} {
	raw := manifestArray(man, "functions")
	if len(raw) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(raw))
	for _, item := range raw {
		if fn, ok := item.(map[string]interface{}); ok {
			out = append(out, fn)
		}
	}
	return out
}

func buildProviderMeta(caps reg.ProviderCaps, includeManifest bool) map[string]interface{} {
	meta := map[string]interface{}{
		"id":        caps.ID,
		"version":   caps.Version,
		"lang":      caps.Lang,
		"sdk":       caps.SDK,
		"updatedAt": utils.FormatTimestamp(caps.UpdatedAt),
	}
	manifest, err := decodeManifest(caps.Manifest)
	if err == nil {
		functions := manifestFunctions(manifest)
		entities := manifestEntities(manifest)
		meta["functions"] = len(functions)
		meta["entities"] = len(entities)
		if includeManifest {
			meta["manifest"] = manifest
		}
	} else if includeManifest {
		meta["manifestError"] = err.Error()
	}
	return meta
}

func aggregateEntities(store *reg.Store) []map[string]interface{} {
	if store == nil {
		return nil
	}
	caps := store.ListProviderCaps()
	out := make([]map[string]interface{}, 0)
	for _, item := range caps {
		manifest, err := decodeManifest(item.Manifest)
		if err != nil {
			continue
		}
		if entities := manifestEntities(manifest); len(entities) > 0 {
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
	manifest, err := decodeManifest(caps.Manifest)
	if err != nil {
		return nil, err
	}
	entities := manifestEntities(manifest)
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

func refreshProviderTimestamp(store *reg.Store, caps reg.ProviderCaps) {
	if store == nil {
		return
	}
	caps.UpdatedAt = time.Now()
	store.UpsertProviderCaps(caps)
}
