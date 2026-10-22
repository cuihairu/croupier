/**
 * 设置页「主题」项（AppearanceTab）回归（双维：主题套 × 明暗）：
 * - 两组 Segmented 渲染：主题套（默认蓝/荷官墨粉）+ 明暗（亮/暗/跟随系统）；
 *   默认无偏好时 blue 选中、跟随系统选中；
 * - 点「荷官墨粉」→ 主题套持久化 + data-preset 立即生效，明暗档不受牵连；
 * - 点「暗色」→ 明暗持久化 + data-theme 立即生效，主题套不受牵连；
 * - 当前生效 Tag 文案跟随两维（「默认蓝 · 暗色」形态）；
 * - 重挂载（模拟刷新）后从 localStorage 读回，持久化不丢。
 */
import React from 'react';
import { act, fireEvent, render, screen } from '@testing-library/react';
import AppearanceTab from '../AppearanceTab';
import {
  initThemeAttr,
  readThemePref,
  readThemePreset,
  THEME_PREF_STORAGE_KEY,
  THEME_PRESET_STORAGE_KEY,
} from '@/utils/themeMode';

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
  document.documentElement.removeAttribute('data-preset');
});

function setup() {
  return render(<AppearanceTab />);
}

/** 取指定 Segmented（按 testid）当前选中项文案 */
function selectedLabelOf(testId: string): string {
  const seg = screen.getByTestId(testId);
  const selected = seg.querySelector('.ant-segmented-item-selected');
  return selected?.textContent ?? '';
}

describe('AppearanceTab（双维主题偏好）', () => {
  it('渲染两组 Segmented，默认 blue + 跟随系统选中', () => {
    setup();
    // 主题套
    expect(screen.getByText('默认蓝')).toBeInTheDocument();
    expect(screen.getByText('荷官墨粉')).toBeInTheDocument();
    // 明暗档
    expect(screen.getByText('亮色')).toBeInTheDocument();
    expect(screen.getByText('暗色')).toBeInTheDocument();
    expect(screen.getByText('跟随系统')).toBeInTheDocument();
    // 默认选中态：blue + system
    expect(selectedLabelOf('theme-preset-segmented')).toContain('默认蓝');
    expect(selectedLabelOf('theme-mode-segmented')).toContain('跟随系统');
    // 当前生效 Tag：默认蓝 · 亮色（jsdom matchMedia mock 为浅色）
    expect(screen.getByText('当前生效：默认蓝 · 亮色')).toBeInTheDocument();
  });

  it('点「荷官墨粉」：主题套持久化 + data-preset 生效，明暗档不牵连', () => {
    setup();
    act(() => {
      fireEvent.click(screen.getByText('荷官墨粉'));
    });
    expect(window.localStorage.getItem(THEME_PRESET_STORAGE_KEY)).toBe('inkpink');
    expect(document.documentElement.getAttribute('data-preset')).toBe('inkpink');
    expect(readThemePreset()).toBe('inkpink');
    // 明暗档维持跟随系统
    expect(readThemePref()).toBe('system');
    expect(selectedLabelOf('theme-mode-segmented')).toContain('跟随系统');
    // 当前生效 Tag 换套
    expect(screen.getByText('当前生效：荷官墨粉 · 亮色')).toBeInTheDocument();
  });

  it('点「暗色」：明暗持久化 + data-theme 生效，主题套不牵连', () => {
    setup();
    act(() => {
      fireEvent.click(screen.getByText('暗色'));
    });
    expect(window.localStorage.getItem(THEME_PREF_STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
    // 主题套维持默认蓝
    expect(readThemePreset()).toBe('blue');
    expect(selectedLabelOf('theme-preset-segmented')).toContain('默认蓝');
    expect(screen.getByText('当前生效：默认蓝 · 暗色')).toBeInTheDocument();
  });

  it('跟随系统档显示系统跟随提示', () => {
    setup();
    expect(screen.getByText('当前生效：默认蓝 · 亮色')).toBeInTheDocument();
    // pref=system → 提示可见
    expect(screen.getByText(/操作系统切换深浅色时界面实时跟随/)).toBeInTheDocument();
    act(() => {
      fireEvent.click(screen.getByText('亮色'));
    });
    // 显式档提示退场
    expect(screen.queryByText(/操作系统切换深浅色时界面实时跟随/)).not.toBeInTheDocument();
  });

  it('重挂载读回持久化两维偏好（刷新不丢）', () => {
    // 预填：墨粉 + 暗色（模拟他处/顶栏菜单已设置）
    window.localStorage.setItem(THEME_PRESET_STORAGE_KEY, 'inkpink');
    window.localStorage.setItem(THEME_PREF_STORAGE_KEY, 'dark');
    // 模拟应用启动链：initThemeAttr 写 data-preset
    initThemeAttr();

    setup();
    expect(selectedLabelOf('theme-preset-segmented')).toContain('荷官墨粉');
    expect(selectedLabelOf('theme-mode-segmented')).toContain('暗色');
    expect(screen.getByText('当前生效：荷官墨粉 · 暗色')).toBeInTheDocument();
  });
});
