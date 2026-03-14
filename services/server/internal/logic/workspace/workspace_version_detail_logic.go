// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"gorm.io/gorm"
)

type WorkspaceVersionDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取工作区版本详情
func NewWorkspaceVersionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceVersionDetailLogic {
	return &WorkspaceVersionDetailLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *WorkspaceVersionDetailLogic) WorkspaceVersionDetail(req *types.WorkspaceVersionDetailRequest) (resp *types.WorkspaceVersionDetailResponse, err error) {
	// Parse version from VersionId (could be "v1", "1", etc.)
	versionID := strings.TrimSpace(req.VersionId)
	versionID = strings.TrimPrefix(versionID, "v")
	version, err := strconv.Atoi(versionID)
	if err != nil {
		return nil, errors.New("invalid version ID")
	}

	// Get the workspace version key
	key := workspaceVersionKey(req.ObjectKey)

	// Find the specific version record
	record, err := l.svcCtx.ConfigVersionModel.Find(l.ctx, key, version)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("workspace version not found")
		}
		return nil, err
	}

	// Parse the config JSON
	var configData interface{}
	if record.Value != "" {
		if err := json.Unmarshal([]byte(record.Value), &configData); err != nil {
			// If JSON parsing fails, return the raw string
			configData = record.Value
		}
	}

	// Build the response record
	versionRecord := types.WorkspaceVersionRecord{
		Id:        strconv.FormatUint(uint64(record.ID), 10),
		ObjectKey: record.Key,
		Version:   record.Version,
		Config:    configData,
		CreatedAt: record.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedBy: record.CreatedBy,
		Comment:   record.Message,
	}

	// Check if this is the current draft or published version
	latest, err := l.svcCtx.ConfigVersionModel.FindLatest(l.ctx, key)
	if err == nil {
		versionRecord.IsCurrentDraft = (latest.ID == record.ID)
	}

	return &types.WorkspaceVersionDetailResponse{
		WorkspaceVersionRecord: versionRecord,
	}, nil
}
