package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm/clause"
)

// SetAgentDraining 设置/清除 agent 的 draining 路由门（ops 侧显式状态）。
// 返回 agent 是否在册（未找到时无操作并返回 false，调用方据此决定是否
// 回 404，与 opsNodeDrain 的 ResolveAgentConn 存在性检查互补）。
func (s *Store) SetAgentDraining(agentID string, draining bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	agent, ok := s.agents[agentID]
	if !ok || agent == nil {
		return false
	}
	agent.Draining = draining
	return true
}

func (s *Store) previousAgentSession(agentID string) *AgentSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneAgentSession(s.agents[agentID])
}

// PreviousAgentSession returns the current in-memory session snapshot. It
// allows callers to enforce a stable agent-to-scope binding before replacing
// a session.
func (s *Store) PreviousAgentSession(agentID string) *AgentSession {
	return s.previousAgentSession(agentID)
}

func cloneAgentSession(session *AgentSession) *AgentSession {
	if session == nil {
		return nil
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		return nil
	}
	var clone AgentSession
	// encoded 是 json.Marshal 的产物，对同一结构再 Unmarshal 恒成功，
	// err 分支为死代码已删（Marshal 的 err 路径因 ProviderSession.OpenAPIDoc
	// 为 json.RawMessage 可失败，可达需保留）。
	_ = json.Unmarshal(encoded, &clone)
	return &clone
}

// PreviousFunctionSchema 返回该 agent 当前会话中函数的上一次注册
// input/output schema（同 scope），用于注册时 schema 兼容性 diff。
// 会话不存在 / scope 不一致 / 函数不在会话中均返回 ok=false（视为首次注册）。
func (s *Store) PreviousFunctionSchema(agentID, gameID, env, functionID string) (inputSchema, outputSchema string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.agents[agentID]
	if current == nil || current.GameID != gameID || current.Env != env {
		return "", "", false
	}
	meta, exists := current.Functions[functionID]
	if !exists {
		return "", "", false
	}
	return meta.InputSchema, meta.OutputSchema, true
}

func (s *Store) survivingFunctionMeta(agentID, gameID, env, functionID string) (FunctionMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agentIDs := make([]string, 0, len(s.agents))
	for id := range s.agents {
		agentIDs = append(agentIDs, id)
	}
	sort.Strings(agentIDs)
	for _, id := range agentIDs {
		if id == agentID {
			continue
		}
		session := s.agents[id]
		if session == nil || session.GameID != gameID || session.Env != env {
			continue
		}
		meta, ok := session.Functions[functionID]
		if ok {
			return meta, true
		}
	}
	return FunctionMeta{}, false
}

func (s *Store) rebuildContext(gameID, env string) context.Context {
	if s != nil && s.scopeContext != nil {
		if ctx := s.scopeContext(gameID, env); ctx != nil {
			return ctx
		}
	}
	return defaultScopeContext(gameID, env)
}

func defaultScopeContext(string, string) context.Context {
	return context.Background()
}

// writeToDB persists agent session to database for recovery purposes.
// Only called when db is enabled.
func (s *Store) writeToDB(ctx context.Context, a *AgentSession) error {
	if s.db == nil {
		return nil
	}

	// Convert to database model — initialise JSON columns to valid empty
	// JSON so nil maps/slices don't produce empty strings that PostgreSQL's
	// json type rejects.
	dbSess := struct {
		AgentID   string
		GameID    string
		Env       string
		Version   string
		Addr      string
		Region    string
		Zone      string
		Labels    string `gorm:"type:text"`
		Functions string `gorm:"type:text"`
		Providers string `gorm:"type:text"`
		ExpireAt  time.Time
		LastSeen  time.Time
	}{
		AgentID:   a.AgentID,
		GameID:    a.GameID,
		Env:       a.Env,
		Version:   a.Version,
		Addr:      a.Addr,
		Region:    a.Region,
		Zone:      a.Zone,
		ExpireAt:  a.ExpireAt,
		LastSeen:  a.LastSeen,
		Labels:    "{}",
		Functions: "{}",
		Providers: "[]",
	}

	// Marshal Labels to JSON
	if a.Labels != nil {
		if labelsJSON, err := json.Marshal(a.Labels); err == nil {
			dbSess.Labels = string(labelsJSON)
		}
	}

	// Marshal Functions to JSON
	if a.Functions != nil {
		if functionsJSON, err := json.Marshal(a.Functions); err == nil {
			dbSess.Functions = string(functionsJSON)
		}
	}

	// Marshal Providers to JSON
	if a.Providers != nil {
		if providersJSON, err := json.Marshal(a.Providers); err == nil {
			dbSess.Providers = string(providersJSON)
		}
	}

	// Perform upsert using GORM
	return dbctx.Resolve(ctx, s.db).WithContext(ctx).
		Table("agent_sessions").
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "agent_id"}},
			UpdateAll: true,
		}).
		Create(&dbSess).Error
}

// ========== OpenAPI 3.0.3 Methods (replaces legacy manifest) ==========

// AgentSessionLoader defines the interface for loading agent sessions from database.
type AgentSessionLoader interface {
	LoadActiveSessions(ctx context.Context) ([]*AgentSession, error)
}

// RemoveAgentIfStale 删除 agent 的注册会话（断连/对账清理）。notAfter
// 之后的新注册或心跳（两者都刷新 LastSeen）会中止删除——断连清理与
// 瞬间重连注册的竞态里不能删掉新会话。返回是否实际删除。
func (s *Store) RemoveAgentIfStale(agentID string, notAfter time.Time) bool {
	if agentID == "" {
		return false
	}
	s.mu.Lock()
	cur := s.agents[agentID]
	if cur == nil {
		s.mu.Unlock()
		return false
	}
	if cur.LastSeen.After(notAfter) {
		s.mu.Unlock()
		return false
	}
	delete(s.agents, agentID)
	s.mu.Unlock()
	// 注意：断连摘除不删 provider 元数据行（#11）——HA 下 agent 断 A 连 B
	// 时，B 的注册可能先于 A 的断连清理落行，此处再删会误删存活实例的
	// 元数据。行随会话过期清理（ExpireAt 过期的 agent 必然全量重注册，
	// 重注册先行刷新行，无竞态窗口），滞后有界。
	return true
}

// LoadFromDB loads active agent sessions from the database and populates the in-memory store.
func (s *Store) LoadFromDB(ctx context.Context, loader AgentSessionLoader) error {
	return s.LoadFromDBFiltered(ctx, loader, nil)
}

// LoadFromDBFiltered 按 keep 过滤灌回快照行；keep == nil 等价全量恢复
// （单实例/归属目录不可用时的语义）。
func (s *Store) LoadFromDBFiltered(ctx context.Context, loader AgentSessionLoader, keep func(*AgentSession) bool) error {
	if s.db == nil {
		return fmt.Errorf("database not enabled")
	}
	if loader == nil {
		return fmt.Errorf("agent session loader is required")
	}

	sessions, err := loader.LoadActiveSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to load sessions: %w", err)
	}

	s.mu.Lock()
	for _, sess := range sessions {
		if keep != nil && !keep(sess) {
			continue
		}
		s.agents[sess.AgentID] = sess
	}
	s.mu.Unlock()

	if err := s.recoverPendingRegistrationOperations(ctx); err != nil {
		return err
	}

	slog.Info("loaded agent sessions from database", "count", len(sessions))
	return nil
}

func (s *Store) recoverPendingRegistrationOperations(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	if s.contractService == nil {
		return nil
	}
	var operations []AgentRegistrationOperationDB
	if err := s.db.WithContext(ctx).
		Where("status IN ?", []string{"pending", "compensation_required"}).
		Order("created_at, id").
		Find(&operations).Error; err != nil {
		return fmt.Errorf("list pending registration recovery operations: %w", err)
	}
	for _, operation := range operations {
		previous, err := decodeRegistrationSession(operation.PreviousSession)
		if err != nil {
			s.markRegistrationOperation(operation.OperationID, "compensation_required", err)
			return fmt.Errorf("decode previous registration session %s: %w", operation.OperationID, err)
		}
		target, err := decodeRegistrationSession(operation.TargetSession)
		if err != nil || target == nil {
			if err == nil {
				err = fmt.Errorf("target session is empty")
			}
			s.markRegistrationOperation(operation.OperationID, "compensation_required", err)
			return fmt.Errorf("decode target registration session %s: %w", operation.OperationID, err)
		}
		if previous == nil {
			previous = &AgentSession{
				AgentID:   target.AgentID,
				GameID:    target.GameID,
				Env:       target.Env,
				Functions: map[string]FunctionMeta{},
			}
		}
		if previous.AgentID == "" {
			previous.AgentID = target.AgentID
		}
		if previous.GameID == "" {
			previous.GameID = target.GameID
		}
		if previous.Env == "" {
			previous.Env = target.Env
		}

		scopeCtx := s.rebuildContext(target.GameID, target.Env)
		reverseDiff := classifyFunctionSnapshot(target.Functions, previous.Functions)
		if _, err := s.materializeScopedTransaction(scopeCtx, previous, s.materializeAgent, reverseDiff); err != nil {
			s.markRegistrationOperation(operation.OperationID, "compensation_required", err)
			return fmt.Errorf("recover registration operation %s: %w", operation.OperationID, err)
		}
		s.markRegistrationOperation(operation.OperationID, "compensated", nil)
		s.regenTemplatesAfterRegistration(scopeCtx, previous, reverseDiff)
	}
	return nil
}

func decodeRegistrationSession(raw string) (*AgentSession, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var session AgentSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// StartCleanupRoutine 启动后台清理过期 Session 的 goroutine
// 定期从内存中删除过期的 AgentSession，保持内存数据有效
func (s *Store) StartCleanupRoutine(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Minute // 默认每分钟清理一次
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("Registry cleanup routine stopped")
				return
			case <-ticker.C:
				s.cleanupExpiredSessions()
			}
		}
	}()

	slog.Info("Started registry cleanup routine", "interval", interval)
}

// cleanupExpiredSessions 清理过期的 AgentSession
func (s *Store) cleanupExpiredSessions() {
	s.mu.Lock()

	now := time.Now()
	expiredCount := 0
	// 过期会话的 provider 元数据行随会话清理（#11）：锁外做 DB 删除，
	// 避免在注册锁内做慢 I/O。
	expiredSessions := make([]*AgentSession, 0)

	for agentID, sess := range s.agents {
		if sess == nil {
			continue
		}

		// 检查是否过期
		if sess.ExpireAt.Before(now) {
			delete(s.agents, agentID)
			expiredCount++
			if len(sess.Providers) > 0 {
				expiredSessions = append(expiredSessions, sess)
			}

			slog.Debug("Cleaned up expired agent session", "agent_id", agentID, "expired_at", sess.ExpireAt.Format(time.RFC3339))
		}
	}

	s.mu.Unlock()

	for _, sess := range expiredSessions {
		s.deleteProviderMetadataForServices(sess)
	}

	if expiredCount > 0 {
		slog.Info("Cleaned up expired agent sessions", "expired_count", expiredCount, "remaining", len(s.agents))
	}
}
