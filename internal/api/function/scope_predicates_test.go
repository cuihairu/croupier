package function

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cuihairu/croupier/internal/svc"
)

// Session scope predicate: an empty scope.GameID matches everything (backwards
// compatible with callers that do not inject a scope); a non-empty scope
// requires both game and env to match.

func TestSessionMatchesScope(t *testing.T) {
	// Empty scope -> always true (callers that do not inject a scope keep the old behavior).
	assert.True(t, sessionMatchesScope(svc.GameScope{}, "any", "any"))
	assert.True(t, sessionMatchesScope(svc.GameScope{Env: "prod"}, "g1", "dev"),
		"仅 Env 无 GameID 时按空 GameID 分支放行")

	// Non-empty scope: both game and env must match (case and whitespace insensitive).
	scope := svc.GameScope{GameID: "g1", Env: "prod"}
	assert.True(t, sessionMatchesScope(scope, "g1", "prod"))
	assert.True(t, sessionMatchesScope(scope, " G1 ", " PROD "))

	// Cross-game / cross-env / only game correct / only env correct -> rejected.
	assert.False(t, sessionMatchesScope(scope, "g2", "prod"))
	assert.False(t, sessionMatchesScope(scope, "g1", "dev"))
	assert.False(t, sessionMatchesScope(scope, "", "prod"))
	assert.False(t, sessionMatchesScope(scope, "g1", ""))
	assert.False(t, sessionMatchesScope(scope, "", ""))
}
