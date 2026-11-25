package schema

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An invalid schema (type set to a NaN number) must be rejected by the schema
// definition validation.
func TestValidateSchemaDefinition_RejectsInvalidKeywordValue(t *testing.T) {
	schema := map[string]interface{}{"type": math.NaN()}

	err := validateSchemaDefinition(schema)

	require.Error(t, err)
}

// The same invalid schema must also return an error, rather than panic, from the
// payload validation helper.
func TestValidatePayloadAgainst_RejectsInvalidKeywordValue(t *testing.T) {
	schema := map[string]interface{}{"type": math.NaN()}

	valid, issues, err := validatePayloadAgainst(schema, map[string]interface{}{"a": 1})

	require.Error(t, err)
	assert.False(t, valid)
	assert.Nil(t, issues)
}
