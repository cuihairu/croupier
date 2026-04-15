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
		jobID   string
		agentID string
	}{
		{"job-001", "agent-1"},
		{"job-002", "agent-2"},
		{"job-003", "agent-3"},
	}

	for _, jr := range jobRoutes {
		// Register job routing
		dispatcher.RegisterJob(jr.jobID, jr.agentID)
		fmt.Printf("   Registered job %s -> %s\n", jr.jobID, jr.agentID)
	}

	// List all job routes
	fmt.Println("\n2. Listing all job routes...")
	routings, err := jobStore.List()
	if err != nil {
		log.Fatalf("Failed to list job routes: %v", err)
	}

	for _, routing := range routings {
		fmt.Printf("   Job: %s, Agent: %s, Created: %s\n",
			routing.JobID,
			routing.AgentID,
			routing.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	// Test job route lookup
	fmt.Println("\n3. Testing job route lookup...")
	for _, jr := range jobRoutes {
		if agentID, exists := dispatcher.JobAddr(jr.jobID); exists {
			fmt.Printf("   Found job %s -> %s\n", jr.jobID, agentID)
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
		fmt.Printf("   Job: %s, Agent: %s\n", routing.JobID, routing.AgentID)
	}

	// Test cleanup of old jobs
	fmt.Println("\n6. Testing cleanup of old jobs...")
	time.Sleep(10 * time.Millisecond) // Ensure jobs are considered "old"
	if err := dispatcher.CleanupOldJobs(1 * time.Millisecond); err != nil {
		log.Fatalf("Failed to cleanup old jobs: %v", err)
	}

	routings, _ = jobStore.List()
	fmt.Printf("   Jobs after cleanup: %d\n", len(routings))

	fmt.Println("\n=== Demo Complete ===")
}
