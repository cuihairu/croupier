package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
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

	// Config always stores the full WorkspaceConfig JSON snapshot.
	if len(m.Config) > 0 {
		var stored WorkspaceConfig
		if err := json.Unmarshal(m.Config, &stored); err == nil {
			if strings.TrimSpace(stored.ObjectKey) != "" {
				cfg.ObjectKey = stored.ObjectKey
			}
			if strings.TrimSpace(stored.Title) != "" {
				cfg.Title = stored.Title
			}
			cfg.Description = stored.Description
			cfg.Layout = stored.Layout
			cfg.MenuOrder = stored.MenuOrder
			if status := normalizeWorkspaceStatus(stored.Status); status != "" {
				cfg.Status = status
			}
			if stored.Published {
				cfg.Published = true
			}
			if strings.TrimSpace(stored.PublishedBy) != "" {
				cfg.PublishedBy = stored.PublishedBy
			}
			if strings.TrimSpace(stored.PublishedAt) != "" {
				cfg.PublishedAt = stored.PublishedAt
			}
			if strings.TrimSpace(stored.CreatedAt) != "" {
				cfg.CreatedAt = stored.CreatedAt
			}
			if strings.TrimSpace(stored.UpdatedAt) != "" {
				cfg.UpdatedAt = stored.UpdatedAt
			}
			if strings.TrimSpace(stored.Meta.CreatedAt) != "" || strings.TrimSpace(stored.Meta.UpdatedAt) != "" {
				cfg.Meta = stored.Meta
			}
			if stored.Version > 0 {
				cfg.Version = stored.Version
			}
		} else {
			// Backward compatibility for legacy rows that stored layout only.
			var legacyLayout interface{}
			if legacyErr := json.Unmarshal(m.Config, &legacyLayout); legacyErr == nil {
				cfg.Layout = legacyLayout
			}
		}
	}

	if cfg.CreatedAt == "" && !m.CreatedAt.IsZero() {
		cfg.CreatedAt = m.CreatedAt.UTC().Format(time.RFC3339)
	}
	if cfg.UpdatedAt == "" && !m.UpdatedAt.IsZero() {
		cfg.UpdatedAt = m.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if cfg.Meta.CreatedAt == "" && cfg.CreatedAt != "" {
		cfg.Meta.CreatedAt = cfg.CreatedAt
	}
	if cfg.Meta.UpdatedAt == "" && cfg.UpdatedAt != "" {
		cfg.Meta.UpdatedAt = cfg.UpdatedAt
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

	payload, err := json.Marshal(cfg)
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

func decodeWorkspaceSnapshot(raw string) (WorkspaceConfig, bool, error) {
	var cfg WorkspaceConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err == nil {
		if strings.TrimSpace(cfg.ObjectKey) != "" ||
			strings.TrimSpace(cfg.Title) != "" ||
			strings.TrimSpace(cfg.Description) != "" ||
			cfg.Layout != nil ||
			cfg.Published ||
			strings.TrimSpace(cfg.PublishedAt) != "" ||
			strings.TrimSpace(cfg.PublishedBy) != "" ||
			cfg.MenuOrder != 0 ||
			strings.TrimSpace(cfg.Status) != "" ||
			strings.TrimSpace(cfg.CreatedAt) != "" ||
			strings.TrimSpace(cfg.UpdatedAt) != "" ||
			cfg.Version != 0 {
			return cfg, false, nil
		}
	}

	var legacyLayout interface{}
	if err := json.Unmarshal([]byte(raw), &legacyLayout); err != nil {
		return WorkspaceConfig{}, false, err
	}
	return WorkspaceConfig{Layout: legacyLayout}, true, nil
}

func workspacePublishedAt(cfg WorkspaceConfig) *time.Time {
	if strings.TrimSpace(cfg.PublishedAt) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(cfg.PublishedAt))
	if err != nil {
		return nil
	}
	return &parsed
}

func resolveWorkspaceVersionPointers(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	objectKey string,
	records []model.ConfigVersion,
) (currentDraftVersion int, currentPublishedVersion int, err error) {
	if len(records) > 0 {
		currentDraftVersion = records[0].Version
	}
	if svcCtx == nil || svcCtx.WorkspaceConfigModel == nil {
		return currentDraftVersion, 0, nil
	}

	currentCfg, err := svcCtx.WorkspaceConfigModel.FindByObjectKey(ctx, objectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return currentDraftVersion, 0, nil
		}
		return 0, 0, err
	}
	if !currentCfg.Published {
		return currentDraftVersion, 0, nil
	}

	for _, rec := range records {
		if rec.Value == "" {
			continue
		}
		cfg, _, decodeErr := decodeWorkspaceSnapshot(rec.Value)
		if decodeErr != nil {
			continue
		}
		if cfg.Published || normalizeWorkspaceStatus(cfg.Status) == workspaceStatusPublished {
			return currentDraftVersion, rec.Version, nil
		}
	}

	return currentDraftVersion, 0, nil
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
	if err := validateWorkspaceConfig(cfg, true); err != nil {
		return err
	}
	return nil
}

func validateWorkspaceConfig(cfg WorkspaceConfig, publishing bool) error {
	if strings.TrimSpace(cfg.ObjectKey) == "" {
		return badWorkspaceRequest("objectKey is required", nil)
	}
	if strings.TrimSpace(cfg.Title) == "" {
		return badWorkspaceRequest("title is required", nil)
	}
	if cfg.Layout == nil {
		return badWorkspaceRequest("layout is required", nil)
	}

	layoutMap, ok := cfg.Layout.(map[string]interface{})
	if !ok {
		return badWorkspaceRequest("layout must be a JSON object", nil)
	}
	layoutType := strings.TrimSpace(asString(layoutMap["type"]))
	if layoutType == "" {
		return badWorkspaceRequest("layout.type is required", map[string]any{"field": "layout.type"})
	}
	if layoutType != "tabs" {
		return badWorkspaceRequest(
			fmt.Sprintf("layout.type must be tabs, got %s", layoutType),
			map[string]any{"field": "layout.type"},
		)
	}

	tabs, ok := layoutMap["tabs"].([]interface{})
	if !ok || len(tabs) == 0 {
		return badWorkspaceRequest("layout.tabs must contain at least one tab", map[string]any{"field": "layout.tabs"})
	}

	supportedTabLayouts := map[string]struct{}{
		"list":        {},
		"form":        {},
		"detail":      {},
		"form-detail": {},
		"kanban":      {},
		"timeline":    {},
		"split":       {},
		"wizard":      {},
		"dashboard":   {},
		"grid":        {},
		"custom":      {},
		"single":      {},
	}

	tabKeys := make(map[string]struct{}, len(tabs))
	for index, rawTab := range tabs {
		tab, ok := rawTab.(map[string]interface{})
		if !ok {
			return badWorkspaceRequest(
				fmt.Sprintf("layout.tabs[%d] must be a JSON object", index),
				map[string]any{"field": fmt.Sprintf("layout.tabs[%d]", index)},
			)
		}

		tabKey := strings.TrimSpace(asString(tab["key"]))
		if tabKey == "" {
			return badWorkspaceRequest(
				fmt.Sprintf("layout.tabs[%d].key is required", index),
				map[string]any{"field": fmt.Sprintf("layout.tabs[%d].key", index)},
			)
		}
		if _, exists := tabKeys[tabKey]; exists {
			return badWorkspaceRequest(
				fmt.Sprintf("layout.tabs[%d].key duplicates %q", index, tabKey),
				map[string]any{"field": fmt.Sprintf("layout.tabs[%d].key", index)},
			)
		}
		tabKeys[tabKey] = struct{}{}

		if strings.TrimSpace(asString(tab["title"])) == "" {
			return badWorkspaceRequest(
				fmt.Sprintf("layout.tabs[%d].title is required", index),
				map[string]any{"field": fmt.Sprintf("layout.tabs[%d].title", index)},
			)
		}

		rawTabLayout, ok := tab["layout"].(map[string]interface{})
		if !ok {
			return badWorkspaceRequest(
				fmt.Sprintf("layout.tabs[%d].layout must be a JSON object", index),
				map[string]any{"field": fmt.Sprintf("layout.tabs[%d].layout", index)},
			)
		}
		tabLayoutType := strings.TrimSpace(asString(rawTabLayout["type"]))
		if tabLayoutType == "" {
			return badWorkspaceRequest(
				fmt.Sprintf("layout.tabs[%d].layout.type is required", index),
				map[string]any{"field": fmt.Sprintf("layout.tabs[%d].layout.type", index)},
			)
		}
		if _, supported := supportedTabLayouts[tabLayoutType]; !supported {
			return badWorkspaceRequest(
				fmt.Sprintf("layout.tabs[%d].layout.type %q is not supported", index, tabLayoutType),
				map[string]any{"field": fmt.Sprintf("layout.tabs[%d].layout.type", index)},
			)
		}

		if err := validateTabLayout(index, tabLayoutType, rawTabLayout, publishing); err != nil {
			return err
		}
	}

	return nil
}

func validateTabLayout(index int, layoutType string, layout map[string]interface{}, publishing bool) error {
	fieldPrefix := fmt.Sprintf("layout.tabs[%d].layout", index)
	switch layoutType {
	case "list":
		if strings.TrimSpace(asString(layout["listFunction"])) == "" {
			return badWorkspaceRequest(
				fmt.Sprintf("%s.listFunction is required", fieldPrefix),
				map[string]any{"field": fieldPrefix + ".listFunction"},
			)
		}
		if publishing && len(asObjectSlice(layout["columns"])) == 0 {
			return badWorkspaceRequest(
				fmt.Sprintf("%s.columns must contain at least one item", fieldPrefix),
				map[string]any{"field": fieldPrefix + ".columns"},
			)
		}
	case "form":
		if strings.TrimSpace(asString(layout["submitFunction"])) == "" {
			return badWorkspaceRequest(
				fmt.Sprintf("%s.submitFunction is required", fieldPrefix),
				map[string]any{"field": fieldPrefix + ".submitFunction"},
			)
		}
		if publishing && len(asObjectSlice(layout["fields"])) == 0 {
			return badWorkspaceRequest(
				fmt.Sprintf("%s.fields must contain at least one item", fieldPrefix),
				map[string]any{"field": fieldPrefix + ".fields"},
			)
		}
	case "detail":
		if strings.TrimSpace(asString(layout["detailFunction"])) == "" &&
			strings.TrimSpace(asString(layout["dataFunction"])) == "" {
			return badWorkspaceRequest(
				fmt.Sprintf("%s.detailFunction or %s.dataFunction is required", fieldPrefix, fieldPrefix),
				map[string]any{"field": fieldPrefix},
			)
		}
	case "form-detail":
		if strings.TrimSpace(asString(layout["queryFunction"])) == "" {
			return badWorkspaceRequest(
				fmt.Sprintf("%s.queryFunction is required", fieldPrefix),
				map[string]any{"field": fieldPrefix + ".queryFunction"},
			)
		}
		if publishing && len(asObjectSlice(layout["queryFields"])) == 0 {
			return badWorkspaceRequest(
				fmt.Sprintf("%s.queryFields must contain at least one item", fieldPrefix),
				map[string]any{"field": fieldPrefix + ".queryFields"},
			)
		}
	}
	return nil
}

func badWorkspaceRequest(message string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	details["code"] = "workspace_invalid_config"
	return errorx.NewBadRequestWithDetails(message, details)
}

func asString(value interface{}) string {
	text, _ := value.(string)
	return text
}

func asObjectSlice(value interface{}) []interface{} {
	items, _ := value.([]interface{})
	return items
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
