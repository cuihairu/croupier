/**
 * 主题偏好设置（主题套 × 明暗两维）。
 *
 * - 主题套：默认蓝（出厂默认）/ 荷官墨粉（可选保留）；明暗档：
 *   亮色 / 暗色 / 跟随系统（在当前主题套内按系统深浅色自动切）；
 * - 偏好是本机浏览器级（localStorage），不落站点配置库——主题是个人
 *   显示偏好，不是站点级数据，换浏览器/设备需各自设置；
 * - 切换即时生效：data-preset/data-theme CSS 变量 + antd algorithm 同步
 *   换肤，无需刷新、无需保存按钮；
 * - 顶栏的主题菜单（ThemeToggle）与本页共享同一状态源（utils/themeMode）。
 */
import React from 'react';
import { Card, Segmented, Space, Tag, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import { usePresetAndResolvedTheme, useThemePref } from '@/hooks/useThemePref';
import { setThemePreset, setThemePref, type ThemePreset, type ThemePref } from '@/utils/themeMode';

const { Text } = Typography;

const PRESET_OPTIONS: { value: ThemePreset; id: string; defaultMessage: string }[] = [
  {
    value: 'blue',
    id: 'pages.systemSiteSettings.appearance.preset.blue',
    defaultMessage: '默认蓝',
  },
  {
    value: 'inkpink',
    id: 'pages.systemSiteSettings.appearance.preset.inkpink',
    defaultMessage: '荷官墨粉',
  },
];

const MODE_OPTIONS: { value: ThemePref; id: string; defaultMessage: string }[] = [
  { value: 'light', id: 'pages.systemSiteSettings.appearance.light', defaultMessage: '亮色' },
  { value: 'dark', id: 'pages.systemSiteSettings.appearance.dark', defaultMessage: '暗色' },
  { value: 'system', id: 'pages.systemSiteSettings.appearance.system', defaultMessage: '跟随系统' },
];

const MODE_TEXT: Record<Exclude<ThemePref, 'system'>, { id: string; defaultMessage: string }> = {
  light: { id: 'pages.systemSiteSettings.appearance.light', defaultMessage: '亮色' },
  dark: { id: 'pages.systemSiteSettings.appearance.dark', defaultMessage: '暗色' },
};

export default function AppearanceTab() {
  const intl = useIntl();
  const pref = useThemePref();
  const { preset, resolved } = usePresetAndResolvedTheme();

  const presetText = intl.formatMessage(
    PRESET_OPTIONS.find((opt) => opt.value === preset) ?? {
      id: 'pages.systemSiteSettings.appearance.preset.blue',
      defaultMessage: '默认蓝',
    },
  );
  const modeText = intl.formatMessage(resolved === 'dark' ? MODE_TEXT.dark : MODE_TEXT.light);

  return (
    <Card>
      <Space orientation="vertical" size="middle" style={{ maxWidth: 640, width: '100%' }}>
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.appearance.hint"
            defaultMessage="主题套与明暗偏好保存在本机浏览器，切换即时生效、无需保存；顶栏的主题菜单与这里共享同一偏好。"
          />
        </Text>
        <div>
          <Text strong>
            <FormattedMessage
              id="pages.systemSiteSettings.appearance.preset"
              defaultMessage="主题套"
            />
          </Text>
          <div style={{ marginTop: 12 }}>
            <Segmented
              data-testid="theme-preset-segmented"
              value={preset}
              onChange={(value) => setThemePreset(value as ThemePreset)}
              options={PRESET_OPTIONS.map((opt) => ({
                value: opt.value,
                label: intl.formatMessage({ id: opt.id, defaultMessage: opt.defaultMessage }),
              }))}
            />
          </div>
        </div>
        <div>
          <Text strong>
            <FormattedMessage
              id="pages.systemSiteSettings.appearance.mode"
              defaultMessage="界面主题"
            />
          </Text>
          <div style={{ marginTop: 12 }}>
            <Segmented
              data-testid="theme-mode-segmented"
              value={pref}
              onChange={(value) => setThemePref(value as ThemePref)}
              options={MODE_OPTIONS.map((opt) => ({
                value: opt.value,
                label: intl.formatMessage({ id: opt.id, defaultMessage: opt.defaultMessage }),
              }))}
            />
          </div>
        </div>
        <Space size="small" wrap>
          <Tag color={resolved === 'dark' ? 'purple' : 'gold'}>
            <FormattedMessage
              id="pages.systemSiteSettings.appearance.current"
              defaultMessage="当前生效：{preset} · {mode}"
              values={{ preset: presetText, mode: modeText }}
            />
          </Tag>
          {pref === 'system' ? (
            <Text type="secondary">
              <FormattedMessage
                id="pages.systemSiteSettings.appearance.systemHint"
                defaultMessage="跟随系统模式下，操作系统切换深浅色时界面实时跟随（在当前主题套内切换亮暗）"
              />
            </Text>
          ) : null}
        </Space>
      </Space>
    </Card>
  );
}
