package resource

import (
	"context"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/descriptors"
	"github.com/cuihairu/croupier/internal/dashboard/generator"
	"github.com/cuihairu/croupier/internal/dashboard/normalizer"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/svc"
)

// Service provides Resource API operations.
type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates a new Resource Service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns normalized ResourceSpec list from registered functions.
func (s *Service) List(ctx context.Context, req *ResourceListRequest) (*ResourceListResponse, error) {
	if err := s.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	// Get all function descriptors and normalize them
	inputs := descriptors.Collect(ctx, s.svcCtx)
	_, resources := normalizer.NormalizeBatch(inputs)

	// Convert map to slice
	items := make([]spec.ResourceSpec, 0, len(resources))
	for _, r := range resources {
		if r == nil {
			continue
		}
		// Apply category filter
		if req.Category != "" && r.Category.Key != req.Category {
			continue
		}
		// Apply search filter
		if req.Query != "" {
			q := strings.ToLower(req.Query)
			if !strings.Contains(strings.ToLower(r.Key), q) &&
				!matchesLocalizedText(r.Labels, q) {
				continue
			}
		}
		items = append(items, *r)
	}

	// Sort by category order, then key
	sort.Slice(items, func(i, j int) bool {
		if items[i].Category.Order != items[j].Category.Order {
			return items[i].Category.Order < items[j].Category.Order
		}
		return items[i].Key < items[j].Key
	})

	return &ResourceListResponse{Items: items}, nil
}

// Detail returns a single ResourceSpec by key.
func (s *Service) Detail(ctx context.Context, req *ResourceDetailRequest) (*ResourceDetailResponse, error) {
	if err := s.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	inputs := descriptors.Collect(ctx, s.svcCtx)
	_, resources := normalizer.NormalizeBatch(inputs)

	r, ok := resources[req.ResourceKey]
	if !ok || r == nil {
		return nil, ErrResourceNotFound(req.ResourceKey)
	}

	return &ResourceDetailResponse{Resource: *r}, nil
}

// Operations returns OperationSpec list for a resource.
func (s *Service) Operations(ctx context.Context, req *ResourceOperationsRequest) (*ResourceOperationsResponse, error) {
	if err := s.requireResourceRead(ctx); err != nil {
		return nil, err
	}
	inputs := descriptors.Collect(ctx, s.svcCtx)
	_, resources := normalizer.NormalizeBatch(inputs)

	r, ok := resources[req.ResourceKey]
	if !ok || r == nil {
		return nil, ErrResourceNotFound(req.ResourceKey)
	}

	return &ResourceOperationsResponse{Items: r.Operations}, nil
}

// GeneratedPages returns generated PageSpec suggestions for a resource.
func (s *Service) GeneratedPages(ctx context.Context, req *ResourceGeneratedPagesRequest) (*ResourceGeneratedPagesResponse, error) {
	if err := s.requireResourceDiagnose(ctx); err != nil {
		return nil, err
	}
	inputs := descriptors.Collect(ctx, s.svcCtx)
	_, resources := normalizer.NormalizeBatch(inputs)

	r, ok := resources[req.ResourceKey]
	if !ok || r == nil {
		return nil, ErrResourceNotFound(req.ResourceKey)
	}

	// Use generator to create page suggestions
	opts := generator.DefaultGenerateOptions()
	pages := generator.GenerateForResource(*r, opts)

	// Collect all diagnostics
	var diags []spec.Diagnostic
	for _, page := range pages {
		diags = append(diags, page.Diagnostics...)
	}

	return &ResourceGeneratedPagesResponse{
		Items:       pages,
		Diagnostics: diags,
	}, nil
}

// matchesLocalizedText checks if any localized text value matches the query.
func matchesLocalizedText(labels spec.LocalizedText, query string) bool {
	for _, v := range labels {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}

func (s *Service) requireResourceRead(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权查看资源", "admin:all", "resources:read", "resources:diagnose", "functions:read", "functions:manage", "pages:read", "pages:edit")
	return err
}

func (s *Service) requireResourceDiagnose(ctx context.Context) error {
	_, _, err := logicutils.RequireAnyPermission(ctx, s.svcCtx, "无权生成页面候选", "admin:all", "resources:diagnose", "functions:manage", "pages:edit")
	return err
}

// ErrResourceNotFound returns a not-found error for a resource key.
func ErrResourceNotFound(key string) error {
	return &ResourceNotFoundError{Key: key}
}

// ResourceNotFoundError indicates a resource was not found.
type ResourceNotFoundError struct {
	Key string
}

func (e *ResourceNotFoundError) Error() string {
	return "resource not found: " + e.Key
}
