export const API_V1_PREFIX = "/api/v1";

const LEGACY_PREFIX = /^\/api(?=\/|$)/;
const ABSOLUTE_RE = /^(https?:\/\/[^/]+)(\/.*)$/;

function applyPrefix(path: string): string {
  if (path.startsWith(API_V1_PREFIX)) return path;
  return path.replace(LEGACY_PREFIX, API_V1_PREFIX);
}

/**
 * Ensure an API URL uses the versioned /api/v1 prefix. Works for relative and absolute URLs.
 */
export function normalizeApiUrl(url?: string | null): string | undefined {
  if (!url) return url ?? undefined;
  if (url.startsWith(API_V1_PREFIX)) return url;
  if (LEGACY_PREFIX.test(url)) {
    return applyPrefix(url);
  }
  const match = url.match(ABSOLUTE_RE);
  if (match && LEGACY_PREFIX.test(match[2])) {
    return `${match[1]}${applyPrefix(match[2])}`;
  }
  return url;
}

/** Convert a legacy relative path into a versioned API path. */
export function apiUrl(path: string): string {
  return normalizeApiUrl(path) || path;
}
