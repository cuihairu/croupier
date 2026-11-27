package gameops

import (
	"context"
	"errors"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 凭据状态机的合法迁移表。终态（succeeded/failed/canceled/cancel_failed）
// 不再自动迁移：撤销/补偿永远走新审批 + 新 operationId（D5），已成功凭据
// 不可被任何后续响应改写（防伪造）。
var allowedTransitions = map[string]map[string]bool{
	model.BizOpOutcomePending: {
		model.BizOpOutcomeSucceeded: true, model.BizOpOutcomeFailed: true,
		model.BizOpOutcomeRetryableFailed: true, model.BizOpOutcomeUnknown: true,
		model.BizOpOutcomeCanceled: true, model.BizOpOutcomeCancelFailed: true,
	},
	model.BizOpOutcomeUnknown: {
		model.BizOpOutcomeSucceeded: true, model.BizOpOutcomeFailed: true,
		model.BizOpOutcomeRetryableFailed: true,
		model.BizOpOutcomeCanceled:        true, model.BizOpOutcomeCancelFailed: true,
	},
	model.BizOpOutcomeRetryableFailed: {
		model.BizOpOutcomeSucceeded: true, model.BizOpOutcomeFailed: true,
		model.BizOpOutcomeUnknown:  true,
		model.BizOpOutcomeCanceled: true, model.BizOpOutcomeCancelFailed: true,
	},
	model.BizOpOutcomeSucceeded:    {},
	model.BizOpOutcomeFailed:       {},
	model.BizOpOutcomeCanceled:     {},
	model.BizOpOutcomeCancelFailed: {},
}

// ErrFingerprintConflict 是 R5 参数漂移判定：同 operationId 携带不同参数
// （不同指纹）的重放被拒绝——审批通过的意图不得被静默改写。
var ErrFingerprintConflict = errors.New("operationId already registered with a different request fingerprint")

// ErrCredentialNotFound 表示凭据不存在：查询收敛/取消确认前必须先有凭据行。
var ErrCredentialNotFound = errors.New("business operation credential not found")

// CredentialStore 是 business_operation_credentials 的访问层（DB 句柄按请求
// context 解析，multiGame 模式下路由到对应物理库）。
type CredentialStore struct {
	db *gorm.DB
}

func NewCredentialStore(db *gorm.DB) *CredentialStore {
	return &CredentialStore{db: db}
}

// Register 原子登记业务意图凭据（CreateOrGet）：
//   - 无同 ID 行 → 插入 pending 凭据（携带参数快照 payload，D4 自动重试依据），
//     返回 (credential, created=true)；
//   - 同 ID 且指纹一致 → 返回既有凭据（created=false）——响应丢失后的重试
//     携带同一 operationId 命中同一行，游戏侧据此去重（R1/R3）；快照保持
//     首次登记值不覆盖；
//   - 同 ID 但指纹不同 → ErrFingerprintConflict（R5 参数漂移拒绝）。
func (s *CredentialStore) Register(ctx context.Context, gameID, env, operationID, functionID, fingerprint string, payload []byte, actor, queryHint, cancelHint string) (*model.BusinessOperationCredential, bool, error) {
	if err := ValidateOperationID(operationID); err != nil {
		return nil, false, err
	}
	domain, _, err := ParseOperationID(operationID)
	if err != nil {
		return nil, false, err
	}
	db := dbctx.Resolve(ctx, s.db)
	now := time.Now().UTC()
	rec := &model.BusinessOperationCredential{
		GameID:             gameID,
		Env:                env,
		OperationID:        operationID,
		Domain:             domain,
		FunctionID:         functionID,
		RequestFingerprint: fingerprint,
		RequestPayload:     model.JSON(payload),
		Outcome:            model.BizOpOutcomePending,
		QueryHint:          queryHint,
		CancelHint:         cancelHint,
		Actor:              actor,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	// 唯一索引命中不报错：OnConflict DoNothing 后 ID 为 0 → 读现存行再判定。
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(rec).Error; err != nil {
		return nil, false, err
	}
	if rec.ID != 0 {
		return rec, true, nil
	}
	existing, err := s.Get(ctx, gameID, env, operationID)
	if err != nil {
		return nil, false, err
	}
	if existing.RequestFingerprint != fingerprint {
		return existing, false, ErrFingerprintConflict
	}
	return existing, false, nil
}

// Get 按 scope + operationId 读凭据。
func (s *CredentialStore) Get(ctx context.Context, gameID, env, operationID string) (*model.BusinessOperationCredential, error) {
	db := dbctx.Resolve(ctx, s.db)
	var rec model.BusinessOperationCredential
	err := db.WithContext(ctx).
		Where("game_id = ? AND env = ? AND operation_id = ?", gameID, env, operationID).
		First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCredentialNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// ApplyEnvelope 以游戏信封驱动凭据状态迁移（D3/D4）：outcome 一律来自
// 游戏方权威分类，平台不发明状态。信封身份与凭据不一致按错误拒绝，不改行。
// 返回迁移后的凭据；迁移被状态机拒绝时返回既有行与 false（终态保护）。
func (s *CredentialStore) ApplyEnvelope(ctx context.Context, gameID, env, operationID string, envelope *GameOperationResult) (*model.BusinessOperationCredential, bool, error) {
	if envelope == nil {
		return nil, false, errors.New("nil envelope")
	}
	if envelope.OperationID != operationID {
		return nil, false, errors.New("envelope operationId does not match credential")
	}
	existing, err := s.Get(ctx, gameID, env, operationID)
	if err != nil {
		return nil, false, err
	}
	next := envelope.Outcome
	if !allowedTransitions[existing.Outcome][next] {
		return existing, false, nil
	}
	db := dbctx.Resolve(ctx, s.db)
	updates := map[string]interface{}{
		"outcome":    next,
		"updated_at": time.Now().UTC(),
	}
	// 回执/结果只在有新内容时覆盖；cancel_failed 保留原 result（D5：
	// 取消不谎称回滚，补偿引用原凭据）。
	if envelope.GameReceipt != "" {
		updates["game_receipt"] = envelope.GameReceipt
	}
	if len(envelope.Result) > 0 {
		updates["result_json"] = envelope.Result
	}
	if envelope.QueryHint != "" {
		updates["query_hint"] = envelope.QueryHint
	}
	result := db.WithContext(ctx).Model(&model.BusinessOperationCredential{}).
		Where("game_id = ? AND env = ? AND operation_id = ? AND outcome = ?", gameID, env, operationID, existing.Outcome).
		Updates(updates)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		// 并发迁移：条件更新未命中（他人已改 outcome），回读终值。
		latest, err := s.Get(ctx, gameID, env, operationID)
		return latest, false, err
	}
	updated, err := s.Get(ctx, gameID, env, operationID)
	if err != nil {
		return nil, false, err
	}
	return updated, true, nil
}

// MarkUnknownOnTransportFailure 在执行链无权威信封返回（传输失败/超时）时
// 落 unknown——绝不落 failed（R2：未知误判失败会诱发重复发放）。凭据不存在
// 时登记占位凭据（指纹必须与登记时一致）。
func (s *CredentialStore) MarkUnknownOnTransportFailure(ctx context.Context, gameID, env, operationID, functionID, fingerprint string, payload []byte, actor string) (*model.BusinessOperationCredential, error) {
	rec, created, err := s.Register(ctx, gameID, env, operationID, functionID, fingerprint, payload, actor, "", "")
	if err != nil {
		return nil, err
	}
	if created && rec.Outcome == model.BizOpOutcomePending {
		// 全新登记行直接落 unknown；既有行经状态机迁移。
		db := dbctx.Resolve(ctx, s.db)
		if err := db.WithContext(ctx).Model(&model.BusinessOperationCredential{}).
			Where("game_id = ? AND env = ? AND operation_id = ? AND outcome = ?", gameID, env, operationID, model.BizOpOutcomePending).
			Updates(map[string]interface{}{
				"outcome":    model.BizOpOutcomeUnknown,
				"updated_at": time.Now().UTC(),
			}).Error; err != nil {
			return nil, err
		}
		return s.Get(ctx, gameID, env, operationID)
	}
	if rec.Outcome == model.BizOpOutcomePending {
		return s.applyOutcome(ctx, gameID, env, operationID, rec.Outcome, model.BizOpOutcomeUnknown)
	}
	return rec, nil
}

// applyOutcome 是无信封场景的内部迁移（同 ApplyEnvelope 的状态机保护）。
func (s *CredentialStore) applyOutcome(ctx context.Context, gameID, env, operationID, from, to string) (*model.BusinessOperationCredential, error) {
	db := dbctx.Resolve(ctx, s.db)
	result := db.WithContext(ctx).Model(&model.BusinessOperationCredential{}).
		Where("game_id = ? AND env = ? AND operation_id = ? AND outcome = ?", gameID, env, operationID, from).
		Updates(map[string]interface{}{"outcome": to, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return nil, result.Error
	}
	return s.Get(ctx, gameID, env, operationID)
}

// BumpQueryAttempts / BumpRetryAttempts 计量 D4 自动查询（≤10）与自动重试
// （≤8）；达到上限的凭据由扫描器转入告警路径，不再自动处理。
func (s *CredentialStore) BumpQueryAttempts(ctx context.Context, gameID, env, operationID string) (int, error) {
	return s.bumpAttempts(ctx, gameID, env, operationID, "query_attempts")
}

func (s *CredentialStore) BumpRetryAttempts(ctx context.Context, gameID, env, operationID string) (int, error) {
	return s.bumpAttempts(ctx, gameID, env, operationID, "retry_attempts")
}

func (s *CredentialStore) bumpAttempts(ctx context.Context, gameID, env, operationID, column string) (int, error) {
	db := dbctx.Resolve(ctx, s.db)
	result := db.WithContext(ctx).Model(&model.BusinessOperationCredential{}).
		Where("game_id = ? AND env = ? AND operation_id = ?", gameID, env, operationID).
		Updates(map[string]interface{}{
			column:       gorm.Expr(column + " + 1"),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, ErrCredentialNotFound
	}
	rec, err := s.Get(ctx, gameID, env, operationID)
	if err != nil {
		return 0, err
	}
	if column == "query_attempts" {
		return rec.QueryAttempts, nil
	}
	return rec.RetryAttempts, nil
}

// ListConvergeable 列出仍需自动收敛的凭据：unknown（查询收敛）与
// retryable_failed（自动重试），且各自计量未达上限。扫描器按 updated_at
// 排序处理，不负责限流（并发上限/健康门控由调用方策略执行）。
func (s *CredentialStore) ListConvergeable(ctx context.Context, gameID, env string, limit int) ([]model.BusinessOperationCredential, error) {
	db := dbctx.Resolve(ctx, s.db)
	var rows []model.BusinessOperationCredential
	query := db.WithContext(ctx).
		Where("(outcome = ? AND query_attempts < ?) OR (outcome = ? AND retry_attempts < ?)",
			model.BizOpOutcomeUnknown, model.BizOpQueryAttemptsMax,
			model.BizOpOutcomeRetryableFailed, model.BizOpRetryAttemptsMax)
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}
	err := query.Order("updated_at ASC").Limit(limit).Find(&rows).Error
	return rows, err
}
