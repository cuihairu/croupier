// 覆盖目标：agentlocal 包 ListByService 双层索引、RemoveProvider/Prune 的
// onUpdate 回调分支、cloneStringMap、Register 跳过 nil 描述符等路径。
package agentlocal

import (
	"testing"
	"time"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

func TestLocalStore_ListByService(t *testing.T) {
	store := NewLocalStore()
	store.Register("p1", "svc-a", "127.0.0.1:1", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
		{Id: "f2", Version: "1.0.0"},
	}, nil)
	store.Register("p2", "svc-b", "127.0.0.1:2", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "f1", Version: "2.0.0"},
	}, nil)

	out := store.ListByService()
	if len(out) != 2 {
		t.Fatalf("expected 2 function keys, got %d", len(out))
	}
	if len(out["f1"]) != 2 {
		t.Fatalf("expected f1 in 2 services, got %d", len(out["f1"]))
	}
	if len(out["f1"]["svc-a"]) != 1 || out["f1"]["svc-a"][0].ProviderID != "p1" {
		t.Fatalf("unexpected svc-a instances: %+v", out["f1"]["svc-a"])
	}
	if len(out["f2"]["svc-a"]) != 1 {
		t.Fatalf("expected f2 only in svc-a, got %+v", out["f2"])
	}
	if _, ok := out["f1"]["svc-b"]; !ok {
		t.Fatal("expected svc-b to host f1")
	}

	// 快照与内部数据隔离：外部修改不影响 store。
	out["f1"]["svc-a"][0].Addr = "mutated"
	again := store.ListByService()
	if again["f1"]["svc-a"][0].Addr == "mutated" {
		t.Fatal("ListByService must return a deep-enough copy")
	}
}

func TestLocalStore_RemoveProvider_TriggersOnUpdate(t *testing.T) {
	store := NewLocalStore()
	called := make(chan struct{}, 1)
	store.OnUpdate(func() { called <- struct{}{} })

	store.Register("p1", "svc", "127.0.0.1:1", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
	}, nil)
	// 消费 register 回调，避免与 remove 回调混淆。
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("register should trigger OnUpdate")
	}

	store.RemoveProvider("p1")
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("RemoveProvider should trigger OnUpdate")
	}

	if list := store.List(); len(list) != 0 {
		t.Fatalf("expected empty store after RemoveProvider, got %+v", list)
	}

	// 版本索引也一并清理。
	if versions := store.FunctionVersions(); len(versions) != 0 {
		t.Fatalf("expected empty versions after RemoveProvider, got %+v", versions)
	}
}

func TestLocalStore_Prune_TriggersOnUpdateWhenRemoved(t *testing.T) {
	store := NewLocalStore()
	called := make(chan struct{}, 4)
	store.OnUpdate(func() { called <- struct{}{} })

	store.Register("p1", "svc", "127.0.0.1:1", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
	}, nil)
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("register should trigger OnUpdate")
	}

	// 无过期实例：Prune 返回 0 且不触发回调。
	if removed := store.Prune(time.Hour); removed != 0 {
		t.Fatalf("expected 0 removed, got %d", removed)
	}
	select {
	case <-called:
		t.Fatal("Prune without removals must not trigger OnUpdate")
	default:
	}

	// 负 maxAge：全部实例判定过期。
	if removed := store.Prune(-time.Second); removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("Prune with removals should trigger OnUpdate")
	}
	if list := store.List(); len(list) != 0 {
		t.Fatalf("expected empty store after prune, got %+v", list)
	}
}

func TestLocalStore_Register_SkipsNilAndEmptyDescriptors(t *testing.T) {
	store := NewLocalStore()
	store.Register("p1", "svc", "127.0.0.1:1", "v1", []*sdkv1.ProviderFunctionDescriptor{
		nil,
		{Id: "", Version: "1.0.0"},
		{Id: "f1", Version: "1.0.0"},
	}, nil)

	list := store.List()
	if _, ok := list["f1"]; !ok {
		t.Fatalf("expected f1 registered, got %+v", list)
	}
	if len(list) != 1 {
		t.Fatalf("expected only f1, got %d functions", len(list))
	}

	// 默认 service 分组。
	store.Register("p2", "", "127.0.0.1:2", "v1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "f9"},
	}, nil)
	out := store.ListByService()
	if _, ok := out["f9"]["__default__"]; !ok {
		t.Fatalf("expected __default__ service, got %+v", out["f9"])
	}
}

func TestCloneStringMap_Branches(t *testing.T) {
	if cloneStringMap(nil) != nil {
		t.Fatal("nil input should return nil")
	}
	if cloneStringMap(map[string]string{}) != nil {
		t.Fatal("empty input should return nil")
	}
	in := map[string]string{"a": "1", "b": "2"}
	out := cloneStringMap(in)
	if len(out) != 2 || out["a"] != "1" || out["b"] != "2" {
		t.Fatalf("unexpected clone: %+v", out)
	}
	out["a"] = "mutated"
	if in["a"] != "1" {
		t.Fatal("clone must not alias input map")
	}
}
