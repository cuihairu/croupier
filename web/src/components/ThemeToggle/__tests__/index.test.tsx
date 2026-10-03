/**
 * ThemeToggle 顶栏快捷切换回归：点击写显式亮/暗偏好（覆盖「跟随系统」），
 * 图标/aria 随生效主题翻转；偏好状态源在 utils/themeMode（三档）。
 */
import React from 'react';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { ThemeToggle } from '../index';
import { readThemePref, THEME_PREF_STORAGE_KEY } from '@/utils/themeMode';

// setupTests 的 localStorage 是无状态 jest.fn 桩；这里需要真实读写语义
const storageBacking = new Map<string, string>();
Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: (k: string) => (storageBacking.has(k) ? (storageBacking.get(k) ?? null) : null),
    setItem: (k: string, v: string) => void storageBacking.set(k, v),
    removeItem: (k: string) => void storageBacking.delete(k),
    clear: () => storageBacking.clear(),
  },
  writable: true,
  configurable: true,
});

beforeEach(() => {
  window.localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
});

function setup() {
  return render(
    <div data-testid="host">
      <ThemeToggle />
    </div>,
  );
}

describe('ThemeToggle', () => {
  it('默认（无偏好）显示亮色态，aria 为「启用暗色模式」', () => {
    setup();
    expect(screen.getByRole('button', { name: '启用暗色模式' })).toBeInTheDocument();
  });

  it('点击切换到暗色：偏好显式 dark、属性生效、aria 翻转', () => {
    setup();
    const btn = screen.getByRole('button', { name: '启用暗色模式' });
    act(() => {
      fireEvent.click(btn);
    });
    expect(window.localStorage.getItem(THEME_PREF_STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(readThemePref()).toBe('dark');
    expect(screen.getByRole('button', { name: '启用亮色模式' })).toBeInTheDocument();
  });

  it('再点回亮色', () => {
    setup();
    act(() => {
      fireEvent.click(screen.getByRole('button', { name: '启用暗色模式' }));
    });
    // aria-label 更新依赖重渲染，两跳点击必须分开 act
    act(() => {
      fireEvent.click(screen.getByRole('button', { name: '启用亮色模式' }));
    });
    expect(window.localStorage.getItem(THEME_PREF_STORAGE_KEY)).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });
});
