package function

import (
	"context"
	"strings"
	"testing"
)

// 路径参数未声明 → LoadFromData 成功但 doc.Validate 失败。
func TestRegisterFromOpenAPI_ValidationFailure(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.0.3",
		"info": {"title": "Invalid API", "version": "1.0.0"},
		"paths": {
			"/items/{itemId}": {
				"get": {
					"operationId": "getItem",
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`)

	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	err := registry.RegisterFromOpenAPI(spec, nil, func(operationID string) Handler {
		return func(ctx context.Context, input []byte) ([]byte, error) { return input, nil }
	})
	if err == nil {
		t.Fatal("expected validation failure for undeclared path parameter")
	}
	if !strings.Contains(err.Error(), "validate OpenAPI spec failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultLoggerMethods(t *testing.T) {
	l := &DefaultLogger{}
	l.Debug("debug %d", 1)
	l.Info("info %s", "x")
	l.Warn("warn %v", true)
	l.Error("error")
}
