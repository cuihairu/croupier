/**
 * 主题偏好的 React 绑定（useSyncExternalStore 订阅 utils/themeMode 的外部
 * store）。快照是原始字符串（稳定引用），preset/明暗任一变化都会触发重渲染。
 */
import { useSyncExternalStore } from 'react';
import {
  readThemePreset,
  readThemePref,
  resolveTheme,
  subscribeThemePref,
  DEFAULT_THEME_PRESET,
  type ResolvedTheme,
  type ThemePref,
  type ThemePreset,
} from '@/utils/themeMode';
import { getPresetAccent } from '@/utils/antdThemeConfig';

function getPrefSnapshot(): ThemePref {
  return readThemePref();
}

function getResolvedSnapshot(): ResolvedTheme {
  return resolveTheme(readThemePref());
}

function getPresetSnapshot(): ThemePreset {
  return readThemePreset();
}

/** 主题套偏好（'blue' 默认 | 'inkpink'），设置页 Segmented 直接绑定 */
export function useThemePreset(): ThemePreset {
  return useSyncExternalStore(subscribeThemePref, getPresetSnapshot, () => DEFAULT_THEME_PRESET);
}

/** 用户明暗偏好三档值（'light' | 'dark' | 'system'），设置页 Segmented 直接绑定 */
export function useThemePref(): ThemePref {
  return useSyncExternalStore(subscribeThemePref, getPrefSnapshot, () => 'system');
}

/** 当前生效主题（跟随系统档已按 prefers-color-scheme 解析） */
export function useResolvedTheme(): ResolvedTheme {
  return useSyncExternalStore(subscribeThemePref, getResolvedSnapshot, () => 'light');
}

/** 主题套 + 生效明暗一次取齐（ThemeSync 等同时消费两维的组件用） */
export function usePresetAndResolvedTheme(): { preset: ThemePreset; resolved: ResolvedTheme } {
  const preset = useThemePreset();
  const resolved = useResolvedTheme();
  return { preset, resolved };
}

/**
 * 当前主题套的主色 hex（蓝 #1677ff / 墨粉 #93394d）。供无法走 CSS 变量的
 * 场景使用：SVG 属性（fill/stroke 不认 var()）、图表库序列色、antd Tag
 * 的 color prop 等需要真色值的入口。CSS 场景请用 var(--brand-*)。
 */
export function useThemeAccent(): string {
  const preset = useThemePreset();
  return getPresetAccent(preset);
}
