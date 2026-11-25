package console

import (
	"context"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/svc"
)

// Binding execution origin marker: pageSnapshotGovernance decides whether
// FunctionInvoke skips the function-level policy approval gate (the page-level gate
// has already run up front in the published-snapshot / draft path).
const (
	governanceValidated = "validated" // published snapshot: approval resumption validates against the snapshot version
	governanceDraft     = "draft"     // draft preview: the page has no snapshot, so approval resumption validates against the contract version
)

// Runtime entry-point markers (telemetry/audit details; they carry no permission semantics).
const (
	runtimeAPIPublishedExecute = "console.binding.execute"
	runtimeAPIDraftExecute     = "versioning.composite.preview.execute"
)

// rendererSchemaVersion shares its source with the publish path (a same-named
// constant in api/page).
const rendererSchemaVersion = "page-spec:1"

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// Menu returns the runtime console navigation assembled from menu management
// (menu_items) — the single navigation source since T-M8. A published page
// appears only when mounted to an accessible menu; no menus → empty items
// (the console front end shows its menu-management guidance).
func (s *Service) Menu(ctx context.Context, req *ConsoleMenuRequest) (*ConsoleMenuResponse, error) {
	if err := s.requireConsoleRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	menu, err := s.generateMenuFromMenuItems(ctx, gameID, env, normalizeLanguage(req.Language))
	if err != nil {
		return nil, err
	}
	return &ConsoleMenuResponse{ConsoleMenuSpec: menu}, nil
}

func (s *Service) Pages(ctx context.Context, req *ConsolePagesRequest) (*ConsolePagesResponse, error) {
	if err := s.requireConsoleRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	publishedPages, err := s.svcCtx.PublishedPageSpecModel.ListLatestActiveByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	pages, err := s.attachBindingFreshness(ctx, parsePublishedPages(publishedPages))
	if err != nil {
		return nil, err
	}
	items := make([]spec.PublishedPageSpec, 0, len(pages))
	for _, page := range pages {
		if req.Category != "" && page.Category.Key != req.Category {
			continue
		}
		items = append(items, page)
	}
	return &ConsolePagesResponse{Items: items}, nil
}

func (s *Service) Page(ctx context.Context, req *ConsolePageRequest) (*ConsolePageResponse, error) {
	if err := s.requireConsoleRead(ctx); err != nil {
		return nil, err
	}
	gameID, env, err := requireScope(ctx)
	if err != nil {
		return nil, err
	}
	pp, err := s.svcCtx.PublishedPageSpecModel.FindLatestByScopeAndPageKey(ctx, gameID, env, req.PageKey)
	if err != nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	pageSpec := parsePublishedPageSpec(*pp)
	if pageSpec == nil {
		return nil, ErrPageNotFound(req.PageKey)
	}
	if err := s.attachBindingFreshnessToPage(ctx, pageSpec); err != nil {
		return nil, err
	}
	return &ConsolePageResponse{Page: *pageSpec}, nil
}
