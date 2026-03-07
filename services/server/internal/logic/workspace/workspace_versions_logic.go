package workspace

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/common/errorx"
	"github.com/cuihairu/croupier/services/server/internal/model"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
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

func (l *WorkspaceVersionsLogic) List(objectKey, from, to string) ([]map[string]interface{}, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	fromAt, toAt, err := parseWorkspaceVersionTimeRange(from, to)
	if err != nil {
		return nil, err
	}
	versionKey := workspaceVersionKey(objectKey)
	records, err := l.svcCtx.ConfigVersionModel.List(l.ctx, versionKey)
	if err != nil {
		return nil, err
	}
	currentDraftVersion := 0
	if latest, latestErr := l.svcCtx.ConfigVersionModel.FindLatest(l.ctx, versionKey); latestErr == nil {
		currentDraftVersion = latest.Version
	} else if latestErr != nil && !errors.Is(latestErr, gorm.ErrRecordNotFound) {
		return nil, latestErr
	}

	currentPublishedVersion := 0
	items := make([]map[string]interface{}, 0, len(records))
	for i := range records {
		if !isWorkspaceVersionWithin(records[i], fromAt, toAt) {
			continue
		}
		item, parseErr := mapWorkspaceVersionRecord(objectKey, records[i])
		if parseErr != nil {
			continue
		}
		if currentPublishedVersion == 0 {
			if cfg, ok := item["config"].(*types.WorkspaceConfig); ok && isPublishedWorkspaceVersion(cfg) {
				currentPublishedVersion = records[i].Version
			}
		}
		items = append(items, item)
	}
	return withWorkspaceVersionState(items, currentDraftVersion, currentPublishedVersion), nil
}

func (l *WorkspaceVersionsLogic) Detail(objectKey, versionID string) (map[string]interface{}, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, errorx.NewBadRequest("objectKey is required")
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, errorx.NewBadRequest("versionId is required")
	}
	version, err := strconv.Atoi(versionID)
	if err != nil || version <= 0 {
		return nil, errorx.NewBadRequest("invalid versionId")
	}

	record, err := l.svcCtx.ConfigVersionModel.Find(l.ctx, workspaceVersionKey(objectKey), version)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("workspace version not found")
		}
		return nil, err
	}

	item, err := mapWorkspaceVersionRecord(objectKey, *record)
	if err != nil {
		return nil, errorx.NewBadRequest("workspace version payload is invalid")
	}
	return item, nil
}

func mapWorkspaceVersionRecord(objectKey string, record model.ConfigVersion) (map[string]interface{}, error) {
	cfg, parseErr := parseWorkspaceDTOFromVersion(record.Value)
	if parseErr != nil {
		return nil, parseErr
	}
	return map[string]interface{}{
		"id":        strconv.Itoa(record.Version),
		"objectKey": objectKey,
		"version":   record.Version,
		"config":    cfg,
		"createdAt": record.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"createdBy": record.CreatedBy,
		"comment":   record.Message,
	}, nil
}

func parseWorkspaceVersionTimeRange(from, to string) (*time.Time, *time.Time, error) {
	var fromAt *time.Time
	var toAt *time.Time
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from != "" {
		parsed, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return nil, nil, errorx.NewBadRequest("invalid from time, expected RFC3339")
		}
		utc := parsed.UTC()
		fromAt = &utc
	}
	if to != "" {
		parsed, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return nil, nil, errorx.NewBadRequest("invalid to time, expected RFC3339")
		}
		utc := parsed.UTC()
		toAt = &utc
	}
	if fromAt != nil && toAt != nil && fromAt.After(*toAt) {
		return nil, nil, errorx.NewBadRequest("from time cannot be after to time")
	}
	return fromAt, toAt, nil
}

func isWorkspaceVersionWithin(record model.ConfigVersion, fromAt, toAt *time.Time) bool {
	createdAt := record.CreatedAt.UTC()
	if fromAt != nil && createdAt.Before(*fromAt) {
		return false
	}
	if toAt != nil && createdAt.After(*toAt) {
		return false
	}
	return true
}

func isPublishedWorkspaceVersion(cfg *types.WorkspaceConfig) bool {
	return resolveWorkspaceStatus(cfg) == workspaceStatusPublished
}

func withWorkspaceVersionState(
	items []map[string]interface{},
	currentDraftVersion int,
	currentPublishedVersion int,
) []map[string]interface{} {
	if len(items) == 0 {
		return items
	}
	for i := range items {
		items[i]["isCurrentDraft"] = items[i]["version"] == currentDraftVersion
		items[i]["isCurrentPublished"] = items[i]["version"] == currentPublishedVersion
	}
	return items
}
