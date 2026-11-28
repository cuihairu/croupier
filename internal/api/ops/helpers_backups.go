package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

func opsBackupsList(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupsListRequest) (*OpsBackupsListResponse, error) {
	// OPS-008：按请求 scope 过滤（游戏域控制台只看本 scope 的备份；平台级
	// 调用无 scope 时全量，与旧行为兼容）。
	scope := svc.GameScopeFromContext(ctx)
	opts := model.ListBackupsOptions{
		PaginationOptions: model.NewPagination(1, 1000),
		GameID:            scope.GameID,
		Env:               scope.Env,
	}
	backups, _, err := svcCtx.BackupModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &OpsBackupsListResponse{
		Backups: buildOpsBackupItems(backups),
	}, nil
}

func opsBackupCreate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupCreateRequest) (*OpsBackupCreateResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	backup, err := createBackupForScope(ctx, svcCtx, scope, req.Name, req.Type, 0)
	if err != nil {
		return nil, err
	}

	return &OpsBackupCreateResponse{
		BackupID: backup.BackupID,
	}, nil
}

func opsBackupDelete(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupDeleteRequest) (*OpsBackupDeleteResponse, error) {
	if svcCtx != nil && svcCtx.BackupModel != nil {
		backupSvc := NewBackupService(svcCtx)
		if err := backupSvc.Delete(ctx, strings.TrimSpace(req.ID)); err != nil {
			return nil, err
		}
	}
	return &OpsBackupDeleteResponse{
		Deleted: true,
	}, nil
}

func opsBackupDownload(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsBackupDownloadRequest) (*OpsBackupDownloadResponse, error) {
	if svcCtx != nil && svcCtx.BackupModel != nil {
		backupSvc := NewBackupService(svcCtx)
		url, _, err := backupSvc.GetDownloadURL(ctx, strings.TrimSpace(req.ID))
		if err != nil {
			return nil, err
		}
		return &OpsBackupDownloadResponse{
			Url: url,
		}, nil
	}
	return &OpsBackupDownloadResponse{
		Url: fmt.Sprintf("/backups/%s/download", req.ID),
	}, nil
}

// Alert operations implementations
