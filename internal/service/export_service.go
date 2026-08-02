package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"gorm.io/gorm"
)

// DataExportService provides data export and backup functionality.
type DataExportService struct {
	db *gorm.DB
}

// NewDataExportService creates the service.
func NewDataExportService(db *gorm.DB) *DataExportService {
	return &DataExportService{db: db}
}

// PageSpecExport represents exported page spec data.
type PageSpecExport struct {
	GameID           string    `json:"gameId"`
	Env              string    `json:"env"`
	PageKey          string    `json:"pageKey"`
	Type             string    `json:"type"`
	ResourceKey      string    `json:"resourceKey,omitempty"`
	Title            string    `json:"title"`
	SpecJSON         string    `json:"specJson"`
	Status           string    `json:"status"`
	DraftRevision    int       `json:"draftRevision"`
	PublishedVersion int       `json:"publishedVersion"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// PublishedPageExport represents exported published page data.
type PublishedPageExport struct {
	GameID      string    `json:"gameId"`
	Env         string    `json:"env"`
	PageKey     string    `json:"pageKey"`
	Version     int       `json:"version"`
	SpecJSON    string    `json:"specJson"`
	Active      bool      `json:"active"`
	PublishedAt time.Time `json:"publishedAt"`
	PublishedBy string    `json:"publishedBy,omitempty"`
}

// ExportReport is the complete export report.
type ExportReport struct {
	ExportedAt     time.Time             `json:"exportedAt"`
	GameID         string                `json:"gameId,omitempty"`
	Env            string                `json:"env,omitempty"`
	PageSpecs      []PageSpecExport      `json:"pageSpecs"`
	PublishedPages []PublishedPageExport `json:"publishedPages"`
	Summary        ExportSummary         `json:"summary"`
}

// ExportSummary provides summary statistics.
type ExportSummary struct {
	TotalPageSpecs      int `json:"totalPageSpecs"`
	TotalPublishedPages int `json:"totalPublishedPages"`
	DraftPages          int `json:"draftPages"`
	PublishedPages      int `json:"publishedPages"`
	ArchivedPages       int `json:"archivedPages"`
}

// ExportAllPages exports all page spec data as a read-only report.
// This is used before deleting historical data to create a backup.
func (s *DataExportService) ExportAllPages(ctx context.Context, gameID, env string) (*ExportReport, error) {
	report := &ExportReport{
		ExportedAt: time.Now(),
		GameID:     gameID,
		Env:        env,
	}

	// Export page specs
	var pageSpecs []model.PageSpec
	query := s.db.WithContext(ctx)
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}

	if err := query.Find(&pageSpecs).Error; err != nil {
		return nil, fmt.Errorf("export page specs: %w", err)
	}

	for _, ps := range pageSpecs {
		export := PageSpecExport{
			GameID:           ps.GameID,
			Env:              ps.Env,
			PageKey:          ps.PageKey,
			Type:             ps.Type,
			ResourceKey:      ps.ResourceKey,
			Title:            ps.TitleJSON,
			SpecJSON:         ps.SpecJSON,
			Status:           ps.Status,
			DraftRevision:    ps.DraftRevision,
			PublishedVersion: ps.PublishedVersion,
			CreatedAt:        ps.CreatedAt,
			UpdatedAt:        ps.UpdatedAt,
		}
		report.PageSpecs = append(report.PageSpecs, export)

		// Update summary
		report.Summary.TotalPageSpecs++
		switch ps.Status {
		case "draft":
			report.Summary.DraftPages++
		case "published":
			report.Summary.PublishedPages++
		case "archived":
			report.Summary.ArchivedPages++
		}
	}

	// Export published pages
	var publishedPages []model.PublishedPageSpec
	query = s.db.WithContext(ctx)
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}

	if err := query.Find(&publishedPages).Error; err != nil {
		return nil, fmt.Errorf("export published pages: %w", err)
	}

	for _, pp := range publishedPages {
		export := PublishedPageExport{
			GameID:      pp.GameID,
			Env:         pp.Env,
			PageKey:     pp.PageKey,
			Version:     pp.Version,
			SpecJSON:    pp.SpecJSON,
			Active:      pp.Active,
			PublishedAt: pp.PublishedAt,
			PublishedBy: pp.PublishedBy,
		}
		report.PublishedPages = append(report.PublishedPages, export)
		report.Summary.TotalPublishedPages++
	}

	return report, nil
}

// ExportToJSON exports the report as JSON bytes.
func (s *DataExportService) ExportToJSON(ctx context.Context, gameID, env string) ([]byte, error) {
	report, err := s.ExportAllPages(ctx, gameID, env)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(report, "", "  ")
}
