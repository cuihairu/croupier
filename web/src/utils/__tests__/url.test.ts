/**
 * utils/url：getServerOrigin 优先级链与 assetURL 拼接矩阵。
 *
 * 优先级：window.CROUPIER_SERVER_ORIGIN > process.env.CROUPIER_SERVER_ORIGIN
 * > window.location.origin > localhost 兜底。
 */
import { assetURL, getServerOrigin, isAbsoluteUrl } from '../url';

const ORIGIN_KEY = 'CROUPIER_SERVER_ORIGIN';

describe('getServerOrigin', () => {
  const originalEnv = process.env[ORIGIN_KEY];

  afterEach(() => {
    delete window.CROUPIER_SERVER_ORIGIN;
    if (originalEnv === undefined) delete process.env[ORIGIN_KEY];
    else process.env[ORIGIN_KEY] = originalEnv;
  });

  it('window.CROUPIER_SERVER_ORIGIN 优先级最高', () => {
    window.CROUPIER_SERVER_ORIGIN = 'https://srv.example';
    process.env[ORIGIN_KEY] = 'https://env.example';
    expect(getServerOrigin()).toBe('https://srv.example');
  });

  it('无 window 注入时回落 env', () => {
    delete window.CROUPIER_SERVER_ORIGIN;
    process.env[ORIGIN_KEY] = 'https://env.example';
    expect(getServerOrigin()).toBe('https://env.example');
  });

  it('两者皆无时回落当前 location.origin（jsdom，与 testURL 配置无关）', () => {
    delete window.CROUPIER_SERVER_ORIGIN;
    delete process.env[ORIGIN_KEY];
    expect(getServerOrigin()).toBe(window.location.origin);
  });
});

describe('isAbsoluteUrl', () => {
  it('http/https/data/blob 判定绝对，空串与相对路径非绝对', () => {
    expect(isAbsoluteUrl('https://a.example/x.png')).toBe(true);
    expect(isAbsoluteUrl('HTTP://A/x')).toBe(true);
    expect(isAbsoluteUrl('data:image/png;base64,xx')).toBe(true);
    expect(isAbsoluteUrl('blob:http://x/uuid')).toBe(true);
    expect(isAbsoluteUrl('/icons/x.png')).toBe(false);
    expect(isAbsoluteUrl('icons/x.png')).toBe(false);
    expect(isAbsoluteUrl('')).toBe(false);
    expect(isAbsoluteUrl(undefined)).toBe(false);
  });
});

describe('assetURL', () => {
  afterEach(() => {
    delete window.CROUPIER_SERVER_ORIGIN;
  });

  it('空值返回空串', () => {
    expect(assetURL()).toBe('');
    expect(assetURL('')).toBe('');
  });

  it('绝对 URL 原样返回', () => {
    expect(assetURL('https://cdn.example/a.png')).toBe('https://cdn.example/a.png');
  });

  it('以 / 开头：base 直接拼接；否则补 /', () => {
    window.CROUPIER_SERVER_ORIGIN = 'https://srv.example';
    expect(assetURL('/icons/x.png')).toBe('https://srv.example/icons/x.png');
    expect(assetURL('icons/x.png')).toBe('https://srv.example/icons/x.png');
  });

  it('base 末尾多余 / 被去掉避免双斜杠', () => {
    window.CROUPIER_SERVER_ORIGIN = 'https://srv.example/';
    expect(assetURL('/icons/x.png')).toBe('https://srv.example/icons/x.png');
    expect(assetURL('icons/x.png')).toBe('https://srv.example/icons/x.png');
  });
});
