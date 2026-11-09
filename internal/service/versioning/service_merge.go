package versioning

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	dashboardmerge "github.com/cuihairu/croupier/internal/dashboard/merge"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	"github.com/cuihairu/croupier/internal/model"
)

func (s *Service) mergePreviewForPage(
	ctx context.Context,
	gameID string,
	env string,
	pageKey string,
	page *model.PageSpec,
	proposal *model.PageProposal,
) (dashboardmerge.MergeResult, error) {
	if page == nil {
		return dashboardmerge.MergeResult{}, errorx.NewNotFound("page draft not found")
	}
	proposalKey := strings.TrimSpace(page.BaseProposalKey)
	if proposalKey == "" || page.BaseProposalVersion <= 0 || proposal == nil {
		return dashboardmerge.MergeResult{}, nil
	}
	if strings.TrimSpace(proposal.PageKey) != strings.TrimSpace(pageKey) {
		return dashboardmerge.MergeResult{}, nil
	}
	baseVersion, err := s.proposalVersionModel.FindByProposalIDAndVersion(ctx, proposal.ID, page.BaseProposalVersion)
	if err != nil {
		return dashboardmerge.MergeResult{}, nil
	}
	basePage, err := pageSpecFromProposalSnapshot(json.RawMessage(baseVersion.Proposal))
	if err != nil {
		return dashboardmerge.MergeResult{}, err
	}
	draftPage, err := pageSpecFromModel(page)
	if err != nil {
		return dashboardmerge.MergeResult{}, err
	}
	latestPage, err := pageSpecFromProposalModel(proposal)
	if err != nil {
		return dashboardmerge.MergeResult{}, err
	}
	return dashboardmerge.ThreeWayMerge(basePage, draftPage, latestPage), nil
}

// MergeStrategy represents how to handle conflicts.
type MergeStrategy string

const (
	MergeStrategyAuto   MergeStrategy = "auto"   // Auto-merge safe changes
	MergeStrategyAccept MergeStrategy = "accept" // Accept all incoming changes
	MergeStrategyReject MergeStrategy = "reject" // Reject all incoming changes
	MergeStrategyManual MergeStrategy = "manual" // Manual conflict resolution
)

// MergeRequest is the request for merging changes.
type MergeRequest struct {
	GameID                string               `json:"-"`
	Env                   string               `json:"-"`
	PageKey               string               `uri:"pageKey" json:"-"`
	ExpectedDraftRevision int                  `json:"expectedDraftRevision,omitempty"`
	Strategy              MergeStrategy        `json:"strategy"`
	DryRun                bool                 `json:"dryRun,omitempty"`
	Conflicts             []ConflictResolution `json:"conflicts,omitempty"`
	Reason                string               `json:"reason,omitempty"`
}

// ConflictResolution represents how to resolve a specific conflict.
type ConflictResolution struct {
	Path      string          `json:"path"`
	AcceptNew bool            `json:"acceptNew"`       // true = accept new value, false = keep old
	Value     json.RawMessage `json:"value,omitempty"` // custom value if neither old nor new
}

// MergeResponse is the response for merge operation.
type MergeResponse struct {
	Merged         int                            `json:"merged"`    // number of auto-merged changes
	Conflicts      int                            `json:"conflicts"` // number of conflicts requiring resolution
	Message        string                         `json:"message"`
	DraftRevision  int                            `json:"draftRevision,omitempty"`
	AutoMergeItems []dashboardmerge.MergeItem     `json:"autoMergeItems,omitempty"`
	ConflictItems  []dashboardmerge.MergeConflict `json:"conflictItems,omitempty"`
}

func draftRevisionConflict(expected, current int) error {
	return errorx.NewConflictWithDetails("page draft revision conflict", map[string]any{
		"expected": expected,
		"current":  current,
	})
}

// Merge applies changes with the given strategy.
// threeWayMerge 是 dashboardmerge.ThreeWayMerge 的包级接缝：生产恒为真实
// 实现，测试注入携带白名单外 field 的 AutoMerge 项以驱动 Merge 内
// applyAutoMergeItems 的 fail-fast 分支（ThreeWayMerge 产出项恒在白名单内，
// 不可达论证见各调用处注释）。
var threeWayMerge = dashboardmerge.ThreeWayMerge

func (s *Service) Merge(ctx context.Context, req *MergeRequest) (*MergeResponse, error) {
	if req.Strategy != MergeStrategyAuto &&
		req.Strategy != MergeStrategyAccept &&
		req.Strategy != MergeStrategyReject &&
		req.Strategy != MergeStrategyManual {
		return nil, errorx.NewBadRequest("unknown merge strategy: " + string(req.Strategy))
	}
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	if req.Strategy == MergeStrategyReject {
		return &MergeResponse{Message: "all changes rejected"}, nil
	}
	if req.Strategy == MergeStrategyAccept {
		return nil, errorx.NewValidationError("accept-all merge is forbidden; resolve execution-affecting conflicts explicitly")
	}
	if !req.DryRun && req.ExpectedDraftRevision <= 0 {
		return nil, errorx.NewBadRequest("expectedDraftRevision is required")
	}

	page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	if !req.DryRun && page.DraftRevision != req.ExpectedDraftRevision {
		return nil, draftRevisionConflict(req.ExpectedDraftRevision, page.DraftRevision)
	}
	proposalKey := strings.TrimSpace(page.BaseProposalKey)
	if proposalKey == "" || page.BaseProposalVersion <= 0 {
		return nil, errorx.NewConflict("page draft has no base proposal snapshot; regenerate or accept a new proposal before merge")
	}
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, req.GameID, req.Env, proposalKey)
	if err != nil {
		return nil, errorx.NewNotFound("latest proposal not found")
	}
	if strings.TrimSpace(proposal.PageKey) != pageKey {
		return nil, errorx.NewConflict("latest proposal pageKey does not match the requested page")
	}
	latestVersion, err := s.proposalVersionModel.LatestByProposalID(ctx, proposal.ID)
	if err != nil {
		return nil, errorx.NewConflict("latest proposal snapshot not found; regenerate proposal before merge")
	}
	baseVersion, err := s.proposalVersionModel.FindByProposalIDAndVersion(ctx, proposal.ID, page.BaseProposalVersion)
	if err != nil {
		return nil, errorx.NewConflict("base proposal snapshot not found; regenerate or accept a new proposal before merge")
	}
	basePage, err := pageSpecFromProposalSnapshot(json.RawMessage(baseVersion.Proposal))
	if err != nil {
		return nil, err
	}
	draftPage, err := pageSpecFromModel(page)
	if err != nil {
		return nil, err
	}
	latestPage, err := pageSpecFromProposalModel(proposal)
	if err != nil {
		return nil, err
	}

	mergeResult := threeWayMerge(basePage, draftPage, latestPage)
	if req.DryRun {
		return &MergeResponse{
			Merged:         len(mergeResult.AutoMerge),
			Conflicts:      len(mergeResult.Conflicts),
			Message:        mergePreviewMessage(len(mergeResult.AutoMerge), len(mergeResult.Conflicts), latestVersion.Version != page.BaseProposalVersion),
			DraftRevision:  page.DraftRevision,
			AutoMergeItems: mergeResult.AutoMerge,
			ConflictItems:  mergeResult.Conflicts,
		}, nil
	}
	if req.Strategy == MergeStrategyManual {
		resolutions, err := validateManualConflictResolutions(mergeResult.Conflicts, req.Conflicts)
		if err != nil {
			return nil, err
		}
		// 不可达论证（C 类）：applyAutoMergeItems 对 ThreeWayMerge 产出的
		// items 恒成功——①非索引字段的 MergedValue 为 toJSON(同类型字段)
		// 产物，decode 回同类型恒成功；②索引字段的 index 由 compareColumns/
		// compareDetailFields 等的三方存在性检查保证小于 len(draft)，apply
		// 目标即同一 draftPage（normalizePageSpec 不改数组长度），越界分支
		// 不可达；③AutoMergeFields 白名单的每个 key 均被 applyAutoMergeItem
		// 的 switch 或 applyIndexedAutoMergeItem 覆盖。保留检查原因：白名单
		// （dashboardmerge 包）与处理器（本包）是跨包隐式契约，合并器新增
		// key 未同步时 fail-fast 报错优于静默丢弃 auto-merge 项。
		mergedPage, err := applyAutoMergeItems(draftPage, mergeResult.AutoMerge)
		if err != nil {
			return nil, err
		}
		mergedPage, err = applyConflictResolutions(mergedPage, mergeResult.Conflicts, resolutions)
		if err != nil {
			return nil, err
		}
		nextRevision, err := s.persistMergedDraft(ctx, persistMergedDraftParams{
			Request:             req,
			PageKey:             pageKey,
			Current:             page,
			MergedPage:          mergedPage,
			ProposalKey:         proposalKey,
			NextBaseProposalVer: latestVersion.Version,
			Message:             firstNonEmpty(req.Reason, "resolved merge conflicts against latest proposal"),
			ForceRevision:       true,
		})
		if err != nil {
			return nil, err
		}
		return &MergeResponse{
			Merged:         len(mergeResult.AutoMerge) + len(mergeResult.Conflicts),
			Conflicts:      0,
			Message:        manualMergeMessage(len(mergeResult.AutoMerge), len(mergeResult.Conflicts)),
			DraftRevision:  nextRevision,
			AutoMergeItems: mergeResult.AutoMerge,
		}, nil
	}
	if len(mergeResult.AutoMerge) == 0 {
		if len(mergeResult.Conflicts) == 0 && latestVersion.Version != page.BaseProposalVersion {
			nextRevision, err := s.persistMergedDraft(ctx, persistMergedDraftParams{
				Request:             req,
				PageKey:             pageKey,
				Current:             page,
				MergedPage:          draftPage,
				ProposalKey:         proposalKey,
				NextBaseProposalVer: latestVersion.Version,
				Message:             firstNonEmpty(req.Reason, "accepted latest proposal snapshot without page content changes"),
				ForceRevision:       true,
			})
			if err != nil {
				return nil, err
			}
			return &MergeResponse{
				Merged:         0,
				Conflicts:      0,
				Message:        "accepted latest proposal snapshot without page content changes",
				DraftRevision:  nextRevision,
				AutoMergeItems: mergeResult.AutoMerge,
				ConflictItems:  mergeResult.Conflicts,
			}, nil
		}
		return &MergeResponse{
			Merged:         0,
			Conflicts:      len(mergeResult.Conflicts),
			Message:        mergeMessage(0, len(mergeResult.Conflicts), false),
			DraftRevision:  page.DraftRevision,
			AutoMergeItems: mergeResult.AutoMerge,
			ConflictItems:  mergeResult.Conflicts,
		}, nil
	}

	// 不可达论证（C 类）：同上方 MergeStrategyManual 分支——apply 对
	// ThreeWayMerge 产出的 items 恒成功，保留检查仅为跨包白名单失配时
	// fail-fast。
	mergedPage, err := applyAutoMergeItems(draftPage, mergeResult.AutoMerge)
	if err != nil {
		return nil, err
	}
	if samePageSpec(mergedPage, draftPage) && len(mergeResult.Conflicts) > 0 {
		return &MergeResponse{
			Merged:         len(mergeResult.AutoMerge),
			Conflicts:      len(mergeResult.Conflicts),
			Message:        mergeMessage(len(mergeResult.AutoMerge), len(mergeResult.Conflicts), false),
			DraftRevision:  page.DraftRevision,
			AutoMergeItems: mergeResult.AutoMerge,
			ConflictItems:  mergeResult.Conflicts,
		}, nil
	}
	nextBaseProposalVer := page.BaseProposalVersion
	if len(mergeResult.Conflicts) == 0 {
		nextBaseProposalVer = latestVersion.Version
	}
	nextRevision, err := s.persistMergedDraft(ctx, persistMergedDraftParams{
		Request:             req,
		PageKey:             pageKey,
		Current:             page,
		MergedPage:          mergedPage,
		ProposalKey:         proposalKey,
		NextBaseProposalVer: nextBaseProposalVer,
		Message:             firstNonEmpty(req.Reason, "auto-merge safe display changes from latest proposal"),
		ForceRevision:       !samePageSpec(mergedPage, draftPage) || len(mergeResult.Conflicts) == 0,
	})
	if err != nil {
		return nil, err
	}
	return &MergeResponse{
		Merged:         len(mergeResult.AutoMerge),
		Conflicts:      len(mergeResult.Conflicts),
		Message:        mergeMessage(len(mergeResult.AutoMerge), len(mergeResult.Conflicts), true),
		DraftRevision:  nextRevision,
		AutoMergeItems: mergeResult.AutoMerge,
		ConflictItems:  mergeResult.Conflicts,
	}, nil
}

func applyAutoMergeItems(page spec.PageSpec, items []dashboardmerge.MergeItem) (spec.PageSpec, error) {
	page = normalizePageSpec(page)
	for _, item := range items {
		if err := applyAutoMergeItem(&page, item); err != nil {
			return spec.PageSpec{}, err
		}
	}
	return page, nil
}

func applyAutoMergeItem(page *spec.PageSpec, item dashboardmerge.MergeItem) error {
	switch item.Field {
	case "title":
		return decodeMergeValue(item, &page.Title)
	case "description":
		return decodeMergeValue(item, &page.Description)
	case "icon":
		return decodeMergeValue(item, &page.Icon)
	case "order":
		return decodeMergeValue(item, &page.Order)
	case "category.order":
		return decodeMergeValue(item, &page.Category.Order)
	case "navigation.title":
		if page.Navigation == nil {
			page.Navigation = &spec.NavigationSpec{}
		}
		return decodeMergeValue(item, &page.Navigation.Title)
	case "navigation.breadcrumb":
		if page.Navigation == nil {
			page.Navigation = &spec.NavigationSpec{}
		}
		return decodeMergeValue(item, &page.Navigation.Breadcrumb)
	case "resource.listView.defaultSort":
		var value *spec.SortSpec
		if err := decodeMergeValue(item, &value); err != nil {
			return err
		}
		ensureResourceListView(page).DefaultSort = value
		return nil
	}
	if handled, err := applyIndexedAutoMergeItem(page, item); handled || err != nil {
		return err
	}
	return errorx.NewValidationError("unsupported auto-merge field: " + item.Field)
}

type persistMergedDraftParams struct {
	Request             *MergeRequest
	PageKey             string
	Current             *model.PageSpec
	MergedPage          spec.PageSpec
	ProposalKey         string
	NextBaseProposalVer int
	Message             string
	ForceRevision       bool
}

func (s *Service) persistMergedDraft(ctx context.Context, params persistMergedDraftParams) (int, error) {
	if params.Request == nil {
		return 0, errorx.NewValidationError("merge request is required")
	}
	current := params.Current
	if current == nil {
		return 0, errorx.NewNotFound("page draft not found")
	}
	mergedPage := normalizePageSpec(params.MergedPage)
	writeRevision := params.ForceRevision || !samePageSpec(mergedPage, mustPageSpecFromModel(current))
	if !writeRevision &&
		strings.TrimSpace(current.BaseProposalKey) == strings.TrimSpace(params.ProposalKey) &&
		current.BaseProposalVersion == params.NextBaseProposalVer {
		return current.DraftRevision, nil
	}

	actor := actorFromContext(ctx)
	now := time.Now()
	nextRevision := current.DraftRevision

	err := s.withTransaction(ctx, func(txCtx context.Context) error {
		pageModel := model.NewPageSpecModel(dbctx.Resolve(txCtx, s.db))
		versionModel := model.NewPageVersionModel(dbctx.Resolve(txCtx, s.db))

		latestCurrent, err := pageModel.FindByScopeAndPageKey(txCtx, params.Request.GameID, params.Request.Env, params.PageKey)
		if err != nil {
			return err
		}
		if latestCurrent.DraftRevision != params.Request.ExpectedDraftRevision {
			return draftRevisionConflict(params.Request.ExpectedDraftRevision, latestCurrent.DraftRevision)
		}
		if writeRevision {
			nextRevision, err = versionModel.GetNextVersion(txCtx, params.Request.GameID, params.Request.Env, params.PageKey)
			if err != nil {
				return err
			}
		}
		if err := applyPageSpecToModel(latestCurrent, mergedPage); err != nil {
			return err
		}
		latestCurrent.GameID = params.Request.GameID
		latestCurrent.Env = params.Request.Env
		latestCurrent.Status = "draft"
		latestCurrent.PublishedActive = current.PublishedActive
		latestCurrent.PublishedVersion = current.PublishedVersion
		if writeRevision {
			latestCurrent.DraftRevision = nextRevision
		}
		latestCurrent.BaseProposalKey = params.ProposalKey
		latestCurrent.BaseProposalVersion = params.NextBaseProposalVer
		latestCurrent.UpdatedAt = now
		latestCurrent.UpdatedBy = actor
		if err := pageModel.Upsert(txCtx, latestCurrent); err != nil {
			return err
		}
		if !writeRevision {
			return nil
		}
		// 同上：mergedPage 为 Unmarshal/合并产物，marshalPageSpec 恒成功。
		specJSON, _ := marshalPageSpec(mergedPage)
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    params.Request.GameID,
			Env:       params.Request.Env,
			PageKey:   params.PageKey,
			Version:   nextRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   params.Message,
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return 0, err
	}
	return nextRevision, nil
}

func applyIndexedAutoMergeItem(page *spec.PageSpec, item dashboardmerge.MergeItem) (bool, error) {
	if ok, index, leaf := parseIndexedMergeField(item.Field, "resource.listView.columns"); ok {
		if page.Resource == nil || page.Resource.ListView == nil || index < 0 || index >= len(page.Resource.ListView.Columns) {
			return true, errorx.NewValidationError("auto-merge field is out of range: " + item.Field)
		}
		column := &page.Resource.ListView.Columns[index]
		switch leaf {
		case "title":
			return true, decodeMergeValue(item, &column.Title)
		case "width":
			return true, decodeMergeValue(item, &column.Width)
		case "visible":
			return true, decodeMergeValue(item, &column.Visible)
		case "sortable":
			return true, decodeMergeValue(item, &column.Sortable)
		case "filterable":
			return true, decodeMergeValue(item, &column.Filterable)
		default:
			return true, errorx.NewValidationError("unsupported column auto-merge field: " + item.Field)
		}
	}
	if ok, index, leaf := parseIndexedMergeField(item.Field, "resource.detailView.fields"); ok {
		if page.Resource == nil || page.Resource.DetailView == nil || index < 0 || index >= len(page.Resource.DetailView.Fields) {
			return true, errorx.NewValidationError("auto-merge field is out of range: " + item.Field)
		}
		field := &page.Resource.DetailView.Fields[index]
		switch leaf {
		case "title":
			return true, decodeMergeValue(item, &field.Title)
		case "span":
			return true, decodeMergeValue(item, &field.Span)
		case "visible":
			return true, decodeMergeValue(item, &field.Visible)
		default:
			return true, errorx.NewValidationError("unsupported detail auto-merge field: " + item.Field)
		}
	}
	if ok, index, leaf := parseIndexedMergeField(item.Field, "operation.form.fields"); ok {
		if page.Operation == nil || page.Operation.Form == nil || index < 0 || index >= len(page.Operation.Form.Fields) {
			return true, errorx.NewValidationError("auto-merge field is out of range: " + item.Field)
		}
		return true, applyFormFieldAutoMergeItem(&page.Operation.Form.Fields[index], leaf, item)
	}
	if ok, index, leaf := parseIndexedMergeField(item.Field, "task.form.fields"); ok {
		if page.Task == nil || page.Task.Form == nil || index < 0 || index >= len(page.Task.Form.Fields) {
			return true, errorx.NewValidationError("auto-merge field is out of range: " + item.Field)
		}
		return true, applyFormFieldAutoMergeItem(&page.Task.Form.Fields[index], leaf, item)
	}
	if ok, index, leaf := parseIndexedMergeField(item.Field, "report.queryForm.fields"); ok {
		if page.Report == nil || page.Report.QueryForm == nil || index < 0 || index >= len(page.Report.QueryForm.Fields) {
			return true, errorx.NewValidationError("auto-merge field is out of range: " + item.Field)
		}
		return true, applyFormFieldAutoMergeItem(&page.Report.QueryForm.Fields[index], leaf, item)
	}
	if ok, index, leaf := parseIndexedMergeField(item.Field, "report.charts"); ok {
		if page.Report == nil || index < 0 || index >= len(page.Report.Charts) {
			return true, errorx.NewValidationError("auto-merge field is out of range: " + item.Field)
		}
		if leaf != "title" {
			return true, errorx.NewValidationError("unsupported chart auto-merge field: " + item.Field)
		}
		return true, decodeMergeValue(item, &page.Report.Charts[index].Title)
	}
	return false, nil
}

func applyFormFieldAutoMergeItem(field *spec.FormFieldSpec, leaf string, item dashboardmerge.MergeItem) error {
	switch leaf {
	case "label":
		return decodeMergeValue(item, &field.Label)
	case "placeholder":
		return decodeMergeValue(item, &field.Placeholder)
	case "description":
		return decodeMergeValue(item, &field.Description)
	case "order":
		return decodeMergeValue(item, &field.Order)
	case "widget":
		return decodeMergeValue(item, &field.Widget)
	default:
		return errorx.NewValidationError("unsupported form field auto-merge field: " + item.Field)
	}
}

func parseIndexedMergeField(field string, prefix string) (bool, int, string) {
	start := prefix + "["
	if !strings.HasPrefix(field, start) {
		return false, 0, ""
	}
	rest := strings.TrimPrefix(field, start)
	closeIndex := strings.Index(rest, "]")
	if closeIndex <= 0 || closeIndex+2 > len(rest) || rest[closeIndex+1] != '.' {
		return true, -1, ""
	}
	index, err := strconv.Atoi(rest[:closeIndex])
	if err != nil {
		return true, -1, ""
	}
	return true, index, rest[closeIndex+2:]
}

func decodeMergeValue(item dashboardmerge.MergeItem, target interface{}) error {
	if len(item.MergedValue) == 0 || string(item.MergedValue) == "null" {
		return nil
	}
	if err := json.Unmarshal(item.MergedValue, target); err != nil {
		return fmt.Errorf("decode auto-merge field %s: %w", item.Field, err)
	}
	return nil
}

func validateManualConflictResolutions(
	conflicts []dashboardmerge.MergeConflict,
	resolutions []ConflictResolution,
) (map[string]ConflictResolution, error) {
	expected := make(map[string]dashboardmerge.MergeConflict, len(conflicts))
	for _, conflict := range conflicts {
		expected[conflict.Field] = conflict
	}
	if len(expected) == 0 {
		if len(resolutions) > 0 {
			return nil, errorx.NewValidationError("manual merge received conflict resolutions, but the latest proposal has no conflicts")
		}
		return map[string]ConflictResolution{}, nil
	}

	byField := make(map[string]ConflictResolution, len(resolutions))
	extras := make([]string, 0)
	for _, resolution := range resolutions {
		field := strings.TrimSpace(resolution.Path)
		if field == "" {
			return nil, errorx.NewValidationError("manual merge conflict path is required")
		}
		if _, exists := byField[field]; exists {
			return nil, errorx.NewValidationError("manual merge conflict path duplicated: " + field)
		}
		if _, ok := expected[field]; !ok {
			extras = append(extras, field)
			continue
		}
		resolution.Path = field
		byField[field] = resolution
	}

	missing := make([]string, 0)
	for field := range expected {
		if _, ok := byField[field]; !ok {
			missing = append(missing, field)
		}
	}
	sort.Strings(missing)
	sort.Strings(extras)
	if len(missing) > 0 || len(extras) > 0 {
		parts := make([]string, 0, 2)
		if len(missing) > 0 {
			parts = append(parts, "missing resolutions for: "+strings.Join(missing, ", "))
		}
		if len(extras) > 0 {
			parts = append(parts, "unexpected resolutions for: "+strings.Join(extras, ", "))
		}
		return nil, errorx.NewValidationError("manual merge must resolve exactly the current conflicts; " + strings.Join(parts, "; "))
	}
	return byField, nil
}

func applyConflictResolutions(
	page spec.PageSpec,
	conflicts []dashboardmerge.MergeConflict,
	resolutions map[string]ConflictResolution,
) (spec.PageSpec, error) {
	page = normalizePageSpec(page)
	for _, conflict := range conflicts {
		resolution, ok := resolutions[conflict.Field]
		if !ok {
			return spec.PageSpec{}, errorx.NewValidationError("missing conflict resolution for: " + conflict.Field)
		}
		raw := resolveConflictRawValue(conflict, resolution)
		if err := applyConflictField(&page, conflict.Field, raw); err != nil {
			return spec.PageSpec{}, err
		}
	}
	return page, nil
}

func resolveConflictRawValue(
	conflict dashboardmerge.MergeConflict,
	resolution ConflictResolution,
) json.RawMessage {
	if len(resolution.Value) > 0 {
		return cloneRawJSON(resolution.Value)
	}
	if resolution.AcceptNew {
		return cloneRawJSON(conflict.LatestValue)
	}
	return cloneRawJSON(conflict.DraftValue)
}

func applyConflictField(page *spec.PageSpec, field string, raw json.RawMessage) error {
	switch field {
	case "type":
		return decodeRawJSONField(raw, field, &page.Type)
	case "resourceKey":
		return decodeRawJSONField(raw, field, &page.ResourceKey)
	case "category.key":
		return decodeRawJSONField(raw, field, &page.Category.Key)
	case "bindings":
		var bindings []spec.PageFunctionBinding
		if err := decodeRawJSONField(raw, field, &bindings); err != nil {
			return err
		}
		page.Bindings = bindings
		return nil
	case "resource.listView.identityKey":
		listView := ensureResourceListView(page)
		return decodeRawJSONField(raw, field, &listView.IdentityKey)
	case "resource.listView.rowSchema":
		listView := ensureResourceListView(page)
		listView.RowSchema = cloneJSONSchema(raw)
		return nil
	case "resource.listView.defaultSort":
		var value *spec.SortSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureResourceListView(page).DefaultSort = value
		return nil
	case "resource.listView.filters":
		var value []spec.FilterSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureResourceListView(page).Filters = value
		return nil
	case "resource.listView.pagination":
		var value *spec.PaginationSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureResourceListView(page).Pagination = value
		return nil
	case "resource.listView.rowActions":
		listView := ensureResourceListView(page)
		return decodeActionListField(raw, field, &listView.RowActions)
	case "resource.listView.batchActions":
		listView := ensureResourceListView(page)
		return decodeActionListField(raw, field, &listView.BatchActions)
	case "resource.listView.toolbarActions":
		listView := ensureResourceListView(page)
		return decodeActionListField(raw, field, &listView.ToolbarActions)
	case "resource.detailView.actions":
		detailView := ensureResourceDetailView(page)
		return decodeActionListField(raw, field, &detailView.Actions)
	case "resource.actions":
		resource := ensureResourcePage(page)
		return decodeActionListField(raw, field, &resource.Actions)
	case "resource.createForm":
		var value *spec.FormPresentationSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureResourcePage(page).CreateForm = value
		return nil
	case "resource.updateForm":
		var value *spec.FormPresentationSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureResourcePage(page).UpdateForm = value
		return nil
	case "resource.deleteAction":
		var value *spec.ConfirmActionSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureResourcePage(page).DeleteAction = value
		return nil
	case "operation.form.jsonSchema":
		ensureOperationForm(page).JSONSchema = cloneJSONSchema(raw)
		return nil
	case "operation.confirm":
		var value *spec.ConfirmActionSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureOperationPage(page).Confirm = value
		return nil
	case "operation.resultView":
		var value *spec.ResultViewSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureOperationPage(page).ResultView = value
		return nil
	case "task.form.jsonSchema":
		ensureTaskForm(page).JSONSchema = cloneJSONSchema(raw)
		return nil
	case "task.taskView":
		var value *spec.TaskViewSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureTaskPage(page).TaskView = value
		return nil
	case "task.resultView":
		var value *spec.ResultViewSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureTaskPage(page).ResultView = value
		return nil
	case "report.queryForm.jsonSchema":
		ensureReportQueryForm(page).JSONSchema = cloneJSONSchema(raw)
		return nil
	case "report.dataset":
		var value *spec.DatasetSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureReportPage(page).Dataset = value
		return nil
	case "report.table":
		var value *spec.ListViewSpec
		if err := decodeRawJSONField(raw, field, &value); err != nil {
			return err
		}
		ensureReportPage(page).Table = value
		return nil
	}
	if handled, err := applyIndexedConflictField(page, field, raw); handled || err != nil {
		return err
	}
	return errorx.NewValidationError("unsupported manual merge field: " + field)
}

func applyIndexedConflictField(page *spec.PageSpec, field string, raw json.RawMessage) (bool, error) {
	if ok, index, leaf := parseIndexedMergeField(field, "operation.form.fields"); ok {
		form := ensureOperationForm(page)
		if index < 0 || index >= len(form.Fields) {
			return true, errorx.NewValidationError("manual merge field is out of range: " + field)
		}
		return true, applyFormFieldConflictValue(&form.Fields[index], leaf, raw, field)
	}
	if ok, index, leaf := parseIndexedMergeField(field, "task.form.fields"); ok {
		form := ensureTaskForm(page)
		if index < 0 || index >= len(form.Fields) {
			return true, errorx.NewValidationError("manual merge field is out of range: " + field)
		}
		return true, applyFormFieldConflictValue(&form.Fields[index], leaf, raw, field)
	}
	if ok, index, leaf := parseIndexedMergeField(field, "report.queryForm.fields"); ok {
		form := ensureReportQueryForm(page)
		if index < 0 || index >= len(form.Fields) {
			return true, errorx.NewValidationError("manual merge field is out of range: " + field)
		}
		return true, applyFormFieldConflictValue(&form.Fields[index], leaf, raw, field)
	}
	return false, nil
}

func applyFormFieldConflictValue(
	field *spec.FormFieldSpec,
	leaf string,
	raw json.RawMessage,
	fullField string,
) error {
	switch leaf {
	case "key":
		return decodeRawJSONField(raw, fullField, &field.Key)
	case "visibleWhen":
		var value *spec.ConditionSpec
		if err := decodeRawJSONField(raw, fullField, &value); err != nil {
			return err
		}
		field.VisibleWhen = value
		return nil
	case "required":
		var value *bool
		if err := decodeRawJSONField(raw, fullField, &value); err != nil {
			return err
		}
		field.Required = value
		return nil
	case "defaultValue":
		field.DefaultValue = cloneRawJSON(raw)
		return nil
	case "disabled":
		var value *bool
		if err := decodeRawJSONField(raw, fullField, &value); err != nil {
			return err
		}
		field.Disabled = value
		return nil
	case "widgetProps":
		var value map[string]json.RawMessage
		if err := decodeRawJSONField(raw, fullField, &value); err != nil {
			return err
		}
		field.WidgetProps = value
		return nil
	case "validationRules":
		var value []spec.ValidationRule
		if err := decodeRawJSONField(raw, fullField, &value); err != nil {
			return err
		}
		field.ValidationRules = value
		return nil
	default:
		return errorx.NewValidationError("unsupported manual merge form field: " + fullField)
	}
}

func decodeActionListField(raw json.RawMessage, field string, target *[]spec.ActionSpec) error {
	var value []spec.ActionSpec
	if err := decodeRawJSONField(raw, field, &value); err != nil {
		return err
	}
	*target = value
	return nil
}

func decodeRawJSONField(raw json.RawMessage, field string, target interface{}) error {
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode manual merge field %s: %w", field, err)
	}
	return nil
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

func cloneJSONSchema(raw json.RawMessage) spec.JSONSchema {
	if len(raw) == 0 {
		return nil
	}
	cloned := cloneRawJSON(raw)
	return spec.JSONSchema(cloned)
}

func ensureResourcePage(page *spec.PageSpec) *spec.ResourcePageSpec {
	if page.Resource == nil {
		page.Resource = &spec.ResourcePageSpec{}
	}
	return page.Resource
}

func ensureResourceListView(page *spec.PageSpec) *spec.ListViewSpec {
	resource := ensureResourcePage(page)
	if resource.ListView == nil {
		resource.ListView = &spec.ListViewSpec{}
	}
	return resource.ListView
}

func ensureResourceDetailView(page *spec.PageSpec) *spec.DetailViewSpec {
	resource := ensureResourcePage(page)
	if resource.DetailView == nil {
		resource.DetailView = &spec.DetailViewSpec{}
	}
	return resource.DetailView
}

func ensureOperationPage(page *spec.PageSpec) *spec.OperationPageSpec {
	if page.Operation == nil {
		page.Operation = &spec.OperationPageSpec{}
	}
	return page.Operation
}

func ensureOperationForm(page *spec.PageSpec) *spec.FormPresentationSpec {
	operation := ensureOperationPage(page)
	if operation.Form == nil {
		operation.Form = &spec.FormPresentationSpec{}
	}
	return operation.Form
}

func ensureTaskPage(page *spec.PageSpec) *spec.TaskPageSpec {
	if page.Task == nil {
		page.Task = &spec.TaskPageSpec{}
	}
	return page.Task
}

func ensureTaskForm(page *spec.PageSpec) *spec.FormPresentationSpec {
	task := ensureTaskPage(page)
	if task.Form == nil {
		task.Form = &spec.FormPresentationSpec{}
	}
	return task.Form
}

func ensureReportPage(page *spec.PageSpec) *spec.ReportPageSpec {
	if page.Report == nil {
		page.Report = &spec.ReportPageSpec{}
	}
	return page.Report
}

func ensureReportQueryForm(page *spec.PageSpec) *spec.FormPresentationSpec {
	report := ensureReportPage(page)
	if report.QueryForm == nil {
		report.QueryForm = &spec.FormPresentationSpec{}
	}
	return report.QueryForm
}

func mergeMessage(merged int, conflicts int, changed bool) string {
	if merged == 0 && conflicts == 0 {
		return "no contract changes require merge"
	}
	if !changed {
		return fmt.Sprintf("found %d safe changes and %d conflicts; no draft change written", merged, conflicts)
	}
	if conflicts > 0 {
		return fmt.Sprintf("auto-merged %d safe changes; %d conflicts still require manual review", merged, conflicts)
	}
	return fmt.Sprintf("auto-merged %d safe changes", merged)
}

func manualMergeMessage(autoMerged int, resolved int) string {
	if autoMerged == 0 && resolved == 0 {
		return "accepted latest proposal snapshot"
	}
	if autoMerged == 0 {
		return fmt.Sprintf("resolved %d conflicts against the latest proposal", resolved)
	}
	return fmt.Sprintf("auto-merged %d safe changes and resolved %d conflicts", autoMerged, resolved)
}

func mergePreviewMessage(autoMerged int, conflicts int, proposalAdvanced bool) string {
	if autoMerged == 0 && conflicts == 0 {
		if proposalAdvanced {
			return "latest proposal snapshot can be accepted without page content changes"
		}
		return "no contract changes require merge"
	}
	if conflicts > 0 {
		return fmt.Sprintf("preview: %d safe changes can be auto-merged, %d conflicts require manual resolution", autoMerged, conflicts)
	}
	return fmt.Sprintf("preview: %d safe changes can be auto-merged", autoMerged)
}

func samePageSpec(left spec.PageSpec, right spec.PageSpec) bool {
	leftJSON, err := marshalPageSpec(left)
	if err != nil {
		return false
	}
	rightJSON, err := marshalPageSpec(right)
	if err != nil {
		return false
	}
	return leftJSON == rightJSON
}
