// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package backup

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type BackupDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除备份
func NewBackupDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BackupDeleteLogic {
	return &BackupDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BackupDeleteLogic) BackupDelete(req *types.BackupDeleteRequest) error {
	backupID := strings.TrimSpace(req.ID)
	if backupID == "" {
		return errors.New("备份ID不能为空")
	}

	if _, err := l.svcCtx.BackupModel.FindByBackupID(l.ctx, backupID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorx.NewNotFound("备份不存在: " + backupID)
		}
		return err
	}

	return l.svcCtx.BackupModel.DeleteByBackupID(l.ctx, backupID)
}
