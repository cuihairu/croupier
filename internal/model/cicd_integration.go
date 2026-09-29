package model

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CicdIntegration 是一条外部 CI/CD 系统接入记录（OPEN-ISSUES #58：可插拔
// provider 抽象）。Kind 决定对接协议（jenkins/gitlab-ci/github-actions/generic，
// 枚举见 internal/cicd 包），连接凭据存 Token 列——API 读取侧掩码（tokenSet/
// tokenMasked 口径同 notification.*），不回显明文。Provider 专属参数
// （job 名、project id、workflow 文件、状态值映射等）放 Extra。
type CicdIntegration struct {
	gorm.Model
	GameID string `gorm:"size:64;index:idx_cicd_int_scope,priority:1"`
	Env    string `gorm:"size:64;index:idx_cicd_int_scope,priority:2"`

	// provider 类型（internal/cicd.Kinds 闭集）
	Kind string `gorm:"size:32;index"`
	Name string `gorm:"size:128;uniqueIndex:uniq_cicd_int_name,priority:3"`

	// 服务端点（base URL，如 https://ci.example.com）
	Endpoint string `gorm:"size:512"`
	// 接入凭据（API token / PAT / 自定义 header 值）。读侧掩码，不回显。
	Token string `gorm:"size:512"`
	// provider 专属配置（jenkins: job/crumbDisabled；gitlab-ci: project；
	// github-actions: repo/workflow/branch；generic: triggerUrl/statusUrl/header*）
	Extra datatypes.JSONMap `gorm:"type:json"`

	Enabled   bool   `gorm:"default:true"`
	CreatedBy string `gorm:"size:64"`
}

func (CicdIntegration) TableName() string { return "cicd_integrations" }

// 合法的 provider 类型闭集（与 internal/cicd 注册表对齐；model 层独立持有
// 以保持 model 不 import 内部业务包的分层方向）。
var cicdKinds = map[string]struct{}{
	"jenkins": {}, "gitlab-ci": {}, "github-actions": {}, "generic": {},
}

// ValidateCicdIntegration 校验必填与形态。错误文案面向管理端表单透出。
func ValidateCicdIntegration(in *CicdIntegration) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Endpoint = strings.TrimSpace(in.Endpoint)
	in.Kind = strings.TrimSpace(in.Kind)
	if in.Name == "" {
		return errors.New("名称不能为空")
	}
	if len(in.Name) > 128 {
		return errors.New("名称过长（≤128）")
	}
	if _, ok := cicdKinds[in.Kind]; !ok {
		return errors.New("未知的 CI/CD 类型（可选 jenkins/gitlab-ci/github-actions/generic）")
	}
	if !strings.HasPrefix(in.Endpoint, "http://") && !strings.HasPrefix(in.Endpoint, "https://") {
		return errors.New("服务端点必须是 http(s) URL")
	}
	if len(in.Endpoint) > 512 {
		return errors.New("服务端点过长（≤512）")
	}
	if len(in.Token) > 512 {
		return errors.New("凭据过长（≤512）")
	}
	return nil
}

type CicdIntegrationModel struct {
	db *gorm.DB
}

func NewCicdIntegrationModel(db *gorm.DB) *CicdIntegrationModel {
	return &CicdIntegrationModel{db: db}
}

func (m *CicdIntegrationModel) Create(ctx context.Context, in *CicdIntegration) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(in).Error
}

func (m *CicdIntegrationModel) GetByID(ctx context.Context, id uint) (*CicdIntegration, error) {
	var row CicdIntegration
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// List 按可选 scope 过滤（空 game/env 不限），name 升序稳定输出。
func (m *CicdIntegrationModel) List(ctx context.Context, gameID, env string) ([]CicdIntegration, error) {
	q := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&CicdIntegration{})
	if gameID = strings.TrimSpace(gameID); gameID != "" {
		q = q.Where("game_id = ?", gameID)
	}
	if env = strings.TrimSpace(env); env != "" {
		q = q.Where("env = ?", env)
	}
	var rows []CicdIntegration
	if err := q.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Update 覆写非零字段（Token 空串表示「保留原凭据」，由 service 层保证）。
func (m *CicdIntegrationModel) Update(ctx context.Context, in *CicdIntegration) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Save(in).Error
}

func (m *CicdIntegrationModel) Delete(ctx context.Context, id uint) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Delete(&CicdIntegration{}, id).Error
}

// CountByKind 返回各 provider 类型的接入数（设置页概览用）。
func (m *CicdIntegrationModel) CountByKind(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Kind  string
		Count int64
	}
	var rows []row
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&CicdIntegration{}).
		Select("kind, count(*) as count").Group("kind").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Kind] = r.Count
	}
	return out, nil
}

// MaskToken 与 notification secret 同口径：非空凭据只回显尾 4 位。
func MaskToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}
