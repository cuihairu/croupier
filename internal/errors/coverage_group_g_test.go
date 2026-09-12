package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WithVersion 在尚未挂过 Metadata 的构建器上首次调用时自行创建 Metadata，
// 不应 panic 或丢失版本号（此前只覆盖了先 WithMetadata 再 WithVersion 的路径）。
func TestResponseBuilderWithVersionCreatesMetadataG(t *testing.T) {
	resp := NewResponseBuilder[string]().WithVersion("v1.2.3").Build()
	require.NotNil(t, resp.Metadata, "WithVersion must lazily create Metadata")
	assert.Equal(t, "v1.2.3", resp.Metadata.Version)
}

// 先 WithMetadata 再 WithVersion 的既有路径顺带复核：版本号覆盖、其余字段保留。
func TestResponseBuilderWithVersionKeepsExistingMetadataG(t *testing.T) {
	resp := NewResponseBuilder[string]().
		WithMetadata(&Metadata{RequestID: "req-1"}).
		WithVersion("v2.0.0").
		Build()
	require.NotNil(t, resp.Metadata)
	assert.Equal(t, "v2.0.0", resp.Metadata.Version)
	assert.Equal(t, "req-1", resp.Metadata.RequestID)
}
