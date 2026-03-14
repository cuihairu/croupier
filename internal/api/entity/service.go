package entity

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns the list of entities
func (s *Service) List(ctx context.Context, req *EntitiesListRequest) (*EntitiesListResponse, error) {
	opts := model.ListEntitiesOptions{
		Page:     req.Page,
		PageSize: req.PageSize,
		Type:     strings.TrimSpace(req.Type),
	}

	entities, total, err := s.svcCtx.EntityModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}

	items := make([]EntityItem, 0, len(entities))
	for i := range entities {
		dto := utils.BuildEntityDTO(&entities[i])
		items = append(items, EntityItem{
			ID:         fmt.Sprint(dto["id"]),
			Type:       dto["type"].(string),
			Data:       dto["data"],
			ProviderID: getStringField(dto, "providerId"),
			Status:     getIntField(dto, "status"),
			CreatedAt:  getStringField(dto, "createdAt"),
			UpdatedAt:  getStringField(dto, "updatedAt"),
		})
	}

	return &EntitiesListResponse{
		Items: items,
		Total: total,
		Page:  opts.Page,
		Size:  opts.PageSize,
	}, nil
}

// Create creates a new entity
func (s *Service) Create(ctx context.Context, req *EntityCreateRequest) (*EntityCreateResponse, error) {
	entityType := strings.TrimSpace(req.Type)
	if entityType == "" {
		return nil, errors.New("实体类型不能为空")
	}
	if req.Data == nil {
		return nil, errors.New("实体数据不能为空")
	}

	if err := s.svcCtx.EntityModel.ValidateEntityData(entityType, req.Data); err != nil {
		return nil, err
	}

	entity := &model.Entity{
		Type:   entityType,
		Status: 1,
	}
	if err := entity.SetData(req.Data); err != nil {
		return nil, err
	}

	if err := s.svcCtx.EntityModel.Create(ctx, entity); err != nil {
		return nil, err
	}

	dto := utils.BuildEntityDTO(entity)
	return &EntityCreateResponse{
		ID:         fmt.Sprint(dto["id"]),
		Type:       dto["type"].(string),
		Data:       dto["data"],
		ProviderID: getStringField(dto, "providerId"),
		Status:     getIntField(dto, "status"),
		CreatedAt:  getStringField(dto, "createdAt"),
		UpdatedAt:  getStringField(dto, "updatedAt"),
	}, nil
}

// Detail returns the details of an entity
func (s *Service) Detail(ctx context.Context, req *EntityDetailRequest) (*EntityDetailResponse, error) {
	id, err := utils.ParseUintID(req.ID, "实体ID")
	if err != nil {
		return nil, err
	}

	entity, err := s.svcCtx.EntityModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	dto := utils.BuildEntityDTO(entity)
	return &EntityDetailResponse{
		ID:         fmt.Sprint(dto["id"]),
		Type:       dto["type"].(string),
		Data:       dto["data"],
		ProviderID: getStringField(dto, "providerId"),
		Status:     getIntField(dto, "status"),
		CreatedAt:  getStringField(dto, "createdAt"),
		UpdatedAt:  getStringField(dto, "updatedAt"),
	}, nil
}

// Update updates an entity
func (s *Service) Update(ctx context.Context, req *EntityUpdateRequest) (*EntityUpdateResponse, error) {
	id, err := utils.ParseUintID(req.ID, "实体ID")
	if err != nil {
		return nil, err
	}
	if req.Data == nil {
		return nil, errors.New("实体数据不能为空")
	}

	entity, err := s.svcCtx.EntityModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.svcCtx.EntityModel.ValidateEntityData(entity.Type, req.Data); err != nil {
		return nil, err
	}

	if err := entity.SetData(req.Data); err != nil {
		return nil, err
	}

	if err := s.svcCtx.EntityModel.Update(ctx, id, map[string]interface{}{
		"data": entity.Data,
	}); err != nil {
		return nil, err
	}

	dto := utils.BuildEntityDTO(entity)
	return &EntityUpdateResponse{
		ID:         fmt.Sprint(dto["id"]),
		Type:       dto["type"].(string),
		Data:       dto["data"],
		ProviderID: getStringField(dto, "providerId"),
		Status:     getIntField(dto, "status"),
		CreatedAt:  getStringField(dto, "createdAt"),
		UpdatedAt:  getStringField(dto, "updatedAt"),
	}, nil
}

// Delete deletes an entity
func (s *Service) Delete(ctx context.Context, req *EntityDeleteRequest) (*EntityDeleteResponse, error) {
	id, err := utils.ParseUintID(req.ID, "实体ID")
	if err != nil {
		return nil, err
	}

	if err := s.svcCtx.EntityModel.Delete(ctx, id); err != nil {
		return nil, err
	}

	return &EntityDeleteResponse{}, nil
}

// Preview returns a preview of an entity
func (s *Service) Preview(ctx context.Context, req *EntityPreviewRequest) (*EntityPreviewResponse, error) {
	id, err := utils.ParseUintID(req.ID, "实体ID")
	if err != nil {
		return nil, err
	}

	entity, err := s.svcCtx.EntityModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}

	dto := utils.BuildEntityDTO(entity)
	return &EntityPreviewResponse{
		Data: dto["data"],
	}, nil
}

// Validate validates entity data
func (s *Service) Validate(ctx context.Context, req *EntityValidateRequest) (*EntityValidateResponse, error) {
	entityType := strings.TrimSpace(req.Type)
	if entityType == "" {
		return nil, errors.New("实体类型不能为空")
	}
	if req.Data == nil {
		return nil, errors.New("实体数据不能为空")
	}

	if err := s.svcCtx.EntityModel.ValidateEntityData(entityType, req.Data); err != nil {
		return nil, err
	}

	return &EntityValidateResponse{
		Valid: true,
	}, nil
}

// Helper functions to safely extract fields from map[string]interface{}

func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getIntField(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok && v != nil {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case string:
			if i, err := strconv.Atoi(val); err == nil {
				return i
			}
		}
	}
	return 0
}
