package normalizer

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
)

// ResolveConflict：来源未参与冲突 → false；字段不存在冲突 → false。
func TestResolveConflictEdgeCases(t *testing.T) {
	tracker := NewSemanticProvenanceTracker()
	tracker.TrackString("identityField", "player_id", spec.SemanticSourceSDKExplicit, "d1", "user1")
	tracker.TrackString("identityField", "id", spec.SemanticSourceSDKExplicit, "d2", "user2")

	// 该来源没有冲突值
	assert.False(t, tracker.ResolveConflict("identityField", spec.SemanticSourcePlatformReview, "admin"))
	// 冲突列表里没有该字段
	assert.False(t, tracker.ResolveConflict("no-such-field", spec.SemanticSourceSDKExplicit, "admin"))
}
