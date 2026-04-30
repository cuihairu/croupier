package registry

import (
	"context"
	"testing"

	functionv1 "github.com/cuihairu/croupier/pkg/pb/croupier/function/v1"
)

func TestStore_Register(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadata := &functionv1.FunctionMetadata{
		Id:           "test.function",
		Version:      "1.0.0",
		Category:     "test",
		Name:         "Test Function",
		Description:  "A test function",
		Tags:         []string{"test", "example"},
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"object"}`,
		Behavior: &functionv1.FunctionBehavior{
			Mode:          functionv1.FunctionBehavior_QUERY,
			Idempotent:    true,
			TimeoutMs:     30000,
			RouteStrategy: functionv1.FunctionBehavior_ROUTE_LB,
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel:        functionv1.FunctionSecurity_RISK_LOW,
			Permission:       "test.function.invoke",
			RequiresApproval: false,
			AuditLog:         true,
		},
	}

	err := store.Register(ctx, metadata)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify retrieval
	retrieved, err := store.Get(ctx, "test.function")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Id != metadata.Id {
		t.Errorf("Expected ID %s, got %s", metadata.Id, retrieved.Id)
	}

	if retrieved.Category != metadata.Category {
		t.Errorf("Expected category %s, got %s", metadata.Category, retrieved.Category)
	}
}

func TestStore_RegisterBatch(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadatas := []*functionv1.FunctionMetadata{
		{
			Id:       "test.function1",
			Category: "test",
			Tags:     []string{"test"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
		},
		{
			Id:       "test.function2",
			Category: "test",
			Tags:     []string{"test"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_MEDIUM},
		},
	}

	err := store.RegisterBatch(ctx, metadatas)
	if err != nil {
		t.Fatalf("RegisterBatch failed: %v", err)
	}

	count := store.Count(ctx)
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

func TestStore_ListByCategory(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadatas := []*functionv1.FunctionMetadata{
		{
			Id:       "player.ban",
			Category: "player",
			Tags:     []string{"moderation"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_HIGH},
		},
		{
			Id:       "player.kick",
			Category: "player",
			Tags:     []string{"moderation"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_MEDIUM},
		},
		{
			Id:       "game.shutdown",
			Category: "game",
			Tags:     []string{"admin"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_DANGER},
		},
	}

	_ = store.RegisterBatch(ctx, metadatas)

	// List player functions
	players, err := store.ListByCategory(ctx, "player")
	if err != nil {
		t.Fatalf("ListByCategory failed: %v", err)
	}

	if len(players) != 2 {
		t.Errorf("Expected 2 player functions, got %d", len(players))
	}

	// List game functions
	games, err := store.ListByCategory(ctx, "game")
	if err != nil {
		t.Fatalf("ListByCategory failed: %v", err)
	}

	if len(games) != 1 {
		t.Errorf("Expected 1 game function, got %d", len(games))
	}
}

func TestStore_ListByTag(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadatas := []*functionv1.FunctionMetadata{
		{
			Id:       "player.ban",
			Category: "player",
			Tags:     []string{"moderation", "high-risk"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_HIGH},
		},
		{
			Id:       "player.kick",
			Category: "player",
			Tags:     []string{"moderation"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_MEDIUM},
		},
	}

	_ = store.RegisterBatch(ctx, metadatas)

	// List by moderation tag
	moderation, err := store.ListByTag(ctx, "moderation")
	if err != nil {
		t.Fatalf("ListByTag failed: %v", err)
	}

	if len(moderation) != 2 {
		t.Errorf("Expected 2 moderation functions, got %d", len(moderation))
	}

	// List by high-risk tag
	highRisk, err := store.ListByTag(ctx, "high-risk")
	if err != nil {
		t.Fatalf("ListByTag failed: %v", err)
	}

	if len(highRisk) != 1 {
		t.Errorf("Expected 1 high-risk function, got %d", len(highRisk))
	}
}

func TestStore_ListByRiskLevel(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadatas := []*functionv1.FunctionMetadata{
		{
			Id:       "safe.query",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
		},
		{
			Id:       "danger.command",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_DANGER},
		},
	}

	_ = store.RegisterBatch(ctx, metadatas)

	// Debug: check what's in the store
	count := store.Count(ctx)
	t.Logf("Total functions registered: %d", count)

	// Debug: List all functions to check their risk levels
	allFuncs, _ := store.List(ctx)
	for _, f := range allFuncs {
		t.Logf("Function %s has risk level %s", f.Id, f.Security.RiskLevel.String())
	}

	// Check risk index
	categories := store.GetCategories(ctx)
	t.Logf("Categories: %v", categories)

	low, err := store.ListByRiskLevel(ctx, "risk_low")
	if err != nil {
		t.Fatalf("ListByRiskLevel failed: %v", err)
	}

	if len(low) != 1 {
		t.Errorf("Expected 1 low-risk function, got %d", len(low))
	}

	danger, err := store.ListByRiskLevel(ctx, "risk_danger")
	if err != nil {
		t.Fatalf("ListByRiskLevel failed: %v", err)
	}

	if len(danger) != 1 {
		t.Errorf("Expected 1 danger function, got %d", len(danger))
	}
}

func TestStore_ListByMode(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadatas := []*functionv1.FunctionMetadata{
		{
			Id:       "query.test",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
		},
		{
			Id:       "command.test",
			Category: "test",
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_MEDIUM},
		},
	}

	_ = store.RegisterBatch(ctx, metadatas)

	queries, err := store.ListByMode(ctx, functionv1.FunctionBehavior_QUERY)
	if err != nil {
		t.Fatalf("ListByMode failed: %v", err)
	}

	if len(queries) != 1 {
		t.Errorf("Expected 1 query function, got %d", len(queries))
	}

	commands, err := store.ListByMode(ctx, functionv1.FunctionBehavior_COMMAND)
	if err != nil {
		t.Fatalf("ListByMode failed: %v", err)
	}

	if len(commands) != 1 {
		t.Errorf("Expected 1 command function, got %d", len(commands))
	}
}

func TestStore_Filter(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadatas := []*functionv1.FunctionMetadata{
		{
			Id:       "player.ban",
			Category: "player",
			Tags:     []string{"moderation"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_HIGH},
		},
		{
			Id:       "player.info",
			Category: "player",
			Tags:     []string{"query"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
		},
		{
			Id:       "game.shutdown",
			Category: "game",
			Tags:     []string{"admin"},
			Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
			Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_DANGER},
		},
	}

	_ = store.RegisterBatch(ctx, metadatas)

	// Filter by category
	filter := &functionv1.FunctionFilter{
		Category: "player",
	}
	results, err := store.Filter(ctx, filter)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Filter by tag
	filter = &functionv1.FunctionFilter{
		Tags: []string{"moderation"},
	}
	results, err = store.Filter(ctx, filter)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// Filter by mode
	filter = &functionv1.FunctionFilter{
		Mode: "command",
	}
	results, err = store.Filter(ctx, filter)
	if err != nil {
		t.Fatalf("Filter failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestStore_Unregister(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadata := &functionv1.FunctionMetadata{
		Id:       "test.function",
		Category: "test",
		Tags:     []string{"test"},
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
	}

	_ = store.Register(ctx, metadata)

	err := store.Unregister(ctx, "test.function")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	exists := store.Exists(ctx, "test.function")
	if exists {
		t.Error("Function should not exist after unregister")
	}

	count := store.Count(ctx)
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}
}

func TestStore_GetCategories(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadatas := []*functionv1.FunctionMetadata{
		{Id: "p1", Category: "player", Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY}, Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW}},
		{Id: "p2", Category: "player", Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY}, Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW}},
		{Id: "g1", Category: "game", Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY}, Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW}},
	}

	_ = store.RegisterBatch(ctx, metadatas)

	categories := store.GetCategories(ctx)
	if len(categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(categories))
	}
}

func TestStore_GetTags(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadatas := []*functionv1.FunctionMetadata{
		{Id: "t1", Category: "test", Tags: []string{"tag1", "tag2"}, Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY}, Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW}},
		{Id: "t2", Category: "test", Tags: []string{"tag2", "tag3"}, Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY}, Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW}},
	}

	_ = store.RegisterBatch(ctx, metadatas)

	tags := store.GetTags(ctx)
	if len(tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tags))
	}
}

func TestStore_Update(t *testing.T) {
	ctx := context.Background()
	store := NewStore()

	metadata := &functionv1.FunctionMetadata{
		Id:       "test.function",
		Version:  "1.0.0",
		Category: "test",
		Tags:     []string{"test"},
		Name:     "Original Name",
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
	}

	_ = store.Register(ctx, metadata)

	// Update
	updated := &functionv1.FunctionMetadata{
		Id:       "test.function",
		Version:  "2.0.0",
		Category: "test",
		Tags:     []string{"test", "updated"},
		Name:     "Updated Name",
		Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_COMMAND},
		Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_MEDIUM},
	}

	_ = store.Register(ctx, updated)

	retrieved, _ := store.Get(ctx, "test.function")
	if retrieved.Version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", retrieved.Version)
	}

	if len(retrieved.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(retrieved.Tags))
	}
}

func TestRegistry_Validation(t *testing.T) {
	ctx := context.Background()
	registry := New()

	tests := []struct {
		name        string
		metadata    *functionv1.FunctionMetadata
		expectError bool
	}{
		{
			name: "valid metadata",
			metadata: &functionv1.FunctionMetadata{
				Id:       "test.function",
				Category: "test",
				Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
				Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
			},
			expectError: false,
		},
		{
			name:        "nil metadata",
			metadata:    nil,
			expectError: true,
		},
		{
			name: "missing ID",
			metadata: &functionv1.FunctionMetadata{
				Category: "test",
				Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
				Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
			},
			expectError: true,
		},
		{
			name: "missing security",
			metadata: &functionv1.FunctionMetadata{
				Id:       "test.function",
				Category: "test",
				Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
			},
			expectError: true,
		},
		{
			name: "missing behavior",
			metadata: &functionv1.FunctionMetadata{
				Id:       "test.function",
				Category: "test",
				Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
			},
			expectError: true,
		},
		{
			name: "invalid ID format",
			metadata: &functionv1.FunctionMetadata{
				Id:       "invalid",
				Category: "test",
				Behavior: &functionv1.FunctionBehavior{Mode: functionv1.FunctionBehavior_QUERY},
				Security: &functionv1.FunctionSecurity{RiskLevel: functionv1.FunctionSecurity_RISK_LOW},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Register(ctx, tt.metadata)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestRegistry_Wrapperspb(t *testing.T) {
	// Test that wrapperspb types work correctly
	ctx := context.Background()
	registry := New()

	metadata := &functionv1.FunctionMetadata{
		Id:       "test.function",
		Category: "test",
		Behavior: &functionv1.FunctionBehavior{
			Mode:            functionv1.FunctionBehavior_QUERY,
			Idempotent:      true,
			TimeoutMs:       30000,
			RouteStrategy:   functionv1.FunctionBehavior_ROUTE_LB,
			Cacheable:       true,
			CacheTtlSeconds: 60,
		},
		Security: &functionv1.FunctionSecurity{
			RiskLevel:         functionv1.FunctionSecurity_RISK_LOW,
			Permission:        "test.invoke",
			RequiresApproval:  false,
			AuditLog:          true,
			MaskSensitiveData: true,
		},
	}

	err := registry.Register(ctx, metadata)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	retrieved, err := registry.Get(ctx, "test.function")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Behavior.CacheTtlSeconds != 60 {
		t.Errorf("Expected CacheTtlSeconds 60, got %d", retrieved.Behavior.CacheTtlSeconds)
	}
}
