// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ComponentsInstallLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewComponentsInstallLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ComponentsInstallLogic {
	return &ComponentsInstallLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ComponentsInstallLogic) ComponentsInstall(r *http.Request) (*types.ComponentUploadResponse, error) {
	if r == nil {
		return nil, errors.New("nil request")
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "component-*.tar.gz")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return nil, err
	}
	defer os.Remove(tmpPath)

	staging := l.svcCtx.ComponentStagingDir()
	if staging == "" {
		os.Remove(tmpPath)
		return nil, errors.New("staging directory not configured")
	}
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return nil, err
	}
	if err := extractPackArchive(tmpPath, staging); err != nil {
		return nil, err
	}
	cm := l.svcCtx.ComponentManager()
	if cm == nil {
		return nil, errors.New("component manager unavailable")
	}
	if err := cm.InstallComponent(staging); err != nil {
		return nil, err
	}
	return &types.ComponentUploadResponse{Ok: true}, nil
}
