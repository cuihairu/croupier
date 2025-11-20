#!/bin/bash

# 生成简化的 OpenAPI 规范，避免复杂的语法问题

set -e

echo "🚀 生成 Croupier API Swagger 规范..."

export PATH=$PATH:$HOME/go/bin

# 创建简化版本的 API 文件，只包含核心端点
cat > simple-swagger.api << 'EOF'
syntax = "v1"

info (
	title:   "Croupier API"
	desc:    "Croupier 游戏管理系统 API"
	author:  "Croupier Team"
	email:   "team@croupier.com"
	version: "v1.0"
)

// 认证请求类型
type LoginRequest {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse {
	Token string     `json:"token"`
	User  UserInfo   `json:"user"`
}

type UserInfo {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

// 游戏管理类型
type GameInfo {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Config      string `json:"config"`
}

type GamesListResponse {
	Games []GameInfo `json:"games"`
}

// 通用响应类型
type GenericResponse {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

// 服务定义
service croupier-api {
	// 认证相关
	@handler AuthLoginHandler
	post /api/auth/sessions (LoginRequest) returns (LoginResponse)

	@handler AuthLogoutHandler
	delete /api/auth/sessions returns (GenericResponse)

	// 用户相关
	@handler UserCurrentHandler
	get /api/users/current returns (UserInfo)

	@handler UserProfileHandler
	get /api/users/current/profile returns (UserInfo)

	// 游戏管理
	@handler GamesListHandler
	get /api/games returns (GamesListResponse)

	@handler GameCreateHandler
	post /api/games (GameInfo) returns (GenericResponse)

	// 系统运维
	@handler HealthHandler
	get /api/health returns (GenericResponse)

	@handler ServicesHandler
	get /api/services returns (GenericResponse)

	@handler NodesHandler
	get /api/nodes returns (GenericResponse)

	// 注册中心
	@handler RegistryHandler
	get /api/registry returns (GenericResponse)

	// 功能实例
	@handler FunctionInstancesHandler
	get /api/function_instances returns (GenericResponse)

	// 上传文件
	@handler UploadHandler
	post /api/upload returns (GenericResponse)

	// 证书管理
	@handler CertificatesListHandler
	get /api/certificates returns (GenericResponse)

	@handler CertificateAddHandler
	post /api/certificates returns (GenericResponse)

	@handler CertificateCheckHandler
	post /api/certificates/:id/check returns (GenericResponse)

	@handler CertificateStatsHandler
	get /api/certificates/stats returns (GenericResponse)

	// 分析相关
	@handler AnalyticsOverviewHandler
	get /api/analytics/overview returns (GenericResponse)

	@handler AnalyticsRealtimeHandler
	get /api/analytics/realtime returns (GenericResponse)

	@handler AnalyticsBehaviorHandler
	get /api/analytics/behavior returns (GenericResponse)

	@handler AnalyticsPaymentsHandler
	get /api/analytics/payments returns (GenericResponse)

	@handler AnalyticsLevelsHandler
	get /api/analytics/levels returns (GenericResponse)

	@handler AnalyticsRetentionHandler
	get /api/analytics/retention returns (GenericResponse)

	// 运维操作
	@handler OpsJobsHandler
	get /api/jobs returns (GenericResponse)

	@handler OpsBackupsHandler
	get /api/backups returns (GenericResponse)

	@handler OpsNotificationsHandler
	get /api/notifications returns (GenericResponse)

	@handler OpsAlertsHandler
	get /api/alerts returns (GenericResponse)

	// 配置管理
	@handler ConfigsListHandler
	get /api/configs returns (GenericResponse)

	@handler ConfigDetailHandler
	get /api/configs/:id returns (GenericResponse)

	@handler ConfigValidateHandler
	post /api/configs/:id/validate returns (GenericResponse)

	// 权限管理
	@handler UsersListHandler
	get /api/users returns (GenericResponse)

	@handler UserCreateHandler
	post /api/users returns (GenericResponse)

	@handler UserUpdateHandler
	put /api/users/:id returns (GenericResponse)

	@handler RolesListHandler
	get /api/roles returns (GenericResponse)

	@handler RoleCreateHandler
	post /api/roles returns (GenericResponse)

	@handler RoleUpdateHandler
	put /api/roles/:id returns (GenericResponse)

	// 支持系统
	@handler SupportTicketsHandler
	get /api/support/tickets returns (GenericResponse)

	@handler SupportTicketCreateHandler
	post /api/support/tickets returns (GenericResponse)

	@handler SupportFAQHandler
	get /api/support/faq returns (GenericResponse)

	@handler SupportFeedbackHandler
	get /api/support/feedback returns (GenericResponse)

	// 消息系统
	@handler MessagesListHandler
	get /api/messages returns (GenericResponse)

	@handler MessageSendHandler
	post /api/messages returns (GenericResponse)

	// 功能管理
	@handler FunctionsListHandler
	get /api/functions returns (GenericResponse)

	@handler FunctionDetailHandler
	get /api/functions/:id returns (GenericResponse)

	@handler ComponentsListHandler
	get /api/components returns (GenericResponse)

	@handler ProvidersListHandler
	get /api/providers returns (GenericResponse)
}
EOF

echo "✅ 简化的 API 文件已创建"

# 生成 OpenAPI 规范
echo "📄 生成 OpenAPI/Swagger 规范..."
goctl api swagger --api simple-swagger.api --dir . --filename croupier-api-simple

if [ $? -eq 0 ]; then
    echo "✅ OpenAPI 规范生成成功: croupier-api-simple.json"
    echo ""
    echo "📋 生成的文件:"
    echo "  - croupier-api-simple.json (简化版 OpenAPI 规范)"
    echo "  - simple-swagger.api (简化版 API 定义)"
    echo ""
    echo "💡 使用方法:"
    echo "  1. 导入 croupier-api-simple.json 到 Swagger UI"
    echo "  2. 导入到 Postman: Import → Link"
    echo "  3. 导入到 APIfox: 导入 → OpenAPI"
else
    echo "❌ OpenAPI 规范生成失败"
    exit 1
fi

# 清理临时文件
rm -f simple-swagger.api

echo ""
echo "🎉 任务完成！"
echo ""
echo "📊 文件统计:"
echo "  简化 API 定义: $(cat simple-swagger.api 2>/dev/null | wc -l) 行"
echo "  生成文档大小: $(cat croupier-api-simple.json 2>/dev/null | wc -c) 字节"