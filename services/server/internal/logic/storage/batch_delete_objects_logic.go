// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BatchDeleteObjectsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量删除对象
func NewBatchDeleteObjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchDeleteObjectsLogic {
	return &BatchDeleteObjectsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchDeleteObjectsLogic) BatchDeleteObjects(req *types.BatchDeleteObjectsRequest) (resp *types.BatchDeleteObjectsResponse, err error) {
	// 检查对象存储是否初始化
	if l.svcCtx.ObjectStore == nil {
		return &types.BatchDeleteObjectsResponse{
			Code:    -1,
			Message: "对象存储未配置",
		}, nil
	}

	// 验证请求参数
	if len(req.Keys) == 0 {
		return &types.BatchDeleteObjectsResponse{
			Code:    -1,
			Message: "对象键列表不能为空",
		}, nil
	}

	// 批量删除对象
	deleted := make([]string, 0, len(req.Keys))
	failed := make([]string, 0)

	for _, key := range req.Keys {
		err := l.svcCtx.ObjectStore.Delete(l.ctx, key)
		if err != nil {
			l.Errorf("删除对象失败: key=%s, error=%v", key, err)
			failed = append(failed, key)
		} else {
			deleted = append(deleted, key)
			l.Infof("删除对象成功: key=%s", key)
		}
	}

	return &types.BatchDeleteObjectsResponse{
		Code:    0,
		Message: "OK",
		Data: types.BatchDeleteObjectsData{
			Deleted: deleted,
			Failed:  failed,
		},
	}, nil
}
