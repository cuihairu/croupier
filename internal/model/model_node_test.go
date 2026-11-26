package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewNodeModel(t *testing.T) {
	db := setupNodeTestDB(t)
	model := NewNodeModel(db)
	assert.NotNil(t, model)
}

func TestNodeModel_Upsert(t *testing.T) {
	db := setupNodeTestDB(t)
	model := NewNodeModel(db)
	ctx := context.Background()

	node := &Node{
		NodeID: "node-001",
		Name:   "Test Node",
		Type:   "agent",
		Status: "active",
		IP:     "192.168.1.1",
		Port:   8080,
	}

	err := model.Upsert(ctx, node)
	require.NoError(t, err)
	assert.NotZero(t, node.ID)

	// Update via upsert
	node.Status = "offline"
	err = model.Upsert(ctx, node)
	require.NoError(t, err)
}

func TestNodeModel_List(t *testing.T) {
	db := setupNodeTestDB(t)
	model := NewNodeModel(db)
	ctx := context.Background()

	nodes := []*Node{
		{NodeID: "node-list-1", Name: "Node 1", Type: "agent", Status: "active", IP: "192.168.1.1", Port: 8080},
		{NodeID: "node-list-2", Name: "Node 2", Type: "server", Status: "active", IP: "192.168.1.2", Port: 8081},
		{NodeID: "node-list-3", Name: "Node 3", Type: "agent", Status: "offline", IP: "192.168.1.3", Port: 8082},
	}
	for _, n := range nodes {
		err := model.Upsert(ctx, n)
		require.NoError(t, err)
	}

	// List all
	all, err := model.List(ctx, ListNodesOptions{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 3)

	// Filter by type
	agents, err := model.List(ctx, ListNodesOptions{Type: "agent"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(agents), 2)

	// Filter by status
	active, err := model.List(ctx, ListNodesOptions{Status: "active"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(active), 2)
}

func TestNodeModel_UpdateMeta(t *testing.T) {
	db := setupNodeTestDB(t)
	model := NewNodeModel(db)
	ctx := context.Background()

	node := &Node{
		NodeID: "node-meta-001",
		Name:   "Meta Node",
		Type:   "agent",
		Status: "active",
		IP:     "192.168.1.1",
		Port:   8080,
	}
	err := model.Upsert(ctx, node)
	require.NoError(t, err)

	// Update metadata
	updates := map[string]interface{}{
		"name":   "Updated Node",
		"status": "busy",
	}
	err = model.UpdateMeta(ctx, "node-meta-001", updates)
	require.NoError(t, err)

	// Verify
	found, err := model.FindByNodeID(ctx, "node-meta-001")
	require.NoError(t, err)
	assert.Equal(t, "Updated Node", found.Name)
	assert.Equal(t, "busy", found.Status)
}

func TestNodeModel_FindByNodeID(t *testing.T) {
	db := setupNodeTestDB(t)
	model := NewNodeModel(db)
	ctx := context.Background()

	node := &Node{
		NodeID: "node-find-001",
		Name:   "Find Node",
		Type:   "agent",
		Status: "active",
		IP:     "192.168.1.1",
		Port:   8080,
	}
	err := model.Upsert(ctx, node)
	require.NoError(t, err)

	// Find existing
	found, err := model.FindByNodeID(ctx, "node-find-001")
	require.NoError(t, err)
	assert.Equal(t, "Find Node", found.Name)

	// Find non-existent
	_, err = model.FindByNodeID(ctx, "non-existent")
	assert.Error(t, err)
}

func TestNodeModel_UpdateStatus(t *testing.T) {
	db := setupNodeTestDB(t)
	model := NewNodeModel(db)
	ctx := context.Background()

	node := &Node{
		NodeID: "node-status-001",
		Name:   "Status Node",
		Type:   "agent",
		Status: "active",
		IP:     "192.168.1.1",
		Port:   8080,
	}
	err := model.Upsert(ctx, node)
	require.NoError(t, err)

	// Update status
	err = model.UpdateStatus(ctx, "node-status-001", "offline")
	require.NoError(t, err)

	// Verify
	found, err := model.FindByNodeID(ctx, "node-status-001")
	require.NoError(t, err)
	assert.Equal(t, "offline", found.Status)
}

func TestNodeModel_ListCommands(t *testing.T) {
	db := setupNodeTestDB(t)
	model := NewNodeModel(db)
	ctx := context.Background()

	commands := []*NodeCommand{
		{Name: "cmd1", Description: "Command 1"},
		{Name: "cmd2", Description: "Command 2"},
		{Name: "cmd3", Description: "Command 3"},
	}
	for _, cmd := range commands {
		err := model.UpsertCommand(ctx, cmd)
		require.NoError(t, err)
	}

	// List commands
	list, err := model.ListCommands(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 3)
}

func TestNodeModel_UpsertCommand(t *testing.T) {
	db := setupNodeTestDB(t)
	model := NewNodeModel(db)
	ctx := context.Background()

	cmd := &NodeCommand{
		Name:        "test-command",
		Description: "Test command description",
	}

	err := model.UpsertCommand(ctx, cmd)
	require.NoError(t, err)
	assert.NotZero(t, cmd.ID)

	// Update via upsert
	cmd.Description = "Updated description"
	err = model.UpsertCommand(ctx, cmd)
	require.NoError(t, err)
}

// ===== TicketModel Tests =====
