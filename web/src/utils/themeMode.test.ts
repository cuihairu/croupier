/**
 * utils/themeMode 单测：三档偏好（亮/暗/跟随系统）的读取/解析/持久化/订阅。
 *
 * 契约：
 * - 未写入或脏值 → 'system'（跟随系统）；旧版只写过 light/dark，视为显式选择；
 * - setThemePref：localStorage 持久化 + data-theme 属性立即切换 + 订阅者广播；
 * - initThemeAttr：只写属性不落盘（从未选择的用户保持跟随系统）。
 */
import {
  getResolvedTheme,
  initThemeAttr,
  readThemePref,
  resolveTheme,
  setThemePref,
  subscribeThemePref,
  THEME_PREF_STORAGE_KEY,
} from './themeMode';

// tests/setupTests.jsx 的 localStorage 是无状态 jest.fn 桩；本套件需要
// 真实读写语义（偏好持久化是核心契约），换成 Map 背书的实现
const storageBacking = new Map<string, string>();
Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: (k: string) => (storageBacking.has(k) ? (storageBacking.get(k) ?? null) : null),
    setItem: (k: string, v: string) => void storageBacking.set(k, v),
    removeItem: (k: string) => void storageBacking.delete(k),
    clear: () => storageBacking.clear(),
  },
  writable: true,
  configurable: true,
});

const html = () => document.documentElement;

beforeEach(() => {
  window.localStorage.clear();
  html().removeAttribute('data-theme');
});

describe('readThemePref', () => {
  it('未写入时默认跟随系统', () => {
    expect(readThemePref()).toBe('system');
  });

  it.each(['light', 'dark', 'system'] as const)('合法值 %s 原样读回', (v) => {
    window.localStorage.setItem(THEME_PREF_STORAGE_KEY, v);
    expect(readThemePref()).toBe(v);
  });

  it('脏值回退跟随系统', () => {
    window.localStorage.setItem(THEME_PREF_STORAGE_KEY, 'blue');
    expect(readThemePref()).toBe('system');
  });
});

describe('resolveTheme', () => {
  it('显式档不读系统色', () => {
    expect(resolveTheme('light', true)).toBe('light');
    expect(resolveTheme('dark', false)).toBe('dark');
  });

  it('跟随系统档按系统色解析', () => {
    expect(resolveTheme('system', true)).toBe('dark');
    expect(resolveTheme('system', false)).toBe('light');
  });
});

describe('setThemePref', () => {
  it('持久化 + data-theme 立即生效 + 广播订阅者', () => {
    const listener = jest.fn();
    const unsubscribe = subscribeThemePref(listener);
    // jsdom matchMedia mock matches=false → system 解析为 light
    expect(setThemePref('dark')).toBe('dark');
    expect(window.localStorage.getItem(THEME_PREF_STORAGE_KEY)).toBe('dark');
    expect(html().getAttribute('data-theme')).toBe('dark');
    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    setThemePref('light');
    expect(listener).toHaveBeenCalledTimes(1);
  });

  it('跟随系统档也持久化偏好本身（而非解析结果）', () => {
    setThemePref('system');
    expect(window.localStorage.getItem(THEME_PREF_STORAGE_KEY)).toBe('system');
    expect(html().getAttribute('data-theme')).toBe('light');
  });
});

describe('initThemeAttr', () => {
  it('按当前偏好写属性但不落盘', () => {
    expect(initThemeAttr()).toBe('light');
    expect(html().getAttribute('data-theme')).toBe('light');
    expect(window.localStorage.getItem(THEME_PREF_STORAGE_KEY)).toBeNull();

    setThemePref('dark');
    expect(initThemeAttr()).toBe('dark');
  });

  it('getResolvedTheme 与属性一致', () => {
    setThemePref('dark');
    expect(getResolvedTheme()).toBe('dark');
    expect(html().getAttribute('data-theme')).toBe(getResolvedTheme());
  });
});
