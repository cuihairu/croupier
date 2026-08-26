package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var errStoreUnavailable = errors.New("audit store unavailable")

// buildAuditQuery 构造与 GetAuditLogs 完全一致的过滤查询
// （作用域鉴权 + action/actor/ip/game/env/时间窗）。
func (s *Service) buildAuditQuery(ctx context.Context, req *AuditRequest, visibleScopes auditScopeSet, unrestricted bool) (*gorm.DB, error) {
	query := s.svcCtx.DB.WithContext(ctx).Table("audit_records")
	// Scope authorization: non-admin viewers only see records within their
	// game/env scopes. SQL-side so counts and pagination stay correct.
	if !unrestricted {
		if len(visibleScopes) == 0 {
			return nil, nil // 空可见域：调用方直接返回空集
		}
		orParts := []string{}
		orArgs := []interface{}{}
		for gameID, envs := range visibleScopes {
			for env := range envs {
				orParts = append(orParts, "(game_id = ? AND env = ?)")
				orArgs = append(orArgs, gameID, env)
			}
		}
		query = query.Where(strings.Join(orParts, " OR "), orArgs...)
	}
	if req == nil {
		return query, nil
	}

	// Resolve action filter (kind / kinds / action aliases).
	actionSet := map[string]struct{}{}
	addAlias := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		actionSet[value] = struct{}{}
		for _, canonical := range kindAliases[strings.ToLower(value)] {
			actionSet[canonical] = struct{}{}
		}
	}
	addAlias(req.Action)
	for _, item := range strings.Split(strings.TrimSpace(req.Kinds), ",") {
		addAlias(item)
	}
	for _, item := range strings.Split(strings.TrimSpace(req.Kind), ",") {
		addAlias(item)
	}
	if len(actionSet) > 0 {
		types := make([]string, 0, len(actionSet))
		for t := range actionSet {
			types = append(types, t)
		}
		query = query.Where("event_type IN ?", types)
	}
	if actor := strings.TrimSpace(req.Actor); actor != "" {
		query = query.Where("actor_id = ?", actor)
	}
	if userID := strings.TrimSpace(req.UserID); userID != "" {
		query = query.Where("actor_id = ?", userID)
	}
	if ip := strings.TrimSpace(req.IP); ip != "" {
		query = query.Where("ip = ?", ip)
	}
	if gameID := strings.TrimSpace(req.GameID); gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env := strings.TrimSpace(req.Env); env != "" {
		query = query.Where("env = ?", env)
	}
	if trimmed := strings.TrimSpace(req.Start); trimmed != "" {
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			query = query.Where("timestamp >= ?", parsed)
		}
	}
	if trimmed := strings.TrimSpace(req.End); trimmed != "" {
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			query = query.Where("timestamp <= ?", parsed)
		}
	}
	return query, nil
}

// ExportRows 按过滤条件导出审计行（上限保护，超限截断）。
func (s *Service) ExportRows(ctx context.Context, req *AuditRequest, limit int) ([]AuditItem, bool, error) {
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		return nil, false, errStoreUnavailable
	}
	if limit <= 0 {
		limit = exportRowLimit
	}
	visibleScopes, unrestricted, err := s.resolveVisibleScopes(ctx)
	if err != nil {
		return nil, false, err
	}
	query, err := s.buildAuditQuery(ctx, req, visibleScopes, unrestricted)
	if err != nil {
		return nil, false, err
	}
	if query == nil {
		return []AuditItem{}, false, nil
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, false, err
	}

	rows := []auditRawRow{}
	if err := query.
		Select(exportSelectColumns).
		Order("timestamp DESC").
		Limit(limit + 1).
		Scan(&rows).Error; err != nil {
		return nil, false, err
	}
	truncated := false
	if len(rows) > limit {
		rows = rows[:limit]
		truncated = true
	}
	items := make([]AuditItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, buildAuditItem(&r))
	}
	return items, truncated, nil
}

const exportSelectColumns = "audit_id, timestamp, event_type, outcome, actor_json, resource_json, details_json, error_message, game_id, env, ip, actor_id"

// auditRawRow 与 GetAuditLogs 共用的行扫描形态。
type auditRawRow struct {
	AuditID      string
	Timestamp    time.Time
	EventType    string
	Outcome      string
	ActorJSON    []byte
	ResourceJSON []byte
	DetailsJSON  []byte
	ErrorMessage string
	GameID       string
	Env          string
	IP           string
	ActorID      string
}

// buildAuditItem 把原始行组装为 REST 条目。
func buildAuditItem(r *auditRawRow) AuditItem {
	var actor, resource, details map[string]interface{}
	if len(r.ActorJSON) > 0 {
		_ = json.Unmarshal(r.ActorJSON, &actor)
	}
	if len(r.ResourceJSON) > 0 {
		_ = json.Unmarshal(r.ResourceJSON, &resource)
	}
	if len(r.DetailsJSON) > 0 {
		_ = json.Unmarshal(r.DetailsJSON, &details)
	}
	metadata := map[string]interface{}{}
	for k, v := range details {
		metadata[k] = v
	}
	if r.IP != "" {
		metadata["ip"] = r.IP
	}
	if ua, ok := actor["userAgent"].(string); ok && ua != "" {
		metadata["userAgent"] = ua
	}
	item := AuditItem{
		ID:        r.AuditID,
		CreatedAt: r.Timestamp.UTC().Format(time.RFC3339),
		Action:    r.EventType,
		Result:    r.Outcome,
		Metadata:  metadata,
		UserID:    r.ActorID,
	}
	if id, ok := resource["id"].(string); ok {
		item.Target = id
	}
	if tid, ok := details["traceId"].(string); ok && tid != "" {
		item.TraceID = tid
	} else if tid, ok := details["trace_id"].(string); ok {
		item.TraceID = tid
	}
	if r.GameID != "" {
		item.GameID = r.GameID
	}
	if r.Env != "" {
		item.Env = r.Env
	}
	if r.ErrorMessage != "" {
		metadata["error"] = r.ErrorMessage
	}
	return item
}

// ChainVerifyResult 是链完整性校验结果。
type ChainVerifyResult struct {
	Valid         bool   `json:"valid"`
	Checked       int64  `json:"checked"`
	FirstBreakSeq int64  `json:"firstBreakSeq,omitempty"`
	Message       string `json:"message,omitempty"`
}

// VerifyChain 校验审计哈希链完整性（全量）。
func (s *Service) VerifyChain(ctx context.Context) (*ChainVerifyResult, error) {
	if s.svcCtx == nil || s.svcCtx.AuditService == nil {
		return nil, errStoreUnavailable
	}
	var count int64
	if err := s.svcCtx.DB.WithContext(ctx).Table("audit_records").Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return &ChainVerifyResult{Valid: true, Checked: 0, Message: "empty chain"}, nil
	}
	store := s.svcCtx.AuditService.Store()
	if store == nil {
		return nil, errStoreUnavailable
	}
	records, err := store.GetChainRange(1, count)
	if err != nil {
		return nil, err
	}
	prevHash := ""
	for _, r := range records {
		if prevHash != "" && r.ChainInfo.PrevHash != prevHash {
			return &ChainVerifyResult{
				Valid:         false,
				Checked:       int64(len(records)),
				FirstBreakSeq: r.ChainInfo.Sequence,
				Message:       "prev-hash mismatch",
			}, nil
		}
		prevHash = r.ChainInfo.Hash
	}
	return &ChainVerifyResult{Valid: true, Checked: int64(len(records))}, nil
}
