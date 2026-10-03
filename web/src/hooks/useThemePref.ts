/**
 * 主题偏好的 React 绑定（useSyncExternalStore 订阅 utils/themeMode 的外部
 * store）。快照是原始字符串（稳定引用），pref/系统色任一变化都会触发重渲染。
 */
import { useSyncExternalStore } from 'react';
import {
  readThemePref,
  resolveTheme,
  subscribeThemePref,
  type ResolvedTheme,
  type ThemePref,
} from '@/utils/themeMode';

function getPrefSnapshot(): ThemePref {
  return readThemePref();
}

function getResolvedSnapshot(): ResolvedTheme {
  return resolveTheme(readThemePref());
}

/** 用户偏好三档值（'light' | 'dark' | 'system'），设置页 Segmented 直接绑定 */
export function useThemePref(): ThemePref {
  return useSyncExternalStore(subscribeThemePref, getPrefSnapshot, () => 'system');
}

/** 当前生效主题（跟随系统档已按 prefers-color-scheme 解析） */
export function useResolvedTheme(): ResolvedTheme {
  return useSyncExternalStore(subscribeThemePref, getResolvedSnapshot, () => 'light');
}
