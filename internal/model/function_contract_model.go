package model

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
	// ExecutionState 空值归一为 bound（spec.ExecutionStateBound）：列默认
	// default:'bound' 使零值落库即 bound，不归一的话零值重注册与库中
	// bound 行在 contractSemanticallyEqual（T6 纳入比较）里恒不等，
	// 「内容无变化跳过写」失效。显式 unbound（上传物料）与状态翻转
	// （T6 自动绑定）不受影响。
	if strings.TrimSpace(contract.ExecutionState) == "" {
		contract.ExecutionState = "bound"
	}
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

// ContractSemanticallyEqual 暴露 UpsertContract 的「内容无变化」判据：
// 版本历史写路径（B2）复用同一函数，保证「跳过写」与「跳过历史」永不漂移。
func ContractSemanticallyEqual(a, b *FunctionContract) bool {
	return contractSemanticallyEqual(a, b)
}

// contractSemanticallyEqual 比较契约全部字段（含展示层 Summary/
// Description/Tags——文案变化需要写入，e2e 契约链路依赖 proposal
// 反映新文案）；schema 用 canonical JSON（键序/空格形态差异不算
// 变化——六语言 SDK 各自序列化同 schema 字节不同），JSONMap 走
// 零值归一（nil 与 {required:false} 等价）。Diagnostics 属注册期
// 质检快照，轮换不构成契约变化，不参与。ExecutionState 参与比较
// （D2/T6）：执行状态翻转（unbound→bound 自动绑定）必须落库，不能
// 被「内容无变化」跳过——但它在 digest 序列里被 json:"-" 排除，翻转
// 不会扰动下游 freshness/proposal。
func contractSemanticallyEqual(a, b *FunctionContract) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Version == b.Version &&
		a.Enabled == b.Enabled &&
		a.Deprecated == b.Deprecated &&
		a.ExecutionState == b.ExecutionState &&
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
	// v 是 json.Unmarshal 的产物，仅含 JSON 基础类型，对其 Marshal 恒成功，
	// err 分支（回退原字节）为死代码已删。
	out, _ := json.Marshal(v)
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

// MarkRemovalPending 标记摘除宽限起点（函数从注册中消失时调用）。
// 返回是否存在被标记的行——行已不存在时 false（调用方走即时删除路径
// 也无对象）。已 pending 的行重复标记会刷新时间点：连续缺席只算最新
// 一次，宽限从最后一次消失起算。用 UpdateColumn 而非 Update：gorm 的
// Update 会自动触碰 updated_at，而 computeDigest 整行序列化含
// updated_at——触碰会让宽限内首轮提案重建误判「契约已变化」，把
// accepted 提案打回 pending（与宽限「零版本噪音」目标相抵）。
func (m *FunctionContractModel) MarkRemovalPending(ctx context.Context, gameID, env, functionID string) (bool, error) {
	res := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Model(&FunctionContract{}).
		Where("game_id = ? AND env = ? AND function_id = ?", gameID, env, functionID).
		UpdateColumn("removal_pending_at", time.Now())
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ClearRemovalPending 清除摘除宽限标记（宽限期内重注册）。无 pending
// 时是零成本 no-op（WHERE 条件兜底）。UpdateColumn 理由同 MarkRemovalPending。
func (m *FunctionContractModel) ClearRemovalPending(ctx context.Context, gameID, env, functionID string) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Model(&FunctionContract{}).
		Where("game_id = ? AND env = ? AND function_id = ? AND removal_pending_at IS NOT NULL", gameID, env, functionID).
		UpdateColumn("removal_pending_at", nil).Error
}

// ListExpiredPendingRemovals 列出宽限已到期的 pending 行（清扫器候选项）。
// limit>0 时按 removal_pending_at 最旧优先截断（清扫每轮批量认领，剩余
// 候选下一轮继续——未到期的量不该拖垮单轮预算）；limit<=0 不限量。
func (m *FunctionContractModel) ListExpiredPendingRemovals(ctx context.Context, cutoff time.Time, limit int) ([]*FunctionContract, error) {
	q := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("removal_pending_at IS NOT NULL AND removal_pending_at < ?", cutoff).
		Order("removal_pending_at ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var rows []*FunctionContract
	err := q.Find(&rows).Error
	return rows, err
}

// DeleteIfRemovalPending 条件删除：仅当行仍处于 pending 时删除成功。
// 这是 HA 多实例清扫的原子认领——两个实例同时 finalize 同一行时只有一个
// DELETE 命中（另一个 RowsAffected=0 跳过），removed 版本历史不会双写；
// 与重注册的竞态同理：宽限内重注册已清除 pending，条件删除命中不了，
// 刚回来的函数不会被误删。
func (m *FunctionContractModel) DeleteIfRemovalPending(ctx context.Context, gameID, env, functionID string) (bool, error) {
	res := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND function_id = ? AND removal_pending_at IS NOT NULL", gameID, env, functionID).
		Delete(&FunctionContract{})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
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
//
// 返回 changed 表示语义稳定内容是否发生变化。agent 重注册会以相同内容反复
// upsert 同一行（BUG-031 第三病灶）：内容未变时不得 bump Version、不得触碰
// 行（否则 Version 无条件 +1 会被下游 digest 投影吃进去，每次重连灌一版
// 提案快照）。只有真实内容变化才递增版本并落库。
func (m *CapabilitySemanticsModel) UpsertSemantics(ctx context.Context, sem *CapabilitySemantics) (bool, error) {
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx)
	var existing CapabilitySemantics
	err := db.Unscoped().Where("game_id = ? AND env = ? AND resource_key = ?",
		sem.GameID, sem.Env, sem.ResourceKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		sem.Version = 1
		return true, db.Create(sem).Error
	}
	if err != nil {
		return false, err
	}
	if existing.StableContentDigest() == sem.StableContentDigest() {
		// 内容未变：保持行原样（Version/UpdatedAt 都不动），并回填行标识，
		// 让调用方拿到的是持久化行的真实状态。
		sem.ID = existing.ID
		sem.CreatedAt = existing.CreatedAt
		sem.DeletedAt = existing.DeletedAt
		sem.Version = existing.Version
		sem.UpdatedAt = existing.UpdatedAt
		return false, nil
	}
	sem.ID = existing.ID
	sem.CreatedAt = existing.CreatedAt
	sem.DeletedAt = gorm.DeletedAt{}
	sem.Version = existing.Version + 1
	return true, db.Save(sem).Error
}

// normalizeJSONContent 归一 JSON 列内容用于内容比较：jsonb 回读的字节序/空白
// 漂移不代表内容变化（BUG-031 同病类），Unmarshal→Marshal 输出稳定形态。
func normalizeJSONContent(raw JSON) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		// 非法 JSON（不应出现）：按原文字节比较，不掩盖坏数据。
		return string(raw)
	}
	return v
}

// stripVolatileKeys 递归剔除 JSON 树里的生命周期元数据键（updatedAt/updatedBy）。
// Provenance 条目在每次重建时刷新时间戳（normalizer/provenance.go:75/85/103/180
// `UpdatedAt: time.Now()`），这些是「何时写入」的行级元数据而非语义内容，
// 必须在内容比较前剥除（BUG-031 第四病灶：Provenance 内嵌时间戳致 digest
// 永不等，每次重连/重建灌版）。
func stripVolatileKeys(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, child := range t {
			if k == "updatedAt" || k == "updatedBy" {
				delete(t, k)
				continue
			}
			t[k] = stripVolatileKeys(child)
		}
		return t
	case []interface{}:
		for i, child := range t {
			t[i] = stripVolatileKeys(child)
		}
		return t
	default:
		return v
	}
}

// normalizeProvenance = normalizeJSONContent + 剥除 Provenance 里的生命周期
// 键；条目的实质内容（field/value/source/status/confidence/sourceDigest）
// 全部保留参与变化判定。
func normalizeProvenance(raw JSON) interface{} {
	return stripVolatileKeys(normalizeJSONContent(raw))
}

// StableContentDigest 返回语义**稳定内容**的摘要：剔除 ID/时间戳/UpdatedBy/
// Version 等行生命周期元数据。Version 是行版本计数（历史实现里内容未变也
// 自增），不是语义内容，绝不能参与变化判定；语义内容变化必然反映在其余
// 投影字段上。service 层的 semanticsComparableDigest 委托本方法。
func (sem *CapabilitySemantics) StableContentDigest() string {
	payload, err := json.Marshal(struct {
		IdentityField     string
		IdentityFieldType string
		IdentityPath      string
		CollectionQueryID uint
		CollectionPath    string
		PageFieldName     string
		PageSizeFieldName string
		ItemsFieldName    string
		TotalFieldName    string
		ItemQueryID       uint
		ItemPath          string
		CreateID          uint
		UpdateID          uint
		DeleteID          uint
		Actions           interface{}
		Tasks             interface{}
		Reports           interface{}
		Source            string
		SourceDigest      string
		Diagnostics       interface{}
		Provenance        interface{}
		Conflicts         interface{}
	}{
		IdentityField:     sem.IdentityField,
		IdentityFieldType: sem.IdentityFieldType,
		IdentityPath:      sem.IdentityPath,
		CollectionQueryID: sem.CollectionQueryID,
		CollectionPath:    sem.CollectionPath,
		PageFieldName:     sem.PageFieldName,
		PageSizeFieldName: sem.PageSizeFieldName,
		ItemsFieldName:    sem.ItemsFieldName,
		TotalFieldName:    sem.TotalFieldName,
		ItemQueryID:       sem.ItemQueryID,
		ItemPath:          sem.ItemPath,
		CreateID:          sem.CreateID,
		UpdateID:          sem.UpdateID,
		DeleteID:          sem.DeleteID,
		Actions:           normalizeJSONContent(sem.Actions),
		Tasks:             normalizeJSONContent(sem.Tasks),
		Reports:           normalizeJSONContent(sem.Reports),
		Source:            sem.Source,
		SourceDigest:      sem.SourceDigest,
		Diagnostics:       normalizeJSONContent(sem.Diagnostics),
		Provenance:        normalizeProvenance(sem.Provenance),
		Conflicts:         normalizeJSONContent(sem.Conflicts),
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
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
