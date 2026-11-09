package versioning

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	dashboardmerge "github.com/cuihairu/croupier/internal/dashboard/merge"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service"
)

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
		if latest.Risk.String() != string(frozen.Risk) {
			changes = append(changes, FieldChange{
				Path:       "bindings." + frozen.BindingID + ".risk",
				OldValue:   jsonString(string(frozen.Risk)),
				NewValue:   jsonString(latest.Risk.String()),
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
			Risk:                  spec.RiskLevel(contract.Risk.String()),
			Permission:            strings.TrimSpace(contract.Permission),
			Approval:              service.ApprovalPolicyFromJSONMap(contract.Approval),
			ExecutionMode:         binding.Execution.Mode,
			RendererSchemaVersion: rendererSchemaVersion,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].BindingID < out[j].BindingID })
	return out, nil
}
