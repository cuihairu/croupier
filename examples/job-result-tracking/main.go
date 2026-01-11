package main

import (
	"context"
	"fmt"
	"log"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	localv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/local/v1"
)

func main() {
	fmt.Println("=== Job Result Tracking Demo ===")

	// Create local store and server
	store := agentlocal.NewLocalStore()
	server := agentlocal.NewServer(store)

	// Simulate job execution
	fmt.Println("\n1. Simulating job execution...")
	jobs := []struct {
		jobID    string
		state    string
		payload  []byte
		error    string
		duration time.Duration
	}{
		{"job-001", "pending", nil, "", 0},
		{"job-002", "pending", nil, "", 0},
		{"job-003", "pending", nil, "", 0},
	}

	// Start jobs
	for _, job := range jobs {
		store.SetJobResult(job.jobID, job.state, job.payload, job.error)
		fmt.Printf("   Started job %s (state: %s)\n", job.jobID, job.state)
	}

	// Simulate job progress
	fmt.Println("\n2. Simulating job progress...")
	time.Sleep(1 * time.Second)

	// Update job states
	updates := []struct {
		jobID   string
		state   string
		payload []byte
		error   string
	}{
		{"job-001", "running", []byte(`{"progress": 25}`), ""},
		{"job-002", "running", []byte(`{"progress": 50}`), ""},
		{"job-003", "failed", nil, "Connection timeout"},
	}

	for _, update := range updates {
		store.SetJobResult(update.jobID, update.state, update.payload, update.error)
		fmt.Printf("   Updated job %s (state: %s)\n", update.jobID, update.state)
	}

	// Query job results
	fmt.Println("\n3. Querying job results...")
	ctx := context.Background()

	for _, job := range jobs {
		req := &localv1.GetJobResultRequest{JobId: job.jobID}
		resp, err := server.GetJobResult(ctx, req)
		if err != nil {
			log.Printf("Error getting job %s: %v", job.jobID, err)
			continue
		}

		fmt.Printf("   Job %s:\n", job.jobID)
		fmt.Printf("     State: %s\n", resp.State)
		if resp.Payload != nil {
			fmt.Printf("     Payload: %s\n", string(resp.Payload))
		}
		if resp.Error != "" {
			fmt.Printf("     Error: %s\n", resp.Error)
		}
	}

	// Complete remaining jobs
	fmt.Println("\n4. Completing remaining jobs...")
	time.Sleep(1 * time.Second)

	completions := []struct {
		jobID    string
		state    string
		payload  []byte
		errorMsg string
	}{
		{"job-001", "completed", []byte(`{"result": "success", "value": 42}`), ""},
		{"job-002", "completed", []byte(`{"result": "success", "value": 100}`), ""},
	}

	for _, comp := range completions {
		store.SetJobResult(comp.jobID, comp.state, comp.payload, comp.errorMsg)
		fmt.Printf("   Completed job %s\n", comp.jobID)
	}

	// Final status check
	fmt.Println("\n5. Final status check...")
	for _, job := range jobs {
		req := &localv1.GetJobResultRequest{JobId: job.jobID}
		resp, err := server.GetJobResult(ctx, req)
		if err != nil {
			log.Printf("Error getting job %s: %v", job.jobID, err)
			continue
		}

		fmt.Printf("   Job %s: %s", job.jobID, resp.State)
		if resp.Payload != nil {
			fmt.Printf(" - %s", string(resp.Payload))
		}
		fmt.Println()
	}

	// Test cleanup
	fmt.Println("\n6. Testing cleanup of old job results...")
	// Add an old job
	store.SetJobResult("old-job", "completed", []byte(`{"old": true}`), "")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Clean up jobs older than 50ms
	removed := store.CleanupOldJobResults(50 * time.Millisecond)
	fmt.Printf("   Removed %d old job results\n", removed)

	// Check remaining jobs
	fmt.Println("\n7. Remaining jobs:")
	allJobs := store.List()
	fmt.Printf("   Functions registered: %d\n", len(allJobs))

	// Note: We can't directly list job results from store in this example
	// but in real usage, the GetJobResult method would return "not_found" for cleaned jobs

	fmt.Println("\n=== Demo Complete ===")
}
