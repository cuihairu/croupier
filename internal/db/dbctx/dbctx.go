// Package dbctx carries a per-request *gorm.DB override in context, so that
// game-scoped models can route to the correct per-game database without
// changing their method signatures.
//
// In the database-per-game architecture, a middleware resolves the game
// database from request scope (game_id + env) and stores it in the request
// context via WithDB. Game-scoped models then call Resolve(ctx, fallback) to
// pick up the override, falling back to the meta DB when no override is set.
package dbctx

import (
	"context"

	"gorm.io/gorm"
)

type ctxKey struct{}

// WithDB stores a *gorm.DB override in ctx. Passing nil clears any prior
// override (subsequent Resolve calls return the fallback).
func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKey{}, db)
}

// Get returns the *gorm.DB override stored in ctx, or nil if none is set.
func Get(ctx context.Context) *gorm.DB {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(ctxKey{}).(*gorm.DB); ok {
		return v
	}
	return nil
}

// Resolve returns the per-request DB override when present, otherwise the
// fallback. Game-scoped models should call this at the start of each method:
//
//	db := dbctx.Resolve(ctx, m.db)
func Resolve(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if db := Get(ctx); db != nil {
		return db
	}
	return fallback
}
