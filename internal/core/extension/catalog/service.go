package catalog

import (
	"context"

	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"gorm.io/gorm"
)

type Service struct {
	catalogRepo *extensiongorm.CatalogRepo
	releaseRepo *extensiongorm.ReleaseRepo
}

type ListQuery struct {
	Keyword string
	Kind    string
	Status  string
	Limit   int
	Offset  int
}

func NewService(catalogRepo *extensiongorm.CatalogRepo, releaseRepo *extensiongorm.ReleaseRepo) *Service {
	return &Service{catalogRepo: catalogRepo, releaseRepo: releaseRepo}
}

func (s *Service) List(ctx context.Context, q ListQuery) ([]model.ExtensionCatalog, int64, error) {
	if s == nil || s.catalogRepo == nil {
		return []model.ExtensionCatalog{}, 0, nil
	}
	return s.catalogRepo.List(ctx, extensiongorm.CatalogListQuery{
		Keyword: q.Keyword,
		Kind:    q.Kind,
		Status:  q.Status,
		Limit:   q.Limit,
		Offset:  q.Offset,
	})
}

func (s *Service) Get(ctx context.Context, extensionID string) (*model.ExtensionCatalog, []model.ExtensionRelease, error) {
	if s == nil || s.catalogRepo == nil || s.releaseRepo == nil {
		return nil, nil, gorm.ErrInvalidDB
	}
	item, err := s.catalogRepo.GetByExtensionID(ctx, extensionID)
	if err != nil {
		return nil, nil, err
	}
	releases, err := s.releaseRepo.ListByExtensionID(ctx, extensionID)
	if err != nil {
		return nil, nil, err
	}
	return item, releases, nil
}

// ListByExtensionIDs fetches catalog rows by a set of ids in batch (for installation list assembly to join real names); nil when input is empty.
func (s *Service) ListByExtensionIDs(ctx context.Context, extensionIDs []string) ([]model.ExtensionCatalog, error) {
	if s == nil || s.catalogRepo == nil {
		return nil, nil
	}
	return s.catalogRepo.GetByExtensionIDs(ctx, extensionIDs)
}

func (s *Service) Create(ctx context.Context, item *model.ExtensionCatalog) error {
	if s == nil || s.catalogRepo == nil {
		return gorm.ErrInvalidDB
	}
	return s.catalogRepo.Create(ctx, item)
}

func (s *Service) UpdateFields(ctx context.Context, extensionID string, updates map[string]any) error {
	if s == nil || s.catalogRepo == nil {
		return gorm.ErrInvalidDB
	}
	return s.catalogRepo.UpdateByExtensionID(ctx, extensionID, updates)
}

func (s *Service) Remove(ctx context.Context, extensionID string) error {
	if s == nil || s.catalogRepo == nil {
		return gorm.ErrInvalidDB
	}
	return s.catalogRepo.DeleteByExtensionID(ctx, extensionID)
}

// PublishRelease writes a release record (uniqueness validation and catalog.latestVersion rollback are handled by the caller).
func (s *Service) PublishRelease(ctx context.Context, item *model.ExtensionRelease) error {
	if s == nil || s.releaseRepo == nil {
		return gorm.ErrInvalidDB
	}
	return s.releaseRepo.Create(ctx, item)
}

func (s *Service) ReleaseByVersion(ctx context.Context, extensionID, version string) (*model.ExtensionRelease, error) {
	if s == nil || s.releaseRepo == nil {
		return nil, gorm.ErrInvalidDB
	}
	return s.releaseRepo.GetByExtensionIDAndVersion(ctx, extensionID, version)
}

func (s *Service) RemoveReleases(ctx context.Context, extensionID string) error {
	if s == nil || s.releaseRepo == nil {
		return gorm.ErrInvalidDB
	}
	return s.releaseRepo.DeleteByExtensionID(ctx, extensionID)
}
