package console

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// Service provides Console API operations.
type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates a new Console Service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// Menu returns the ConsoleMenuSpec generated from published pages.
func (s *Service) Menu(ctx context.Context, req *ConsoleMenuRequest) (*ConsoleMenuResponse, error) {
	lang := normalizeLanguage(req.Language)

	// Load latest published pages from database
	publishedPages, err := s.svcCtx.PublishedPageSpecModel.ListLatestActive(ctx)
	if err != nil {
		return nil, err
	}

	// Convert to spec format
	pages := make([]spec.PublishedPageSpec, 0, len(publishedPages))
	for _, pp := range publishedPages {
		pageSpec := parsePublishedPageSpec(pp)
		if pageSpec != nil {
			pages = append(pages, *pageSpec)
		}
	}

	// Generate menu
	menu := generateMenuFromPages(pages, lang)

	return &ConsoleMenuResponse{ConsoleMenuSpec: menu}, nil
}

// Pages returns all published pages, optionally filtered by category.
func (s *Service) Pages(ctx context.Context, req *ConsolePagesRequest) (*ConsolePagesResponse, error) {
	publishedPages, err := s.svcCtx.PublishedPageSpecModel.ListLatestActive(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]spec.PublishedPageSpec, 0, len(publishedPages))
	for _, pp := range publishedPages {
		pageSpec := parsePublishedPageSpec(pp)
		if pageSpec == nil {
			continue
		}

		// Apply category filter
		if req.Category != "" && pageSpec.Category.Key != req.Category {
			continue
		}

		items = append(items, *pageSpec)
	}

	return &ConsolePagesResponse{Items: items}, nil
}

// Page returns a single published page by key.
func (s *Service) Page(ctx context.Context, req *ConsolePageRequest) (*ConsolePageResponse, error) {
	pp, err := s.svcCtx.PublishedPageSpecModel.FindLatestByPageKey(ctx, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}

	pageSpec := parsePublishedPageSpec(*pp)
	if pageSpec == nil {
		return nil, ErrPageNotFound(req.PageKey)
	}

	return &ConsolePageResponse{Page: *pageSpec}, nil
}

// parsePublishedPageSpec converts a PublishedPageSpec model to spec format.
func parsePublishedPageSpec(pp model.PublishedPageSpec) *spec.PublishedPageSpec {
	var pageSpec spec.PageSpec
	if err := json.Unmarshal([]byte(pp.SpecJSON), &pageSpec); err != nil {
		return nil
	}
	if pageSpec.PageKey == "" {
		return nil
	}

	return &spec.PublishedPageSpec{
		PageSpec:    pageSpec,
		Version:     pp.Version,
		PublishedAt: pp.PublishedAt.Format("2006-01-02T15:04:05Z"),
		PublishedBy: pp.PublishedBy,
	}
}

// generateMenuFromPages builds ConsoleMenuSpec from published pages.
func generateMenuFromPages(pages []spec.PublishedPageSpec, lang string) spec.ConsoleMenuSpec {
	// Group pages by category
	categories := map[string]*categoryGroup{}

	for _, page := range pages {
		catKey := page.Category.Key
		if catKey == "" {
			continue
		}

		if _, ok := categories[catKey]; !ok {
			categories[catKey] = &categoryGroup{
				key:    catKey,
				labels: page.Category.Labels,
				order:  page.Category.Order,
				pages:  []pageEntry{},
			}
		}

		categories[catKey].pages = append(categories[catKey].pages, pageEntry{
			key:   page.PageKey,
			title: page.Title,
			icon:  page.Icon,
			order: page.Order,
		})
	}

	// Build menu items
	items := make([]spec.ConsoleMenuItem, 0, len(categories))
	for _, cat := range categories {
		// Sort pages within category
		sort.Slice(cat.pages, func(i, j int) bool {
			if cat.pages[i].order != cat.pages[j].order {
				return cat.pages[i].order < cat.pages[j].order
			}
			left := getLocalizedText(cat.pages[i].title, lang, cat.pages[i].key)
			right := getLocalizedText(cat.pages[j].title, lang, cat.pages[j].key)
			if left != right {
				return left < right
			}
			return cat.pages[i].key < cat.pages[j].key
		})

		// Build page children
		children := make([]spec.ConsoleMenuItem, 0, len(cat.pages))
		for _, p := range cat.pages {
			children = append(children, spec.ConsoleMenuItem{
				Key:    p.key,
				Path:   "/console/" + cat.key + "/" + p.key,
				Title:  p.title,
				Locale: false,
				Icon:   p.icon,
				Order:  p.order,
			})
		}

		items = append(items, spec.ConsoleMenuItem{
			Key:      cat.key,
			Path:     "/console/" + cat.key,
			Title:    cat.labels,
			Locale:   false,
			Order:    cat.order,
			Children: children,
		})
	}

	// Sort categories
	sort.Slice(items, func(i, j int) bool {
		if items[i].Order != items[j].Order {
			return items[i].Order < items[j].Order
		}
		left := getLocalizedText(items[i].Title, lang, items[i].Key)
		right := getLocalizedText(items[j].Title, lang, items[j].Key)
		if left != right {
			return left < right
		}
		return items[i].Key < items[j].Key
	})

	return spec.ConsoleMenuSpec{Items: items}
}

// normalizeLanguage normalizes language code.
func normalizeLanguage(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	switch lang {
	case "zh", "zh-cn", "zh_cn", "":
		return "zh-CN"
	case "en", "en-us", "en_us":
		return "en-US"
	default:
		return lang
	}
}

// getLocalizedText gets text from LocalizedText with fallback.
func getLocalizedText(labels spec.LocalizedText, lang, fallback string) string {
	if labels == nil {
		return fallback
	}
	if v, ok := labels[lang]; ok && v != "" {
		return v
	}
	if v, ok := labels["zh-CN"]; ok && v != "" {
		return v
	}
	for _, v := range labels {
		if v != "" {
			return v
		}
	}
	return fallback
}

type categoryGroup struct {
	key    string
	labels spec.LocalizedText
	order  int
	pages  []pageEntry
}

type pageEntry struct {
	key   string
	title spec.LocalizedText
	icon  string
	order int
}

// ErrPageNotFound returns a not-found error.
func ErrPageNotFound(key string) error {
	return &PageNotFoundError{Key: key}
}

// PageNotFoundError indicates a page was not found.
type PageNotFoundError struct {
	Key string
}

func (e *PageNotFoundError) Error() string {
	return "page not found: " + e.Key
}
