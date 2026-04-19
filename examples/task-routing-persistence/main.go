package main

import (
	"fmt"
	"log"
	"time"

	"github.com/cuihairu/croupier/internal/platform/dispatch"
	"github.com/cuihairu/croupier/internal/platform/registry"
)

func main() {
	fmt.Println("=== Task Routing Persistence Demo ===")

	// Create registry store
	registryStore := registry.NewStore()

	// Create dispatcher with file-based task routing store
	taskStore, err := dispatch.NewFileTaskRoutingStore("data")
	if err != nil {
		log.Fatalf("Failed to create task routing store: %v", err)
	}
	defer taskStore.Close()

	// Create dispatcher with persistent task store
	dispatcher := dispatch.NewDispatcherWithTaskStore(registryStore, taskStore, nil)
	defer dispatcher.Close()

	// Simulate task routing
	fmt.Println("\n1. Registering task routes...")
	taskRoutes := []struct {
		taskID  string
		agentID string
	}{
		{"task-001", "agent-1"},
		{"task-002", "agent-2"},
		{"task-003", "agent-3"},
	}

	for _, tr := range taskRoutes {
		// Register task routing
		dispatcher.RegisterTask(tr.taskID, tr.agentID)
		fmt.Printf("   Registered task %s -> %s\n", tr.taskID, tr.agentID)
	}

	// List all task routes
	fmt.Println("\n2. Listing all task routes...")
	routings, err := taskStore.List()
	if err != nil {
		log.Fatalf("Failed to list task routes: %v", err)
	}

	for _, routing := range routings {
		fmt.Printf("   Task: %s, Agent: %s, Created: %s\n",
			routing.TaskID,
			routing.AgentID,
			routing.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	// Test task route lookup
	fmt.Println("\n3. Testing task route lookup...")
	for _, tr := range taskRoutes {
		if routing, err := taskStore.Get(tr.taskID); err == nil {
			fmt.Printf("   Found task %s -> %s\n", tr.taskID, routing.AgentID)
		} else {
			fmt.Printf("   Task %s not found\n", tr.taskID)
		}
	}

	// Simulate task completion
	fmt.Println("\n4. Simulating task completion...")
	completedTasks := []string{"task-001", "task-003"}
	for _, taskID := range completedTasks {
		dispatcher.UnregisterTask(taskID)
		fmt.Printf("   Unregistered task %s\n", taskID)
	}

	// Check remaining tasks
	fmt.Println("\n5. Checking remaining tasks...")
	routings, err = taskStore.List()
	if err != nil {
		log.Fatalf("Failed to list task routes: %v", err)
	}

	fmt.Printf("   Remaining tasks: %d\n", len(routings))
	for _, routing := range routings {
		fmt.Printf("   Task: %s, Agent: %s\n", routing.TaskID, routing.AgentID)
	}

	// Test cleanup of old tasks
	fmt.Println("\n6. Testing cleanup of old tasks...")
	time.Sleep(10 * time.Millisecond) // Ensure tasks are considered "old"
	if err := dispatcher.CleanupOldTasks(1 * time.Millisecond); err != nil {
		log.Fatalf("Failed to cleanup old tasks: %v", err)
	}

	routings, _ = taskStore.List()
	fmt.Printf("   Tasks after cleanup: %d\n", len(routings))

	fmt.Println("\n=== Demo Complete ===")
}
