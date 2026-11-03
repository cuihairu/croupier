/**
 * OPS-011 运维大屏组件单测。
 *
 * 锁定契约：
 * - 挂载链：listOpsNodes + fetchOpsAlerts + listOpsProvisionRuns 三源轮询
 *   （Promise.allSettled best-effort，单源失败不整体挂）；nodes 内含
 *   gameId/env 时 best-effort 拉 listSyntheticsResults（探针失败→过载信号）；
 * - 20 节点模拟（验收）：1 离线 2 过载 1 压力 → 灰/红/橙 Tag 计数正确，
 *   离线置底（DOM 顺序最后）；
 * - 聚合带：全网节点 20 / 在线率 95% / 今日扩缩容 / 活动告警数；
 * - 断流：nodes 拉取失败 → lastUpdated 为 null → 「数据过期」横幅显示；
 *   成功拉取 → 横幅不显示；
 * - 告警滚动条：活动告警渲染 severity Tag + summary 文本；全静默 → 暂无；
 * - 节点下钻：点击卡片 → Modal 打开（节点 ID/状态/CPU/内存）+
 *   getAgentMetricsHistory(agentId, {window:'hourly', limit:48}) 被调；
 * - 账号负载视图：边界占位（依赖调度编排账号分布数据，OPS-002）。
 *
 * mock 口径：services/api/ops 五函数 jest.mock（service 边界）；@umijs/max
 * 本地 defaultMessage mock + useLocation；antd/pro-components 真实实现。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import OpsDashboardPage from '../index';
import type { OpsNode } from '@/services/api/ops';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  listOpsNodes: jest.fn(),
  fetchOpsAlerts: jest.fn(),
  listOpsProvisionRuns: jest.fn(),
  listSyntheticsResults: jest.fn(),
  getAgentMetricsHistory: jest.fn(),
}));

const mockIntl = {
  formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
};
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
  useLocation: () => ({ search: '', pathname: '/ops/dashboard' }),
}));

import {
  fetchOpsAlerts,
  getAgentMetricsHistory,
  listOpsNodes,
  listOpsProvisionRuns,
  listSyntheticsResults,
} from '@/services/api/ops';

const mNodes = listOpsNodes as jest.MockedFunction<typeof listOpsNodes>;
const mAlerts = fetchOpsAlerts as jest.MockedFunction<typeof fetchOpsAlerts>;
const mProv = listOpsProvisionRuns as jest.MockedFunction<typeof listOpsProvisionRuns>;
const mProbe = listSyntheticsResults as jest.MockedFunction<typeof listSyntheticsResults>;
const mHist = getAgentMetricsHistory as jest.MockedFunction<typeof getAgentMetricsHistory>;

type PartialNode = {
  id: string;
  hostname?: string;
  status?: string;
  gameId?: string;
  env?: string;
  cpu?: { usagePercent?: number };
  memory?: { usagePercent?: number };
  labels?: Record<string, string>;
};

function mkNode(partial: PartialNode): OpsNode {
  const cpu = partial.cpu
    ? { usagePercent: partial.cpu.usagePercent ?? 0, cores: 0, load1m: 0, load5m: 0, load15m: 0 }
    : undefined;
  const memory = partial.memory
    ? {
        totalBytes: 0,
        usedBytes: 0,
        availableBytes: 0,
        usagePercent: partial.memory.usagePercent ?? 0,
        swapTotal: 0,
        swapUsed: 0,
      }
    : undefined;
  return {
    id: partial.id,
    hostname: partial.hostname,
    status: partial.status,
    gameId: partial.gameId,
    env: partial.env,
    labels: partial.labels,
    cpu,
    memory,
  } as OpsNode;
}

/** 20 节点模拟（验收形态）：16 正常 + 1 离线 + 2 过载 + 1 压力。 */
function fleet20(): OpsNode[] {
  const nodes: OpsNode[] = [];
  for (let i = 0; i < 16; i++) {
    nodes.push(
      mkNode({
        id: `n${i}`,
        hostname: `h${i}`,
        status: 'online',
        gameId: 'g',
        env: 'e',
        cpu: { usagePercent: 20 },
        memory: { usagePercent: 30 },
      }),
    );
  }
  nodes.push(mkNode({ id: 'off', hostname: 'h-off', status: 'offline', gameId: 'g', env: 'e' }));
  nodes.push(
    mkNode({
      id: 'ov1',
      hostname: 'h-ov1',
      status: 'online',
      gameId: 'g',
      env: 'e',
      cpu: { usagePercent: 95 },
      memory: { usagePercent: 40 },
    }),
  );
  nodes.push(
    mkNode({
      id: 'ov2',
      hostname: 'h-ov2',
      status: 'online',
      gameId: 'g',
      env: 'e',
      cpu: { usagePercent: 40 },
      memory: { usagePercent: 92 },
    }),
  );
  nodes.push(
    mkNode({
      id: 'pr1',
      hostname: 'h-pr1',
      status: 'online',
      gameId: 'g',
      env: 'e',
      cpu: { usagePercent: 70 },
      memory: { usagePercent: 30 },
    }),
  );
  return nodes;
}

function mockHealthySources() {
  mNodes.mockResolvedValue({ nodes: fleet20() });
  mAlerts.mockResolvedValue({
    alerts: [
      { severity: 'critical', summary: 'CPU 饱和', instance: 'h-ov1', silenced: false },
      { severity: 'warning', summary: '磁盘水位', instance: 'h-pr1', silenced: true },
    ],
  });
  mProv.mockResolvedValue([
    { id: 'r1', state: 'adopted', createdAt: new Date().toISOString() },
    { id: 'r2', state: 'destroyed', createdAt: new Date().toISOString() },
    { id: 'r3', state: 'destroyed', createdAt: '2020-01-01T00:00:00Z' },
  ]);
  mProbe.mockResolvedValue({ items: [] });
  mHist.mockResolvedValue([]);
}

beforeEach(() => {
  jest.clearAllMocks();
});

describe('OpsDashboardPage 20 节点模拟与聚合带', () => {
  it('三色 Tag 计数正确（1 离线 2 过载 1 压力），离线置底', async () => {
    mockHealthySources();
    render(<OpsDashboardPage />);
    await waitFor(() => expect(screen.getByText('全网节点')).toBeInTheDocument());

    // 聚合带（Statistic value 与 suffix 是分离节点，按 value 文本断言）
    expect(screen.getByText('20')).toBeInTheDocument(); // 全网节点
    expect(screen.getByText('95')).toBeInTheDocument(); // 在线率 19/20
    expect(screen.getByText('2')).toBeInTheDocument(); // 今日扩缩容（r1/r2 今日，r3 非今日）
    // 状态计数 Tag：正常 16 / 压力 1 / 过载 2 / 离线 1
    expect(screen.getByText('正常 16')).toBeInTheDocument();
    expect(screen.getByText('压力 1')).toBeInTheDocument();
    expect(screen.getByText('过载 2')).toBeInTheDocument();
    expect(screen.getByText('离线 1')).toBeInTheDocument();

    // 节点卡健康档 Tag：16 正常 + 2 过载 + 1 压力 + 1 离线
    const wall = screen.getByText('节点健康').closest('.ant-card') as HTMLElement;
    expect(within(wall).getAllByText('正常').length).toBe(16);
    expect(within(wall).getAllByText('过载').length).toBe(2);
    expect(within(wall).getAllByText('压力').length).toBe(1);
    expect(within(wall).getAllByText('离线').length).toBe(1);

    // 离线置底：h-off 卡在 DOM 中排最后（每卡首个 <strong> 即 hostname）
    const cards = Array.from(wall.querySelectorAll('[data-testid^="node-card-"]'));
    const labels = cards.map((c) => c.querySelector('strong')?.textContent);
    expect(labels[labels.length - 1]).toBe('h-off');
  });

  it('成功拉取后不显示数据过期横幅', async () => {
    mockHealthySources();
    render(<OpsDashboardPage />);
    await waitFor(() => expect(screen.getByText('全网节点')).toBeInTheDocument());
    expect(screen.queryByText(/数据过期/)).not.toBeInTheDocument();
    expect(screen.getByText('实时')).toBeInTheDocument();
  });

  it('nodes 拉取失败 → 数据过期横幅（lastUpdated 为 null）', async () => {
    mNodes.mockRejectedValue(new Error('boom'));
    mAlerts.mockResolvedValue({ alerts: [] });
    mProv.mockResolvedValue([]);
    render(<OpsDashboardPage />);
    await waitFor(() => expect(screen.getAllByText(/数据过期/).length).toBeGreaterThan(0));
  });
});

describe('OpsDashboardPage 告警滚动条', () => {
  it('活动告警渲染 severity + summary；静默告警不计入', async () => {
    mockHealthySources();
    render(<OpsDashboardPage />);
    await waitFor(() => expect(screen.getByText('CPU 饱和')).toBeInTheDocument());
    expect(screen.queryByText('磁盘水位')).not.toBeInTheDocument(); // silenced
    // 活动告警区块（聚合带 Statistic 标题 + 卡片标题各一处）
    expect(screen.getAllByText('活动告警').length).toBeGreaterThan(0);
  });
  it('无活动告警 → 暂无提示', async () => {
    mNodes.mockResolvedValue({ nodes: [] });
    mAlerts.mockResolvedValue({ alerts: [] });
    mProv.mockResolvedValue([]);
    render(<OpsDashboardPage />);
    await waitFor(() => expect(screen.getByText('暂无活动告警')).toBeInTheDocument());
    expect(screen.getByText('无节点数据')).toBeInTheDocument();
  });
});

describe('OpsDashboardPage 节点下钻与账号负载边界', () => {
  it('点击节点卡打开详情 Modal 并拉 hourly 历史', async () => {
    mockHealthySources();
    mHist.mockResolvedValue([
      {
        timestamp: '2026-11-03T00:00:00Z',
        cpu: { usagePercent: 42, cores: 4, load1m: 0, load5m: 0, load15m: 0 },
      },
    ]);
    render(<OpsDashboardPage />);
    await waitFor(() => expect(screen.getByTestId('node-card-ov1')).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('node-card-ov1'));
    await waitFor(() => expect(screen.getByText('h-ov1 详情')).toBeInTheDocument());
    expect(mHist).toHaveBeenCalledWith('ov1', { window: 'hourly', limit: 48 });
    expect(screen.getByText('节点 ID')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTitle(/cpu 42%/)).toBeInTheDocument());
  });

  it('账号负载视图显示待接入边界（不伪造数据）', async () => {
    mockHealthySources();
    render(<OpsDashboardPage />);
    await waitFor(() => expect(screen.getByText('账号负载视图')).toBeInTheDocument());
    expect(screen.getByText(/账号负载分布依赖调度编排的账号分布数据/)).toBeInTheDocument();
  });
});
