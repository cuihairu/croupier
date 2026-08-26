package model

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// AlertRule 是主机指标告警规则（阈值评估）。
//
// 评估语义：report 上报时提取 metric 路径的当前值，与 threshold 按
// operator 比较；可选持续窗口（durationSeconds 内连续 N 次命中才触发，
// N = durationSeconds / 上报间隔，工程上简化为连续命中次数 ≥ forCount）。
// 触发后进入冷却（cooldownSeconds），避免每 30s 上报刷屏。
type AlertRule struct {
	gorm.Model
	Name        string `gorm:"size:128"`
	Description string `gorm:"size:255"`
	// Metric 是指标路径：cpu.usagePercent / memory.usagePercent /
	// memory.usedPercent / disk.<挂载点>.usedPercent / custom.<key>
	Metric string `gorm:"size:128;index;not null"`
	// Operator: gt | gte | lt | lte
	Operator string `gorm:"size:8;not null"`
	// Threshold 阈值（百分比规则用 0-100，字节规则用字节数）。
	Threshold float64 `gorm:"not null"`
	// ForCount 连续命中次数（默认 1；>=2 实现持续窗口语义）。
	ForCount int `gorm:"default:1"`
	// CooldownSeconds 触发后的冷却窗口，抑制重复告警（默认 300）。
	CooldownSeconds int `gorm:"default:300"`
	// Level: info | warning | critical
	Level   string `gorm:"size:16;default:warning"`
	Enabled bool   `gorm:"default:true"`
	// AgentFilter 空串 = 所有 agent；否则仅匹配该 agent。
	AgentFilter string `gorm:"size:128;index"`
	// Scope 限定的 game/env（空 = 平台级）。
	GameID string `gorm:"size:64;index"`
	Env    string `gorm:"size:64;index"`
	// 运行态（评估器维护）：
	HitCount    int        `gorm:"default:0"` // 当前连续命中计数
	LastFiredAt *time.Time // 上次触发时间（冷却判定）
	CreatedBy   string     `gorm:"size:64"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (AlertRule) TableName() string { return "alert_rules" }

// 规则常量。
const (
	AlertRuleLevelInfo     = "info"
	AlertRuleLevelWarning  = "warning"
	AlertRuleLevelCritical = "critical"
)

// AlertRuleModel provides CRUD for alert rules.
type AlertRuleModel struct {
	db *gorm.DB
}

func NewAlertRuleModel(db *gorm.DB) *AlertRuleModel {
	return &AlertRuleModel{db: db}
}

type ListAlertRulesOptions struct {
	Enabled *bool
	Metric  string
}

func (m *AlertRuleModel) List(ctx context.Context, opts ListAlertRulesOptions) ([]AlertRule, error) {
	q := m.db.WithContext(ctx).Model(&AlertRule{})
	if opts.Enabled != nil {
		q = q.Where("enabled = ?", *opts.Enabled)
	}
	if opts.Metric != "" {
		q = q.Where("metric = ?", opts.Metric)
	}
	var out []AlertRule
	err := q.Order("id ASC").Find(&out).Error
	return out, err
}

func (m *AlertRuleModel) FindByID(ctx context.Context, id uint) (*AlertRule, error) {
	var r AlertRule
	if err := m.db.WithContext(ctx).First(&r, id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (m *AlertRuleModel) Create(ctx context.Context, r *AlertRule) error {
	// Enabled=false 是零值：gorm Create 会跳过零值字段而落 default:true，
	// 且会在 Create 后把结构体回填为 default 值。先记住期望值再补写。
	desired := r.Enabled
	if err := m.db.WithContext(ctx).Create(r).Error; err != nil {
		return err
	}
	return m.db.WithContext(ctx).Model(&AlertRule{}).
		Where("id = ?", r.ID).
		Update("enabled", desired).Error
}

func (m *AlertRuleModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	return m.db.WithContext(ctx).Model(&AlertRule{}).Where("id = ?", id).Updates(updates).Error
}

func (m *AlertRuleModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&AlertRule{}, id).Error
}
