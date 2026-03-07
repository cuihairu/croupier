package workspace

import (
	"context"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type WorkspaceVersionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceVersionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceVersionsLogic {
	return &WorkspaceVersionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceVersionsLogic) List(objectKey string) ([]map[string]interface{}, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	records, err := l.svcCtx.ConfigVersionModel.List(l.ctx, workspaceVersionKey(objectKey))
	if err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(records))
	for i := range records {
		cfg, parseErr := parseWorkspaceDTOFromVersion(records[i].Value)
		if parseErr != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"id":        strconv.Itoa(records[i].Version),
			"objectKey": objectKey,
			"version":   records[i].Version,
			"config":    cfg,
			"createdAt": records[i].CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			"createdBy": records[i].CreatedBy,
			"comment":   records[i].Message,
		})
	}
	return items, nil
}
