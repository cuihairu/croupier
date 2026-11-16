package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// lifecycleHarness registers every function against a capturing client,
// reproducing the production wiring (descriptor → handler → store) and
// exposing the handler map for invocation.
type lifecycleHarness struct {
	handlers map[string]croupier.FunctionHandler
}

func newLifecycleHarness(t *testing.T) *lifecycleHarness {
	t.Helper()
	st := newTestStore(t)
	h := &lifecycleHarness{handlers: map[string]croupier.FunctionHandler{}}
	if err := registerGameFunctions(&capturingClient{handlers: h.handlers}, st); err != nil {
		t.Fatal(err)
	}
	return h
}

type capturingClient struct {
	handlers map[string]croupier.FunctionHandler
}

func (c *capturingClient) RegisterFunction(desc croupier.FunctionDescriptor, handler croupier.FunctionHandler) error {
	c.handlers[desc.ID] = handler
	return nil
}
func (c *capturingClient) Connect(context.Context) error { return nil }
func (c *capturingClient) Serve(context.Context) error   { return nil }
func (c *capturingClient) Stop() error                   { return nil }
func (c *capturingClient) Close() error                  { return nil }

func (h *lifecycleHarness) invoke(t *testing.T, id string, payload map[string]any) map[string]any {
	t.Helper()
	handler, ok := h.handlers[id]
	if !ok {
		t.Fatalf("%s not registered", id)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	out, err := handler(context.Background(), raw)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("%s decode: %v", id, err)
	}
	return got
}

func (h *lifecycleHarness) invokeErr(t *testing.T, id string, payload map[string]any) string {
	t.Helper()
	handler, ok := h.handlers[id]
	if !ok {
		t.Fatalf("%s not registered", id)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	_, err = handler(context.Background(), raw)
	if err == nil {
		t.Fatalf("%s: expected error", id)
	}
	return err.Error()
}

// TestWriteReadChainThroughRegisteredHandlers proves the wiring end to end:
// create → get → list → recharge → replay through the same descriptors the
// agent will call.
func TestWriteReadChainThroughRegisteredHandlers(t *testing.T) {
	h := newLifecycleHarness(t)

	created := h.invoke(t, "player.create", map[string]any{"nickname": "lifecycle", "serverId": "s1"})
	playerID := created["id"].(string)

	got := h.invoke(t, "player.get", map[string]any{"id": playerID})
	if got["nickname"] != "lifecycle" || got["serverId"] != "s1" {
		t.Fatalf("player.get = %v", got)
	}

	listed := h.invoke(t, "player.list", map[string]any{"nickname": "lifecycle"})
	if listed["total"].(float64) != 1 {
		t.Fatalf("player.list filtered total = %v", listed["total"])
	}

	payload := map[string]any{"operationId": "op-lifecycle-1", "playerId": playerID, "amount": int64(500)}
	first := h.invoke(t, "player.recharge", payload)
	if first["success"] != true {
		t.Fatalf("recharge = %v", first)
	}
	replay := h.invoke(t, "player.recharge", payload)
	for _, key := range []string{"success", "id", "amount", "approvalId"} {
		if replay[key] != first[key] {
			t.Fatalf("replay %s = %v, first = %v", key, replay[key], first[key])
		}
	}
	after := h.invoke(t, "player.get", map[string]any{"id": playerID})
	if after["balanceGold"].(float64) != 500 {
		t.Fatalf("balance after replay = %v, want exactly one credit", after["balanceGold"])
	}

	// Business failure must surface as a handler error string, not a receipt.
	errMsg := h.invokeErr(t, "player.recharge", map[string]any{"playerId": "999999", "amount": int64(1)})
	if errMsg != "player_not_found" {
		t.Fatalf("recharge unknown player error = %q", errMsg)
	}
}
