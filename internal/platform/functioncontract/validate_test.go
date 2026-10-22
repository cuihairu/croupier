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

func TestValidateInputUnavailableOrMalformedContractPassesThrough(t *testing.T) {
	require.NoError(t, ValidateInput(nil, []byte(`invalid`)))
	require.NoError(t, ValidateInput(nil, []byte(`[1]`)))
	require.NoError(t, ValidateInput(map[string]interface{}{}, []byte(`invalid`)))
	require.NoError(t, ValidateInput(map[string]interface{}{"type": 42}, []byte(`{}`)))
}
