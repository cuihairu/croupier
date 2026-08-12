package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	svc := NewService()
	assert.NotNil(t, svc)
}

func TestParse_Empty(t *testing.T) {
	svc := NewService()
	result, err := svc.Parse(nil)
	assert.NoError(t, err)
	assert.Empty(t, result)

	result, err = svc.Parse([]byte{})
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestParse_ValidJSON(t *testing.T) {
	svc := NewService()
	result, err := svc.Parse([]byte(`{"key": "value", "number": 42}`))
	require.NoError(t, err)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, float64(42), result["number"])
}

func TestParse_InvalidJSON(t *testing.T) {
	svc := NewService()
	_, err := svc.Parse([]byte(`{invalid}`))
	assert.Error(t, err)
}

func TestMustJSON_Valid(t *testing.T) {
	svc := NewService()
	result := svc.MustJSON(`{"key": "value"}`)
	assert.Equal(t, "value", result["key"])
}

func TestMustJSON_Invalid(t *testing.T) {
	svc := NewService()
	result := svc.MustJSON(`{invalid}`)
	assert.Empty(t, result)
}
