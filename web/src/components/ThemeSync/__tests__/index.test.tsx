/**
 * ThemeSync 主题生效器回归：主题套（preset）× 生效明暗（resolved）任一
 * 变化时，把对应 algorithm/token 推给 useAntdConfigSetter（antd 组件换肤）
 * 并同步静态方法配置（ConfigProvider.config —— message/notification/
 * Modal.confirm 不在 React 树内）。algorithm 恒传满长数组，来回切换不
 * 残留旧算法。同时把 ProLayout 壳层 token（navTheme/sider/header/bgLayout）
 * 经 setInitialState 推进 layout 运行时配置（defaultSettings 静态亮色，
 * 暗色不推会漏亮壳）。
 */
import React from 'react';
import { act, render } from '@testing-library/react';
import { theme as antdTheme } from 'antd';

const setAntdConfig = jest.fn();
const setInitialState = jest.fn();
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
  useModel: () => ({ setInitialState }),
}));

import { ConfigProvider } from 'antd';
import { ThemeSync } from '../index';
import { setThemePreset, setThemePref } from '@/utils/themeMode';

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
  document.documentElement.removeAttribute('data-preset');
  setAntdConfig.mockClear();
  setInitialState.mockClear();
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

/** 取 setInitialState 最近一次收到的 updater，应用在给定 prev 上看结果 */
function applyLastStateUpdate(prev: Record<string, unknown>) {
  const last = setInitialState.mock.calls[setInitialState.mock.calls.length - 1];
  const updater = last[0] as (p: Record<string, unknown>) => Record<string, unknown>;
  return updater(prev);
}

describe('ThemeSync', () => {
  it('默认（blue 亮色）：推送 defaultAlgorithm + 拂晓蓝主色，并同步静态配置', () => {
    render(<ThemeSync />);
    expect(setAntdConfig).toHaveBeenCalledTimes(1);
    const merged = applyLastUpdate({ theme: { token: { borderRadius: 8 } } });
    expect(merged.theme.algorithm).toEqual([antdTheme.defaultAlgorithm]);
    expect(merged.theme.token.colorPrimary).toBe('#1677ff');
    // 既有 token（config.ts 的 borderRadius）不被清掉
    expect(merged.theme.token.borderRadius).toBe(8);
    expect(staticConfigSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        theme: expect.objectContaining({ algorithm: [antdTheme.defaultAlgorithm] }),
      }),
    );
  });

  it('墨粉套切到暗色：darkAlgorithm + 提亮色板；再切回亮色不残留 dark', () => {
    render(<ThemeSync />);
    act(() => {
      setThemePreset('inkpink');
    });
    expect(applyLastUpdate({ theme: {} }).theme.token.colorPrimary).toBe('#93394d');

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

  it('同明暗换主题套：blue↔inkpink 主色切换，algorithm 不变', () => {
    render(<ThemeSync />);
    act(() => {
      setThemePreset('inkpink');
    });
    expect(applyLastUpdate({ theme: {} }).theme.token.colorPrimary).toBe('#93394d');

    act(() => {
      setThemePreset('blue');
    });
    expect(applyLastUpdate({ theme: {} }).theme.token.colorPrimary).toBe('#1677ff');
    const merged = applyLastUpdate({ theme: { token: { borderRadius: 8 } } });
    expect(merged.theme.algorithm).toEqual([antdTheme.defaultAlgorithm]);
  });

  it('壳层 token 随明暗切换推进 initialState.settings（ProLayout 暗色不漏亮壳）', () => {
    render(<ThemeSync />);
    // 默认 blue 亮色：settings 拿亮色壳，title 等既有字段保留
    let settings = applyLastStateUpdate({
      settings: { title: 'Croupier', token: { legacy: true } },
    }).settings as Record<string, unknown>;
    expect(settings.navTheme).toBe('light');
    expect(settings.title).toBe('Croupier');
    let token = settings.token as Record<string, unknown>;
    expect(token.bgLayout).toBe('#f3f5f7');
    expect((token.sider as Record<string, unknown>).colorTextMenu).toBe('#4b5563');

    act(() => {
      setThemePref('dark');
    });
    settings = applyLastStateUpdate({
      settings: { title: 'Croupier', token: { legacy: true } },
    }).settings as Record<string, unknown>;
    expect(settings.navTheme).toBe('realDark');
    token = settings.token as Record<string, unknown>;
    expect(token.bgLayout).toBe('#1a1a2e');
    // 旧 token 字段不残留：整体替换，不做深合并
    expect((token.sider as Record<string, unknown>).colorTextMenu).toBe(
      'rgba(255, 255, 255, 0.65)',
    );
    expect(token.legacy).toBeUndefined();

    act(() => {
      setThemePref('light');
    });
    settings = applyLastStateUpdate({ settings: {} }).settings as Record<string, unknown>;
    expect(settings.navTheme).toBe('light');
    expect(
      ((settings.token as Record<string, unknown>).sider as Record<string, unknown>).colorTextMenu,
    ).toBe('#4b5563');
  });

  it('墨粉套亮色壳层 colorPrimary 换 #93394d，暗色换 #cf6a82', () => {
    render(<ThemeSync />);
    act(() => {
      setThemePreset('inkpink');
    });
    let settings = applyLastStateUpdate({ settings: {} }).settings as Record<string, unknown>;
    expect(settings.colorPrimary).toBe('#93394d');
    act(() => {
      setThemePref('dark');
    });
    settings = applyLastStateUpdate({ settings: {} }).settings as Record<string, unknown>;
    expect(settings.colorPrimary).toBe('#cf6a82');
    expect(settings.navTheme).toBe('realDark');
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
