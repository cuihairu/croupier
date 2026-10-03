/**
 * antd ConfigProvider 主题（按生效主题二选一），色板对齐「荷官墨粉」双套
 * token（docs/.vitepress/theme/custom.css 与 global.less 的 CSS 变量同源）：
 * - 亮色：#93394d 系按钮 + 墨黑正文 #262626；
 * - 暗色：整体提亮一档 #cf6a82 系 + 白系正文。
 *
 * 消费方两处：app.tsx 的 antd 运行时导出（首帧即正确）与 <ThemeSync>
 * （切换时实时推送）。algorithm 恒传满长数组（default/dark 各一）——
 * useAntdConfigSetter 内部按索引合并，回切亮色时才能干净覆盖。
 */
import { theme as antdTheme, type ThemeConfig } from 'antd';
import type { ResolvedTheme } from './themeMode';

export const lightAntdTheme: ThemeConfig = {
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

export const darkAntdTheme: ThemeConfig = {
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

export function getAntdThemeConfig(resolved: ResolvedTheme): ThemeConfig {
  return resolved === 'dark' ? darkAntdTheme : lightAntdTheme;
}
