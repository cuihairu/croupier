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
}

func TestBuildFallbackUISchema(t *testing.T) {
	t.Run("player.update", func(t *testing.T) {
		schema := BuildFallbackUISchema("player.update")
		if schema == nil {
			t.Fatal("expected non-nil schema")
		}
		props, ok := schema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("expected properties map")
		}
		if _, exists := props["playerId"]; !exists {
			t.Error("expected playerId property")
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
}

func TestFallbackFields_AllEntities(t *testing.T) {
	tests := []struct {
		entity    string
		action    string
		minFields int
	}{
		{"player", "list", 2},
		{"player", "get", 1},
		{"player", "create", 2},
		{"player", "update", 2},
		{"player", "delete", 1},
		{"player", "unknown", 1},
		{"order", "list", 2},
		{"order", "get", 1},
		{"order", "create", 3},
		{"order", "update", 2},
		{"order", "delete", 1},
		{"order", "unknown", 1},
		{"inventory", "list", 2},
		{"inventory", "grant", 3},
		{"inventory", "consume", 3},
		{"inventory", "unknown", 1},
		{"leaderboard", "list", 2},
		{"leaderboard", "upsert", 3},
		{"leaderboard", "reset", 1},
		{"leaderboard", "unknown", 1},
		{"mail", "list", 2},
		{"mail", "claim", 2},
		{"mail", "send", 3},
		{"mail", "unknown", 1},
		{"unknown", "action", 2},
	}

	for _, tt := range tests {
		t.Run(tt.entity+"."+tt.action, func(t *testing.T) {
			fields := fallbackFields(tt.entity, tt.action)
			if len(fields) < tt.minFields {
				t.Errorf("expected at least %d fields, got %d", tt.minFields, len(fields))
			}
		})
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

func TestInferFallbackEntityAction(t *testing.T) {
	tests := []struct {
		input      string
		wantEntity string
		wantAction string
	}{
		{"player.get", "player", "get"},
		{"order.list", "order", "list"},
		{"standalone", "standalone", "invoke"},
		{"", "", "invoke"},
		{"Player.Get", "player", "get"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			entity, action := inferFallbackEntityAction(tt.input)
			if entity != tt.wantEntity || action != tt.wantAction {
				t.Errorf("inferFallbackEntityAction(%q) = (%q, %q), want (%q, %q)",
					tt.input, entity, action, tt.wantEntity, tt.wantAction)
			}
		})
	}
}
