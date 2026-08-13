package svc

import (
	"context"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/config"
)

func TestGameScopeFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected GameScope
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: GameScope{},
		},
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: GameScope{},
		},
		{
			name:     "with scope",
			ctx:      WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"}),
			expected: GameScope{GameID: "g1", Env: "prod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GameScopeFromContext(tt.ctx)
			if result.GameID != tt.expected.GameID || result.Env != tt.expected.Env {
				t.Errorf("GameScopeFromContext() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWithGameScope(t *testing.T) {
	ctx := WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"})
	scope := GameScopeFromContext(ctx)
	if scope.GameID != "g1" || scope.Env != "prod" {
		t.Errorf("WithGameScope() scope = %v, want {g1, prod}", scope)
	}
}

func TestResolveGameID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		fallback string
		expected string
	}{
		{
			name:     "from context",
			ctx:      WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"}),
			fallback: "fallback",
			expected: "g1",
		},
		{
			name:     "fallback when no scope",
			ctx:      context.Background(),
			fallback: "fallback",
			expected: "fallback",
		},
		{
			name:     "nil context uses fallback",
			ctx:      nil,
			fallback: "fallback",
			expected: "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveGameID(tt.ctx, tt.fallback)
			if result != tt.expected {
				t.Errorf("ResolveGameID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestResolveEnv(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		fallback string
		expected string
	}{
		{
			name:     "from context",
			ctx:      WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"}),
			fallback: "dev",
			expected: "prod",
		},
		{
			name:     "fallback when no scope",
			ctx:      context.Background(),
			fallback: "dev",
			expected: "dev",
		},
		{
			name:     "nil context uses fallback",
			ctx:      nil,
			fallback: "dev",
			expected: "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveEnv(tt.ctx, tt.fallback)
			if result != tt.expected {
				t.Errorf("ResolveEnv() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCurrentScope(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantErr bool
	}{
		{
			name:    "with scope",
			ctx:     WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"}),
			wantErr: false,
		},
		{
			name:    "empty context",
			ctx:     context.Background(),
			wantErr: true,
		},
		{
			name:    "partial scope",
			ctx:     WithGameScope(context.Background(), GameScope{GameID: "g1"}),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope, err := CurrentScope(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("CurrentScope() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && scope.GameID == "" {
				t.Error("CurrentScope() should return non-empty GameID")
			}
		})
	}
}

func TestScopeMatches(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		gameID string
		env    string
		want   bool
	}{
		{
			name:   "matches",
			ctx:    WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"}),
			gameID: "g1",
			env:    "prod",
			want:   true,
		},
		{
			name:   "game mismatch",
			ctx:    WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"}),
			gameID: "g2",
			env:    "prod",
			want:   false,
		},
		{
			name:   "env mismatch",
			ctx:    WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"}),
			gameID: "g1",
			env:    "dev",
			want:   false,
		},
		{
			name:   "no scope",
			ctx:    context.Background(),
			gameID: "g1",
			env:    "prod",
			want:   false,
		},
		{
			name:   "case insensitive",
			ctx:    WithGameScope(context.Background(), GameScope{GameID: "G1", Env: "PROD"}),
			gameID: "g1",
			env:    "prod",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ScopeMatches(tt.ctx, tt.gameID, tt.env)
			if result != tt.want {
				t.Errorf("ScopeMatches() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestScopeMatchesGame(t *testing.T) {
	tests := []struct {
		name   string
		ctx    context.Context
		gameID string
		want   bool
	}{
		{
			name:   "matches",
			ctx:    WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"}),
			gameID: "g1",
			want:   true,
		},
		{
			name:   "mismatch",
			ctx:    WithGameScope(context.Background(), GameScope{GameID: "g1", Env: "prod"}),
			gameID: "g2",
			want:   false,
		},
		{
			name:   "no scope",
			ctx:    context.Background(),
			gameID: "g1",
			want:   false,
		},
		{
			name:   "case insensitive",
			ctx:    WithGameScope(context.Background(), GameScope{GameID: "G1", Env: "prod"}),
			gameID: "g1",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ScopeMatchesGame(tt.ctx, tt.gameID)
			if result != tt.want {
				t.Errorf("ScopeMatchesGame() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestGameScopeNotFoundError(t *testing.T) {
	err := errGameScopeNotFound
	if err.Error() != "game scope not found" {
		t.Errorf("gameScopeNotFoundError.Error() = %q, want %q", err.Error(), "game scope not found")
	}
}

// --- Service context helper tests ---

func TestFirstNonEmptySvc(t *testing.T) {
	tests := []struct {
		name     string
		values   []string
		expected string
	}{
		{"all empty", []string{"", "", ""}, ""},
		{"first non-empty", []string{"a", "b", "c"}, "a"},
		{"second non-empty", []string{"", "b", "c"}, "b"},
		{"third non-empty", []string{"", "", "c"}, "c"},
		{"with spaces", []string{"  ", "hello", ""}, "hello"},
		{"empty slice", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmpty(tt.values...)
			if got != tt.expected {
				t.Errorf("firstNonEmpty() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseTelemetryDurationSvc(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{"empty", "", 0},
		{"spaces", "  ", 0},
		{"valid seconds", "5s", 5 * time.Second},
		{"valid minutes", "1m", time.Minute},
		{"invalid", "abc", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTelemetryDuration(tt.value)
			if got != tt.expected {
				t.Errorf("parseTelemetryDuration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsDevelopmentConfigSvc(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.Config
		expected bool
	}{
		{"dev mode", config.Config{Server: config.ServerConfig{Mode: "dev"}}, true},
		{"development mode", config.Config{Server: config.ServerConfig{Mode: "development"}}, true},
		{"debug mode", config.Config{Server: config.ServerConfig{Mode: "debug"}}, true},
		{"empty mode", config.Config{}, true}, // defaults to dev
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDevelopmentConfig(tt.cfg)
			if got != tt.expected {
				t.Errorf("isDevelopmentConfig() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSplitPermissionCodeSvc(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		wantResource string
		wantAction   string
	}{
		{"empty", "", "", ""},
		{"resource only", "player", "player", "*"},
		{"resource and action", "player:create", "player", "create"},
		{"with spaces", "  player : create  ", "player", "create"},
		{"empty action", "player:", "player", "*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, action := splitPermissionCode(tt.code)
			if resource != tt.wantResource || action != tt.wantAction {
				t.Errorf("splitPermissionCode(%q) = (%q, %q), want (%q, %q)",
					tt.code, resource, action, tt.wantResource, tt.wantAction)
			}
		})
	}
}

func TestDerivePermissionResourceActionSvc(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		module       string
		wantResource string
		wantAction   string
	}{
		{"code only", "player:create", "", "player", "create"},
		{"module overrides resource", "player:create", "admin", "admin", "create"},
		{"empty code and module", "", "", "global", "*"},
		{"module only", "", "player", "player", "*"},
		{"code no action", "player", "", "player", "*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, action := derivePermissionResourceAction(tt.code, tt.module)
			if resource != tt.wantResource || action != tt.wantAction {
				t.Errorf("derivePermissionResourceAction(%q, %q) = (%q, %q), want (%q, %q)",
					tt.code, tt.module, resource, action, tt.wantResource, tt.wantAction)
			}
		})
	}
}

func TestResolveBootstrapAuthDirSvc(t *testing.T) {
	t.Run("empty config uses default", func(t *testing.T) {
		dir := resolveBootstrapAuthDir(config.Config{})
		if dir == "" {
			t.Error("expected non-empty dir")
		}
	})
}

func TestResolveBootstrapBaseDirSvc(t *testing.T) {
	t.Run("empty config uses default", func(t *testing.T) {
		dir := resolveBootstrapBaseDir(config.Config{})
		if dir == "" {
			t.Error("expected non-empty dir")
		}
	})
}

func TestResolveTaskRoutingDirSvc(t *testing.T) {
	t.Run("empty config uses default", func(t *testing.T) {
		dir := resolveTaskRoutingDir(config.Config{})
		if dir == "" {
			t.Error("expected non-empty dir")
		}
	})
}
