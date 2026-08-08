package agentlocal

import (
	"testing"
	"time"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

func TestLocalStore_RegisterAndList(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("svc-1", "service-1", "127.0.0.1:19090", "sv1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
		{Id: "f2", Version: "2.0.0"},
	}, nil)

	list := store.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(list))
	}
	if got := list["f1"]; len(got) != 1 || got[0].ProviderID != "svc-1" {
		t.Fatalf("unexpected f1 instances: %+v", got)
	}

	versions := store.FunctionVersions()
	if got := versions["f2"]["svc-1"]; got != "2.0.0" {
		t.Fatalf("expected f2 version 2.0.0, got %q", got)
	}
}

func TestLocalStore_Prune(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("svc-1", "service-1", "127.0.0.1:19090", "sv1", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
	}, nil)

	time.Sleep(10 * time.Millisecond)
	removed := store.Prune(0)
	if removed == 0 {
		t.Fatal("expected prune to remove instances")
	}
	if len(store.List()) != 0 {
		t.Fatalf("expected store empty after prune, got %+v", store.List())
	}
}
