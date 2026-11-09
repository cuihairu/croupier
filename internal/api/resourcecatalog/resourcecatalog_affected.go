package resourcecatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/freshness"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

func (s *Service) buildAffectedPages(ctx context.Context, gameID, env, resourceKey string) ([]AffectedPageInfo, error) {
	byKey := map[string]*AffectedPageInfo{}
	drafts, err := s.pageModel.ListByScope(ctx, gameID, env)
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list page drafts: %w", err)
	}
	for _, draft := range drafts {
		if strings.TrimSpace(draft.ResourceKey) != resourceKey {
			continue
		}
		byKey["draft:"+draft.PageKey] = &AffectedPageInfo{
			PageKey:          draft.PageKey,
			PageType:         draft.Type,
			Title:            draft.GetTitle(),
			Kind:             "draft",
			Status:           draft.Status,
			DraftRevision:    draft.DraftRevision,
			PublishedVersion: draft.PublishedVersion,
			UpdatedAt:        draft.UpdatedAt.Format(time.RFC3339),
		}
	}

	publishedPages, err := s.publishedModel.ListByScope(ctx, gameID, env)
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list published pages: %w", err)
	}
	functions := s.functionSpecsByID(ctx, gameID, env)
	for _, published := range publishedPages {
		pageSpec := parsePublishedPageSpec(published)
		if strings.TrimSpace(pageSpec.ResourceKey) != resourceKey {
			continue
		}
		bindingFreshness := freshness.EvaluatePublishedBindings(pageSpec.Bindings, parseBindingContracts(published.BindingContractsJSON), functions)
		byKey["published:"+published.PageKey] = &AffectedPageInfo{
			PageKey:          published.PageKey,
			PageType:         string(pageSpec.Type),
			Title:            pageSpec.Title,
			Kind:             "published",
			Status:           activeStatus(published.Active),
			PublishedVersion: published.Version,
			Stale:            len(bindingFreshness) > 0,
			BindingFreshness: bindingFreshness,
			UpdatedAt:        published.PublishedAt.Format(time.RFC3339),
		}
	}

	proposals, err := s.proposalModel.ListByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		if isMissingTableErr(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	for _, proposal := range proposals {
		byKey["proposal:"+proposal.ProposalKey] = &AffectedPageInfo{
			PageKey:         proposal.PageKey,
			PageType:        proposal.PageType,
			Title:           toStringMap(proposal.Title),
			Kind:            "proposal",
			ProposalKey:     proposal.ProposalKey,
			ProposalQuality: proposal.Quality,
			ProposalStatus:  proposal.Status.String(),
			Status:          proposal.Status.String(),
			UpdatedAt:       proposal.UpdatedAt.Format(time.RFC3339),
		}
	}

	items := make([]AffectedPageInfo, 0, len(byKey))
	for _, item := range byKey {
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return affectedKindOrder(items[i].Kind) < affectedKindOrder(items[j].Kind)
		}
		return items[i].PageKey < items[j].PageKey
	})
	return items, nil
}

func (s *Service) functionSpecsByID(ctx context.Context, gameID, env string) map[string]spec.FunctionSpec {
	contracts, err := s.contractModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return map[string]spec.FunctionSpec{}
	}
	out := make(map[string]spec.FunctionSpec, len(contracts))
	for _, contract := range contracts {
		if contract == nil || strings.TrimSpace(contract.FunctionID) == "" {
			continue
		}
		out[contract.FunctionID] = spec.FunctionSpec{
			ID:           contract.FunctionID,
			Version:      contract.Version,
			Enabled:      contract.Enabled,
			InputSchema:  spec.JSONSchema(contract.InputSchema),
			OutputSchema: spec.JSONSchema(contract.OutputSchema),
			Risk:         spec.RiskLevel(contract.Risk.String()),
			Permission:   strings.TrimSpace(contract.Permission),
		}
	}
	return out
}

func parsePublishedPageSpec(published model.PublishedPageSpec) spec.PageSpec {
	var pageSpec spec.PageSpec
	if strings.TrimSpace(published.SpecJSON) != "" {
		_ = json.Unmarshal([]byte(published.SpecJSON), &pageSpec)
	}
	if strings.TrimSpace(pageSpec.PageKey) == "" {
		pageSpec.PageKey = published.PageKey
	}
	return pageSpec
}

func parseBindingContracts(raw string) []spec.BindingContractSnapshot {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var contracts []spec.BindingContractSnapshot
	_ = json.Unmarshal([]byte(raw), &contracts)
	return contracts
}

func activeStatus(active bool) string {
	if active {
		return "active"
	}
	return "inactive"
}

func affectedKindOrder(kind string) int {
	switch kind {
	case "published":
		return 0
	case "draft":
		return 1
	case "proposal":
		return 2
	default:
		return 3
	}
}

func isMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "undefined table")
}

func decodeDiagnostics(raw []byte, fallbackFunctionID string) []DiagnosticInfo {
	if len(raw) == 0 {
		return nil
	}
	var values []spec.Diagnostic
	if err := json.Unmarshal(raw, &values); err != nil {
		return []DiagnosticInfo{{
			Code:       "diagnostic_parse_failed",
			Severity:   string(spec.SeverityWarning),
			Message:    "diagnostics payload is not readable",
			FunctionID: fallbackFunctionID,
		}}
	}
	out := make([]DiagnosticInfo, 0, len(values))
	for _, value := range values {
		functionID := value.FunctionID
		if functionID == "" {
			functionID = fallbackFunctionID
		}
		out = append(out, DiagnosticInfo{
			Code:       value.Code,
			Severity:   string(value.Severity),
			Message:    value.Message,
			FunctionID: functionID,
			Field:      value.Field,
		})
	}
	return out
}

func countUnresolvedConflicts(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	var conflicts []spec.SemanticConflict
	if err := json.Unmarshal(raw, &conflicts); err != nil {
		return 0
	}
	count := 0
	for _, conflict := range conflicts {
		if conflict.Resolution == "" {
			count++
		}
	}
	return count
}

func toStringMap(m map[string]interface{}) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

// humanizeResourceKey turns "inventory" / "player_item" into "Inventory" /
// "Player Item" so the catalog still shows a readable name before platform
// review fills in proper localized labels.
func humanizeResourceKey(key string) string {
	key = strings.Trim(strings.TrimSpace(key), "._-")
	if key == "" {
		return ""
	}
	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	// strings.FieldsFunc 不产生空片段（连续分隔符被跳过），空片段分支为死代码，已删除。
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

// categoryKeyForResource falls back to the key prefix ("mail.template" ->
// "mail") when no reviewed category exists, matching the page generator's
// InferCategoryFromKey behavior.
func categoryKeyForResource(resourceKey, reviewed string) string {
	if reviewed != "" {
		return reviewed
	}
	resourceKey = strings.TrimSpace(resourceKey)
	if idx := strings.Index(resourceKey, "."); idx > 0 {
		return resourceKey[:idx]
	}
	return resourceKey
}

// labelsForResource returns reviewed labels when present, otherwise a
// humanized fallback derived from the resource key. Derived values are
// display-only: the stored capability keeps empty labels until reviewed.
func labelsForResource(labels map[string]interface{}, resourceKey string) map[string]string {
	reviewed := toStringMap(labels)
	if len(reviewed) > 0 {
		return reviewed
	}
	if text := humanizeResourceKey(resourceKey); text != "" {
		return map[string]string{"zh-CN": text, "en-US": text}
	}
	return nil
}

func matchesQuery(item ResourceCatalogItem, query string) bool {
	q := strings.ToLower(query)
	if strings.Contains(strings.ToLower(item.ResourceKey), q) {
		return true
	}
	for _, label := range item.Labels {
		if strings.Contains(strings.ToLower(label), q) {
			return true
		}
	}
	return false
}
