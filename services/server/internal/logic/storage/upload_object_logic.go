// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UploadObjectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 上传对象
func NewUploadObjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadObjectLogic {
	return &UploadObjectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UploadObjectLogic) UploadObject(req *types.UploadObjectRequest) (resp *types.UploadObjectResponse, err error) {
	// 这个方法保留用于 API 兼容性，但实际上传逻辑在 UploadObjectWithFile 中
	return &types.UploadObjectResponse{
		Code:    -1,
		Message: "请使用 multipart/form-data 格式上传文件",
	}, nil
}

// UploadObjectWithFile 实际的文件上传逻辑
func (l *UploadObjectLogic) UploadObjectWithFile(file io.ReadSeeker, filename string, size int64, contentType string) (resp *types.UploadObjectResponse, err error) {
	// 检查对象存储是否初始化
	if l.svcCtx.ObjectStore == nil {
		return &types.UploadObjectResponse{
			Code:    -1,
			Message: "对象存储未配置",
		}, nil
	}

	// 生成对象键（这里使用文件名作为键，可以根据需要添加路径前缀）
	key := filename

	l.Infof("开始上传文件: key=%s, size=%d, contentType=%s", key, size, contentType)

	// 上传文件
	err = l.svcCtx.ObjectStore.Put(l.ctx, key, file, size, contentType)
	if err != nil {
		l.Errorf("上传文件失败: key=%s, error=%v", key, err)
		return &types.UploadObjectResponse{
			Code:    -1,
			Message: fmt.Sprintf("上传文件失败: %v", err),
		}, nil
	}

	l.Infof("文件上传成功: key=%s", key)

	// 生成签名 URL（可选，根据业务需求）
	url, err := l.svcCtx.ObjectStore.SignedURL(l.ctx, key, "GET", 7*24*time.Hour) // 7天有效期
	if err != nil {
		l.Errorf("生成签名 URL 失败: key=%s, error=%v", key, err)
		// 即使签名 URL 生成失败，文件已上传成功，仍返回成功响应
		return &types.UploadObjectResponse{
			Code:    0,
			Message: "OK",
			Data: types.UploadObjectData{
				Key: key,
				URL: "",
			},
		}, nil
	}

	l.Infof("生成签名 URL 成功: key=%s, url=%s", key, url)

	return &types.UploadObjectResponse{
		Code:    0,
		Message: "OK",
		Data: types.UploadObjectData{
			Key: key,
			URL: url,
		},
	}, nil
}

// 辅助函数：根据文件名检测 Content-Type
func detectContentType(filename string) string {
	ext := filepath.Ext(filename)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}
