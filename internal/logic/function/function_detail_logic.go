
package function

import (
	"context"
	"errors"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"

	"gorm.io/gorm"
)

type FunctionDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取函数详情
func NewFunctionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionDetailLogic {
	return &FunctionDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionDetailLogic) FunctionDetail(req *FunctionDetailRequest) (*FunctionDetailResponse, error) {
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
				desc := FunctionDescriptor{}
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
				if op, opErr := store.GetOpenAPI(functionID); opErr == nil && op != nil {
					desc.Schema = extractOperationRequestSchema(op)
					if op.RequestBody != nil && op.RequestBody.Value != nil {
						desc.Input = op.RequestBody.Value
					}
					if op.Responses != nil {
						desc.Output = op.Responses
					}
				}
				return &FunctionDetailResponse{
					Function: Function{
						ID:        functionID,
						Name:      functionID,
						Category:  "",
						GameId:    gameID,
						Status:    1,
						Version:   version,
						Instances: instances,
					},
					Descriptor: desc,
				}, nil
			}
		}
		return nil, err
	}

	desc := FunctionDescriptor{}
	descs, err := l.svcCtx.FunctionModel.ListDescriptors(l.ctx, functionID)
	if err == nil && len(descs) > 0 {
		desc = FunctionDescriptor{
			Input:  descs[0].Input,
			Output: descs[0].Output,
			Schema: descs[0].Schema,
		}
	} else if l.svcCtx.RegistryStore != nil {
		// Fallback to runtime OpenAPI descriptor when DB descriptors are absent.
		if op, opErr := l.svcCtx.RegistryStore.GetOpenAPI(functionID); opErr == nil && op != nil {
			desc.Schema = extractOperationRequestSchema(op)
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				desc.Input = op.RequestBody.Value
			}
			if op.Responses != nil {
				desc.Output = op.Responses
			}
		}
	}

	return &FunctionDetailResponse{
		Function: Function{
			ID:          fn.FunctionID,
			Name:        fn.Name,
			Description: fn.Description,
			Category:    fn.Category,
			GameId:      fn.GameID,
			Status:      fn.Status,
			Version:     fn.Version,
			Instances:   fn.Instances,
			SpecFormat:  fn.SpecFormat,
			OpenAPISpec: fn.OpenAPISpec,
		},
		Descriptor: desc,
	}, nil
}
