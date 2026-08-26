package audit

import (
	"context"
	"errors"
	"strings"

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

	query, err := s.buildAuditQuery(ctx, req, visibleScopes, unrestricted)
	if err != nil {
		return nil, err
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	if query == nil {
		return &AuditListResponse{Items: []AuditItem{}, Total: 0, Page: page, PageSize: size}, nil
	}
	rows := []auditRawRow{}
	if err := query.
		Select(exportSelectColumns).
		Order("timestamp DESC").
		Offset((page - 1) * size).Limit(size).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AuditItem, 0, len(rows))
	for i := range rows {
		items = append(items, buildAuditItem(&rows[i]))
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
