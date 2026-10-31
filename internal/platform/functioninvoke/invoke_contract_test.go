package functioninvoke

import (
	"context"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newContractSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FunctionDescriptor{}))
	return &svc.ServiceContext{DB: db, FunctionModel: model.NewFunctionModel(db)}
}

func TestValidateInvokeInput_DescriptorQueryFailsFailClosed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/bad.db"), &gorm.Config{})
	require.NoError(t, err)
	// Table intentionally not migrated → ListDescriptors fails.
	svcCtx := &svc.ServiceContext{DB: db, FunctionModel: model.NewFunctionModel(db)}
	err = ValidateInvokeInput(context.Background(), svcCtx, &Request{ID: "demo.high"}, []byte(`{}`))
	require.Error(t, err, "descriptor lookup failure must not be treated as no-schema pass-through")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusInternalServerError, codeErr.Code)
	assert.Contains(t, codeErr.Message, "输入契约读取失败")
}

func TestValidateInvokeInput_LegacyNoDescriptorPassthrough(t *testing.T) {
	svcCtx := newContractSvcCtx(t)
	// Registered functions with no descriptor row: legacy pass-through.
	require.NoError(t, ValidateInvokeInput(context.Background(), svcCtx, &Request{ID: "demo.legacy"}, []byte(`{}`)))
}

func TestValidateInvokeInput_RequiredMissingRejected(t *testing.T) {
	svcCtx := newContractSvcCtx(t)
	require.NoError(t, svcCtx.FunctionModel.UpsertDescriptor(context.Background(), &model.FunctionDescriptor{
		FunctionID: "demo.grant",
		Version:    "v1",
		Input: datatypes.JSONMap{
			"type":     "object",
			"required": []interface{}{"playerId"},
			"properties": map[string]interface{}{
				"playerId": map[string]interface{}{"type": "string"},
			},
		},
	}))
	err := ValidateInvokeInput(context.Background(), svcCtx, &Request{ID: "demo.grant"}, []byte(`{}`))
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Equal(t, "required", codeErr.Details["playerId"])
	require.NoError(t, ValidateInvokeInput(context.Background(), svcCtx, &Request{ID: "demo.grant"}, []byte(`{"playerId":"p1"}`)))
}

func TestValidateInvokeInput_LatestVersionWins(t *testing.T) {
	svcCtx := newContractSvcCtx(t)
	require.NoError(t, svcCtx.FunctionModel.UpsertDescriptor(context.Background(), &model.FunctionDescriptor{
		FunctionID: "demo.drift",
		Version:    "v1",
		Input:      datatypes.JSONMap{"type": "object"},
	}))
	// Upsert with different Version creates a second row (latest wins).
	require.NoError(t, svcCtx.FunctionModel.UpsertDescriptor(context.Background(), &model.FunctionDescriptor{
		FunctionID: "demo.drift",
		Version:    "v2",
		Input: datatypes.JSONMap{
			"type":     "object",
			"required": []interface{}{"amount"},
		},
	}))
	// v2 requires amount; v1 did not. Missing amount must reject (v2 wins).
	err := ValidateInvokeInput(context.Background(), svcCtx, &Request{ID: "demo.drift"}, []byte(`{}`))
	require.Error(t, err, "version drift must be caught by latest contract")
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusBadRequest, codeErr.Code)
	assert.Equal(t, "required", codeErr.Details["amount"])
}
