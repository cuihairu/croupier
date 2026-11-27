package openapi

// GAME-009 D3：x-durable-idempotency 能力声明在驱动三面的判定——发现面
// （spec→APIMethod 投影）、转换面（descriptor→operation 扩展）与校验面
// （声明形态 fail-fast）。运行时消费面在 internal/platform/functioninvoke。

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/platform/provider"
)

const durableSpecJSON = `{
	"openapi": "3.0.3",
	"paths": {
		"/api/grant": {
			"post": {
				"operationId": "grantOp",
				"x-risk": "high",
				"x-durable-idempotency": {
					"query": "grant.query",
					"cancel": "grant.cancel"
				}
			}
		},
		"/api/plain": {
			"post": {
				"operationId": "plainOp"
			}
		}
	}
}`

func TestDiscoverProjectsDurableIdempotency(t *testing.T) {
	tmpFile := t.TempDir() + "/spec.json"
	require.NoError(t, os.WriteFile(tmpFile, []byte(durableSpecJSON), 0644))

	p := NewProvider()
	require.NoError(t, p.Init(context.Background(), provider.ProviderConfig{
		Enabled: true,
		Type:    "openapi",
		Config: map[string]interface{}{
			"baseUrl":     "http://example.com",
			"openapiSpec": tmpFile,
		},
	}))

	grant := p.methodMap["grantOp"]
	require.NotNil(t, grant)
	assert.True(t, grant.DurableIdempotency.Declared())
	assert.Equal(t, "grant.query", grant.DurableIdempotency.Query)
	assert.Equal(t, "grant.cancel", grant.DurableIdempotency.Cancel)
	assert.Empty(t, grant.DurableIdempotency.Undo)

	plain := p.methodMap["plainOp"]
	require.NotNil(t, plain)
	assert.False(t, plain.DurableIdempotency.Declared(), "未声明扩展不得被当作已声明")
}

func TestConverterPropagatesDurableIdempotency(t *testing.T) {
	converter := NewProviderConverter()
	op, err := converter.ToOpenAPIOperation(&FunctionDescriptor{
		ID: "grantOp",
		DurableIdempotency: &DurableIdempotencyCapability{
			Query: "grant.query", Undo: "grant.undo",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, op)
	ext, ok := op.Extensions["x-durable-idempotency"].(map[string]interface{})
	require.True(t, ok, "声明必须以扩展对象写回 spec 面")
	assert.Equal(t, "grant.query", ext["query"])
	assert.Equal(t, "grant.undo", ext["undo"])

	// 未声明不写空扩展。
	plain, err := converter.ToOpenAPIOperation(&FunctionDescriptor{ID: "plainOp"})
	require.NoError(t, err)
	_, exists := plain.Extensions["x-durable-idempotency"]
	assert.False(t, exists, "未声明不得写出空壳扩展（空壳会被校验面拒绝）")
}

func TestValidatorRejectsMalformedDurableIdempotency(t *testing.T) {
	v := NewValidator()

	require.NoError(t, v.ValidateExtensionFields(map[string]interface{}{
		"x-durable-idempotency": map[string]interface{}{"query": "q"},
	}))

	assert.Error(t, v.ValidateExtensionFields(map[string]interface{}{
		"x-durable-idempotency": "query",
	}), "非对象形态必须拒绝")
	assert.Error(t, v.ValidateExtensionFields(map[string]interface{}{
		"x-durable-idempotency": map[string]interface{}{},
	}), "全空声明必须拒绝（空壳会伪装成已声明）")
	assert.Error(t, v.ValidateExtensionFields(map[string]interface{}{
		"x-durable-idempotency": map[string]interface{}{"query": 1},
	}), "非字符串字段必须拒绝")
}
