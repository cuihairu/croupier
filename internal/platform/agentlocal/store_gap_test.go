package agentlocal

import (
	"testing"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// TestLocalStore_RegisterCopiesMetadata：Register 收到非空 metadata 时必须
// 逐项拷贝进 Instance.Metadata（防御外部 map 后续被修改）。
func TestLocalStore_RegisterCopiesMetadata(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	meta := map[string]string{"sdk_language": "go", "sdk_version": "1.2.3"}
	store.Register("provider-meta", "service-1", "127.0.0.1:19091", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "fn-meta", Version: "1.0.0"},
	}, meta)

	list := store.List()
	instances := list["fn-meta"]
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	got := instances[0].Metadata
	if got["sdk_language"] != "go" || got["sdk_version"] != "1.2.3" {
		t.Fatalf("unexpected metadata copy: %v", got)
	}

	// 拷贝语义：外部修改原 map 不得影响 store 内部状态。
	meta["sdk_language"] = "mutated"
	if instances[0].Metadata["sdk_language"] != "go" {
		t.Fatal("metadata must be copied, not aliased")
	}
}

// TestLocalStore_FunctionMetadataSkipsNilEntries：funcMeta 中出现 nil 条目
// （防御式守卫）时快照必须跳过而非解引用崩溃。
func TestLocalStore_FunctionMetadataSkipsNilEntries(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("provider-live", "service-1", "127.0.0.1:19091", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "fn-live", Version: "1.0.0", Summary: "live"},
	}, nil)

	// 直接注入 nil 元数据条目（同包可访问内部索引）。
	store.mu.Lock()
	store.funcMeta["fn-ghost"] = nil
	store.mu.Unlock()

	out := store.FunctionMetadata()
	if _, exists := out["fn-ghost"]; exists {
		t.Fatal("nil funcMeta entry must be skipped")
	}
	if len(out) != 1 {
		t.Fatalf("expected only the live entry, got %d: %v", len(out), out)
	}
	if out["fn-live"].Summary != "live" {
		t.Fatalf("live entry missing: %+v", out["fn-live"])
	}
}
