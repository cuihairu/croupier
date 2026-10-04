/**
 * antd ConfigProvider 主题：主题套（preset）× 生效明暗（resolved）四套。
 *
 * - blue（拂晓蓝，出厂默认）：只给 colorPrimary，其余 token 走 antd 6
 *   默认（#1677ff 种子派生 hover/active，暗色由 darkAlgorithm 提亮）——
 *   即「荷官墨粉」改造前的原默认观感；
 * - inkpink（荷官墨粉，可选保留）：双套 token 与 docs/.vitepress/theme/
 *   custom.css、global.less 的 CSS 变量同源：
 *   亮色 #93394d 系按钮 + 墨黑正文 #262626；暗色提亮一档 #cf6a82 系。
 *
 * 消费方两处：app.tsx 的 antd 运行时导出（首帧即正确）与 <ThemeSync>
 * （切换时实时推送）。algorithm 恒传满长数组（default/dark 各一）——
 * useAntdConfigSetter 内部按索引合并，回切亮色时才能干净覆盖。
 */
import { theme as antdTheme, type ThemeConfig } from 'antd';
import type { ThemePreset, ResolvedTheme } from './themeMode';

export const blueLightAntdTheme: ThemeConfig = {
  algorithm: [antdTheme.defaultAlgorithm],
  token: {
    colorPrimary: '#1677ff',
  },
};

export const blueDarkAntdTheme: ThemeConfig = {
  algorithm: [antdTheme.darkAlgorithm],
  token: {
    colorPrimary: '#1677ff',
  },
};

export const inkPinkLightAntdTheme: ThemeConfig = {
  algorithm: [antdTheme.defaultAlgorithm],
  token: {
    colorPrimary: '#93394d',
    colorPrimaryHover: '#b8556b',
    colorPrimaryActive: '#7a2b3e',
    colorPrimaryTextHover: '#b8556b',
    colorText: '#262626',
    colorTextSecondary: 'rgba(38, 38, 38, 0.65)',
    colorBgLayout: '#f3f5f7',
    colorBgContainer: 'rgba(255, 255, 255, 0.9)',
    colorBorder: '#e6eaf0',
  },
};

export const inkPinkDarkAntdTheme: ThemeConfig = {
  algorithm: [antdTheme.darkAlgorithm],
  token: {
    colorPrimary: '#cf6a82',
    colorPrimaryHover: '#e58aa0',
    colorPrimaryActive: '#b8556b',
    colorPrimaryTextHover: '#e58aa0',
    colorText: 'rgba(255, 255, 255, 0.87)',
    colorTextSecondary: 'rgba(235, 235, 245, 0.6)',
    colorBgLayout: '#1a1a2e',
    colorBgContainer: 'rgba(30, 30, 50, 0.92)',
    colorBorder: 'rgba(255, 255, 255, 0.14)',
  },
};

export function getAntdThemeConfig(preset: ThemePreset, resolved: ResolvedTheme): ThemeConfig {
  if (preset === 'inkpink')
    return resolved === 'dark' ? inkPinkDarkAntdTheme : inkPinkLightAntdTheme;
  return resolved === 'dark' ? blueDarkAntdTheme : blueLightAntdTheme;
}

/** 主题套主色 hex（SVG 属性/图表序列色/Tag color 等非 CSS 场景用） */
export function getPresetAccent(preset: ThemePreset): string {
  return preset === 'inkpink' ? '#93394d' : '#1677ff';
}
