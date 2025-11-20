package logic

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/repo/gorm/support"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SupportCommentsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportCommentsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportCommentsListLogic {
	return &SupportCommentsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportCommentsListLogic) SupportCommentsList(req *types.SupportTicketUpdateRequest) (*types.SupportCommentsResponse, error) {
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return &types.SupportCommentsResponse{Comments: []types.SupportComment{}}, nil
	}
	comments, err := repo.ListComments(l.ctx, uint(req.Id))
	if err != nil {
		return nil, err
	}
	out := make([]types.SupportComment, 0, len(comments))
	for _, c := range comments {
		out = append(out, types.SupportComment{
			Id:        int64(c.ID),
			Author:    c.Author,
			Content:   c.Content,
			Attach:    c.Attach,
			CreatedAt: formatSupportTime(c.CreatedAt),
		})
	}
	return &types.SupportCommentsResponse{Comments: out}, nil
}

type SupportCommentCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSupportCommentCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SupportCommentCreateLogic {
	return &SupportCommentCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SupportCommentCreateLogic) SupportCommentCreate(req *types.SupportCommentCreateRequest) (*types.SupportCommentCreateResponse, error) {
	if req == nil || strings.TrimSpace(req.Content) == "" {
		return nil, ErrInvalidRequest
	}
	repo := l.svcCtx.SupportRepository()
	if repo == nil {
		return nil, svc.ErrSupportTicketNotFound
	}
	author := strings.TrimSpace(svc.ActorFromContext(l.ctx))
	if author == "" {
		author = "system"
	}
	comment := &support.TicketComment{
		TicketID: uint(req.Id),
		Author:   author,
		Content:  strings.TrimSpace(req.Content),
		Attach:   req.Attach,
	}
	if err := repo.CreateComment(l.ctx, comment); err != nil {
		return nil, err
	}
	return &types.SupportCommentCreateResponse{Id: int64(comment.ID)}, nil
}
