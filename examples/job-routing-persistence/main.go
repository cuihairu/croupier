package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/registry"
)

func main() {
	fmt.Println("=== Job Routing Persistence Demo ===")

	// Create registry store
	registryStore := registry.NewStore()

	// Create dispatcher with file-based job routing store
	jobStore, err := dispatch.NewFileJobRoutingStore("data")
	if err != nil {
		log.Fatalf("Failed to create job routing store: %v", err)
	}
	defer jobStore.Close()

	// Create dispatcher with persistent job store
	dispatcher := dispatch.NewDispatcherWithJobStore(registryStore, jobStore)
	defer dispatcher.Close()

	// Simulate job routing
	fmt.Println("\n1. Registering job routes...")
	jobRoutes := []struct {
		jobID string
		addr  string
	}{
		{"job-001", "127.0.0.1:9001"},
		{"job-002", "127.0.0.1:9002"},
		{"job-003", "127.0.0.1:9003"},
	}

	for _, jr := range jobRoutes {
		// Register job routing
		dispatcher.RegisterJob(jr.jobID, jr.addr)
		fmt.Printf("   Registered job %s -> %s\n", jr.jobID, jr.addr)
	}

	// List all job rout
	fmt.Println("\n2. Listing all job routes...")
	routings, err := jobStore.List()
	if err != nil {
		log.Fatalf("Failed to list job routes: %v", err)
	}

	for _, routing := range routings {
		fmt.Printf("   Job: %s, Agent: %s, Created: %s\n",
			routing.JobID,
			routing.AgentAddr,
			routing.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	// Test job route lookup
	fmt.Println("\n3. Testing job route lookup...")
	for _, jr := range jobRoutes {
		if addr, exists := dispatcher.JobAddr(jr.jobID); exists {
			fmt.Printf("   Found job %s -> %s\n", jr.jobID, addr)
		} else {
			fmt.Printf("   Job %s not found\n", jr.jobID)
		}
	}

	// Simulate job completion
	fmt.Println("\n4. Simulating job completion...")
	completedJobs := []string{"job-001", "job-003"}
	for _, jobID := range completedJobs {
		dispatcher.UnregisterJob(jobID)
		fmt.Printf("   Unregistered job %s\n", jobID)
	}

	// Check remaining jobs
	fmt.Println("\n5. Checking remaining jobs...")
	routings, err = jobStore.List()
	if err != nil {
		log.Fatalf("Failed to list job routes: %v", err)
	}

	fmt.Printf("   Remaining jobs: %d\n", len(routings))
	for _, routing := range routings {
		fmt.Printf("   - Job: %s\n", routing.JobID)
	}

	// Test persistence by creating new dispatcher
	fmt.Println("\n6. Testing persistence...")
	newDispatcher := dispatch.NewDispatcherWithJobStore(registryStore, jobStore)
	defer newDispatcher.Close()

	if addr, exists := newDispatcher.JobAddr("job-002"); exists {
		fmt.Printf("   Successfully loaded job-002 from persistent store: %s\n", addr)
	} else {
		fmt.Println("   Failed to load job-002 from persistent store")
	}

	// Test cleanup of old jobs
	fmt.Println("\n7. Testing cleanup of old jobs...")
	// Add an old job entry
	dispatcher.RegisterJob("old-job", "127.0.0.1:9999")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Cleanup jobs older than 50ms
	if err := dispatcher.CleanupOldJobs(50 * time.Millisecond); err != nil {
		log.Printf("Warning: cleanup failed: %v", err)
	}

	// Check remaining jobs
	routings, _ = jobStore.List()
	fmt.Printf("   Jobs after cleanup: %d\n", len(routings))
	for _, routing := range routings {
		fmt.Printf("   - Job: %s\n", routing.JobID)
	}

	fmt.Println("\n=== Demo Complete ===")
}
