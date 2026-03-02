// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package backup

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// DownloadPayload captures the data streaming details for a backup file.
type DownloadPayload struct {
	Filename    string
	Size        int64
	Reader      io.ReadCloser
	RedirectURL string
}

type BackupDownloadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 下载备份
func NewBackupDownloadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BackupDownloadLogic {
	return &BackupDownloadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BackupDownloadLogic) BackupDownload(req *types.BackupDownloadRequest) (*DownloadPayload, error) {
	backupID := strings.TrimSpace(req.ID)
	if backupID == "" {
		return nil, errors.New("备份ID不能为空")
	}

	record, err := l.svcCtx.BackupModel.FindByBackupID(l.ctx, backupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("备份不存在: " + backupID)
		}
		return nil, err
	}

	location := strings.TrimSpace(record.Location)
	if location == "" {
		return nil, errorx.NewBadRequest("备份尚未生成可下载文件: " + backupID)
	}

	if payload, ok := l.tryRemoteDownload(location); ok {
		return payload, nil
	}

	path, err := resolveBackupPath(location)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, errorx.NewInternalError("无法打开备份文件")
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, errorx.NewInternalError("读取备份文件信息失败")
	}

	return &DownloadPayload{
		Filename: utils.GuessBackupFilename(record),
		Size:     info.Size(),
		Reader:   file,
	}, nil
}

func (l *BackupDownloadLogic) tryRemoteDownload(location string) (*DownloadPayload, bool) {
	u, err := url.Parse(location)
	if err != nil || u.Scheme == "" || u.Scheme == "file" {
		return nil, false
	}
	return &DownloadPayload{
		RedirectURL: location,
	}, true
}

func resolveBackupPath(location string) (string, error) {
	if strings.HasPrefix(location, "file://") {
		u, err := url.Parse(location)
		if err != nil {
			return "", errorx.NewBadRequest("解析下载路径失败")
		}
		return u.Path, nil
	}
	if filepath.IsAbs(location) {
		return location, nil
	}
	abs, err := filepath.Abs(location)
	if err != nil {
		return "", errorx.NewBadRequest("解析备份路径失败")
	}
	return abs, nil
}
