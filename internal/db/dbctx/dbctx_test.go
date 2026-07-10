package dbctx

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

// stubDB is a sentinel *gorm.DB distinguishable only by pointer identity.
func stubDB() *gorm.DB { return &gorm.DB{} }

// TestResolve_OverrideWins asserts the database-per-game routing contract:
// when a request context carries a game DB override (set by GameDBMiddleware),
// Resolve MUST return that override, never the fallback meta DB.
//
// This is the regression guard for P1 scope boundary: a game-scoped model
// constructed with the meta DB (fallback) must still route to the per-game DB
// when the request context injected one.
func TestResolve_OverrideWins(t *testing.T) {
	metaDB := stubDB()
	gameDB := stubDB()

	// Without override: fallback (meta DB) is used.
	ctx := context.Background()
	if got := Resolve(ctx, metaDB); got != metaDB {
		t.Error("Resolve without override must return fallback meta DB")
	}

	// With override: per-game DB wins.
	gameCtx := WithDB(ctx, gameDB)
	if got := Resolve(gameCtx, metaDB); got != gameDB {
		t.Error("Resolve with override must return the per-game DB, not fallback")
	}
}

// TestResolve_NilClearsOverride verifies that a nil override clears any prior
// override so the fallback is used again.
func TestResolve_NilClearsOverride(t *testing.T) {
	metaDB := stubDB()
	gameDB := stubDB()

	ctx := WithDB(context.Background(), gameDB)
	if got := Resolve(ctx, metaDB); got != gameDB {
		t.Fatal("override should be active")
	}

	cleared := WithDB(ctx, nil)
	if got := Resolve(cleared, metaDB); got != metaDB {
		t.Error("nil override must clear the prior override, falling back to meta DB")
	}
}

// TestGet_NilContext ensures the helpers are nil-context safe so middleware
// and model code never panic on a missing context.
func TestGet_NilContext(t *testing.T) {
	if got := Get(nil); got != nil {
		t.Error("Get(nil) must return nil, not panic")
	}
	if got := Resolve(nil, nil); got != nil {
		t.Error("Resolve(nil, nil) must return nil, not panic")
	}
}
