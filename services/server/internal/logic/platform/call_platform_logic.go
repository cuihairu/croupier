// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package platform

import (
	"context"
	"encoding/json"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type CallPlatformLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 调用第三方平台 API
func NewCallPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CallPlatformLogic {
	return &CallPlatformLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CallPlatformLogic) CallPlatform(req *types.CallPlatformRequest) (resp *types.CallPlatformResponse, err error) {
	// 检查平台加载器是否初始化
	if l.svcCtx.PlatformLoader == nil {
		return &types.CallPlatformResponse{
			Code:    503,
			Message: "Platform integration is not enabled",
		}, nil
	}

	// 验证必填参数
	if req.Platform == "" {
		return &types.CallPlatformResponse{
			Code:    400,
			Message: "platform is required",
		}, nil
	}
	if req.Method == "" {
		return &types.CallPlatformResponse{
			Code:    400,
			Message: "method is required",
		}, nil
	}

	// 将 JSON 字符串请求转换为 []byte
	var requestData []byte
	if req.Request != "" {
		requestData = []byte(req.Request)
	}

	// 通过注册表调用平台 API
	response, err := l.svcCtx.PlatformLoader.Registry().Call(l.ctx, req.Platform, req.Method, requestData)
	if err != nil {
		// 转换错误为响应
		return &types.CallPlatformResponse{
			Code:    500,
			Message: err.Error(),
		}, nil
	}

	// 解析响应以返回结构化数据
	var result interface{}
	if len(response) > 0 {
		_ = json.Unmarshal(response, &result)
	}

	return &types.CallPlatformResponse{
		Code:     200,
		Message:  "success",
		Response: result,
	}, nil
}
