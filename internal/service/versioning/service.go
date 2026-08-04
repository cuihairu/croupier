package versioning

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	dashboardmerge "github.com/cuihairu/croupier/internal/dashboard/merge"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service provides versioning and change management operations.
type Service struct {
	db                   *gorm.DB
	contractModel        *model.FunctionContractModel
	semanticsModel       *model.CapabilitySemanticsModel
	proposalModel        *model.PageProposalModel
	proposalVersionModel *model.PageProposalVersionModel
	pageModel            *model.PageSpecModel
	publishedModel       *model.PublishedPageSpecModel
	pageVersionModel     *model.PageVersionModel
}

// NewService creates the service.
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:                   db,
		contractModel:        model.NewFunctionContractModel(db),
		semanticsModel:       model.NewCapabilitySemanticsModel(db),
		proposalModel:        model.NewPageProposalModel(db),
		proposalVersionModel: model.NewPageProposalVersionModel(db),
		pageModel:            model.NewPageSpecModel(db),
		publishedModel:       model.NewPublishedPageSpecModel(db),
		pageVersionModel:     model.NewPageVersionModel(db),
	}
}

// ChangeType represents the type of change.
type ChangeType string

const (
	ChangeTypeFunctionUpdate ChangeType = "function_update"
	ChangeTypeSemanticUpdate ChangeType = "semantic_update"
	ChangeTypeProposalUpdate ChangeType = "proposal_update"
	ChangeTypeDraftUpdate    ChangeType = "draft_update"
	ChangeTypePublish        ChangeType = "publish"
)

// ChangeItem represents a single change in the change chain.
type ChangeItem struct {
	Type      ChangeType      `json:"type"`
	Timestamp string          `json:"timestamp"`
	Version   int             `json:"version,omitempty"`
	Summary   string          `json:"summary"`
	Details   json.RawMessage `json:"details,omitempty"`
	Actor     string          `json:"actor,omitempty"`
}

// ChangeChain represents the full chain of changes for a resource.
type ChangeChain struct {
	ResourceKey string       `json:"resourceKey"`
	Items       []ChangeItem `json:"items"`
	Current     CurrentState `json:"current"`
}

// CurrentState represents the current state of a resource.
type CurrentState struct {
	FunctionVersion  string `json:"functionVersion,omitempty"`
	SemanticVersion  int    `json:"semanticVersion,omitempty"`
	ProposalVersion  int    `json:"proposalVersion,omitempty"`
	DraftRevision    int    `json:"draftRevision,omitempty"`
	PublishedVersion int    `json:"publishedVersion,omitempty"`
}

// GetChangeChainRequest is the request for getting change chain.
type GetChangeChainRequest struct {
	GameID      string
	Env         string
	ResourceKey string
}

// GetChangeChain returns the change chain for a resource.
func (s *Service) GetChangeChain(ctx context.Context, req *GetChangeChainRequest) (*ChangeChain, error) {
	chain := &ChangeChain{
		ResourceKey: req.ResourceKey,
		Items:       []ChangeItem{},
	}

	// Get function contracts and their versions
	contracts, err := s.contractModel.ListByResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("list contracts: %w", err)
	}

	// Add function changes
	for _, c := range contracts {
		chain.Items = append(chain.Items, ChangeItem{
			Type:      ChangeTypeFunctionUpdate,
			Timestamp: c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			Version:   0, // FunctionContract doesn't have version yet
			Summary:   fmt.Sprintf("function %s updated", c.FunctionID),
			Actor:     c.UpdatedBy,
		})
		if chain.Current.FunctionVersion == "" {
			chain.Current.FunctionVersion = c.Version
		}
	}

	// Get semantics
	semantics, _ := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)
	if semantics != nil {
		chain.Items = append(chain.Items, ChangeItem{
			Type:      ChangeTypeSemanticUpdate,
			Timestamp: semantics.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			Version:   semantics.Version,
			Summary:   fmt.Sprintf("semantics updated to version %d", semantics.Version),
			Actor:     semantics.UpdatedBy,
		})
		chain.Current.SemanticVersion = semantics.Version
	}

	// Get proposal
	proposal, _ := s.proposalModel.FindByScopeAndKey(ctx, req.GameID, req.Env, resourceProposalKey(req.ResourceKey))
	if proposal != nil {
		versions, err := s.proposalVersionModel.ListByProposalID(ctx, proposal.ID)
		if err != nil {
			return nil, fmt.Errorf("list proposal versions: %w", err)
		}
		chain.Items = append(chain.Items, ChangeItem{
			Type:      ChangeTypeProposalUpdate,
			Timestamp: proposal.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			Summary:   fmt.Sprintf("proposal status: %s", proposal.Status),
			Actor:     proposal.UpdatedBy,
		})
		if len(versions) > 0 {
			chain.Current.ProposalVersion = versions[0].Version
		}
	}

	pageKey := resourcePageKey(req.ResourceKey)
	if page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey); err == nil {
		chain.Current.DraftRevision = page.DraftRevision
		chain.Current.PublishedVersion = page.PublishedVersion
	}
	pageVersions, err := s.pageVersionModel.ListByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil, fmt.Errorf("list page versions: %w", err)
	}
	for _, version := range pageVersions {
		changeType := ChangeTypeDraftUpdate
		if version.Status == "published" {
			changeType = ChangeTypePublish
		}
		chain.Items = append(chain.Items, ChangeItem{
			Type:      changeType,
			Timestamp: version.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Version:   version.Version,
			Summary:   firstNonEmpty(version.Message, fmt.Sprintf("page %s version %d", version.Status, version.Version)),
			Actor:     version.CreatedBy,
		})
	}

	sort.SliceStable(chain.Items, func(i, j int) bool {
		return chain.Items[i].Timestamp > chain.Items[j].Timestamp
	})

	return chain, nil
}

// DiffRequest is the request for comparing versions.
type DiffRequest struct {
	GameID      string
	Env         string
	ResourceKey string
	FromVersion int
	ToVersion   int
}

// DiffResponse represents the diff between two versions.
type DiffResponse struct {
	Changes []FieldChange `json:"changes"`
	Summary string        `json:"summary"`
}

// FieldChange represents a single field change.
type FieldChange struct {
	Path       string          `json:"path"`
	OldValue   json.RawMessage `json:"oldValue,omitempty"`
	NewValue   json.RawMessage `json:"newValue,omitempty"`
	ChangeType string          `json:"changeType"` // added|removed|modified
	IsSemantic bool            `json:"isSemantic"` // true if this is a semantic change
}

// Diff compares two semantic versions and returns changes.
func (s *Service) Diff(ctx context.Context, req *DiffRequest) (*DiffResponse, error) {
	// Get current semantics
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("semantics not found: %w", err)
	}

	// Get proposal to compare against
	proposal, _ := s.proposalModel.FindByScopeAndKey(ctx, req.GameID, req.Env, resourceProposalKey(req.ResourceKey))

	var changes []FieldChange

	// Compare identity field
	if semantics.IdentityField != "" {
		changes = append(changes, FieldChange{
			Path:       "identityField",
			NewValue:   jsonString(semantics.IdentityField),
			ChangeType: "modified",
			IsSemantic: true,
		})
	}

	// Compare collection query
	if semantics.CollectionQueryID > 0 {
		changes = append(changes, FieldChange{
			Path:       "collectionQueryId",
			NewValue:   jsonNumber(semantics.CollectionQueryID),
			ChangeType: "modified",
			IsSemantic: true,
		})
	}

	// Compare lifecycle capabilities
	if semantics.CreateID > 0 {
		changes = append(changes, FieldChange{
			Path:       "lifecycle.create",
			NewValue:   jsonNumber(semantics.CreateID),
			ChangeType: "modified",
			IsSemantic: true,
		})
	}
	if semantics.UpdateID > 0 {
		changes = append(changes, FieldChange{
			Path:       "lifecycle.update",
			NewValue:   jsonNumber(semantics.UpdateID),
			ChangeType: "modified",
			IsSemantic: true,
		})
	}
	if semantics.DeleteID > 0 {
		changes = append(changes, FieldChange{
			Path:       "lifecycle.delete",
			NewValue:   jsonNumber(semantics.DeleteID),
			ChangeType: "modified",
			IsSemantic: true,
		})
	}

	// If we have a proposal, compare proposal quality
	if proposal != nil {
		changes = append(changes, FieldChange{
			Path:       "proposal.quality",
			NewValue:   jsonString(proposal.Quality),
			ChangeType: "modified",
			IsSemantic: false,
		})
	}

	changes = append(changes, s.bindingContractChanges(ctx, req)...)

	return &DiffResponse{
		Changes: changes,
		Summary: fmt.Sprintf("found %d changes", len(changes)),
	}, nil
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
	GameID      string               `json:"-"`
	Env         string               `json:"-"`
	ResourceKey string               `json:"-"`
	Strategy    MergeStrategy        `json:"strategy"`
	Conflicts   []ConflictResolution `json:"conflicts,omitempty"`
	Reason      string               `json:"reason,omitempty"`
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

// Merge applies changes with the given strategy.
func (s *Service) Merge(ctx context.Context, req *MergeRequest) (*MergeResponse, error) {
	if req.Strategy != MergeStrategyAuto &&
		req.Strategy != MergeStrategyAccept &&
		req.Strategy != MergeStrategyReject &&
		req.Strategy != MergeStrategyManual {
		return nil, errorx.NewBadRequest("unknown merge strategy: " + string(req.Strategy))
	}
	if strings.TrimSpace(req.ResourceKey) == "" {
		return nil, errorx.NewBadRequest("resourceKey is required")
	}
	if req.Strategy == MergeStrategyReject {
		return &MergeResponse{Message: "all changes rejected"}, nil
	}
	if req.Strategy == MergeStrategyAccept {
		return nil, errorx.NewValidationError("accept-all merge is forbidden; resolve execution-affecting conflicts explicitly")
	}
	if req.Strategy == MergeStrategyManual {
		return nil, errorx.NewNotImplemented("manual conflict resolution is not wired yet; edit the PageSpec draft and publish")
	}

	pageKey := resourcePageKey(req.ResourceKey)
	proposalKey := resourceProposalKey(req.ResourceKey)
	page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	if strings.TrimSpace(page.BaseProposalKey) == "" || page.BaseProposalVersion <= 0 {
		return nil, errorx.NewConflict("page draft has no base proposal snapshot; regenerate or accept a new proposal before merge")
	}
	if page.BaseProposalKey != proposalKey {
		return nil, errorx.NewConflict("page draft base proposal does not match the requested resource")
	}
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, req.GameID, req.Env, proposalKey)
	if err != nil {
		return nil, errorx.NewNotFound("latest proposal not found")
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

	mergeResult := dashboardmerge.ThreeWayMerge(basePage, draftPage, latestPage)
	if len(mergeResult.AutoMerge) == 0 {
		return &MergeResponse{
			Merged:         0,
			Conflicts:      len(mergeResult.Conflicts),
			Message:        mergeMessage(0, len(mergeResult.Conflicts), false),
			DraftRevision:  page.DraftRevision,
			AutoMergeItems: mergeResult.AutoMerge,
			ConflictItems:  mergeResult.Conflicts,
		}, nil
	}

	mergedPage, err := applyAutoMergeItems(draftPage, mergeResult.AutoMerge)
	if err != nil {
		return nil, err
	}
	if samePageSpec(mergedPage, draftPage) {
		return &MergeResponse{
			Merged:         len(mergeResult.AutoMerge),
			Conflicts:      len(mergeResult.Conflicts),
			Message:        mergeMessage(len(mergeResult.AutoMerge), len(mergeResult.Conflicts), false),
			DraftRevision:  page.DraftRevision,
			AutoMergeItems: mergeResult.AutoMerge,
			ConflictItems:  mergeResult.Conflicts,
		}, nil
	}
	actor := actorFromContext(ctx)
	now := time.Now()
	var nextRevision int
	err = s.withTransaction(ctx, func(txCtx context.Context) error {
		pageModel := model.NewPageSpecModel(dbctx.Resolve(txCtx, s.db))
		versionModel := model.NewPageVersionModel(dbctx.Resolve(txCtx, s.db))
		current, err := pageModel.FindByScopeAndPageKey(txCtx, req.GameID, req.Env, pageKey)
		if err != nil {
			return err
		}
		nextRevision, err = versionModel.GetNextVersion(txCtx, req.GameID, req.Env, pageKey)
		if err != nil {
			return err
		}
		if err := applyPageSpecToModel(current, mergedPage); err != nil {
			return err
		}
		current.GameID = req.GameID
		current.Env = req.Env
		current.Status = "draft"
		current.PublishedActive = page.PublishedActive
		current.PublishedVersion = page.PublishedVersion
		current.DraftRevision = nextRevision
		current.BaseProposalKey = proposalKey
		current.BaseProposalVersion = page.BaseProposalVersion
		if len(mergeResult.Conflicts) == 0 {
			current.BaseProposalVersion = latestVersion.Version
		}
		current.UpdatedAt = now
		current.UpdatedBy = actor
		specJSON, err := marshalPageSpec(mergedPage)
		if err != nil {
			return err
		}
		if err := pageModel.Upsert(txCtx, current); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    req.GameID,
			Env:       req.Env,
			PageKey:   pageKey,
			Version:   nextRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   firstNonEmpty(req.Reason, "auto-merge safe display changes from latest proposal"),
			CreatedBy: actor,
			CreatedAt: now,
		})
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

func pageSpecFromProposalSnapshot(raw json.RawMessage) (spec.PageSpec, error) {
	var proposal model.PageProposal
	if err := json.Unmarshal(raw, &proposal); err != nil {
		return spec.PageSpec{}, fmt.Errorf("decode proposal snapshot: %w", err)
	}
	return pageSpecFromProposalModel(&proposal)
}

func pageSpecFromProposalModel(proposal *model.PageProposal) (spec.PageSpec, error) {
	if proposal == nil || len(proposal.PageSpec) == 0 {
		return spec.PageSpec{}, errorx.NewNotFound("proposal PageSpec not found")
	}
	var page spec.PageSpec
	if err := json.Unmarshal(proposal.PageSpec, &page); err != nil {
		return spec.PageSpec{}, fmt.Errorf("decode proposal PageSpec: %w", err)
	}
	return normalizePageSpec(page), nil
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
	case "category.labels":
		return decodeMergeValue(item, &page.Category.Labels)
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
	}
	if handled, err := applyIndexedAutoMergeItem(page, item); handled || err != nil {
		return err
	}
	return errorx.NewValidationError("unsupported auto-merge field: " + item.Field)
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

// RollbackRequest is the request for rollback.
type RollbackRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-"`
	Version     int    `json:"version"`
	Reason      string `json:"reason,omitempty"`
}

// RollbackResponse is the response for rollback.
type RollbackResponse struct {
	PageKey       string `json:"pageKey"`
	DraftRevision int    `json:"draftRevision,omitempty"`
	Version       int    `json:"version,omitempty"`
	Message       string `json:"message"`
}

// RollbackDraft rolls back to a previous draft version.
func (s *Service) RollbackDraft(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error) {
	if req.Version <= 0 {
		return nil, errorx.NewBadRequest("version is required")
	}
	pageKey := resourcePageKey(req.ResourceKey)
	target, err := s.findPageVersion(ctx, req.GameID, req.Env, pageKey, req.Version)
	if err != nil {
		return nil, err
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(target.SpecJSON), &pageSpec); err != nil {
		return nil, fmt.Errorf("decode page version: %w", err)
	}
	pageSpec = normalizePageSpec(pageSpec)
	actor := actorFromContext(ctx)
	now := time.Now()
	var nextRevision int

	err = s.withTransaction(ctx, func(txCtx context.Context) error {
		pageModel := model.NewPageSpecModel(dbctx.Resolve(txCtx, s.db))
		versionModel := model.NewPageVersionModel(dbctx.Resolve(txCtx, s.db))

		page, err := pageModel.FindByScopeAndPageKey(txCtx, req.GameID, req.Env, pageKey)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if page == nil {
			page = &model.PageSpec{
				GameID:    req.GameID,
				Env:       req.Env,
				PageKey:   pageKey,
				CreatedAt: now,
			}
		}
		nextRevision, err = versionModel.GetNextVersion(txCtx, req.GameID, req.Env, pageKey)
		if err != nil {
			return err
		}
		if err := applyPageSpecToModel(page, pageSpec); err != nil {
			return err
		}
		page.GameID = req.GameID
		page.Env = req.Env
		page.Status = "draft"
		page.DraftRevision = nextRevision
		page.UpdatedBy = actor
		page.UpdatedAt = now
		specJSON, err := marshalPageSpec(pageSpec)
		if err != nil {
			return err
		}
		if err := pageModel.Upsert(txCtx, page); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    req.GameID,
			Env:       req.Env,
			PageKey:   pageKey,
			Version:   nextRevision,
			SpecJSON:  specJSON,
			Status:    "draft",
			Message:   firstNonEmpty(req.Reason, "rollback draft to version "+strconv.Itoa(req.Version)),
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}
	return &RollbackResponse{
		PageKey:       pageKey,
		DraftRevision: nextRevision,
		Message:       fmt.Sprintf("rolled back draft %s to version %d", pageKey, req.Version),
	}, nil
}

// RollbackPublish rolls back to a previous published version.
func (s *Service) RollbackPublish(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error) {
	if req.Version <= 0 {
		return nil, errorx.NewBadRequest("version is required")
	}
	pageKey := resourcePageKey(req.ResourceKey)
	target, err := s.publishedModel.FindByScopePageKeyAndVersion(ctx, req.GameID, req.Env, pageKey, req.Version)
	if err != nil {
		return nil, errorx.NewNotFound("published page version not found")
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(target.SpecJSON), &pageSpec); err != nil {
		return nil, fmt.Errorf("decode published page spec: %w", err)
	}
	pageSpec = normalizePageSpec(pageSpec)
	actor := actorFromContext(ctx)
	now := time.Now()
	var nextVersion int

	err = s.withTransaction(ctx, func(txCtx context.Context) error {
		pageModel := model.NewPageSpecModel(dbctx.Resolve(txCtx, s.db))
		publishedModel := model.NewPublishedPageSpecModel(dbctx.Resolve(txCtx, s.db))
		versionModel := model.NewPageVersionModel(dbctx.Resolve(txCtx, s.db))

		var err error
		nextVersion, err = versionModel.GetNextVersion(txCtx, req.GameID, req.Env, pageKey)
		if err != nil {
			return err
		}
		page, err := pageModel.FindByScopeAndPageKey(txCtx, req.GameID, req.Env, pageKey)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if page == nil {
			page = &model.PageSpec{GameID: req.GameID, Env: req.Env, PageKey: pageKey, CreatedAt: now}
		}
		if err := applyPageSpecToModel(page, pageSpec); err != nil {
			return err
		}
		page.GameID = req.GameID
		page.Env = req.Env
		page.Status = "published"
		page.PublishedActive = true
		page.PublishedVersion = nextVersion
		page.DraftRevision = nextVersion
		page.UpdatedBy = actor
		page.UpdatedAt = now

		specJSON, err := marshalPageSpec(pageSpec)
		if err != nil {
			return err
		}
		if err := publishedModel.DeactivatePage(txCtx, req.GameID, req.Env, pageKey, now); err != nil {
			return err
		}
		if err := publishedModel.Create(txCtx, &model.PublishedPageSpec{
			GameID:                req.GameID,
			Env:                   req.Env,
			PageKey:               pageKey,
			Version:               nextVersion,
			SpecJSON:              specJSON,
			BindingContractsJSON:  target.BindingContractsJSON,
			RendererSchemaVersion: target.RendererSchemaVersion,
			Active:                true,
			PublishedAt:           now,
			PublishedBy:           actor,
		}); err != nil {
			return err
		}
		if err := pageModel.Upsert(txCtx, page); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    req.GameID,
			Env:       req.Env,
			PageKey:   pageKey,
			Version:   nextVersion,
			SpecJSON:  specJSON,
			Status:    "published",
			Message:   firstNonEmpty(req.Reason, "rollback publish to version "+strconv.Itoa(req.Version)),
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}
	return &RollbackResponse{
		PageKey:       pageKey,
		DraftRevision: nextVersion,
		Version:       nextVersion,
		Message:       fmt.Sprintf("rolled back published page %s to version %d", pageKey, req.Version),
	}, nil
}

// RegenerateProposalRequest is the request for regenerating proposal.
type RegenerateProposalRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-"`
	Force       bool   `json:"force,omitempty"`
}

// RegenerateProposalResponse is the response for regenerating proposal.
type RegenerateProposalResponse struct {
	Message string `json:"message"`
}

// RegenerateProposal regenerates a proposal from current contracts and semantics.
func (s *Service) RegenerateProposal(ctx context.Context, req *RegenerateProposalRequest) (*RegenerateProposalResponse, error) {
	contractService := service.NewContractService(s.db)
	if err := contractService.RebuildProposalsForResource(ctx, req.GameID, req.Env, req.ResourceKey); err != nil {
		return nil, fmt.Errorf("regenerate proposal: %w", err)
	}

	return &RegenerateProposalResponse{
		Message: fmt.Sprintf("proposal regenerated for resource %s", req.ResourceKey),
	}, nil
}

// RepublishRequest is the request for republishing.
type RepublishRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-"`
	Reason      string `json:"reason,omitempty"`
}

// RepublishResponse is the response for republishing.
type RepublishResponse struct {
	Version int    `json:"version"`
	Message string `json:"message"`
}

// Republish creates a new published version from current draft.
func (s *Service) Republish(ctx context.Context, req *RepublishRequest) (*RepublishResponse, error) {
	pageKey := resourcePageKey(req.ResourceKey)
	page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	pageSpec, err := pageSpecFromModel(page)
	if err != nil {
		return nil, err
	}
	contracts, err := s.buildBindingContracts(ctx, req.GameID, req.Env, pageSpec.Bindings)
	if err != nil {
		return nil, err
	}
	specJSON, err := marshalPageSpec(pageSpec)
	if err != nil {
		return nil, err
	}
	contractsJSON, err := json.Marshal(contracts)
	if err != nil {
		return nil, err
	}

	actor := actorFromContext(ctx)
	now := time.Now()
	var nextVersion int
	err = s.withTransaction(ctx, func(txCtx context.Context) error {
		pageModel := model.NewPageSpecModel(dbctx.Resolve(txCtx, s.db))
		publishedModel := model.NewPublishedPageSpecModel(dbctx.Resolve(txCtx, s.db))
		versionModel := model.NewPageVersionModel(dbctx.Resolve(txCtx, s.db))
		var err error
		nextVersion, err = versionModel.GetNextVersion(txCtx, req.GameID, req.Env, pageKey)
		if err != nil {
			return err
		}
		if err := publishedModel.DeactivatePage(txCtx, req.GameID, req.Env, pageKey, now); err != nil {
			return err
		}
		if err := publishedModel.Create(txCtx, &model.PublishedPageSpec{
			GameID:                req.GameID,
			Env:                   req.Env,
			PageKey:               pageKey,
			Version:               nextVersion,
			SpecJSON:              specJSON,
			BindingContractsJSON:  string(contractsJSON),
			RendererSchemaVersion: rendererSchemaVersion,
			Active:                true,
			PublishedAt:           now,
			PublishedBy:           actor,
		}); err != nil {
			return err
		}
		page.Status = "published"
		page.PublishedActive = true
		page.PublishedVersion = nextVersion
		page.DraftRevision = nextVersion
		page.UpdatedAt = now
		page.UpdatedBy = actor
		if err := pageModel.Upsert(txCtx, page); err != nil {
			return err
		}
		return versionModel.UpsertByScopePageKeyVersion(txCtx, &model.PageVersion{
			GameID:    req.GameID,
			Env:       req.Env,
			PageKey:   pageKey,
			Version:   nextVersion,
			SpecJSON:  specJSON,
			Status:    "published",
			Message:   firstNonEmpty(req.Reason, "republish current draft"),
			CreatedBy: actor,
			CreatedAt: now,
		})
	})
	if err != nil {
		return nil, err
	}

	return &RepublishResponse{
		Version: nextVersion,
		Message: fmt.Sprintf("republished %s at version %d", pageKey, nextVersion),
	}, nil
}

func resourceProposalKey(resourceKey string) string {
	return "resource:" + resourceKey
}

const rendererSchemaVersion = "page-spec:1"

func resourcePageKey(resourceKey string) string {
	return "resource--" + strings.TrimSpace(resourceKey)
}

func (s *Service) findPageVersion(ctx context.Context, gameID, env, pageKey string, version int) (*model.PageVersion, error) {
	versions, err := s.pageVersionModel.ListByScopeAndPageKey(ctx, gameID, env, pageKey)
	if err != nil {
		return nil, err
	}
	for _, item := range versions {
		if item.Version == version {
			return &item, nil
		}
	}
	return nil, errorx.NewNotFound("page version not found")
}

func (s *Service) bindingContractChanges(ctx context.Context, req *DiffRequest) []FieldChange {
	pageKey := resourcePageKey(req.ResourceKey)
	published, err := s.publishedModel.FindLatestByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil
	}
	var contracts []spec.BindingContractSnapshot
	if strings.TrimSpace(published.BindingContractsJSON) != "" {
		_ = json.Unmarshal([]byte(published.BindingContractsJSON), &contracts)
	}
	var changes []FieldChange
	for _, frozen := range contracts {
		latest, err := s.contractModel.FindByScopeAndFunctionID(ctx, req.GameID, req.Env, frozen.FunctionID)
		if err != nil {
			changes = append(changes, FieldChange{
				Path:       "bindings." + frozen.BindingID + ".function",
				OldValue:   jsonString(frozen.FunctionID),
				ChangeType: "removed",
				IsSemantic: true,
			})
			continue
		}
		inputDigest := digestRaw(latest.InputSchema)
		outputDigest := digestRaw(latest.OutputSchema)
		if inputDigest != frozen.InputSchemaDigest {
			changes = append(changes, FieldChange{
				Path:       "bindings." + frozen.BindingID + ".inputSchemaDigest",
				OldValue:   jsonString(frozen.InputSchemaDigest),
				NewValue:   jsonString(inputDigest),
				ChangeType: "modified",
				IsSemantic: true,
			})
		}
		if outputDigest != frozen.OutputSchemaDigest {
			changes = append(changes, FieldChange{
				Path:       "bindings." + frozen.BindingID + ".outputSchemaDigest",
				OldValue:   jsonString(frozen.OutputSchemaDigest),
				NewValue:   jsonString(outputDigest),
				ChangeType: "modified",
				IsSemantic: true,
			})
		}
		if strings.TrimSpace(latest.Risk) != string(frozen.Risk) {
			changes = append(changes, FieldChange{
				Path:       "bindings." + frozen.BindingID + ".risk",
				OldValue:   jsonString(string(frozen.Risk)),
				NewValue:   jsonString(latest.Risk),
				ChangeType: "modified",
				IsSemantic: true,
			})
		}
		if strings.TrimSpace(latest.Permission) != frozen.Permission {
			changes = append(changes, FieldChange{
				Path:       "bindings." + frozen.BindingID + ".permission",
				OldValue:   jsonString(frozen.Permission),
				NewValue:   jsonString(latest.Permission),
				ChangeType: "modified",
				IsSemantic: true,
			})
		}
	}
	return changes
}

func (s *Service) buildBindingContracts(ctx context.Context, gameID, env string, bindings []spec.PageFunctionBinding) ([]spec.BindingContractSnapshot, error) {
	out := make([]spec.BindingContractSnapshot, 0, len(bindings))
	for _, binding := range bindings {
		functionID := strings.TrimSpace(binding.FunctionID)
		if functionID == "" {
			return nil, errorx.NewValidationError("binding.functionId is required")
		}
		contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
		if err != nil {
			return nil, errorx.NewValidationError("bound function contract does not exist: " + functionID)
		}
		if !contract.Enabled {
			return nil, errorx.NewValidationError("bound function is disabled: " + functionID)
		}
		out = append(out, spec.BindingContractSnapshot{
			BindingID:             strings.TrimSpace(binding.ID),
			FunctionID:            functionID,
			FunctionVersion:       strings.TrimSpace(contract.Version),
			InputSchemaDigest:     digestRaw(contract.InputSchema),
			OutputSchemaDigest:    digestRaw(contract.OutputSchema),
			Risk:                  spec.RiskLevel(contract.Risk),
			Permission:            strings.TrimSpace(contract.Permission),
			Approval:              approvalPolicyFromJSONMap(contract.Approval),
			ExecutionMode:         binding.Execution.Mode,
			RendererSchemaVersion: rendererSchemaVersion,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BindingID < out[j].BindingID })
	return out, nil
}

func pageSpecFromModel(page *model.PageSpec) (spec.PageSpec, error) {
	if page == nil || strings.TrimSpace(page.SpecJSON) == "" {
		return spec.PageSpec{}, errorx.NewNotFound("page draft not found")
	}
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(page.SpecJSON), &pageSpec); err != nil {
		return spec.PageSpec{}, fmt.Errorf("decode page spec: %w", err)
	}
	return normalizePageSpec(pageSpec), nil
}

func applyPageSpecToModel(page *model.PageSpec, pageSpec spec.PageSpec) error {
	pageSpec = normalizePageSpec(pageSpec)
	page.PageKey = pageSpec.PageKey
	page.Type = string(pageSpec.Type)
	page.ResourceKey = pageSpec.ResourceKey
	page.CategoryKey = pageSpec.Category.Key
	page.CategoryOrder = pageSpec.Category.Order
	page.Order = pageSpec.Order
	page.Icon = pageSpec.Icon
	if err := page.SetTitle(pageSpec.Title); err != nil {
		return err
	}
	if err := page.SetCategoryLabels(pageSpec.Category.Labels); err != nil {
		return err
	}
	specJSON, err := marshalPageSpec(pageSpec)
	if err != nil {
		return err
	}
	page.SpecJSON = specJSON
	return nil
}

func normalizePageSpec(page spec.PageSpec) spec.PageSpec {
	page.PageKey = strings.TrimSpace(page.PageKey)
	page.ResourceKey = strings.TrimSpace(page.ResourceKey)
	page.Icon = strings.TrimSpace(page.Icon)
	page.Title = normalizeLocalizedText(page.Title)
	page.Description = normalizeLocalizedText(page.Description)
	page.Category.Key = strings.TrimSpace(page.Category.Key)
	page.Category.Labels = normalizeLocalizedText(page.Category.Labels)
	for i := range page.Bindings {
		page.Bindings[i].ID = strings.TrimSpace(page.Bindings[i].ID)
		page.Bindings[i].FunctionID = strings.TrimSpace(page.Bindings[i].FunctionID)
	}
	return page
}

func marshalPageSpec(page spec.PageSpec) (string, error) {
	page = normalizePageSpec(page)
	raw, err := json.Marshal(page)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func normalizeLocalizedText(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch strings.ToLower(strings.ReplaceAll(key, "_", "-")) {
		case "zh", "zh-cn":
			out["zh-CN"] = value
		case "en", "en-us":
			out["en-US"] = value
		default:
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func actorFromContext(ctx context.Context) string {
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil {
		return "system"
	}
	return actor
}

func (s *Service) withTransaction(ctx context.Context, fn func(context.Context) error) error {
	db := dbctx.Resolve(ctx, s.db)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(dbctx.WithDB(ctx, tx))
	})
}

func digestRaw(raw datatypes.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func approvalPolicyFromJSONMap(values map[string]interface{}) spec.ApprovalPolicy {
	if len(values) == 0 {
		return spec.ApprovalPolicy{}
	}
	required, _ := values["required"].(bool)
	policyKey, _ := values["policyKey"].(string)
	if policyKey == "" {
		policyKey, _ = values["policy_key"].(string)
	}
	return spec.ApprovalPolicy{
		Required:  required,
		PolicyKey: strings.TrimSpace(policyKey),
	}
}

func jsonString(value string) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func jsonNumber(value uint) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
