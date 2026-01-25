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

type DeleteObjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除对象
func NewDeleteObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteObjectLogic {
	return &DeleteObjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteObjectLogic) DeleteObject(req *types.DeleteObjectRequest) (resp *types.DeleteObjectResponse, err error) {
	// 检查对象存储是否初始化
	if l.svcCtx.ObjectStore == nil {
		return &types.DeleteObjectResponse{
			Code:    -1,
			Message: "对象存储未配置",
		}, nil
	}

	// 验证请求参数
	if req.Key == "" {
		return &types.DeleteObjectResponse{
			Code:    -1,
			Message: "对象键不能为空",
		}, nil
	}

	// 删除对象
	err = l.svcCtx.ObjectStore.Delete(l.ctx, req.Key)
	if err != nil {
		l.Errorf("删除对象失败: key=%s, error=%v", req.Key, err)
		return &types.DeleteObjectResponse{
			Code:    -1,
			Message: fmt.Sprintf("删除对象失败: %v", err),
		}, nil
	}

	l.Infof("删除对象成功: key=%s", req.Key)
	return &types.DeleteObjectResponse{
		Code:    0,
		Message: "OK",
	}, nil
}
