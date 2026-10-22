/// scoped API 前缀清单——从 web `services/core/scope.ts` 的
/// SCOPED_API_PREFIXES 移植（后端挂在 GameDBMiddleware 的路由组）。
///
/// 命中前缀的请求由拦截器注入 `X-Game-ID` / `X-Env`（见 api_client）。
/// 移动端只消费设计稿 §3.1 的域，但清单保持与 web 同源全量移植：
/// 后端把新域挂进 scoped 组时改一处即可。
library;

const List<String> kScopedApiPrefixes = [
  '/api/v1/analytics',
  // #45：公告按顶栏游戏过滤
  '/api/v1/announcements',
  '/api/v1/approvals',
  '/api/v1/assignments',
  '/api/v1/configs',
  '/api/v1/console',
  '/api/v1/feedback',
  '/api/v1/function-calls',
  '/api/v1/functions',
  '/api/v1/metadata',
  '/api/v1/menus',
  '/api/v1/openapi',
  '/api/v1/ops',
  '/api/v1/pages',
  '/api/v1/players',
  // #38 审计：后端 scoped 组一律显式带头，不依赖中间件「持久化 scope 兜底」
  '/api/v1/proposals',
  '/api/v1/versioning',
  '/api/v1/execution-logs',
  '/api/v1/schedules',
  '/api/v1/config-explorer',
  '/api/v1/providers/sdk-stats',
  '/api/v1/resource-catalog',
  '/api/v1/resources',
  '/api/v1/tasks',
  // #21：工单迁入后端 scoped 组
  '/api/v1/tickets',
];

/// 路径是否命中 scoped 组：精确相等或 `前缀 + '/'` 起始。
/// 与 web `needsResolvedScope` 同语义——`/api/v1/opsfoo` 不得误命中
/// `/api/v1/ops`。
bool isScopedApiPath(String path) {
  for (final prefix in kScopedApiPrefixes) {
    if (path == prefix || path.startsWith('$prefix/')) {
      return true;
    }
  }
  return false;
}
