package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
)

func (s *ProposalService) listContractChanges(ctx context.Context, gameID, env, resourceKey string) ([]ContractChangeDTO, error) {
	functions, err := s.functionSpecsByID(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	publishedChanges, err := s.publishedContractChanges(ctx, gameID, env, strings.TrimSpace(resourceKey), functions)
	if err != nil {
		return nil, err
	}
	draftChanges, err := s.draftContractChanges(ctx, gameID, env, strings.TrimSpace(resourceKey), publishedChanges, functions)
	if err != nil {
		return nil, err
	}
	out := append(publishedChanges, draftChanges...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].PageKey < out[j].PageKey
	})
	return out, nil
}

func (s *ProposalService) publishedContractChanges(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	functions map[string]spec.FunctionSpec,
) ([]ContractChangeDTO, error) {
	published, err := s.publishedModel.ListLatestActiveByScope(ctx, gameID, env)
	if err != nil {
		if isMissingTableErr(err) {
			return []ContractChangeDTO{}, nil
		}
		return nil, err
	}
	out := make([]ContractChangeDTO, 0)
	for _, item := range published {
		page, contracts := parsePublishedSnapshot(item)
		if page.PageKey == "" {
			continue
		}
		if resourceKey != "" && strings.TrimSpace(page.ResourceKey) != resourceKey {
			continue
		}
		diags := freshness.EvaluatePublishedBindings(page.Bindings, contracts, functions)
		if len(diags) == 0 {
			continue
		}
		out = append(out, ContractChangeDTO{
			PageKey:          item.PageKey,
			PageType:         page.Type,
			ResourceKey:      strings.TrimSpace(page.ResourceKey),
			Title:            page.Title,
			CategoryKey:      strings.TrimSpace(page.Category.Key),
			Kind:             "published",
			Status:           "stale",
			PublishedVersion: item.Version,
			BindingFreshness: diags,
			UpdatedAt:        item.PublishedAt.Format(time.RFC3339),
			UpdatedBy:        item.PublishedBy,
		})
	}
	return out, nil
}

func (s *ProposalService) draftContractChanges(
	ctx context.Context,
	gameID string,
	env string,
	resourceKey string,
	publishedChanges []ContractChangeDTO,
	functions map[string]spec.FunctionSpec,
) ([]ContractChangeDTO, error) {
	drafts, err := s.pageModel.ListByScope(ctx, gameID, env)
	if err != nil {
		if isMissingTableErr(err) {
			return []ContractChangeDTO{}, nil
		}
		return nil, err
	}
	publishedStale := make(map[string][]spec.BindingFreshnessDiagnostic, len(publishedChanges))
	for _, change := range publishedChanges {
		publishedStale[change.PageKey] = change.BindingFreshness
	}
	out := make([]ContractChangeDTO, 0)
	for _, draft := range drafts {
		if resourceKey != "" && strings.TrimSpace(draft.ResourceKey) != resourceKey {
			continue
		}
		page := parseDraftPageSpec(draft)
		diags := staleDiagnosticsForDraft(page, functions)
		if len(diags) == 0 && draft.PublishedVersion > 0 {
			diags = publishedStale[draft.PageKey]
		}
		if len(diags) == 0 {
			continue
		}
		out = append(out, ContractChangeDTO{
			PageKey:          draft.PageKey,
			PageType:         spec.PageType(draft.Type),
			ResourceKey:      strings.TrimSpace(draft.ResourceKey),
			Title:            draft.GetTitle(),
			CategoryKey:      strings.TrimSpace(draft.CategoryKey),
			Kind:             "draft",
			Status:           draft.Status,
			DraftRevision:    draft.DraftRevision,
			PublishedVersion: draft.PublishedVersion,
			BindingFreshness: diags,
			UpdatedAt:        draft.UpdatedAt.Format(time.RFC3339),
			UpdatedBy:        draft.UpdatedBy,
		})
	}
	return out, nil
}
