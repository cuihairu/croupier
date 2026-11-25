package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WithVersion on a builder that has not had Metadata attached yet must create the
// Metadata itself on first call, without panicking or losing the version
// (previously only the WithMetadata-then-WithVersion path was covered).
func TestResponseBuilderWithVersionCreatesMetadataG(t *testing.T) {
	resp := NewResponseBuilder[string]().WithVersion("v1.2.3").Build()
	require.NotNil(t, resp.Metadata, "WithVersion must lazily create Metadata")
	assert.Equal(t, "v1.2.3", resp.Metadata.Version)
}

// Also re-checks the existing WithMetadata-then-WithVersion path: the version
// number is overwritten while the remaining fields are preserved.
func TestResponseBuilderWithVersionKeepsExistingMetadataG(t *testing.T) {
	resp := NewResponseBuilder[string]().
		WithMetadata(&Metadata{RequestID: "req-1"}).
		WithVersion("v2.0.0").
		Build()
	require.NotNil(t, resp.Metadata)
	assert.Equal(t, "v2.0.0", resp.Metadata.Version)
	assert.Equal(t, "req-1", resp.Metadata.RequestID)
}
