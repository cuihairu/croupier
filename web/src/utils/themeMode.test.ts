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
  readThemePreset,
  readThemePref,
  resolveTheme,
  setThemePreset,
  setThemePref,
  subscribeThemePref,
  DEFAULT_THEME_PRESET,
  THEME_PREF_STORAGE_KEY,
  THEME_PRESET_STORAGE_KEY,
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
  html().removeAttribute('data-preset');
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

describe('readThemePreset', () => {
  it('未写入时默认 blue（出厂默认主题，不随彩蛋主题漂移）', () => {
    expect(readThemePreset()).toBe('blue');
    expect(readThemePreset()).toBe(DEFAULT_THEME_PRESET);
  });

  it.each(['blue', 'inkpink'] as const)('合法值 %s 原样读回', (v) => {
    window.localStorage.setItem(THEME_PRESET_STORAGE_KEY, v);
    expect(readThemePreset()).toBe(v);
  });

  it('脏值回退 blue', () => {
    window.localStorage.setItem(THEME_PRESET_STORAGE_KEY, 'neon');
    expect(readThemePreset()).toBe('blue');
  });
});

describe('setThemePreset', () => {
  it('持久化 + data-preset 立即生效 + 广播订阅者', () => {
    const listener = jest.fn();
    const unsubscribe = subscribeThemePref(listener);
    expect(setThemePreset('inkpink')).toBe('inkpink');
    expect(window.localStorage.getItem(THEME_PRESET_STORAGE_KEY)).toBe('inkpink');
    expect(html().getAttribute('data-preset')).toBe('inkpink');
    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    setThemePreset('blue');
    expect(listener).toHaveBeenCalledTimes(1);
    expect(html().getAttribute('data-preset')).toBe('blue');
  });
});

describe('双维独立（主题套 × 明暗档）', () => {
  it('换明暗档不动主题套，换主题套不动明暗档', () => {
    setThemePreset('inkpink');
    setThemePref('dark');
    expect(readThemePreset()).toBe('inkpink');
    expect(readThemePref()).toBe('dark');
    expect(html().getAttribute('data-preset')).toBe('inkpink');
    expect(html().getAttribute('data-theme')).toBe('dark');

    setThemePref('light');
    expect(readThemePreset()).toBe('inkpink');
    setThemePreset('blue');
    expect(readThemePref()).toBe('light');
  });
});

describe('initThemeAttr（主题套）', () => {
  it('按当前主题套写 data-preset 但不落盘', () => {
    // 无偏好：init 写默认套（blue），不持久化
    expect(initThemeAttr()).toBe('light');
    expect(html().getAttribute('data-preset')).toBe('blue');
    expect(window.localStorage.getItem(THEME_PRESET_STORAGE_KEY)).toBeNull();
    // 显式偏好：init 读回并落属性
    setThemePreset('inkpink');
    expect(initThemeAttr()).toBe('light');
    expect(html().getAttribute('data-preset')).toBe('inkpink');
  });
});
