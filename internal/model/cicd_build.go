package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CicdBuild 是一次外部 CI 构建（打包）记录（OPEN-ISSUES #58）。行来源
// 有二：①管理端经 provider 触发/拉取（triggered_by=api）；②外部系统
// webhook 状态回写（triggered_by=webhook）。ExternalID 是 provider 侧
// 构建标识（jenkins 队列/构建 URL、gitlab pipeline id、gh run id、generic
// 自定义 id），(integration_id, external_id) 唯一——webhook 重复投递幂等。
type CicdBuild struct {
	gorm.Model
	GameID string `gorm:"size:64;index:idx_cicd_build_scope,priority:1"`
	Env    string `gorm:"size:64;index:idx_cicd_build_scope,priority:2"`

	IntegrationID uint   `gorm:"index;uniqueIndex:uniq_cicd_build_ext,priority:1"`
	ExternalID    string `gorm:"size:255;uniqueIndex:uniq_cicd_build_ext,priority:2"`
	Kind          string `gorm:"size:32;index"` // provider 类型快照（integration 删除后仍可辨）

	// 流水线标识（jenkins job / gitlab ref 名 / gh workflow 文件 / 自定义）
	Pipeline string `gorm:"size:255;index"`

	// queued|running|success|failed|cancelled|unknown（internal/cicd 状态闭集）
	Status      string `gorm:"size:32;index"`
	TriggeredBy string `gorm:"size:16"` // api|webhook
	Version     string `gorm:"size:64;index"`
	WebURL      string `gorm:"size:512"`

	ArtifactURL      string `gorm:"size:512"`
	ArtifactChecksum string `gorm:"size:128"`

	StartedAt  *time.Time
	FinishedAt *time.Time

	Raw datatypes.JSONMap `gorm:"type:json"` // webhook/provider 原始字段留存排障
}

func (CicdBuild) TableName() string { return "cicd_builds" }

// 构建状态闭集。
const (
	CicdBuildQueued    = "queued"
	CicdBuildRunning   = "running"
	CicdBuildSuccess   = "success"
	CicdBuildFailed    = "failed"
	CicdBuildCancelled = "cancelled"
	CicdBuildUnknown   = "unknown"
)

// NormalizeCicdBuildStatus 把任意输入归一到状态闭集（未识别 → unknown）。
func NormalizeCicdBuildStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case CicdBuildQueued, "pending", "created", "waiting", "waiting_for_resource", "preparing", "queued_or_running":
		return CicdBuildQueued
	case CicdBuildRunning, "in_progress", "building":
		return CicdBuildRunning
	case CicdBuildSuccess, "succeeded", "ok", "passed", "done", "complete", "completed_ok":
		return CicdBuildSuccess
	case CicdBuildFailed, "failure", "error", "errored", "unstable":
		return CicdBuildFailed
	case CicdBuildCancelled, "canceled", "aborted", "skipped":
		return CicdBuildCancelled
	default:
		return CicdBuildUnknown
	}
}

// ValidateCicdWebhook 校验 webhook 回写必填：externalId 必须有（幂等键），
// status 归一后不允许 unknown-only 空串输入。
func ValidateCicdWebhook(pipeline, externalID, status string) (string, error) {
	if strings.TrimSpace(externalID) == "" {
		return "", errors.New("externalId 不能为空")
	}
	if len(externalID) > 255 {
		return "", errors.New("externalId 过长（≤255）")
	}
	norm := NormalizeCicdBuildStatus(status)
	return norm, nil
}

type CicdBuildModel struct {
	db *gorm.DB
}

func NewCicdBuildModel(db *gorm.DB) *CicdBuildModel {
	return &CicdBuildModel{db: db}
}

// UpsertByExternalKey 按 (integration_id, external_id) 幂等写入：存在则
// 更新状态/URL/产物/时间与 Raw，不存在则建行。返回行 ID。
func (m *CicdBuildModel) UpsertByExternalKey(ctx context.Context, in *CicdBuild) (uint, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var existing CicdBuild
	err := db.Where("integration_id = ? AND external_id = ?", in.IntegrationID, in.ExternalID).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := db.Create(in).Error; err != nil {
			return 0, err
		}
		return in.ID, nil
	}
	if err != nil {
		return 0, err
	}
	updates := map[string]interface{}{
		"status":            in.Status,
		"web_url":           in.WebURL,
		"artifact_url":      in.ArtifactURL,
		"artifact_checksum": in.ArtifactChecksum,
		"raw":               in.Raw,
	}
	if in.StartedAt != nil {
		updates["started_at"] = in.StartedAt
	}
	if in.FinishedAt != nil {
		updates["finished_at"] = in.FinishedAt
	}
	if err := db.Model(&existing).Updates(updates).Error; err != nil {
		return 0, err
	}
	return existing.ID, nil
}

type CicdBuildQueryOptions struct {
	GameID        string
	Env           string
	IntegrationID uint
	Kind          string
	Status        string
	Pipeline      string
	Version       string
	Limit         int
	Offset        int
}

// List 按过滤条件倒序（新构建在前）分页，返回行与总数。
func (m *CicdBuildModel) List(ctx context.Context, opts CicdBuildQueryOptions) ([]CicdBuild, int64, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&CicdBuild{})
	if opts.GameID = strings.TrimSpace(opts.GameID); opts.GameID != "" {
		db = db.Where("game_id = ?", opts.GameID)
	}
	if opts.Env = strings.TrimSpace(opts.Env); opts.Env != "" {
		db = db.Where("env = ?", opts.Env)
	}
	if opts.IntegrationID != 0 {
		db = db.Where("integration_id = ?", opts.IntegrationID)
	}
	if opts.Kind = strings.TrimSpace(opts.Kind); opts.Kind != "" {
		db = db.Where("kind = ?", opts.Kind)
	}
	if opts.Status = strings.TrimSpace(opts.Status); opts.Status != "" {
		db = db.Where("status = ?", opts.Status)
	}
	if opts.Pipeline = strings.TrimSpace(opts.Pipeline); opts.Pipeline != "" {
		db = db.Where("pipeline = ?", opts.Pipeline)
	}
	if opts.Version = strings.TrimSpace(opts.Version); opts.Version != "" {
		db = db.Where("version = ?", opts.Version)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	var rows []CicdBuild
	if err := db.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetByID 取单条构建记录。
func (m *CicdBuildModel) GetByID(ctx context.Context, id uint) (*CicdBuild, error) {
	var row CicdBuild
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// UpdateStatus 回写状态与 URL/时间戳（RefreshBuild 与 webhook 共用）。
func (m *CicdBuildModel) UpdateStatus(ctx context.Context, id uint, status, webURL string,
	startedAt, finishedAt *time.Time) (*CicdBuild, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	updates := map[string]interface{}{"status": status}
	if webURL != "" {
		updates["web_url"] = webURL
	}
	if startedAt != nil {
		updates["started_at"] = startedAt
	}
	if finishedAt != nil {
		updates["finished_at"] = finishedAt
	}
	if err := db.Model(&CicdBuild{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return m.GetByID(ctx, id)
}

// DeleteByIntegration 删除接入时级联清构建记录（外键不设，业务级联）。
func (m *CicdBuildModel) DeleteByIntegration(ctx context.Context, integrationID uint) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("integration_id = ?", integrationID).Delete(&CicdBuild{}).Error
}
