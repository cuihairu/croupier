package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

// HasBlockingDiagnostics 提案诊断是否含 error 级项（发布质量门槛）。
// 供发布分级（T10）等跨包调用方复用同一判定。
func HasBlockingDiagnostics(proposal *model.PageProposal) bool {
	if proposal == nil {
		return false
	}
	return hasBlockingDiagnostics(proposal.Diagnostics)
}

// hasBlockingDiagnostics checks if diagnostics contain error-level items.
func hasBlockingDiagnostics(diagnosticsJSON []byte) bool {
	if len(diagnosticsJSON) == 0 {
		return false
	}

	var diagnostics []struct {
		Severity string `json:"severity"`
	}
	if err := json.Unmarshal(diagnosticsJSON, &diagnostics); err != nil {
		return false
	}

	for _, d := range diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

func (s *ProposalService) listBlockedIssueDTOs(ctx context.Context, gameID, env, resourceKey string) ([]BlockedProposalIssueDTO, error) {
	var issues []*model.BlockedProposalIssue
	var err error
	resourceKey = strings.TrimSpace(resourceKey)
	if resourceKey == "" {
		issues, err = s.blockedModel.ListByScope(ctx, gameID, env)
	} else {
		issues, err = s.blockedModel.ListByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	}
	if err != nil {
		if isMissingTableErr(err) {
			return []BlockedProposalIssueDTO{}, nil
		}
		return nil, err
	}
	out := make([]BlockedProposalIssueDTO, 0, len(issues))
	// 两个 List* 均为纯 gorm Find：每行由 gorm 构造非 nil 指针追加，
	// 不会产生 nil 元素，原 issue==nil 防御分支为死代码，已删。
	for _, issue := range issues {
		out = append(out, blockedIssueDTOFromModel(issue))
	}
	return out, nil
}

func blockedIssueDTOFromModel(issue *model.BlockedProposalIssue) BlockedProposalIssueDTO {
	return BlockedProposalIssueDTO{
		ID:            issue.ID,
		GameID:        issue.GameID,
		Env:           issue.Env,
		ResourceKey:   strings.TrimSpace(issue.ResourceKey),
		FunctionID:    strings.TrimSpace(issue.FunctionID),
		SourceDigests: stringSliceFromJSON(issue.SourceDigests),
		Diagnostics:   diagnosticsFromJSON(issue.Diagnostics),
		RepairHint:    normalizeJSONMap(issue.RepairHint),
		Status:        issue.Status,
		UpdatedAt:     issue.UpdatedAt.Format(time.RFC3339),
		UpdatedBy:     issue.UpdatedBy,
	}
}
