package executionlog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List queries execution traces with pagination.
//
// Permission boundary: mine=true forces filtering to the current logged-in
// user (no audit permission needed); otherwise admin:all / audit:read is
// required.
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	if s.svcCtx.ExecutionLogModel == nil {
		return nil, errors.New("execution log model unavailable")
	}
	if req == nil {
		req = &ListRequest{}
	}
	opts := model.ExecutionLogListOptions{
		FunctionID: strings.TrimSpace(req.FunctionID),
		Source:     strings.TrimSpace(req.Source),
		Status:     strings.TrimSpace(req.Status),
		TraceID:    strings.TrimSpace(req.TraceID),
	}
	var err error
	if opts.From, err = parseTimeParam(req.From); err != nil {
		return nil, err
	}
	if opts.To, err = parseTimeParam(req.To); err != nil {
		return nil, err
	}

	scope := svc.GameScopeFromContext(ctx)
	opts.GameID = scope.GameID
	opts.Env = scope.Env

	if req.Mine {
		actor := currentUser(ctx)
		if actor == "" {
			// fail-safe: return an empty set when the identity cannot be
			// determined, avoiding degradation into the full listing
			return &ListResponse{Items: []ExecutionLogItem{}, Page: 1, Size: req.PageSize}, nil
		}
		opts.Actor = actor
	} else {
		if _, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权查看执行留痕", "admin:all", "audit:read"); err != nil {
			return nil, err
		}
		// Audit view: filtering by applicant is allowed (the mine branch
		// ignores this parameter)
		opts.Actor = strings.TrimSpace(req.Actor)
	}

	items, total, err := s.svcCtx.ExecutionLogModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = 20
	}
	out := make([]ExecutionLogItem, 0, len(items))
	for _, item := range items {
		out = append(out, toListItem(item))
	}
	return &ListResponse{Items: out, Total: total, Page: page, Size: size}, nil
}

// Get returns one detail row: either the caller's own record or with audit
// permission.
func (s *Service) Get(ctx context.Context, req *GetRequest) (*ExecutionLogDetail, error) {
	if s.svcCtx.ExecutionLogModel == nil {
		return nil, errors.New("execution log model unavailable")
	}
	if req == nil || req.ID <= 0 {
		return nil, errorx.NewBadRequest("id 不能为空")
	}
	item, err := s.svcCtx.ExecutionLogModel.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID != "" && item.GameID != scope.GameID {
		return nil, errorx.NewNotFound("执行留痕不存在")
	}
	if strings.TrimSpace(item.Actor) != currentUser(ctx) {
		if _, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权查看执行留痕", "admin:all", "audit:read"); err != nil {
			return nil, err
		}
	}
	detail := &ExecutionLogDetail{ExecutionLogItem: toListItem(*item)}
	if len(item.RequestPayload) > 0 {
		detail.RequestPayload = json.RawMessage(item.RequestPayload)
	}
	if len(item.ResponseBody) > 0 {
		detail.ResponseBody = json.RawMessage(item.ResponseBody)
	}
	return detail, nil
}

func currentUser(ctx context.Context) string {
	username, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(username)
}

func parseTimeParam(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t, nil
		}
	}
	return nil, errorx.NewBadRequest("时间参数格式无效（期望 RFC3339）: " + raw)
}

func toListItem(item model.ExecutionLog) ExecutionLogItem {
	return ExecutionLogItem{
		ID:         item.ID,
		GameID:     item.GameID,
		Env:        item.Env,
		Source:     item.Source,
		FunctionID: item.FunctionID,
		PageKey:    item.PageKey,
		BindingID:  item.BindingID,
		Actor:      item.Actor,
		Route:      item.Route,
		Status:     item.Status,
		DurationMs: item.DurationMs,
		TraceID:    item.TraceID,
		Truncated:  item.Truncated,
		CreatedAt:  item.CreatedAt,
	}
}

// OperatorOptions builds the operator-aggregation options (#33): same
// permission boundary as the non-mine branch of List (admin:all / audit:read);
// under the current scope and the same filter dimensions it returns the
// distinct actors and their trace counts, consumed by the operator dropdown on
// the execution-trace page.
func (s *Service) OperatorOptions(ctx context.Context, req *ListRequest) (*OperatorOptionsResponse, error) {
	if s.svcCtx.ExecutionLogModel == nil {
		return nil, errors.New("execution log model unavailable")
	}
	if _, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权查看执行留痕", "admin:all", "audit:read"); err != nil {
		return nil, err
	}
	if req == nil {
		req = &ListRequest{}
	}
	var err error
	opts := model.ExecutionLogListOptions{
		FunctionID: strings.TrimSpace(req.FunctionID),
		Source:     strings.TrimSpace(req.Source),
		Status:     strings.TrimSpace(req.Status),
		TraceID:    strings.TrimSpace(req.TraceID),
	}
	if opts.From, err = parseTimeParam(req.From); err != nil {
		return nil, err
	}
	if opts.To, err = parseTimeParam(req.To); err != nil {
		return nil, err
	}
	scope := svc.GameScopeFromContext(ctx)
	opts.GameID = scope.GameID
	opts.Env = scope.Env
	rows, err := s.svcCtx.ExecutionLogModel.OperatorOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	items := make([]OperatorOptionsItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, OperatorOptionsItem{
			Value: row.Name,
			Label: row.Name,
			Count: row.Count,
		})
	}
	return &OperatorOptionsResponse{Items: items}, nil
}
