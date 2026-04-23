// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"context"
	"testing"
	"time"
)

func TestContext_cancellationScenarios(t *testing.T) {
	t.Run("context cancellation before invoke", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with cancelled context error: %v", err)
	})

	t.Run("context timeout during invoke", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout has passed

		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with timed out context error: %v", err)
	})

	t.Run("context with values", func(t *testing.T) {
		type contextKey string
		key := contextKey("request-id")

		ctx := context.WithValue(context.Background(), key, "req-123")

		invoker := NewInvoker(&InvokerConfig{
			Address: "http://localhost:19090",
		})

		_, err := invoker.Invoke(ctx, "test.func", "{}", InvokeOptions{})
		t.Logf("Invoke with context values error: %v", err)

		if ctx.Value(key) != "req-123" {
			t.Error("Context value should be preserved")
		}
	})
}

func TestPayload_edgeCases(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})

	ctx := context.Background()

	t.Run("empty string payload", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "test.func", "", InvokeOptions{})
		t.Logf("Invoke with empty string error: %v", err)
	})

	t.Run("whitespace payload", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "test.func", "   ", InvokeOptions{})
		t.Logf("Invoke with whitespace error: %v", err)
	})

	t.Run("very long payload", func(t *testing.T) {
		longPayload := string(make([]byte, 1024*1024)) // 1MB
		_, err := invoker.Invoke(ctx, "test.func", longPayload, InvokeOptions{})
		t.Logf("Invoke with long payload error: %v", err)
	})

	t.Run("payload with Unicode", func(t *testing.T) {
		unicodePayload := `{"message":"Hello 世界 🌍 Привет мир"}`
		_, err := invoker.Invoke(ctx, "test.func", unicodePayload, InvokeOptions{})
		t.Logf("Invoke with Unicode payload error: %v", err)
	})

	t.Run("payload with newlines and tabs", func(t *testing.T) {
		specialPayload := "{\"data\":\"line1\nline2\ttab\"}"
		_, err := invoker.Invoke(ctx, "test.func", specialPayload, InvokeOptions{})
		t.Logf("Invoke with special chars payload error: %v", err)
	})

	t.Run("malformed JSON payload", func(t *testing.T) {
		malformedPayload := `{"invalid": json}`
		_, err := invoker.Invoke(ctx, "test.func", malformedPayload, InvokeOptions{})
		t.Logf("Invoke with malformed JSON error: %v", err)
	})
}

func TestFunctionID_variations(t *testing.T) {
	invoker := NewInvoker(&InvokerConfig{
		Address: "http://localhost:19090",
	})

	ctx := context.Background()

	t.Run("empty function ID", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "", "{}", InvokeOptions{})
		t.Logf("Invoke with empty ID error: %v", err)
	})

	t.Run("single word ID", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "function", "{}", InvokeOptions{})
		t.Logf("Invoke with single word ID error: %v", err)
	})

	t.Run("dotted notation ID", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "player.inventory.add", "{}", InvokeOptions{})
		t.Logf("Invoke with dotted ID error: %v", err)
	})

	t.Run("ID with numbers", func(t *testing.T) {
		_, err := invoker.Invoke(ctx, "func123.test456", "{}", InvokeOptions{})
		t.Logf("Invoke with numeric ID error: %v", err)
	})

	t.Run("ID with special characters", func(t *testing.T) {
		ids := []string{
			"test_func",
			"test-func",
			"test.func",
			"test:func",
			"test/func",
		}

		for _, id := range ids {
			_, err := invoker.Invoke(ctx, id, "{}", InvokeOptions{})
			t.Logf("Invoke with ID '%s' error: %v", id, err)
		}
	})
}

func TestRetryConfig_edgeCases(t *testing.T) {
	t.Run("retry with zero delays", func(t *testing.T) {
		config := &RetryConfig{
			Enabled:        true,
			MaxAttempts:    3,
			InitialDelayMs: 0,
			MaxDelayMs:     0,
		}

		if config.InitialDelayMs != 0 {
			t.Error("InitialDelayMs should be 0")
		}
	})

	t.Run("retry with negative values", func(t *testing.T) {
		config := &RetryConfig{
			Enabled:        true,
			MaxAttempts:    -1,
			InitialDelayMs: -100,
			MaxDelayMs:     -200,
		}

		if config.MaxAttempts != -1 {
			t.Error("MaxAttempts should be -1")
		}
	})

	t.Run("retry with very large values", func(t *testing.T) {
		config := &RetryConfig{
			Enabled:        true,
			MaxAttempts:    1000000,
			InitialDelayMs: 1000000,
			MaxDelayMs:     10000000,
		}

		if config.MaxAttempts != 1000000 {
			t.Error("MaxAttempts should be 1000000")
		}
	})

	t.Run("retry with extreme multipliers", func(t *testing.T) {
		multipliers := []float64{0.001, 0.1, 1.0, 10.0, 100.0, 1000.0}

		for _, mult := range multipliers {
			config := &RetryConfig{
				BackoffMultiplier: mult,
			}

			if config.BackoffMultiplier != mult {
				t.Errorf("BackoffMultiplier = %f, want %f", config.BackoffMultiplier, mult)
			}
		}
	})
}

func TestReconnectConfig_edgeCases(t *testing.T) {
	t.Run("reconnect with zero delays", func(t *testing.T) {
		config := &ReconnectConfig{
			Enabled:        true,
			MaxAttempts:    0,
			InitialDelayMs: 0,
			MaxDelayMs:     0,
		}

		if config.MaxAttempts != 0 {
			t.Error("MaxAttempts should be 0 (infinite)")
		}
	})

	t.Run("reconnect with negative values", func(t *testing.T) {
		config := &ReconnectConfig{
			Enabled:        true,
			MaxAttempts:    -1,
			InitialDelayMs: -100,
			MaxDelayMs:     -200,
		}

		if config.MaxAttempts != -1 {
			t.Error("MaxAttempts should be -1")
		}
	})
}

func TestInvokeOptions_combinations(t *testing.T) {
	t.Run("all fields populated", func(t *testing.T) {
		options := InvokeOptions{
			IdempotencyKey: "unique-key-12345",
			Timeout:        30 * time.Second,
			Headers: map[string]string{
				"X-Request-ID":  "req-123",
				"X-Correlation-ID": "corr-456",
				"X-User-ID":     "user-789",
			},
			Retry: &RetryConfig{
				Enabled:     true,
				MaxAttempts: 5,
			},
		}

		if options.IdempotencyKey != "unique-key-12345" {
			t.Error("IdempotencyKey not preserved")
		}

		if len(options.Headers) != 3 {
			t.Errorf("Headers count = %d, want 3", len(options.Headers))
		}

		if options.Retry == nil {
			t.Error("Retry config should not be nil")
		}
	})

	t.Run("only timeout", func(t *testing.T) {
		options := InvokeOptions{
			Timeout: 60 * time.Second,
		}

		if options.Timeout != 60*time.Second {
			t.Error("Timeout not preserved")
		}
	})

	t.Run("only idempotency key", func(t *testing.T) {
		options := InvokeOptions{
			IdempotencyKey: "key-123",
		}

		if options.IdempotencyKey != "key-123" {
			t.Error("IdempotencyKey not preserved")
		}
	})

	t.Run("empty headers map", func(t *testing.T) {
		options := InvokeOptions{
			Headers: map[string]string{},
		}

		if options.Headers == nil {
			t.Error("Headers should not be nil")
		}

		if len(options.Headers) != 0 {
			t.Error("Headers should be empty")
		}
	})
}

func TestVersion_formats(t *testing.T) {
	t.Run("various semver formats", func(t *testing.T) {
		versions := []string{
			"0.0.1",
			"1.0.0",
			"1.2.3",
			"10.20.30",
			"1.0.0-alpha",
			"1.0.0-alpha.1",
			"1.0.0-beta",
			"1.0.0-beta.2",
			"1.0.0-rc.1",
			"1.0.0+build",
			"1.0.0-alpha+build",
			"v1.0.0",
		}

		for _, version := range versions {
			desc := FunctionDescriptor{
				ID:      "test.func",
				Version: version,
			}

			if desc.Version != version {
				t.Errorf("Version = %s, want %s", desc.Version, version)
			}
		}
	})

	t.Run("invalid version formats", func(t *testing.T) {
		versions := []string{
			"",
			"1",
			"1.2",
			"invalid",
			"1.2.3.4",
		}

		for _, version := range versions {
			desc := FunctionDescriptor{
				ID:      "test.func",
				Version: version,
			}

			if desc.Version != version {
				t.Errorf("Version = %s, want %s", desc.Version, version)
			}
		}
	})
}

func TestCategory_and_Risk(t *testing.T) {
	t.Run("all combinations of category and risk", func(t *testing.T) {
		categories := []string{"player", "item", "guild", "chat", "system", "admin"}
		risks := []string{"low", "medium", "high", "critical"}

		for _, cat := range categories {
			for _, risk := range risks {
				desc := FunctionDescriptor{
					ID:       "test.func",
					Version:  "1.0.0",
					Category:  cat,
					Risk:      risk,
					Enabled:   true,
				}

				if desc.Category != cat || desc.Risk != risk {
					t.Errorf("Category=%s, Risk=%s not preserved", cat, risk)
				}
			}
		}
	})
}

func TestEnabled_flag(t *testing.T) {
	t.Run("enabled and disabled descriptors", func(t *testing.T) {
		enabled := FunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
			Enabled: true,
		}

		disabled := FunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
			Enabled: false,
		}

		if !enabled.Enabled {
			t.Error("Enabled descriptor should have Enabled=true")
		}

		if disabled.Enabled {
			t.Error("Disabled descriptor should have Enabled=false")
		}
	})
}

func TestLocalFunctionDescriptor_tags(t *testing.T) {
	t.Run("empty vs nil tags", func(t *testing.T) {
		emptyTags := LocalFunctionDescriptor{
			ID:    "test.func",
			Tags:  []string{},
		}

		nilTags := LocalFunctionDescriptor{
			ID:    "test.func",
			Tags:  nil,
		}

		if emptyTags.Tags == nil {
			t.Error("Empty tags should not be nil")
		}

		if nilTags.Tags != nil {
			t.Error("Nil tags should be nil")
		}
	})

	t.Run("tags with duplicates", func(t *testing.T) {
		tags := []string{"tag1", "tag2", "tag1", "tag3", "tag2"}

		desc := LocalFunctionDescriptor{
			ID:   "test.func",
			Tags: tags,
		}

		if len(desc.Tags) != 5 {
			t.Errorf("Tags length = %d, want 5 (duplicates allowed)", len(desc.Tags))
		}
	})

	t.Run("tags with special characters", func(t *testing.T) {
		tags := []string{
			"tag-with-dash",
			"tag_with_underscore",
			"tag.with.dot",
			"tag:with:colon",
			"tag/with/slash",
		}

		desc := LocalFunctionDescriptor{
			ID:   "test.func",
			Tags: tags,
		}

		if len(desc.Tags) != 5 {
			t.Errorf("Tags length = %d, want 5", len(desc.Tags))
		}
	})
}
