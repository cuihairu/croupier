package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/repo/gorm/support"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFeedbackListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportFeedbackListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFeedbackListLogic {
	return &SupportFeedbackListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFeedbackListLogic) SupportFeedbackList(req *types.SupportFeedbackQuery) (*types.SupportFeedbackListResponse, error) {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return &types.SupportFeedbackListResponse{Feedback: []types.SupportFeedback{}}, nil
	}
	opts := svc.SupportFeedbackListOptions{
		Query:    req.Q,
		Category: req.Category,
		Status:   req.Status,
		GameID:   req.GameId,
		Env:      req.Env,
		Page:     req.Page,
		Size:     req.Size,
	}
	items, total, err := repo.ListFeedback(l.ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]types.SupportFeedback, 0, len(items))
	for _, fb := range items {
		out = append(out, supportFeedbackToType(fb))
	}
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	size := opts.Size
	if size <= 0 || size > 200 {
		size = 20
	}
	return &types.SupportFeedbackListResponse{
		Feedback: out,
		Total:    total,
		Page:     page,
		Size:     size,
	}, nil
}

type SupportFeedbackCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportFeedbackCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFeedbackCreateLogic {
	return &SupportFeedbackCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFeedbackCreateLogic) SupportFeedbackCreate(req *types.SupportFeedbackCreateRequest) (*types.SupportFeedbackCreateResponse, error) {
	if req == nil || strings.TrimSpace(req.Content) == "" {
		return nil, ErrInvalidRequest
	}
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return nil, ErrInvalidRequest
	}
	fb := &support.Feedback{
		PlayerID: req.PlayerId,
		Contact:  req.Contact,
		Content:  strings.TrimSpace(req.Content),
		Category: req.Category,
		Priority: supportFeedbackDefaultPriority(req.Priority),
		Status:   supportFeedbackDefaultStatus(req.Status),
		Attach:   req.Attach,
		GameID:   req.GameId,
		Env:      req.Env,
	}
	if err := repo.CreateFeedback(l.ctx, fb); err != nil {
		return nil, err
	}
	return &types.SupportFeedbackCreateResponse{Id: int64(fb.ID)}, nil
}

type SupportFeedbackUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportFeedbackUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFeedbackUpdateLogic {
	return &SupportFeedbackUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFeedbackUpdateLogic) SupportFeedbackUpdate(req *types.SupportFeedbackUpdateRequest) error {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return svc.ErrSupportFeedbackNotFound
	}
	current, err := repo.GetFeedback(l.ctx, uint(req.Id))
	if err != nil {
		return err
	}
	current.Category = req.Category
	if strings.TrimSpace(req.Priority) != "" {
		current.Priority = strings.TrimSpace(req.Priority)
	}
	if strings.TrimSpace(req.Status) != "" {
		current.Status = strings.TrimSpace(req.Status)
	}
	current.Attach = req.Attach
	return repo.UpdateFeedback(l.ctx, current)
}

type SupportFeedbackDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportFeedbackDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFeedbackDeleteLogic {
	return &SupportFeedbackDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFeedbackDeleteLogic) SupportFeedbackDelete(req *types.SupportFeedbackDeleteRequest) error {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return svc.ErrSupportFeedbackNotFound
	}
	return repo.DeleteFeedback(l.ctx, uint(req.Id))
}

func supportFeedbackDefaultStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "new"
	}
	return status
}

func supportFeedbackDefaultPriority(priority string) string {
	priority = strings.TrimSpace(priority)
	if priority == "" {
		return "normal"
	}
	return priority
}
