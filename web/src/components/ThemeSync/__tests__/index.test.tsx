/**
 * ThemeSync 主题生效器回归：生效主题变化时把对应 algorithm/token 推给
 * useAntdConfigSetter（antd 组件换肤）并同步静态方法配置
 * （ConfigProvider.config —— message/notification/Modal.confirm 不在
 * React 树内）。algorithm 恒传满长数组，来回切换不残留旧算法。
 */
import React from 'react';
import { act, render } from '@testing-library/react';
import { theme as antdTheme } from 'antd';

const setAntdConfig = jest.fn();
// 复刻 umi 模板形态：useAntdConfigSetter 每次渲染返回**新引用**的包装函数
// （AntdProvider 内联创建，未 memo）。默认转发到共享 spy；回归用例把实现
// 指到「推送即触发上层重渲染」的形态（见无限循环用例）。
let setterImpl: (...args: unknown[]) => void = (...args: unknown[]) =>
  void setAntdConfig(...(args as Parameters<typeof setAntdConfig>));
jest.mock('@umijs/max', () => ({
  useAntdConfigSetter:
    () =>
    (...args: unknown[]) =>
      setterImpl(...args),
}));

import { ConfigProvider } from 'antd';
import { ThemeSync } from '../index';
import { setThemePref } from '@/utils/themeMode';

// setupTests 的 localStorage 是无状态 jest.fn 桩；切换断言依赖真实读写语义
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

const staticConfigSpy = jest.spyOn(ConfigProvider, 'config').mockImplementation(() => {});

beforeEach(() => {
  window.localStorage.clear();
  document.documentElement.removeAttribute('data-theme');
  setAntdConfig.mockClear();
  staticConfigSpy.mockClear();
  setterImpl = (...args: unknown[]) =>
    void setAntdConfig(...(args as Parameters<typeof setAntdConfig>));
});

/** 取 setter 最近一次收到的 updater，应用在给定 prev 上看结果 */
function applyLastUpdate(prev: Record<string, unknown>) {
  const last = setAntdConfig.mock.calls[setAntdConfig.mock.calls.length - 1];
  const updater = last[0] as (p: Record<string, unknown>) => Record<string, unknown>;
  return updater(prev);
}

describe('ThemeSync', () => {
  it('亮色生效：推送 defaultAlgorithm + 亮色 token，并同步静态配置', () => {
    render(<ThemeSync />);
    expect(setAntdConfig).toHaveBeenCalledTimes(1);
    const merged = applyLastUpdate({ theme: { token: { borderRadius: 8 } } });
    expect(merged.theme.algorithm).toEqual([antdTheme.defaultAlgorithm]);
    expect(merged.theme.token.colorPrimary).toBe('#93394d');
    // 既有 token（config.ts 的 borderRadius）不被清掉
    expect(merged.theme.token.borderRadius).toBe(8);
    expect(staticConfigSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        theme: expect.objectContaining({ algorithm: [antdTheme.defaultAlgorithm] }),
      }),
    );
  });

  it('切到暗色：darkAlgorithm + 提亮色板；再切回亮色不残留 dark', () => {
    render(<ThemeSync />);
    act(() => {
      setThemePref('dark');
    });
    let merged = applyLastUpdate({ theme: {} });
    expect(merged.theme.algorithm).toEqual([antdTheme.darkAlgorithm]);
    expect(merged.theme.token.colorPrimary).toBe('#cf6a82');

    act(() => {
      setThemePref('light');
    });
    merged = applyLastUpdate({ theme: {} });
    expect(merged.theme.algorithm).toEqual([antdTheme.defaultAlgorithm]);
  });

  it('setter 引用不稳定且推送触发上层重渲染：不陷入无限循环（线上白屏回归）', () => {
    // 模拟 AntdProvider 真实形态：每次渲染产生新 setter 引用，推送即引发
    // 上层 state 更新（重渲染）。旧实现把不稳定 setter 放进 effect 依赖，
    // 会「推送 → 重渲染 → 新引用 → 再推送」直到 React 抛 Maximum update
    // depth exceeded、整树卸载（线上全站白屏）。
    let renderCount = 0;
    const Harness: React.FC = () => {
      const [, bump] = React.useReducer((c: number) => c + 1, 0);
      renderCount += 1;
      setterImpl = () => {
        setAntdConfig();
        bump();
      };
      return <ThemeSync />;
    };
    const { unmount } = render(<Harness />);
    expect(renderCount).toBe(2); // 首渲染 + 首次推送引发的一次
    expect(setAntdConfig).toHaveBeenCalledTimes(1);
    unmount();
  });
});
