// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package config

import (
	"context"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
)

func TestConfigAuthor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ctx      context.Context
		wantUser string
	}{
		{
			name:     "nil context",
			ctx:      nil,
			wantUser: "system",
		},
		{
			name:     "context without username",
			ctx:      context.Background(),
			wantUser: "system",
		},
		{
			name:     "context with username",
			ctx:      context.WithValue(context.Background(), "username", "testuser"),
			wantUser: "testuser",
		},
		{
			name:     "context with whitespace username",
			ctx:      context.WithValue(context.Background(), "username", "  testuser  "),
			wantUser: "testuser",
		},
		{
			name:     "context with empty string username",
			ctx:      context.WithValue(context.Background(), "username", ""),
			wantUser: "system",
		},
		{
			name:     "context with only whitespace username",
			ctx:      context.WithValue(context.Background(), "username", "   "),
			wantUser: "system",
		},
		{
			name:     "context with non-string username value",
			ctx:      context.WithValue(context.Background(), "username", 123),
			wantUser: "system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configAuthor(tt.ctx)
			if got != tt.wantUser {
				t.Errorf("configAuthor() = %q, want %q", got, tt.wantUser)
			}
		})
	}
}

func TestMapConfigVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		version       *model.ConfigVersion
		includeValue  bool
		wantNil       bool
		checkValueKey bool
	}{
		{
			name:         "nil version",
			version:      nil,
			includeValue: false,
			wantNil:      true,
		},
		{
			name: "without value",
			version: &model.ConfigVersion{
				Key:       "test.key",
				Version:   1,
				CreatedBy: "admin",
				GameID:    "100",
				Env:       "production",
				Format:    "json",
				Message:   "initial version",
				Value:     `{"key": "value"}`,
			},
			includeValue:  false,
			wantNil:       false,
			checkValueKey: false,
		},
		{
			name: "with value",
			version: &model.ConfigVersion{
				Key:       "test.key",
				Version:   2,
				CreatedBy: "user",
				GameID:    "200",
				Env:       "dev",
				Format:    "yaml",
				Message:   "update",
				Value:     "key: value",
			},
			includeValue:  true,
			wantNil:       false,
			checkValueKey: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapConfigVersion(tt.version, tt.includeValue)

			if tt.wantNil {
				if got != nil {
					t.Error("mapConfigVersion() should return nil for nil input")
				}
				return
			}

			if got == nil {
				t.Fatal("mapConfigVersion() returned nil unexpectedly")
			}

			// Check required keys
			if got["key"] != tt.version.Key {
				t.Errorf("key = %v, want %v", got["key"], tt.version.Key)
			}
			if got["version"] != tt.version.Version {
				t.Errorf("version = %v, want %v", got["version"], tt.version.Version)
			}
			if got["createdBy"] != tt.version.CreatedBy {
				t.Errorf("createdBy = %v, want %v", got["createdBy"], tt.version.CreatedBy)
			}
			if got["gameId"] != tt.version.GameID {
				t.Errorf("gameId = %v, want %v", got["gameId"], tt.version.GameID)
			}
			if got["env"] != tt.version.Env {
				t.Errorf("env = %v, want %v", got["env"], tt.version.Env)
			}
			if got["format"] != tt.version.Format {
				t.Errorf("format = %v, want %v", got["format"], tt.version.Format)
			}
			if got["message"] != tt.version.Message {
				t.Errorf("message = %v, want %v", got["message"], tt.version.Message)
			}

			// Check value key presence
			_, hasValue := got["value"]
			if hasValue != tt.checkValueKey {
				t.Errorf("value key presence = %v, want %v", hasValue, tt.checkValueKey)
			}
		})
	}
}

func TestMapConfigItem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version *model.ConfigVersion
		wantNil bool
	}{
		{
			name:    "nil version",
			version: nil,
			wantNil: true,
		},
		{
			name: "normal config item",
			version: &model.ConfigVersion{
				Key:       "feature.flag",
				Version:   5,
				CreatedBy: "developer",
				GameID:    "300",
				Env:       "staging",
				Format:    "yaml",
				Message:   "enabled feature",
				Value:     "enabled: true",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapConfigItem(tt.version)

			if tt.wantNil {
				if got != nil {
					t.Error("mapConfigItem() should return nil for nil input")
				}
				return
			}

			if got == nil {
				t.Fatal("mapConfigItem() returned nil unexpectedly")
			}

			// Check all fields
			if got["id"] != tt.version.Key {
				t.Errorf("id = %v, want %v", got["id"], tt.version.Key)
			}
			if got["format"] != tt.version.Format {
				t.Errorf("format = %v, want %v", got["format"], tt.version.Format)
			}
			if got["gameId"] != tt.version.GameID {
				t.Errorf("gameId = %v, want %v", got["gameId"], tt.version.GameID)
			}
			if got["env"] != tt.version.Env {
				t.Errorf("env = %v, want %v", got["env"], tt.version.Env)
			}
			if got["latestVersion"] != tt.version.Version {
				t.Errorf("latestVersion = %v, want %v", got["latestVersion"], tt.version.Version)
			}
			if got["lastMessage"] != tt.version.Message {
				t.Errorf("lastMessage = %v, want %v", got["lastMessage"], tt.version.Message)
			}
			if got["lastModifiedBy"] != tt.version.CreatedBy {
				t.Errorf("lastModifiedBy = %v, want %v", got["lastModifiedBy"], tt.version.CreatedBy)
			}
		})
	}
}

func TestValidateConfigContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		format    string
		content   string
		wantErr   bool
		errSubstr string
	}{
		// JSON tests
		{
			name:    "valid JSON object",
			format:  "json",
			content: `{"key": "value", "number": 123}`,
			wantErr: false,
		},
		{
			name:    "valid JSON array",
			format:  "json",
			content: `["item1", "item2"]`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			format:  "json",
			content: `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "empty JSON",
			format:  "json",
			content: `{}`,
			wantErr: false,
		},
		// YAML tests
		{
			name:    "valid YAML",
			format:  "yaml",
			content: "key: value\nnumber: 123",
			wantErr: false,
		},
		{
			name:    "valid yml extension",
			format:  "yml",
			content: "key: value",
			wantErr: false,
		},
		{
			name:    "invalid YAML",
			format:  "yaml",
			content: ":\nbad: [yaml",
			wantErr: true,
		},
		// XML tests
		{
			name:    "valid XML",
			format:  "xml",
			content: `<root><key>value</key></root>`,
			wantErr: false,
		},
		{
			name:    "invalid XML",
			format:  "xml",
			content: `<root><unclosed>`,
			wantErr: true,
		},
		// INI tests
		{
			name:    "valid INI with sections",
			format:  "ini",
			content: "[section]\nkey=value\nkey2: value2",
			wantErr: false,
		},
		{
			name:    "valid INI with comments",
			format:  "ini",
			content: "# comment\n[section]\n; another comment\nkey=value",
			wantErr: false,
		},
		{
			name:    "empty INI",
			format:  "ini",
			content: "",
			wantErr: true,
		},
		{
			name:    "whitespace only INI",
			format:  "ini",
			content: "   \n  \t  ",
			wantErr: true,
		},
		{
			name:    "invalid INI line",
			format:  "ini",
			content: "[section]\ninvalid line without equals or colon",
			wantErr: true,
		},
		// CSV tests
		{
			name:    "valid CSV",
			format:  "csv",
			content: "header1,header2\nvalue1,value2",
			wantErr: false,
		},
		{
			name:    "empty CSV",
			format:  "csv",
			content: "",
			wantErr: true,
		},
		{
			name:    "whitespace CSV",
			format:  "csv",
			content: "  ",
			wantErr: true,
		},
		// lua/python 脚本式配置：非空即通过（语法由编辑器/游戏侧兜底）
		{
			name:    "lua config",
			format:  "lua",
			content: `return { level = 1, name = "slime" }`,
			wantErr: false,
		},
		{
			name:    "python config",
			format:  "python",
			content: "CONFIG = {'level': 1}\n",
			wantErr: false,
		},
		{
			name:    "python alias py",
			format:  "py",
			content: "CONFIG = {}\n",
			wantErr: false,
		},
		{
			name:    "empty lua",
			format:  "lua",
			content: "  ",
			wantErr: true,
		},
		{
			name:    "lua with NUL byte (binary upload)",
			format:  "lua",
			content: "return {}\x00",
			wantErr: true,
		},
		// Default/empty format (should parse as JSON)
		{
			name:    "empty format defaults to JSON",
			format:  "",
			content: `{"key": "value"}`,
			wantErr: false,
		},
		{
			name:      "unsupported format",
			format:    "toml",
			content:   "key = value",
			wantErr:   true,
			errSubstr: "unsupported",
		},
		// Case insensitive format
		{
			name:    "uppercase JSON",
			format:  "JSON",
			content: `{"key": "value"}`,
			wantErr: false,
		},
		{
			name:    "mixed case YAML",
			format:  "Yaml",
			content: "key: value",
			wantErr: false,
		},
		// Format with whitespace
		{
			name:    "format with leading/trailing space",
			format:  "  json  ",
			content: `{"key": "value"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfigContent(tt.format, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfigContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errSubstr)
				} else if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("Expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
			}
		})
	}
}
