package function

import (
	"context"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// validateInvokeInput validates the payload against the descriptor
// inputSchema before dispatch; client-side contract errors must return 400
// before the request is routed to an agent.

func upsertInputContract(t *testing.T, f *invokeFixture, functionID string, input datatypes.JSONMap) {
	t.Helper()
	require.NoError(t, f.svcCtx.FunctionModel.UpsertDescriptor(context.Background(), &model.FunctionDescriptor{
		FunctionID: functionID,
		Version:    "v1",
		Input:      input,
	}))
}

func TestValidateInvokeInput_MissingRequired(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"type":     "object",
		"required": []interface{}{"playerId", "amount"},
		"properties": map[string]interface{}{
			"playerId": map[string]interface{}{"type": "string"},
			"amount":   map[string]interface{}{"type": "integer"},
		},
	})

	err := validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{"playerId":"p-1"}`))
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expected *errorx.CodeError, got %T", err)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Contains(t, codeErr.Message, "amount")
	assert.NotContains(t, codeErr.Message, "playerId")
	assert.Equal(t, "required", codeErr.Details["amount"])
	assert.NotContains(t, codeErr.Details, "playerId")
}

func TestValidateInvokeInput_ProvidedPasses(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"type":     "object",
		"required": []interface{}{"playerId"},
	})

	// Empty strings/false are valid JSON Schema values (required only constrains presence).
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{"playerId":""}`)))
	// Explicit null only fails when the type disallows null; required only
	// constrains property presence.
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{"playerId":null}`)))
	// Empty object payload.
	require.Error(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{}`)))
}

func TestValidateInvokeInput_TypeAndNestedSchema(t *testing.T) {
	f := newInvokeFixture(t)
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"type":     "object",
		"required": []interface{}{"profile"},
		"properties": map[string]interface{}{
			"amount": map[string]interface{}{"type": "integer"},
			"profile": map[string]interface{}{
				"type":     "object",
				"required": []interface{}{"level"},
				"properties": map[string]interface{}{
					"level": map[string]interface{}{"type": "string"},
				},
			},
		},
	})

	err := validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{"amount":"1","profile":{"level":2}}`))
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Contains(t, codeErr.Details, "payload.amount")
	assert.Contains(t, codeErr.Details, "payload.profile.level")
}

func TestValidateInvokeInput_NoContractPassthrough(t *testing.T) {
	f := newInvokeFixture(t)
	// No descriptor: pass through (the game server is the fallback for the contract).
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.bare"}, []byte(`{}`)))
	// Descriptor with Input but no required: pass through.
	upsertInputContract(t, f, "demo.noreq", datatypes.JSONMap{
		"type": "object",
		"properties": map[string]interface{}{
			"x": map[string]interface{}{"type": "string"},
		},
	})
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.noreq"}, []byte(`{}`)))
	// A non-object payload (array) is not intercepted at this layer.
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"required": []interface{}{"playerId"},
	})
	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`[1,2]`)))
	// FunctionModel not initialized: skip.
	require.NoError(t, validateInvokeInput(context.Background(), nil,
		&FunctionInvokeRequest{ID: "demo.grant"}, []byte(`{}`)))
}

// Missing parameters must be rejected before dispatch: the agent must not
// receive the request even when online (otherwise the user sees a routing
// error such as no live agent instead of which parameter is missing).
func TestFunctionInvoke_MissingRequiredRejectedBeforeDispatch(t *testing.T) {
	f := newInvokeFixture(t)
	f.createOperator(t, "opuser", "admin")
	f.registerAgent(t, "agent-1", "demo.grant")
	f.caller = &fakeSessionCaller{invokePayload: []byte(`{}`)}
	f.resolver.callers["agent-1"] = f.caller
	upsertInputContract(t, f, "demo.grant", datatypes.JSONMap{
		"type":     "object",
		"required": []interface{}{"playerId", "amount"},
	})

	_, err := NewService(f.svcCtx).FunctionInvoke(f.ctxFor("opuser"), &FunctionInvokeRequest{
		ID:      "demo.grant",
		Payload: []byte(`{}`),
	})
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok, "expected *errorx.CodeError, got %T", err)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Contains(t, codeErr.Message, "playerId")
	assert.Contains(t, codeErr.Message, "amount")
	assert.Empty(t, f.caller.requests)
}
