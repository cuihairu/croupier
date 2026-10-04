/**
 * ThemeToggle 顶栏主题菜单回归：月亮按钮点开「主题套 × 明暗」全组合菜单
 * （默认蓝/荷官墨粉 × 亮/暗 + 跟随系统），点击写两维偏好（即时生效 +
 * 持久化），当前选中态带勾（aria/勾图标随生效组合翻转）。偏好状态机在
 * utils/themeMode（双维）。
 */
import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { ThemeToggle } from '../index';
import {
  initThemeAttr,
  readThemePreset,
  readThemePref,
  THEME_PREF_STORAGE_KEY,
  THEME_PRESET_STORAGE_KEY,
} from '@/utils/themeMode';

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

// ThemeToggle 用 useIntl/FormattedMessage 取菜单文案（文案来自 locale，
// 断言用 defaultMessage）
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

beforeEach(() => {
  window.localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.removeAttribute('data-preset');
});

function openMenu() {
  const btn = screen.getByRole('button', { name: '主题' });
  act(() => {
    fireEvent.mouseEnter(btn);
    fireEvent.click(btn);
  });
}

describe('ThemeToggle（主题菜单）', () => {
  it('点开菜单列出全部主题套 × 明暗组合与跟随系统', async () => {
    render(<ThemeToggle />);
    openMenu();
    // antd Dropdown 渲染进 portal，用 document 查询
    const bodyText = await waitFor(() => {
      const t = document.body.textContent;
      if (!t?.includes('跟随系统')) throw new Error('not ready');
      return t;
    });
    expect(bodyText).toContain('默认蓝 · 亮色');
    expect(bodyText).toContain('默认蓝 · 暗色');
    expect(bodyText).toContain('荷官墨粉 · 亮色');
    expect(bodyText).toContain('荷官墨粉 · 暗色');
  });

  it('默认（无偏好）：跟随系统带选中勾，四个组合无勾', async () => {
    render(<ThemeToggle />);
    openMenu();
    await waitFor(() => expect(document.querySelectorAll('.ant-dropdown-menu-item')).toBeTruthy());
    const items = Array.from(document.querySelectorAll('.ant-dropdown-menu-item'));
    const checked = items.filter((el) => el.querySelector('.anticon-check'));
    expect(checked).toHaveLength(1);
    expect(checked[0].textContent).toContain('跟随系统');
  });

  it('点「荷官墨粉 · 暗色」：两维偏好持久化 + data 属性立即生效', async () => {
    render(<ThemeToggle />);
    openMenu();
    const item = await waitFor(() => {
      const el = Array.from(document.querySelectorAll('.ant-dropdown-menu-item')).find((n) =>
        n.textContent?.includes('荷官墨粉 · 暗色'),
      );
      if (!el) throw new Error('not ready');
      return el;
    });
    act(() => {
      fireEvent.click(item);
    });
    expect(window.localStorage.getItem(THEME_PRESET_STORAGE_KEY)).toBe('inkpink');
    expect(window.localStorage.getItem(THEME_PREF_STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.getAttribute('data-preset')).toBe('inkpink');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('点「默认蓝 · 亮色」后选中态迁移（勾唯一且指向新组合）', async () => {
    window.localStorage.setItem(THEME_PRESET_STORAGE_KEY, 'inkpink');
    window.localStorage.setItem(THEME_PREF_STORAGE_KEY, 'dark');
    render(<ThemeToggle />);
    openMenu();
    let item = await waitFor(() => {
      const el = Array.from(document.querySelectorAll('.ant-dropdown-menu-item')).find((n) =>
        n.textContent?.includes('荷官墨粉 · 暗色'),
      );
      if (!el) throw new Error('not ready');
      return el;
    });
    expect(item.querySelector('.anticon-check')).toBeTruthy();

    // 点「默认蓝 · 亮色」完成迁移
    const target = await waitFor(() => {
      const el = Array.from(document.querySelectorAll('.ant-dropdown-menu-item')).find((n) =>
        n.textContent?.includes('默认蓝 · 亮色'),
      );
      if (!el) throw new Error('not ready');
      return el;
    });
    act(() => {
      fireEvent.click(target);
    });
    // 重新展开（菜单随选择关闭），确认勾指向默认蓝 · 亮色
    openMenu();
    await waitFor(() => {
      const items = Array.from(document.querySelectorAll('.ant-dropdown-menu-item'));
      const checked = items.filter((el) => el.querySelector('.anticon-check'));
      if (checked.length !== 1 || !checked[0].textContent?.includes('默认蓝 · 亮色')) {
        throw new Error('not ready');
      }
    });
    expect(readThemePreset()).toBe('blue');
    expect(readThemePref()).toBe('light');
  });

  it('点「跟随系统」只改明暗档，保留当前主题套', async () => {
    window.localStorage.setItem(THEME_PRESET_STORAGE_KEY, 'inkpink');
    // 模拟应用启动链：app.tsx 渲染前 initThemeAttr() 把持久化套写上 data-preset
    initThemeAttr();
    render(<ThemeToggle />);
    openMenu();
    const item = await waitFor(() => {
      const el = Array.from(document.querySelectorAll('.ant-dropdown-menu-item')).find((n) =>
        n.textContent?.includes('跟随系统'),
      );
      if (!el) throw new Error('not ready');
      return el;
    });
    act(() => {
      fireEvent.click(item);
    });
    expect(readThemePreset()).toBe('inkpink');
    expect(readThemePref()).toBe('system');
    // jsdom matchMedia mock 为浅色 → 生效 light
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
    expect(document.documentElement.getAttribute('data-preset')).toBe('inkpink');
  });
});
