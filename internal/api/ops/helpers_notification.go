package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

const officialNotificationID = "official.notification"

const (
	notificationEnabledKey  = "enabled"
	notificationChannelsKey = "channels"
	notificationRulesKey    = "rules"
)

// Agent operations implementations

func opsNotificationsGet(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNotificationsGetRequest) (*OpsNotificationsGetResponse, error) {
	if enabled, channels, rules, ok, err := loadNotificationsFromExtensionInstallation(ctx, svcCtx); err != nil {
		return nil, err
	} else if ok {
		return &OpsNotificationsGetResponse{
			Enabled:  enabled,
			Channels: channels,
			Rules:    rules,
		}, nil
	}
	// No active notification installation - return defaults.
	return &OpsNotificationsGetResponse{
		Enabled:  false,
		Channels: []OpsNotificationChannel{},
		Rules:    []OpsNotificationRule{},
	}, nil
}

func opsNotificationsUpdate(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNotificationsUpdateRequest) (*OpsNotificationsUpdateResponse, error) {
	if req == nil {
		req = &OpsNotificationsUpdateRequest{}
	}
	if err := saveNotificationsToExtensionInstallation(ctx, svcCtx, req); err != nil {
		return nil, err
	}
	if err := recordNotificationEvent(ctx, svcCtx, "notifications_update", "notifications updated",
		fmt.Sprintf(`{"enabled":%t,"channels":%d,"rules":%d}`, req.Enabled, len(req.Channels), len(req.Rules)),
	); err != nil {
		// Persistence already succeeded; only the audit event failed. Log-style
		// swallow is acceptable here, but surface it so callers know the audit
		// trail is incomplete.
		return nil, err
	}
	return &OpsNotificationsUpdateResponse{}, nil
}

func findActiveExtensionInstallationByID(ctx context.Context, svcCtx *svc.ServiceContext, extensionID string) (*model.ExtensionInstallation, bool, error) {
	if svcCtx == nil || svcCtx.Extensions == nil || svcCtx.Extensions.Installation == nil {
		return nil, false, nil
	}
	items, _, err := svcCtx.Extensions.Installation.List(ctx, extensioninstallation.ListQuery{
		ExtensionID: strings.TrimSpace(extensionID),
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		return nil, false, err
	}
	for i := range items {
		item := items[i]
		if strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") ||
			strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
			continue
		}
		return &item, true, nil
	}
	return nil, false, nil
}

func recordExtensionEvent(ctx context.Context, svcCtx *svc.ServiceContext, extensionID, eventType, message, payload string) error {
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx, extensionID)
	if err != nil || !ok || item == nil {
		return err
	}
	operator := "system"
	if username, userErr := utils.CurrentUsername(ctx); userErr == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return svcCtx.Extensions.Installation.RecordEvent(ctx, item.ID, eventType, "info", message, operator, payload)
}

func recordNotificationEvent(ctx context.Context, svcCtx *svc.ServiceContext, eventType, message, payload string) error {
	return recordExtensionEvent(ctx, svcCtx, officialNotificationID, eventType, message, payload)
}

func loadNotificationsFromExtensionInstallation(ctx context.Context, svcCtx *svc.ServiceContext) (bool, []OpsNotificationChannel, []OpsNotificationRule, bool, error) {
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx, officialNotificationID)
	if err != nil || !ok || item == nil {
		return false, nil, nil, false, err
	}
	config := map[string]any{}
	if len(item.ConfigJSON) > 0 {
		if err := json.Unmarshal(item.ConfigJSON, &config); err != nil {
			return false, nil, nil, false, err
		}
	}
	enabled, channels, rules, extracted, err := extractNotificationConfig(config)
	if err != nil {
		return false, nil, nil, false, err
	}
	if !extracted {
		return false, []OpsNotificationChannel{}, []OpsNotificationRule{}, true, nil
	}
	return enabled, channels, rules, true, nil
}

func saveNotificationsToExtensionInstallation(ctx context.Context, svcCtx *svc.ServiceContext, req *OpsNotificationsUpdateRequest) error {
	item, ok, err := findActiveExtensionInstallationByID(ctx, svcCtx, officialNotificationID)
	if err != nil || !ok || item == nil {
		return err
	}
	config := map[string]any{}
	if len(item.ConfigJSON) > 0 {
		_ = json.Unmarshal(item.ConfigJSON, &config)
	}
	config[notificationEnabledKey] = req.Enabled
	config[notificationChannelsKey] = req.Channels
	config[notificationRulesKey] = req.Rules
	secretRefs := map[string]string{}
	if len(item.SecretRefsJSON) > 0 {
		_ = json.Unmarshal(item.SecretRefsJSON, &secretRefs)
	}
	operator := "system"
	if username, userErr := utils.CurrentUsername(ctx); userErr == nil && strings.TrimSpace(username) != "" {
		operator = strings.TrimSpace(username)
	}
	return svcCtx.Extensions.Installation.UpdateConfig(ctx, item.ID, config, secretRefs, operator)
}

func extractNotificationConfig(config map[string]any) (bool, []OpsNotificationChannel, []OpsNotificationRule, bool, error) {
	if config == nil {
		return false, nil, nil, false, nil
	}
	rawEnabled, hasEnabled := config[notificationEnabledKey]
	rawChannels, hasChannels := config[notificationChannelsKey]
	rawRules, hasRules := config[notificationRulesKey]
	if !hasEnabled && !hasChannels && !hasRules {
		return false, nil, nil, false, nil
	}
	enabled := false
	if hasEnabled {
		data, err := json.Marshal(rawEnabled)
		if err != nil {
			return false, nil, nil, false, err
		}
		_ = json.Unmarshal(data, &enabled)
	}
	channels := []OpsNotificationChannel{}
	if hasChannels {
		data, err := json.Marshal(rawChannels)
		if err != nil {
			return false, nil, nil, false, err
		}
		if err := json.Unmarshal(data, &channels); err != nil {
			return false, nil, nil, false, err
		}
	}
	rules := []OpsNotificationRule{}
	if hasRules {
		data, err := json.Marshal(rawRules)
		if err != nil {
			return false, nil, nil, false, err
		}
		if err := json.Unmarshal(data, &rules); err != nil {
			return false, nil, nil, false, err
		}
	}
	return enabled, channels, rules, true, nil
}
