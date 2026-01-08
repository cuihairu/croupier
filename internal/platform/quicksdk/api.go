package quicksdk

import (
	"context"
	"encoding/json"
	"fmt"
)

// API methods for QuickSDK.
// These are convenience wrappers around the Client.Do method.

// Channel represents a channel (platform) where the game is published.
type Channel struct {
	ChannelName string `json:"channelName"`
	ChannelCode string `json:"channelCode"`
	MarketOS    string `json:"marketOs"` // "1" for Android, "2" for iOS
}

// Server represents a game server/region.
type Server struct {
	ServerName string `json:"serverName"`
}

// Product represents a game product.
type Product struct {
	ProductName string `json:"productName"`
	ProductCode string `json:"productCode"`
	ProductKey  string `json:"productKey"`
	CallbackKey string `json:"callbackKey"`
	Md5Key      string `json:"md5Key"`
	GameType    int    `json:"gameType"`
	CallbackURL string `json:"callbackUrl"`
}

// Role represents a game role/character.
type Role struct {
	UID             string `json:"uid"`
	Username        string `json:"username"`
	ChannelCode     string `json:"channelCode"`
	ServerName      string `json:"serverName"`
	RoleName        string `json:"roleName"`
	RoleID          string `json:"roleId"`
	RoleBalance     string `json:"roleBalance"`
	RoleLevel       string `json:"roleLevel"`
	VIPLevel        string `json:"vipLevel"`
	Guild           string `json:"guild"`
	LastLoginTime   string `json:"lastLoginTime"`
	CreateTime      string `json:"createTime"`
	LoginTimes      string `json:"loginTimes"`
	LastLoginDevice string `json:"lastLoginDevice"`
	PayTimes        string `json:"payTimes"`
	PayAmount       string `json:"payAmount"`
	IP              string `json:"ip"`
}

// Order represents a game order.
type Order struct {
	MarketOS       int     `json:"marketOs"`
	ChannelName    string  `json:"channelName"`
	Server         string  `json:"server"`
	OrderNo        string  `json:"orderNo"`
	UserName       string  `json:"userName"`
	UserID         string  `json:"userId"`
	RoleName       string  `json:"roleName"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	CreateTime     int64   `json:"createTime"`
	Status         int     `json:"status"`
	ChannelOrderNo string  `json:"channelOrderNo"`
	CPOrderNo      string  `json:"cpOrderNo"`
	ExtrasParams   string  `json:"extrasParams"`
	PayTime        int64   `json:"payTime"`
	AsyTime        int64   `json:"asyTime"`
	GoodsID        string  `json:"goodsId"`
	GoodsName      string  `json:"goodsName"`
	Price          float64 `json:"price"`
	ChannelCode    int     `json:"channelCode"`
}

// DayReport represents daily operation report data.
type DayReport struct {
	Date         string  `json:"date"`
	ChannelName  string  `json:"channelName"`
	NewUser      int     `json:"newUser"`
	NewDevice    int     `json:"newDevice"`
	UserLogin    int     `json:"userLogin"`
	NewPayUser   int     `json:"newPayUser"`
	AllPayUser   int     `json:"allPayUser"`
	NewUserPay   float64 `json:"newUserPay"`
	AllPay       float64 `json:"allPay"`
	FirstPayUser int     `json:"firstPayUser"`
	Currency     string  `json:"currency"`
	UserMB       int     `json:"usermb"`
}

// DayHourReport represents hourly operation report data.
type DayHourReport struct {
	DateHour     string  `json:"date_hour"`
	ChannelName  string  `json:"channelName"`
	NewUser      int     `json:"newUser"`
	NewDevice    int     `json:"newDevice"`
	UserLogin    int     `json:"userLogin"`
	NewPayUser   int     `json:"newPayUser"`
	AllPayUser   int     `json:"allPayUser"`
	NewUserPay   float64 `json:"newUserPay"`
	AllPay       float64 `json:"allPay"`
	FirstPayUser int     `json:"firstPayUser"`
	Currency     string  `json:"currency"`
	UserMB       int     `json:"usermb"`
}

// UserLive represents user retention data.
type UserLive struct {
	Date        string `json:"date"`
	ChannelName string `json:"channelName"`
	NewUser     int    `json:"newUser"`
	Live2       int    `json:"live_2"`
	Live3       int    `json:"live_3"`
	Live7       int    `json:"live_7"`
	Live15      int    `json:"live_15"`
}

// ChannelDaysReport represents multi-day channel report.
type ChannelDaysReport struct {
	ChannelName string  `json:"channelName"`
	NewUser     int     `json:"newUser"`
	NewDevice   int     `json:"newDevice"`
	AllPayUser  int     `json:"allPayUser"`
	AllPay      float64 `json:"allPay"`
	Currency    string  `json:"currency"`
}

// ChannelReport represents daily channel report.
type ChannelReport struct {
	ChannelName  string  `json:"channelName"`
	NewUser      int     `json:"newUser"`
	NewDevice    int     `json:"newDevice"`
	UserLogin    int     `json:"userLogin"`
	NewPayUser   int     `json:"newPayUser"`
	AllPayUser   int     `json:"allPayUser"`
	NewUserPay   float64 `json:"newUserPay"`
	AllPay       float64 `json:"allPay"`
	FirstPayUser int     `json:"firstPayUser"`
	Currency     string  `json:"currency"`
	UserMB       int     `json:"usermb"`
}

// AdReport represents advertising performance report.
type AdReport struct {
	Date             string  `json:"date"`
	PlanCode         string  `json:"planCode"`
	InstallNum       int     `json:"installNum"`
	NewUser          int     `json:"newUser"`
	DAU              int     `json:"dau"`
	PayNewUser       int     `json:"payNewUser"`
	PayNewUserAmount float64 `json:"payNewUserAmount"`
	Pay3             float64 `json:"pay_3"`
	Pay7             float64 `json:"pay_7"`
	Pay15            float64 `json:"pay_15"`
	Pay30            float64 `json:"pay_30"`
	Pay60            float64 `json:"pay_60"`
	PayUser          int     `json:"payUser"`
	PayAmount        float64 `json:"payAmount"`
	AdChannel        string  `json:"adChannel"`
	AdMediaCost      float64 `json:"adMedia_cost"`
	LiveN            int     `json:"live_N"`
	LTVN             float64 `json:"ltvN"`
}

// MediaApp represents an advertising media app.
type MediaApp struct {
	AppID     string `json:"appId"`
	MediaID   string `json:"mediaId"`
	MediaName string `json:"mediaName"`
}

// AdPlanGroup represents an ad plan group.
type AdPlanGroup struct {
	GroupID   string `json:"groupId"`
	GroupName string `json:"groupName"`
}

// PackageVersion represents a game package version.
type PackageVersion struct {
	PackID      string `json:"packId"`
	PackName    string `json:"packName"`
	VersionNo   string `json:"versionNo"`
	VersionName string `json:"versionName"`
}

// AdPage represents an ad landing page.
type AdPage struct {
	PageID   string `json:"pageId"`
	PageName string `json:"pageName"`
	TempID   string `json:"tempId"`
	TempName string `json:"tempName"`
}

// AdPlan represents an ad plan.
type AdPlan struct {
	PlanName   string `json:"planName"`
	PlanCode   string `json:"planCode"`
	GroupID    int    `json:"groupId"`
	ChannelID  int    `json:"channelId"`
	Stat       string `json:"stat"`
	Message    string `json:"message"`
	CreateTime int64  `json:"createTime"`
	AdPageURL  string `json:"adPageUrl"`
	DownURL    string `json:"downUrl"`
}

// UserLost represents user at-risk (churn warning) data.
type UserLost struct {
	ChannelName   string  `json:"channelName"`
	UserID        string  `json:"userId"`
	PayAmount     float64 `json:"payAmount"`
	LastLoginTime int64   `json:"lastLoginTime"`
	OnlineWarn    string  `json:"onlineWarn"`
	PayWarn       string  `json:"payWarn"`
	WarnLevel     string  `json:"warnLevel"`
}

// API methods

// GetChannelList retrieves all channels for a product.
func (c *Client) GetChannelList(ctx context.Context, productCode string) ([]Channel, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}
	resp, err := c.Do(ctx, "open/channelList", params)
	if err != nil {
		return nil, err
	}

	var channels []Channel
	if err := json.Unmarshal(resp.Data, &channels); err != nil {
		return nil, fmt.Errorf("failed to parse channels: %w", err)
	}
	return channels, nil
}

// GetServerList retrieves all servers for a product.
func (c *Client) GetServerList(ctx context.Context, productCode string) ([]Server, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}
	resp, err := c.Do(ctx, "open/serverList", params)
	if err != nil {
		return nil, err
	}

	var servers []Server
	if err := json.Unmarshal(resp.Data, &servers); err != nil {
		return nil, fmt.Errorf("failed to parse servers: %w", err)
	}
	return servers, nil
}

// GetProductList retrieves all products for the account.
func (c *Client) GetProductList(ctx context.Context) ([]Product, error) {
	params := map[string]interface{}{}
	resp, err := c.Do(ctx, "open/productList", params)
	if err != nil {
		return nil, err
	}

	var products []Product
	if err := json.Unmarshal(resp.Data, &products); err != nil {
		return nil, fmt.Errorf("failed to parse products: %w", err)
	}
	return products, nil
}

// GetRoleInfo retrieves role information.
func (c *Client) GetRoleInfo(ctx context.Context, productCode, serverName string, roleID, roleName, username string) ([]Role, error) {
	params := map[string]interface{}{
		"productCode": productCode,
		"serverName":  serverName,
	}
	if roleID != "" {
		params["roleId"] = roleID
	}
	if roleName != "" {
		params["roleName"] = roleName
	}
	if username != "" {
		params["username"] = username
	}

	resp, err := c.Do(ctx, "open/roleInfo", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Total int    `json:"total"`
		List  []Role `json:"list"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse roles: %w", err)
	}
	return result.List, nil
}

// GetOrderList retrieves orders with optional filtering.
func (c *Client) GetOrderList(ctx context.Context, productCode string, opts OrderListOptions) ([]Order, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}
	if opts.BeginTime != "" {
		params["btime"] = opts.BeginTime
	}
	if opts.EndTime != "" {
		params["etime"] = opts.EndTime
	}
	if opts.ChannelList != "" {
		params["channelList"] = opts.ChannelList
	}
	if opts.Page > 0 {
		params["page"] = opts.Page
	}
	if opts.OrderStatus > 0 {
		params["orderStatus"] = opts.OrderStatus
	}

	resp, err := c.Do(ctx, "open/orderList", params)
	if err != nil {
		return nil, err
	}

	var orders []Order
	if err := json.Unmarshal(resp.Data, &orders); err != nil {
		return nil, fmt.Errorf("failed to parse orders: %w", err)
	}
	return orders, nil
}

// OrderListOptions holds options for GetOrderList.
type OrderListOptions struct {
	BeginTime   string // Format: "YYYY-MM-DD HH:MM:SS"
	EndTime     string // Format: "YYYY-MM-DD HH:MM:SS"
	ChannelList string // Comma-separated channel codes
	Page        int
	OrderStatus int // 1:failed, 2:not delivered, 3:delivery failed, 4:completed
}

// GetDayReport retrieves daily report data.
func (c *Client) GetDayReport(ctx context.Context, productCode string, opts DayReportOptions) ([]DayReport, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}
	if opts.ChannelCode != "" {
		params["channelCode"] = opts.ChannelCode
	}
	if opts.BeginTime > 0 {
		params["bTime"] = opts.BeginTime
	}
	if opts.EndTime > 0 {
		params["eTime"] = opts.EndTime
	}
	if opts.UseRMB {
		params["usermb"] = 1
	}

	resp, err := c.Do(ctx, "open/dayReport", params)
	if err != nil {
		return nil, err
	}

	var reports []DayReport
	if err := json.Unmarshal(resp.Data, &reports); err != nil {
		return nil, fmt.Errorf("failed to parse day reports: %w", err)
	}
	return reports, nil
}

// DayReportOptions holds options for GetDayReport.
type DayReportOptions struct {
	ChannelCode string
	BeginTime   int64 // Unix timestamp
	EndTime     int64 // Unix timestamp, max 30 days from begin
	UseRMB      bool  // If true, return amounts in RMB
}

// GetDayHourReport retrieves hourly report data for a specific day.
func (c *Client) GetDayHourReport(ctx context.Context, productCode string, opts DayHourReportOptions) ([]DayHourReport, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}
	if opts.ChannelCode != "" {
		params["channelCode"] = opts.ChannelCode
	}
	if opts.DateTime > 0 {
		params["dateTime"] = opts.DateTime
	}
	if opts.UseRMB {
		params["usermb"] = 1
	}

	resp, err := c.Do(ctx, "open/dayHourReport", params)
	if err != nil {
		return nil, err
	}

	var reports []DayHourReport
	if err := json.Unmarshal(resp.Data, &reports); err != nil {
		return nil, fmt.Errorf("failed to parse hour reports: %w", err)
	}
	return reports, nil
}

// DayHourReportOptions holds options for GetDayHourReport.
type DayHourReportOptions struct {
	ChannelCode string
	DateTime    int64 // Unix timestamp, defaults to today
	UseRMB      bool
}

// GetUserLive retrieves user retention data.
func (c *Client) GetUserLive(ctx context.Context, productCode string, opts UserLiveOptions) ([]UserLive, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}
	if opts.ChannelCode != "" {
		params["channelCode"] = opts.ChannelCode
	}
	if opts.BeginTime > 0 {
		params["bTime"] = opts.BeginTime
	}
	if opts.EndTime > 0 {
		params["eTime"] = opts.EndTime
	}

	resp, err := c.Do(ctx, "open/userLive", params)
	if err != nil {
		return nil, err
	}

	var reports []UserLive
	if err := json.Unmarshal(resp.Data, &reports); err != nil {
		return nil, fmt.Errorf("failed to parse user live: %w", err)
	}
	return reports, nil
}

// UserLiveOptions holds options for GetUserLive.
type UserLiveOptions struct {
	ChannelCode string
	BeginTime   int64 // Unix timestamp
	EndTime     int64 // Unix timestamp, max 30 days from begin
}

// GetChannelDaysReport retrieves multi-day channel report.
func (c *Client) GetChannelDaysReport(ctx context.Context, productCode string, opts ChannelDaysReportOptions) ([]ChannelDaysReport, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}
	if opts.ChannelCode != "" {
		params["channelCode"] = opts.ChannelCode
	}
	if opts.BeginTime > 0 {
		params["bTime"] = opts.BeginTime
	}
	if opts.EndTime > 0 {
		params["eTime"] = opts.EndTime
	}
	if opts.UseRMB {
		params["usermb"] = 1
	}

	resp, err := c.Do(ctx, "open/channelDaysReport", params)
	if err != nil {
		return nil, err
	}

	var reports []ChannelDaysReport
	if err := json.Unmarshal(resp.Data, &reports); err != nil {
		return nil, fmt.Errorf("failed to parse channel days report: %w", err)
	}
	return reports, nil
}

// ChannelDaysReportOptions holds options for GetChannelDaysReport.
type ChannelDaysReportOptions struct {
	ChannelCode string
	BeginTime   int64
	EndTime     int64
	UseRMB      bool
}

// GetChannelReport retrieves daily channel report.
func (c *Client) GetChannelReport(ctx context.Context, productCode string, opts ChannelReportOptions) ([]ChannelReport, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}
	if opts.ChannelCode != "" {
		params["channelCode"] = opts.ChannelCode
	}
	if opts.DateTime > 0 {
		params["dateTime"] = opts.DateTime
	}
	if opts.UseRMB {
		params["usermb"] = 1
	}

	resp, err := c.Do(ctx, "open/channelReport", params)
	if err != nil {
		return nil, err
	}

	var reports []ChannelReport
	if err := json.Unmarshal(resp.Data, &reports); err != nil {
		return nil, fmt.Errorf("failed to parse channel report: %w", err)
	}
	return reports, nil
}

// ChannelReportOptions holds options for GetChannelReport.
type ChannelReportOptions struct {
	ChannelCode string
	DateTime    int64
	UseRMB      bool
}

// GetAdReport retrieves advertising performance report.
func (c *Client) GetAdReport(ctx context.Context, productCode, startDate, endDate string, plans string) ([]AdReport, error) {
	params := map[string]interface{}{
		"productCode": productCode,
		"sdate":       startDate,
		"edate":       endDate,
	}
	if plans != "" {
		params["plans"] = plans
	}

	resp, err := c.Do(ctx, "open/adReport", params)
	if err != nil {
		return nil, err
	}

	var reports []AdReport
	if err := json.Unmarshal(resp.Data, &reports); err != nil {
		return nil, fmt.Errorf("failed to parse ad report: %w", err)
	}
	return reports, nil
}

// GetMediaAppList retrieves advertising media apps.
func (c *Client) GetMediaAppList(ctx context.Context, mediaType string) ([]MediaApp, error) {
	params := map[string]interface{}{
		"mediaType": mediaType, // e.g., "Toutiao"
	}

	resp, err := c.Do(ctx, "open/getMediaApp", params)
	if err != nil {
		return nil, err
	}

	var apps []MediaApp
	if err := json.Unmarshal(resp.Data, &apps); err != nil {
		return nil, fmt.Errorf("failed to parse media apps: %w", err)
	}
	return apps, nil
}

// GetAdPlanGroupList retrieves ad plan groups.
func (c *Client) GetAdPlanGroupList(ctx context.Context, productCode string) ([]AdPlanGroup, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}

	resp, err := c.Do(ctx, "open/getAdPlanGroup", params)
	if err != nil {
		return nil, err
	}

	var groups []AdPlanGroup
	if err := json.Unmarshal(resp.Data, &groups); err != nil {
		return nil, fmt.Errorf("failed to parse ad plan groups: %w", err)
	}
	return groups, nil
}

// GetPackageVersionList retrieves game package versions.
func (c *Client) GetPackageVersionList(ctx context.Context, productCode string) ([]PackageVersion, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}

	resp, err := c.Do(ctx, "open/getPackageVersion", params)
	if err != nil {
		return nil, err
	}

	var versions []PackageVersion
	if err := json.Unmarshal(resp.Data, &versions); err != nil {
		return nil, fmt.Errorf("failed to parse package versions: %w", err)
	}
	return versions, nil
}

// GetAdPagesList retrieves ad landing pages.
func (c *Client) GetAdPagesList(ctx context.Context, productCode string) ([]AdPage, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}

	resp, err := c.Do(ctx, "open/getAdPages", params)
	if err != nil {
		return nil, err
	}

	var pages []AdPage
	if err := json.Unmarshal(resp.Data, &pages); err != nil {
		return nil, fmt.Errorf("failed to parse ad pages: %w", err)
	}
	return pages, nil
}

// CreateAdPlan creates one or more ad plans.
func (c *Client) CreateAdPlan(ctx context.Context, opts CreateAdPlanOptions) (*CreateAdPlanResult, error) {
	params := map[string]interface{}{
		"productCode": opts.ProductCode,
		"plans":       opts.Plans,
		"platform":    opts.Platform,
		"activeNode":  opts.ActiveNode,
		"urlType":     opts.URLType,
	}
	if opts.ChannelID != "" {
		params["channelId"] = opts.ChannelID
	}
	if opts.PrivateChannel != "" {
		params["privateChannel"] = opts.PrivateChannel
	}
	if opts.MediaAppID != "" {
		params["mediaAppId"] = opts.MediaAppID
	}
	if opts.ConvertSDKType != "" {
		params["convertSdkType"] = opts.ConvertSDKType
	}
	if opts.CovertType != "" {
		params["covertType"] = opts.CovertType
	}
	if opts.GroupID != "" {
		params["groupId"] = opts.GroupID
	}
	if opts.AdPageID != "" {
		params["adPageId"] = opts.AdPageID
	}
	if opts.Note != "" {
		params["note"] = opts.Note
	}
	if opts.GameURL != "" {
		params["gameUrl"] = opts.GameURL
	}
	if opts.Package != "" {
		params["package"] = opts.Package
	}
	if opts.GameVersionID != "" {
		params["gameVersionId"] = opts.GameVersionID
	}
	if opts.CPSList != "" {
		params["cpsList"] = opts.CPSList
	}

	resp, err := c.Do(ctx, "open/createAdPlan", params)
	if err != nil {
		return nil, err
	}

	var result CreateAdPlanResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse create ad plan result: %w", err)
	}
	return &result, nil
}

// CreateAdPlanOptions holds options for CreateAdPlan.
type CreateAdPlanOptions struct {
	ProductCode    string
	Plans          string // Comma-separated plan names
	Platform       string // "APP" or "WEB"
	ChannelID      string
	PrivateChannel string
	MediaAppID     string
	ConvertSDKType string
	CovertType     string
	GroupID        string
	AdPageID       string
	Note           string
	ActiveNode     string // "INSTALL", "REGISTER", "ROLE", "PAY"
	URLType        string // "URL" or "CLOUD"
	GameURL        string
	Package        string
	GameVersionID  string
	CPSList        string
}

// CreateAdPlanResult represents the result of CreateAdPlan.
type CreateAdPlanResult struct {
	Success int `json:"success"`
	Total   int `json:"total"`
	List    []struct {
		PlanName string `json:"planName"`
		PlanCode string `json:"planCode"`
	} `json:"list"`
}

// UpdateAdPlan updates existing ad plans.
func (c *Client) UpdateAdPlan(ctx context.Context, opts UpdateAdPlanOptions) error {
	params := map[string]interface{}{
		"productCode": opts.ProductCode,
		"action":      opts.Action,
		"urlType":     opts.URLType,
	}
	if opts.Plans != "" {
		params["plans"] = opts.Plans
	}
	if opts.GameURL != "" {
		params["gameUrl"] = opts.GameURL
	}
	if opts.GameVersionID != "" {
		params["gameVersionId"] = opts.GameVersionID
	}
	if opts.OldVersionID != "" {
		params["oldVersionId"] = opts.OldVersionID
	}
	if opts.NewVersionID != "" {
		params["newVersionId"] = opts.NewVersionID
	}

	_, err := c.Do(ctx, "open/updateAdPlan", params)
	return err
}

// UpdateAdPlanOptions holds options for UpdateAdPlan.
type UpdateAdPlanOptions struct {
	ProductCode   string
	Action        string // "FROM_CODE" or "FROM_VERSION"
	Plans         string
	URLType       string // "URL" or "CLOUD"
	GameURL       string
	GameVersionID string
	OldVersionID  string
	NewVersionID  string
}

// GetAdPlanList retrieves ad plans.
func (c *Client) GetAdPlanList(ctx context.Context, productCode string, opts AdPlanListOptions) (*AdPlanListResult, error) {
	params := map[string]interface{}{
		"productCode": productCode,
		"page":        opts.Page,
	}
	if opts.PageRows > 0 {
		params["pageRows"] = opts.PageRows
	}
	if opts.Status != "" {
		params["status"] = opts.Status
	}
	if opts.ChannelID != "" {
		params["channelId"] = opts.ChannelID
	}
	if opts.AdGroupID != "" {
		params["adGroupId"] = opts.AdGroupID
	}
	if opts.PlanCode != "" {
		params["planCode"] = opts.PlanCode
	}

	resp, err := c.Do(ctx, "open/getAdPlan", params)
	if err != nil {
		return nil, err
	}

	var result AdPlanListResult
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ad plan list: %w", err)
	}
	return &result, nil
}

// AdPlanListOptions holds options for GetAdPlanList.
type AdPlanListOptions struct {
	Page      int
	PageRows  int
	Status    string // "DEPLOY_WAITTING", "DEPLOY_PROCESS", "DEPLOY_COMPLETE", "DEPLOY_FAILD"
	ChannelID string
	AdGroupID string
	PlanCode  string
}

// AdPlanListResult represents the result of GetAdPlanList.
type AdPlanListResult struct {
	Total int      `json:"total"`
	List  []AdPlan `json:"list"`
}

// GetUserLostList retrieves user churn warning list.
func (c *Client) GetUserLostList(ctx context.Context, productCode string, opts UserLostListOptions) ([]UserLost, error) {
	params := map[string]interface{}{
		"productCode": productCode,
	}
	if opts.ChannelCode != "" {
		params["channelCode"] = opts.ChannelCode
	}
	if opts.DateTime > 0 {
		params["dateTime"] = opts.DateTime
	}
	if opts.Page > 0 {
		params["page"] = opts.Page
	}
	if opts.Level > 0 {
		params["level"] = opts.Level
	}

	resp, err := c.Do(ctx, "open/uwlLost", params)
	if err != nil {
		return nil, err
	}

	var users []UserLost
	if err := json.Unmarshal(resp.Data, &users); err != nil {
		return nil, fmt.Errorf("failed to parse user lost list: %w", err)
	}
	return users, nil
}

// UserLostListOptions holds options for GetUserLostList.
type UserLostListOptions struct {
	ChannelCode string
	DateTime    int64
	Page        int
	Level       int // 0:all, 1:high risk, 2:medium risk, 3:low risk
}

// PushMessage sends a push notification to devices.
func (c *Client) PushMessage(ctx context.Context, productCode, channelCodes, gateway, title, body string) error {
	params := map[string]interface{}{
		"productCode":  productCode,
		"channel_code": channelCodes,
		"gateway":      gateway, // "huawei", "vivo", or "oppo"
		"title":        title,
		"body":         body,
	}

	_, err := c.Do(ctx, "open/pushMessage", params)
	return err
}
