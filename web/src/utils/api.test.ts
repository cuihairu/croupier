import { API_V1_PREFIX, apiUrl, normalizeApiUrl } from './api';

describe('normalizeApiUrl', () => {
  it('returns undefined for nullish inputs; empty string passes through falsy-shortcut', () => {
    expect(normalizeApiUrl(undefined)).toBeUndefined();
    expect(normalizeApiUrl(null)).toBeUndefined();
    // '' 走 `url ?? undefined` 保底：?? 只兜 null/undefined，空串原样返回
    expect(normalizeApiUrl('')).toBe('');
  });

  it('keeps already-versioned /api/v1 paths untouched', () => {
    expect(normalizeApiUrl('/api/v1/audit')).toBe('/api/v1/audit');
    expect(normalizeApiUrl('/api/v1')).toBe('/api/v1');
  });

  it('upgrades legacy /api paths to /api/v1', () => {
    expect(normalizeApiUrl('/api/audit')).toBe('/api/v1/audit');
    expect(normalizeApiUrl('/api')).toBe('/api/v1');
  });

  it('rewrites the path part of absolute URLs only', () => {
    expect(normalizeApiUrl('https://ops.example.com/api/audit')).toBe(
      'https://ops.example.com/api/v1/audit',
    );
    // 非 /api 开头的绝对路径保持原样
    expect(normalizeApiUrl('https://cdn.example.com/static/app.js')).toBe(
      'https://cdn.example.com/static/app.js',
    );
  });

  it('leaves unrelated relative paths unchanged', () => {
    expect(normalizeApiUrl('/static/logo.png')).toBe('/static/logo.png');
  });
});

describe('apiUrl', () => {
  it('converts a legacy relative path into a versioned one', () => {
    expect(apiUrl('/api/players')).toBe(`${API_V1_PREFIX}/players`);
  });

  it('falls back to the original path when normalization yields nothing', () => {
    expect(apiUrl('/healthz')).toBe('/healthz');
  });
});
