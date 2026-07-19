package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/gorm"
)

const (
	workspaceStatusDraft     = "draft"
	workspaceStatusPublished = "published"
	workspaceStatusArchived  = "archived"
)

func normalizeWorkspaceStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case workspaceStatusDraft:
		return workspaceStatusDraft
	case workspaceStatusPublished:
		return workspaceStatusPublished
	case workspaceStatusArchived:
		return workspaceStatusArchived
	default:
		return ""
	}
}

func resolveWorkspaceStatus(config *WorkspaceConfig) string {
	if config == nil {
		return workspaceStatusDraft
	}
	if config.Published {
		return workspaceStatusPublished
	}
	switch normalizeWorkspaceStatus(config.Status) {
	case workspaceStatusArchived:
		return workspaceStatusArchived
	case workspaceStatusDraft:
		return workspaceStatusDraft
	default:
		return workspaceStatusDraft
	}
}

func workspaceVersionKey(objectKey string) string {
	return fmt.Sprintf("workspace:%s", strings.TrimSpace(objectKey))
}

func workspaceActorFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if username, ok := ctx.Value("username").(string); ok {
		return strings.TrimSpace(username)
	}
	return ""
}

func toDTO(m *model.WorkspaceConfig) WorkspaceConfig {
	cfg := WorkspaceConfig{
		ObjectKey:   m.ObjectKey,
		Title:       m.Title,
		Published:   m.Published,
		MenuOrder:   int(m.MenuOrder),
		Status:      "draft",
		PublishedBy: m.PublishedBy,
	}

	if m.PublishedAt != nil {
		cfg.PublishedAt = m.PublishedAt.Format(time.RFC3339)
	}

	// Unmarshal the JSON config - 支持两种格式：
	// 1. 完整的 WorkspaceConfig JSON（包含 layout, description, status 等）
	// 2. 仅 Layout JSON（向后兼容，由 createDefaultConfig 创建的旧记录）
	if m.Config != "" {
		// 先检查 JSON 是否包含 "layout" 键（完整 config 格式）
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(m.Config), &raw); err == nil {
			if layoutRaw, ok := raw["layout"]; ok {
				// 完整 config 格式：提取 layout 字段
				var layout interface{}
				if err := json.Unmarshal(layoutRaw, &layout); err == nil {
					cfg.Layout = layout
				}
				// 提取其他可选字段
				if descRaw, ok := raw["description"]; ok {
					var desc string
					if err := json.Unmarshal(descRaw, &desc); err == nil && desc != "" {
						cfg.Description = desc
					}
				}
				if statusRaw, ok := raw["status"]; ok {
					var status string
					if err := json.Unmarshal(statusRaw, &status); err == nil && status != "" {
						cfg.Status = status
					}
				}
				if categoryRaw, ok := raw["category"]; ok {
					var category string
					if err := json.Unmarshal(categoryRaw, &category); err == nil && category != "" {
						cfg.Category = category
					}
				}
				if permsRaw, ok := raw["permissions"]; ok {
					var perms WorkspacePermissions
					if err := json.Unmarshal(permsRaw, &perms); err == nil {
						cfg.Permissions = &perms
					}
				}
				if metaRaw, ok := raw["meta"]; ok {
					var meta WorkspaceConfigMeta
					if err := json.Unmarshal(metaRaw, &meta); err == nil {
						if meta.CreatedAt != "" {
							cfg.CreatedAt = meta.CreatedAt
						}
						if meta.UpdatedAt != "" {
							cfg.UpdatedAt = meta.UpdatedAt
						}
					}
				}
			} else {
				// 向后兼容：整个 JSON 就是 layout（createDefaultConfig 旧格式）
				var layout interface{}
				if err := json.Unmarshal([]byte(m.Config), &layout); err == nil {
					cfg.Layout = layout
				}
			}
		}
	}

	if m.Published {
		cfg.Status = "published"
	}

	return cfg
}

func persistWorkspaceVersion(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	cfg WorkspaceConfig,
	actor string,
	comment string,
) (int, error) {
	if svcCtx == nil || svcCtx.ConfigVersionModel == nil {
		return 0, nil
	}

	// Convert WorkspaceConfig to WorkspaceConfig for versioning
	typesCfg := WorkspaceConfig{
		ObjectKey:   cfg.ObjectKey,
		Title:       cfg.Title,
		Layout:      cfg.Layout,
		Published:   cfg.Published,
		MenuOrder:   cfg.MenuOrder,
		Status:      cfg.Status,
		PublishedBy: cfg.PublishedBy,
		PublishedAt: cfg.PublishedAt,
		Meta: WorkspaceConfigMeta{
			CreatedAt: cfg.CreatedAt,
			UpdatedAt: cfg.UpdatedAt,
		},
		Version: cfg.Version,
	}

	payload, err := json.Marshal(typesCfg)
	if err != nil {
		return 0, err
	}
	record, err := svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:     workspaceVersionKey(cfg.ObjectKey),
		Content: string(payload),
		Format:  "json",
		Message: comment,
	}, strings.TrimSpace(actor))
	if err != nil {
		return 0, err
	}
	return record.Version, nil
}

func enrichWorkspaceVersion(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	config *WorkspaceConfig,
) error {
	if config == nil || svcCtx == nil || svcCtx.ConfigVersionModel == nil {
		return nil
	}
	latest, err := svcCtx.ConfigVersionModel.FindLatest(ctx, workspaceVersionKey(config.ObjectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			config.Version = 0
			return nil
		}
		return err
	}
	config.Version = latest.Version
	return nil
}

func validateWorkspaceForPublish(cfg WorkspaceConfig) error {
	if strings.TrimSpace(cfg.Title) == "" {
		return errors.New("title is required for publishing")
	}
	if cfg.Layout == nil {
		return errors.New("layout is required for publishing")
	}
	return nil
}

func parseWorkspaceVersionTimeRange(from, to string) (fromAt, toAt time.Time, err error) {
	if from = strings.TrimSpace(from); from != "" {
		fromAt, err = time.Parse(time.RFC3339, from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'from' time format: %w", err)
		}
	}
	if to = strings.TrimSpace(to); to != "" {
		toAt, err = time.Parse(time.RFC3339, to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid 'to' time format: %w", err)
		}
	}
	return fromAt, toAt, nil
}
