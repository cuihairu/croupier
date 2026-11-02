// External-URL scheme guard for href/src/window.open sinks.
//
// Mirrors the allow-list used by MarkdownText (LINK_PROTOCOL): only absolute
// http/https URLs are considered safe to navigate/embed. Anything else —
// `javascript:`, `data:`, protocol-relative, relative paths — returns '' so
// callers can render a non-link fallback. This prevents stored-XSS via a
// user-supplied URL (e.g. a bug link `javascript:alert(document.cookie)`).

const EXTERNAL_PROTOCOL = /^https?:\/\//i;

/**
 * Returns `url` unchanged when its scheme is http/https, otherwise ''.
 * Callers treat '' as "no safe link" (render without href / skip window.open).
 */
export function safeExternalHref(url: string | null | undefined): string {
  const trimmed = url?.trim() ?? '';
  return EXTERNAL_PROTOCOL.test(trimmed) ? trimmed : '';
}
