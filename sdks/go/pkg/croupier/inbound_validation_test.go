package croupier

import (
	"strings"
	"testing"

	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
)

// F15：provider 侧入站 payload 校验（按函数声明的 input schema）。
func TestTCPManager_ValidateInboundPayload(t *testing.T) {
	newManager := func(validate bool) *TCPManager {
		config := ClientConfig{ValidateInputPayloads: validate}
		manager, err := NewTCPManager(config, map[string]FunctionHandler{})
		if err != nil {
			t.Fatalf("NewTCPManager: %v", err)
		}
		tm := manager.(*TCPManager)
		tm.functions = []*sdkv1.ProviderFunctionDescriptor{
			{
				Id:          "player.ban",
				InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
			},
			{
				Id:          "player.free",
				InputSchema: ``,
			},
			{
				Id:          "player.broken",
				InputSchema: `not-json`,
			},
		}
		return tm
	}

	t.Run("disabled flag skips validation", func(t *testing.T) {
		m := newManager(false)
		if err := m.validateInboundPayload("player.ban", []byte(`{}`)); err != nil {
			t.Fatalf("expected skip when disabled, got %v", err)
		}
	})

	t.Run("valid payload passes", func(t *testing.T) {
		m := newManager(true)
		if err := m.validateInboundPayload("player.ban", []byte(`{"id":"p1"}`)); err != nil {
			t.Fatalf("expected pass, got %v", err)
		}
	})

	t.Run("missing required rejected", func(t *testing.T) {
		m := newManager(true)
		err := m.validateInboundPayload("player.ban", []byte(`{}`))
		if err == nil || !strings.Contains(err.Error(), "payload validation failed") {
			t.Fatalf("expected validation failure, got %v", err)
		}
	})

	t.Run("type mismatch rejected", func(t *testing.T) {
		m := newManager(true)
		err := m.validateInboundPayload("player.ban", []byte(`{"id":123}`))
		if err == nil || !strings.Contains(err.Error(), "payload validation failed") {
			t.Fatalf("expected validation failure, got %v", err)
		}
	})

	t.Run("invalid payload json rejected", func(t *testing.T) {
		m := newManager(true)
		err := m.validateInboundPayload("player.ban", []byte(`not-json`))
		if err == nil || !strings.Contains(err.Error(), "payload must be valid JSON") {
			t.Fatalf("expected json error, got %v", err)
		}
	})

	t.Run("unknown function skips", func(t *testing.T) {
		m := newManager(true)
		if err := m.validateInboundPayload("ghost", []byte(`{}`)); err != nil {
			t.Fatalf("expected skip for unknown function, got %v", err)
		}
	})

	t.Run("empty schema skips", func(t *testing.T) {
		m := newManager(true)
		if err := m.validateInboundPayload("player.free", []byte(`{}`)); err != nil {
			t.Fatalf("expected skip for empty schema, got %v", err)
		}
	})

	t.Run("broken schema skips", func(t *testing.T) {
		m := newManager(true)
		if err := m.validateInboundPayload("player.broken", []byte(`{}`)); err != nil {
			t.Fatalf("expected skip for broken schema, got %v", err)
		}
	})
}
