/**
 * 设置页「主题」项（AppearanceTab）回归：
 * - 三档选项（亮色/暗色/跟随系统）渲染，默认无偏好时选中「跟随系统」；
 * - 点击暗色 → localStorage 显式偏好 + data-theme 立即生效（无需保存按钮）；
 * - 点回「跟随系统」→ 偏好为 system；
 * - 重挂载（模拟刷新）后从 localStorage 读回，持久化不丢。
 */
import React from 'react';
import { act, fireEvent, render, screen } from '@testing-library/react';
import AppearanceTab from '../AppearanceTab';
import { readThemePref, THEME_PREF_STORAGE_KEY } from '@/utils/themeMode';

// setupTests 的 localStorage 是无状态 jest.fn 桩；持久化断言需要真实读写语义
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

// @umijs/max 无 jest 映射，按仓内页面测试惯例 mock（见 index.test.tsx）
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
    values,
  }: {
    defaultMessage?: string;
    values?: Record<string, React.ReactNode>;
  }) => {
    let text = defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, String(v));
    }
    return <>{text}</>;
  },
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
}));

beforeEach(() => {
  window.localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
});

function setup() {
  return render(<AppearanceTab />);
}

describe('AppearanceTab', () => {
  it('渲染三档选项，默认选中「跟随系统」', () => {
    setup();
    expect(screen.getByText('亮色')).toBeInTheDocument();
    expect(screen.getByText('暗色')).toBeInTheDocument();
    expect(screen.getByText('跟随系统')).toBeInTheDocument();
    // Segmented 选中态：跟随系统所在 item 带 ant-segmented-item-selected
    const selected = document.querySelector('.ant-segmented-item-selected');
    expect(selected?.textContent).toContain('跟随系统');
  });

  it('点击暗色：即时生效 + 持久化', () => {
    setup();
    act(() => {
      fireEvent.click(screen.getByText('暗色'));
    });
    expect(window.localStorage.getItem(THEME_PREF_STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    expect(screen.getByText('当前生效：暗色')).toBeInTheDocument();
  });

  it('点击亮色后再点「跟随系统」回默认档', () => {
    setup();
    act(() => {
      fireEvent.click(screen.getByText('亮色'));
    });
    expect(window.localStorage.getItem(THEME_PREF_STORAGE_KEY)).toBe('light');
    act(() => {
      fireEvent.click(screen.getByText('跟随系统'));
    });
    expect(readThemePref()).toBe('system');
    // jsdom matchMedia mock 为浅色 → 生效仍是 light
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');
  });

  it('重挂载读回持久化偏好（刷新不丢）', () => {
    const first = setup();
    act(() => {
      fireEvent.click(first.getByText('暗色'));
    });
    first.unmount();

    const second = setup();
    const selected = document.querySelector('.ant-segmented-item-selected');
    expect(selected?.textContent).toContain('暗色');
    expect(second.getByText('当前生效：暗色')).toBeInTheDocument();
  });
});
