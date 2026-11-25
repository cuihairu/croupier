package functioncontract

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/common/errorx"
)

func TestValidateInputRequiredAndNull(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"playerId": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"playerId"},
	}

	require.NoError(t, ValidateInput(schema, []byte(`{"playerId":""}`)))
	other := map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"other"},
	}
	require.NoError(t, ValidateInput(other, []byte(`{"other":null}`)))

	err := ValidateInput(schema, []byte(`{}`))
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Equal(t, "required", codeErr.Details["playerId"])
}

func TestValidateInputTypesAndNestedSchemas(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"amount": map[string]interface{}{"type": "integer"},
			"profile": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"level": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"level"},
			},
		},
		"required": []interface{}{"profile"},
	}

	err := ValidateInput(schema, []byte(`{"amount":"1","profile":{"level":2}}`))
	require.Error(t, err)
	codeErr := err.(*errorx.CodeError)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Contains(t, codeErr.Details, "payload.amount")
	assert.Contains(t, codeErr.Details, "payload.profile.level")

	require.NoError(t, ValidateInput(schema, []byte(`{"amount":1,"profile":{"level":"gold"}}`)))
	require.Error(t, ValidateInput(schema, []byte(`{"amount":1,"profile":{}}`)))
}

// SUI-016: a missing schema stays pass-through, but a schema that cannot
// compile and a payload that is not valid JSON are defects, not absences —
// they must be rejected instead of silently degrading a declared contract to
// "no schema". The distinction is what keeps the direct-invoke entry aligned
// with the console execution entry.
func TestValidateInputMissingSchemaPassesDefectsReject(t *testing.T) {
	// missing schema: pass regardless of payload
	require.NoError(t, ValidateInput(nil, []byte(`invalid`)))
	require.NoError(t, ValidateInput(nil, []byte(`[1]`)))
	require.NoError(t, ValidateInput(map[string]interface{}{}, []byte(`invalid`)))

	// schema compile failure: reject
	err := ValidateInput(map[string]interface{}{"type": 42}, []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON Schema")

	// invalid payload JSON: reject
	err = ValidateInput(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"playerId": map[string]interface{}{"type": "string"},
		},
	}, []byte(`{broken`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

// SUI-016: unknown fields follow standard JSON Schema semantics — accepted
// unless the schema explicitly declares additionalProperties:false. Valid
// dictionary data (an object without properties, or additionalProperties
// typed values) must pass on this entry exactly as on the console entry.
func TestValidateInputUnknownFieldsAndDictionaryData(t *testing.T) {
	// omitted additionalProperties: unknown fields accepted
	omitted := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"playerId": map[string]interface{}{"type": "string"},
		},
	}
	require.NoError(t, ValidateInput(omitted, []byte(`{"playerId":"p-1","extra":"context"}`)))

	// explicit additionalProperties:false: unknown fields rejected
	strict := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"playerId": map[string]interface{}{"type": "string"},
		},
	}
	err := ValidateInput(strict, []byte(`{"playerId":"p-1","extra":"context"}`))
	require.Error(t, err)
	codeErr := err.(*errorx.CodeError)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Contains(t, codeErr.Message, "extra")

	// dictionary without properties: any keys accepted
	dictionary := map[string]interface{}{"type": "object"}
	require.NoError(t, ValidateInput(dictionary, []byte(`{"anything":{"nested":[1,2],"k":"v"}}`)))

	// dictionary with typed additionalProperties: valid values pass, bad fail
	typed := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": map[string]interface{}{"type": "string"},
	}
	require.NoError(t, ValidateInput(typed, []byte(`{"a":"x","b":"y"}`)))
	require.Error(t, ValidateInput(typed, []byte(`{"a":1}`)))
}
