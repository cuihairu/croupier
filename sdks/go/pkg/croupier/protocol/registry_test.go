// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package protocol

import (
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// mockMessage is a simple protobuf message for testing
// Using wrapperspb.StringValue as a real protobuf message
type mockMessage = wrapperspb.StringValue

// TestNewRegistry verifies that NewRegistry creates an empty registry.
func TestNewRegistry(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()

	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}

	// Should not be able to create any messages from empty registry
	_, err := registry.Create(0x010101)
	if err == nil {
		t.Error("expected error when creating message from empty registry")
	}
}

// TestRegister verifies that Register adds a message factory.
func TestRegister(t *testing.T) {
	t.Parallel()

	t.Run("adds a single message factory", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		msgID := uint32(0x010101)

		factory := func() proto.Message {
			return wrapperspb.String("test")
		}

		registry.Register(msgID, factory)

		msg, err := registry.Create(msgID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		mock, ok := msg.(*mockMessage)
		if !ok {
			t.Fatal("expected mockMessage type")
		}
		if mock.Value != "test" {
			t.Errorf("expected value 'test', got '%s'", mock.Value)
		}
	})

	t.Run("allows overwriting existing factory", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		msgID := uint32(0x010102)

		registry.Register(msgID, func() proto.Message {
			return wrapperspb.String("first")
		})

		registry.Register(msgID, func() proto.Message {
			return wrapperspb.String("second")
		})

		msg, err := registry.Create(msgID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		mock := msg.(*mockMessage)
		if mock.Value != "second" {
			t.Errorf("expected value 'second', got '%s'", mock.Value)
		}
	})

	t.Run("registers multiple message types", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		msgIDs := []uint32{MsgRegisterRequest, MsgHeartbeatRequest, MsgInvokeRequest}

		for _, msgID := range msgIDs {
			registry.Register(msgID, func() proto.Message {
				return wrapperspb.String("registered")
			})
		}

		for _, msgID := range msgIDs {
			_, err := registry.Create(msgID)
			if err != nil {
				t.Errorf("expected no error for msgID 0x%06X, got: %v", msgID, err)
			}
		}
	})
}

// TestRegisterBatch verifies that RegisterBatch adds multiple factories at once.
func TestRegisterBatch(t *testing.T) {
	t.Parallel()

	t.Run("adds multiple message factories", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		types := map[uint32]func() proto.Message{
			0x010101: func() proto.Message { return wrapperspb.String("first") },
			0x010102: func() proto.Message { return wrapperspb.String("second") },
			0x010103: func() proto.Message { return wrapperspb.String("third") },
		}

		registry.RegisterBatch(types)

		// Verify all types were registered
		for msgID := range types {
			_, err := registry.Create(msgID)
			if err != nil {
				t.Errorf("expected no error for msgID 0x%06X, got: %v", msgID, err)
			}
		}
	})

	t.Run("handles empty batch", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		registry.RegisterBatch(map[uint32]func() proto.Message{})

		// Should still work, just empty
		_, err := registry.Create(0x010101)
		if err == nil {
			t.Error("expected error for unregistered message type")
		}
	})

	t.Run("can be called multiple times", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		batch1 := map[uint32]func() proto.Message{
			0x010101: func() proto.Message { return wrapperspb.String("batch1") },
		}
		batch2 := map[uint32]func() proto.Message{
			0x010102: func() proto.Message { return wrapperspb.String("batch2") },
		}

		registry.RegisterBatch(batch1)
		registry.RegisterBatch(batch2)

		// Both batches should be registered
		_, err1 := registry.Create(0x010101)
		_, err2 := registry.Create(0x010102)

		if err1 != nil || err2 != nil {
			t.Error("expected both messages to be registered")
		}
	})
}

// TestCreate verifies that Create creates message instances.
func TestCreate(t *testing.T) {
	t.Parallel()

	t.Run("returns error for unknown message type", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		_, err := registry.Create(0xDEADBEEF)
		if err == nil {
			t.Error("expected error for unknown message type")
		}

		expectedErr := "unknown message type"
		if err.Error() == "" || !containsString(err.Error(), expectedErr) {
			t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("returns new instance each time", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		msgID := uint32(0x010101)

		callCount := 0
		factory := func() proto.Message {
			callCount++
			return wrapperspb.String("test")
		}

		registry.Register(msgID, factory)

		// Create multiple instances
		msg1, _ := registry.Create(msgID)
		msg2, _ := registry.Create(msgID)

		if msg1 == msg2 {
			t.Error("expected different instances")
		}

		if callCount != 2 {
			t.Errorf("expected factory to be called twice, was called %d times", callCount)
		}
	})

	t.Run("preserves message type", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		msgID := uint32(0x010101)

		registry.Register(msgID, func() proto.Message {
			return wrapperspb.String("test value")
		})

		msg, err := registry.Create(msgID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		strValue, ok := msg.(*wrapperspb.StringValue)
		if !ok {
			t.Fatal("expected StringValue type")
		}

		if strValue.Value != "test value" {
			t.Errorf("expected 'test value', got '%s'", strValue.Value)
		}
	})
}

// TestUnmarshal verifies that Unmarshal creates and populates messages.
func TestUnmarshal(t *testing.T) {
	t.Parallel()

	t.Run("unmarshals valid protobuf data", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		msgID := uint32(0x010101)

		// Register StringValue factory
		registry.Register(msgID, func() proto.Message {
			return wrapperspb.String("")
		})

		// Create protobuf data
		original := wrapperspb.String("test data")
		data, err := proto.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		// Unmarshal
		msg, err := registry.Unmarshal(msgID, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		strValue := msg.(*wrapperspb.StringValue)
		if strValue.Value != "test data" {
			t.Errorf("expected 'test data', got '%s'", strValue.Value)
		}
	})

	t.Run("returns error for unknown message type", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		_, err := registry.Unmarshal(0xDEADBEEF, []byte{0x01, 0x02, 0x03})
		if err == nil {
			t.Error("expected error for unknown message type")
		}
	})

	t.Run("returns error for invalid protobuf data", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		msgID := uint32(0x010101)

		registry.Register(msgID, func() proto.Message {
			return wrapperspb.String("")
		})

		// Invalid protobuf data
		_, err := registry.Unmarshal(msgID, []byte{0xFF, 0xFF, 0xFF})
		if err == nil {
			t.Error("expected error for invalid protobuf data")
		}
	})

	t.Run("handles empty body", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		msgID := uint32(0x010101)

		registry.Register(msgID, func() proto.Message {
			return wrapperspb.String("")
		})

		msg, err := registry.Unmarshal(msgID, []byte{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if msg == nil {
			t.Error("expected non-nil message")
		}
	})
}

// TestMustRegister verifies that MustRegister panics on duplicate.
func TestMustRegister(t *testing.T) {
	t.Parallel()

	t.Run("registers new message type", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		msgID := uint32(0x010101)

		registry.MustRegister(msgID, func() proto.Message {
			return wrapperspb.String("must-register")
		})

		// Should be able to create the message
		_, err := registry.Create(msgID)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("panics on duplicate registration", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on duplicate registration")
			}
		}()

		registry := NewRegistry()
		msgID := uint32(0x010101)

		registry.MustRegister(msgID, func() proto.Message {
			return wrapperspb.String("first")
		})

		// This should panic
		registry.MustRegister(msgID, func() proto.Message {
			return wrapperspb.String("second")
		})
	})

	t.Run("panic message contains message ID", func(t *testing.T) {
		t.Parallel()

		defer func() {
			if r := recover(); r != nil {
				panicMsg, ok := r.(string)
				if !ok {
					t.Errorf("expected string panic, got %T", r)
					return
				}
				if !containsString(panicMsg, "0x010101") {
					t.Errorf("expected panic message to contain message ID, got: %s", panicMsg)
				}
			} else {
				t.Error("expected panic")
			}
		}()

		registry := NewRegistry()
		msgID := uint32(0x010101)

		registry.MustRegister(msgID, func() proto.Message {
			return wrapperspb.String("")
		})
		registry.MustRegister(msgID, func() proto.Message {
			return wrapperspb.String("")
		})
	})
}

// TestRegistry_ConcurrentAccess verifies thread safety.
func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	t.Run("concurrent Register operations", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		var wg sync.WaitGroup

		numGoroutines := 100
		msgIDBase := uint32(0x020000)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				msgID := msgIDBase + uint32(idx)
				registry.Register(msgID, func() proto.Message {
					return wrapperspb.String("concurrent")
				})
			}(i)
		}

		wg.Wait()

		// Verify all registrations
		for i := 0; i < numGoroutines; i++ {
			msgID := msgIDBase + uint32(i)
			_, err := registry.Create(msgID)
			if err != nil {
				t.Errorf("message 0x%06X not registered", msgID)
			}
		}
	})

	t.Run("concurrent Create operations", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		msgID := uint32(0x030101)

		registry.Register(msgID, func() proto.Message {
			return wrapperspb.String("concurrent-create")
		})

		var wg sync.WaitGroup
		numGoroutines := 100

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := registry.Create(msgID)
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent Register and Create", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		var wg sync.WaitGroup

		// Register goroutines
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				msgID := uint32(0x040000 + idx)
				registry.Register(msgID, func() proto.Message {
					return wrapperspb.String("mixed")
				})
			}(i)
		}

		// Create goroutines (for already registered types)
		registry.Register(0x050101, func() proto.Message {
			return wrapperspb.String("create-test")
		})

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = registry.Create(0x050101)
			}()
		}

		wg.Wait()
	})

	t.Run("concurrent RegisterBatch operations", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()
		var wg sync.WaitGroup

		numGoroutines := 20

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				batch := make(map[uint32]func() proto.Message)
				for j := 0; j < 10; j++ {
					msgID := uint32(0x060000 + idx*10 + j)
					batch[msgID] = func() proto.Message {
						return wrapperspb.String("batch")
					}
				}

				registry.RegisterBatch(batch)
			}(i)
		}

		wg.Wait()

		// Verify at least some registrations worked
		successCount := 0
		for i := 0; i < numGoroutines*10; i++ {
			msgID := uint32(0x060000 + i)
			if _, err := registry.Create(msgID); err == nil {
				successCount++
			}
		}

		if successCount == 0 {
			t.Error("expected some successful registrations")
		}
	})
}

// TestRegistry_RealMessageTypes tests with actual protocol message types.
func TestRegistry_RealMessageTypes(t *testing.T) {
	t.Parallel()

	t.Run("registers all control service messages", func(t *testing.T) {
		t.Parallel()

		registry := NewRegistry()

		// This would typically register actual protobuf message factories
		// For testing, we use mock factories
		controlMessages := []uint32{
			MsgRegisterRequest,
			MsgRegisterResponse,
			MsgHeartbeatRequest,
			MsgHeartbeatResponse,
			MsgRegisterCapabilitiesReq,
			MsgRegisterCapabilitiesResp,
		}

		for _, msgID := range controlMessages {
			registry.Register(msgID, func() proto.Message {
				return wrapperspb.String("control")
			})
		}

		for _, msgID := range controlMessages {
			_, err := registry.Create(msgID)
			if err != nil {
				t.Errorf("failed to create message 0x%06X: %v", msgID, err)
			}
		}
	})
}

// Helper function for string contains check
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
