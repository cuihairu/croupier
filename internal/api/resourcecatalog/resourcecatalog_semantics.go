package resourcecatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
)

func (s *Service) createSemanticVersion(ctx context.Context, semantics *model.CapabilitySemantics, reason string, actor string) error {
	if s.versionModel == nil || semantics == nil {
		return nil
	}
	version := &model.CapabilitySemanticVersion{
		SemanticsID:  semantics.ID,
		Version:      semantics.Version,
		Semantics:    capabilitySemanticsJSON(semantics),
		SourceDigest: semantics.SourceDigest,
		ChangeReason: strings.TrimSpace(reason),
		CreatedBy:    actor,
	}
	if version.ChangeReason == "" {
		version.ChangeReason = "resource catalog semantic update"
	}
	if err := s.versionModel.CreateVersion(ctx, version); err != nil {
		return fmt.Errorf("create semantic version: %w", err)
	}
	return nil
}

func capabilitySemanticsJSON(value *model.CapabilitySemantics) model.JSON {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return model.JSON(raw)
}

func (s *Service) rebuildProposals(ctx context.Context, gameID string, env string, resourceKey string) error {
	if s.contractService == nil {
		return nil
	}
	if err := s.contractService.RebuildProposalsForResource(ctx, gameID, env, resourceKey); err != nil {
		return fmt.Errorf("rebuild proposals: %w", err)
	}
	return nil
}
