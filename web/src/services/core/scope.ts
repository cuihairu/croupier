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
  '/api/v1/openapi',
  '/api/v1/ops',
  '/api/v1/pages',
  '/api/v1/players',
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
