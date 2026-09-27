import { normalizeApiUrl } from '@/utils/api';
import { getScope, isScopeReady, scopeReadyPromise } from '@/stores/scope';

const SCOPED_API_PREFIXES = [
  '/api/v1/analytics',
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
  // 后端挂在 scoped 组（GameDBMiddleware）但此前前端未注入头，靠中间件的
  // 「持久化 scope 兜底」——与顶栏选择器存在竞态/漂移，统一显式带头（#38 审计）：
  '/api/v1/proposals',
  '/api/v1/versioning',
  '/api/v1/execution-logs',
  '/api/v1/schedules',
  '/api/v1/config-explorer',
  '/api/v1/providers/sdk-stats',
  '/api/v1/resource-catalog',
  '/api/v1/resources',
  '/api/v1/tasks',
];

export function needsResolvedScope(url?: string): boolean {
  const normalized = normalizeApiUrl(url);
  if (!normalized) return false;
  return SCOPED_API_PREFIXES.some(
    (prefix) => normalized === prefix || normalized.startsWith(`${prefix}/`),
  );
}

export async function waitForResolvedScope(url?: string): Promise<void> {
  if (needsResolvedScope(url) && !isScopeReady()) {
    await scopeReadyPromise;
  }
}

function isHeaderValue(value?: string): value is string {
  return typeof value === 'string' && value.length > 0 && /^[\x20-\x7e]+$/.test(value);
}

// getScopeHeaders deliberately treats game and environment as one unit. A
// partial scope must never be sent because the server rejects it atomically.
export function getScopeHeaders(): { gameID: string; env: string } | undefined {
  const scope = getScope();
  const gameID = scope.gameId?.trim();
  const env = scope.env?.trim();
  if (!isHeaderValue(gameID) || !isHeaderValue(env)) return undefined;
  return { gameID, env };
}

export function applyScopeHeaders(headers: Headers): void {
  headers.delete('X-Game-ID');
  headers.delete('X-Env');

  const scope = getScopeHeaders();
  if (!scope) return;
  headers.set('X-Game-ID', scope.gameID);
  headers.set('X-Env', scope.env);
}
