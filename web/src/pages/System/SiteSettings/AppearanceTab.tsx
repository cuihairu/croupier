/**
 * 主题偏好设置（亮色/暗色/跟随系统三档）。
 *
 * - 偏好是本机浏览器级（localStorage），不落站点配置库——主题是个人
 *   显示偏好，不是站点级数据，换浏览器/设备需各自设置；
 * - 切换即时生效：data-theme CSS 变量 + antd algorithm 同步换肤，无需
 *   刷新、无需保存按钮；
 * - 顶栏的明暗快捷按钮（ThemeToggle）写显式亮/暗偏好，与本页共享同一
 *   状态源（utils/themeMode）。
 */
import React from 'react';
import { Card, Segmented, Space, Tag, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import { useResolvedTheme, useThemePref } from '@/hooks/useThemePref';
import { setThemePref, type ThemePref } from '@/utils/themeMode';

const { Text } = Typography;

const PREF_OPTIONS: { value: ThemePref; id: string; defaultMessage: string }[] = [
  { value: 'light', id: 'pages.systemSiteSettings.appearance.light', defaultMessage: '亮色' },
  { value: 'dark', id: 'pages.systemSiteSettings.appearance.dark', defaultMessage: '暗色' },
  { value: 'system', id: 'pages.systemSiteSettings.appearance.system', defaultMessage: '跟随系统' },
];

export default function AppearanceTab() {
  const intl = useIntl();
  const pref = useThemePref();
  const resolved = useResolvedTheme();

  return (
    <Card>
      <Space direction="vertical" size="middle" style={{ maxWidth: 640, width: '100%' }}>
        <Text type="secondary">
          <FormattedMessage
            id="pages.systemSiteSettings.appearance.hint"
            defaultMessage="主题偏好保存在本机浏览器，切换即时生效、无需保存；顶栏的明暗快捷按钮与这里共享同一偏好。"
          />
        </Text>
        <div>
          <Text strong>
            <FormattedMessage
              id="pages.systemSiteSettings.appearance.mode"
              defaultMessage="界面主题"
            />
          </Text>
          <div style={{ marginTop: 12 }}>
            <Segmented
              value={pref}
              onChange={(value) => setThemePref(value as ThemePref)}
              options={PREF_OPTIONS.map((opt) => ({
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
              defaultMessage="当前生效：{mode}"
              values={{
                mode: intl.formatMessage({
                  id:
                    resolved === 'dark'
                      ? 'pages.systemSiteSettings.appearance.dark'
                      : 'pages.systemSiteSettings.appearance.light',
                  defaultMessage: resolved === 'dark' ? '暗色' : '亮色',
                }),
              }}
            />
          </Tag>
          {pref === 'system' ? (
            <Text type="secondary">
              <FormattedMessage
                id="pages.systemSiteSettings.appearance.systemHint"
                defaultMessage="跟随系统模式下，操作系统切换深浅色时界面实时跟随"
              />
            </Text>
          ) : null}
        </Space>
      </Space>
    </Card>
  );
}
