import { safeExternalHref, safeRelativeHref } from '../safeUrl';

describe('safeExternalHref', () => {
  it('allows http and https URLs', () => {
    expect(safeExternalHref('https://github.com/a/b/issues/1')).toBe(
      'https://github.com/a/b/issues/1',
    );
    expect(safeExternalHref('http://example.com')).toBe('http://example.com');
  });

  it('rejects javascript: and data: URLs (stored-XSS vector)', () => {
    expect(safeExternalHref('javascript:alert(document.cookie)')).toBe('');
    expect(safeExternalHref('JAVASCRIPT:alert(1)')).toBe('');
    expect(safeExternalHref('data:text/html,<script>alert(1)</script>')).toBe('');
  });

  it('rejects protocol-relative, relative, and bare paths', () => {
    expect(safeExternalHref('//evil.com/x')).toBe('');
    expect(safeExternalHref('/relative/path')).toBe('');
    expect(safeExternalHref('not-a-url')).toBe('');
  });

  it('rejects empty and nullish input', () => {
    expect(safeExternalHref('')).toBe('');
    expect(safeExternalHref('   ')).toBe('');
    expect(safeExternalHref(null)).toBe('');
    expect(safeExternalHref(undefined)).toBe('');
  });

  it('trims surrounding whitespace', () => {
    expect(safeExternalHref('  https://example.com  ')).toBe('https://example.com');
  });
});

describe('safeRelativeHref', () => {
  it('allows same-origin relative paths (SPA route sinks)', () => {
    expect(safeRelativeHref('/admin/operation-logs?actor=ops1')).toBe(
      '/admin/operation-logs?actor=ops1',
    );
    expect(safeRelativeHref('/functions/invoke?fid=player%20list')).toBe(
      '/functions/invoke?fid=player%20list',
    );
    expect(safeRelativeHref('/')).toBe('/');
  });

  it('rejects protocol-relative, absolute, and scheme URLs', () => {
    expect(safeRelativeHref('//evil.com/x')).toBe('');
    expect(safeRelativeHref('https://evil.com/x')).toBe('');
    expect(safeRelativeHref('javascript:alert(1)')).toBe('');
    expect(safeRelativeHref('data:text/html,<script>alert(1)</script>')).toBe('');
  });

  it('rejects backslash (browser path-separator normalization) and control chars', () => {
    expect(safeRelativeHref('/admin\\evil')).toBe('');
    expect(safeRelativeHref('/a\nb')).toBe('');
  });

  it('rejects non-rooted, empty, and nullish input', () => {
    expect(safeRelativeHref('not-a-path')).toBe('');
    expect(safeRelativeHref('')).toBe('');
    expect(safeRelativeHref('   ')).toBe('');
    expect(safeRelativeHref(null)).toBe('');
    expect(safeRelativeHref(undefined)).toBe('');
  });

  it('trims surrounding whitespace', () => {
    expect(safeRelativeHref('  /admin/logs  ')).toBe('/admin/logs');
  });
});
