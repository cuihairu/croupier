package agentlocal

import (
	"strings"
	"testing"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// 展示层字段空值不覆盖（2026-09-11 线上 tags 被抹根因）：多语言 SDK
// 共享注册同一 function_id 时，无展示元数据的注册者不得抹掉已有
// tags/summary；同一 provider 重新注册仍全量生效。
func TestLocalStore_DisplayFieldsNotErasedByBareRegistration(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("go-demo", "service-1", "127.0.0.1:1", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "player.get", Version: "1.0.0", Tags: []string{"player", "get"}, Summary: "player get", Description: "fetch one player"},
	}, nil)

	// 另一 provider（无展示元数据）重注册同一函数：不得抹掉 tags/summary
	store.Register("node-demo", "service-1", "127.0.0.1:2", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "player.get", Version: "1.0.0"},
	}, nil)

	meta := store.FunctionMetadata()["player.get"]
	if meta == nil {
		t.Fatal("funcMeta missing for player.get")
	}
	if len(meta.Tags) != 2 || meta.Tags[0] != "player" || meta.Tags[1] != "get" {
		t.Fatalf("tags erased by bare re-registration: %+v", meta.Tags)
	}
	if meta.Summary != "player get" {
		t.Fatalf("summary erased by bare re-registration: %q", meta.Summary)
	}
	if meta.Description != "fetch one player" {
		t.Fatalf("description erased by bare re-registration: %q", meta.Description)
	}
	// 合并后的展示字段必须进入 OpenAPI operation JSON
	if !strings.Contains(meta.OpenAPIOperation, `"player get"`) {
		t.Fatalf("merged summary missing from OpenAPI operation: %s", meta.OpenAPIOperation)
	}
}

func TestLocalStore_SameProviderReRegistrationStillWins(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("go-demo", "service-1", "127.0.0.1:1", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "player.get", Version: "1.0.0", Tags: []string{"player", "get"}, Summary: "player get"},
	}, nil)

	// 同一 provider 重新注册可自行清空自己的展示字段（最后注册者胜）
	store.Register("go-demo", "service-1", "127.0.0.1:1", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "player.get", Version: "1.1.0"},
	}, nil)

	meta := store.FunctionMetadata()["player.get"]
	if meta == nil {
		t.Fatal("funcMeta missing for player.get")
	}
	if len(meta.Tags) != 0 {
		t.Fatalf("same-provider re-registration should clear tags, got %+v", meta.Tags)
	}
	if meta.Summary != "" {
		t.Fatalf("same-provider re-registration should clear summary, got %q", meta.Summary)
	}
	if meta.Version != "1.1.0" {
		t.Fatalf("version should follow last registration, got %q", meta.Version)
	}
}

func TestLocalStore_RicherRegistrationOverridesAcrossProviders(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("node-demo", "service-1", "127.0.0.1:2", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "mail.send", Version: "1.0.0"},
	}, nil)
	// 后来者带完整展示元数据：正常覆盖（合并不阻塞升级）
	store.Register("go-demo", "service-1", "127.0.0.1:1", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "mail.send", Version: "1.0.0", Tags: []string{"mail", "send"}, Summary: "mail send"},
	}, nil)

	meta := store.FunctionMetadata()["mail.send"]
	if meta == nil {
		t.Fatal("funcMeta missing for mail.send")
	}
	if len(meta.Tags) != 2 {
		t.Fatalf("expected rich registration to override tags, got %+v", meta.Tags)
	}
	if meta.Summary != "mail send" {
		t.Fatalf("expected rich registration to override summary, got %q", meta.Summary)
	}
}
