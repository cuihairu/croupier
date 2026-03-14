// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
)

type CreateDirectoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建目录
func NewCreateDirectoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateDirectoryLogic {
	return &CreateDirectoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateDirectoryLogic) CreateDirectory(req *types.CreateDirectoryRequest) (resp *types.CreateDirectoryResponse, err error) {
	// 检查对象存储是否初始化
	if l.svcCtx.ObjectStore == nil {
		return &types.CreateDirectoryResponse{
			Code:    -1,
			Message: "对象存储未配置",
		}, nil
	}

	// 验证请求参数
	if req.Prefix == "" {
		return &types.CreateDirectoryResponse{
			Code:    -1,
			Message: "目录前缀不能为空",
		}, nil
	}

	// 创建目录
	err = l.svcCtx.ObjectStore.CreatePrefix(l.ctx, req.Prefix)
	if err != nil {
		slog.ErrorContext(l.ctx, "创建目录失败", "prefix", req.Prefix, "error", err)
		return &types.CreateDirectoryResponse{
			Code:    -1,
			Message: fmt.Sprintf("创建目录失败: %v", err),
		}, nil
	}

	slog.InfoContext(l.ctx, "创建目录成功", "prefix", req.Prefix)
	return &types.CreateDirectoryResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
