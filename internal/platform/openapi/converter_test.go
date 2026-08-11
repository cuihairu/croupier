package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProviderConverter(t *testing.T) {
	c := NewProviderConverter()
	assert.NotNil(t, c)
	assert.NotNil(t, c.validator)
}

func TestToOpenAPIOperation_NilDescriptor(t *testing.T) {
	c := NewProviderConverter()
	_, err := c.ToOpenAPIOperation(nil)
	assert.Error(t, err)
}

func TestToOpenAPIOperation_Basic(t *testing.T) {
	c := NewProviderConverter()
	desc := &FunctionDescriptor{
		ID:      "player.getList",
		Name:    "getPlayerList",
		Summary: "获取玩家列表",
	}
	op, err := c.ToOpenAPIOperation(desc)
	require.NoError(t, err)
	assert.Equal(t, "player.getList", op.OperationID)
	assert.Equal(t, "获取玩家列表", op.Summary)
	assert.NotNil(t, op.RequestBody)
	assert.NotNil(t, op.Responses)
}

func TestToOpenAPIOperation_WithExtensions(t *testing.T) {
	c := NewProviderConverter()
	desc := &FunctionDescriptor{
		ID:        "player.getCount",
		Summary:   "获取玩家数量",
		Resource:  "player",
		Risk:      "low",
		Operation: "count",
	}
	op, err := c.ToOpenAPIOperation(desc)
	require.NoError(t, err)
	assert.Equal(t, "player", op.Extensions["x-resource"])
	assert.Equal(t, "low", op.Extensions["x-risk"])
	assert.Equal(t, "count", op.Extensions["x-operation"])
}

func TestToOpenAPIOperation_WithoutExtensions(t *testing.T) {
	c := NewProviderConverter()
	desc := &FunctionDescriptor{
		ID:      "test.fn",
		Summary: "test",
	}
	op, err := c.ToOpenAPIOperation(desc)
	require.NoError(t, err)
	assert.Nil(t, op.Extensions["x-resource"])
	assert.Nil(t, op.Extensions["x-risk"])
	assert.Nil(t, op.Extensions["x-operation"])
}
