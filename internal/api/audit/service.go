package audit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
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

// GetAuditLogs retrieves audit logs with filtering and pagination.
func (s *Service) GetAuditLogs(ctx context.Context, req *AuditRequest) (*AuditListResponse, error) {
	if s.svcCtx == nil || s.svcCtx.OpsStateStore == nil {
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
	if page <= 0 {
		page = 1
	} else if page > maxPage {
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

	actionFilter := strings.TrimSpace(req.Action)
	if actionFilter == "" {
		actionFilter = strings.TrimSpace(req.Kind)
	}
	userFilter := strings.TrimSpace(req.UserID)
	if userFilter == "" {
		userFilter = strings.TrimSpace(req.Actor)
	}
	// Audit is scope-neutral: these are filters over audit records, never a
	// replacement for the request scope resolved by GameDBMiddleware.
	gameFilter := strings.TrimSpace(req.GameID)
	envFilter := strings.TrimSpace(req.Env)
	ipFilter := strings.TrimSpace(req.IP)

	actionSet := make(map[string]struct{})
	addActionAlias := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		actionSet[value] = struct{}{}
		switch value {
		case "login":
			actionSet["auth.login"] = struct{}{}
		case "login_fail":
			actionSet["auth.login_failed"] = struct{}{}
		case "login_failed":
			actionSet["auth.login_failed"] = struct{}{}
		}
	}
	if actionFilter != "" {
		addActionAlias(actionFilter)
	}
	for _, item := range strings.Split(strings.TrimSpace(req.Kinds), ",") {
		addActionAlias(item)
	}

	var startAt time.Time
	if trimmed := strings.TrimSpace(req.Start); trimmed != "" {
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			startAt = parsed
		}
	}
	var endAt time.Time
	if trimmed := strings.TrimSpace(req.End); trimmed != "" {
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			endAt = parsed
		}
	}

	state := s.svcCtx.OpsStateStore.Snapshot()
	entries := state.Audit.Entries

	filtered := make([]svc.OpsAuditEntry, 0, len(entries))
	for _, entry := range entries {
		if !unrestricted && !visibleScopes.allows(entry.GameID, entry.Env) {
			continue
		}
		if len(actionSet) > 0 {
			if _, ok := actionSet[strings.ToLower(strings.TrimSpace(entry.Action))]; !ok {
				continue
			}
		}
		if userFilter != "" && !strings.EqualFold(entry.UserID, userFilter) {
			continue
		}
		if gameFilter != "" && !strings.EqualFold(entry.GameID, gameFilter) {
			continue
		}
		if envFilter != "" && !strings.EqualFold(entry.Env, envFilter) {
			continue
		}
		if ipFilter != "" && !strings.EqualFold(fmt.Sprint(entry.Metadata["ip"]), ipFilter) {
			continue
		}
		if !startAt.IsZero() && entry.CreatedAt.Before(startAt) {
			continue
		}
		if !endAt.IsZero() && entry.CreatedAt.After(endAt) {
			continue
		}
		filtered = append(filtered, entry)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	if total == 0 {
		return &AuditListResponse{
			Items:    []AuditItem{},
			Total:    0,
			Page:     page,
			PageSize: size,
		}, nil
	}

	// Use int64 for pagination arithmetic and validate before converting back to int
	total64 := int64(total)
	page64 := int64(page)
	size64 := int64(size)

	if page64 < 1 {
		page64 = 1
	}

	start64 := (page64 - 1) * size64
	if start64 > total64 {
		start64 = total64
	}

	end64 := start64 + size64
	if end64 > total64 {
		end64 = total64
	}

	// Ensure values are within int range before converting
	if start64 < 0 {
		start64 = 0
	}
	if end64 < start64 {
		end64 = start64
	}
	if start64 > int64(math.MaxInt) {
		start64 = int64(math.MaxInt)
	}
	if end64 > int64(math.MaxInt) {
		end64 = int64(math.MaxInt)
	}

	start := int(start64)
	end := int(end64)

	items := make([]AuditItem, 0, end-start)
	for _, entry := range filtered[start:end] {
		items = append(items, AuditItem{
			ID:        entry.ID,
			Action:    entry.Action,
			UserID:    entry.UserID,
			GameID:    entry.GameID,
			Env:       entry.Env,
			Target:    entry.Target,
			Result:    entry.Result,
			TraceID:   entry.TraceID,
			Metadata:  entry.Metadata,
			CreatedAt: utils.FormatTimestamp(entry.CreatedAt),
		})
	}

	return &AuditListResponse{
		Items:    items,
		Total:    total,
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
