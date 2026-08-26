// Package tool implements the internal toolbox API (tool registry for
// Jenkins/GitLab/Grafana/Wiki links; see docs/research/tool-registry-design.md).
package tool

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates a tool service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns tools visible in the caller's scope (global + matching pair).
func (s *Service) List(ctx context.Context, req *ToolListRequest) (*ToolListResponse, error) {
	items, err := s.svcCtx.ToolModel.List(ctx, model.ToolQueryOptions{
		GameID: strings.TrimSpace(req.GameID),
		Env:    strings.TrimSpace(req.Env),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Tool, 0, len(items))
	for i := range items {
		out = append(out, buildToolDTO(&items[i]))
	}
	return &ToolListResponse{Items: out}, nil
}

// Create registers a tool.
func (s *Service) Create(ctx context.Context, req *ToolCreateRequest) (*ToolCreateResponse, error) {
	tool := &model.ToolLink{
		Name:        req.Name,
		URL:         req.URL,
		Description: strings.TrimSpace(req.Description),
		Category:    strings.TrimSpace(req.Category),
		Icon:        strings.TrimSpace(req.Icon),
		GameID:      strings.TrimSpace(req.GameID),
		Env:         strings.TrimSpace(req.Env),
		Enabled:     true,
		Sort:        req.Sort,
		CreatedBy:   currentUsername(ctx),
	}
	if err := model.ValidateToolLink(tool); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	if err := s.svcCtx.ToolModel.Create(ctx, tool); err != nil {
		return nil, err
	}
	return &ToolCreateResponse{Tool: buildToolDTO(tool)}, nil
}

// Update applies a partial update.
func (s *Service) Update(ctx context.Context, req *ToolUpdateRequest) (*ToolUpdateResponse, error) {
	id, err := utils.ParseUintID(req.ID, "工具 ID")
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Name); v != "" {
		updates["name"] = v
	}
	if req.URL != nil {
		updates["url"] = strings.TrimSpace(*req.URL)
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.Category != nil {
		updates["category"] = strings.TrimSpace(*req.Category)
	}
	if req.Icon != nil {
		updates["icon"] = strings.TrimSpace(*req.Icon)
	}
	if req.Sort != nil {
		updates["sort"] = *req.Sort
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.GameID != nil {
		updates["game_id"] = strings.TrimSpace(*req.GameID)
	}
	if req.Env != nil {
		updates["env"] = strings.TrimSpace(*req.Env)
	}
	if len(updates) == 0 {
		return nil, errorx.NewBadRequest("请提供需要更新的字段")
	}
	if err := s.svcCtx.ToolModel.Update(ctx, id, updates); err != nil {
		return nil, err
	}
	items, err := s.svcCtx.ToolModel.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &ToolUpdateResponse{Tool: buildToolDTO(&items[i])}, nil
		}
	}
	return nil, errorx.NewBadRequest("工具不存在")
}

// Delete removes a tool.
func (s *Service) Delete(ctx context.Context, req *ToolDeleteRequest) error {
	id, err := utils.ParseUintID(req.ID, "工具 ID")
	if err != nil {
		return err
	}
	return s.svcCtx.ToolModel.Delete(ctx, id)
}

func currentUsername(ctx context.Context) string {
	if name, err := utils.CurrentUsername(ctx); err == nil && name != "" {
		return name
	}
	return "system"
}

func buildToolDTO(t *model.ToolLink) Tool {
	return Tool{
		Id:          int64(t.ID),
		Name:        t.Name,
		Url:         t.URL,
		Description: t.Description,
		Category:    t.Category,
		Icon:        t.Icon,
		GameId:      t.GameID,
		Env:         t.Env,
		Enabled:     t.Enabled,
		Sort:        t.Sort,
		CreatedBy:   t.CreatedBy,
		UpdatedAt:   utils.FormatTimestamp(t.UpdatedAt),
	}
}
