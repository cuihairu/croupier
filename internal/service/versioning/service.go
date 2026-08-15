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
	"github.com/cuihairu/croupier/internal/dashboard/generator"
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

// ChangeChain represents the full chain of changes for a page.
type ChangeChain struct {
	PageKey     string       `json:"pageKey"`
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
	GameID  string
	Env     string
	PageKey string `uri:"pageKey"`
}

// GetChangeChain returns the change chain for a page.
func (s *Service) GetChangeChain(ctx context.Context, req *GetChangeChainRequest) (*ChangeChain, error) {
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	chain := &ChangeChain{
		PageKey: pageKey,
		Items:   []ChangeItem{},
	}

	page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	pageSpec, _ := pageSpecFromModel(page)
	chain.ResourceKey = strings.TrimSpace(page.ResourceKey)
	chain.Current.DraftRevision = page.DraftRevision
	chain.Current.PublishedVersion = page.PublishedVersion

	proposalKey := strings.TrimSpace(page.BaseProposalKey)
	if proposalKey == "" {
		if proposal, err := s.proposalModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey); err == nil {
			proposalKey = strings.TrimSpace(proposal.ProposalKey)
		}
	}

	// Get function contracts and their versions
	contracts, err := s.contractsForPage(ctx, req.GameID, req.Env, pageSpec)
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

	// Get semantics when the page belongs to a resource.
	if chain.ResourceKey != "" {
		semantics, _ := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, chain.ResourceKey)
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
	}

	// Get proposal
	var proposal *model.PageProposal
	if proposalKey != "" {
		proposal, _ = s.proposalModel.FindByScopeAndKey(ctx, req.GameID, req.Env, proposalKey)
	}
	if proposal == nil {
		proposal, _ = s.proposalModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	}
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
	PageKey     string `uri:"pageKey"`
	FromVersion int
	ToVersion   int
}

// DiffResponse represents the diff between two versions.
type DiffResponse struct {
	Changes        []FieldChange                  `json:"changes"`
	Summary        string                         `json:"summary"`
	AutoMergeItems []dashboardmerge.MergeItem     `json:"autoMergeItems,omitempty"`
	ConflictItems  []dashboardmerge.MergeConflict `json:"conflictItems,omitempty"`
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
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	pageSpec, _ := pageSpecFromModel(page)
	resourceKey := strings.TrimSpace(page.ResourceKey)

	// Get proposal to compare against
	proposal, _ := s.proposalForPage(ctx, req.GameID, req.Env, page)
	mergeResult, _ := s.mergePreviewForPage(ctx, req.GameID, req.Env, pageKey, page, proposal)

	var changes []FieldChange

	if resourceKey != "" {
		// Get current semantics
		semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, resourceKey)
		if err == nil && semantics != nil {
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
		}
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

	changes = append(changes, s.bindingContractChanges(ctx, req.GameID, req.Env, pageKey, pageSpec)...)

	return &DiffResponse{
		Changes:        changes,
		Summary:        diffSummary(len(changes), len(mergeResult.AutoMerge), len(mergeResult.Conflicts)),
		AutoMergeItems: mergeResult.AutoMerge,
		ConflictItems:  mergeResult.Conflicts,
	}, nil
}

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

	mergeResult := dashboardmerge.ThreeWayMerge(basePage, draftPage, latestPage)
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

func (s *Service) proposalForPage(ctx context.Context, gameID, env string, page *model.PageSpec) (*model.PageProposal, error) {
	if page == nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	if key := strings.TrimSpace(page.BaseProposalKey); key != "" {
		return s.proposalModel.FindByScopeAndKey(ctx, gameID, env, key)
	}
	return s.proposalModel.FindByScopeAndPageKey(ctx, gameID, env, strings.TrimSpace(page.PageKey))
}

func (s *Service) contractsForPage(ctx context.Context, gameID, env string, pageSpec spec.PageSpec) ([]*model.FunctionContract, error) {
	seen := map[string]struct{}{}
	contracts := make([]*model.FunctionContract, 0, len(pageSpec.Bindings))
	for _, binding := range pageSpec.Bindings {
		functionID := strings.TrimSpace(binding.FunctionID)
		if functionID == "" {
			continue
		}
		if _, ok := seen[functionID]; ok {
			continue
		}
		seen[functionID] = struct{}{}
		contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
		if err != nil {
			continue
		}
		contracts = append(contracts, contract)
	}
	return contracts, nil
}

func (s *Service) regenerateStandaloneProposal(ctx context.Context, gameID, env string, pageSpec spec.PageSpec) error {
	mainContract, err := s.mainContractForStandalonePage(ctx, gameID, env, pageSpec)
	if err != nil {
		return err
	}
	functions, err := s.functionSpecsByID(ctx, gameID, env, pageSpec)
	if err != nil {
		return err
	}
	generated := generator.GenerateForOperation(service.OperationSpecFromContract(mainContract), generator.GenerateOptions{
		DefaultLocale: "zh-CN",
		Functions:     functions,
	})
	if strings.TrimSpace(generated.PageKey) != strings.TrimSpace(pageSpec.PageKey) {
		return errorx.NewConflict("regenerated proposal pageKey does not match current page")
	}
	proposalKey := proposalKeyForPage(pageSpec.Type, strings.TrimSpace(mainContract.FunctionID))
	if proposalKey == "" {
		return errorx.NewValidationError("cannot derive proposalKey for page")
	}
	return s.upsertGeneratedProposal(ctx, gameID, env, proposalKey, []*model.FunctionContract{mainContract}, generated)
}

func (s *Service) mainContractForStandalonePage(ctx context.Context, gameID, env string, pageSpec spec.PageSpec) (*model.FunctionContract, error) {
	if pageSpec.Type == spec.PageTypeResource {
		return nil, errorx.NewValidationError("resource pages must be regenerated from resource semantics")
	}
	for _, binding := range pageSpec.Bindings {
		if binding.Usage != spec.BindingUsageAction &&
			binding.Usage != spec.BindingUsageTask &&
			binding.Usage != spec.BindingUsageReport {
			continue
		}
		functionID := strings.TrimSpace(binding.FunctionID)
		if functionID == "" {
			continue
		}
		contract, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID)
		if err != nil {
			return nil, errorx.NewNotFound("main page function contract not found")
		}
		return contract, nil
	}
	return nil, errorx.NewValidationError("page has no main executable binding")
}

func (s *Service) functionSpecsByID(ctx context.Context, gameID, env string, pageSpec spec.PageSpec) (map[string]spec.FunctionSpec, error) {
	contracts, err := s.contractsForPage(ctx, gameID, env, pageSpec)
	if err != nil {
		return nil, err
	}
	return service.FunctionSpecsFromContracts(contracts), nil
}

func (s *Service) upsertGeneratedProposal(ctx context.Context, gameID, env, proposalKey string, contracts []*model.FunctionContract, generated spec.GeneratedPageSpec) error {
	pageJSON, err := json.Marshal(generated.PageSpec)
	if err != nil {
		return fmt.Errorf("marshal generated page spec: %w", err)
	}
	proposal := &model.PageProposal{
		GameID:           gameID,
		Env:              env,
		ProposalKey:      proposalKey,
		PageKey:          generated.PageKey,
		PageType:         string(generated.Type),
		ResourceKey:      generated.ResourceKey,
		Quality:          string(generated.Quality),
		GeneratorVersion: pageProposalGeneratorVersion,
		FunctionDigest:   computeDigest(contracts),
		Title:            localizedTextToJSONMap(generated.Title),
		Description:      localizedTextToJSONMap(generated.Description),
		CategoryKey:      generated.Category.Key,
		PageSpec:         datatypes.JSON(pageJSON),
		Diagnostics:      jsonValue(generated.Diagnostics),
		Status:           "pending",
		UpdatedBy:        actorFromContext(ctx),
	}
	if err := s.proposalModel.UpsertProposal(ctx, proposal); err != nil {
		return fmt.Errorf("upsert page proposal %s: %w", proposalKey, err)
	}
	if _, err := createProposalVersionSnapshot(ctx, s.proposalVersionModel, proposal, "regenerate proposal from latest page contracts", actorFromContext(ctx)); err != nil {
		return fmt.Errorf("snapshot page proposal %s: %w", proposalKey, err)
	}
	return nil
}

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
		specJSON, err := marshalPageSpec(mergedPage)
		if err != nil {
			return err
		}
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

func mustPageSpecFromModel(page *model.PageSpec) spec.PageSpec {
	pageSpec, err := pageSpecFromModel(page)
	if err != nil {
		return spec.PageSpec{}
	}
	return pageSpec
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

func diffSummary(changes int, autoMergeItems int, conflicts int) string {
	if changes == 0 && autoMergeItems == 0 && conflicts == 0 {
		return "found no changes"
	}
	return fmt.Sprintf(
		"found %d changes, %d safe merge items, %d conflicts",
		changes,
		autoMergeItems,
		conflicts,
	)
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

// RollbackRequest is the request for rollback.
type RollbackRequest struct {
	GameID                string `json:"-"`
	Env                   string `json:"-"`
	PageKey               string `uri:"pageKey" json:"-"`
	ExpectedDraftRevision int    `json:"expectedDraftRevision"`
	Version               int    `json:"version"`
	Reason                string `json:"reason,omitempty"`
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
	if req.ExpectedDraftRevision <= 0 {
		return nil, errorx.NewBadRequest("expectedDraftRevision is required")
	}
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
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
			return errorx.NewNotFound("page draft not found")
		}
		if page.DraftRevision != req.ExpectedDraftRevision {
			return draftRevisionConflict(req.ExpectedDraftRevision, page.DraftRevision)
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
	if req.ExpectedDraftRevision <= 0 {
		return nil, errorx.NewBadRequest("expectedDraftRevision is required")
	}
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
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
			return errorx.NewNotFound("page draft not found")
		}
		if page.DraftRevision != req.ExpectedDraftRevision {
			return draftRevisionConflict(req.ExpectedDraftRevision, page.DraftRevision)
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
			BaseProposalKey:       target.BaseProposalKey,
			BaseProposalVersion:   target.BaseProposalVersion,
			FunctionDigest:        target.FunctionDigest,
			SemanticsDigest:       target.SemanticsDigest,
			GeneratorVersion:      target.GeneratorVersion,
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
	GameID  string `json:"-"`
	Env     string `json:"-"`
	PageKey string `uri:"pageKey" json:"-"`
	Force   bool   `json:"force,omitempty"`
}

// RegenerateProposalResponse is the response for regenerating proposal.
type RegenerateProposalResponse struct {
	Message string `json:"message"`
}

// RegenerateProposal regenerates a proposal from current contracts and semantics.
func (s *Service) RegenerateProposal(ctx context.Context, req *RegenerateProposalRequest) (*RegenerateProposalResponse, error) {
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
	page, err := s.pageModel.FindByScopeAndPageKey(ctx, req.GameID, req.Env, pageKey)
	if err != nil {
		return nil, errorx.NewNotFound("page draft not found")
	}
	pageSpec, err := pageSpecFromModel(page)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(page.ResourceKey) != "" && pageSpec.Type == spec.PageTypeResource {
		contractService := service.NewContractService(s.db)
		if err := contractService.RebuildProposalsForResource(ctx, req.GameID, req.Env, page.ResourceKey); err != nil {
			return nil, fmt.Errorf("regenerate proposal: %w", err)
		}
		return &RegenerateProposalResponse{
			Message: fmt.Sprintf("proposal regenerated for page %s", pageKey),
		}, nil
	}

	if err := s.regenerateStandaloneProposal(ctx, req.GameID, req.Env, pageSpec); err != nil {
		return nil, err
	}
	return &RegenerateProposalResponse{
		Message: fmt.Sprintf("proposal regenerated for page %s", pageKey),
	}, nil
}

// RepublishRequest is the request for republishing.
type RepublishRequest struct {
	GameID  string `json:"-"`
	Env     string `json:"-"`
	PageKey string `uri:"pageKey" json:"-"`
	Reason  string `json:"reason,omitempty"`
}

// RepublishResponse is the response for republishing.
type RepublishResponse struct {
	Version int    `json:"version"`
	Message string `json:"message"`
}

// Republish creates a new published version from current draft.
func (s *Service) Republish(ctx context.Context, req *RepublishRequest) (*RepublishResponse, error) {
	pageKey := strings.TrimSpace(req.PageKey)
	if pageKey == "" {
		return nil, errorx.NewBadRequest("pageKey is required")
	}
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
			BaseProposalKey:       page.BaseProposalKey,
			BaseProposalVersion:   page.BaseProposalVersion,
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

const rendererSchemaVersion = "page-spec:1"
const pageProposalGeneratorVersion = "page-generator:1"

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

func (s *Service) bindingContractChanges(ctx context.Context, gameID, env, pageKey string, pageSpec spec.PageSpec) []FieldChange {
	published, err := s.publishedModel.FindLatestByScopeAndPageKey(ctx, gameID, env, pageKey)
	if err != nil {
		return s.draftBindingContractChanges(ctx, gameID, env, pageSpec)
	}
	var contracts []spec.BindingContractSnapshot
	if strings.TrimSpace(published.BindingContractsJSON) != "" {
		_ = json.Unmarshal([]byte(published.BindingContractsJSON), &contracts)
	}
	var changes []FieldChange
	for _, frozen := range contracts {
		latest, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, frozen.FunctionID)
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

func (s *Service) draftBindingContractChanges(ctx context.Context, gameID, env string, pageSpec spec.PageSpec) []FieldChange {
	var changes []FieldChange
	for _, binding := range pageSpec.Bindings {
		functionID := strings.TrimSpace(binding.FunctionID)
		if functionID == "" {
			continue
		}
		if _, err := s.contractModel.FindByScopeAndFunctionID(ctx, gameID, env, functionID); err != nil {
			changes = append(changes, FieldChange{
				Path:       "bindings." + strings.TrimSpace(binding.ID) + ".function",
				OldValue:   jsonString(functionID),
				ChangeType: "removed",
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
			Approval:              service.ApprovalPolicyFromJSONMap(contract.Approval),
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

func localizedTextToJSONMap(values spec.LocalizedText) datatypes.JSONMap {
	if len(values) == 0 {
		return datatypes.JSONMap{}
	}
	out := datatypes.JSONMap{}
	for key, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			out[key] = text
		}
	}
	return out
}

func proposalKeyForPage(pageType spec.PageType, functionID string) string {
	functionID = strings.TrimSpace(functionID)
	if functionID == "" {
		return ""
	}
	switch pageType {
	case spec.PageTypeTask:
		return "task:" + functionID
	case spec.PageTypeReport:
		return "report:" + functionID
	default:
		return "operation:" + functionID
	}
}

func jsonValue(v interface{}) datatypes.JSON {
	raw, err := json.Marshal(v)
	if err != nil {
		return datatypes.JSON([]byte("null"))
	}
	return datatypes.JSON(raw)
}

func computeDigest(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
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
