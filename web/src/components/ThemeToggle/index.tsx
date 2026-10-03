import { SunOutlined, MoonOutlined } from '@ant-design/icons';
import { Tooltip } from 'antd';
import React from 'react';
import { useResolvedTheme } from '@/hooks/useThemePref';
import { setThemePref } from '@/utils/themeMode';
import styles from './index.module.less';

/**
 * 顶栏明暗快捷切换（二档）。偏好状态机（三档：亮/暗/跟随系统）在
 * utils/themeMode；此处点击写入显式亮/暗偏好——会覆盖「跟随系统」。
 * 三档完整入口在 系统设置 → 主题（SiteSettings AppearanceTab）。
 */
export const ThemeToggle: React.FC = () => {
  const resolved = useResolvedTheme();
  const toggle = () => setThemePref(resolved === 'light' ? 'dark' : 'light');

  return (
    <Tooltip title={resolved === 'light' ? '切换到暗色模式' : '切换到亮色模式'}>
      <button
        type="button"
        className={styles.toggleBtn}
        onClick={toggle}
        aria-label={resolved === 'light' ? '启用暗色模式' : '启用亮色模式'}
      >
        {resolved === 'light' ? <MoonOutlined /> : <SunOutlined />}
      </button>
    </Tooltip>
  );
};
