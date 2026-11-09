package function

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/svc"
)

func functionCopy(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionCopyRequest) (*FunctionCopyResponse, error) {
	// Implementation would copy function to target env
	return &FunctionCopyResponse{
		FunctionId: req.ID,
		NewId:      "",
	}, nil
}

func functionDelete(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionDeleteRequest) error {
	return svcCtx.FunctionModel.DeleteFunction(ctx, req.FunctionId)
}

func functionDisable(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionDisableRequest) error {
	return setFunctionEnabled(ctx, svcCtx, req.FunctionId, 0)
}

func functionEnable(ctx context.Context, svcCtx *svc.ServiceContext, req *FunctionEnableRequest) error {
	return setFunctionEnabled(ctx, svcCtx, req.FunctionId, 1)
}

// setFunctionEnabled flips a function's enabled status by its external
// function_id. The previous implementation passed the numeric primary key 0
// (a placeholder that matches no row), so enable/disable silently did
// nothing while reporting success.
func setFunctionEnabled(ctx context.Context, svcCtx *svc.ServiceContext, functionID string, status int) error {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return errorx.NewBadRequest("functionId is required")
	}
	fn, err := svcCtx.FunctionModel.FindByFunctionID(ctx, functionID)
	if err != nil {
		return fmt.Errorf("find function %s: %w", functionID, err)
	}
	if err := svcCtx.FunctionModel.Update(ctx, fn.ID, map[string]interface{}{"status": status}); err != nil {
		return fmt.Errorf("update function %s status: %w", functionID, err)
	}
	return nil
}
