// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package backup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/logic/utils"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type BackupCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建备份
func NewBackupCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BackupCreateLogic {
	return &BackupCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BackupCreateLogic) BackupCreate(req *types.BackupCreateRequest) (*types.BackupDetailResponse, error) {
	backupType := strings.ToLower(strings.TrimSpace(req.Type))
	if backupType == "" {
		backupType = "full"
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("%s-%s", backupType, time.Now().UTC().Format("20060102-150405"))
	}

	backup := &model.Backup{
		BackupID: utils.GenerateBackupID(),
		Name:     name,
		Type:     backupType,
		Status:   "pending",
	}

	if err := l.svcCtx.BackupModel.Create(l.ctx, backup); err != nil {
		return nil, err
	}

	return &types.BackupDetailResponse{
		Backup: utils.BuildBackupDTO(backup),
	}, nil
}
