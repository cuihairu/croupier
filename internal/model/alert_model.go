package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AlertModel provides CRUD operations for alerts and silences.
type AlertModel struct {
	db *gorm.DB
}

// NewAlertModel creates a new alert model helper.
func NewAlertModel(db *gorm.DB) *AlertModel {
	return &AlertModel{db: db}
}

// ListAlertsOptions controls filtering/pagination.
type ListAlertsOptions struct {
	PaginationOptions
	Level  string
	Status string
	Source string
	// HandleStatus 工作流状态过滤（pending/acknowledged/ignored/escalated）。
	HandleStatus string
	// ExcludeSources 排除的告警来源：/ops/alerts 是基础设施告警中心，
	// 函数域业务告警（source=contract 的 schema 破坏性变更）由函数域
	// 诊断呈现，不混入运维告警列表。
	ExcludeSources []string
	// GameID/Env scope 过滤（4862）。非空时：
	//   (game_id = ? AND env = ?) OR (game_id = '' AND env = '')
	// 即当前 scope 的告警 + 平台级告警（全局可见）。
	// 为空时不过滤（全局视野，面板告警页 /api/v1/alerts 无 scope）。
	// 由 service 层从 GameScopeFromContext 传入，model 层不读 ctx scope
	//（internal/model 不能 import internal/svc，环）。
	ScopeGameID string
	ScopeEnv    string
}

// Create inserts a new alert.
func (m *AlertModel) Create(ctx context.Context, alert *Alert) error {
	return m.db.WithContext(ctx).Create(alert).Error
}

// UpsertByAlertID 按 alert_id 唯一键幂等写入（入站 webhook 去重）：
// 同 alert_id 重推返回已存在行（dedup），不重复落库。返回 (row, created, error)。
func (m *AlertModel) UpsertByAlertID(ctx context.Context, alert *Alert) (row *Alert, created bool, err error) {
	existing, err := m.FindByAlertID(ctx, alert.AlertID)
	if err == nil {
		return existing, false, nil
	}
	if !strings.Contains(err.Error(), "not found") {
		return nil, false, err
	}
	if err := m.Create(ctx, alert); err != nil {
		return nil, false, err
	}
	return alert, true, nil
}

// FindByAlertID returns alert by external alert id.
func (m *AlertModel) FindByAlertID(ctx context.Context, alertID string) (*Alert, error) {
	if strings.TrimSpace(alertID) == "" {
		return nil, errors.New("alert_id is required")
	}
	var alert Alert
	if err := m.db.WithContext(ctx).
		Where("alert_id = ?", alertID).
		First(&alert).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("alert %s not found", alertID)
		}
		return nil, err
	}
	return &alert, nil
}

// UpdateStatus updates alert status.
func (m *AlertModel) UpdateStatus(ctx context.Context, id uint, status string) error {
	return m.db.WithContext(ctx).
		Model(&Alert{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateLifecycle 生命周期转换一次落库：status + message + details 同语句
// 更新（不留半态），按 alert_id 幂等键定位。details 须走 datatypes.JSONMap
// 才会被 Valuer 序列化（裸 map 直更不被驱动接受）。
func (m *AlertModel) UpdateLifecycle(ctx context.Context, alertID, status, message string, details map[string]interface{}) error {
	if strings.TrimSpace(alertID) == "" {
		return errors.New("alert_id is required")
	}
	return m.db.WithContext(ctx).
		Model(&Alert{}).
		Where("alert_id = ?", alertID).
		Updates(map[string]interface{}{
			"status":  status,
			"message": message,
			"details": datatypes.JSONMap(details),
		}).Error
}

// HandleTransition 工作流流转审计载荷（handled_at 服务端取库钟，不信任客户端）。
type HandleTransition struct {
	HandleStatus  string
	HandledBy     string
	HandledRemark string
}

// UpdateHandling 应用风险事件工作流流转：handle_status + 处理人/备注/
// 时间一次落库（审计字段同事务，不留半态）。
func (m *AlertModel) UpdateHandling(ctx context.Context, id uint, t HandleTransition) error {
	now := NowUTC()
	return m.db.WithContext(ctx).
		Model(&Alert{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"handle_status":  t.HandleStatus,
			"handled_by":     t.HandledBy,
			"handled_remark": t.HandledRemark,
			"handled_at":     now,
		}).Error
}

// List returns paginated alerts.
func (m *AlertModel) List(ctx context.Context, opts ListAlertsOptions) ([]Alert, int64, error) {
	opts.Normalize()

	var (
		items []Alert
		total int64
	)

	query := m.db.WithContext(ctx).Model(&Alert{})

	if opts.Level != "" {
		query = query.Where("level = ?", opts.Level)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.HandleStatus != "" {
		query = query.Where("handle_status = ?", opts.HandleStatus)
	}
	if opts.Source != "" {
		query = query.Where("source = ?", opts.Source)
	}
	if len(opts.ExcludeSources) > 0 {
		query = query.Where("source NOT IN ?", opts.ExcludeSources)
	}
	// 4862 scope 过滤：game_id/env 同时非空时当前 scope 告警 + 平台级（空 game_id/env）告警；
	// 任一为空时不过滤（全局视野，面板告警页）。平台级告警的 game_id/env 可能为空串或 NULL。
	if opts.ScopeGameID != "" && opts.ScopeEnv != "" {
		query = query.Where(
			"(game_id = ? AND env = ?) OR ((game_id = '' OR game_id IS NULL) AND (env = '' OR env IS NULL))",
			opts.ScopeGameID, opts.ScopeEnv,
		)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// CreateSilence records a new silence window.
func (m *AlertModel) CreateSilence(ctx context.Context, silence *AlertSilence) error {
	if silence.ExpiresAt.IsZero() && silence.DurationMinute > 0 {
		silence.ExpiresAt = NowUTC().Add(time.Duration(silence.DurationMinute) * time.Minute)
	}
	return m.db.WithContext(ctx).Create(silence).Error
}

// ListSilencesOptions filters silence entries.
type ListSilencesOptions struct {
	ActiveOnly bool
}

// ListSilences returns alert silences.
func (m *AlertModel) ListSilences(ctx context.Context, opts ListSilencesOptions) ([]AlertSilence, error) {
	query := m.db.WithContext(ctx).Model(&AlertSilence{})
	if opts.ActiveOnly {
		query = query.Where("expires_at > ?", NowUTC())
	}

	var silences []AlertSilence
	if err := query.Order("created_at DESC").Find(&silences).Error; err != nil {
		return nil, err
	}
	return silences, nil
}

// DeleteSilence removes silence entry by ID.
func (m *AlertModel) DeleteSilence(ctx context.Context, silenceID uint) error {
	return m.db.WithContext(ctx).Where("id = ?", silenceID).Delete(&AlertSilence{}).Error
}

// BootstrapAlerts ensures seed data for testing/dev.
func (m *AlertModel) BootstrapAlerts(ctx context.Context, alerts []Alert) error {
	for _, alert := range alerts {
		if alert.AlertID == "" {
			continue
		}
		var existing Alert
		if err := m.db.WithContext(ctx).Where("alert_id = ?", alert.AlertID).First(&existing).Error; err == nil {
			continue
		}
		if err := m.Create(ctx, &alert); err != nil {
			return err
		}
	}
	return nil
}

// PruneExpiredSilences removes expired silences.
func (m *AlertModel) PruneExpiredSilences(ctx context.Context) error {
	return m.db.WithContext(ctx).
		Where("expires_at <= ?", NowUTC()).
		Delete(&AlertSilence{}).Error
}

// FindByIDs returns alerts indexed by their primary key.
func (m *AlertModel) FindByIDs(ctx context.Context, ids []uint) (map[uint]*Alert, error) {
	result := make(map[uint]*Alert)
	if len(ids) == 0 {
		return result, nil
	}

	var alerts []Alert
	if err := m.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&alerts).Error; err != nil {
		return nil, err
	}

	for i := range alerts {
		alert := alerts[i]
		result[alert.ID] = &alerts[i]
	}
	return result, nil
}
