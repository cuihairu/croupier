package main

import (
	"context"
	"encoding/json"
	"testing"
)

func contextTODO() context.Context { return context.Background() }

func invokeHandler(t *testing.T, store *gameOpsStore, id string, payload map[string]any) map[string]any {
	t.Helper()
	for _, def := range gameOpsFunctionDefinitions(store) {
		if def.desc.ID != id {
			continue
		}
		raw, err := def.handler(contextTODO(), mustJSON(t, payload))
		if err != nil {
			t.Fatalf("%s failed: %v", id, err)
		}
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("%s returned non-object: %v", id, err)
		}
		return out
	}
	t.Fatalf("function %s not registered", id)
	return nil
}

func invokeHandlerErr(t *testing.T, store *gameOpsStore, id string, payload map[string]any) error {
	t.Helper()
	for _, def := range gameOpsFunctionDefinitions(store) {
		if def.desc.ID != id {
			continue
		}
		_, err := def.handler(contextTODO(), mustJSON(t, payload))
		return err
	}
	t.Fatalf("function %s not registered", id)
	return nil
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
