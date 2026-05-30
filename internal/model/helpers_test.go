// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package model

import (
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
