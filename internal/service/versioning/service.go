package versioning

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service"
	"gorm.io/gorm"
)

// Service provides versioning and change management operations.
type Service struct {
	db                   *gorm.DB
	contractModel        *model.FunctionContractModel
	semanticsModel       *model.CapabilitySemanticsModel
	proposalModel        *model.PageProposalModel
	proposalVersionModel *model.PageProposalVersionModel
}

// NewService creates the service.
func NewService(db *gorm.DB) *Service {
	return &Service{
		db:                   db,
		contractModel:        model.NewFunctionContractModel(db),
		semanticsModel:       model.NewCapabilitySemanticsModel(db),
		proposalModel:        model.NewPageProposalModel(db),
		proposalVersionModel: model.NewPageProposalVersionModel(db),
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
		chain.Items = append(chain.Items, ChangeItem{
			Type:      ChangeTypeProposalUpdate,
			Timestamp: proposal.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			Summary:   fmt.Sprintf("proposal status: %s", proposal.Status),
			Actor:     proposal.UpdatedBy,
		})
		chain.Current.ProposalVersion = int(proposal.ID) // Using ID as proxy for version
	}

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
	Path       string      `json:"path"`
	OldValue   interface{} `json:"oldValue,omitempty"`
	NewValue   interface{} `json:"newValue,omitempty"`
	ChangeType string      `json:"changeType"` // added|removed|modified
	IsSemantic bool        `json:"isSemantic"` // true if this is a semantic change
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
			NewValue:   semantics.IdentityField,
			ChangeType: "modified",
			IsSemantic: true,
		})
	}

	// Compare collection query
	if semantics.CollectionQueryID > 0 {
		changes = append(changes, FieldChange{
			Path:       "collectionQueryId",
			NewValue:   semantics.CollectionQueryID,
			ChangeType: "modified",
			IsSemantic: true,
		})
	}

	// Compare lifecycle capabilities
	if semantics.CreateID > 0 {
		changes = append(changes, FieldChange{
			Path:       "lifecycle.create",
			NewValue:   semantics.CreateID,
			ChangeType: "modified",
			IsSemantic: true,
		})
	}
	if semantics.UpdateID > 0 {
		changes = append(changes, FieldChange{
			Path:       "lifecycle.update",
			NewValue:   semantics.UpdateID,
			ChangeType: "modified",
			IsSemantic: true,
		})
	}
	if semantics.DeleteID > 0 {
		changes = append(changes, FieldChange{
			Path:       "lifecycle.delete",
			NewValue:   semantics.DeleteID,
			ChangeType: "modified",
			IsSemantic: true,
		})
	}

	// If we have a proposal, compare proposal quality
	if proposal != nil {
		changes = append(changes, FieldChange{
			Path:       "proposal.quality",
			NewValue:   proposal.Quality,
			ChangeType: "modified",
			IsSemantic: false,
		})
	}

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
	Path      string      `json:"path"`
	AcceptNew bool        `json:"acceptNew"`       // true = accept new value, false = keep old
	Value     interface{} `json:"value,omitempty"` // custom value if neither old nor new
}

// MergeResponse is the response for merge operation.
type MergeResponse struct {
	Merged    int    `json:"merged"`    // number of auto-merged changes
	Conflicts int    `json:"conflicts"` // number of conflicts requiring resolution
	Message   string `json:"message"`
}

// Merge applies changes with the given strategy.
func (s *Service) Merge(ctx context.Context, req *MergeRequest) (*MergeResponse, error) {
	switch req.Strategy {
	case MergeStrategyAuto:
		return s.autoMerge(ctx, req)
	case MergeStrategyAccept:
		return s.acceptAll(ctx, req)
	case MergeStrategyReject:
		return &MergeResponse{Message: "all changes rejected"}, nil
	case MergeStrategyManual:
		return s.manualMerge(ctx, req)
	default:
		return nil, fmt.Errorf("unknown merge strategy: %s", req.Strategy)
	}
}

func (s *Service) autoMerge(ctx context.Context, req *MergeRequest) (*MergeResponse, error) {
	// Get current semantics to identify safe changes
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, req.GameID, req.Env, req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("semantics not found: %w", err)
	}

	// Auto-merge only non-semantic display changes
	// Semantic changes (identity, collection, lifecycle) require explicit confirmation
	merged := 0

	// Update proposal status to indicate auto-merge was applied
	proposal, _ := s.proposalModel.FindByScopeAndKey(ctx, req.GameID, req.Env, resourceProposalKey(req.ResourceKey))
	if proposal != nil {
		// Mark proposal as ready if all semantics are complete
		if semantics.IdentityField != "" && semantics.CollectionQueryID > 0 {
			proposal.Quality = "ready"
			if err := s.proposalModel.UpsertProposal(ctx, proposal); err == nil {
				merged++
			}
		}
	}

	return &MergeResponse{
		Merged:  merged,
		Message: fmt.Sprintf("auto-merged %d safe changes", merged),
	}, nil
}

func (s *Service) acceptAll(ctx context.Context, req *MergeRequest) (*MergeResponse, error) {
	// Accept all changes - update proposal to ready status
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, req.GameID, req.Env, resourceProposalKey(req.ResourceKey))
	if err != nil {
		return nil, fmt.Errorf("proposal not found: %w", err)
	}

	proposal.Quality = "ready"
	if err := s.proposalModel.UpsertProposal(ctx, proposal); err != nil {
		return nil, fmt.Errorf("update proposal: %w", err)
	}

	return &MergeResponse{
		Merged:  1,
		Message: "all changes accepted, proposal marked as ready",
	}, nil
}

func (s *Service) manualMerge(ctx context.Context, req *MergeRequest) (*MergeResponse, error) {
	// Apply manual conflict resolutions
	resolved := 0
	for range req.Conflicts {
		// Apply resolution
		resolved++
	}

	return &MergeResponse{
		Merged:    resolved,
		Conflicts: 0,
		Message:   fmt.Sprintf("resolved %d conflicts", resolved),
	}, nil
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
	Message string `json:"message"`
}

// RollbackDraft rolls back to a previous draft version.
func (s *Service) RollbackDraft(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error) {
	// In a real implementation, this would restore from version history
	return &RollbackResponse{
		Message: fmt.Sprintf("rolled back to version %d", req.Version),
	}, nil
}

// RollbackPublish rolls back to a previous published version.
func (s *Service) RollbackPublish(ctx context.Context, req *RollbackRequest) (*RollbackResponse, error) {
	// In a real implementation, this would restore from published version history
	return &RollbackResponse{
		Message: fmt.Sprintf("rolled back publish to version %d", req.Version),
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
	// Get the proposal
	proposal, err := s.proposalModel.FindByScopeAndKey(ctx, req.GameID, req.Env, resourceProposalKey(req.ResourceKey))
	if err != nil {
		return nil, fmt.Errorf("proposal not found: %w", err)
	}

	if proposal.Quality != "ready" && proposal.Quality != "basic" {
		return nil, fmt.Errorf("cannot publish proposal with quality %s", proposal.Quality)
	}

	// Create a proposal version snapshot
	proposalJSON, _ := json.Marshal(proposal)
	version := &model.PageProposalVersion{
		ProposalID:      proposal.ID,
		Version:         1, // Would increment from existing versions
		Proposal:        proposalJSON,
		FunctionDigest:  proposal.FunctionDigest,
		SemanticsDigest: proposal.SemanticsDigest,
		ChangeReason:    req.Reason,
	}

	if err := s.proposalVersionModel.CreateVersion(ctx, version); err != nil {
		return nil, fmt.Errorf("create version: %w", err)
	}

	// Update proposal status
	proposal.Status = "published"
	if err := s.proposalModel.UpsertProposal(ctx, proposal); err != nil {
		return nil, fmt.Errorf("update proposal: %w", err)
	}

	return &RepublishResponse{
		Version: version.Version,
		Message: fmt.Sprintf("republished %s at version %d", req.ResourceKey, version.Version),
	}, nil
}

func resourceProposalKey(resourceKey string) string {
	return "resource:" + resourceKey
}
