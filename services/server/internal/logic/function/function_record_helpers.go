package function

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"gorm.io/gorm"
)

func getOrCreateFunctionRecord(ctx context.Context, svcCtx *svc.ServiceContext, functionID string) (*model.Function, error) {
	fn, err := svcCtx.FunctionModel.FindByFunctionID(ctx, functionID)
	if err == nil {
		return fn, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	created := &model.Function{
		FunctionID: functionID,
		Name:       functionID,
		Status:     1,
	}
	if createErr := svcCtx.FunctionModel.Create(ctx, created); createErr != nil {
		if errors.Is(createErr, gorm.ErrDuplicatedKey) {
			return svcCtx.FunctionModel.FindByFunctionID(ctx, functionID)
		}
		return nil, createErr
	}
	return created, nil
}
