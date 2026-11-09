package console

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func parsePublishedPages(models []model.PublishedPageSpec) []spec.PublishedPageSpec {
	pages := make([]spec.PublishedPageSpec, 0, len(models))
	for _, pp := range models {
		pageSpec := parsePublishedPageSpec(pp)
		if pageSpec != nil {
			pages = append(pages, *pageSpec)
		}
	}
	return pages
}

func parsePublishedPageSpec(pp model.PublishedPageSpec) *spec.PublishedPageSpec {
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(pp.SpecJSON), &pageSpec); err != nil || pageSpec.PageKey == "" {
		return nil
	}
	var contracts []spec.BindingContractSnapshot
	if pp.BindingContractsJSON != "" {
		_ = json.Unmarshal([]byte(pp.BindingContractsJSON), &contracts)
	}
	return &spec.PublishedPageSpec{
		PageSpec:              pageSpec,
		GameID:                pp.GameID,
		Env:                   pp.Env,
		Version:               pp.Version,
		PublishedAt:           pp.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
		PublishedBy:           pp.PublishedBy,
		RendererSchemaVersion: pp.RendererSchemaVersion,
		BaseProposalKey:       pp.BaseProposalKey,
		BaseProposalVersion:   pp.BaseProposalVersion,
		FunctionDigest:        pp.FunctionDigest,
		SemanticsDigest:       pp.SemanticsDigest,
		GeneratorVersion:      pp.GeneratorVersion,
		BindingContracts:      contracts,
	}
}

func (s *Service) attachBindingFreshness(ctx context.Context, pages []spec.PublishedPageSpec) ([]spec.PublishedPageSpec, error) {
	if len(pages) == 0 {
		return pages, nil
	}
	functions, err := loadPublishedFunctionSpecs(ctx, s.svcCtx)
	if err != nil {
		return nil, err
	}
	for i := range pages {
		pages[i].BindingFreshness = freshness.EvaluatePublishedBindings(pages[i].Bindings, pages[i].BindingContracts, functions)
	}
	return pages, nil
}

func (s *Service) attachBindingFreshnessToPage(ctx context.Context, page *spec.PublishedPageSpec) error {
	if page == nil {
		return nil
	}
	functions, err := loadPublishedFunctionSpecs(ctx, s.svcCtx)
	if err != nil {
		return err
	}
	page.BindingFreshness = freshness.EvaluatePublishedBindings(page.Bindings, page.BindingContracts, functions)
	return nil
}

func bindingFreshnessStatuses(diags []spec.BindingFreshnessDiagnostic) []string {
	statuses := make([]string, 0, len(diags))
	for _, diag := range diags {
		if diag.Status != "" {
			statuses = append(statuses, string(diag.Status))
		}
	}
	sort.Strings(statuses)
	return statuses
}
