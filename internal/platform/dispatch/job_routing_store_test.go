package dispatch

import (
	"testing"
	"time"
)

func TestFileJobRoutingStore_PersistsAcrossInstances(t *testing.T) {
	dataDir := t.TempDir()

	store1, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore: %v", err)
	}

	if err := store1.Set("job-1", "127.0.0.1:9001"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	store2, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore (second): %v", err)
	}

	routing, err := store2.Get("job-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got, want := routing.AgentAddr, "127.0.0.1:9001"; got != want {
		t.Fatalf("AgentAddr=%q want %q", got, want)
	}
}

func TestFileJobRoutingStore_Cleanup(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore: %v", err)
	}

	if err := store.Set("job-old", "127.0.0.1:9999"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := store.Cleanup(1 * time.Millisecond); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	routings, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := len(routings); got != 0 {
		t.Fatalf("len(List)=%d want 0", got)
	}
}

func TestDispatcher_LoadsJobRoutingFromStore(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore: %v", err)
	}

	if err := store.Set("job-2", "127.0.0.1:9002"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	d := NewDispatcherWithJobStore(nil, store)
	if got, ok := d.JobAddr("job-2"); !ok || got != "127.0.0.1:9002" {
		t.Fatalf("JobAddr(job-2)=(%q,%v) want (%q,true)", got, ok, "127.0.0.1:9002")
	}
}

func TestDispatcher_CleanupOldJobsClearsMemoryCache(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewFileJobRoutingStore(dataDir)
	if err != nil {
		t.Fatalf("NewFileJobRoutingStore: %v", err)
	}

	d := NewDispatcherWithJobStore(nil, store)

	d.RegisterJob("job-old", "127.0.0.1:9009")
	time.Sleep(10 * time.Millisecond)

	if err := d.CleanupOldJobs(1 * time.Millisecond); err != nil {
		t.Fatalf("CleanupOldJobs: %v", err)
	}

	if addr, ok := d.JobAddr("job-old"); ok {
		t.Fatalf("JobAddr(job-old)=(%q,%v) want (_,false)", addr, ok)
	}
}
