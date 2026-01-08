package quicksdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/cuihairu/croupier/internal/platform/provider"
)

// Provider implements the provider.Provider interface for QuickSDK.
type Provider struct {
	client  *Client
	config  provider.ProviderConfig
	logger  *slog.Logger
	enabled bool
}

// NewProvider creates a new QuickSDK provider.
func NewProvider(logger *slog.Logger) *Provider {
	if logger == nil {
		logger = slog.Default()
	}
	return &Provider{
		logger:  logger,
		enabled: false,
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "quicksdk"
}

// Init initializes the provider with configuration.
func (p *Provider) Init(ctx context.Context, config provider.ProviderConfig) error {
	// Extract QuickSDK-specific config
	openID, ok := config.Config["open_id"].(string)
	if !ok || openID == "" {
		return fmt.Errorf("open_id is required")
	}

	openKey, ok := config.Config["open_key"].(string)
	if !ok || openKey == "" {
		return fmt.Errorf("open_key is required")
	}

	apiBaseURL, _ := config.Config["api_base_url"].(string)
	timeoutVal, _ := config.Config["timeout"].(float64)
	retryCountVal, _ := config.Config["retry_count"].(float64)
	enableCacheVal, _ := config.Config["enable_cache"].(bool)
	cacheDurationVal, _ := config.Config["cache_duration"].(float64)
	requestsPerMinuteVal, _ := config.Config["requests_per_minute"].(float64)

	// Build client config
	clientConfig := Config{
		OpenID:            openID,
		OpenKey:           openKey,
		APIBaseURL:        apiBaseURL,
		Timeout:           time.Duration(timeoutVal) * time.Second,
		RetryCount:        int(retryCountVal),
		EnableCache:       enableCacheVal,
		CacheDuration:     time.Duration(cacheDurationVal) * time.Second,
		RequestsPerMinute: int(requestsPerMinuteVal),
	}

	// Override rate limit from config if specified
	if config.RateLimit != nil && config.RateLimit.RequestsPerMinute > 0 {
		clientConfig.RequestsPerMinute = config.RateLimit.RequestsPerMinute
	}

	// Create client
	client, err := NewClient(clientConfig, p.logger)
	if err != nil {
		return fmt.Errorf("failed to create QuickSDK client: %w", err)
	}

	p.client = client
	p.config = config
	p.enabled = config.Enabled

	p.logger.Info("QuickSDK provider initialized",
		"enabled", p.enabled,
		"open_id", openID,
		"api_base_url", clientConfig.APIBaseURL)

	return nil
}

// IsEnabled returns whether the provider is enabled.
func (p *Provider) IsEnabled() bool {
	return p.enabled
}

// SupportedMethods returns the list of supported methods.
func (p *Provider) SupportedMethods() []string {
	return []string{
		// Basic data (5)
		"channel_list",
		"server_list",
		"product_list",
		"role_info",
		"order_list",
		// Operation reports (5)
		"day_report",
		"day_hour_report",
		"user_live",
		"channel_days_report",
		"channel_report",
		// Ad management (8)
		"ad_report",
		"media_app_list",
		"ad_plan_group_list",
		"package_version_list",
		"ad_pages_list",
		"create_ad_plan",
		"update_ad_plan",
		"ad_plan_list",
		// Other (2)
		"user_lost_list",
		"push_message",
	}
}

// Call invokes a method on the QuickSDK API.
func (p *Provider) Call(ctx context.Context, method string, request []byte) ([]byte, error) {
	if p.client == nil {
		return nil, fmt.Errorf("QuickSDK client not initialized")
	}

	// Parse request
	var req map[string]interface{}
	if len(request) > 0 {
		if err := json.Unmarshal(request, &req); err != nil {
			return nil, fmt.Errorf("failed to parse request: %w", err)
		}
	}

	// Route to appropriate method
	var result interface{}
	var err error

	switch method {
	// Basic data
	case "channel_list":
		productCode := getString(req, "product_code")
		result, err = p.client.GetChannelList(ctx, productCode)

	case "server_list":
		productCode := getString(req, "product_code")
		result, err = p.client.GetServerList(ctx, productCode)

	case "product_list":
		result, err = p.client.GetProductList(ctx)

	case "role_info":
		productCode := getString(req, "product_code")
		serverName := getString(req, "server_name")
		roleID := getString(req, "role_id")
		roleName := getString(req, "role_name")
		username := getString(req, "username")
		result, err = p.client.GetRoleInfo(ctx, productCode, serverName, roleID, roleName, username)

	case "order_list":
		productCode := getString(req, "product_code")
		opts := OrderListOptions{
			BeginTime:   getString(req, "begin_time"),
			EndTime:     getString(req, "end_time"),
			ChannelList: getString(req, "channel_list"),
			Page:        getInt(req, "page"),
			OrderStatus: getInt(req, "order_status"),
		}
		result, err = p.client.GetOrderList(ctx, productCode, opts)

	// Operation reports
	case "day_report":
		productCode := getString(req, "product_code")
		opts := DayReportOptions{
			ChannelCode: getString(req, "channel_code"),
			BeginTime:   getInt64(req, "begin_time"),
			EndTime:     getInt64(req, "end_time"),
			UseRMB:      getBool(req, "use_rmb"),
		}
		result, err = p.client.GetDayReport(ctx, productCode, opts)

	case "day_hour_report":
		productCode := getString(req, "product_code")
		opts := DayHourReportOptions{
			ChannelCode: getString(req, "channel_code"),
			DateTime:    getInt64(req, "date_time"),
			UseRMB:      getBool(req, "use_rmb"),
		}
		result, err = p.client.GetDayHourReport(ctx, productCode, opts)

	case "user_live":
		productCode := getString(req, "product_code")
		opts := UserLiveOptions{
			ChannelCode: getString(req, "channel_code"),
			BeginTime:   getInt64(req, "begin_time"),
			EndTime:     getInt64(req, "end_time"),
		}
		result, err = p.client.GetUserLive(ctx, productCode, opts)

	case "channel_days_report":
		productCode := getString(req, "product_code")
		opts := ChannelDaysReportOptions{
			ChannelCode: getString(req, "channel_code"),
			BeginTime:   getInt64(req, "begin_time"),
			EndTime:     getInt64(req, "end_time"),
			UseRMB:      getBool(req, "use_rmb"),
		}
		result, err = p.client.GetChannelDaysReport(ctx, productCode, opts)

	case "channel_report":
		productCode := getString(req, "product_code")
		opts := ChannelReportOptions{
			ChannelCode: getString(req, "channel_code"),
			DateTime:    getInt64(req, "date_time"),
			UseRMB:      getBool(req, "use_rmb"),
		}
		result, err = p.client.GetChannelReport(ctx, productCode, opts)

	// Ad management
	case "ad_report":
		productCode := getString(req, "product_code")
		startDate := getString(req, "start_date")
		endDate := getString(req, "end_date")
		plans := getString(req, "plans")
		result, err = p.client.GetAdReport(ctx, productCode, startDate, endDate, plans)

	case "media_app_list":
		mediaType := getString(req, "media_type")
		if mediaType == "" {
			mediaType = "Toutiao"
		}
		result, err = p.client.GetMediaAppList(ctx, mediaType)

	case "ad_plan_group_list":
		productCode := getString(req, "product_code")
		result, err = p.client.GetAdPlanGroupList(ctx, productCode)

	case "package_version_list":
		productCode := getString(req, "product_code")
		result, err = p.client.GetPackageVersionList(ctx, productCode)

	case "ad_pages_list":
		productCode := getString(req, "product_code")
		result, err = p.client.GetAdPagesList(ctx, productCode)

	case "create_ad_plan":
		productCode := getString(req, "product_code")
		opts := CreateAdPlanOptions{
			ProductCode:    productCode,
			Plans:          getString(req, "plans"),
			Platform:       getString(req, "platform"),
			ChannelID:      getString(req, "channel_id"),
			PrivateChannel: getString(req, "private_channel"),
			MediaAppID:     getString(req, "media_app_id"),
			ConvertSDKType: getString(req, "convert_sdk_type"),
			CovertType:     getString(req, "covert_type"),
			GroupID:        getString(req, "group_id"),
			AdPageID:       getString(req, "ad_page_id"),
			Note:           getString(req, "note"),
			ActiveNode:     getString(req, "active_node"),
			URLType:        getString(req, "url_type"),
			GameURL:        getString(req, "game_url"),
			Package:        getString(req, "package"),
			GameVersionID:  getString(req, "game_version_id"),
			CPSList:        getString(req, "cps_list"),
		}
		result, err = p.client.CreateAdPlan(ctx, opts)

	case "update_ad_plan":
		productCode := getString(req, "product_code")
		opts := UpdateAdPlanOptions{
			ProductCode:   productCode,
			Action:        getString(req, "action"),
			Plans:         getString(req, "plans"),
			URLType:       getString(req, "url_type"),
			GameURL:       getString(req, "game_url"),
			GameVersionID: getString(req, "game_version_id"),
			OldVersionID:  getString(req, "old_version_id"),
			NewVersionID:  getString(req, "new_version_id"),
		}
		err = p.client.UpdateAdPlan(ctx, opts)
		// UpdateAdPlan doesn't return data
		if err == nil {
			result = map[string]interface{}{"status": "ok"}
		}

	case "ad_plan_list":
		productCode := getString(req, "product_code")
		opts := AdPlanListOptions{
			Page:      getInt(req, "page"),
			PageRows:  getInt(req, "page_rows"),
			Status:    getString(req, "status"),
			ChannelID: getString(req, "channel_id"),
			AdGroupID: getString(req, "ad_group_id"),
			PlanCode:  getString(req, "plan_code"),
		}
		result, err = p.client.GetAdPlanList(ctx, productCode, opts)

	// Other
	case "user_lost_list":
		productCode := getString(req, "product_code")
		opts := UserLostListOptions{
			ChannelCode: getString(req, "channel_code"),
			DateTime:    getInt64(req, "date_time"),
			Page:        getInt(req, "page"),
			Level:       getInt(req, "level"),
		}
		result, err = p.client.GetUserLostList(ctx, productCode, opts)

	case "push_message":
		productCode := getString(req, "product_code")
		channelCodes := getString(req, "channel_codes")
		gateway := getString(req, "gateway")
		title := getString(req, "title")
		body := getString(req, "body")
		err = p.client.PushMessage(ctx, productCode, channelCodes, gateway, title, body)
		// PushMessage doesn't return data
		if err == nil {
			result = map[string]interface{}{"status": "ok"}
		}

	default:
		return nil, &provider.MethodNotSupportedError{Provider: p.Name(), Method: method}
	}

	if err != nil {
		return nil, err
	}

	// Marshal response
	response, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return response, nil
}

// Close closes the provider.
func (p *Provider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// Helper functions for extracting values from request map

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprint(v)
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case string:
			var i int
			fmt.Sscanf(val, "%d", &i)
			return i
		}
	}
	return 0
}

func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return int64(val)
		case int64:
			return val
		case float64:
			return int64(val)
		case string:
			var i int64
			fmt.Sscanf(val, "%d", &i)
			return i
		}
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		case string:
			return val == "1" || val == "true"
		case float64:
			return val != 0
		}
	}
	return false
}
