package requestbind

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression (OPEN-ISSUES #5): on structs without form/json tags, gin's
// ShouldBindQuery matches by exact field name (case-sensitive), so frontend
// lowerCamelCase query keys were never bound successfully; and for
// string-only structs ShouldBindQuery always succeeds, so the old reflection
// fallback never ran — the resource-catalog category/query filters were
// therefore silently ineffective.

type untaggedDTO struct {
	GameID   string
	Env      string
	Category string
	Query    string
	ID       string
	Ignored  string `form:"-"`
}

func TestBindQueryCompat_UntaggedFieldsBindLowercaseNames(t *testing.T) {
	ctx := newBindContext(t, "gameId=demo&env=prod&category=inventory&query=sword&id=abc&ignored=nope")

	var req untaggedDTO
	require.NoError(t, BindQueryCompat(ctx, &req))

	assert.Equal(t, "demo", req.GameID, "lcFirst fallback: GameID→gameId")
	assert.Equal(t, "prod", req.Env)
	assert.Equal(t, "inventory", req.Category)
	assert.Equal(t, "sword", req.Query)
	assert.Equal(t, "abc", req.ID, "all-lower fallback: ID→id")
	assert.Empty(t, req.Ignored, `form:"-" must stay unbound`)
}

func TestBindQueryCompat_TaggedFieldsStillWinOverLowercaseFallback(t *testing.T) {
	type aliased struct {
		Category string `form:"cat"`
	}
	ctx := newBindContext(t, "cat=mail&category=player")

	var req aliased
	require.NoError(t, BindQueryCompat(ctx, &req))

	assert.Equal(t, "mail", req.Category, "form tag key takes precedence over field-name fallback")
}
