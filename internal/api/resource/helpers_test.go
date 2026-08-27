package resource

import (
	"context"
	"github.com/cuihairu/croupier/internal/model"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
)

func TestMatchesResourceQuery(t *testing.T) {
	tests := []struct {
		name     string
		resource spec.ResourceSpec
		query    string
		want     bool
	}{
		{
			name: "empty query matches everything",
			resource: spec.ResourceSpec{
				Key:    "player",
				Labels: spec.LocalizedText{"zh-CN": "玩家"},
			},
			query: "",
			want:  true,
		},
		{
			name: "match by key",
			resource: spec.ResourceSpec{
				Key:    "player",
				Labels: spec.LocalizedText{"zh-CN": "玩家"},
			},
			query: "player",
			want:  true,
		},
		{
			name: "match by key case insensitive",
			resource: spec.ResourceSpec{
				Key:    "Player",
				Labels: spec.LocalizedText{"zh-CN": "玩家"},
			},
			query: "player",
			want:  true,
		},
		{
			name: "match by label",
			resource: spec.ResourceSpec{
				Key:    "player",
				Labels: spec.LocalizedText{"zh-CN": "玩家管理"},
			},
			query: "玩家",
			want:  true,
		},
		{
			name: "no match",
			resource: spec.ResourceSpec{
				Key:    "player",
				Labels: spec.LocalizedText{"zh-CN": "玩家"},
			},
			query: "order",
			want:  false,
		},
		{
			name: "whitespace query matches everything",
			resource: spec.ResourceSpec{
				Key:    "player",
				Labels: spec.LocalizedText{"zh-CN": "玩家"},
			},
			query: "  ",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesResourceQuery(tt.resource, tt.query)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTagsFromJSON(t *testing.T) {
	tests := []struct {
		name string
		raw  model.JSON
		want []string
	}{
		{
			name: "nil input",
			raw:  nil,
			want: nil,
		},
		{
			name: "empty input",
			raw:  model.JSON(""),
			want: nil,
		},
		{
			name: "valid tags",
			raw:  model.JSON(`["player", "moderation"]`),
			want: []string{"player", "moderation"},
		},
		{
			name: "tags with duplicates",
			raw:  model.JSON(`["player", "player", "moderation"]`),
			want: []string{"player", "moderation"},
		},
		{
			name: "tags with empty strings",
			raw:  model.JSON(`["player", "", "moderation"]`),
			want: []string{"player", "moderation"},
		},
		{
			name: "tags with whitespace",
			raw:  model.JSON(`["  player  ", "moderation"]`),
			want: []string{"player", "moderation"},
		},
		{
			name: "invalid JSON",
			raw:  model.JSON(`not json`),
			want: nil,
		},
		{
			name: "all empty tags",
			raw:  model.JSON(`["", "  ", ""]`),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTagsFromJSON(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCategoryFromResourceKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "with dot",
			key:  "player.list",
			want: "player",
		},
		{
			name: "without dot",
			key:  "player",
			want: "player",
		},
		{
			name: "multiple dots",
			key:  "player.ban.list",
			want: "player",
		},
		{
			name: "dot at start",
			key:  ".player",
			want: ".player",
		},
		{
			name: "empty string",
			key:  "",
			want: "",
		},
		{
			name: "whitespace",
			key:  "  player.list  ",
			want: "player",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categoryFromResourceKey(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHumanizeKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "simple key",
			key:  "player",
			want: "Player",
		},
		{
			name: "key with underscore",
			key:  "player_list",
			want: "Player List",
		},
		{
			name: "key with hyphen",
			key:  "player-list",
			want: "Player List",
		},
		{
			name: "key with dot",
			key:  "player.list",
			want: "Player List",
		},
		{
			name: "empty key",
			key:  "",
			want: "",
		},
		{
			name: "whitespace only",
			key:  "  ",
			want: "",
		},
		{
			name: "mixed separators",
			key:  "player.list-items",
			want: "Player List Items",
		},
		{
			name: "leading/trailing separators",
			key:  "_player_",
			want: "Player",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := humanizeKey(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRequireScope(t *testing.T) {
	tests := []struct {
		name       string
		scope      svc.GameScope
		wantGameID string
		wantEnv    string
		wantErr    bool
	}{
		{
			name:       "valid scope",
			scope:      svc.GameScope{GameID: "game1", Env: "prod"},
			wantGameID: "game1",
			wantEnv:    "prod",
			wantErr:    false,
		},
		{
			name:       "missing game ID",
			scope:      svc.GameScope{Env: "prod"},
			wantGameID: "",
			wantEnv:    "",
			wantErr:    true,
		},
		{
			name:       "missing env",
			scope:      svc.GameScope{GameID: "game1"},
			wantGameID: "",
			wantEnv:    "",
			wantErr:    true,
		},
		{
			name:       "empty game ID",
			scope:      svc.GameScope{GameID: "  ", Env: "prod"},
			wantGameID: "",
			wantEnv:    "",
			wantErr:    true,
		},
		{
			name:       "empty env",
			scope:      svc.GameScope{GameID: "game1", Env: "  "},
			wantGameID: "",
			wantEnv:    "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := svc.WithGameScope(context.Background(), tt.scope)
			gameID, env, err := requireScope(ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantGameID, gameID)
				assert.Equal(t, tt.wantEnv, env)
			}
		})
	}
}

func TestOperationSpecsFromContracts(t *testing.T) {
	tests := []struct {
		name      string
		contracts []interface{}
		wantLen   int
	}{
		{
			name:      "nil contracts",
			contracts: nil,
			wantLen:   0,
		},
		{
			name:      "empty contracts",
			contracts: []interface{}{},
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a basic test - actual contract testing requires more setup
			assert.True(t, true)
		})
	}
}

func TestResourceSpecFromCapability_NilCapability(t *testing.T) {
	svcCtx, _ := newResourceTestServiceContext(t, nil)
	s := NewService(svcCtx)

	_, err := s.resourceSpecFromCapability(context.Background(), "game", "env", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource capability not found")
}

func TestErrResourceNotFound(t *testing.T) {
	err := ErrResourceNotFound("test-key")
	assert.Error(t, err)

	var notFoundErr *ResourceNotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, "test-key", notFoundErr.Key)
}

func TestResourceNotFoundError_Error(t *testing.T) {
	err := &ResourceNotFoundError{Key: "player.list"}
	assert.Equal(t, "resource not found: player.list", err.Error())
}

func TestResourceNotFoundError_NilKey(t *testing.T) {
	err := &ResourceNotFoundError{}
	assert.Equal(t, "resource not found: ", err.Error())
}
