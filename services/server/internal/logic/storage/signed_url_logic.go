// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package storage

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SignedUrlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取签名URL
func NewSignedUrlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SignedUrlLogic {
	return &SignedUrlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SignedUrlLogic) SignedUrl(req *types.SignedUrlRequest) (*types.SignedUrlResponse, error) {
	if l.svcCtx.ObjectStore == nil {
		return nil, errors.New("对象存储未配置")
	}
	if req == nil {
		return nil, errors.New("缺少请求参数")
	}

	key := strings.TrimSpace(req.Path)
	if key == "" {
		return nil, errors.New("path 参数不能为空")
	}

	expiry := time.Duration(req.Expire) * time.Second
	url, err := l.svcCtx.ObjectStore.SignedURL(l.ctx, key, httpMethodGet, expiry)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"path": key,
		"url":  url,
	}
	if req.Expire > 0 {
		data["expiresIn"] = req.Expire
	}

	return &types.SignedUrlResponse{
		Code:    0,
		Message: "OK",
		Data:    data,
	}, nil
}

const httpMethodGet = "GET"
