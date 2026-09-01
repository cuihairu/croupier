package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/db/dbctx"
)

// FunctionContractModel wraps data access for function contracts.
type FunctionContractModel struct {
	db *gorm.DB
}

// NewFunctionContractModel creates the helper.
func NewFunctionContractModel(db *gorm.DB) *FunctionContractModel {
	return &FunctionContractModel{db: db}
}

// UpsertContract creates or updates a function contract.
func (m *FunctionContractModel) UpsertContract(ctx context.Context, contract *FunctionContract) error {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var existing FunctionContract
	err := db.Unscoped().Where("game_id = ? AND env = ? AND function_id = ?",
		contract.GameID, contract.Env, contract.FunctionID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(contract).Error
	}
	if err != nil {
		return err
	}
	// 内容无变化则跳过写：agent 重启/SDK 重连的重注册是常态，
	// 盲目 Save 会整行覆盖并刷新 updated_at——下游（proposal 重生成、
	// 页面 freshness 对比快照）被幽灵扰动，已发布页面出现假性 stale。
	// 软删除行不算"无变化"——重注册语义上就是复活（保存 DeletedAt 复位）。
	if existing.DeletedAt.Time.IsZero() && contractSemanticallyEqual(&existing, contract) {
		return nil
	}
	contract.ID = existing.ID
	contract.CreatedAt = existing.CreatedAt
	contract.DeletedAt = gorm.DeletedAt{}
	return db.Save(contract).Error
}

// contractSemanticallyEqual 比较契约全部字段（含展示层 Summary/
// Description/Tags——文案变化需要写入，e2e 契约链路依赖 proposal
// 反映新文案）；schema 用 canonical JSON（键序/空格形态差异不算
// 变化——六语言 SDK 各自序列化同 schema 字节不同），JSONMap 走
// 零值归一（nil 与 {required:false} 等价）。Diagnostics 属注册期
// 质检快照，轮换不构成契约变化，不参与。
func contractSemanticallyEqual(a, b *FunctionContract) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Version == b.Version &&
		a.Enabled == b.Enabled &&
		a.Deprecated == b.Deprecated &&
		a.ResourceKey == b.ResourceKey &&
		a.OperationKey == b.OperationKey &&
		a.Capability == b.Capability &&
		a.Execution == b.Execution &&
		a.TimeoutMs == b.TimeoutMs &&
		a.Risk == b.Risk &&
		strings.TrimSpace(a.Permission) == strings.TrimSpace(b.Permission) &&
		a.Source == b.Source &&
		jsonMapEqual(a.Approval, b.Approval) &&
		jsonMapEqual(a.Summary, b.Summary) &&
		jsonMapEqual(a.Description, b.Description) &&
		bytes.Equal(canonicalJSON(a.Tags), canonicalJSON(b.Tags)) &&
		bytes.Equal(canonicalJSON(a.InputSchema), canonicalJSON(b.InputSchema)) &&
		bytes.Equal(canonicalJSON(a.OutputSchema), canonicalJSON(b.OutputSchema))
}

// canonicalJSON 解析后按字典序键重排再序列化——语义相同、字节形态
// 不同的 JSON（六语言 SDK 各自序列化）相等。
func canonicalJSON(raw JSON) []byte {
	if len(raw) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return []byte(raw)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return []byte(raw)
	}
	return out
}

// jsonMapEqual 比较 datatypes.JSONMap：nil / 空 / 零值（false、""）等价。
// 注册链对 Approval 的零值在不同路径产生 nil 与 {required:false,policyKey:""}
// 两种形态语义相同，不应触发重写。
func jsonMapEqual(a, b datatypes.JSONMap) bool {
	na, nb := normalizeJSONMap(a), normalizeJSONMap(b)
	if len(na) != len(nb) {
		return false
	}
	if len(na) == 0 {
		return true
	}
	return bytes.Equal(canonicalJSON(mustMarshalMap(na)), canonicalJSON(mustMarshalMap(nb)))
}

// normalizeJSONMap 剔除零值条目（false/空串），nil 返回空 map。
func normalizeJSONMap(m datatypes.JSONMap) datatypes.JSONMap {
	out := datatypes.JSONMap{}
	for k, v := range m {
		switch tv := v.(type) {
		case bool:
			if !tv {
				continue
			}
		case string:
			if strings.TrimSpace(tv) == "" {
				continue
			}
		}
		out[k] = v
	}
	return out
}

func mustMarshalMap(m datatypes.JSONMap) JSON {
	b, _ := json.Marshal(m)
	return JSON(b)
}

// FindByScopeAndFunctionID fetches a contract by scope and function ID.
func (m *FunctionContractModel) FindByScopeAndFunctionID(ctx context.Context, gameID, env, functionID string) (*FunctionContract, error) {
	var contract FunctionContract
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
		First(&contract).Error; err != nil {
		return nil, err
	}
	return &contract, nil
}

// ListByScope lists all contracts in a scope.
func (m *FunctionContractModel) ListByScope(ctx context.Context, gameID, env string) ([]*FunctionContract, error) {
	var contracts []*FunctionContract
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("function_id").
		Find(&contracts).Error; err != nil {
		return nil, err
	}
	return contracts, nil
}

// ListByResourceKey lists contracts for a resource.
func (m *FunctionContractModel) ListByResourceKey(ctx context.Context, gameID, env, resourceKey string) ([]*FunctionContract, error) {
	var contracts []*FunctionContract
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, resourceKey).
		Order("function_id").
		Find(&contracts).Error; err != nil {
		return nil, err
	}
	return contracts, nil
}

// DeleteByScopeAndFunctionID removes a contract.
func (m *FunctionContractModel) DeleteByScopeAndFunctionID(ctx context.Context, gameID, env, functionID string) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
		Delete(&FunctionContract{}).Error
}

// ResourceCapabilityModel wraps data access for resource capabilities.
type ResourceCapabilityModel struct {
	db *gorm.DB
}

// NewResourceCapabilityModel creates the helper.
func NewResourceCapabilityModel(db *gorm.DB) *ResourceCapabilityModel {
	return &ResourceCapabilityModel{db: db}
}

// UpsertCapability creates or updates a resource capability.
func (m *ResourceCapabilityModel) UpsertCapability(ctx context.Context, cap *ResourceCapability) error {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var existing ResourceCapability
	err := db.Unscoped().Where("game_id = ? AND env = ? AND resource_key = ?",
		cap.GameID, cap.Env, cap.ResourceKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(cap).Error
	}
	if err != nil {
		return err
	}
	cap.ID = existing.ID
	cap.CreatedAt = existing.CreatedAt
	cap.DeletedAt = gorm.DeletedAt{}
	return db.Save(cap).Error
}

// FindByScopeAndResourceKey fetches by scope and resource key.
func (m *ResourceCapabilityModel) FindByScopeAndResourceKey(ctx context.Context, gameID, env, resourceKey string) (*ResourceCapability, error) {
	var cap ResourceCapability
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, resourceKey).
		First(&cap).Error; err != nil {
		return nil, err
	}
	return &cap, nil
}

// ListByScope lists all resource capabilities in a scope.
func (m *ResourceCapabilityModel) ListByScope(ctx context.Context, gameID, env string) ([]*ResourceCapability, error) {
	var caps []*ResourceCapability
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("resource_key").
		Find(&caps).Error; err != nil {
		return nil, err
	}
	return caps, nil
}

// DeleteByScopeAndResourceKey removes the derived capability when no
// executable contracts remain for the resource in this scope.
func (m *ResourceCapabilityModel) DeleteByScopeAndResourceKey(ctx context.Context, gameID, env, resourceKey string) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, resourceKey).
		Delete(&ResourceCapability{}).Error
}

// CapabilitySemanticsModel wraps data access for capability semantics.
type CapabilitySemanticsModel struct {
	db *gorm.DB
}

// NewCapabilitySemanticsModel creates the helper.
func NewCapabilitySemanticsModel(db *gorm.DB) *CapabilitySemanticsModel {
	return &CapabilitySemanticsModel{db: db}
}

// UpsertSemantics creates or updates capability semantics.
func (m *CapabilitySemanticsModel) UpsertSemantics(ctx context.Context, sem *CapabilitySemantics) error {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var existing CapabilitySemantics
	err := db.Unscoped().Where("game_id = ? AND env = ? AND resource_key = ?",
		sem.GameID, sem.Env, sem.ResourceKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		sem.Version = 1
		return db.Create(sem).Error
	}
	if err != nil {
		return err
	}
	sem.ID = existing.ID
	sem.CreatedAt = existing.CreatedAt
	sem.DeletedAt = gorm.DeletedAt{}
	sem.Version = existing.Version + 1
	return db.Save(sem).Error
}

// FindByScopeAndResourceKey fetches by scope and resource key.
func (m *CapabilitySemanticsModel) FindByScopeAndResourceKey(ctx context.Context, gameID, env, resourceKey string) (*CapabilitySemantics, error) {
	var sem CapabilitySemantics
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, resourceKey).
		First(&sem).Error; err != nil {
		return nil, err
	}
	return &sem, nil
}

// ListByScope lists all capability semantics in a scope.
func (m *CapabilitySemanticsModel) ListByScope(ctx context.Context, gameID, env string) ([]*CapabilitySemantics, error) {
	var sems []*CapabilitySemantics
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ?", gameID, env).
		Order("resource_key").
		Find(&sems).Error; err != nil {
		return nil, err
	}
	return sems, nil
}

// Update updates a capability semantics record.
func (m *CapabilitySemanticsModel) Update(ctx context.Context, sem *CapabilitySemantics) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Save(sem).Error
}

// DeleteByScopeAndResourceKey removes the current semantic aggregate when
// the resource has no remaining contracts. It keeps the aggregate row soft
// deleted so a subsequent rebuild restores the same ID and its version history.
func (m *CapabilitySemanticsModel) DeleteByScopeAndResourceKey(ctx context.Context, gameID, env, resourceKey string) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND resource_key = ?", gameID, env, resourceKey).
		Delete(&CapabilitySemantics{}).Error
}

// CapabilitySemanticVersionModel wraps data access for semantic versions.
type CapabilitySemanticVersionModel struct {
	db *gorm.DB
}

// NewCapabilitySemanticVersionModel creates the helper.
func NewCapabilitySemanticVersionModel(db *gorm.DB) *CapabilitySemanticVersionModel {
	return &CapabilitySemanticVersionModel{db: db}
}

// CreateVersion creates a new semantic version record.
func (m *CapabilitySemanticVersionModel) CreateVersion(ctx context.Context, ver *CapabilitySemanticVersion) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).Create(ver).Error
}

// ListBySemanticsID lists versions for a semantics.
func (m *CapabilitySemanticVersionModel) ListBySemanticsID(ctx context.Context, semanticsID uint) ([]*CapabilitySemanticVersion, error) {
	var vers []*CapabilitySemanticVersion
	if err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("semantics_id = ?", semanticsID).
		Order("version DESC").
		Find(&vers).Error; err != nil {
		return nil, err
	}
	return vers, nil
}

// ListBySemanticsIDPaged returns one newest-first page of semantic versions
// plus the total row count, so catalog detail views never materialize the
// full history (automated re-registrations can accumulate thousands of rows).
func (m *CapabilitySemanticVersionModel) ListBySemanticsIDPaged(ctx context.Context, semanticsID uint, limit, offset int) ([]*CapabilitySemanticVersion, int64, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var total int64
	if err := db.Model(&CapabilitySemanticVersion{}).
		Where("semantics_id = ?", semanticsID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var vers []*CapabilitySemanticVersion
	if err := db.Where("semantics_id = ?", semanticsID).
		Order("version DESC").
		Limit(limit).
		Offset(offset).
		Find(&vers).Error; err != nil {
		return nil, 0, err
	}
	return vers, total, nil
}
