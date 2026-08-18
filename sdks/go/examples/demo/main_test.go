package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/xeipuuv/gojsonschema"
)

func TestGameDemoDescriptorsMatchHandlers(t *testing.T) {
	definitions := gameDemoFunctionDefinitions(newDemoStore())
	if len(definitions) == 0 {
		t.Fatal("game demo must register functions")
	}

	for _, definition := range definitions {
		desc := definition.desc
		t.Run(desc.ID, func(t *testing.T) {
			var handler func(context.Context, []byte) ([]byte, error)
			for _, isolated := range gameDemoFunctionDefinitions(newDemoStore()) {
				if isolated.desc.ID == desc.ID {
					handler = isolated.handler
					break
				}
			}
			if handler == nil {
				t.Fatalf("handler not found for %s", desc.ID)
			}
			if err := validateDemoDescriptor(desc); err != nil {
				t.Fatal(err)
			}
			if desc.Risk != "safe" && desc.Risk != "warning" && desc.Risk != "high" && desc.Risk != "danger" {
				t.Fatalf("unsupported risk %q", desc.Risk)
			}
			if desc.Capability == "" || desc.Execution != "sync" {
				t.Fatalf("incomplete execution contract: capability=%q execution=%q", desc.Capability, desc.Execution)
			}

			payload := demoPayloadFor(desc.ID)
			if _, err := gojsonschema.Validate(gojsonschema.NewStringLoader(desc.InputSchema), gojsonschema.NewGoLoader(payload)); err != nil {
				t.Fatalf("input payload does not satisfy descriptor schema: %v", err)
			}

			raw, err := handler(context.Background(), mustJSON(t, payload))
			if err != nil {
				t.Fatalf("handler rejected descriptor-compatible input: %v", err)
			}
			var result any
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatalf("handler returned invalid JSON: %v", err)
			}
			if _, err := gojsonschema.Validate(gojsonschema.NewStringLoader(desc.OutputSchema), gojsonschema.NewGoLoader(result)); err != nil {
				t.Fatalf("handler result does not satisfy descriptor schema: %v\nresult=%s", err, raw)
			}
		})
	}
}

func TestGameDemoLifecycleUsesRegisteredIdentityFields(t *testing.T) {
	store := newDemoStore()
	definitions := make(map[string]demoFunctionDefinition)
	for _, definition := range gameDemoFunctionDefinitions(store) {
		definitions[definition.desc.ID] = definition
	}

	createPlayer := invokeDemo(t, definitions["player.create"], map[string]any{"id": "player_contract", "name": "Contract Player"})
	assertJSONString(t, createPlayer, "/id", "player_contract")

	updatedPlayer := invokeDemo(t, definitions["player.update"], map[string]any{"id": "player_contract", "level": 9})
	assertJSONNumber(t, updatedPlayer, "/level", 9)

	createdOrder := invokeDemo(t, definitions["order.create"], map[string]any{"id": "order_contract", "playerId": "player_contract", "productId": "sku.contract"})
	assertJSONString(t, createdOrder, "/id", "order_contract")

	updatedOrder := invokeDemo(t, definitions["order.update"], map[string]any{"id": "order_contract", "status": "paid"})
	assertJSONString(t, updatedOrder, "/status", "paid")

	granted := invokeDemo(t, definitions["inventory.grant"], map[string]any{"playerId": "player_contract", "templateId": "contract_token", "quantity": 2})
	assertJSONString(t, granted, "/id", inventoryItemID("player_contract", "contract_token"))

	consumed := invokeDemo(t, definitions["inventory.consume"], map[string]any{"playerId": "player_contract", "templateId": "contract_token", "quantity": 1})
	assertJSONNumber(t, consumed, "/quantity", 1)

	sentMail := invokeDemo(t, definitions["mail.send"], map[string]any{"playerId": "player_contract", "title": "Contract mail"})
	mailID := jsonStringAt(t, sentMail, "/id")
	claimedMail := invokeDemo(t, definitions["mail.claim"], map[string]any{"playerId": "player_contract", "id": mailID})
	assertJSONString(t, claimedMail, "/status", "claimed")
}

func demoPayloadFor(functionID string) map[string]any {
	switch functionID {
	case "player.create":
		return map[string]any{"id": "player_schema", "name": "Schema Player"}
	case "player.get", "player.update", "player.delete":
		return map[string]any{"id": "player_1001"}
	case "player.list", "leaderboard.list", "leaderboard.reset":
		return map[string]any{"page": 1, "pageSize": 20}
	case "order.create":
		return map[string]any{"id": "order_schema", "playerId": "player_1001"}
	case "order.get", "order.update", "order.delete":
		return map[string]any{"id": "order_3001"}
	case "order.list":
		return map[string]any{"playerId": "player_1001", "page": 1, "pageSize": 20}
	case "leaderboard.upsert":
		return map[string]any{"playerId": "player_1001", "score": 100000}
	case "inventory.list":
		return map[string]any{"playerId": "player_1001", "page": 1, "pageSize": 20}
	case "inventory.grant":
		return map[string]any{"playerId": "player_1001", "templateId": "schema_token", "quantity": 1}
	case "inventory.consume":
		return map[string]any{"playerId": "player_1001", "templateId": "gold_coin", "quantity": 1}
	case "mail.send":
		return map[string]any{"playerId": "player_1001", "title": "Schema mail"}
	case "mail.list":
		return map[string]any{"playerId": "player_1001", "page": 1, "pageSize": 20}
	case "mail.claim":
		return map[string]any{"playerId": "player_1001", "id": "mail_5001"}
	default:
		return map[string]any{}
	}
}

func invokeDemo(t *testing.T, definition demoFunctionDefinition, payload map[string]any) any {
	t.Helper()
	raw, err := definition.handler(context.Background(), mustJSON(t, payload))
	if err != nil {
		t.Fatalf("%s failed: %v", definition.desc.ID, err)
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("%s returned invalid JSON: %v", definition.desc.ID, err)
	}
	return result
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func jsonStringAt(t *testing.T, value any, pointer string) string {
	t.Helper()
	resolved, err := jsonValueAt(value, pointer)
	if err != nil {
		t.Fatalf("missing %s: %v", pointer, err)
	}
	text, ok := resolved.(string)
	if !ok {
		t.Fatalf("%s = %#v, want string", pointer, resolved)
	}
	return text
}

func assertJSONString(t *testing.T, value any, pointer, want string) {
	t.Helper()
	if got := jsonStringAt(t, value, pointer); got != want {
		t.Fatalf("%s = %q, want %q", pointer, got, want)
	}
}

func assertJSONNumber(t *testing.T, value any, pointer string, want float64) {
	t.Helper()
	resolved, err := jsonValueAt(value, pointer)
	if err != nil {
		t.Fatalf("missing %s: %v", pointer, err)
	}
	got, ok := resolved.(float64)
	if !ok || got != want {
		t.Fatalf("%s = %#v, want %v", pointer, resolved, want)
	}
}

func jsonValueAt(value any, pointer string) (any, error) {
	if pointer == "" {
		return value, nil
	}
	current := value
	for _, token := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q is not an object", token)
		}
		var exists bool
		current, exists = object[token]
		if !exists {
			return nil, fmt.Errorf("field %q not found", token)
		}
	}
	return current, nil
}
