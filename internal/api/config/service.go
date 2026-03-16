package config

import (
	"context"
	"errors"
	"strings"

	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
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
			GameID:    versionData["game_id"].(string),
			Env:       versionData["env"].(string),
			Format:    versionData["format"].(string),
			Message:   versionData["message"].(string),
			Value:     versionData["value"].(string),
		},
	}, nil
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
			GameID:    versionData["game_id"].(string),
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
			GameID:    versionData["game_id"].(string),
			Env:       versionData["env"].(string),
			Format:    versionData["format"].(string),
			Message:   versionData["message"].(string),
			Value:     versionData["value"].(string),
		},
	}, nil
}
