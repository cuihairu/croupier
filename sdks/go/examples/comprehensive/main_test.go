package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// stubClient captures registrations and lifecycle calls without networking.
type stubClient struct {
	registered map[string]croupier.FunctionDescriptor
	duplicate  string // ID that the SDK would reject as a duplicate
}

func newStubClient() *stubClient {
	return &stubClient{registered: make(map[string]croupier.FunctionDescriptor)}
}

func (s *stubClient) RegisterFunction(desc croupier.FunctionDescriptor, handler croupier.FunctionHandler) error {
	if desc.ID == "" {
		return fmt.Errorf("function ID cannot be empty")
	}
	if desc.ID == s.duplicate {
		return fmt.Errorf("function already registered: %s", desc.ID)
	}
	s.registered[desc.ID] = desc
	return nil
}
func (s *stubClient) Connect(ctx context.Context) error { return nil }
func (s *stubClient) Serve(ctx context.Context) error   { return nil }
func (s *stubClient) Stop() error                       { return nil }
func (s *stubClient) Close() error                      { return nil }

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func TestComprehensiveHandlersReturnValidJSON(t *testing.T) {
	handlers := map[string]func(context.Context, []byte) ([]byte, error){
		"playerBan":   playerBanHandler,
		"itemCreate":  itemCreateHandler,
		"playerData":  playerDataHandler,
		"guildManage": guildManageHandler,
		"utility":     utilityHandler,
	}
	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			output, err := handler(context.Background(), []byte(`{"input":1}`))
			if err != nil {
				t.Fatalf("handler %s: %v", name, err)
			}
			var decoded map[string]interface{}
			if err := json.Unmarshal(output, &decoded); err != nil {
				t.Fatalf("handler %s returned invalid JSON: %v\n%s", name, err, output)
			}
			if decoded["status"] != "success" {
				t.Fatalf("handler %s status = %v", name, decoded["status"])
			}
			if _, ok := decoded["timestamp"]; ok && name != "guildManage" && name != "utility" {
				// timestamp expected for most handlers
			}
		})
	}
}

func TestComprehensivePlayerBanResult(t *testing.T) {
	output, err := playerBanHandler(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("playerBanHandler: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["action"] != "ban" || decoded["playerId"] != "player_123" {
		t.Fatalf("unexpected ban result: %v", decoded)
	}
}

func TestComprehensiveGuildManageResult(t *testing.T) {
	output, err := guildManageHandler(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["guild_id"] != "guild_456" || decoded["members"] != float64(25) {
		t.Fatalf("unexpected guild result: %v", decoded)
	}
}

// ---------------------------------------------------------------------------
// Registration demo
// ---------------------------------------------------------------------------

func TestComprehensiveDemonstrateClientRegistration(t *testing.T) {
	client := newStubClient()
	if err := demonstrateClientRegistration(client); err != nil {
		t.Fatalf("demonstrateClientRegistration: %v", err)
	}
	for _, id := range []string{"player.ban", "item.create", "player.data", "guild.manage", "util.process"} {
		if _, ok := client.registered[id]; !ok {
			t.Fatalf("expected %s to be registered, got %v", id, keys(client.registered))
		}
	}
}

func TestComprehensiveDemonstrateErrorHandling(t *testing.T) {
	client := newStubClient()
	client.duplicate = "player.ban" // first registration conflicts immediately
	// demonstrateErrorHandling only prints expected failures; it must not panic.
	demonstrateErrorHandling(client)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestComprehensiveGenerateIdempotencyKey(t *testing.T) {
	first := generateIdempotencyKey()
	second := generateIdempotencyKey()
	if first == second {
		t.Fatalf("idempotency keys must differ: %q", first)
	}
	if !strings.HasPrefix(first, "key_") {
		t.Fatalf("unexpected key format: %q", first)
	}
}

func TestComprehensiveGetenv(t *testing.T) {
	t.Setenv("EXAMPLE_COMP_KEY", "value")
	if got := getenv("EXAMPLE_COMP_KEY", "fallback"); got != "value" {
		t.Fatalf("getenv set = %q", got)
	}
	if got := getenv("EXAMPLE_COMP_UNSET", "fallback"); got != "fallback" {
		t.Fatalf("getenv unset = %q", got)
	}
}

func TestComprehensiveConfigurationVariations(t *testing.T) {
	if err := demonstrateConfigurationVariations(); err != nil {
		t.Fatalf("demonstrateConfigurationVariations: %v", err)
	}
}

func TestComprehensiveSetupGracefulShutdownReturnsContext(t *testing.T) {
	client := newStubClient()
	ctx := setupGracefulShutdown(client)
	if ctx == nil {
		t.Fatal("setupGracefulShutdown must return a context")
	}
	select {
	case <-ctx.Done():
		t.Fatal("context should not be cancelled immediately")
	default:
	}
	// The signal goroutine listens for SIGINT/SIGTERM; simply exercise the path.
	time.Sleep(10 * time.Millisecond)
	if err := ctx.Err(); err != nil {
		t.Fatalf("unexpected ctx error: %v", err)
	}
}

func keys(m map[string]croupier.FunctionDescriptor) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
