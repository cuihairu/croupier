package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

const (
	// maxPageSize limits the number of audit entries returned per page to
	// avoid overflow and excessive memory allocations.
	maxPageSize = 1000
	// maxPage is a very large upper bound on the page number to ensure that
	// (page-1)*size cannot overflow an int when size is bounded by maxPageSize.
	maxPage = 1_000_000_000
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// kindAliases maps frontend "kind" shortcuts to canonical audit event types.
// Login-logs / operation-logs pages send these; unknown values pass through
// verbatim (they may already be canonical event types).
var kindAliases = map[string][]string{
	"login":              {"auth.login"},
	"login_fail":         {"auth.login_failed"},
	"login_failed":       {"auth.login_failed"},
	"login_rate_limited": {"auth.login_rate_limited"},
	"logout":             {"auth.logout"},
	"invoke":             {"function.invoke", "page.execute"},
	"page_execute":       {"page.execute"},
	"start_job":          {"job.start"},
	"cancel_job":         {"job.cancel"},
	"node_drain":         {"node.drain"},
	"node_undrain":       {"node.undrain"},
	"node_restart":       {"node.restart"},
	"user_create":        {"admin.user_create"},
	"user_update":        {"admin.user_update"},
	"user_delete":        {"admin.user_delete"},
	"user_lock":          {"admin.user_lock"},
	"user_unlock":        {"admin.user_unlock"},
	"approval_approve":   {"approval.approved"},
	"approval_reject":    {"approval.rejected"},
}

// GetAuditLogs retrieves audit logs from the persistent audit_records table
// (single source of truth). The legacy in-memory OpsStateStore audit trail
// was removed — audit history must survive restarts.
func (s *Service) GetAuditLogs(ctx context.Context, req *AuditRequest) (*AuditListResponse, error) {
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		return nil, errors.New("audit store unavailable")
	}
	if req == nil {
		req = &AuditRequest{}
	}
	visibleScopes, unrestricted, err := s.resolveVisibleScopes(ctx)
	if err != nil {
		return nil, err
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	if page > maxPage {
		page = maxPage
	}
	size := req.PageSize
	if size <= 0 {
		size = req.Size
	}
	if size <= 0 {
		size = 20
	}
	if size > maxPageSize {
		size = maxPageSize
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

	query := s.svcCtx.DB.WithContext(ctx).Table("audit_records")
	// Scope authorization: non-admin viewers only see records within their
	// game/env scopes. SQL-side so counts and pagination stay correct.
	if !unrestricted {
		if len(visibleScopes) == 0 {
			return &AuditListResponse{Items: []AuditItem{}, Total: 0, Page: page, PageSize: size}, nil
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
	var startAt, endAt time.Time
	if trimmed := strings.TrimSpace(req.Start); trimmed != "" {
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			startAt = parsed
			query = query.Where("timestamp >= ?", startAt)
		}
	}
	if trimmed := strings.TrimSpace(req.End); trimmed != "" {
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			endAt = parsed
			query = query.Where("timestamp <= ?", endAt)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	type rawRow struct {
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
	rows := []rawRow{}
	if err := query.
		Select("audit_id, timestamp, event_type, outcome, actor_json, resource_json, details_json, error_message, game_id, env, ip, actor_id").
		Order("timestamp DESC").
		Offset((page - 1) * size).Limit(size).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]AuditItem, 0, len(rows))
	for _, r := range rows {
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
		items = append(items, item)
	}

	return &AuditListResponse{
		Items:    items,
		Total:    int(total),
		Page:     page,
		PageSize: size,
	}, nil
}

// auditScopeSet contains every game/environment pair a non-administrator may
// inspect through the scope-neutral audit API. Audit access has two layers:
// audit:read grants entry to the API, while game-environment authorization
// limits the records that can be observed.
type auditScopeSet map[string]map[string]struct{}

func (scopes auditScopeSet) allows(gameID, env string) bool {
	gameID = strings.ToLower(strings.TrimSpace(gameID))
	env = strings.ToLower(strings.TrimSpace(env))
	if gameID == "" || env == "" {
		return false
	}
	_, allowed := scopes[gameID][env]
	return allowed
}

func (s *Service) resolveVisibleScopes(ctx context.Context) (auditScopeSet, bool, error) {
	// Direct service calls used by migrations and legacy tests do not carry an
	// authorization model. HTTP construction always provides these models and
	// therefore always takes the checked branch below.
	if s.svcCtx.AdminModel == nil || s.svcCtx.RoleModel == nil {
		return nil, true, nil
	}

	roles, _, err := utils.RequireAnyPermission(ctx, s.svcCtx, "无权查看审计日志", "admin:all", "audit:read")
	if err != nil {
		return nil, false, err
	}
	if utils.HasAdminRole(utils.RoleNamesFromModels(roles)) {
		return nil, true, nil
	}
	if s.svcCtx.GameModel == nil {
		return nil, false, errors.New("game model unavailable")
	}

	admin, _, err := utils.LoadCurrentAdmin(ctx, s.svcCtx)
	if err != nil {
		return nil, false, err
	}
	envScopes, err := s.svcCtx.AdminModel.GetAdminEnvScopes(ctx, admin.ID)
	if err != nil {
		return nil, false, err
	}

	visible := make(auditScopeSet, len(envScopes))
	for _, envScope := range envScopes {
		game, err := s.svcCtx.GameModel.FindOne(ctx, envScope.GameID)
		if err != nil || game == nil {
			continue
		}
		gameID := strings.ToLower(strings.TrimSpace(game.GameID))
		env := strings.ToLower(strings.TrimSpace(envScope.Env))
		if gameID == "" || env == "" {
			continue
		}
		if visible[gameID] == nil {
			visible[gameID] = make(map[string]struct{})
		}
		visible[gameID][env] = struct{}{}
	}
	return visible, false, nil
}
