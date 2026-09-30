/*
 * Ops/Nodes 节点详情抽屉组件测试
 *
 * 覆盖面：基本信息（健康/运维状态映射、TTL/心跳）、系统指标卡
 * （CPU/内存/磁盘与空态）、历史指标趋势（加载成功/失败/空态、loading
 * Spin、时间窗切换的 since/limit 三档）、三个动作回调、node 置空与
 * agentId 切换的重载语义。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import NodeDetailDrawer from '../NodeDetailDrawer';
import type { NodeRow } from '../shared';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  getAgentMetricsHistory: jest.fn(),
}));
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { id: string; defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
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
// Line 图表依赖 canvas，jsdom 下无渲染实现；用替身透出数据点数供断言。
jest.mock('@ant-design/charts', () => ({
  Line: ({ data }: { data?: Array<unknown> }) => (
    <div data-testid="mock-line-chart" data-points={data?.length ?? 0} />
  ),
}));

const { getAgentMetricsHistory } = jest.requireMock('@/services/api/ops') as {
  getAgentMetricsHistory: jest.Mock;
};

const HISTORY = [
  {
    timestamp: '2026-09-30T10:00:00Z',
    cpu: { usagePercent: 42.5 },
    memory: { usagePercent: 60.25 },
    disks: [{ mountPoint: '/', usagePercent: 55.75 }],
  },
  {
    timestamp: '2026-09-30T10:01:00Z',
    cpu: { usagePercent: 31.25 },
    memory: { usagePercent: 58.5 },
    disks: [{ mountPoint: '/', usagePercent: 55.8 }],
  },
];

const makeNode = (over: Partial<NodeRow> = {}): NodeRow =>
  ({
    agentId: 'agent-42',
    addr: '10.1.1.42:19091',
    functions: 3,
    healthy: true,
    expiresInSec: 30,
    gameId: 'demo',
    env: 'dev',
    type: 'agent',
    ip: '10.1.1.42',
    nodeStatus: 'active',
    lastSeen: '2026-09-30 10:00:00',
    labels: {},
    ...over,
  }) as NodeRow;

type Handlers = {
  onClose?: jest.Mock;
  onDrain?: jest.Mock;
  onUndrain?: jest.Mock;
  onRestart?: jest.Mock;
};

const renderDrawer = (node: NodeRow | null, handlers: Handlers = {}) =>
  render(
    <NodeDetailDrawer
      node={node}
      onClose={handlers.onClose ?? jest.fn()}
      onDrain={handlers.onDrain ?? jest.fn()}
      onUndrain={handlers.onUndrain ?? jest.fn()}
      onRestart={handlers.onRestart ?? jest.fn()}
    />,
  );

beforeEach(() => {
  jest.clearAllMocks();
  act(() => {
    getAgentMetricsHistory.mockResolvedValue(HISTORY);
  });
});

describe('基本信息', () => {
  test('字段渲染 + 健康节点 + TTL/心跳', async () => {
    renderDrawer(makeNode());

    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());

    expect(screen.getByText('agent-42')).toBeInTheDocument();
    expect(screen.getByText('节点详情 - agent-42')).toBeInTheDocument();
    expect(screen.getByText('10.1.1.42')).toBeInTheDocument();
    expect(screen.getByText('10.1.1.42:19091')).toBeInTheDocument();
    expect(screen.getByText('健康')).toBeInTheDocument();
    expect(screen.getByText('在线')).toBeInTheDocument();
    expect(screen.getByText('30秒')).toBeInTheDocument();
    expect(screen.getByText('2026-09-30 10:00:00')).toBeInTheDocument();
  });

  test('健康异常 + 未知运维状态兜底「未知」', async () => {
    renderDrawer(makeNode({ healthy: false, nodeStatus: 'weird' }));

    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());
    expect(screen.getByText('异常')).toBeInTheDocument();
    expect(screen.getByText('未知')).toBeInTheDocument();
  });

  test.each([
    ['active', '在线'],
    ['online', '在线'],
    ['drained', '已下线'],
    ['stale', '离线'],
    ['offline', '离线'],
  ] as const)('运维状态 %s → %s', async (status, label) => {
    renderDrawer(makeNode({ nodeStatus: status }));
    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  test('缺省字段：type/gameId/env/ip/addr/lastSeen 空值兜底', async () => {
    renderDrawer(
      makeNode({
        type: '',
        gameId: '',
        env: '',
        ip: '',
        addr: '',
        lastSeen: '',
        nodeStatus: '',
      }),
    );
    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());

    expect(screen.getAllByText('agent').length).toBeGreaterThan(0);
    // 空值兜底展示 "-"（ip/addr/gameId/env/lastSeen 各一处）。
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(5);
  });

  test('node 为 null：抽屉关闭不渲染内容', () => {
    const { container } = renderDrawer(null);
    expect(screen.queryByText('基本信息')).not.toBeInTheDocument();
    expect(container).toBeInTheDocument();
  });
});

describe('系统指标', () => {
  test('CPU/内存/磁盘卡片全量渲染', async () => {
    renderDrawer(
      makeNode({
        cpu: {
          usagePercent: 42.456,
          cores: 8,
          load1m: 1.5,
          load5m: 1.2,
          load15m: 0.9,
        },
        memory: {
          totalBytes: 16 * 1024 ** 3,
          usedBytes: 8 * 1024 ** 3,
          availableBytes: 8 * 1024 ** 3,
          usagePercent: 50.5,
          swapTotal: 2 * 1024 ** 3,
          swapUsed: 1024 ** 3,
        },
        disks: [
          {
            mountPoint: '/',
            device: 'sda1',
            fsType: 'ext4',
            totalBytes: 100 * 1024 ** 3,
            usedBytes: 55 * 1024 ** 3,
            availableBytes: 45 * 1024 ** 3,
            usagePercent: 55.25,
          },
          {
            mountPoint: '/data',
            device: '',
            fsType: '',
            totalBytes: 10 * 1024 ** 3,
            usedBytes: 5 * 1024 ** 3,
            availableBytes: 5 * 1024 ** 3,
            usagePercent: 50,
          },
        ],
      }),
    );
    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());

    expect(screen.getByText('CPU')).toBeInTheDocument();
    // 正文与 Progress 指示器可能同文，断言至少出现一次。
    expect(screen.getAllByText('42.46%').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('内存')).toBeInTheDocument();
    // 正文与 Progress 指示器可能同文，断言至少出现一次。
    expect(screen.getAllByText('50.50%').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('磁盘')).toBeInTheDocument();
    expect(screen.getByText('/')).toBeInTheDocument();
    expect(screen.getByText('/data')).toBeInTheDocument();
    // 正文与 Progress 指示器可能同文，断言至少出现一次。
    expect(screen.getAllByText('55.25%').length).toBeGreaterThanOrEqual(1);
    // 磁盘设备/文件系统空值兜底。
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(2);
    // formatBytes 渲染（16GB 总量 / 8GB 已用）。
    expect(screen.getByText('16.0 GB')).toBeInTheDocument();
    // 已用与可用均为 8 GiB → 同文两处。
    expect(screen.getAllByText('8.00 GB').length).toBeGreaterThanOrEqual(2);
  });

  test('无系统指标 → 空态文案', async () => {
    renderDrawer(makeNode({}));
    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());
    expect(screen.getByText('暂无系统指标数据')).toBeInTheDocument();
  });

  test('仅 cpu 无 memory/disks：只渲染 CPU 卡', async () => {
    renderDrawer(
      makeNode({
        cpu: { usagePercent: 12.5, cores: 4, load1m: 0, load5m: 0, load15m: 0 },
      }),
    );
    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());

    expect(screen.getByText('CPU')).toBeInTheDocument();
    expect(screen.queryByText('内存')).not.toBeInTheDocument();
    expect(screen.queryByText('磁盘')).not.toBeInTheDocument();
  });

  test('load 负载三元渲染（含 undefined 兜底链）', async () => {
    renderDrawer(
      makeNode({
        cpu: {
          usagePercent: 5,
          cores: 2,
          load1m: 0.25,
          load5m: 0.5,
          load15m: 0.75,
        },
      }),
    );
    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());
    expect(screen.getByText('0.25 / 0.50 / 0.75')).toBeInTheDocument();
  });
});

describe('历史指标趋势', () => {
  test('加载成功：三张趋势图（CPU/内存/磁盘）+ 默认 5 分钟窗 limit 50', async () => {
    renderDrawer(makeNode());
    await waitFor(() => {
      expect(getAgentMetricsHistory).toHaveBeenCalledWith('agent-42', {
        limit: 50,
        since: expect.stringMatching(/^\d{4}-\d{2}-\d{2}T/),
      });
    });

    const lines = await screen.findAllByTestId('mock-line-chart');
    expect(lines).toHaveLength(3);
    // CPU/内存趋势各 2 点（每 entry 一点）；磁盘趋势 flatMap 2×1=2 点。
    expect(lines[0]).toHaveAttribute('data-points', '2');
    expect(lines[1]).toHaveAttribute('data-points', '2');
    expect(lines[2]).toHaveAttribute('data-points', '2');
    expect(screen.getByText('CPU 使用率趋势')).toBeInTheDocument();
    expect(screen.getByText('内存使用率趋势')).toBeInTheDocument();
    expect(screen.getByText('磁盘使用率趋势')).toBeInTheDocument();
  });

  test('返回空数组 → 暂无历史数据', async () => {
    act(() => {
      getAgentMetricsHistory.mockResolvedValue([]);
    });
    renderDrawer(makeNode());
    expect(await screen.findByText('暂无历史数据')).toBeInTheDocument();
    expect(screen.queryByTestId('mock-line-chart')).not.toBeInTheDocument();
  });

  test('加载失败：console.error + 空态不崩', async () => {
    const errSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    act(() => {
      getAgentMetricsHistory.mockRejectedValue(new Error('boom'));
    });
    renderDrawer(makeNode());
    expect(await screen.findByText('暂无历史数据')).toBeInTheDocument();
    expect(errSpy).toHaveBeenCalledWith('Failed to load metrics history:', expect.any(Error));
    errSpy.mockRestore();
  });

  test('加载中：Spin 渲染', async () => {
    act(() => {
      getAgentMetricsHistory.mockImplementation(() => new Promise<typeof HISTORY>(() => {}));
    });
    renderDrawer(makeNode());
    expect(document.querySelector('.ant-spin')).toBeInTheDocument();
  });

  test('时间窗切换：1小时→limit 120、7天→limit 200、5分钟→limit 50 + 选中态', async () => {
    renderDrawer(makeNode());
    await waitFor(() => {
      expect(getAgentMetricsHistory).toHaveBeenCalledWith('agent-42', {
        limit: 50,
        since: expect.stringMatching(/^20\d{2}-/),
      });
    });

    fireEvent.click(screen.getByText('1小时'));
    await waitFor(() => {
      expect(getAgentMetricsHistory).toHaveBeenLastCalledWith('agent-42', {
        limit: 120,
        since: expect.stringMatching(/^20\d{2}-/),
      });
    });
    expect(screen.getByText('1小时').closest('button')).toHaveClass('ant-btn-primary');

    fireEvent.click(screen.getByText('7天'));
    await waitFor(() => {
      expect(getAgentMetricsHistory).toHaveBeenLastCalledWith('agent-42', {
        limit: 200,
        since: expect.stringMatching(/^20\d{2}-/),
      });
    });
    expect(screen.getByText('7天').closest('button')).toHaveClass('ant-btn-primary');

    fireEvent.click(screen.getByText('5分钟'));
    await waitFor(() => {
      expect(getAgentMetricsHistory).toHaveBeenLastCalledWith('agent-42', {
        limit: 50,
        since: expect.stringMatching(/^20\d{2}-/),
      });
    });
  });

  test('agentId 切换触发重载；node 置空清理', async () => {
    const { rerender } = renderDrawer(makeNode({ agentId: 'agent-42' }));
    await waitFor(() => {
      expect(getAgentMetricsHistory).toHaveBeenCalledWith('agent-42', expect.anything());
    });

    rerender(
      <NodeDetailDrawer
        node={makeNode({ agentId: 'agent-7' })}
        onClose={jest.fn()}
        onDrain={jest.fn()}
        onUndrain={jest.fn()}
        onRestart={jest.fn()}
      />,
    );
    await waitFor(() => {
      expect(getAgentMetricsHistory).toHaveBeenCalledWith('agent-7', expect.anything());
    });

    rerender(
      <NodeDetailDrawer
        node={null}
        onClose={jest.fn()}
        onDrain={jest.fn()}
        onUndrain={jest.fn()}
        onRestart={jest.fn()}
      />,
    );
    expect(getAgentMetricsHistory).toHaveBeenCalledTimes(2);
  });

  test('趋势 entries 缺 disks：只渲染 CPU/内存两张图', async () => {
    act(() => {
      getAgentMetricsHistory.mockResolvedValue([
        {
          timestamp: '2026-09-30T10:00:00Z',
          cpu: { usagePercent: 1 },
          memory: { usagePercent: 2 },
        },
      ]);
    });
    renderDrawer(makeNode());
    const lines = await screen.findAllByTestId('mock-line-chart');
    expect(lines).toHaveLength(2);
  });
});

describe('动作与关闭', () => {
  test('下线/恢复/重启按钮回调透传 agentId', async () => {
    const handlers: Handlers = {
      onDrain: jest.fn(),
      onUndrain: jest.fn(),
      onRestart: jest.fn(),
    };
    renderDrawer(makeNode(), handlers);
    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());

    fireEvent.click(screen.getByText('下线节点'));
    fireEvent.click(screen.getByText('恢复节点'));
    fireEvent.click(screen.getByText('重启节点'));

    expect(handlers.onDrain).toHaveBeenCalledWith('agent-42');
    expect(handlers.onUndrain).toHaveBeenCalledWith('agent-42');
    expect(handlers.onRestart).toHaveBeenCalledWith('agent-42');
  });

  test('onClose 经 Drawer 关闭图标透传', async () => {
    const onClose = jest.fn();
    renderDrawer(makeNode(), { onClose });
    await waitFor(() => expect(getAgentMetricsHistory).toHaveBeenCalled());

    fireEvent.click(document.querySelector('.ant-drawer-close') as HTMLElement);
    expect(onClose).toHaveBeenCalled();
  });
});
