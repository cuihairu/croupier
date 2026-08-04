package normalizer

import (
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
)

func TestSemanticProvenanceTracker_TrackField(t *testing.T) {
	tracker := NewSemanticProvenanceTracker()

	// Test tracking a new field
	accepted := tracker.TrackField(
		"identityField",
		"player_id",
		spec.SemanticSourceSDKExplicit,
		"digest1",
		"test-user",
	)
	assert.True(t, accepted)

	provenance := tracker.GetProvenance()
	assert.Len(t, provenance, 1)
	assert.Equal(t, "\"player_id\"", string(provenance["identityField"].Value))
	assert.Equal(t, spec.SemanticSourceSDKExplicit, provenance["identityField"].Source)
	assert.Equal(t, "effective", provenance["identityField"].Status)
	assert.Equal(t, "high", provenance["identityField"].Confidence)

	// Test tracking same field with same value from different source
	accepted = tracker.TrackField(
		"identityField",
		"player_id",
		spec.SemanticSourceOpenAPIRest,
		"digest2",
		"test-user",
	)
	assert.True(t, accepted) // Same value, accepted

	// Test tracking same field with different value from lower priority source
	accepted = tracker.TrackField(
		"identityField",
		"id",
		spec.SemanticSourceOpenAPIRest,
		"digest3",
		"test-user",
	)
	assert.False(t, accepted) // Lower priority, rejected

	assert.True(t, tracker.HasConflicts())
	conflicts := tracker.GetConflicts()
	assert.Len(t, conflicts, 1)
	assert.Equal(t, "identityField", conflicts[0].Field)
}

func TestSemanticProvenanceTracker_PriorityOverride(t *testing.T) {
	tracker := NewSemanticProvenanceTracker()

	// Track with low priority first
	tracker.TrackField(
		"identityField",
		"id",
		spec.SemanticSourceOpenAPIRest,
		"digest1",
		"test-user",
	)

	// Track with higher priority - should override
	accepted := tracker.TrackField(
		"identityField",
		"player_id",
		spec.SemanticSourceSDKExplicit,
		"digest2",
		"test-user",
	)
	assert.True(t, accepted)

	provenance := tracker.GetProvenance()
	assert.Equal(t, "\"player_id\"", string(provenance["identityField"].Value))
	assert.Equal(t, spec.SemanticSourceSDKExplicit, provenance["identityField"].Source)
	assert.Equal(t, "\"id\"", string(provenance["identityField"].OverriddenValue))
	assert.False(t, tracker.HasConflicts())
}

func TestSemanticProvenanceTracker_ResolveConflict(t *testing.T) {
	tracker := NewSemanticProvenanceTracker()

	// Create a conflict with same priority sources
	tracker.TrackField("identityField", "player_id", spec.SemanticSourceSDKExplicit, "d1", "user1")
	tracker.TrackField("identityField", "id", spec.SemanticSourceSDKExplicit, "d2", "user2")

	assert.True(t, tracker.HasConflicts())

	// Resolve conflict by choosing one of the conflicting sources
	resolved := tracker.ResolveConflict("identityField", spec.SemanticSourceSDKExplicit, "admin")
	assert.True(t, resolved)

	provenance := tracker.GetProvenance()
	// The value should be the last one tracked (since ResolveConflict picks from conflict.Values)
	assert.NotNil(t, provenance["identityField"])
	assert.Equal(t, spec.SemanticSourceSDKExplicit, provenance["identityField"].Source)
	assert.Equal(t, "effective", provenance["identityField"].Status)
	assert.Equal(t, "admin", provenance["identityField"].UpdatedBy)

	// Check conflict is resolved
	conflicts := tracker.GetConflicts()
	assert.Len(t, conflicts, 1)
	assert.Equal(t, spec.SemanticSourceSDKExplicit, conflicts[0].Resolution)
	assert.NotEmpty(t, conflicts[0].ResolvedAt)
	assert.Equal(t, "admin", conflicts[0].ResolvedBy)
}

func TestSourcePriority(t *testing.T) {
	assert.Equal(t, 1, SourcePriority[spec.SemanticSourceOpenAPIRest])
	assert.Equal(t, 2, SourcePriority[spec.SemanticSourceSDKExplicit])
	assert.Equal(t, 3, SourcePriority[spec.SemanticSourcePlatformReview])
}
