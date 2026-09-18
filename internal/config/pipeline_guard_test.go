package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// M6 大文档护栏阈值解析：0=默认 500、正数=显式值、负数=禁用。
func TestPipelineOperationGuardThreshold(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 500, DefaultPipelineOperationGuard)

	threshold, enabled := (OpenAPIConfig{}).PipelineOperationGuardThreshold()
	assert.True(t, enabled)
	assert.Equal(t, DefaultPipelineOperationGuard, threshold)

	threshold, enabled = (OpenAPIConfig{PipelineOperationGuard: 120}).PipelineOperationGuardThreshold()
	assert.True(t, enabled)
	assert.Equal(t, 120, threshold)

	threshold, enabled = (OpenAPIConfig{PipelineOperationGuard: -1}).PipelineOperationGuardThreshold()
	assert.False(t, enabled)
	assert.Zero(t, threshold)
}

// M6 配置契约：openapi.pipelineOperationGuard 必须以 lowerCamelCase 键
// 解析（配置命名契约，禁止大写/蛇形漂移）。
func TestOpenAPIConfigYAMLKeys(t *testing.T) {
	t.Parallel()

	var c Config
	require.NoError(t, yaml.Unmarshal([]byte("openapi:\n  pipelineOperationGuard: 120\n"), &c))
	assert.Equal(t, 120, c.OpenAPI.PipelineOperationGuard)
}
