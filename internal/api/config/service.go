package config

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// ListConfigs returns the latest version summary for each config key.
func (s *Service) ListConfigs(ctx context.Context, req *ListConfigsRequest) (*ListConfigsResponse, error) {
	if req == nil {
		req = &ListConfigsRequest{}
	}
	records, err := s.svcCtx.ConfigVersionModel.ListLatest(ctx, model.ConfigListOptions{
		GameID: configScopeValue(ctx, req.GameID, true),
		Env:    configScopeValue(ctx, req.Env, false),
		Format: req.Format,
		IDLike: req.IDLike,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ConfigItem, 0, len(records))
	for i := range records {
		items = append(items, ConfigItem{
			ID:             records[i].Key,
			Format:         records[i].Format,
			GameID:         records[i].GameID,
			Env:            records[i].Env,
			LatestVersion:  records[i].Version,
			UpdatedAt:      mapConfigItem(&records[i])["updatedAt"].(string),
			LastMessage:    records[i].Message,
			LastModifiedBy: records[i].CreatedBy,
		})
	}
	return &ListConfigsResponse{Items: items}, nil
}

// GetConfig returns the latest editable version for a single config key.
func (s *Service) GetConfig(ctx context.Context, req *GetConfigRequest) (*GetConfigResponse, error) {
	if req == nil {
		return nil, errors.New("request parameters cannot be empty")
	}
	record, err := findLatestConfig(ctx, s.svcCtx.ConfigVersionModel, req.ID)
	if err != nil {
		return nil, err
	}
	return &GetConfigResponse{
		ID:      record.Key,
		Format:  record.Format,
		Content: record.Value,
		Version: record.Version,
		GameID:  record.GameID,
		Env:     record.Env,
	}, nil
}

// Upsert creates or updates a config value
func (s *Service) Upsert(ctx context.Context, req *UpsertRequest) (*UpsertResponse, error) {
	if req == nil {
		return nil, errors.New("request body cannot be empty")
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, errors.New("config key cannot be empty")
	}

	record, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:     key,
		Content: req.Value,
		GameID:  configScopeValue(ctx, "", true),
		Env:     configScopeValue(ctx, "", false),
	}, configAuthor(ctx))
	if err != nil {
		return nil, err
	}

	versionData := mapConfigVersion(record, true)
	// 设计债清理：mapConfigVersion 的 "version" 由 model.ConfigVersion.Version（int）
	// 直接装箱而来，动态类型恒为 int，原先的 int64/int32 兼容分支不可达，已删除。
	version := versionData["version"].(int)

	return &UpsertResponse{
		Version: ConfigVersion{
			Key:       versionData["key"].(string),
			Version:   version,
			CreatedBy: versionData["createdBy"].(string),
			CreatedAt: versionData["createdAt"].(string),
			GameID:    versionData["gameId"].(string),
			Env:       versionData["env"].(string),
			Format:    versionData["format"].(string),
			Message:   versionData["message"].(string),
			Value:     versionData["value"].(string),
		},
	}, nil
}

// SaveConfig creates a new config version using the canonical RESTful contract.
func (s *Service) SaveConfig(ctx context.Context, id string, req *SaveConfigRequest) (*SaveConfigResponse, error) {
	if req == nil {
		return nil, errors.New("request body cannot be empty")
	}
	key := strings.TrimSpace(id)
	if key == "" {
		return nil, errors.New("config key cannot be empty")
	}
	record, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:         key,
		Content:     req.Content,
		Format:      strings.TrimSpace(req.Format),
		GameID:      configScopeValue(ctx, req.GameID, true),
		Env:         configScopeValue(ctx, req.Env, false),
		Message:     strings.TrimSpace(req.Message),
		BaseVersion: req.BaseVersion,
	}, configAuthor(ctx))
	if err != nil {
		return nil, err
	}
	return &SaveConfigResponse{Version: record.Version}, nil
}

// ValidateConfig validates the submitted config content according to the declared format.
// 设计债清理：纯校验函数无 IO，校验失败以 Valid=false 表达而非 error，不存在出错路径，
// 故收紧为无 error 返回（原先 handler 侧的 err 分支随之不可达并已删除）；
// req==nil 降级为空内容校验（nil 安全，唯一调用方 handler 经 bind 恒传非 nil）。
func (s *Service) ValidateConfig(req *ValidateConfigRequest) *ValidateConfigResponse {
	if req == nil {
		req = &ValidateConfigRequest{}
	}
	if err := validateConfigContent(req.Format, req.Content); err != nil {
		return &ValidateConfigResponse{Valid: false, Errors: []string{err.Error()}}
	}
	return &ValidateConfigResponse{Valid: true, Errors: []string{}}
}

// ListVersions retrieves all versions for a given config key
func (s *Service) ListVersions(ctx context.Context, req *ListVersionsRequest) (*ListVersionsResponse, error) {
	if req == nil {
		return nil, errors.New("request body cannot be empty")
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, errors.New("config key cannot be empty")
	}

	versions, err := listConfigVersions(ctx, s.svcCtx.ConfigVersionModel, key)
	if err != nil {
		return nil, err
	}

	items := make([]ConfigVersionItem, 0, len(versions))
	for i := range versions {
		versionData := mapConfigVersion(&versions[i], true)
		// 设计债清理：version 动态类型恒为 int（见 Upsert 同注），int64/int32 分支已删除。
		version := versionData["version"].(int)

		items = append(items, ConfigVersionItem{
			Key:       versionData["key"].(string),
			Version:   version,
			CreatedBy: versionData["createdBy"].(string),
			CreatedAt: versionData["createdAt"].(string),
			GameID:    versionData["gameId"].(string),
			Env:       versionData["env"].(string),
			Format:    versionData["format"].(string),
			Message:   versionData["message"].(string),
			Value:     versionData["value"].(string),
		})
	}

	return &ListVersionsResponse{
		Key:      key,
		Total:    len(items),
		Versions: items,
	}, nil
}

// GetVersion retrieves a specific version of a config
func (s *Service) GetVersion(ctx context.Context, req *GetVersionRequest) (*GetVersionResponse, error) {
	if req == nil {
		return nil, errors.New("request parameters cannot be empty")
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		return nil, errors.New("config key cannot be empty")
	}
	if req.Version <= 0 {
		return nil, errors.New("version number must be greater than 0")
	}

	record, err := findConfigVersion(ctx, s.svcCtx.ConfigVersionModel, key, req.Version)
	if err != nil {
		return nil, err
	}

	versionData := mapConfigVersion(record, true)
	// 设计债清理：version 动态类型恒为 int（见 Upsert 同注），int64/int32 分支已删除。
	version := versionData["version"].(int)

	return &GetVersionResponse{
		Version: ConfigVersion{
			Key:       versionData["key"].(string),
			Version:   version,
			CreatedBy: versionData["createdBy"].(string),
			CreatedAt: versionData["createdAt"].(string),
			GameID:    versionData["gameId"].(string),
			Env:       versionData["env"].(string),
			Format:    versionData["format"].(string),
			Message:   versionData["message"].(string),
			Value:     versionData["value"].(string),
		},
	}, nil
}

func configScopeValue(ctx context.Context, fallback string, game bool) string {
	scope := svc.GameScopeFromContext(ctx)
	if game {
		if scope.GameID != "" {
			return scope.GameID
		}
	} else if scope.Env != "" {
		return scope.Env
	}
	return strings.TrimSpace(fallback)
}

func findLatestConfig(ctx context.Context, model *model.ConfigVersionModel, key string) (*model.ConfigVersion, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID != "" && scope.Env != "" {
		return model.FindLatestByScope(ctx, key, scope.GameID, scope.Env)
	}
	return model.FindLatest(ctx, key)
}

func listConfigVersions(ctx context.Context, model *model.ConfigVersionModel, key string) ([]model.ConfigVersion, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID != "" && scope.Env != "" {
		return model.ListByScope(ctx, key, scope.GameID, scope.Env)
	}
	return model.List(ctx, key)
}

func findConfigVersion(ctx context.Context, model *model.ConfigVersionModel, key string, version int) (*model.ConfigVersion, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID != "" && scope.Env != "" {
		return model.FindByScope(ctx, key, version, scope.GameID, scope.Env)
	}
	return model.Find(ctx, key, version)
}
