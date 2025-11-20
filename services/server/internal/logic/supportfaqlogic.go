package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/repo/gorm/support"
	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportFAQListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportFAQListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFAQListLogic {
	return &SupportFAQListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFAQListLogic) SupportFAQList(req *types.SupportFAQQuery) (*types.SupportFAQListResponse, error) {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return &types.SupportFAQListResponse{Faq: []types.SupportFAQ{}}, nil
	}
	opts := svc.SupportFAQListOptions{
		Query:    req.Q,
		Category: req.Category,
		Visible:  parseVisibleFlag(req.Visible),
	}
	arr, err := repo.ListFAQ(l.ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]types.SupportFAQ, 0, len(arr))
	for _, faq := range arr {
		out = append(out, supportFAQToType(faq))
	}
	return &types.SupportFAQListResponse{Faq: out}, nil
}

type SupportFAQCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportFAQCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFAQCreateLogic {
	return &SupportFAQCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFAQCreateLogic) SupportFAQCreate(req *types.SupportFAQCreateRequest) (*types.SupportFAQCreateResponse, error) {
	if req == nil || strings.TrimSpace(req.Question) == "" {
		return nil, ErrInvalidRequest
	}
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return nil, ErrInvalidRequest
	}
	faq := &support.FAQ{
		Question: strings.TrimSpace(req.Question),
		Answer:   strings.TrimSpace(req.Answer),
		Category: req.Category,
		Tags:     req.Tags,
		Visible:  true,
	}
	if req.Visible != nil {
		faq.Visible = *req.Visible
	}
	if req.Sort != nil {
		faq.Sort = *req.Sort
	}
	if err := repo.CreateFAQ(l.ctx, faq); err != nil {
		return nil, err
	}
	return &types.SupportFAQCreateResponse{Id: int64(faq.ID)}, nil
}

type SupportFAQUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportFAQUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFAQUpdateLogic {
	return &SupportFAQUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFAQUpdateLogic) SupportFAQUpdate(req *types.SupportFAQUpdateRequest) error {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return svc.ErrSupportFAQNotFound
	}
	current, err := repo.GetFAQ(l.ctx, uint(req.Id))
	if err != nil {
		return err
	}
	if strings.TrimSpace(req.Question) != "" {
		current.Question = strings.TrimSpace(req.Question)
	}
	if strings.TrimSpace(req.Answer) != "" {
		current.Answer = strings.TrimSpace(req.Answer)
	}
	if req.Category != "" || req.Category == "" {
		current.Category = req.Category
	}
	if req.Tags != "" || req.Tags == "" {
		current.Tags = req.Tags
	}
	if req.Visible != nil {
		current.Visible = *req.Visible
	}
	if req.Sort != nil {
		current.Sort = *req.Sort
	}
	return repo.UpdateFAQ(l.ctx, current)
}

type SupportFAQDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportFAQDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportFAQDeleteLogic {
	return &SupportFAQDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportFAQDeleteLogic) SupportFAQDelete(req *types.SupportFAQDeleteRequest) error {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return svc.ErrSupportFAQNotFound
	}
	return repo.DeleteFAQ(l.ctx, uint(req.Id))
}

func parseVisibleFlag(val string) *bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "yes":
		b := true
		return &b
	case "0", "false", "no":
		b := false
		return &b
	default:
		return nil
	}
}
