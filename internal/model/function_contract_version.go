package model

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/db/dbctx"
)

// FunctionContractVersionRetention 是每个函数保留的版本历史上限（用户已定：
// 每函数上限 N 条）。超过后按 seq 最老淘汰；历史是审计便利件而非合规存档，
// 有界增长防高频重注册场景无限膨胀。
const FunctionContractVersionRetention = 50

// FunctionContractVersion 记录函数契约的内容变更历史（B2）：以
// (game_id, env, function_id) 为主维度，每次内容变化（与
// contractSemanticallyEqual 同一判据，重复注册相同内容不写）追加一条快照。
//
// seq 为函数内单调递增序号（max+1 计算，不加唯一索引——多实例 HA 对同库
// 并发注册时唯一约束冲突会毒化注册事务；乱序竞态仅产生并列 seq，读取按
// seq DESC, id DESC 稳定排序）。Snapshot 是变更后的 spec.FunctionSpec 投影
// （removed 时为删除前最后一版）。Diff 是字段级变更摘要 + schema 兼容性命
// 中结果（schemadiff Finding 数组）。
type FunctionContractVersion struct {
	gorm.Model
	GameID       string `gorm:"size:64;index:idx_function_contract_versions_function"`
	Env          string `gorm:"size:64;index:idx_function_contract_versions_function"`
	FunctionID   string `gorm:"size:128;index:idx_function_contract_versions_function"`
	Seq          int64  `gorm:"index:idx_function_contract_versions_function"`
	Version      string `gorm:"size:32"`
	Source       string `gorm:"size:32"` // sdk|openapi|catalog
	SourceDigest string `gorm:"size:64"` // 变更后契约内容 digest（与契约行同源）
	ChangeType   string `gorm:"size:16"` // created|updated|removed
	Breaking     bool   // input/output schema 破坏性变更
	Actor        string `gorm:"size:64"`   // system|用户名（注册路径恒 system）
	Snapshot     JSON   `gorm:"type:json"` // 变更后 FunctionSpec 投影
	Diff         JSON   `gorm:"type:json"` // 变更摘要数组
}

// FunctionContractVersionModel wraps data access for contract version history.
type FunctionContractVersionModel struct {
	db        *gorm.DB
	retention int
}

// NewFunctionContractVersionModel creates the helper.
func NewFunctionContractVersionModel(db *gorm.DB) *FunctionContractVersionModel {
	return &FunctionContractVersionModel{db: db, retention: FunctionContractVersionRetention}
}

// AppendVersion 追加一条历史（seq=max+1）并把该函数的历史裁剪到保留上限。
// 与契约写在同一 ctx/db（dbctx 解析）——注册路径常在调用方事务内，
// 共享同一事务句柄保证契约行与历史行原子可见。
func (m *FunctionContractVersionModel) AppendVersion(ctx context.Context, ver *FunctionContractVersion) error {
	if ver == nil {
		return nil
	}
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var maxSeq *int64
	if err := db.Model(&FunctionContractVersion{}).
		Where("game_id = ? AND env = ? AND function_id = ?", ver.GameID, ver.Env, ver.FunctionID).
		Select("MAX(seq)").Scan(&maxSeq).Error; err != nil {
		return err
	}
	if maxSeq != nil {
		ver.Seq = *maxSeq + 1
	} else {
		ver.Seq = 1
	}
	if err := db.Create(ver).Error; err != nil {
		return err
	}
	return m.trim(db, ver.GameID, ver.Env, ver.FunctionID)
}

// trim 删除超出保留上限的最老历史（按 seq、id 降序保留最新 N 条）。
func (m *FunctionContractVersionModel) trim(db *gorm.DB, gameID, env, functionID string) error {
	retention := m.retention
	if retention <= 0 {
		retention = FunctionContractVersionRetention
	}
	var keepIDs []uint
	if err := db.Unscoped().Model(&FunctionContractVersion{}).
		Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
		Order("seq DESC, id DESC").
		Limit(retention).
		Pluck("id", &keepIDs).Error; err != nil {
		return err
	}
	if len(keepIDs) < retention {
		return nil
	}
	return db.Unscoped().
		Where("game_id = ? AND env = ? AND function_id = ? AND id NOT IN ?", gameID, env, functionID, keepIDs).
		Delete(&FunctionContractVersion{}).Error
}

// ListByFunctionPaged 返回该函数最新在前的历史页（不含 Snapshot，载荷瘦身）。
func (m *FunctionContractVersionModel) ListByFunctionPaged(ctx context.Context, gameID, env, functionID string, limit, offset int) ([]*FunctionContractVersion, int64, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var total int64
	if err := db.Model(&FunctionContractVersion{}).
		Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var vers []*FunctionContractVersion
	if err := db.Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
		Order("seq DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&vers).Error; err != nil {
		return nil, 0, err
	}
	for _, ver := range vers {
		ver.Snapshot = nil
	}
	return vers, total, nil
}

// FindBySeq 返回单条历史（含 Snapshot）。
func (m *FunctionContractVersionModel) FindBySeq(ctx context.Context, gameID, env, functionID string, seq int64) (*FunctionContractVersion, error) {
	var ver FunctionContractVersion
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND function_id = ? AND seq = ?", gameID, env, functionID, seq).
		Order("id DESC").
		First(&ver).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &ver, nil
}
