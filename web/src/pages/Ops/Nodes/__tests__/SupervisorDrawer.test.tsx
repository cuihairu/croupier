/**
 * SupervisorDrawer 覆盖收口（Supervisor S1）：拉取/渲染、状态徽标、
 * 资源格式化、flags 两翼、失败空态、未打开不拉取。Drawer 内容挂在
 * portal，用 document 级查询（见 web jest 测试坑记录）。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import SupervisorDrawer from '../SupervisorDrawer';
import {
  fetchAgentSupervisor,
  type OpsAgentSupervisorResponse,
  type SupervisedProcess,
} from '@/services/api/ops';
import type { NodeRow } from '../shared';

jest.mock('@/services/api/ops', () => ({
  fetchAgentSupervisor: jest.fn(),
}));
const mockFetch = fetchAgentSupervisor as jest.MockedFunction<typeof fetchAgentSupervisor>;

const NODE: NodeRow = {
  agentId: 'agent-1',
} as unknown as NodeRow;

const RUNNING: SupervisedProcess = {
  name: 'gameserver',
  pid: 42,
  state: 'running',
  uptimeSeconds: 3661,
  restartCount: 1,
  rssBytes: 2 * 1024 * 1024 * 1024,
  cpuPercent: 12.34,
  flags: ['mem_over_limit'],
};

const STOPPED: SupervisedProcess = {
  name: 'worker',
  pid: 0,
  state: 'stopped',
  uptimeSeconds: 0,
  restartCount: 0,
  rssBytes: 0,
  cpuPercent: 0,
  flags: [],
};

function resp(overrides: Partial<OpsAgentSupervisorResponse> = {}): OpsAgentSupervisorResponse {
  return {
    agentId: 'agent-1',
    timestamp: '2026-10-09 10:00:00',
    processes: [RUNNING, STOPPED],
    summary: { status: 'warn', total: 2, running: 1 },
    ...overrides,
  };
}

beforeEach(() => {
  jest.clearAllMocks();
});

describe('SupervisorDrawer', () => {
  it('node 非空：按 agentId 拉取并渲染进程行，标题带 agentId', async () => {
    mockFetch.mockResolvedValue(resp());
    render(<SupervisorDrawer node={NODE} onClose={() => {}} />);

    expect(mockFetch).toHaveBeenCalledWith('agent-1');
    expect(await screen.findByText('进程监管 · agent-1')).toBeInTheDocument();
    expect(await screen.findByText('gameserver')).toBeInTheDocument();
    expect(screen.getByText('worker')).toBeInTheDocument();
    // 最后上报时间戳展示
    expect(screen.getByText('最后上报：2026-10-09 10:00:00')).toBeInTheDocument();
    // 资源格式化：2GiB → "2.0 GB"；3661s → "1h 1m"；CPU 12.34 → "12.3%"
    expect(screen.getByText('2.0 GB')).toBeInTheDocument();
    expect(screen.getByText('1h 1m')).toBeInTheDocument();
    expect(screen.getByText('12.3%')).toBeInTheDocument();
  });

  it('状态徽标与 flags 两翼：running/failed/oom_suspect 标红，无 flags 显示 -', async () => {
    mockFetch.mockResolvedValue(
      resp({
        processes: [
          { ...RUNNING, flags: ['oom_suspect', 'breaker_tripped'] },
          STOPPED,
          { ...RUNNING, name: 'clean', state: 'failed', pid: 9, flags: [] },
        ],
      }),
    );
    render(<SupervisorDrawer node={NODE} onClose={() => {}} />);

    expect(await screen.findByText('oom_suspect')).toBeInTheDocument();
    expect(screen.getByText('breaker_tripped')).toBeInTheDocument();
    // 无 flags 的行渲染 "-"（worker 与 clean 两行）
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(2);
    // stopped 行 pid 0 → '-'
    expect(screen.getByText('failed')).toBeInTheDocument();
  });

  it('拉取失败：面板展示空态文案，不崩', async () => {
    mockFetch.mockRejectedValue(new Error('agent offline'));
    render(<SupervisorDrawer node={NODE} onClose={() => {}} />);

    expect(
      await screen.findByText(
        '该 agent 未上报托管进程（未配置 ops.managedProcesses 或 ops 未启用）',
      ),
    ).toBeInTheDocument();
    // 失败时最后上报落「暂无上报」
    expect(screen.getByText('最后上报：暂无上报')).toBeInTheDocument();
  });

  it('node 为空：effect 早退不发起拉取；抽屉未打开前内容不挂载', () => {
    const { baseElement } = render(<SupervisorDrawer node={null} onClose={() => {}} />);

    expect(mockFetch).not.toHaveBeenCalled();
    expect(baseElement.textContent).not.toContain('gameserver');
  });

  it('无上报数据：timestamp 空串落「暂无上报」，进程列表空', async () => {
    mockFetch.mockResolvedValue(
      resp({ timestamp: '', processes: [], summary: { status: 'ok', total: 0, running: 0 } }),
    );
    render(<SupervisorDrawer node={NODE} onClose={() => {}} />);

    expect(await screen.findByText('最后上报：暂无上报')).toBeInTheDocument();
  });
});
