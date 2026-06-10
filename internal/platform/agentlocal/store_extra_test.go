package agentlocal

import (
	"testing"
	"time"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

func TestLocalStore_OnUpdate(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	called := make(chan struct{}, 1)
	store.OnUpdate(func() {
		called <- struct{}{}
	})

	store.Register("svc-1", "127.0.0.1:19090", "v1", []*sdkv1.LocalFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
	})

	select {
	case <-called:
		// callback was invoked
	case <-time.After(time.Second):
		t.Error("OnUpdate callback was not invoked")
	}
}

func TestLocalStore_Heartbeat(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("svc-1", "127.0.0.1:19090", "v1", []*sdkv1.LocalFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
	})

	// Record initial last seen
	list1 := store.List()
	initialSeen := list1["f1"][0].LastSeen

	time.Sleep(10 * time.Millisecond)
	store.Heartbeat("svc-1")

	list2 := store.List()
	if !list2["f1"][0].LastSeen.After(initialSeen) {
		t.Error("Heartbeat should update LastSeen")
	}
}

func TestLocalStore_Heartbeat_UnknownProvider(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("svc-1", "127.0.0.1:19090", "v1", []*sdkv1.LocalFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
	})

	// Should not panic for unknown provider
	store.Heartbeat("unknown-svc")
}

func TestLocalStore_SetTaskResult(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()

	// Set new task result
	store.SetTaskResult("task-1", "done", []byte(`{"result":"ok"}`), "")

	result, exists := store.GetTaskResult("task-1")
	if !exists {
		t.Fatal("task result should exist")
	}
	if result.State != "done" {
		t.Errorf("State = %q, want %q", result.State, "done")
	}
	if string(result.Payload) != `{"result":"ok"}` {
		t.Errorf("Payload = %q", string(result.Payload))
	}
}

func TestLocalStore_SetTaskResult_Update(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()

	store.SetTaskResult("task-1", "running", nil, "")
	store.SetTaskResult("task-1", "done", []byte("data"), "")

	result, exists := store.GetTaskResult("task-1")
	if !exists {
		t.Fatal("task result should exist")
	}
	if result.State != "done" {
		t.Errorf("State = %q, want %q", result.State, "done")
	}
}

func TestLocalStore_GetTaskResult_NotFound(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()

	_, exists := store.GetTaskResult("nonexistent")
	if exists {
		t.Error("should not exist")
	}
}

func TestLocalStore_RemoveTaskResult(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.SetTaskResult("task-1", "done", nil, "")

	store.RemoveTaskResult("task-1")

	_, exists := store.GetTaskResult("task-1")
	if exists {
		t.Error("should be removed")
	}
}

func TestLocalStore_RemoveTaskResult_NotFound(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	// Should not panic
	store.RemoveTaskResult("nonexistent")
}

func TestLocalStore_CleanupOldTaskResults(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.SetTaskResult("task-old", "done", nil, "")
	store.SetTaskResult("task-new", "done", nil, "")

	// Manually set old timestamp
	store.mu.Lock()
	store.taskResults["task-old"].UpdatedAt = time.Now().Add(-time.Hour)
	store.mu.Unlock()

	removed := store.CleanupOldTaskResults(30 * time.Minute)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	_, oldExists := store.GetTaskResult("task-old")
	if oldExists {
		t.Error("old task should be cleaned up")
	}

	_, newExists := store.GetTaskResult("task-new")
	if !newExists {
		t.Error("new task should still exist")
	}
}

func TestLocalStore_CleanupOldTaskResults_None(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.SetTaskResult("task-1", "done", nil, "")

	removed := store.CleanupOldTaskResults(time.Hour)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}

func TestLocalStore_Register_NilFunction(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("svc-1", "127.0.0.1:19090", "v1", []*sdkv1.LocalFunctionDescriptor{
		nil,
		{Id: "", Version: "1.0.0"},
		{Id: "f1", Version: "1.0.0"},
	})

	list := store.List()
	if _, ok := list["f1"]; !ok {
		t.Error("f1 should be registered")
	}
}

func TestLocalStore_Register_ReplaceProvider(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	store.Register("svc-1", "127.0.0.1:19090", "v1", []*sdkv1.LocalFunctionDescriptor{
		{Id: "f1", Version: "1.0.0"},
	})

	store.Register("svc-1", "127.0.0.1:19090", "v2", []*sdkv1.LocalFunctionDescriptor{
		{Id: "f1", Version: "2.0.0"},
		{Id: "f2", Version: "1.0.0"},
	})

	list := store.List()
	if len(list["f1"]) != 1 {
		t.Errorf("f1 should have 1 instance, got %d", len(list["f1"]))
	}
	if _, ok := list["f2"]; !ok {
		t.Error("f2 should be registered")
	}
}

func TestLocalStore_FunctionVersions_Empty(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	versions := store.FunctionVersions()
	if len(versions) != 0 {
		t.Errorf("expected empty, got %d", len(versions))
	}
}

func TestLocalStore_FunctionMetadata_Empty(t *testing.T) {
	t.Parallel()

	store := NewLocalStore()
	meta := store.FunctionMetadata()
	if len(meta) != 0 {
		t.Errorf("expected empty, got %d", len(meta))
	}
}
