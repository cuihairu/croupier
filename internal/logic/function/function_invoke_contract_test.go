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

func TestFunctionInvokeLogic_SharedInputContract(t *testing.T) {
	f := newInvokeLogicFixture(t)
	f.caller.invokePayload = []byte(`{"ok":true}`)
	require.NoError(t, f.svcCtx.FunctionModel.UpsertDescriptor(context.Background(), &model.FunctionDescriptor{
		FunctionID: "player.ban",
		Version:    "v1",
		Input: datatypes.JSONMap{
			"type":     "object",
			"required": []interface{}{"playerId"},
			"properties": map[string]interface{}{
				"playerId": map[string]interface{}{"type": "string"},
			},
		},
	}))
	logic := NewFunctionInvokeLogic(f.ctxUser, f.svcCtx)

	err := validateInvokeInput(context.Background(), f.svcCtx, "player.ban", []byte(`{}`))
	require.Error(t, err)
	codeErr, ok := err.(*errorx.CodeError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Equal(t, "required", codeErr.Details["playerId"])

	require.NoError(t, validateInvokeInput(context.Background(), f.svcCtx, "player.ban", []byte(`{"playerId":"p-1"}`)))

	resp, err := logic.FunctionInvoke(&FunctionInvokeRequest{
		ID:      "player.ban",
		Payload: []byte(`{"playerId":"p-1"}`),
		Route:   "broadcast",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Broadcast.Success)
}
