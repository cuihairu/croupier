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

// ---- 对外 REST API（incident-reports §9）----

// ExternalService 承载对外 REST API 的业务逻辑：令牌鉴权、限流、事故注册、
// 外部查询、webhook 发送——全部复用既有 service/handler，外层仅做鉴权与审计。
type ExternalService struct {
	svcCtx   *svc.ServiceContext
	incSvc   *Service
	bugSvc   *bug.Service
	tokenMod *model.ExternalTokenModel
	limiter  *ratelimit.SlidingWindowLimiter
}

// NewExternalService 创建外部 API 服务。rpm<=0 时用默认值 30。
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

// ResolveToken 从明文 token 解析出 ExternalToken 记录并执行启停校验。
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

// TouchUsed 刷新令牌最近使用时间（非阻塞，失败静默）。
func (s *ExternalService) TouchUsed(ctx context.Context, tok *model.ExternalToken) {
	if tok == nil {
		return
	}
	now := time.Now().UTC()
	_ = s.tokenMod.TouchUsed(ctx, tok.ID, now)
}

// CheckRateLimit 按令牌名做滑动窗口限流。超限时返回 rate_limited CodeError。
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

// RegisterIncident 外部登记事故：复用 incSvc.CreateIncident，Source=external，
// CreatedBy=ext:<tokenName>，IncidentKey=idempotencyKey。
func (s *ExternalService) RegisterIncident(ctx context.Context, req *CreateIncidentFromExternalRequest, token *model.ExternalToken, idempotencyKey string) (*IncidentDTO, bool, error) {
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

// ListIncidents 外部事故查询，按 token scope 限可见面：无过滤条件时行级
// 收窄到 scope 类别集；显式查询 scope 外类别直接返回空（不降级为全量）。
func (s *ExternalService) ListIncidents(ctx context.Context, opts model.IncidentQueryOptions, token *model.ExternalToken) (*IncidentListResponse, error) {
	allowed := s.scopeMap(token)
	sqlFilter := opts
	if len(allowed) > 0 && opts.CategoryID != 0 {
		cat, err := model.NewIncidentCategoryModel(s.svcCtx.DB).Get(ctx, opts.CategoryID)
		if err == nil && !allowed[cat.Slug] {
			return &IncidentListResponse{Items: []IncidentDTO{}}, nil
		}
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

// CreateBug 外部注册 bug：透传至 bug.Service.Create。
func (s *ExternalService) CreateBug(ctx context.Context, req *bug.BugCreateRequest, token *model.ExternalToken) (*bug.Bug, error) {
	resp, err := s.bugSvc.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return &resp.Bug, nil
}

// scopeMap 把 token scope 的 categories 转成集合；空 scope 返回空 map（全量）。
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

// SendLifecycleWebhook 推一条生命周期事件到外部出口链（§9.3）。
func (s *ExternalService) SendLifecycleWebhook(ctx context.Context, row *model.Incident, kind IncidentLifecycleKind) {
	DispatchLifecycle(s.svcCtx, row, kind)
}
