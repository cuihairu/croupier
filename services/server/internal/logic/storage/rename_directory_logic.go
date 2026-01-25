// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RenameDirectoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 重命名/移动目录
func NewRenameDirectoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RenameDirectoryLogic {
	return &RenameDirectoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RenameDirectoryLogic) RenameDirectory(req *types.RenameDirectoryRequest) (resp *types.RenameDirectoryResponse, err error) {
	// 检查对象存储是否初始化
	if l.svcCtx.ObjectStore == nil {
		return &types.RenameDirectoryResponse{
			Code:    -1,
			Message: "对象存储未配置",
		}, nil
	}

	// 验证请求参数
	if req.OldPrefix == "" || req.NewPrefix == "" {
		return &types.RenameDirectoryResponse{
			Code:    -1,
			Message: "目录前缀不能为空",
		}, nil
	}

	// 重命名/移动目录
	err = l.svcCtx.ObjectStore.RenamePrefix(l.ctx, req.OldPrefix, req.NewPrefix)
	if err != nil {
		l.Errorf("重命名目录失败: oldPrefix=%s, newPrefix=%s, error=%v", req.OldPrefix, req.NewPrefix, err)
		return &types.RenameDirectoryResponse{
			Code:    -1,
			Message: fmt.Sprintf("重命名目录失败: %v", err),
		}, nil
	}

	l.Infof("重命名目录成功: oldPrefix=%s, newPrefix=%s", req.OldPrefix, req.NewPrefix)
	return &types.RenameDirectoryResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
