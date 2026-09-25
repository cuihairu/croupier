/**
 * BUG-003 回归：LB 监控「归属 vs LB 对账」卡片。
 *
 * 原实现用 `@ant-design/charts` 的 `Gauge` 展示归属率。实测该路径拿不到百分数：
 * `@ant-design/plots` 2.6.8 的 gauge adaptor 把 `data: percent` 改写成
 * `data: { value: percent }`，而 `@antv/g2` 5.4.8 的 Gauge mark 只解构
 * `{name, target, total, percent, thresholds}`——`value` 不在其中，于是
 * target/total/percent 全为 undefined，通道 y 退化成 undefined/NaN，
 * 指针与读数都不会反映真实归属率。
 *
 * 现在用 antd `Progress type="dashboard"` 呈现。本用例锁两件事：
 * 1. 归属率换算的边界（0 agent、0 backend、超出 100% 的截断）；
 * 2. 组件实际渲染出可读的数字读数（而不是空白的图表画布）。
 */
import React from 'react';
import { act, render, screen, waitFor } from '@testing-library/react';

// 组件内含 30s 轮询 + 多个异步加载，渲染与断言都偏慢
jest.setTimeout(60000);

jest.mock('@ant-design/plots', () => {
  throw new Error(
    'BUG-003 回归：LBMonitor 不得再依赖 @ant-design/plots 的 Gauge（gauge adaptor 会把 percent 丢成 {value}）',
  );
});

const mockListOpsNodes = jest.fn();
const mockQueryLbStats = jest.fn();
const mockFetchClusterInfo = jest.fn();

jest.mock('@/services/api/ops', () => ({
  listOpsNodes: (...args: unknown[]) => mockListOpsNodes(...args),
  queryLbStats: () => mockQueryLbStats(),
  fetchClusterInfo: () => mockFetchClusterInfo(),
}));
jest.mock('@/services/api/lbStats', () => ({
  // 必须透传入参：组件按 query 内容区分「会话数」与「server_status」两次调用
  queryLbStats: (args: { query: string }) => mockQueryLbStats(args),
}));

// Line 图表依赖 canvas，jsdom 下无渲染实现；与被测的对账卡片无关，替换为占位。
jest.mock('@ant-design/charts', () => ({
  Line: () => <div data-testid="mock-line-chart" />,
  Gauge: () => {
    throw new Error('BUG-003 回归：不得再使用 @ant-design/charts 的 Gauge');
  },
}));

import { ownershipRatioPercent } from '../index';
import type { OpsNode } from '@/services/api/ops';

function makeNode(id: string): OpsNode {
  return { agentId: id } as unknown as OpsNode;
}

describe('ownershipRatioPercent（BUG-003 换算边界）', () => {
  it('无 agent 时为 0，不返回 NaN', () => {
    expect(ownershipRatioPercent([], ['a', 'b'])).toBe(0);
    expect(ownershipRatioPercent([], [])).toBe(0);
  });

  it('无 backend 时分母取 1，避免除零', () => {
    expect(ownershipRatioPercent([makeNode('1'), makeNode('2')], [])).toBe(100);
    expect(Number.isNaN(ownershipRatioPercent([makeNode('1')], []))).toBe(false);
  });

  it('正常比例按 0-100 换算并四舍五入', () => {
    expect(ownershipRatioPercent([makeNode('1')], ['a', 'b', 'c', 'd'])).toBe(25);
    expect(ownershipRatioPercent([makeNode('1'), makeNode('2')], ['a', 'b'])).toBe(100);
    // 1/3 → 33.33… → 33
    expect(ownershipRatioPercent([makeNode('1')], ['a', 'b', 'c'])).toBe(33);
  });

  it('agent 数多于 backend 数时截断到 100', () => {
    const nodes = [makeNode('1'), makeNode('2'), makeNode('3')];
    expect(ownershipRatioPercent(nodes, ['only-one'])).toBe(100);
  });

  it('结果始终落在 0-100 整数区间（任意输入组合）', () => {
    for (let n = 0; n <= 5; n += 1) {
      for (let b = 0; b <= 5; b += 1) {
        const pct = ownershipRatioPercent(
          Array.from({ length: n }, (_, i) => makeNode(`n${i}`)),
          Array.from({ length: b }, (_, i) => `b${i}`),
        );
        expect(Number.isInteger(pct)).toBe(true);
        expect(pct).toBeGreaterThanOrEqual(0);
        expect(pct).toBeLessThanOrEqual(100);
        expect(Number.isNaN(pct)).toBe(false);
      }
    }
  });
});

describe('LBMonitor 对账卡片渲染（BUG-003）', () => {
  /**
   * 造一条 Prometheus 即时查询结果行。
   *
   * 时间戳必须取「当前时刻」：组件把采样并入 10 分钟滚动窗口
   *（appendHistory 丢弃 cutoff 之前的点），写死历史时间会被整批过滤掉，
   * 后端去重数组因而为空。
   */
  const nowSec = () => Math.floor(Date.now() / 1000);
  const promRow = (proxy: string, value: number) => ({
    metric: { proxy },
    value: [nowSec(), String(value)],
  });

  beforeEach(() => {
    jest.clearAllMocks();
    // nodes 决定归属数（分子）
    mockListOpsNodes.mockResolvedValue({
      nodes: [makeNode('a1'), makeNode('a2')],
    });
    // lbStats 未启用时组件直接走空态（不渲染对账卡片），必须开启
    mockFetchClusterInfo.mockResolvedValue({ lbStats: { enabled: true } });
    // 后端数（分母）由 sessions 查询结果去重得出：这里给 4 个 backend
    mockQueryLbStats.mockImplementation((args: { query: string }) => {
      if (args.query.includes('haproxy_backend_current_sessions')) {
        return Promise.resolve({
          data: {
            result: [
              promRow('web-1', 3),
              promRow('web-2', 2),
              promRow('web-3', 1),
              promRow('web-4', 5),
            ],
          },
        });
      }
      // haproxy_server_status：per-state 指标族，UP 且值为 1 = 健康
      return Promise.resolve({
        data: {
          result: ['web1', 'web2', 'web3', 'web4'].map((server) => ({
            metric: { server, state: 'UP' },
            value: [nowSec(), '1'],
          })),
        },
      });
    });
  });

  it('渲染出归属率读数与归属/后端计数文本，不出现空白画布', async () => {
    const LBMonitor = (await import('../index')).default;
    // 组件在 mount 后异步拉取 clusterInfo/lbStats/nodes；用 act 包裹并 flush
    // 这些 promise，避免状态更新落在 act 之外。
    let container: HTMLElement = document.body;
    await act(async () => {
      ({ container } = render(<LBMonitor />));
    });

    // 归属率文本（Progress 之外的可读读数）
    await waitFor(() => {
      expect(screen.getByText(/归属\s*2\s*\/\s*LB 后端\s*4/)).toBeInTheDocument();
    });
    // Progress 的读数：2 agent / 4 backend → 50%
    await waitFor(() => {
      expect(screen.getByText('50%')).toBeInTheDocument();
    });
    expect(container.textContent).not.toMatch(/NaN|Infinity/);
  });

  it('无 agent 时归属率为 0%，不出现 NaN/Infinity', async () => {
    mockListOpsNodes.mockResolvedValue({ nodes: [] });
    const LBMonitor = (await import('../index')).default;
    let container: HTMLElement = document.body;
    await act(async () => {
      ({ container } = render(<LBMonitor />));
    });

    await waitFor(() => {
      expect(screen.getByText(/归属\s*0\s*\/\s*LB 后端\s*4/)).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByText('0%')).toBeInTheDocument();
    });
    expect(container.textContent).not.toMatch(/NaN|Infinity/);
  });
});
