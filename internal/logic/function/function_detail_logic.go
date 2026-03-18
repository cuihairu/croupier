package function

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/registry"
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
			rt := loadRuntimeFunctionDetail(l.svcCtx.RegistryStore, functionID)
			if rt != nil {
				return &FunctionDetailResponse{
					Function: Function{
						ID:        functionID,
						Name:      functionID,
						Category:  "",
						GameId:    rt.gameID,
						Status:    1,
						Version:   rt.version,
						Instances: rt.instances,
					},
					Descriptor: rt.descriptor,
				}, nil
			}
		}
		return nil, err
	}

	rt := loadRuntimeFunctionDetail(l.svcCtx.RegistryStore, functionID)
	desc := FunctionDescriptor{}
	descs, err := l.svcCtx.FunctionModel.ListDescriptors(l.ctx, functionID)
	if err == nil && len(descs) > 0 {
		desc = FunctionDescriptor{
			Input:  descs[0].Input,
			Output: descs[0].Output,
			Schema: descs[0].Schema,
		}
	} else if rt != nil {
		desc = rt.descriptor
	}

	name := strings.TrimSpace(fn.Name)
	if name == "" {
		name = functionID
	}
	gameID := strings.TrimSpace(fn.GameID)
	if gameID == "" && rt != nil {
		gameID = rt.gameID
	}
	version := strings.TrimSpace(fn.Version)
	if version == "" && rt != nil {
		version = rt.version
	}
	instances := fn.Instances
	if instances == 0 && rt != nil {
		instances = rt.instances
	}

	return &FunctionDetailResponse{
		Function: Function{
			ID:          fn.FunctionID,
			Name:        name,
			Description: fn.Description,
			Category:    fn.Category,
			GameId:      gameID,
			Status:      fn.Status,
			Version:     version,
			Instances:   instances,
			SpecFormat:  fn.SpecFormat,
			OpenAPISpec: fn.OpenAPISpec,
		},
		Descriptor: desc,
	}, nil
}

type runtimeFunctionDetail struct {
	version    string
	gameID     string
	instances  int
	descriptor FunctionDescriptor
}

func loadRuntimeFunctionDetail(store *registry.Store, functionID string) *runtimeFunctionDetail {
	if store == nil {
		return nil
	}

	store.Mu().RLock()
	defer store.Mu().RUnlock()

	out := &runtimeFunctionDetail{}
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil {
			continue
		}
		if meta, ok := sess.Functions[functionID]; ok {
			out.instances++
			if out.version == "" && meta.Version != "" {
				out.version = meta.Version
			}
			if out.gameID == "" && sess.GameID != "" {
				out.gameID = sess.GameID
			}
		}
	}
	if op, err := store.GetOpenAPI(functionID); err == nil && op != nil {
		out.descriptor.Schema = extractOperationRequestSchema(op)
		if op.RequestBody != nil && op.RequestBody.Value != nil {
			out.descriptor.Input = op.RequestBody.Value
		}
		if op.Responses != nil {
			out.descriptor.Output = op.Responses
		}
	} else if out.instances > 0 {
		if op := BuildFallbackOpenAPIOperation(functionID); op != nil {
			out.descriptor.Schema = extractOperationRequestSchema(op)
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				out.descriptor.Input = op.RequestBody.Value
			}
			if op.Responses != nil {
				out.descriptor.Output = op.Responses
			}
		}
	}
	if out.instances == 0 && out.version == "" && out.gameID == "" &&
		out.descriptor.Input == nil && out.descriptor.Output == nil && out.descriptor.Schema == nil {
		return nil
	}
	return out
}
