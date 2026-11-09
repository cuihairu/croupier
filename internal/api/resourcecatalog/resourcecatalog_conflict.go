package resourcecatalog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

func (s *Service) findSemanticsOptional(ctx context.Context, gameID, env, resourceKey string) (*model.CapabilitySemantics, error) {
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, gameID, env, resourceKey)
	if err == nil {
		return semantics, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, fmt.Errorf("find semantics for resource %s: %w", resourceKey, err)
}

func semanticSourceDigest(ctx context.Context, contractModel *model.FunctionContractModel, gameID, env, resourceKey string) string {
	if contractModel == nil {
		return ""
	}
	contracts, err := contractModel.ListByResourceKey(ctx, gameID, env, resourceKey)
	if err != nil {
		return ""
	}
	raw, err := json.Marshal(contracts)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256Bytes(raw))
}

func sha256Bytes(raw []byte) []byte {
	sum := sha256.Sum256(raw)
	return sum[:]
}

// ---------------------------------------------------------------------------
// Conflict Management API
// ---------------------------------------------------------------------------

// ConflictInfo represents a semantic conflict for API response.
type ConflictInfo struct {
	Field      string            `json:"field"`
	Values     map[string]string `json:"values"` // source -> value
	Resolution string            `json:"resolution,omitempty"`
	ResolvedAt string            `json:"resolvedAt,omitempty"`
	ResolvedBy string            `json:"resolvedBy,omitempty"`
}

// ProvenanceInfo represents field-level provenance for API response.
type ProvenanceInfo struct {
	Field      string `json:"field"`
	Source     string `json:"source"`
	Confidence string `json:"confidence"`
	Status     string `json:"status"`
	Value      string `json:"value,omitempty"`
	UpdatedAt  string `json:"updatedAt"`
	UpdatedBy  string `json:"updatedBy"`
}

// ListConflictsRequest is the request for listing conflicts.
type ListConflictsRequest struct {
	GameID      string `json:"-"`
	Env         string `json:"-"`
	ResourceKey string `json:"-" uri:"resourceKey" binding:"required"`
}

// ListConflictsResponse is the response for listing conflicts.
type ListConflictsResponse struct {
	Conflicts  []ConflictInfo   `json:"conflicts"`
	Provenance []ProvenanceInfo `json:"provenance"`
}

// ListConflicts returns conflicts and provenance for a resource.
func (s *Service) ListConflicts(ctx context.Context, req *ListConflictsRequest) (*ListConflictsResponse, error) {
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &ListConflictsResponse{
				Conflicts:  []ConflictInfo{},
				Provenance: []ProvenanceInfo{},
			}, nil
		}
		return nil, fmt.Errorf("find semantics: %w", err)
	}

	response := &ListConflictsResponse{
		Conflicts:  make([]ConflictInfo, 0),
		Provenance: make([]ProvenanceInfo, 0),
	}

	// Parse conflicts
	if len(semantics.Conflicts) > 0 {
		var conflicts []spec.SemanticConflict
		if err := json.Unmarshal(semantics.Conflicts, &conflicts); err == nil {
			for _, c := range conflicts {
				values := make(map[string]string)
				for source, val := range c.Values {
					values[string(source)] = string(val)
				}
				response.Conflicts = append(response.Conflicts, ConflictInfo{
					Field:      c.Field,
					Values:     values,
					Resolution: string(c.Resolution),
					ResolvedAt: c.ResolvedAt,
					ResolvedBy: c.ResolvedBy,
				})
			}
		}
	}

	// Parse provenance
	if len(semantics.Provenance) > 0 {
		var provenance map[string]*spec.SemanticProvenance
		if err := json.Unmarshal(semantics.Provenance, &provenance); err == nil {
			for _, p := range provenance {
				response.Provenance = append(response.Provenance, ProvenanceInfo{
					Field:      p.Field,
					Source:     string(p.Source),
					Confidence: p.Confidence,
					Status:     p.Status,
					Value:      string(p.Value),
					UpdatedAt:  p.UpdatedAt,
					UpdatedBy:  p.UpdatedBy,
				})
			}
		}
	}

	return response, nil
}

// ResolveConflictRequest is the request for resolving a conflict.
type ResolveConflictRequest struct {
	GameID       string `json:"-"`
	Env          string `json:"-"`
	ResourceKey  string `json:"-" uri:"resourceKey" binding:"required"`
	Field        string `json:"field" uri:"field" binding:"required"`
	ChosenSource string `json:"chosenSource"` // platform_review|sdk_explicit|openapi_rest
	Reason       string `json:"reason,omitempty"`
}

// ResolveConflictResponse is the response for resolving a conflict.
type ResolveConflictResponse struct {
	Message string `json:"message"`
}

// ResolveConflict resolves a semantic conflict by choosing a source.
func (s *Service) ResolveConflict(ctx context.Context, req *ResolveConflictRequest) (*ResolveConflictResponse, error) {
	actor := actorFromContext(ctx)
	chosenSource := spec.SemanticSource(strings.TrimSpace(req.ChosenSource))
	if !isValidSemanticSource(chosenSource) {
		return nil, fmt.Errorf("invalid chosenSource %s", req.ChosenSource)
	}
	semantics, err := s.semanticsModel.FindByScopeAndResourceKey(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey)
	if err != nil {
		return nil, fmt.Errorf("find semantics: %w", err)
	}
	if sourceDigest := semanticSourceDigest(ctx, s.contractModel, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey); sourceDigest != "" {
		semantics.SourceDigest = sourceDigest
	}

	// Parse existing conflicts
	var conflicts []spec.SemanticConflict
	if len(semantics.Conflicts) > 0 {
		if err := json.Unmarshal(semantics.Conflicts, &conflicts); err != nil {
			return nil, fmt.Errorf("parse conflicts: %w", err)
		}
	}

	// Find the conflict
	found := false
	for i, c := range conflicts {
		if c.Field == req.Field {
			// Validate chosen source exists in conflict
			if _, ok := c.Values[chosenSource]; !ok {
				return nil, fmt.Errorf("source %s not found in conflict values", req.ChosenSource)
			}

			// Resolve conflict
			conflicts[i].Resolution = chosenSource
			conflicts[i].ResolvedAt = time.Now().UTC().Format(time.RFC3339)
			conflicts[i].ResolvedBy = actor
			if err := applySemanticFieldValue(semantics, req.Field, c.Values[chosenSource]); err != nil {
				return nil, err
			}

			// Update provenance
			var provenance map[string]*spec.SemanticProvenance
			if len(semantics.Provenance) > 0 {
				_ = json.Unmarshal(semantics.Provenance, &provenance)
			}
			if provenance == nil {
				provenance = make(map[string]*spec.SemanticProvenance)
			}

			if prov, exists := provenance[req.Field]; exists {
				prov.Value = c.Values[chosenSource]
				prov.Source = chosenSource
				prov.SourceDigest = semantics.SourceDigest
				prov.Confidence = confidenceForSource(chosenSource)
				prov.Status = "effective"
				prov.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
				prov.UpdatedBy = actor
			} else {
				provenance[req.Field] = provenanceRecord(req.Field, chosenSource, semantics.SourceDigest, c.Values[chosenSource], confidenceForSource(chosenSource), "effective", actor)
			}

			// Save provenance
			provenanceJSON, _ := json.Marshal(provenance)
			semantics.Provenance = provenanceJSON

			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("conflict not found for field %s", req.Field)
	}

	// Save conflicts
	conflictsJSON, _ := json.Marshal(conflicts)
	semantics.Conflicts = conflictsJSON
	semantics.UpdatedBy = actor

	// Update semantics
	// 冲突决议：显式人工动作，零改动提交也要留版本历史，changed 无需判断。
	if _, err := s.semanticsModel.UpsertSemantics(ctx, semantics); err != nil {
		return nil, fmt.Errorf("update semantics: %w", err)
	}
	if err := s.createSemanticVersion(ctx, semantics, req.Reason, actor); err != nil {
		return nil, err
	}
	if err := s.rebuildProposals(ctx, svc.ResolveGameID(ctx, req.GameID), svc.ResolveEnv(ctx, req.Env), req.ResourceKey); err != nil {
		return nil, err
	}

	// Audit log
	if s.auditService != nil {
		if _, err := s.auditService.Log(ctx, audit.EventSemanticConflictResolve,
			audit.WithActorID(actor, "user", actor),
			audit.WithResourceID("resource_catalog", req.ResourceKey),
			audit.WithDetails(map[string]interface{}{
				"field":         req.Field,
				"chosen_source": req.ChosenSource,
				"reason":        req.Reason,
			}),
		); err != nil {
			// 副作用（冲突已解决）已完成：呈现缺口而非静默，也不返回 5xx。
			s.auditService.ReportWriteGap(ctx, audit.EventSemanticConflictResolve, err)
		}
	}

	return &ResolveConflictResponse{
		Message: fmt.Sprintf("Conflict resolved for field %s", req.Field),
	}, nil
}

func isValidSemanticSource(source spec.SemanticSource) bool {
	switch source {
	case spec.SemanticSourcePlatformReview, spec.SemanticSourceSDKExplicit, spec.SemanticSourceOpenAPIRest:
		return true
	default:
		return false
	}
}

func confidenceForSource(source spec.SemanticSource) string {
	switch source {
	case spec.SemanticSourcePlatformReview, spec.SemanticSourceSDKExplicit:
		return "high"
	case spec.SemanticSourceOpenAPIRest:
		return "low"
	default:
		return "low"
	}
}

func actorFromContext(ctx context.Context) string {
	actor, err := logicutils.CurrentUsername(ctx)
	if err != nil || strings.TrimSpace(actor) == "" {
		return "system"
	}
	return strings.TrimSpace(actor)
}

func provenanceRecord(field string, source spec.SemanticSource, sourceDigest string, value json.RawMessage, confidence string, status string, actor string) *spec.SemanticProvenance {
	return &spec.SemanticProvenance{
		Field:        field,
		Source:       source,
		SourceDigest: sourceDigest,
		Confidence:   confidence,
		Status:       status,
		Value:        value,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		UpdatedBy:    actor,
	}
}

func parseProvenance(raw []byte) map[string]*spec.SemanticProvenance {
	provenance := map[string]*spec.SemanticProvenance{}
	if len(raw) == 0 {
		return provenance
	}
	_ = json.Unmarshal(raw, &provenance)
	if provenance == nil {
		return map[string]*spec.SemanticProvenance{}
	}
	return provenance
}

func rawJSONString(value string) json.RawMessage {
	return json.RawMessage(strconv.Quote(value))
}

func rawJSONUint(value uint) json.RawMessage {
	return json.RawMessage(strconv.FormatUint(uint64(value), 10))
}

func applySemanticFieldValue(semantics *model.CapabilitySemantics, field string, raw json.RawMessage) error {
	switch strings.TrimSpace(field) {
	case "identityField":
		return assignString(raw, &semantics.IdentityField)
	case "identityFieldType":
		return assignString(raw, &semantics.IdentityFieldType)
	case "identityPath":
		return assignString(raw, &semantics.IdentityPath)
	case "collectionQueryID", "collectionQueryId":
		return assignUint(raw, &semantics.CollectionQueryID)
	case "collectionPath":
		return assignString(raw, &semantics.CollectionPath)
	case "pageFieldName":
		return assignString(raw, &semantics.PageFieldName)
	case "pageSizeFieldName":
		return assignString(raw, &semantics.PageSizeFieldName)
	case "itemsFieldName":
		return assignString(raw, &semantics.ItemsFieldName)
	case "totalFieldName":
		return assignString(raw, &semantics.TotalFieldName)
	case "itemQueryID", "itemQueryId":
		return assignUint(raw, &semantics.ItemQueryID)
	case "itemPath":
		return assignString(raw, &semantics.ItemPath)
	case "createID", "createId":
		return assignUint(raw, &semantics.CreateID)
	case "updateID", "updateId":
		return assignUint(raw, &semantics.UpdateID)
	case "deleteID", "deleteId":
		return assignUint(raw, &semantics.DeleteID)
	case "actions":
		semantics.Actions = model.JSON(raw)
	case "tasks":
		semantics.Tasks = model.JSON(raw)
	case "reports":
		semantics.Reports = model.JSON(raw)
	default:
		return fmt.Errorf("unsupported semantic conflict field %s", field)
	}
	return nil
}

func assignString(raw json.RawMessage, target *string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("semantic field must be string: %w", err)
	}
	*target = strings.TrimSpace(value)
	return nil
}

func assignUint(raw json.RawMessage, target *uint) error {
	var value uint
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("semantic field must be unsigned integer: %w", err)
	}
	*target = value
	return nil
}
