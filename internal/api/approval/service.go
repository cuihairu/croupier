package approval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a paginated list of approvals
func (s *Service) List(ctx context.Context, req *ApprovalsListRequest) (*ApprovalsListResponse, error) {
	if s.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.PageSize
	if size <= 0 {
		size = 20
	}

	filter := approvals.Filter{
		State: strings.TrimSpace(req.Status),
	}
	items, total, err := s.svcCtx.ApprovalsStore.List(filter, approvals.Page{
		Page: page,
		Size: size,
	})
	if err != nil {
		return nil, err
	}

	list := make([]ApprovalSummary, 0, len(items))
	for _, item := range items {
		list = append(list, buildApprovalSummary(item))
	}

	return &ApprovalsListResponse{
		Approvals: list,
		Total:     int64(total),
		Page:      page,
		Size:      size,
	}, nil
}

// Get retrieves details of a specific approval
func (s *Service) Get(ctx context.Context, req *ApprovalGetRequest) (*ApprovalGetResponse, error) {
	if s.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("id 不能为空")
	}

	approval, err := s.svcCtx.ApprovalsStore.Get(strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}

	detail := buildApprovalDetail(approval)

	return &ApprovalGetResponse{
		Approval: detail,
	}, nil
}

// Approve approves an approval
func (s *Service) Approve(ctx context.Context, req *ApprovalApproveRequest) (*ApprovalApproveResponse, error) {
	if s.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("id 不能为空")
	}

	record, err := s.svcCtx.ApprovalsStore.Approve(strings.TrimSpace(req.ID))
	if err != nil {
		return nil, err
	}

	return &ApprovalApproveResponse{
		ID:    record.ID,
		State: record.State,
	}, nil
}

// Reject rejects an approval
func (s *Service) Reject(ctx context.Context, req *ApprovalRejectRequest) (*ApprovalRejectResponse, error) {
	if s.svcCtx.ApprovalsStore == nil {
		return nil, errors.New("approvals store unavailable")
	}
	if req == nil || strings.TrimSpace(req.ID) == "" {
		return nil, errors.New("id 不能为空")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, errors.New("reason 不能为空")
	}

	record, err := s.svcCtx.ApprovalsStore.Reject(strings.TrimSpace(req.ID), strings.TrimSpace(req.Reason))
	if err != nil {
		return nil, err
	}

	return &ApprovalRejectResponse{
		ID:     record.ID,
		State:  record.State,
		Reason: record.Reason,
	}, nil
}

// Helper functions

func buildApprovalSummary(a *approvals.Approval) ApprovalSummary {
	if a == nil {
		return ApprovalSummary{}
	}
	return ApprovalSummary{
		ID:              a.ID,
		CreatedAt:       utils.FormatTimestamp(a.CreatedAt),
		UpdatedAt:       utils.FormatTimestamp(a.UpdatedAt),
		Actor:           a.Actor,
		FunctionID:      a.FunctionID,
		GameID:          a.GameID,
		Env:             a.Env,
		State:           strings.ToLower(strings.TrimSpace(a.State)),
		Mode:            defaultString(a.Mode, "invoke"),
		Route:           a.Route,
		IdempotencyKey:  a.IdempotencyKey,
		TargetServiceID: a.TargetServiceID,
		HashKey:         a.HashKey,
		Reason:          a.Reason,
	}
}

func buildApprovalDetail(a *approvals.Approval) Approval {
	summary := buildApprovalSummary(a)
	payload, preview := decodeApprovalPayload(a)
	return Approval{
		ID:              summary.ID,
		CreatedAt:       summary.CreatedAt,
		UpdatedAt:       summary.UpdatedAt,
		Actor:           summary.Actor,
		FunctionID:      summary.FunctionID,
		GameID:          summary.GameID,
		Env:             summary.Env,
		State:           summary.State,
		Mode:            summary.Mode,
		Route:           summary.Route,
		IdempotencyKey:  summary.IdempotencyKey,
		TargetServiceID: summary.TargetServiceID,
		HashKey:         summary.HashKey,
		Reason:          summary.Reason,
		Payload:         payload,
		PayloadPreview:  preview,
	}
}

func decodeApprovalPayload(a *approvals.Approval) (map[string]interface{}, string) {
	if a == nil || len(a.Payload) == 0 {
		return nil, ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(a.Payload, &payload); err != nil {
		return nil, string(a.Payload)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, a.Payload, "", "  "); err != nil {
		return payload, string(a.Payload)
	}
	return payload, buf.String()
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
