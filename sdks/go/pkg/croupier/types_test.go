// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"strings"
	"testing"
)

// TestLocalFunctionDescriptor_comprehensive tests LocalFunctionDescriptor
func TestLocalFunctionDescriptor_comprehensive(t *testing.T) {
	t.Run("minimal valid descriptor", func(t *testing.T) {
		desc := LocalFunctionDescriptor{
			ID:      "player.ban",
			Version: "1.0.0",
		}

		if desc.ID != "player.ban" {
			t.Errorf("ID = %s, want player.ban", desc.ID)
		}

		if desc.Version != "1.0.0" {
			t.Errorf("Version = %s, want 1.0.0", desc.Version)
		}
	})

	t.Run("full descriptor with all fields", func(t *testing.T) {
		desc := LocalFunctionDescriptor{
			ID:          "player.create",
			Version:     "2.1.0",
			Tags:        []string{"player", "crud"},
			Summary:     "Create a new player",
			Description: "Creates a new player with the given properties",
			OperationID: "createPlayer",
			Deprecated:  false,
			InputSchema: `{"type":"object","properties":{"name":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"id":{"type":"string"}}}`,
			Category:     "player",
			Risk:         "safe",
			Entity:       "Player",
			Operation:    "create",
		}

		if desc.ID != "player.create" {
			t.Errorf("ID = %s, want player.create", desc.ID)
		}

		if len(desc.Tags) != 2 {
			t.Errorf("Tags length = %d, want 2", len(desc.Tags))
		}

		if desc.Summary != "Create a new player" {
			t.Errorf("Summary = %s, want 'Create a new player'", desc.Summary)
		}

		if desc.InputSchema == "" {
			t.Error("InputSchema should not be empty")
		}

		if desc.OutputSchema == "" {
			t.Error("OutputSchema should not be empty")
		}
	})

	t.Run("descriptor with empty tags", func(t *testing.T) {
		desc := LocalFunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
			Tags:    []string{},
		}

		if desc.Tags == nil {
			t.Error("Tags should be empty slice, not nil")
		}

		if len(desc.Tags) != 0 {
			t.Errorf("Tags length = %d, want 0", len(desc.Tags))
		}
	})

	t.Run("descriptor with nil tags", func(t *testing.T) {
		desc := LocalFunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
			Tags:    nil,
		}

		if desc.Tags != nil {
			t.Error("Tags should be nil")
		}
	})

	t.Run("descriptor with multiple tags", func(t *testing.T) {
		tags := []string{"player", "inventory", "admin", "crud", "async"}
		desc := LocalFunctionDescriptor{
			ID:      "player.update",
			Version: "1.0.0",
			Tags:    tags,
		}

		if len(desc.Tags) != 5 {
			t.Errorf("Tags length = %d, want 5", len(desc.Tags))
		}

		for i, tag := range desc.Tags {
			if tag != tags[i] {
				t.Errorf("Tags[%d] = %s, want %s", i, tag, tags[i])
			}
		}
	})

	t.Run("descriptor deprecated and not deprecated", func(t *testing.T) {
		deprecatedDesc := LocalFunctionDescriptor{
			ID:         "old.func",
			Version:    "1.0.0",
			Deprecated: true,
		}

		if !deprecatedDesc.Deprecated {
			t.Error("Deprecated descriptor should have Deprecated=true")
		}

		activeDesc := LocalFunctionDescriptor{
			ID:         "new.func",
			Version:    "1.0.0",
			Deprecated: false,
		}

		if activeDesc.Deprecated {
			t.Error("Active descriptor should have Deprecated=false")
		}
	})

	t.Run("descriptor with various risk levels", func(t *testing.T) {
		risks := []string{"safe", "warning", "danger"}

		for _, risk := range risks {
			desc := LocalFunctionDescriptor{
				ID:      "test.func",
				Version: "1.0.0",
				Risk:    risk,
			}

			if desc.Risk != risk {
				t.Errorf("Risk = %s, want %s", desc.Risk, risk)
			}
		}
	})

	t.Run("descriptor with various operations", func(t *testing.T) {
		operations := []string{"create", "read", "update", "delete", "custom"}

		for _, op := range operations {
			desc := LocalFunctionDescriptor{
				ID:        "test.func",
				Version:   "1.0.0",
				Operation: op,
			}

			if desc.Operation != op {
				t.Errorf("Operation = %s, want %s", desc.Operation, op)
			}
		}
	})

	t.Run("descriptor with various categories", func(t *testing.T) {
		categories := []string{"game", "system", "player", "monitoring", "inventory"}

		for _, cat := range categories {
			desc := LocalFunctionDescriptor{
				ID:       "test.func",
				Version:  "1.0.0",
				Category: cat,
			}

			if desc.Category != cat {
				t.Errorf("Category = %s, want %s", desc.Category, cat)
			}
		}
	})

	t.Run("descriptor with empty and whitespace strings", func(t *testing.T) {
		desc := LocalFunctionDescriptor{
			ID:          "",
			Version:     "",
			Summary:     "",
			Description: "",
			OperationID: "",
		}

		if desc.ID != "" {
			t.Error("ID should be empty")
		}

		if desc.Summary != "" {
			t.Error("Summary should be empty")
		}
	})

	t.Run("descriptor with markdown description", func(t *testing.T) {
		markdownDesc := `# Function Description

This is a **markdown** description with:
- Lists
- **Bold text**
- *Italic text*
- [Links](https://example.com)
`

		desc := LocalFunctionDescriptor{
			ID:          "markdown.func",
			Version:     "1.0.0",
			Description: markdownDesc,
		}

		if !strings.Contains(desc.Description, "# Function Description") {
			t.Error("Description should contain markdown heading")
		}

		if !strings.Contains(desc.Description, "**markdown**") {
			t.Error("Description should contain markdown formatting")
		}
	})

	t.Run("descriptor with JSON schemas", func(t *testing.T) {
		inputSchema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "name": {"type": "string"},
    "age": {"type": "integer"}
  },
  "required": ["name"]
}`

		outputSchema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "id": {"type": "string"},
    "created": {"type": "string", "format": "date-time"}
  }
}`

		desc := LocalFunctionDescriptor{
			ID:          "schema.func",
			Version:     "1.0.0",
			InputSchema:  inputSchema,
			OutputSchema: outputSchema,
		}

		if !strings.Contains(desc.InputSchema, "name") {
			t.Error("InputSchema should contain 'name' property")
		}

		if !strings.Contains(desc.OutputSchema, "id") {
			t.Error("OutputSchema should contain 'id' property")
		}
	})
}

// TestGenerateUUID tests UUID generation
func TestGenerateUUID(t *testing.T) {
	t.Run("generates non-empty UUID", func(t *testing.T) {
		uuid := generateUUID()

		if uuid == "" {
			t.Error("generateUUID() should not return empty string")
		}
	})

	t.Run("generates unique UUIDs", func(t *testing.T) {
		uuids := make(map[string]bool)
		numUUIDs := 100

		for i := 0; i < numUUIDs; i++ {
			uuid := generateUUID()

			if uuids[uuid] {
				t.Errorf("UUID %s is not unique (iteration %d)", uuid, i)
			}

			uuids[uuid] = true
		}

		if len(uuids) != numUUIDs {
			t.Errorf("Generated %d unique UUIDs, want %d", len(uuids), numUUIDs)
		}
	})

	t.Run("UUID format is correct", func(t *testing.T) {
		uuid := generateUUID()

		// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
		// Our format: xxxxxxxx-xxxx-xxxx-xxxx-xxxx (simplified)
		parts := strings.Split(uuid, "-")

		if len(parts) != 5 {
			t.Errorf("UUID has %d parts, want 5", len(parts))
		}

		// Each part should be hex string
		for i, part := range parts {
			if len(part) == 0 {
				t.Errorf("UUID part %d is empty", i)
			}

			// Check that part contains only hex characters
			for _, c := range part {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("UUID part %d contains non-hex character: %c", i, c)
				}
			}
		}
	})

	t.Run("UUID length is consistent", func(t *testing.T) {
		uuids := make([]string, 10)
		lengths := make(map[int]bool)

		for i := 0; i < 10; i++ {
			uuids[i] = generateUUID()
			lengths[len(uuids[i])] = true
		}

		// All UUIDs should have the same length
		if len(lengths) != 1 {
			t.Errorf("UUIDs have inconsistent lengths: %v", lengths)
		}
	})
}

// TestDefaultClientConfig_comprehensive tests DefaultClientConfig function
func TestDefaultClientConfig_comprehensive(t *testing.T) {
	t.Run("returns non-nil config", func(t *testing.T) {
		config := DefaultClientConfig()

		if config == nil {
			t.Fatal("DefaultClientConfig() should not return nil")
		}
	})

	t.Run("has required fields populated", func(t *testing.T) {
		config := DefaultClientConfig()

		if config.ServiceID == "" {
			t.Error("ServiceID should not be empty")
		}

		if config.AgentAddr == "" {
			t.Error("AgentAddr should not be empty")
		}

		if config.Env == "" {
			t.Error("Env should not be empty")
		}

		if config.ServiceVersion == "" {
			t.Error("ServiceVersion should not be empty")
		}

		if config.ProviderLang == "" {
			t.Error("ProviderLang should not be empty")
		}

		if config.ProviderSDK == "" {
			t.Error("ProviderSDK should not be empty")
		}
	})

	t.Run("has expected default values", func(t *testing.T) {
		config := DefaultClientConfig()

		if config.AgentAddr != "localhost:19090" {
			t.Errorf("AgentAddr = %s, want localhost:19090", config.AgentAddr)
		}

		if config.Env != "development" {
			t.Errorf("Env = %s, want development", config.Env)
		}

		if config.ServiceVersion != "1.0.0" {
			t.Errorf("ServiceVersion = %s, want 1.0.0", config.ServiceVersion)
		}

		if config.TimeoutSeconds != 30 {
			t.Errorf("TimeoutSeconds = %d, want 30", config.TimeoutSeconds)
		}

		if config.HeartbeatInterval != 60 {
			t.Errorf("HeartbeatInterval = %d, want 60", config.HeartbeatInterval)
		}

		if !config.Insecure {
			t.Error("Insecure should be true by default")
		}

		if config.Headers == nil {
			t.Error("Headers should be initialized")
		}

		if config.ProviderLang != "go" {
			t.Errorf("ProviderLang = %s, want go", config.ProviderLang)
		}

		if config.ProviderSDK != "croupier-go-sdk" {
			t.Errorf("ProviderSDK = %s, want croupier-go-sdk", config.ProviderSDK)
		}

		if config.LogLevel != "INFO" {
			t.Errorf("LogLevel = %s, want INFO", config.LogLevel)
		}

		if config.Reconnect == nil {
			t.Error("Reconnect should be initialized")
		}

		if config.Retry == nil {
			t.Error("Retry should be initialized")
		}

		if config.MaxFileSize != 10*1024*1024 {
			t.Errorf("MaxFileSize = %d, want 10485760", config.MaxFileSize)
		}
	})

	t.Run("ServiceID includes go-sdk prefix", func(t *testing.T) {
		config := DefaultClientConfig()

		if !strings.HasPrefix(config.ServiceID, "go-sdk-") {
			t.Errorf("ServiceID = %s, should start with 'go-sdk-'", config.ServiceID)
		}
	})

	t.Run("multiple calls generate unique ServiceIDs", func(t *testing.T) {
		config1 := DefaultClientConfig()
		config2 := DefaultClientConfig()

		if config1.ServiceID == config2.ServiceID {
			t.Error("ServiceID should be unique for each call")
		}
	})

	t.Run("config can be modified without affecting defaults", func(t *testing.T) {
		config1 := DefaultClientConfig()
		originalServiceID := config1.ServiceID

		config1.ServiceID = "modified-service-id"
		config1.TimeoutSeconds = 60

		config2 := DefaultClientConfig()

		if config2.ServiceID == "modified-service-id" {
			t.Error("Modifying config should not affect subsequent calls")
		}

		if config2.ServiceID == originalServiceID {
			t.Error("ServiceID should be unique, not reused")
		}

		if config2.TimeoutSeconds != 30 {
			t.Errorf("TimeoutSeconds = %d, want 30 (default)", config2.TimeoutSeconds)
		}
	})
}

// TestFunctionHandler_type tests FunctionHandler type
func TestFunctionHandler_type(t *testing.T) {
	t.Run("handler can be created", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("response"), nil
		})

		if handler == nil {
			t.Error("Handler should not be nil")
		}
	})

	t.Run("handler can be called", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("response"), nil
		})

		ctx := context.Background()
		response, err := handler(ctx, []byte("request"))

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if string(response) != "response" {
			t.Errorf("Response = %s, want 'response'", string(response))
		}
	})

	t.Run("handler can return error", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return nil, context.Canceled
		})

		ctx := context.Background()
		_, err := handler(ctx, []byte("request"))

		if err == nil {
			t.Error("Handler should return error")
		}

		if err != context.Canceled {
			t.Errorf("Error = %v, want context.Canceled", err)
		}
	})

	t.Run("handler can handle empty payload", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("handled empty"), nil
		})

		ctx := context.Background()
		response, err := handler(ctx, []byte{})

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if string(response) != "handled empty" {
			t.Errorf("Response = %s, want 'handled empty'", string(response))
		}
	})

	t.Run("handler can handle nil payload", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return []byte("handled nil"), nil
		})

		ctx := context.Background()
		response, err := handler(ctx, nil)

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if string(response) != "handled nil" {
			t.Errorf("Response = %s, want 'handled nil'", string(response))
		}
	})

	t.Run("handler can return nil response", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			return nil, nil
		})

		ctx := context.Background()
		response, err := handler(ctx, []byte("request"))

		if err != nil {
			t.Errorf("Handler returned error: %v", err)
		}

		if response != nil {
			t.Errorf("Response = %v, want nil", response)
		}
	})

	t.Run("handler with context cancellation", func(t *testing.T) {
		handler := FunctionHandler(func(ctx context.Context, payload []byte) ([]byte, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return []byte("completed"), nil
			}
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := handler(ctx, []byte("request"))

		if err != context.Canceled {
			t.Errorf("Error = %v, want context.Canceled", err)
		}
	})
}
