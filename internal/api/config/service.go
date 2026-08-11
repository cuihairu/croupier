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
		GameID: svc.ResolveGameID(ctx, req.GameID),
		Env:    svc.ResolveEnv(ctx, req.Env),
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
	record, err := s.svcCtx.ConfigVersionModel.FindLatest(ctx, req.ID)
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

	record, err := s.svcCtx.ConfigVersionModel.Create(ctx, key, req.Value, configAuthor(ctx))
	if err != nil {
		return nil, err
	}

	versionData := mapConfigVersion(record, true)
	// Handle both int and int64 for version (SQLite vs other databases)
	var version int
	switch v := versionData["version"].(type) {
	case int:
		version = v
	case int64:
		version = int(v)
	case int32:
		version = int(v)
	}

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
		GameID:      svc.ResolveGameID(ctx, req.GameID),
		Env:         svc.ResolveEnv(ctx, req.Env),
		Message:     strings.TrimSpace(req.Message),
		BaseVersion: req.BaseVersion,
	}, configAuthor(ctx))
	if err != nil {
		return nil, err
	}
	return &SaveConfigResponse{Version: record.Version}, nil
}

// ValidateConfig validates the submitted config content according to the declared format.
func (s *Service) ValidateConfig(_ context.Context, _ string, req *ValidateConfigRequest) (*ValidateConfigResponse, error) {
	if req == nil {
		return nil, errors.New("request body cannot be empty")
	}
	if err := validateConfigContent(req.Format, req.Content); err != nil {
		return &ValidateConfigResponse{Valid: false, Errors: []string{err.Error()}}, nil
	}
	return &ValidateConfigResponse{Valid: true, Errors: []string{}}, nil
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

	versions, err := s.svcCtx.ConfigVersionModel.List(ctx, key)
	if err != nil {
		return nil, err
	}

	items := make([]ConfigVersionItem, 0, len(versions))
	for i := range versions {
		versionData := mapConfigVersion(&versions[i], true)
		// Handle both int and int64 for version (SQLite vs other databases)
		var version int
		switch v := versionData["version"].(type) {
		case int:
			version = v
		case int64:
			version = int(v)
		case int32:
			version = int(v)
		}

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

	record, err := s.svcCtx.ConfigVersionModel.Find(ctx, key, req.Version)
	if err != nil {
		return nil, err
	}

	versionData := mapConfigVersion(record, true)
	// Handle both int and int64 for version (SQLite vs other databases)
	var version int
	switch v := versionData["version"].(type) {
	case int:
		version = v
	case int64:
		version = int(v)
	case int32:
		version = int(v)
	}

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
