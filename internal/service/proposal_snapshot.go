package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func createProposalVersionSnapshot(
	ctx context.Context,
	versionModel *model.PageProposalVersionModel,
	proposal *model.PageProposal,
	reason string,
	actor string,
) (int, error) {
	if proposal == nil || proposal.ID == 0 {
		return 0, errorx.NewValidationError("proposal must be persisted before snapshot")
	}
	nextVersion, err := versionModel.GetNextVersion(ctx, proposal.ID)
	if err != nil {
		return 0, err
	}
	proposalJSON, err := json.Marshal(proposal)
	if err != nil {
		return 0, err
	}
	if err := versionModel.CreateVersion(ctx, &model.PageProposalVersion{
		ProposalID:      proposal.ID,
		Version:         nextVersion,
		Proposal:        proposalJSON,
		FunctionDigest:  proposal.FunctionDigest,
		SemanticsDigest: proposal.SemanticsDigest,
		ChangeReason:    reason,
		CreatedBy:       actor,
	}); err != nil {
		return 0, err
	}
	return nextVersion, nil
}

func parsePublishedSnapshot(item model.PublishedPageSpec) (spec.PageSpec, []spec.BindingContractSnapshot) {
	var page spec.PageSpec
	if strings.TrimSpace(item.SpecJSON) != "" {
		_ = json.Unmarshal([]byte(item.SpecJSON), &page)
	}
	if strings.TrimSpace(page.PageKey) == "" {
		page.PageKey = item.PageKey
	}
	var contracts []spec.BindingContractSnapshot
	if strings.TrimSpace(item.BindingContractsJSON) != "" {
		_ = json.Unmarshal([]byte(item.BindingContractsJSON), &contracts)
	}
	return page, contracts
}

func parseDraftPageSpec(item model.PageSpec) spec.PageSpec {
	var page spec.PageSpec
	if strings.TrimSpace(item.SpecJSON) != "" {
		_ = json.Unmarshal([]byte(item.SpecJSON), &page)
	}
	if strings.TrimSpace(page.PageKey) == "" {
		page.PageKey = item.PageKey
		page.Type = spec.PageType(item.Type)
		page.ResourceKey = item.ResourceKey
		page.Title = item.GetTitle()
		page.Category = spec.PageCategorySpec{
			Key:   item.CategoryKey,
			Order: item.CategoryOrder,
		}
	}
	return page
}

func pageSpecFromProposal(proposal *model.PageProposal) (spec.PageSpec, string, error) {
	if proposal == nil {
		return spec.PageSpec{}, "", errorx.NewBadRequest("proposal is required")
	}
	if len(proposal.PageSpec) == 0 {
		return spec.PageSpec{}, "", errorx.NewValidationError("proposal does not contain canonical PageSpec")
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal(proposal.PageSpec, &pageSpec); err != nil {
		return spec.PageSpec{}, "", errorx.NewValidationError("proposal PageSpec is invalid JSON")
	}
	pageSpec.PageKey = strings.TrimSpace(pageSpec.PageKey)
	pageSpec.ResourceKey = strings.TrimSpace(pageSpec.ResourceKey)
	pageSpec.Icon = strings.TrimSpace(pageSpec.Icon)
	pageSpec.Title = normalizeLocalizedText(pageSpec.Title)
	pageSpec.Description = normalizeLocalizedText(pageSpec.Description)
	pageSpec.Category.Key = strings.TrimSpace(pageSpec.Category.Key)
	for i := range pageSpec.Bindings {
		pageSpec.Bindings[i].ID = strings.TrimSpace(pageSpec.Bindings[i].ID)
		pageSpec.Bindings[i].FunctionID = strings.TrimSpace(pageSpec.Bindings[i].FunctionID)
	}
	// json.Marshal 对 Unmarshal 产物的 spec.PageSpec 恒成功（唯一自定义
	// Marshaler JSONSchema.MarshalJSON 实现为 `return []byte(s), nil` 无
	// 出错路径；LocalizedText 为 map 类型恒可序列化），err 分支为死代码，
	// 已删。
	raw, _ := json.Marshal(pageSpec)
	return pageSpec, string(raw), nil
}
