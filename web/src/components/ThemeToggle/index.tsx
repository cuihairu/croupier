import { SunOutlined, MoonOutlined } from '@ant-design/icons';
import { Tooltip } from 'antd';
import React, { useEffect, useState } from 'react';
import styles from './index.module.less';

type ThemeMode = 'light' | 'dark';

export const ThemeToggle: React.FC = () => {
  const [mode, setMode] = useState<ThemeMode>(() => {
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem('croupier-theme') as ThemeMode | null;
      if (stored) return stored;
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }
    return 'light';
  });

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', mode);
    localStorage.setItem('croupier-theme', mode);
  }, [mode]);

  const toggle = () => setMode((m) => (m === 'light' ? 'dark' : 'light'));

  return (
    <Tooltip title={mode === 'light' ? '切换到暗色模式' : '切换到亮色模式'}>
      <button
        type="button"
        className={styles.toggleBtn}
        onClick={toggle}
        aria-label={mode === 'light' ? '启用暗色模式' : '启用亮色模式'}
      >
        {mode === 'light' ? <MoonOutlined /> : <SunOutlined />}
      </button>
    </Tooltip>
  );
};