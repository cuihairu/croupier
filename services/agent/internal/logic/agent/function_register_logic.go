// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	localv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/agent/local/v1"
	"github.com/cuihairu/croupier/services/agent/internal/svc"
	"github.com/cuihairu/croupier/services/agent/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FunctionRegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFunctionRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FunctionRegisterLogic {
	return &FunctionRegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FunctionRegisterLogic) FunctionRegister(req *types.FunctionRegisterRequest) (*types.FunctionRegisterResponse, error) {
	if req == nil {
		return nil, errors.New("missing request payload")
	}
	functionID := strings.TrimSpace(req.FunctionId)
	if functionID == "" {
		return nil, errors.New("function_id 不能为空")
	}

	if l.svcCtx.Core != nil && l.svcCtx.Core.Store() != nil {
		rpcAddr := strings.TrimSpace(anyString(req.Metadata["rpc_addr"]))
		if rpcAddr != "" {
			serviceID := strings.TrimSpace(anyString(req.Metadata["service_id"]))
			if serviceID == "" {
				serviceID = "http-" + sanitizeServiceID(functionID)
			}
			serviceVer := strings.TrimSpace(anyString(req.Metadata["service_version"]))
			funcVer := strings.TrimSpace(anyString(req.Metadata["function_version"]))
			if funcVer == "" {
				funcVer = strings.TrimSpace(anyString(req.Descriptor["version"]))
			}

			l.svcCtx.Core.Store().Register(serviceID, rpcAddr, serviceVer, []*localv1.LocalFunctionDescriptor{
				{Id: functionID, Version: funcVer},
			})
			return &types.FunctionRegisterResponse{Success: true, Message: "function registered (core store updated)"}, nil
		}
	}

	return &types.FunctionRegisterResponse{
		Success: true,
		Message: "function registered (no rpc_addr provided; core store unchanged)",
	}, nil
}

func anyString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return ""
	}
}

func sanitizeServiceID(s string) string {
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
