// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package function

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type FunctionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数详情
func NewFunctionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionDetailLogic {
	return &FunctionDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionDetailLogic) FunctionDetail(req *types.FunctionDetailRequest) (*types.FunctionDetailResponse, error) {
	functionID, err := utils.ValidateFunctionID(req.ID)
	if err != nil {
		return nil, err
	}

	fn, err := l.svcCtx.FunctionModel.FindByFunctionID(l.ctx, functionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Fallback to runtime registry when DB record is missing.
			if store := l.svcCtx.RegistryStore; store != nil {
				store.Mu().RLock()
				defer store.Mu().RUnlock()
				var version string
				var gameID string
				instances := 0
				for _, sess := range store.AgentsUnsafe() {
					if sess == nil {
						continue
					}
					if meta, ok := sess.Functions[functionID]; ok {
						instances++
						if version == "" && meta.Version != "" {
							version = meta.Version
						}
						if gameID == "" && sess.GameID != "" {
							gameID = sess.GameID
						}
					}
				}
				return &types.FunctionDetailResponse{
					Function: types.Function{
						Id:        functionID,
						Name:      functionID,
						Category:  "",
						GameId:    gameID,
						Status:    1,
						Version:   version,
						Instances: instances,
					},
					Descriptor: types.FunctionDescriptor{},
				}, nil
			}
		}
		return nil, err
	}

	desc := types.FunctionDescriptor{}
	descs, err := l.svcCtx.FunctionModel.ListDescriptors(l.ctx, functionID)
	if err == nil && len(descs) > 0 {
		desc = types.FunctionDescriptor{
			Input:  descs[0].Input,
			Output: descs[0].Output,
			Schema: descs[0].Schema,
		}
	}

	return &types.FunctionDetailResponse{
		Function:   utils.BuildFunctionDTO(fn),
		Descriptor: desc,
	}, nil
}
