import { SunOutlined, MoonOutlined, CheckOutlined } from '@ant-design/icons';
import { Dropdown, Tooltip } from 'antd';
import type { MenuProps } from 'antd';
import React from 'react';
import { FormattedMessage, useIntl } from '@umijs/max';
import { usePresetAndResolvedTheme, useThemePref } from '@/hooks/useThemePref';
import { setThemePreset, setThemePref, type ThemePreset, type ThemePref } from '@/utils/themeMode';
import styles from './index.module.less';

/**
 * 顶栏主题菜单（月亮图标）。按「主题套 × 明暗」列出全部组合：
 * 默认蓝（亮/暗）、荷官墨粉（亮/暗）、跟随系统（在当前主题套内按系统
 * 深浅色自动切换）；当前选中态带勾。状态源在 utils/themeMode（双维偏好）。
 * 完整入口在 系统设置 → 主题（SiteSettings AppearanceTab）。
 */
type ComboKey = `${ThemePreset}-${Exclude<ThemePref, 'system'>}` | 'system';

export const ThemeToggle: React.FC = () => {
  const intl = useIntl();
  const pref = useThemePref();
  const { preset, resolved } = usePresetAndResolvedTheme();

  const presetLabel = (p: ThemePreset) =>
    intl.formatMessage({
      id: `pages.systemSiteSettings.appearance.preset.${p}`,
      defaultMessage: p === 'blue' ? '默认蓝' : '荷官墨粉',
    });
  const modeLabel = (m: Exclude<ThemePref, 'system'>) =>
    intl.formatMessage({
      id: `pages.systemSiteSettings.appearance.${m}`,
      defaultMessage: m === 'light' ? '亮色' : '暗色',
    });

  const items: MenuProps['items'] = [
    ...(
      [
        ['blue', 'light'],
        ['blue', 'dark'],
        ['inkpink', 'light'],
        ['inkpink', 'dark'],
      ] as const
    ).map(([p, m]) => {
      const key: ComboKey = `${p}-${m}`;
      const active = pref !== 'system' && preset === p && resolved === m;
      return {
        key,
        label: (
          <span className={styles.menuItemLabel}>
            <span>{`${presetLabel(p)} · ${modeLabel(m)}`}</span>
            {active ? <CheckOutlined className={styles.checkIcon} /> : null}
          </span>
        ),
      };
    }),
    { type: 'divider' },
    {
      key: 'system' as ComboKey,
      label: (
        <span className={styles.menuItemLabel}>
          <span>
            <FormattedMessage
              id="pages.systemSiteSettings.appearance.system"
              defaultMessage="跟随系统"
            />
          </span>
          {pref === 'system' ? <CheckOutlined className={styles.checkIcon} /> : null}
        </span>
      ),
    },
  ];

  const onMenuClick: MenuProps['onClick'] = ({ key }) => {
    if (key === 'system') {
      setThemePref('system');
      return;
    }
    const [p, m] = key.split('-') as [ThemePreset, Exclude<ThemePref, 'system'>];
    setThemePreset(p);
    setThemePref(m);
  };

  return (
    <Dropdown
      menu={{ items, onClick: onMenuClick, selectedKeys: [] }}
      trigger={['click']}
      placement="bottomRight"
    >
      <button
        type="button"
        className={styles.toggleBtn}
        aria-label={intl.formatMessage({
          id: 'component.themeToggle.menu',
          defaultMessage: '主题',
        })}
        aria-haspopup="menu"
      >
        {resolved === 'light' ? <MoonOutlined /> : <SunOutlined />}
      </button>
    </Dropdown>
  );
};
