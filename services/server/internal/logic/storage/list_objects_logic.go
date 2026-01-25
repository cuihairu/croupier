// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListObjectsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 列出对象
func NewListObjectsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListObjectsLogic {
	return &ListObjectsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListObjectsLogic) ListObjects(req *types.ListObjectsRequest) (resp *types.ListObjectsResponse, err error) {
	// 检查对象存储是否初始化
	if l.svcCtx.ObjectStore == nil {
		return &types.ListObjectsResponse{
			Code:    -1,
			Message: "对象存储未配置",
		}, nil
	}

	// 调用 Store 的 List 方法
	result, err := l.svcCtx.ObjectStore.List(l.ctx, req.Prefix, req.Marker, req.Delimiter, req.Limit)
	if err != nil {
		l.Errorf("列出对象失败: %v", err)
		return &types.ListObjectsResponse{
			Code:    -1,
			Message: fmt.Sprintf("列出对象失败: %v", err),
		}, nil
	}

	// 转换结果
	objects := make([]types.ObjectInfo, 0, len(result.Objects))
	for _, obj := range result.Objects {
		objects = append(objects, types.ObjectInfo{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified.Format(time.RFC3339),
			ETag:         obj.ETag,
			StorageClass: obj.StorageClass,
		})
	}

	return &types.ListObjectsResponse{
		Code:    0,
		Message: "OK",
		Data: types.ObjectsData{
			Objects:     objects,
			Prefixes:    result.Prefixes,
			IsTruncated: result.IsTruncated,
			NextMarker:  result.NextMarker,
		},
	}, nil
}
