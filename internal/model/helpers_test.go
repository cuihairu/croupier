// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package model

import (
	"strings"
	"testing"

	"gorm.io/gorm/clause"
)

func TestUpsertAllColumns(t *testing.T) {
	t.Parallel()

	result := upsertAllColumns()

	// Verify it returns a clause.OnConflict with UpdateAll set
	onConflict, ok := result.(clause.OnConflict)
	if !ok {
		t.Fatal("upsertAllColumns() should return clause.OnConflict")
	}

	if !onConflict.UpdateAll {
		t.Error("upsertAllColumns() should have UpdateAll set to true")
	}

	// Verify other fields are empty (defaults)
	if onConflict.Columns != nil {
		t.Error("upsertAllColumns() Columns should be nil")
	}
	if onConflict.OnConstraint != "" {
		t.Errorf("upsertAllColumns() OnConstraint should be empty, got %q", onConflict.OnConstraint)
	}
	if len(onConflict.DoUpdates) != 0 {
		t.Error("upsertAllColumns() DoUpdates should be empty when UpdateAll is true")
	}
}

// TestDeriveGameIDFromName tests the deriveGameIDFromName function
func TestDeriveGameIDFromName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", "game"},
		{"whitespace only", "   ", "game"},
		{"simple name", "MyGame", "mygame"},
		{"with spaces", "My Game", "my_game"},
		{"with underscores", "my_game", "my_game"},
		{"with hyphens", "my-game", "my_game"},
		{"multiple spaces", "My   Game", "my_game"},
		{"special chars removed", "My@Game!#$", "mygame"},
		{"numbers preserved", "Game123", "game123"},
		{"leading space", "  Game", "game"},
		{"trailing space", "Game  ", "game"},
		{"mixed separators", "My - Game", "my_game"},
		{"consecutive separators", "My___Game", "my_game"},
		{"only special chars", "@#$%", "game"},
		{"chinese chars removed", "游戏测试", "game"},
		{"lowercase", "game", "game"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveGameIDFromName(tt.input)
			if result != tt.expected {
				t.Errorf("deriveGameIDFromName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestQuoteIdent tests the quoteIdent function
func TestQuoteIdent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple identifier", "users", `"users"`},
		{"with spaces", "user name", `"user name"`},
		{"with double quote", `table"name`, `"table""name"`},
		{"empty string", "", `""`},
		{"with special chars", "my-table", `"my-table"`},
		{"with dots", "public.users", `"public.users"`},
		{"multiple quotes", `a"b"c`, `"a""b""c"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := quoteIdent(tt.input)
			if result != tt.expected {
				t.Errorf("quoteIdent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestQuoteIdentRoundTrip tests that quoteIdent produces valid quoted identifiers
func TestQuoteIdentRoundTrip(t *testing.T) {
	identifiers := []string{"users", "my_table", "schema.table", "column name"}
	for _, id := range identifiers {
		quoted := quoteIdent(id)
		if !strings.HasPrefix(quoted, `"`) || !strings.HasSuffix(quoted, `"`) {
			t.Errorf("quoteIdent(%q) should be quoted, got %q", id, quoted)
		}
	}
}

// TestUniqueStrings tests the uniqueStrings function
func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"nil input", nil, nil},
		{"empty input", []string{}, nil},
		{"single element", []string{"a"}, []string{"a"}},
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"with duplicates", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"with empty strings", []string{"a", "", "b", ""}, []string{"a", "b"}},
		{"with whitespace", []string{"  a  ", "a", " b "}, []string{"a", "b"}},
		{"all empty", []string{"", "", ""}, nil},
		{"all duplicates", []string{"x", "x", "x"}, []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueStrings(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("uniqueStrings(%v) returned %d elements, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("uniqueStrings(%v)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}
