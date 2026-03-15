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
