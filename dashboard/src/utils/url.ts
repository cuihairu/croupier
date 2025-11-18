// Utility helpers for building absolute URLs for assets (icons etc.)
// Priority: window.CROUPIER_SERVER_ORIGIN > process.env.CROUPIER_SERVER_ORIGIN > http://localhost:8080

export function getServerOrigin(): string {
  if (typeof window !== 'undefined' && (window as any).CROUPIER_SERVER_ORIGIN) {
    return (window as any).CROUPIER_SERVER_ORIGIN as string;
  }
  // In Umi, process.env.* can be injected at build/dev time
  const fromEnv = (process as any)?.env?.CROUPIER_SERVER_ORIGIN as string | undefined;
  if (fromEnv) return fromEnv;
  // Fallback to current origin when available (prod build served by server)
  if (typeof window !== 'undefined' && window.location?.origin) {
    return window.location.origin;
  }
  // Dev fallback
  return 'http://localhost:8080';
}

export function isAbsoluteUrl(u?: string): boolean {
  if (!u) return false;
  return /^(https?:|data:|blob:)/i.test(u);
}

// Build full URL for assets. If `u` is already absolute, return as-is.
// If `u` starts with '/', prefix with server origin. Else, join with '/'.
export function assetURL(u?: string): string {
  if (!u) return '';
  if (isAbsoluteUrl(u)) return u;
  const base = getServerOrigin().replace(/\/$/, '');
  if (u.startsWith('/')) return base + u;
  return base + '/' + u;
}
