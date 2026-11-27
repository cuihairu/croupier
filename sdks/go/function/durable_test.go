package function

// GAME-010 slice 5：x-durable-idempotency 能力声明（GAME-009 D3）的 SDK 侧
// 单测——OpenAPI 导入提取 + builder 声明 + 描述符透传三段。

import (
	"context"
	"testing"
)

// openapi 导入路径：x-durable-idempotency 对象提取进 metadata 三字段。
func TestImportExtractsDurableIdempotency(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.0.3",
		"info": {"title": "Test API", "version": "1.0.0"},
		"paths": {
			"/players/ban": {
				"post": {
					"operationId": "player.ban",
					"summary": "Ban Player",
					"x-risk": "high",
					"x-durable-idempotency": {
						"query": "player.ban.query",
						"cancel": "player.ban.cancel",
						"undo": "player.ban.undo"
					},
					"responses": {"200": {"description": "Success"}}
				}
			}
		}
	}`)
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	if err := registry.RegisterFromOpenAPI(spec, nil, func(operationID string) Handler {
		return func(ctx context.Context, input []byte) ([]byte, error) { return nil, nil }
	}); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	metadata, ok := registry.GetMetadata("player.ban")
	if !ok {
		t.Fatal("player.ban not imported")
	}
	if metadata.DurableQuery != "player.ban.query" || metadata.DurableCancel != "player.ban.cancel" || metadata.DurableUndo != "player.ban.undo" {
		t.Fatalf("durable capability not extracted: %+v", metadata)
	}
}

// 未声明扩展 → 三字段全空（= 未声明，不发明）。
func TestImportWithoutDurableIdempotency(t *testing.T) {
	spec := []byte(`{
		"openapi": "3.0.3",
		"info": {"title": "T", "version": "1"},
		"paths": {"/a": {"post": {"operationId": "a.run", "responses": {"200": {"description": "ok"}}}}}
	}`)
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	if err := registry.RegisterFromOpenAPI(spec, nil, func(operationID string) Handler {
		return func(ctx context.Context, input []byte) ([]byte, error) { return nil, nil }
	}); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	metadata, ok := registry.GetMetadata("a.run")
	if !ok {
		t.Fatal("a.run not imported")
	}
	if metadata.DurableQuery != "" || metadata.DurableCancel != "" || metadata.DurableUndo != "" {
		t.Fatalf("durable capability should be empty: %+v", metadata)
	}
}

// builder 声明 → toFunctionDescriptor 透传三字段（进注册线上）。
func TestBuilderDurableIdempotencyPropagates(t *testing.T) {
	registry := NewRegistryWithLogger(newMockClient(), &NoOpLogger{})
	builder := NewMetadataBuilder().SetID("player.ban").SetVersion("1.0.0")
	builder.SetDurableIdempotency("player.ban.query", "player.ban.cancel", "")
	metadata, err := builder.Build()
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if err := registry.Register(metadata, func(ctx context.Context, input []byte) ([]byte, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	metadata, ok := registry.GetMetadata("player.ban")
	if !ok {
		t.Fatal("player.ban not registered")
	}
	desc := registry.toFunctionDescriptor(metadata)
	if desc.DurableQuery != "player.ban.query" || desc.DurableCancel != "player.ban.cancel" || desc.DurableUndo != "" {
		t.Fatalf("descriptor durable fields not propagated: %+v", desc)
	}
}
