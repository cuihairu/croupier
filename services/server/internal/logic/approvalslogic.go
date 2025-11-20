package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	appr "github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ApprovalsListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApprovalsListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalsListLogic {
	return &ApprovalsListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalsListLogic) ApprovalsList(req *types.ApprovalsListRequest) (*types.ApprovalsListResponse, error) {
	store := l.svcCtx.ApprovalsStore()
	if store == nil {
		return nil, ErrUnavailable
	}
	if req == nil {
		req = &types.ApprovalsListRequest{}
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.Size
	if size <= 0 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	filter := appr.Filter{
		State:      strings.TrimSpace(req.State),
		FunctionID: strings.TrimSpace(req.FunctionId),
		GameID:     strings.TrimSpace(req.GameId),
		Env:        strings.TrimSpace(req.Env),
		Actor:      strings.TrimSpace(req.Actor),
		Mode:       strings.TrimSpace(req.Mode),
	}
	pageConf := appr.Page{Page: page, Size: size, Sort: strings.TrimSpace(req.Sort)}
	items, total, err := store.List(filter, pageConf)
	if err != nil {
		return nil, err
	}
	resp := &types.ApprovalsListResponse{
		Approvals: make([]types.ApprovalSummary, 0, len(items)),
		Total:     total,
		Page:      page,
		Size:      size,
	}
	for _, item := range items {
		resp.Approvals = append(resp.Approvals, approvalSummaryFromModel(item))
	}
	return resp, nil
}

type ApprovalGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApprovalGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalGetLogic {
	return &ApprovalGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalGetLogic) ApprovalGet(req *types.ApprovalGetRequest) (*types.ApprovalDetailResponse, error) {
	store := l.svcCtx.ApprovalsStore()
	if store == nil {
		return nil, ErrUnavailable
	}
	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, ErrInvalidRequest
	}
	approval, err := store.Get(strings.TrimSpace(req.Id))
	if err != nil {
		return nil, ErrNotFound
	}
	detail := approvalDetailFromModel(approval)
	return detail, nil
}

type ApprovalApproveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApprovalApproveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalApproveLogic {
	return &ApprovalApproveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalApproveLogic) ApprovalApprove(req *types.ApprovalApproveRequest) (*types.ApprovalSummary, error) {
	store := l.svcCtx.ApprovalsStore()
	if store == nil {
		return nil, ErrUnavailable
	}
	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, ErrInvalidRequest
	}
	approval, err := store.Approve(strings.TrimSpace(req.Id))
	if err != nil {
		return nil, ErrNotFound
	}
	summary := approvalSummaryFromModel(approval)
	return &summary, nil
}

type ApprovalRejectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewApprovalRejectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApprovalRejectLogic {
	return &ApprovalRejectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ApprovalRejectLogic) ApprovalReject(req *types.ApprovalRejectRequest) (*types.ApprovalSummary, error) {
	store := l.svcCtx.ApprovalsStore()
	if store == nil {
		return nil, ErrUnavailable
	}
	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, ErrInvalidRequest
	}
	approval, err := store.Reject(strings.TrimSpace(req.Id), strings.TrimSpace(req.Reason))
	if err != nil {
		return nil, ErrNotFound
	}
	summary := approvalSummaryFromModel(approval)
	return &summary, nil
}

func approvalSummaryFromModel(a *appr.Approval) types.ApprovalSummary {
	if a == nil {
		return types.ApprovalSummary{}
	}
	return types.ApprovalSummary{
		Id:              a.ID,
		CreatedAt:       a.CreatedAt.Format(time.RFC3339),
		Actor:           a.Actor,
		FunctionId:      a.FunctionID,
		IdempotencyKey:  a.IdempotencyKey,
		Route:           a.Route,
		TargetServiceId: a.TargetServiceID,
		HashKey:         a.HashKey,
		GameId:          a.GameID,
		Env:             a.Env,
		State:           a.State,
		Mode:            a.Mode,
		Reason:          a.Reason,
	}
}

func approvalDetailFromModel(a *appr.Approval) *types.ApprovalDetailResponse {
	summary := approvalSummaryFromModel(a)
	preview := ""
	if a != nil {
		preview = summarizePayload(a.Payload)
	}
	return &types.ApprovalDetailResponse{
		ApprovalSummary: summary,
		PayloadPreview:  preview,
	}
}

func summarizePayload(payload []byte) string {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return ""
	}
	var buf bytes.Buffer
	if json.Indent(&buf, []byte(trimmed), "", "  ") == nil {
		trimmed = buf.String()
	}
	if len(trimmed) > 4096 {
		return trimmed[:4096]
	}
	return trimmed
}
