package incident

import (
	"context"
	"errors"
	"time"

	"github.com/cuihairu/croupier/internal/api/bug"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/ratelimit"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

// ---- External REST API (incident-reports section 9) ----

// ExternalService carries the business logic of the external REST API: token
// authentication, rate limiting, incident registration, external queries and
// webhook dispatch. It reuses the existing service/handler layers; the outer
// layer only performs authentication and auditing.
type ExternalService struct {
	svcCtx   *svc.ServiceContext
	incSvc   *Service
	bugSvc   *bug.Service
	tokenMod *model.ExternalTokenModel
	limiter  *ratelimit.SlidingWindowLimiter
}

// NewExternalService creates the external API service. When rpm<=0 the default
// value 30 is used.
func NewExternalService(svcCtx *svc.ServiceContext) *ExternalService {
	rpm := svcCtx.Config.ExternalAPI.RateLimitPerMinute
	if rpm <= 0 {
		rpm = 30
	}
	return &ExternalService{
		svcCtx:   svcCtx,
		incSvc:   NewService(svcCtx),
		bugSvc:   bug.NewService(svcCtx),
		tokenMod: model.NewExternalTokenModel(svcCtx.DB),
		limiter:  ratelimit.NewSlidingWindowLimiter(rpm, time.Minute),
	}
}

// ResolveToken resolves the plain token into an ExternalToken record and runs the
// enabled/disabled check.
func (s *ExternalService) ResolveToken(ctx context.Context, plain string) (*model.ExternalToken, error) {
	if plain == "" {
		return nil, errorx.NewUnauthorized("缺少认证 token")
	}
	hash := model.HashExternalToken(plain)
	tok, err := s.tokenMod.FindByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewUnauthorized("无效的认证 token")
		}
		return nil, err
	}
	if !tok.Enabled {
		return nil, errorx.NewForbidden("该令牌已禁用或已吊销")
	}
	return tok, nil
}

// TouchUsed refreshes the token's last-used time (non-blocking; failures are
// silently ignored).
func (s *ExternalService) TouchUsed(ctx context.Context, tok *model.ExternalToken) {
	if tok == nil {
		return
	}
	now := time.Now().UTC()
	_ = s.tokenMod.TouchUsed(ctx, tok.ID, now)
}

// CheckRateLimit applies sliding-window rate limiting by token name. On
// overflow it returns a rate_limited CodeError.
func (s *ExternalService) CheckRateLimit(ctx context.Context, name string) error {
	result, err := s.limiter.Allow(ctx, name)
	if err != nil {
		return errorx.NewInternalError("限流失效，请重试")
	}
	if !result.Allowed {
		return errorx.NewTooManyRequests("请求速率受限，请稍后重试")
	}
	return nil
}

// RegisterIncident registers an incident externally: it reuses
// incSvc.CreateIncident with Source=external, CreatedBy=ext:<tokenName> and
// IncidentKey=idempotencyKey. Before writing it strictly validates the target
// against the token's game/env allowlist (AUD-036).
func (s *ExternalService) RegisterIncident(ctx context.Context, req *CreateIncidentFromExternalRequest, token *model.ExternalToken, idempotencyKey string) (*IncidentDTO, bool, error) {
	if err := s.checkScopeWrite(token, req.GameID, req.Env); err != nil {
		return nil, false, err
	}
	incidentReq := &IncidentCreateRequest{
		Title:           req.Title,
		CategoryID:      req.CategoryID,
		Subcategory:     req.Subcategory,
		Severity:        req.Severity,
		DetectedAt:      req.DetectedAt,
		ResponsibleType: req.ResponsibleType,
		ResponsibleID:   req.ResponsibleID,
		GameID:          req.GameID,
		Env:             req.Env,
		ExecLogIDs:      req.ExecLogIDs,
		RefType:         req.RefType,
		RefID:           req.RefID,
		Details:         req.Details,
	}
	if incidentReq.Severity == "" {
		incidentReq.Severity = model.IncidentSeverityInfo
	}
	incidentReq.Source = model.IncidentSourceExternal
	if req.Source != "" {
		incidentReq.Source = req.Source
	}
	incidentReq.IncidentKey = idempotencyKey
	dto, created, err := s.incSvc.CreateIncident(ctx, incidentReq, "ext:"+token.Name)
	if err != nil {
		return nil, false, err
	}
	return dto, created, nil
}

// ListIncidents is the external incident query; it limits the visible surface
// by the token scope: with no filter conditions rows are narrowed at the row
// level to the scope category set; querying a category outside the scope
// directly returns an empty set (it does not degrade to the full list).
// AUD-036: it also enforces SQL-level filtering by the token's game/env
// allowlist - when the allowlist is non-empty, an explicit out-of-scope game/env
// query returns empty directly, and an unfiltered query is narrowed by the
// allowlist.
func (s *ExternalService) ListIncidents(ctx context.Context, opts model.IncidentQueryOptions, token *model.ExternalToken) (*IncidentListResponse, error) {
	allowed := s.scopeMap(token)
	games := token.ScopeGameIDs()
	envs := token.ScopeEnvs()
	sqlFilter := opts
	if len(allowed) > 0 && opts.CategoryID != 0 {
		cat, err := model.NewIncidentCategoryModel(s.svcCtx.DB).Get(ctx, opts.CategoryID)
		if err == nil && !allowed[cat.Slug] {
			return &IncidentListResponse{Items: []IncidentDTO{}}, nil
		}
	}
	// Explicit query for game/env outside the allowlist -> empty (no degrade to full list).
	if len(games) > 0 && opts.GameID != "" && !containsString(games, opts.GameID) {
		return &IncidentListResponse{Items: []IncidentDTO{}}, nil
	}
	if len(envs) > 0 && opts.Env != "" && !containsString(envs, opts.Env) {
		return &IncidentListResponse{Items: []IncidentDTO{}}, nil
	}
	// Allowlist non-empty -> force narrowing to the allowlist (SQL level; Total is narrowed too).
	if len(games) > 0 {
		sqlFilter.GameIDs = games
	}
	if len(envs) > 0 {
		sqlFilter.Envs = envs
	}
	sqlFilter.Normalize()
	rows, total, err := model.NewIncidentModel(s.svcCtx.DB).List(ctx, sqlFilter)
	if err != nil {
		return nil, err
	}
	catNames := s.incSvc.categoryNames(ctx, rows)
	items := make([]IncidentDTO, 0, len(rows))
	for i := range rows {
		if len(allowed) > 0 {
			cn := catNames[rows[i].CategoryID]
			if !allowed[cn.slug] {
				continue
			}
		}
		dto := toIncidentDTOBasic(&rows[i])
		if cn, ok := catNames[rows[i].CategoryID]; ok {
			dto.CategorySlug = cn.slug
			dto.CategoryName = cn.name
		}
		items = append(items, dto)
	}
	return &IncidentListResponse{Items: items, Total: total}, nil
}

// CreateBug registers a bug externally: it passes through to
// bug.Service.Create. Before writing it strictly validates the target against the
// token's game/env allowlist (AUD-036).
func (s *ExternalService) CreateBug(ctx context.Context, req *bug.BugCreateRequest, token *model.ExternalToken) (*bug.Bug, error) {
	if err := s.checkScopeWrite(token, req.GameID, req.Env); err != nil {
		return nil, err
	}
	resp, err := s.bugSvc.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return &resp.Bug, nil
}

// checkScopeWrite validates that the write target game/env falls inside the
// token's allowlist (AUD-036).
// An empty allowlist means unrestricted (backwards compatible, same behavior as
// today); when the allowlist is non-empty a missing or out-of-scope target is
// rejected with 403 (an empty target cannot be proven in-scope, so reject rather
// than fall back to the global scope).
func (s *ExternalService) checkScopeWrite(token *model.ExternalToken, gameID, env string) error {
	if token == nil {
		return nil
	}
	games := token.ScopeGameIDs()
	envs := token.ScopeEnvs()
	if len(games) > 0 && (gameID == "" || !containsString(games, gameID)) {
		return errorx.NewForbidden("该令牌不允许写入此游戏")
	}
	if len(envs) > 0 && (env == "" || !containsString(envs, env)) {
		return errorx.NewForbidden("该令牌不允许写入此环境")
	}
	return nil
}

// containsString reports whether val is in slice.
func containsString(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// scopeMap converts the token scope categories into a set; an empty scope
// returns an empty map (full list).
func (s *ExternalService) scopeMap(token *model.ExternalToken) map[string]bool {
	m := make(map[string]bool)
	if token == nil {
		return m
	}
	cats := token.ScopeCategories()
	for _, c := range cats {
		m[c] = true
	}
	return m
}

// SendLifecycleWebhook pushes one lifecycle event to the external egress chain (section 9.3).
func (s *ExternalService) SendLifecycleWebhook(ctx context.Context, row *model.Incident, kind IncidentLifecycleKind) {
	DispatchLifecycle(ctx, s.svcCtx, row, kind)
}
