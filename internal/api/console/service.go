package console

import (
	"context"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/svc"
)

// 绑定执行来源标记：pageSnapshotGovernance 决定 FunctionInvoke 是否跳过函数
// 级 policy 审批闸（页面级门禁已在发布快照/草稿路径前置执行过）。
const (
	governanceValidated = "validated" // 已发布快照：审批续跑按快照版本回读校验
	governanceDraft     = "draft"     // 草稿预览：页面无快照，审批续跑按契约版本校验
)

// 运行时入口标记（遥测/审计详情，不承载权限语义）。
const (
	runtimeAPIPublishedExecute = "console.binding.execute"
	runtimeAPIDraftExecute     = "versioning.composite.preview.execute"
)

// rendererSchemaVersion 与发布路径同源（api/page 同名常量）。
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
