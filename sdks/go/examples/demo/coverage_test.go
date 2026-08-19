package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// fakeDemoClient records RegisterFunction calls.
type fakeDemoClient struct {
	registered []croupier.FunctionDescriptor
	failOn     string
}

func (f *fakeDemoClient) RegisterFunction(desc croupier.FunctionDescriptor, _ croupier.FunctionHandler) error {
	if desc.ID == f.failOn {
		return errors.New("registration rejected")
	}
	f.registered = append(f.registered, desc)
	return nil
}
func (f *fakeDemoClient) Connect(ctx context.Context) error { return nil }
func (f *fakeDemoClient) Serve(ctx context.Context) error   { return nil }
func (f *fakeDemoClient) Stop() error                       { return nil }
func (f *fakeDemoClient) Close() error                      { return nil }

// ---------------------------------------------------------------------------
// registerFunction / enrichDescriptor / validateDemoDescriptor
// ---------------------------------------------------------------------------

func TestRegisterGameDemoFunctions(t *testing.T) {
	client := &fakeDemoClient{}
	if err := registerGameDemoFunctions(client, newDemoStore()); err != nil {
		t.Fatalf("registerGameDemoFunctions: %v", err)
	}
	if len(client.registered) != 19 {
		t.Fatalf("registered = %d functions, want 19", len(client.registered))
	}
	for _, desc := range client.registered {
		if desc.Tags == nil || desc.Summary == "" || desc.Description == "" {
			t.Fatalf("descriptor %s not enriched: %+v", desc.ID, desc)
		}
	}
}

func TestRegisterGameDemoFunctions_ClientError(t *testing.T) {
	client := &fakeDemoClient{failOn: "player.get"}
	err := registerGameDemoFunctions(client, newDemoStore())
	if err == nil || !contains(err.Error(), "register player.get failed") {
		t.Fatalf("expected wrapped registration error, got %v", err)
	}
}

func TestRegisterFunction_DescriptorValidation(t *testing.T) {
	client := &fakeDemoClient{}
	err := registerFunction(client, croupier.FunctionDescriptor{ID: "bad.fn"}, func(context.Context, []byte) ([]byte, error) {
		return nil, nil
	})
	if err == nil || !contains(err.Error(), "missing capability") {
		t.Fatalf("expected capability error, got %v", err)
	}
}

func TestEnrichDescriptor(t *testing.T) {
	enriched := enrichDescriptor(croupier.FunctionDescriptor{ID: "demo.fn", Resource: "player", Operation: "ban"})
	if len(enriched.Tags) != 2 || enriched.Tags[0] != "player" || enriched.Tags[1] != "ban" {
		t.Fatalf("tags = %v", enriched.Tags)
	}
	if enriched.Summary != "player ban" {
		t.Fatalf("summary = %q", enriched.Summary)
	}
	if enriched.Description == "" {
		t.Fatal("description should be generated")
	}

	kept := enrichDescriptor(croupier.FunctionDescriptor{
		ID: "demo.fn", Tags: []string{"x"}, Summary: "s", Description: "d",
	})
	if kept.Tags[0] != "x" || kept.Summary != "s" || kept.Description != "d" {
		t.Fatalf("existing fields were overridden: %+v", kept)
	}
}

func TestValidateDemoDescriptor_Branches(t *testing.T) {
	if err := validateDemoDescriptor(croupier.FunctionDescriptor{ID: "f"}); err == nil {
		t.Fatal("expected missing capability error")
	}
	if err := validateDemoDescriptor(croupier.FunctionDescriptor{ID: "f", Capability: "action"}); err == nil {
		t.Fatal("expected invalid input schema error")
	}
	if err := validateDemoDescriptor(croupier.FunctionDescriptor{
		ID: "f", Capability: "action", InputSchema: `{}`,
	}); err == nil {
		t.Fatal("expected invalid output schema error")
	}
	if err := validateDemoDescriptor(croupier.FunctionDescriptor{
		ID: "f", Capability: "action", InputSchema: `{}`, OutputSchema: `{"type":"object"}`,
	}); err != nil {
		t.Fatalf("valid descriptor rejected: %v", err)
	}
}

// ---------------------------------------------------------------------------
// payload helpers
// ---------------------------------------------------------------------------

type demoStringer struct{ value string }

func (d demoStringer) String() string { return d.value }

func TestDecodePayload_Branches(t *testing.T) {
	body, err := decodePayload(nil)
	if err != nil || len(body) != 0 {
		t.Fatalf("empty payload = %v, %v", body, err)
	}
	if _, err := decodePayload([]byte("not json")); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestStringValue_Branches(t *testing.T) {
	body := map[string]any{
		"str":      "  padded  ",
		"blank":    "   ",
		"stringer": demoStringer{value: "from-stringer"},
		"float":    float64(42),
		"int":      7,
		"int64":    int64(-9),
		"other":    []string{"nope"},
	}
	if got := stringValue(body, "str"); got != "padded" {
		t.Fatalf("str = %q", got)
	}
	if got := stringValue(body, "stringer"); got != "from-stringer" {
		t.Fatalf("stringer = %q", got)
	}
	if got := stringValue(body, "float"); got != "42" {
		t.Fatalf("float = %q", got)
	}
	if got := stringValue(body, "int"); got != "7" {
		t.Fatalf("int = %q", got)
	}
	if got := stringValue(body, "int64"); got != "-9" {
		t.Fatalf("int64 = %q", got)
	}
	if got := stringValue(body, "other"); got != "" {
		t.Fatalf("other = %q", got)
	}
	if got := stringValue(body, "missing"); got != "" {
		t.Fatalf("missing = %q", got)
	}
	// Blank first value falls through to the next key.
	if got := stringValue(map[string]any{"a": " ", "b": "second"}, "a", "b"); got != "second" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestInt64Value_Branches(t *testing.T) {
	body := map[string]any{
		"float":  float64(10),
		"int":    11,
		"int64":  int64(12),
		"parsed": " 13 ",
		"bad":    "not-a-number",
	}
	if got := int64Value(body, 0, "float"); got != 10 {
		t.Fatalf("float = %d", got)
	}
	if got := int64Value(body, 0, "int"); got != 11 {
		t.Fatalf("int = %d", got)
	}
	if got := int64Value(body, 0, "int64"); got != 12 {
		t.Fatalf("int64 = %d", got)
	}
	if got := int64Value(body, 0, "parsed"); got != 13 {
		t.Fatalf("parsed = %d", got)
	}
	if got := int64Value(body, 99, "bad"); got != 99 {
		t.Fatalf("bad fallback = %d", got)
	}
	if got := int64Value(body, 5, "missing"); got != 5 {
		t.Fatalf("missing fallback = %d", got)
	}
}

func TestMapValue_Branches(t *testing.T) {
	body := map[string]any{"obj": map[string]any{"a": 1}, "notobj": "x"}
	if m := mapValue(body, "obj"); m["a"] != 1 {
		t.Fatalf("obj = %v", m)
	}
	if m := mapValue(body, "notobj"); m != nil {
		t.Fatalf("notobj = %v", m)
	}
	if m := mapValue(body, "missing"); m != nil {
		t.Fatalf("missing = %v", m)
	}
}

func TestPaginate_Branches(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	got, page, size := paginate(items, map[string]any{"page": 0, "pageSize": 0})
	if len(got) != 5 || page != 1 || size != 20 {
		t.Fatalf("defaults: %v, %d, %d", got, page, size)
	}

	got, _, _ = paginate(items, map[string]any{"page": -3, "pageSize": -1})
	if page2 := got; len(page2) == 0 {
		t.Fatal("negative params should clamp to page 1")
	}

	got, _, size = paginate(items, map[string]any{"page": 1, "pageSize": 500})
	if size != 100 {
		t.Fatalf("pageSize cap = %d", size)
	}

	got, _, _ = paginate(items, map[string]any{"page": 2, "pageSize": 2})
	if len(got) != 2 || got[0] != 3 {
		t.Fatalf("second page = %v", got)
	}

	got, _, _ = paginate(items, map[string]any{"page": 9, "pageSize": 2})
	if len(got) != 0 {
		t.Fatalf("beyond end = %v", got)
	}

	// pageSize larger than remainder trims the slice.
	got, _, _ = paginate(items, map[string]any{"page": 2, "pageSize": 4})
	if len(got) != 1 || got[0] != 5 {
		t.Fatalf("tail page = %v", got)
	}
}

func TestNextIDs(t *testing.T) {
	store := newDemoStore()
	p1 := store.nextPlayerID()
	p2 := store.nextPlayerID()
	if p1 == p2 {
		t.Fatalf("player IDs not unique: %s", p1)
	}
	o1 := store.nextOrderID()
	o2 := store.nextOrderID()
	if o1 == o2 {
		t.Fatalf("order IDs not unique: %s", o1)
	}
	m1 := store.nextMailID()
	m2 := store.nextMailID()
	if m1 == m2 {
		t.Fatalf("mail IDs not unique: %s", m1)
	}
}

// ---------------------------------------------------------------------------
// store handler error paths
// ---------------------------------------------------------------------------

func runHandler(t *testing.T, store *demoStore, fn func(context.Context, []byte) ([]byte, error), payload string) error {
	t.Helper()
	_, err := fn(context.Background(), []byte(payload))
	return err
}

func TestDemoStore_HandlerErrorPaths(t *testing.T) {
	store := newDemoStore()

	cases := []struct {
		name    string
		fn      func(context.Context, []byte) ([]byte, error)
		payload string
	}{
		{"playerGet invalid json", store.playerGet, "not json"},
		{"playerGet missing id", store.playerGet, `{}`},
		{"playerGet unknown player", store.playerGet, `{"playerId":"ghost"}`},
		{"playerUpdate missing id", store.playerUpdate, `{}`},
		{"playerUpdate unknown player", store.playerUpdate, `{"playerId":"ghost"}`},
		{"playerDelete missing id", store.playerDelete, `{}`},
		{"orderGet invalid json", store.orderGet, "bad"},
		{"orderGet missing id", store.orderGet, `{}`},
		{"orderGet unknown order", store.orderGet, `{"orderId":"ghost"}`},
		{"orderUpdate missing id", store.orderUpdate, `{}`},
		{"orderUpdate unknown order", store.orderUpdate, `{"orderId":"ghost"}`},
		{"orderDelete missing id", store.orderDelete, `{}`},
		{"orderList invalid json", store.orderList, "bad"},
		{"leaderboardUpsert missing player", store.leaderboardUpsert, `{}`},
		{"inventoryList missing player", store.inventoryList, `{}`},
		{"inventoryGrant missing template", store.inventoryGrant, `{"playerId":"player_1001"}`},
		{"inventoryConsume missing player", store.inventoryConsume, `{}`},
		{"inventoryConsume unknown item", store.inventoryConsume, `{"playerId":"player_1001","templateId":"ghost"}`},
		{"inventoryConsume insufficient", store.inventoryConsume, `{"playerId":"player_1001","templateId":"hero_ticket","quantity":9999}`},
		{"mailSend missing player", store.mailSend, `{}`},
		{"mailList missing player", store.mailList, `{}`},
		{"mailClaim missing mail", store.mailClaim, `{"playerId":"player_1001"}`},
		{"mailClaim unknown mail", store.mailClaim, `{"playerId":"player_1001","mailId":"ghost"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := runHandler(t, store, tc.fn, tc.payload); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDemoStore_PlayerUpdateOptionalFields(t *testing.T) {
	store := newDemoStore()
	out, err := store.playerUpdate(context.Background(), []byte(`{"playerId":"player_1001","gold":999,"profile":{"vip":9}}`))
	if err != nil {
		t.Fatalf("playerUpdate: %v", err)
	}
	if !contains(string(out), "player_1001") {
		t.Fatalf("unexpected output: %s", out)
	}
	// Second update without optional keys keeps values.
	out, err = store.playerUpdate(context.Background(), []byte(`{"playerId":"player_1001"}`))
	if err != nil {
		t.Fatalf("playerUpdate second: %v", err)
	}
	if !contains(string(out), "player_1001") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestDemoStore_OrderUpdateOptionalFields(t *testing.T) {
	store := newDemoStore()
	if _, err := store.orderUpdate(context.Background(), []byte(`{"orderId":"order_3001","status":"refunded","attributes":{"k":"v"}}`)); err != nil {
		t.Fatalf("orderUpdate: %v", err)
	}
	if _, err := store.orderUpdate(context.Background(), []byte(`{"orderId":"order_3001"}`)); err != nil {
		t.Fatalf("orderUpdate second: %v", err)
	}
}

func TestDemoStore_PlayerDeleteCascades(t *testing.T) {
	store := newDemoStore()
	if _, err := store.playerDelete(context.Background(), []byte(`{"playerId":"player_1001"}`)); err != nil {
		t.Fatalf("playerDelete: %v", err)
	}
	if err := runHandler(t, store, store.playerGet, `{"playerId":"player_1001"}`); err == nil {
		t.Fatal("player should be gone")
	}
	// Inventory and mails are removed too.
	if err := runHandler(t, store, store.inventoryList, `{"playerId":"player_1001"}`); err != nil {
		t.Fatalf("inventory for deleted player should be empty-listable: %v", err)
	}
}

func TestDemoStore_LeaderboardAndMailFlows(t *testing.T) {
	store := newDemoStore()
	if _, err := store.leaderboardUpsert(context.Background(), []byte(`{"playerId":"player_1001","score":123}`)); err != nil {
		t.Fatalf("leaderboardUpsert: %v", err)
	}
	if _, err := store.leaderboardReset(context.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("leaderboardReset: %v", err)
	}

	if _, err := store.mailSend(context.Background(), []byte(`{"playerId":"player_1002","title":"hi"}`)); err != nil {
		t.Fatalf("mailSend: %v", err)
	}
	if _, err := store.mailClaim(context.Background(), []byte(`{"playerId":"player_1001","mailId":"mail_5001"}`)); err != nil {
		t.Fatalf("mailClaim: %v", err)
	}
}

func TestDemoStore_InventoryGrantTwice(t *testing.T) {
	store := newDemoStore()
	if _, err := store.inventoryGrant(context.Background(), []byte(`{"playerId":"player_1002","templateId":"gem","quantity":5}`)); err != nil {
		t.Fatalf("first grant: %v", err)
	}
	if _, err := store.inventoryGrant(context.Background(), []byte(`{"playerId":"player_1002","templateId":"gem","quantity":3}`)); err != nil {
		t.Fatalf("second grant: %v", err)
	}
	if _, err := store.inventoryConsume(context.Background(), []byte(`{"playerId":"player_1002","templateId":"gem","quantity":8}`)); err != nil {
		t.Fatalf("consume: %v", err)
	}
}

func TestDemoStore_ListsWithFiltersAndPaging(t *testing.T) {
	store := newDemoStore()

	out, err := store.playerList(context.Background(), []byte(`{"page":1,"pageSize":1}`))
	if err != nil {
		t.Fatalf("playerList: %v", err)
	}
	if !contains(string(out), `"total":2`) {
		t.Fatalf("unexpected list payload: %s", out)
	}

	if _, err := store.orderList(context.Background(), []byte(`{"playerId":"player_1001"}`)); err != nil {
		t.Fatalf("orderList: %v", err)
	}
	if _, err := store.leaderboardList(context.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("leaderboardList: %v", err)
	}
	if _, err := store.inventoryList(context.Background(), []byte(`{"playerId":"player_1001"}`)); err != nil {
		t.Fatalf("inventoryList: %v", err)
	}
	if _, err := store.mailList(context.Background(), []byte(`{"playerId":"player_1001","page":9,"pageSize":2}`)); err != nil {
		t.Fatalf("mailList: %v", err)
	}
}

func TestDemoStore_PlayerCreateDefaults(t *testing.T) {
	store := newDemoStore()
	out, err := store.playerCreate(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("playerCreate: %v", err)
	}
	if !contains(string(out), "player_") {
		t.Fatalf("auto ID missing: %s", out)
	}
	if _, err := store.playerCreate(context.Background(), []byte("bad")); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestGetenv(t *testing.T) {
	t.Setenv("DEMO_GETENV_KEY", "value")
	if got := getenv("DEMO_GETENV_KEY", "fallback"); got != "value" {
		t.Fatalf("getenv = %q", got)
	}
	if got := getenv("DEMO_MISSING_KEY", "fallback"); got != "fallback" {
		t.Fatalf("getenv fallback = %q", got)
	}
}

func TestFirstNonEmptyDemo(t *testing.T) {
	if got := firstNonEmpty(" ", " real ", ""); got != "real" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
	if got := firstNonEmpty("", "  "); got != "" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

var _ = fmt.Sprintf
