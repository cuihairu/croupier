package function

import (
	"testing"
)

func TestBuildFallbackOpenAPIOperation(t *testing.T) {
	t.Run("player.get", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("player.get")
		if op == nil {
			t.Fatal("expected non-nil operation")
		}
		if op.OperationID != "player.get" {
			t.Errorf("OperationID = %q", op.OperationID)
		}
		if op.Extensions["x-operation"] != "get" {
			t.Errorf("x-operation = %q", op.Extensions["x-operation"])
		}
		if op.Extensions["x-resource"] != "player" {
			t.Errorf("x-resource = %q", op.Extensions["x-resource"])
		}
		if _, exists := op.Extensions["x-entity"]; exists {
			t.Fatalf("x-entity must not be emitted by fallback operation")
		}
		if op.RequestBody == nil {
			t.Fatal("expected request body")
		}
	})

	t.Run("order.list", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("order.list")
		if op == nil {
			t.Fatal("expected non-nil operation")
		}
	})

	t.Run("inventory.grant", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("inventory.grant")
		if op == nil {
			t.Fatal("expected non-nil operation")
		}
	})

	t.Run("leaderboard.upsert", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("leaderboard.upsert")
		if op == nil {
			t.Fatal("expected non-nil operation")
		}
	})

	t.Run("mail.send", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("mail.send")
		if op == nil {
			t.Fatal("expected non-nil operation")
		}
	})

	t.Run("unknown entity", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("custom.action")
		if op == nil {
			t.Fatal("expected non-nil operation")
		}
	})

	t.Run("empty function ID", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("")
		if op != nil {
			t.Error("expected nil for empty function ID")
		}
	})

	t.Run("whitespace only", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("   ")
		if op != nil {
			t.Error("expected nil for whitespace-only function ID")
		}
	})

	t.Run("single part", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("standalone")
		if op == nil {
			t.Fatal("expected non-nil operation")
		}
	})

	t.Run("underscore separated", func(t *testing.T) {
		op := BuildFallbackOpenAPIOperation("game_player_ban")
		if op == nil {
			t.Fatal("expected non-nil operation")
		}
		if op.Extensions["x-resource"] != "player" || op.Extensions["x-operation"] != "ban" {
			t.Fatalf("unexpected extensions: %#v", op.Extensions)
		}
	})
}

func TestBuildFallbackUISchema(t *testing.T) {
	t.Run("player.update uses payload only", func(t *testing.T) {
		schema := BuildFallbackUISchema("player.update")
		if schema == nil {
			t.Fatal("expected non-nil schema")
		}
		props, ok := schema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("expected properties map")
		}
		if _, exists := props["payload"]; !exists {
			t.Error("expected payload property")
		}
		if _, exists := props["playerId"]; exists {
			t.Error("fallback UI must not infer playerId")
		}
	})

	t.Run("order.create", func(t *testing.T) {
		schema := BuildFallbackUISchema("order.create")
		if schema == nil {
			t.Fatal("expected non-nil schema")
		}
	})

	t.Run("mail.claim", func(t *testing.T) {
		schema := BuildFallbackUISchema("mail.claim")
		if schema == nil {
			t.Fatal("expected non-nil schema")
		}
	})

	t.Run("domain.entity.operation", func(t *testing.T) {
		schema := BuildFallbackUISchema("game.player.ban")
		if schema == nil {
			t.Fatal("expected non-nil schema")
		}
		props, ok := schema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("expected properties map")
		}
		if _, exists := props["payload"]; !exists {
			t.Fatalf("expected payload property, got %#v", props)
		}
	})
}

func TestFallbackFields_UsesSinglePayload(t *testing.T) {
	fields := fallbackFields()
	if len(fields) != 1 {
		t.Fatalf("expected single payload field, got %d", len(fields))
	}
	if fields[0].Name != "payload" || fields[0].Type != "object" {
		t.Fatalf("unexpected fallback field: %#v", fields[0])
	}
}

func TestSanitizeFallbackToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"Hello World", "helloworld"},
		{"test-123", "test-123"},
		{"__trim__", "trim"},
		{"", ""},
		{"   ", ""},
		{"a!b@c", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFallbackToken(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFallbackToken(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInferFallbackResourceAction(t *testing.T) {
	tests := []struct {
		input        string
		wantResource string
		wantAction   string
	}{
		{"player.get", "player", "get"},
		{"order.list", "order", "list"},
		{"game.player.ban", "player", "ban"},
		{"game_player_ban", "player", "ban"},
		{"standalone", "standalone", "invoke"},
		{"", "", "invoke"},
		{"Player.Get", "player", "get"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resource, action := inferFallbackResourceAction(tt.input)
			if resource != tt.wantResource || action != tt.wantAction {
				t.Errorf("inferFallbackResourceAction(%q) = (%q, %q), want (%q, %q)",
					tt.input, resource, action, tt.wantResource, tt.wantAction)
			}
		})
	}
}
