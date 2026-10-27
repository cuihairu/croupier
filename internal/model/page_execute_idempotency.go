package model

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PageExecuteIdempotency 记录页面绑定执行的服务端幂等保证（D4 安全重试）：
// 浏览器以 Idempotency-Key 请求头标记「一次执行意图」，同一 key 在保留窗口内
// 只对应一次真实执行——已完成的重放存储结果，进行中的判为结果未知拒绝重复
// 下发，超窗的遗留行可由同 key 请求回收重跑。
//
// 与审批单/任务各自的 IdempotencyKey（只标记来源、不强制唯一）不同，本表的
// (game_id, env, page_key, binding_id, idempotency_key) 是唯一索引，是执行去重
// 的判据；request_hash 区分「同 key 但请求内容不同」的键误复用。
type PageExecuteIdempotency struct {
	ID uint `gorm:"primaryKey"`
	// 唯一键作用域：(game_id, env, page_key, binding_id, idempotency_key)。
	// 复合唯一索引是幂等判定的物理保证（Begin 的插入冲突即同 key 已占位）；
	// 同名字面键在不同游戏/页面间互不影响。
	GameID         string `gorm:"size:64;not null;uniqueIndex:idx_page_execute_idem_scope_key,priority:1"`
	Env            string `gorm:"size:64;not null;uniqueIndex:idx_page_execute_idem_scope_key,priority:2"`
	PageKey        string `gorm:"size:255;not null;uniqueIndex:idx_page_execute_idem_scope_key,priority:3"`
	BindingID      string `gorm:"size:255;not null;uniqueIndex:idx_page_execute_idem_scope_key,priority:4"`
	IdempotencyKey string `gorm:"size:128;not null;uniqueIndex:idx_page_execute_idem_scope_key,priority:5"`
	// RequestHash 是 (pageKey, 页面版本, bindingID, functionID, payload) 的
	// SHA-256：同一 key 复用给不同请求内容时据此判 409。
	RequestHash string `gorm:"size:64;not null"`
	// Status：running=已占位、副作用尚未确认落定；completed=已回写结果。
	Status string `gorm:"size:32;not null;index"`
	// Result 是 spec.PageExecutionResult 的 JSON（completed 行非空）。
	Result JSON `gorm:"type:json"`
	// StartedAt 是本次占位开始时间；running 行超过 idempotencyStaleWindow
	// 视为遗留（进程崩溃、连接中断），可由同 key 请求回收重跑。
	StartedAt   time.Time  `gorm:"not null;index"`
	CompletedAt *time.Time `gorm:"index"`
	CreatedAt   time.Time  `gorm:"not null"`
	UpdatedAt   time.Time  `gorm:"not null"`
	Actor       string     `gorm:"size:128;index"`
	FunctionID  string     `gorm:"size:128;index"`
}

func (PageExecuteIdempotency) TableName() string {
	return "page_execute_idempotencies"
}

// 状态常量与占位窗口。保留窗口到期后 Begin 顺带清扫（惰性、低频），
// 避免补偿式后台任务；未完成的 running 行同样按 started_at 计入保留窗口。
const (
	PageExecuteIdempotencyRunning   = "running"
	PageExecuteIdempotencyCompleted = "completed"
	idempotencyStaleWindow          = 15 * time.Minute
	idempotencyRetentionWindow      = 24 * time.Hour
	idempotencyPurgeInterval        = 10 * time.Minute
)

// isStale 判断 running 行是否已超出占位窗口（副作用落定的可辨识时限）。
func (r *PageExecuteIdempotency) isStale(now time.Time) bool {
	return now.Sub(r.StartedAt) > idempotencyStaleWindow
}

// PageExecuteIdempotencyModel 是幂等占位表访问（DB 句柄按请求 context 解析，
// multiGame 模式下路由到对应物理库）。
type PageExecuteIdempotencyModel struct {
	db *gorm.DB
}

func NewPageExecuteIdempotencyModel(db *gorm.DB) *PageExecuteIdempotencyModel {
	return &PageExecuteIdempotencyModel{db: db}
}

// 惰性清扫节流：至多每 10 分钟一次物理删除，避免每次执行都发 DELETE。
var pageExecuteIdemPurgeAt atomic.Int64

// purgeOlderThan 物理清除超过保留窗口的行（completed 与遗留 running 统一按
// started_at 判界；completed 行的 started_at 必然不晚于 completed_at）。
// 幂等且容忍失败——保留窗口到期不是正确性前提，只是空间治理。
func (m *PageExecuteIdempotencyModel) purgeOlderThan(ctx context.Context) {
	now := time.Now().UnixNano()
	last := pageExecuteIdemPurgeAt.Load()
	if now-last < int64(idempotencyPurgeInterval) {
		return
	}
	if !pageExecuteIdemPurgeAt.CompareAndSwap(last, now) {
		return
	}
	cutoff := time.Now().UTC().Add(-idempotencyRetentionWindow)
	_ = dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Table("page_execute_idempotencies").
		Where("started_at < ?", cutoff).
		Delete(nil).Error
}

// Begin 原子占位幂等键：
//
//   - 无同 key 记录 → 插入 running 占位，返回 owner=true（本次请求执行）；
//   - 同 key 已 completed → 返回 existing（调用方重放 existing.Result）；
//   - 同 key 在占位窗口内仍 running → 返回 existing（结果未知，调用方拒绝）；
//   - 同 key 的 running 已超窗且请求内容相同 → 条件更新回收该行，owner=true；
//   - 同 key 但请求内容不同（request_hash 不一致）→ 返回 existing，调用方拒绝。
//
// 回收按调用方传入的 request_hash 做条件更新（不是现存行的值）：同 key 但
// 请求内容不同的请求不得接管他人在飞行的执行——否则会重跑一次他人请求。
// 条件更新失败（键误复用，或并发占位者同时抢到）即回读重判。
func (m *PageExecuteIdempotencyModel) Begin(
	ctx context.Context,
	gameID, env, pageKey, bindingID, idemKey, requestHash, actor, functionID string,
) (*PageExecuteIdempotency, bool, error) {
	db := dbctx.Resolve(ctx, m.db)
	now := time.Now().UTC()

	rec := &PageExecuteIdempotency{
		GameID:         gameID,
		Env:            env,
		PageKey:        pageKey,
		BindingID:      bindingID,
		IdempotencyKey: idemKey,
		RequestHash:    requestHash,
		Status:         PageExecuteIdempotencyRunning,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
		Actor:          actor,
		FunctionID:     functionID,
	}
	// 冲突不报错：唯一索引命中即转为读现存记录再判定。
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(rec).Error; err != nil {
		return nil, false, err
	}
	if rec.ID != 0 {
		// 插入成功：本次请求占位。占位结果不带行 id——Complete 按唯一键范围
		// 回写，不依赖各方言的主键回填语义。
		m.purgeOlderThan(ctx)
		return nil, true, nil
	}

	existing, err := m.Find(ctx, gameID, env, pageKey, bindingID, idemKey)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		// 理论上唯一索引冲突即记录存在；读不到说明行被并发清扫，交调用方重试。
		return nil, false, errors.New("page execute idempotency: conflicting key not found")
	}
	if existing.Status == PageExecuteIdempotencyCompleted || !existing.isStale(now) {
		return existing, false, nil
	}

	tx := db.WithContext(ctx).Model(&PageExecuteIdempotency{}).
		Where("id = ? AND request_hash = ? AND status = ? AND started_at < ?",
			existing.ID, requestHash, PageExecuteIdempotencyRunning, now.Add(-idempotencyStaleWindow)).
		Updates(map[string]interface{}{
			"started_at": now,
			"updated_at": now,
		})
	if tx.Error != nil {
		return nil, false, tx.Error
	}
	if tx.RowsAffected == 1 {
		m.purgeOlderThan(ctx)
		return nil, true, nil
	}
	// 回收竞争失败（他进程同时接管）：重读按最新状态判定。
	existing, err = m.Find(ctx, gameID, env, pageKey, bindingID, idemKey)
	return existing, false, err
}

// Complete 按唯一键范围回写本次执行结果并关闭占位（幂等重跑只应发生在超窗
// 回收之后）。范围更新而非按 id：占位路径（插入冲突/条件回收）不需要行 id。
func (m *PageExecuteIdempotencyModel) Complete(
	ctx context.Context,
	gameID, env, pageKey, bindingID, idemKey string,
	result JSON,
) error {
	if len(result) == 0 {
		return errors.New("page execute idempotency: empty result to complete")
	}
	db := dbctx.Resolve(ctx, m.db).WithContext(ctx).Model(&PageExecuteIdempotency{}).
		Where("game_id = ? AND env = ? AND page_key = ? AND binding_id = ? AND idempotency_key = ?",
			gameID, env, pageKey, bindingID, idemKey).
		UpdateColumns(map[string]interface{}{
			"status":       PageExecuteIdempotencyCompleted,
			"result":       result,
			"completed_at": time.Now().UTC(),
			"updated_at":   time.Now().UTC(),
		})
	if db.Error != nil {
		return db.Error
	}
	if db.RowsAffected == 0 {
		return errors.New("page execute idempotency: no claim row to complete")
	}
	return nil
}

// Find 按唯一键范围取记录；无记录返回 (nil, nil)。
func (m *PageExecuteIdempotencyModel) Find(
	ctx context.Context,
	gameID, env, pageKey, bindingID, idemKey string,
) (*PageExecuteIdempotency, error) {
	var rec PageExecuteIdempotency
	err := dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("game_id = ? AND env = ? AND page_key = ? AND binding_id = ? AND idempotency_key = ?",
			gameID, env, pageKey, bindingID, idemKey).
		Take(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// DeleteForTest 仅供用例清理占位行（避免并发用例共享库时相互污染）。
func (m *PageExecuteIdempotencyModel) DeleteForTest(ctx context.Context, key string) error {
	return dbctx.Resolve(ctx, m.db).WithContext(ctx).
		Where("idempotency_key = ?", key).
		Delete(&PageExecuteIdempotency{}).Error
}
