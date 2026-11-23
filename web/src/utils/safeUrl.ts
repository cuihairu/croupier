// External-URL scheme guard for href/src/window.open sinks.
//
// Mirrors the allow-list used by MarkdownText (LINK_PROTOCOL): only absolute
// http/https URLs are considered safe to navigate/embed. Anything else —
// `javascript:`, `data:`, protocol-relative, relative paths — returns '' so
// callers can render a non-link fallback. This prevents stored-XSS via a
// user-supplied URL (e.g. a bug link `javascript:alert(document.cookie)`).

const EXTERNAL_PROTOCOL = /^https?:\/\//i;

/**
 * Same-origin relative-path guard for the other navigation sink family:
 * SPA-relative paths (`/admin/...`) built by string concat. Unlike external
 * URLs these carry no scheme, so the acceptance rule is positional: rooted at
 * exactly one `/` (rejects protocol-relative `//evil.com`), no backslash (the
 * browser normalizes `\` into a path separator), no control characters.
 */
const SAFE_SAME_ORIGIN_RELATIVE = /^\/(?!\/)[^\n\r\\]*$/;

/**
 * Returns `url` unchanged when it is a same-origin relative path, otherwise ''.
 * Callers treat '' as "no safe link" (render without href / skip window.open).
 */
export function safeRelativeHref(url: string | null | undefined): string {
  const trimmed = url?.trim() ?? '';
  return SAFE_SAME_ORIGIN_RELATIVE.test(trimmed) ? trimmed : '';
}

/**
 * Returns `url` unchanged when its scheme is http/https, otherwise ''.
 * Callers treat '' as "no safe link" (render without href / skip window.open).
 */
export function safeExternalHref(url: string | null | undefined): string {
  const trimmed = url?.trim() ?? '';
  return EXTERNAL_PROTOCOL.test(trimmed) ? trimmed : '';
}
