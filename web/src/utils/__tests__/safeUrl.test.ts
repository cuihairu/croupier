import { safeExternalHref } from '../safeUrl';

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
