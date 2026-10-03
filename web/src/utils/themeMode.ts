/**
 * 控制台主题模式单一事实源：三档偏好（亮色/暗色/跟随系统）。
 *
 * - 偏好持久化在 localStorage（key `croupier-theme`）。旧版本只写过
 *   'light' | 'dark'（顶栏快捷切换写入的显式选择），语义天然兼容；新增
 *   'system' 为默认档，未写入时视为跟随系统。
 * - 生效动作 = 写 <html data-theme>（global.less 双套 CSS 变量的开关），
 *   antd 组件侧的 algorithm/token 由 <ThemeSync> 与 app.tsx 的 antd 运行时
 *   导出按生效主题另行同步。
 * - 跟随系统档监听 prefers-color-scheme 变化实时切换；storage 事件让多标签
 *   页偏好一致。
 */
export type ThemePref = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';

export const THEME_PREF_STORAGE_KEY = 'croupier-theme';

const THEME_PREFS: readonly ThemePref[] = ['light', 'dark', 'system'];

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

function writeAttr(resolved: ResolvedTheme): void {
  if (typeof document === 'undefined') return;
  document.documentElement.setAttribute('data-theme', resolved);
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
    writeAttr(resolveTheme('system'));
    notify();
  };
  if (typeof mql.addEventListener === 'function') mql.addEventListener('change', onChange);
  else mql.addListener(onChange);
  // 跨标签页同步：其他页签改偏好，本页跟随
  window.addEventListener('storage', (e) => {
    if (e.key !== THEME_PREF_STORAGE_KEY) return;
    writeAttr(getResolvedTheme());
    notify();
  });
}

/**
 * 启动期初始化（global.tsx 早期调用，避免暗色用户先看到亮色闪屏）。
 * 只写属性与监听，不落盘——「从未选择过」的用户保持跟随系统档。
 */
export function initThemeAttr(): ResolvedTheme {
  const resolved = getResolvedTheme();
  writeAttr(resolved);
  ensureSystemWatch();
  return resolved;
}

/** 设置偏好：持久化 + 立即生效 + 广播订阅者（设置页三档/顶栏二档共用入口） */
export function setThemePref(pref: ThemePref): ResolvedTheme {
  ensureSystemWatch();
  try {
    window.localStorage.setItem(THEME_PREF_STORAGE_KEY, pref);
  } catch {
    // 持久化失败仍切换本页生效值
  }
  const resolved = resolveTheme(pref);
  writeAttr(resolved);
  notify();
  return resolved;
}

export function subscribeThemePref(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
