package main

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/cmd/demo-game/store"
	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	backend, err := store.Open("")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.NewStore("demo-game", "demo", backend)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// fakeClient records RegisterFunction calls and satisfies croupier.Client.
type fakeClient struct {
	mu         sync.Mutex
	registered map[string]croupier.FunctionDescriptor
}

func newFakeClient() *fakeClient {
	return &fakeClient{registered: map[string]croupier.FunctionDescriptor{}}
}

func (f *fakeClient) RegisterFunction(desc croupier.FunctionDescriptor, _ croupier.FunctionHandler) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registered[desc.ID] = desc
	return nil
}
func (f *fakeClient) Connect(context.Context) error { return nil }
func (f *fakeClient) Serve(context.Context) error   { return nil }
func (f *fakeClient) Stop() error                   { return nil }
func (f *fakeClient) Close() error                  { return nil }

func expectedFunctionIDs() []string {
	return []string{
		"player.list", "player.get", "player.create", "player.update", "player.delete",
		"player.ban", "player.recharge", "player.batchrecharge", "player.mail",
		"player.batchban", "player.batchunban",
		"mail_task.list", "mail_task.get", "mail_task.create", "mail_task.update",
		"mail_task.retry_failed", "mail_task.cancel",
		"ban.list", "ban.get", "ban.create", "ban.unban", "ban.extend", "ban.batch_unban",
		"order.list", "order.get", "order.recredit",
		"activity.list", "activity.get", "activity.create", "activity.publish_version",
		"activity.activate", "activity.end", "activity.rollback",
		"report.generate",
	}
}

func TestGameFunctionDefinitionsComplete(t *testing.T) {
	st := newTestStore(t)
	defs := gameFunctionDefinitions(st)

	want := expectedFunctionIDs()
	if len(defs) != len(want) {
		t.Fatalf("definitions = %d, want %d", len(defs), len(want))
	}
	seen := map[string]bool{}
	for _, def := range defs {
		id := def.desc.ID
		if seen[id] {
			t.Fatalf("duplicate descriptor %s", id)
		}
		seen[id] = true
		if def.handler == nil {
			t.Fatalf("%s: handler is nil", id)
		}
		if def.desc.Version != "1.0.0" {
			t.Fatalf("%s: version %q", id, def.desc.Version)
		}
		if !def.desc.Enabled {
			t.Fatalf("%s: not enabled", id)
		}
		enriched := enrichDescriptor(def.desc)
		if err := validateDescriptor(enriched); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if enriched.Summary == "" || enriched.Description == "" || len(enriched.Tags) == 0 {
			t.Fatalf("%s: enrichment left summary/description/tags empty", id)
		}
	}
	for _, id := range want {
		if !seen[id] {
			t.Fatalf("missing descriptor %s", id)
		}
	}
}

func TestListContractsUseCollectionEnvelope(t *testing.T) {
	st := newTestStore(t)
	for _, def := range gameFunctionDefinitions(st) {
		if !strings.HasSuffix(def.desc.ID, ".list") {
			continue
		}
		for _, field := range []string{"items", "total", "page", "pageSize"} {
			if !strings.Contains(def.desc.OutputSchema, `"`+field+`"`) {
				t.Fatalf("%s output schema lacks %q: %s", def.desc.ID, field, def.desc.OutputSchema)
			}
		}
	}
}

func TestRegisterGameFunctionsViaClient(t *testing.T) {
	st := newTestStore(t)
	client := newFakeClient()
	if err := registerGameFunctions(client, st); err != nil {
		t.Fatalf("register: %v", err)
	}
	want := expectedFunctionIDs()
	if len(client.registered) != len(want) {
		t.Fatalf("registered %d functions, want %d", len(client.registered), len(want))
	}
	for _, id := range want {
		desc, ok := client.registered[id]
		if !ok {
			t.Fatalf("%s not registered", id)
		}
		if desc.Capability == "" {
			t.Fatalf("%s registered without capability", id)
		}
	}
}

func TestValidateDescriptorRejectsBrokenSchema(t *testing.T) {
	base := croupier.FunctionDescriptor{
		ID: "x.y", Version: "1.0.0", Resource: "x", Operation: "y",
		Capability: "action", Execution: "sync", Enabled: true,
		InputSchema: `{"type":"object"}`, OutputSchema: `{"type":"object"}`,
	}
	if err := validateDescriptor(base); err != nil {
		t.Fatalf("valid descriptor rejected: %v", err)
	}
	cases := []func(c croupier.FunctionDescriptor) croupier.FunctionDescriptor{
		func(c croupier.FunctionDescriptor) croupier.FunctionDescriptor { c.Capability = ""; return c },
		func(c croupier.FunctionDescriptor) croupier.FunctionDescriptor { c.InputSchema = ""; return c },
		func(c croupier.FunctionDescriptor) croupier.FunctionDescriptor { c.InputSchema = "{not json"; return c },
		func(c croupier.FunctionDescriptor) croupier.FunctionDescriptor { c.OutputSchema = ""; return c },
		func(c croupier.FunctionDescriptor) croupier.FunctionDescriptor { c.OutputSchema = "[]"; return c },
	}
	for i, mutate := range cases {
		if err := validateDescriptor(mutate(base)); err == nil {
			t.Fatalf("case %d: broken descriptor accepted", i)
		}
	}
}

// TestEveryWriteContractAcceptsOperationId documents the exactly-once
// extension input: write contracts route through the idempotency protocol and
// must not declare additionalProperties:false (which would reject the
// undeclared operationId input).
func TestEveryWriteContractAcceptsOperationId(t *testing.T) {
	st := newTestStore(t)
	for _, def := range gameFunctionDefinitions(st) {
		if def.desc.Execution != "sync" || def.desc.Risk == "safe" {
			continue // read-only contracts take no operationId
		}
		if strings.Contains(def.desc.InputSchema, `"additionalProperties":false`) {
			t.Fatalf("%s forbids additionalProperties; operationId input would be rejected", def.desc.ID)
		}
	}
}
