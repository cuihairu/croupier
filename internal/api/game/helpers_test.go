// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package game

import (
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/datatypes"
)

func TestSanitizeGameName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid name with letters",
			input:   "MyGame",
			want:    "MyGame",
			wantErr: false,
		},
		{
			name:    "valid name with numbers",
			input:   "Game123",
			want:    "Game123",
			wantErr: false,
		},
		{
			name:    "valid name with underscore",
			input:   "my_game",
			want:    "my_game",
			wantErr: false,
		},
		{
			name:    "valid name with dash",
			input:   "my-game",
			want:    "my-game",
			wantErr: false,
		},
		{
			name:    "valid name with @",
			input:   "game@prod",
			want:    "game@prod",
			wantErr: false,
		},
		{
			name:    "valid name with all allowed chars",
			input:   "Game-123_@test",
			want:    "Game-123_@test",
			wantErr: false,
		},
		{
			name:      "empty string",
			input:     "",
			want:      "",
			wantErr:   true,
			errSubstr: "游戏名称不能为空",
		},
		{
			name:      "whitespace only",
			input:     "   ",
			want:      "",
			wantErr:   true,
			errSubstr: "游戏名称不能为空",
		},
		{
			name:    "whitespace trimmed",
			input:   "  MyGame  ",
			want:    "MyGame",
			wantErr: false,
		},
		{
			name:      "invalid character - space",
			input:     "My Game",
			want:      "",
			wantErr:   true,
			errSubstr: "游戏名称仅支持",
		},
		{
			name:      "invalid character - special chars",
			input:     "Game!",
			want:      "",
			wantErr:   true,
			errSubstr: "游戏名称仅支持",
		},
		{
			name:      "invalid character - chinese",
			input:     "游戏",
			want:      "",
			wantErr:   true,
			errSubstr: "游戏名称仅支持",
		},
		{
			name:      "invalid character - dot",
			input:     "my.game",
			want:      "",
			wantErr:   true,
			errSubstr: "游戏名称仅支持",
		},
		{
			name:      "invalid character - colon",
			input:     "my:game",
			want:      "",
			wantErr:   true,
			errSubstr: "游戏名称仅支持",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeGameName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeGameName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errSubstr)
				}
			}
			if got != tt.want {
				t.Errorf("sanitizeGameName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			want:    "",
			wantErr: false,
		},
		{
			name:    "valid status - dev",
			input:   "dev",
			want:    "dev",
			wantErr: false,
		},
		{
			name:    "valid status - test",
			input:   "test",
			want:    "test",
			wantErr: false,
		},
		{
			name:    "valid status - running",
			input:   "running",
			want:    "running",
			wantErr: false,
		},
		{
			name:    "valid status - online",
			input:   "online",
			want:    "online",
			wantErr: false,
		},
		{
			name:    "valid status - offline",
			input:   "offline",
			want:    "offline",
			wantErr: false,
		},
		{
			name:    "valid status - maintenance",
			input:   "maintenance",
			want:    "maintenance",
			wantErr: false,
		},
		{
			name:    "whitespace trimmed",
			input:   "  dev  ",
			want:    "dev",
			wantErr: false,
		},
		{
			name:      "invalid status - case sensitive DEV",
			input:     "DEV",
			want:      "",
			wantErr:   true,
			errSubstr: "无效的游戏状态",
		},
		{
			name:      "invalid status - case sensitive Running",
			input:     "Running",
			want:      "",
			wantErr:   true,
			errSubstr: "无效的游戏状态",
		},
		{
			name:      "invalid status",
			input:     "invalid",
			want:      "",
			wantErr:   true,
			errSubstr: "无效的游戏状态",
		},
		{
			name:      "invalid status - partial match",
			input:     "devtest",
			want:      "",
			wantErr:   true,
			errSubstr: "无效的游戏状态",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeStatus(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errSubstr)
				}
			}
			if got != tt.want {
				t.Errorf("sanitizeStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindEnvIndex(t *testing.T) {
	t.Parallel()

	envs := []model.GameEnv{
		{Env: "production"},
		{Env: "staging"},
		{Env: "development"},
	}

	tests := []struct {
		name      string
		envs      []model.GameEnv
		searchEnv string
		want      int
	}{
		{
			name:      "found at beginning",
			envs:      envs,
			searchEnv: "production",
			want:      0,
		},
		{
			name:      "found in middle",
			envs:      envs,
			searchEnv: "staging",
			want:      1,
		},
		{
			name:      "found at end",
			envs:      envs,
			searchEnv: "development",
			want:      2,
		},
		{
			name:      "not found",
			envs:      envs,
			searchEnv: "nonexistent",
			want:      -1,
		},
		{
			name:      "case insensitive match",
			envs:      envs,
			searchEnv: "PRODUCTION",
			want:      0,
		},
		{
			name:      "case insensitive match mixed",
			envs:      envs,
			searchEnv: "StAgInG",
			want:      1,
		},
		{
			name:      "empty slice",
			envs:      []model.GameEnv{},
			searchEnv: "production",
			want:      -1,
		},
		{
			name:      "empty search string",
			envs:      envs,
			searchEnv: "",
			want:      -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findEnvIndex(tt.envs, tt.searchEnv)
			if got != tt.want {
				t.Errorf("findEnvIndex() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEnsureEnvName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "valid name",
			input:   "production",
			want:    "production",
			wantErr: false,
		},
		{
			name:    "whitespace trimmed",
			input:   "  staging  ",
			want:    "staging",
			wantErr: false,
		},
		{
			name:      "empty string",
			input:     "",
			want:      "",
			wantErr:   true,
			errSubstr: "环境名称不能为空",
		},
		{
			name:      "whitespace only",
			input:     "   ",
			want:      "",
			wantErr:   true,
			errSubstr: "环境名称不能为空",
		},
		{
			name:    "name with special chars allowed",
			input:   "prod-1",
			want:    "prod-1",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ensureEnvName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureEnvName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errSubstr)
				}
			}
			if got != tt.want {
				t.Errorf("ensureEnvName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertGameEnvs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		envs  []model.GameEnv
		want  []GameEnvItem
		count int
	}{
		{
			name:  "empty slice",
			envs:  []model.GameEnv{},
			want:  []GameEnvItem{},
			count: 0,
		},
		{
			name: "single env",
			envs: []model.GameEnv{
				{Env: "prod", Description: "Production", Color: "green"},
			},
			count: 1,
		},
		{
			name: "multiple envs",
			envs: []model.GameEnv{
				{Env: "prod", Description: "Production", Color: "green"},
				{Env: "dev", Description: "Development", Color: "blue"},
				{Env: "test", Description: "Testing", Color: "yellow"},
			},
			count: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertGameEnvs(tt.envs)
			if len(got) != tt.count {
				t.Errorf("convertGameEnvs() length = %d, want %d", len(got), tt.count)
			}
			for i := range got {
				if got[i].Env != tt.envs[i].Env {
					t.Errorf("convertGameEnvs()[%d].Env = %q, want %q", i, got[i].Env, tt.envs[i].Env)
				}
				if got[i].Description != tt.envs[i].Description {
					t.Errorf("convertGameEnvs()[%d].Description = %q, want %q", i, got[i].Description, tt.envs[i].Description)
				}
				if got[i].Color != tt.envs[i].Color {
					t.Errorf("convertGameEnvs()[%d].Color = %q, want %q", i, got[i].Color, tt.envs[i].Color)
				}
			}
		})
	}
}

func TestBuildGameInfo(t *testing.T) {
	t.Parallel()

	now := time.Now()
	envsData := []byte(`[{"env":"prod","description":"Production","color":"green"}]`)
	game := &model.Game{
		Name:        "TestGame",
		Icon:        "icon.png",
		Description: "Test Description",
		Enabled:     true,
		AliasName:   "testgame",
		Homepage:    "https://example.com",
		Status:      "online",
		GameType:    "mmo",
		GenreCode:   "rpg",
		Color:       "blue",
		Envs:        datatypes.JSON(envsData),
	}
	// Set gorm.Model fields directly
	game.ID = 123
	game.CreatedAt = now
	game.UpdatedAt = now

	info := buildGameInfo(game)

	if info.ID != 123 {
		t.Errorf("ID = %d, want 123", info.ID)
	}
	if info.Name != "TestGame" {
		t.Errorf(`Name = %s, want "TestGame"`, info.Name)
	}
	if info.Icon != "icon.png" {
		t.Errorf(`Icon = %s, want "icon.png"`, info.Icon)
	}
	if info.Description != "Test Description" {
		t.Errorf(`Description = %s, want "Test Description"`, info.Description)
	}
	if !info.Enabled {
		t.Error("Enabled should be true")
	}
	if info.AliasName != "testgame" {
		t.Errorf(`AliasName = %s, want "testgame"`, info.AliasName)
	}
	if info.Homepage != "https://example.com" {
		t.Errorf(`Homepage = %s, want "https://example.com"`, info.Homepage)
	}
	if info.Status != "online" {
		t.Errorf(`Status = %s, want "online"`, info.Status)
	}
	if info.GameType != "mmo" {
		t.Errorf(`GameType = %s, want "mmo"`, info.GameType)
	}
	if info.GenreCode != "rpg" {
		t.Errorf(`GenreCode = %s, want "rpg"`, info.GenreCode)
	}
	if info.Color != "blue" {
		t.Errorf(`Color = %s, want "blue"`, info.Color)
	}
	if info.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
	if info.UpdatedAt == "" {
		t.Error("UpdatedAt should not be empty")
	}
}

func TestParseGameID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    uint
		wantErr bool
	}{
		{
			name:    "valid numeric string",
			input:   "123",
			want:    123,
			wantErr: false,
		},
		{
			name:    "zero - not allowed",
			input:   "0",
			want:    0,
			wantErr: true,
		},
		{
			name:    "large number",
			input:   "4294967295",
			want:    4294967295,
			wantErr: false,
		},
		{
			name:    "invalid - negative",
			input:   "-1",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid - not a number",
			input:   "abc",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid - empty string",
			input:   "",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid - float",
			input:   "1.5",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid - with spaces",
			input:   " 123 ",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGameID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseGameID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseGameID() = %d, want %d", got, tt.want)
			}
		})
	}
}
