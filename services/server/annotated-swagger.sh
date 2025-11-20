#!/bin/bash

# 生成带完整注释的 OpenAPI 规范

set -e

echo "🚀 生成带完整注释的 Croupier API Swagger 规范..."

export PATH=$PATH:$HOME/go/bin

# 创建带注释的 API 文件
cat > annotated-api.api << 'EOF'
syntax = "v1"

info (
	title:   "Croupier API"
	desc:    "Croupier 游戏管理系统 API - 基于 go-zero 微服务架构"
	author:  "Croupier Team"
	email:   "team@croupier.com"
	version: "v1.0"
)

// ============================================================================
// 认证相关类型定义
// ============================================================================

// 登录请求 - 用户登录认证
type LoginRequest {
	Username string `json:"username"` // 用户名
	Password string `json:"password"` // 密码
}

// 登录响应 - 包含认证令牌和用户信息
type LoginResponse {
	Token string   `json:"token"` // JWT 认证令牌
	User  UserInfo `json:"user"`  // 用户基本信息
}

// 用户信息 - 系统用户基本信息
type UserInfo {
	Username string   `json:"username"` // 用户名
	Roles    []string `json:"roles"`    // 用户角色列表
}

// ============================================================================
// 游戏管理相关类型
// ============================================================================

// 游戏基本信息
type GameInfo {
	Id          string `json:"id"`          // 游戏唯一标识
	Name        string `json:"name"`        // 游戏名称
	Description string `json:"description"` // 游戏描述
	Config      string `json:"config"`      // 游戏配置信息
	Status      string `json:"status"`      // 游戏状态
	CreatedAt   string `json:"created_at"`  // 创建时间
}

// 游戏列表响应
type GamesListResponse {
	Total int        `json:"total"`  // 总数量
	Games []GameInfo `json:"games"` // 游戏列表
	Page  int        `json:"page"`   // 当前页码
	Size  int        `json:"size"`   // 每页大小
}

// ============================================================================
// 系统运维相关类型
// ============================================================================

// 健康检查项
type HealthCheck {
	Name     string `json:"name"`     // 检查项名称
	Status   string `json:"status"`   // 状态: healthy/unhealthy
	Message  string `json:"message"`  // 详细信息
	Duration string `json:"duration"` // 检查耗时
}

// 健康状态响应
type HealthStatus {
	Service string       `json:"service"` // 服务名称
	Status  string       `json:"status"`  // 整体状态
	Checks  []HealthCheck `json:"checks"`  // 检查项列表
}

// 服务信息
type ServiceInfo {
	Id       string `json:"id"`       // 服务ID
	Name     string `json:"name"`     // 服务名称
	Status   string `json:"status"`   // 运行状态
	Version  string `json:"version"`  // 版本号
	Endpoint string `json:"endpoint"` // 服务地址
}

// 节点信息
type NodeInfo {
	Id        string `json:"id"`        // 节点ID
	Name      string `json:"name"`      // 节点名称
	Status    string `json:"status"`    // 节点状态
	Region    string `json:"region"`    // 所在区域
	Ip        string `json:"ip"`        // IP地址
	CpuUsage  float64 `json:"cpu_usage"` // CPU使用率
	MemUsage  float64 `json:"mem_usage"` // 内存使用率
	CreatedAt string `json:"created_at"` // 创建时间
}

// ============================================================================
// 通用响应类型
// ============================================================================

// 标准成功响应
type GenericOkResponse {
	Ok      bool   `json:"ok"`      // 操作是否成功
	Message string `json:"message"`  // 响应消息
	Data    string `json:"data"`     // 响应数据(可选)
}

// 标准分页响应
type PageResponse {
	Total int `json:"total"` // 总记录数
	Page  int `json:"page"`  // 当前页码
	Size  int `json:"size"`  // 每页大小
}

// ============================================================================
// 服务定义 - API 路由配置
// ============================================================================

service croupier-api {
	// ========================================================================
	// 认证服务 (Authentication)
	// ========================================================================

	// 用户登录认证
	@handler AuthLoginHandler
	post /api/auth/sessions (LoginRequest) returns (LoginResponse)

	// 用户登出
	@handler AuthLogoutHandler
	delete /api/auth/sessions returns (GenericOkResponse)

	// ========================================================================
	// 用户服务 (Users)
	// ========================================================================

	// 获取当前用户信息
	@handler UserCurrentHandler
	get /api/users/current returns (UserInfo)

	// 获取当前用户详细资料
	@handler UserProfileHandler
	get /api/users/current/profile returns (UserInfo)

	// ========================================================================
	// 游戏管理 (Games)
	// ========================================================================

	// 获取游戏列表 - 支持分页和筛选
	@handler GamesListHandler
	get /api/games returns (GamesListResponse)

	// 创建新游戏
	@handler GameCreateHandler
	post /api/games (GameInfo) returns (GenericOkResponse)

	// 更新游戏信息
	@handler GameUpdateHandler
	put /api/games/:id (GameInfo) returns (GenericOkResponse)

	// 获取游戏详情
	@handler GameDetailHandler
	get /api/games/:id returns (GameInfo)

	// ========================================================================
	// 系统运维 (Operations)
	// ========================================================================

	// 系统健康检查
	@handler HealthHandler
	get /api/health returns (HealthStatus)

	// 获取服务列表
	@handler ServicesHandler
	get /api/services returns (GenericOkResponse)

	// 获取节点列表
	@handler NodesHandler
	get /api/nodes returns (GenericOkResponse)

	// ========================================================================
	// 注册中心 (Registry)
	// ========================================================================

	// 服务注册信息查询
	@handler RegistryHandler
	get /api/registry returns (GenericOkResponse)

	// 功能实例查询
	@handler FunctionInstancesHandler
	get /api/function_instances returns (GenericOkResponse)

	// ========================================================================
	// 文件管理 (Files)
	// ========================================================================

	// 文件上传接口
	@handler UploadHandler
	post /api/upload returns (GenericOkResponse)

	// ========================================================================
	// 证书管理 (Certificates)
	// ========================================================================

	// 获取证书列表
	@handler CertificatesListHandler
	get /api/certificates returns (GenericOkResponse)

	// 添加新证书
	@handler CertificateAddHandler
	post /api/certificates returns (GenericOkResponse)

	// 检查证书状态
	@handler CertificateCheckHandler
	post /api/certificates/:id/check returns (GenericOkResponse)

	// 获取证书统计信息
	@handler CertificateStatsHandler
	get /api/certificates/stats returns (GenericOkResponse)

	// ========================================================================
	// 数据分析 (Analytics)
	// ========================================================================

	// 概览数据统计
	@handler AnalyticsOverviewHandler
	get /api/analytics/overview returns (GenericOkResponse)

	// 实时数据监控
	@handler AnalyticsRealtimeHandler
	get /api/analytics/realtime returns (GenericOkResponse)

	// 用户行为分析
	@handler AnalyticsBehaviorHandler
	get /api/analytics/behavior returns (GenericOkResponse)

	// 支付数据分析
	@handler AnalyticsPaymentsHandler
	get /api/analytics/payments returns (GenericOkResponse)

	// 等级数据分析
	@handler AnalyticsLevelsHandler
	get /api/analytics/levels returns (GenericOkResponse)

	// 留存率分析
	@handler AnalyticsRetentionHandler
	get /api/analytics/retention returns (GenericOkResponse)

	// ========================================================================
	// 任务管理 (Jobs)
	// ========================================================================

	// 获取任务列表
	@handler OpsJobsHandler
	get /api/jobs returns (GenericOkResponse)

	// 备份管理
	@handler OpsBackupsHandler
	get /api/backups returns (GenericOkResponse)

	// 通知管理
	@handler OpsNotificationsHandler
	get /api/notifications returns (GenericOkResponse)

	// 告警管理
	@handler OpsAlertsHandler
	get /api/alerts returns (GenericOkResponse)

	// ========================================================================
	// 配置管理 (Configurations)
	// ========================================================================

	// 获取配置列表
	@handler ConfigsListHandler
	get /api/configs returns (GenericOkResponse)

	// 获取配置详情
	@handler ConfigDetailHandler
	get /api/configs/:id returns (GenericOkResponse)

	// 验证配置
	@handler ConfigValidateHandler
	post /api/configs/:id/validate returns (GenericOkResponse)

	// ========================================================================
	// 权限管理 (Permissions)
	// ========================================================================

	// 获取用户列表
	@handler UsersListHandler
	get /api/users returns (GenericOkResponse)

	// 创建用户
	@handler UserCreateHandler
	post /api/users returns (GenericOkResponse)

	// 更新用户
	@handler UserUpdateHandler
	put /api/users/:id returns (GenericOkResponse)

	// 获取角色列表
	@handler RolesListHandler
	get /api/roles returns (GenericOkResponse)

	// 创建角色
	@handler RoleCreateHandler
	post /api/roles returns (GenericOkResponse)

	// 更新角色
	@handler RoleUpdateHandler
	put /api/roles/:id returns (GenericOkResponse)

	// ========================================================================
	// 支持系统 (Support)
	// ========================================================================

	// 工单管理
	@handler SupportTicketsHandler
	get /api/support/tickets returns (GenericOkResponse)

	// 创建工单
	@handler SupportTicketCreateHandler
	post /api/support/tickets returns (GenericOkResponse)

	// FAQ管理
	@handler SupportFAQHandler
	get /api/support/faq returns (GenericOkResponse)

	// 反馈管理
	@handler SupportFeedbackHandler
	get /api/support/feedback returns (GenericOkResponse)

	// ========================================================================
	// 消息系统 (Messages)
	// ========================================================================

	// 获取消息列表
	@handler MessagesListHandler
	get /api/messages returns (GenericOkResponse)

	// 发送消息
	@handler MessageSendHandler
	post /api/messages returns (GenericOkResponse)

	// ========================================================================
	// 功能管理 (Functions)
	// ========================================================================

	// 获取功能列表
	@handler FunctionsListHandler
	get /api/functions returns (GenericOkResponse)

	// 获取功能详情
	@handler FunctionDetailHandler
	get /api/functions/:id returns (GenericOkResponse)

	// 获取组件列表
	@handler ComponentsListHandler
	get /api/components returns (GenericOkResponse)

	// 获取提供者列表
	@handler ProvidersListHandler
	get /api/providers returns (GenericOkResponse)
}
EOF

echo "✅ 带注释的 API 文件已创建"

# 生成 OpenAPI 规范
echo "📄 生成带注释的 OpenAPI/Swagger 规范..."
goctl api swagger --api annotated-api.api --dir . --filename croupier-api-annotated

if [ $? -eq 0 ]; then
    echo "✅ OpenAPI 规范生成成功: croupier-api-annotated.json"
    echo ""
    echo "📋 生成的文件:"
    echo "  - croupier-api-annotated.json (带完整注释的 OpenAPI 规范)"
    echo "  - annotated-api.api (带注释的 API 定义)"
    echo ""
    echo "🎯 特点:"
    echo "  ✅ 包含完整的中文注释"
    echo "  ✅ 按功能模块分类"
    echo "  ✅ 详细的字段说明"
    echo "  ✅ 清晰的API分组"
    echo ""
    echo "💡 使用方法:"
    echo "  1. 导入 croupier-api-annotated.json 到 Swagger UI"
    echo "  2. 导入到 Postman: Import → Link"
    echo "  3. 导入到 APIfox: 导入 → OpenAPI"
    echo ""
    echo "📊 文件统计:"
    echo "  API 定义行数: $(cat annotated-api.api | wc -l)"
    echo "  注释行数: $(grep -c '//' annotated-api.api)"
    echo "  API 端点数: $(grep -c '@handler' annotated-api.api)"
else
    echo "❌ OpenAPI 规范生成失败"
    exit 1
fi

# 清理临时文件
rm -f annotated-api.api

echo ""
echo "🎉 带注释的 API 文档生成完成！"