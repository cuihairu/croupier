/**
 * 控制台主题单一事实源：主题套（preset）× 明暗档（mode）两维偏好。
 *
 * - 主题套：'blue'（拂晓蓝，出厂默认）| 'inkpink'（荷官墨粉，可选保留）。
 *   偏好持久化在 localStorage（key `croupier-theme-preset`），未写入/脏值
 *   一律视为 'blue'——主题改造只「加主题」不「换主题」，默认主题恒为蓝。
 * - 明暗档：'light' | 'dark' | 'system'，key `croupier-theme`（沿用历史键；
 *   旧版本只写过 light/dark，语义天然兼容；未写入时视为跟随系统）。
 * - 生效动作 = 写 <html data-preset>（主题套 CSS 变量开关）与
 *   <html data-theme>（明暗 CSS 变量开关）；antd 组件侧 algorithm/token 由
 *   <ThemeSync> 与 app.tsx 的 antd 运行时导出按 (preset, mode) 另行同步。
 * - 跟随系统档按 prefers-color-scheme 在「当前主题套」内解析亮暗；storage
 *   事件让多标签页两维偏好一致。
 */
export type ThemePreset = 'blue' | 'inkpink';
export type ThemePref = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';

export const THEME_PREF_STORAGE_KEY = 'croupier-theme';
export const THEME_PRESET_STORAGE_KEY = 'croupier-theme-preset';
export const DEFAULT_THEME_PRESET: ThemePreset = 'blue';

const THEME_PREFS: readonly ThemePref[] = ['light', 'dark', 'system'];
const THEME_PRESET_LIST: readonly ThemePreset[] = ['blue', 'inkpink'];

type Listener = () => void;
const listeners = new Set<Listener>();
let systemWatchInstalled = false;

export function getSystemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false;
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export function readThemePref(): ThemePref {
  if (typeof window === 'undefined') return 'system';
  try {
    const raw = window.localStorage.getItem(THEME_PREF_STORAGE_KEY);
    return THEME_PREFS.includes(raw as ThemePref) ? (raw as ThemePref) : 'system';
  } catch {
    // 隐私模式等 localStorage 不可用：退回默认档，不抛错
    return 'system';
  }
}

export function readThemePreset(): ThemePreset {
  if (typeof window === 'undefined') return DEFAULT_THEME_PRESET;
  try {
    const raw = window.localStorage.getItem(THEME_PRESET_STORAGE_KEY);
    return THEME_PRESET_LIST.includes(raw as ThemePreset)
      ? (raw as ThemePreset)
      : DEFAULT_THEME_PRESET;
  } catch {
    return DEFAULT_THEME_PRESET;
  }
}

export function resolveTheme(pref: ThemePref, systemDark = getSystemPrefersDark()): ResolvedTheme {
  if (pref === 'system') return systemDark ? 'dark' : 'light';
  return pref;
}

export function getResolvedTheme(): ResolvedTheme {
  return resolveTheme(readThemePref());
}

function notify(): void {
  listeners.forEach((l) => l());
}

function writeAttr(name: string, value: string): void {
  if (typeof document === 'undefined') return;
  document.documentElement.setAttribute(name, value);
}

function ensureSystemWatch(): void {
  if (
    systemWatchInstalled ||
    typeof window === 'undefined' ||
    typeof window.matchMedia !== 'function'
  ) {
    return;
  }
  systemWatchInstalled = true;
  const mql = window.matchMedia('(prefers-color-scheme: dark)');
  const onChange = () => {
    if (readThemePref() !== 'system') return;
    writeAttr('data-theme', resolveTheme('system'));
    notify();
  };
  if (typeof mql.addEventListener === 'function') mql.addEventListener('change', onChange);
  else mql.addListener(onChange);
  // 跨标签页同步：其他页签改两维偏好，本页跟随
  window.addEventListener('storage', (e) => {
    if (e.key !== THEME_PREF_STORAGE_KEY && e.key !== THEME_PRESET_STORAGE_KEY) return;
    writeAttr('data-theme', getResolvedTheme());
    writeAttr('data-preset', readThemePreset());
    notify();
  });
}

/**
 * 启动期初始化（global.tsx 早期调用，避免非默认偏好用户先看到默认闪屏）。
 * 只写属性与监听，不落盘——「从未选择过」的用户保持默认（蓝 × 跟随系统）。
 */
export function initThemeAttr(): ResolvedTheme {
  writeAttr('data-preset', readThemePreset());
  const resolved = getResolvedTheme();
  writeAttr('data-theme', resolved);
  ensureSystemWatch();
  return resolved;
}

/** 设置明暗档：持久化 + 立即生效 + 广播订阅者（设置页/顶栏共用入口） */
export function setThemePref(pref: ThemePref): ResolvedTheme {
  ensureSystemWatch();
  try {
    window.localStorage.setItem(THEME_PREF_STORAGE_KEY, pref);
  } catch {
    // 持久化失败仍切换本页生效值
  }
  const resolved = resolveTheme(pref);
  writeAttr('data-theme', resolved);
  notify();
  return resolved;
}

/** 设置主题套：持久化 + data-preset 立即生效 + 广播订阅者 */
export function setThemePreset(preset: ThemePreset): ThemePreset {
  ensureSystemWatch();
  try {
    window.localStorage.setItem(THEME_PRESET_STORAGE_KEY, preset);
  } catch {
    // 持久化失败仍切换本页生效值
  }
  writeAttr('data-preset', preset);
  notify();
  return preset;
}

export function subscribeThemePref(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
