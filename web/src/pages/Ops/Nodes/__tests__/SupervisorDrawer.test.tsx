/**
 * SupervisorDrawer 覆盖收口（Supervisor S1/S2）：拉取/渲染、状态徽标、
 * 资源格式化、flags 两翼、失败空态、未打开不拉取；S2 增量：进程操作列
 * （重启 Popconfirm 确认 → 服务调用 + 快照复拉）、事件日志 tab（拉取/
 * 渲染/疑似 OOM/事件类型过滤）、下载日志链接、node 空不拉事件。
 * Drawer 内容挂在 portal，用 document 级查询（见 web jest 测试坑记录）；
 * 组件走 App.useApp().message，用例以真实 <App> 包裹（Cluster 页测试先例）。
 */
import React from 'react';
import { render, screen, waitFor, within, fireEvent } from '@testing-library/react';
import { App } from 'antd';
import SupervisorDrawer from '../SupervisorDrawer';
import {
  fetchAgentSupervisor,
  fetchAgentSupervisorEvents,
  pauseAgentProcessSchedule,
  restartAgentProcess,
  resumeAgentProcessSchedule,
  startAgentProcess,
  stopAgentProcess,
  type OpsAgentSupervisorEventsResponse,
  type OpsAgentSupervisorResponse,
  type SupervisedProcess,
  type SupervisorEvent,
} from '@/services/api/ops';
import type { NodeRow } from '../shared';

jest.mock('@/services/api/ops', () => ({
  ...jest.requireActual('@/services/api/ops'),
  fetchAgentSupervisor: jest.fn(),
  fetchAgentSupervisorEvents: jest.fn(),
  startAgentProcess: jest.fn(),
  stopAgentProcess: jest.fn(),
  restartAgentProcess: jest.fn(),
  pauseAgentProcessSchedule: jest.fn(),
  resumeAgentProcessSchedule: jest.fn(),
}));
const mockFetch = fetchAgentSupervisor as jest.MockedFunction<typeof fetchAgentSupervisor>;
const mockFetchEvents = fetchAgentSupervisorEvents as jest.MockedFunction<
  typeof fetchAgentSupervisorEvents
>;
const mockStart = startAgentProcess as jest.MockedFunction<typeof startAgentProcess>;
const mockStop = stopAgentProcess as jest.MockedFunction<typeof stopAgentProcess>;
const mockRestart = restartAgentProcess as jest.MockedFunction<typeof restartAgentProcess>;
const mockPause = pauseAgentProcessSchedule as jest.MockedFunction<
  typeof pauseAgentProcessSchedule
>;
const mockResume = resumeAgentProcessSchedule as jest.MockedFunction<
  typeof resumeAgentProcessSchedule
>;

const NODE: NodeRow = {
  agentId: 'agent-1',
} as unknown as NodeRow;

/** 快照新字段缺省值（无调度、无指纹、program 类别） */
const PROCESS_DEFAULTS: Pick<
  SupervisedProcess,
  | 'lastEventUnix'
  | 'nextRestartAtUnix'
  | 'kind'
  | 'adapterId'
  | 'nextScheduleAtUnix'
  | 'nextScheduleAction'
  | 'fingerprint'
  | 'scheduleSuspended'
> = {
  lastEventUnix: 0,
  nextRestartAtUnix: 0,
  kind: 'program',
  adapterId: '',
  nextScheduleAtUnix: 0,
  nextScheduleAction: '',
  fingerprint: '',
  scheduleSuspended: false,
};

const RUNNING: SupervisedProcess = {
  name: 'gameserver',
  pid: 42,
  state: 'running',
  uptimeSeconds: 3661,
  restartCount: 1,
  rssBytes: 2 * 1024 * 1024 * 1024,
  cpuPercent: 12.34,
  flags: ['mem_over_limit'],
  snapshotProfile: 'jvm',
  ...PROCESS_DEFAULTS,
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
  snapshotProfile: '',
  ...PROCESS_DEFAULTS,
};

/** 下一触发时间固定为整点槽，便于断言 zh-CN 本地化串 */
const NEXT_FIRE_UNIX = 1760000400;

const EVENT_BASE: SupervisorEvent = {
  seq: 0,
  tsUnix: 0,
  process: '',
  event: '',
  oldPid: 0,
  newPid: 0,
  exitCode: 0,
  signal: '',
  restartCount: 0,
  message: '',
  lastHeartbeatUnix: 0,
  lastError: '',
  oomSuspect: false,
  lastRssBytes: 0,
  snapshotDir: '',
  snapshotFiles: [],
};

const EV_DETECT: SupervisorEvent = {
  ...EVENT_BASE,
  seq: 12,
  tsUnix: 1760000000,
  process: 'gameserver',
  event: 'detect_down',
  oldPid: 123,
  newPid: 0,
  exitCode: -1,
  signal: 'killed',
  restartCount: 3,
  message: 'process exited unexpectedly',
  oomSuspect: true,
  lastRssBytes: 812000000,
};

const EV_AUTO: SupervisorEvent = {
  ...EVENT_BASE,
  seq: 11,
  tsUnix: 1759999990,
  process: 'gameserver',
  event: 'auto_restart',
  oldPid: 123,
  newPid: 124,
  restartCount: 4,
  message: 'respawned',
};

const EV_BREAKER: SupervisorEvent = {
  ...EVENT_BASE,
  seq: 10,
  tsUnix: 1759999900,
  process: 'gameserver',
  event: 'breaker_tripped',
  restartCount: 4,
  message: 'too many restart failures',
};

const EV_RESOURCE: SupervisorEvent = {
  ...EVENT_BASE,
  seq: 9,
  tsUnix: 1759999800,
  process: 'worker',
  event: 'resource_over_limit',
  message: 'rss over limit',
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

function eventsResp(
  overrides: Partial<OpsAgentSupervisorEventsResponse> = {},
): OpsAgentSupervisorEventsResponse {
  return {
    agentId: 'agent-1',
    events: [EV_DETECT, EV_AUTO, EV_BREAKER, EV_RESOURCE],
    ...overrides,
  };
}

function renderDrawer(node: NodeRow | null = NODE) {
  return render(
    <App>
      <SupervisorDrawer node={node} onClose={() => {}} />
    </App>,
  );
}

/** 打开事件日志 tab 并等待事件行渲染（Tabs 懒挂载，切换后才拉取） */
async function openEventsTab() {
  mockFetchEvents.mockResolvedValue(eventsResp());
  fireEvent.click(await screen.findByText('事件日志'));
  await screen.findByText('检测到宕机');
}

beforeEach(() => {
  jest.clearAllMocks();
});

describe('SupervisorDrawer', () => {
  it('node 非空：按 agentId 拉取并渲染进程行，标题带 agentId', async () => {
    mockFetch.mockResolvedValue(resp());
    renderDrawer();

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
    renderDrawer();

    expect(await screen.findByText('oom_suspect')).toBeInTheDocument();
    expect(screen.getByText('breaker_tripped')).toBeInTheDocument();
    // 无 flags 的行渲染 "-"（worker 与 clean 两行）
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(2);
    // stopped 行 pid 0 → '-'
    expect(screen.getByText('failed')).toBeInTheDocument();
  });

  it('拉取失败：面板展示空态文案，不崩', async () => {
    mockFetch.mockRejectedValue(new Error('agent offline'));
    renderDrawer();

    expect(
      await screen.findByText(
        '该 agent 未上报托管进程（未配置 ops.managedProcesses 或 ops 未启用）',
      ),
    ).toBeInTheDocument();
    // 失败时最后上报落「暂无上报」
    expect(screen.getByText('最后上报：暂无上报')).toBeInTheDocument();
  });

  it('node 为空：effect 早退不发起拉取；抽屉未打开前内容不挂载', () => {
    const { baseElement } = renderDrawer(null);

    expect(mockFetch).not.toHaveBeenCalled();
    expect(mockFetchEvents).not.toHaveBeenCalled();
    expect(baseElement.textContent).not.toContain('gameserver');
  });

  it('无上报数据：timestamp 空串落「暂无上报」，进程列表空', async () => {
    mockFetch.mockResolvedValue(
      resp({ timestamp: '', processes: [], summary: { status: 'ok', total: 0, running: 0 } }),
    );
    renderDrawer();

    expect(await screen.findByText('最后上报：暂无上报')).toBeInTheDocument();
  });

  describe('S2 进程操作列', () => {
    it('操作列渲染启动/停止/重启/暂停/恢复五按钮；重启 Popconfirm 确认 → restartAgentProcess + 成功提示 + 快照复拉', async () => {
      mockFetch.mockResolvedValue(resp());
      mockRestart.mockResolvedValue(undefined);
      renderDrawer();

      expect(await screen.findByText('gameserver')).toBeInTheDocument();
      // 操作列五按钮（两行进程 → 各两枚 启动/停止/重启/暂停/恢复）
      expect(screen.getByText('操作')).toBeInTheDocument();
      expect(screen.getAllByText('启动').length).toBe(2);
      expect(screen.getAllByText('停止').length).toBe(2);
      expect(screen.getAllByText('暂停').length).toBe(2);
      expect(screen.getAllByText('恢复').length).toBe(2);

      // 「重启」同时命中列头（column.restarts），只点按钮那一枚
      const restartBtn = screen
        .getAllByText('重启')
        .map((el) => el.closest('button'))
        .find(Boolean) as HTMLElement;
      fireEvent.click(restartBtn);
      expect(await screen.findByText('确认重启进程 gameserver 吗？')).toBeInTheDocument();

      const popover = Array.from(document.querySelectorAll('.ant-popover')).find(
        (p) => !p.className.includes('ant-popover-hidden'),
      ) as HTMLElement;
      fireEvent.click(within(popover).getByRole('button', { name: 'OK' }));

      await waitFor(() => expect(mockRestart).toHaveBeenCalledWith('agent-1', 'gameserver'));
      expect(await screen.findByText('已重启进程 gameserver')).toBeInTheDocument();
      // 动作成功后复拉 supervisor 快照刷新状态
      await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
      expect(mockFetch).toHaveBeenLastCalledWith('agent-1');
    });

    it('停止 Popconfirm 确认 → stopAgentProcess；启动直点（无确认）→ startAgentProcess', async () => {
      mockFetch.mockResolvedValue(resp());
      mockStop.mockResolvedValue(undefined);
      mockStart.mockResolvedValue(43);
      renderDrawer();

      expect(await screen.findByText('gameserver')).toBeInTheDocument();
      fireEvent.click(screen.getAllByText('停止')[0]);
      expect(await screen.findByText('确认停止进程 gameserver 吗？')).toBeInTheDocument();
      const popover = Array.from(document.querySelectorAll('.ant-popover')).find(
        (p) => !p.className.includes('ant-popover-hidden'),
      ) as HTMLElement;
      fireEvent.click(within(popover).getByRole('button', { name: 'OK' }));
      await waitFor(() => expect(mockStop).toHaveBeenCalledWith('agent-1', 'gameserver'));

      fireEvent.click(screen.getAllByText('启动')[0]);
      await waitFor(() => expect(mockStart).toHaveBeenCalledWith('agent-1', 'gameserver'));
      expect(await screen.findByText('已启动进程 gameserver')).toBeInTheDocument();
    });

    it('动作失败：Error.message 进 message.error，不复拉快照', async () => {
      mockFetch.mockResolvedValue(resp());
      mockRestart.mockRejectedValue(new Error('agent offline'));
      renderDrawer();

      expect(await screen.findByText('gameserver')).toBeInTheDocument();
      const restartBtn = screen
        .getAllByText('重启')
        .map((el) => el.closest('button'))
        .find(Boolean) as HTMLElement;
      fireEvent.click(restartBtn);
      expect(await screen.findByText('确认重启进程 gameserver 吗？')).toBeInTheDocument();
      const popover = Array.from(document.querySelectorAll('.ant-popover')).find(
        (p) => !p.className.includes('ant-popover-hidden'),
      ) as HTMLElement;
      fireEvent.click(within(popover).getByRole('button', { name: 'OK' }));

      expect(await screen.findByText('agent offline')).toBeInTheDocument();
      await waitFor(() => expect(mockRestart).toHaveBeenCalled());
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
  });

  describe('§10.2 暂停与恢复调度（暂停对象=调度）', () => {
    it('暂停 Popconfirm 确认 → pauseAgentProcessSchedule + 成功提示 + 快照复拉', async () => {
      mockFetch.mockResolvedValue(
        resp({
          processes: [
            { ...RUNNING, nextScheduleAtUnix: NEXT_FIRE_UNIX, nextScheduleAction: 'restart' },
          ],
        }),
      );
      mockPause.mockResolvedValue(undefined);
      renderDrawer();

      expect(await screen.findByText('gameserver')).toBeInTheDocument();
      const pauseBtn = screen
        .getAllByText('暂停')
        .map((el) => el.closest('button'))
        .find(Boolean) as HTMLElement;
      fireEvent.click(pauseBtn);
      expect(
        await screen.findByText(
          '确认暂停进程 gameserver 的定时调度吗？（进程继续运行，仅冻结 cron 触发）',
        ),
      ).toBeInTheDocument();
      const popover = Array.from(document.querySelectorAll('.ant-popover')).find(
        (p) => !p.className.includes('ant-popover-hidden'),
      ) as HTMLElement;
      fireEvent.click(within(popover).getByRole('button', { name: 'OK' }));

      await waitFor(() => expect(mockPause).toHaveBeenCalledWith('agent-1', 'gameserver'));
      expect(await screen.findByText('已暂停进程 gameserver 调度')).toBeInTheDocument();
      await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
    });

    it('恢复直点（无确认）→ resumeAgentProcessSchedule + 成功提示 + 快照复拉', async () => {
      mockFetch.mockResolvedValue(
        resp({
          processes: [
            {
              ...RUNNING,
              nextScheduleAtUnix: NEXT_FIRE_UNIX,
              nextScheduleAction: 'restart',
              scheduleSuspended: true,
            },
          ],
        }),
      );
      mockResume.mockResolvedValue(undefined);
      renderDrawer();

      expect(await screen.findByText('gameserver')).toBeInTheDocument();
      fireEvent.click(screen.getAllByText('恢复')[0]);

      await waitFor(() => expect(mockResume).toHaveBeenCalledWith('agent-1', 'gameserver'));
      expect(await screen.findByText('已恢复进程 gameserver 调度')).toBeInTheDocument();
      await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
    });

    it('禁用态：已暂停行暂停禁用/恢复可用；无调度行暂停禁用；恢复仅暂停态可用', async () => {
      mockFetch.mockResolvedValue(
        resp({
          processes: [
            {
              ...RUNNING,
              name: 'paused',
              nextScheduleAtUnix: NEXT_FIRE_UNIX,
              nextScheduleAction: 'restart',
              scheduleSuspended: true,
            },
            { ...RUNNING, name: 'plain' },
          ],
        }),
      );
      renderDrawer();

      expect(await screen.findByText('paused')).toBeInTheDocument();
      const pausedRow = screen.getByText('paused').closest('tr') as HTMLElement;
      const plainRow = screen.getByText('plain').closest('tr') as HTMLElement;
      const pausedPause = within(pausedRow).getByText('暂停').closest('button') as HTMLElement;
      const pausedResume = within(pausedRow).getByText('恢复').closest('button') as HTMLElement;
      const plainPause = within(plainRow).getByText('暂停').closest('button') as HTMLElement;
      const plainResume = within(plainRow).getByText('恢复').closest('button') as HTMLElement;
      expect(pausedPause).toBeDisabled();
      expect(pausedResume).toBeEnabled();
      expect(plainPause).toBeDisabled();
      expect(plainResume).toBeDisabled();
    });

    it('暂停失败：Error.message 进 message.error，不复拉快照', async () => {
      mockFetch.mockResolvedValue(
        resp({
          processes: [
            { ...RUNNING, nextScheduleAtUnix: NEXT_FIRE_UNIX, nextScheduleAction: 'restart' },
          ],
        }),
      );
      mockPause.mockRejectedValue(new Error('agent offline'));
      renderDrawer();

      expect(await screen.findByText('gameserver')).toBeInTheDocument();
      const pauseBtn = screen
        .getAllByText('暂停')
        .map((el) => el.closest('button'))
        .find(Boolean) as HTMLElement;
      fireEvent.click(pauseBtn);
      const popover = Array.from(document.querySelectorAll('.ant-popover')).find(
        (p) => !p.className.includes('ant-popover-hidden'),
      ) as HTMLElement;
      fireEvent.click(within(popover).getByRole('button', { name: 'OK' }));

      expect(await screen.findByText('agent offline')).toBeInTheDocument();
      await waitFor(() => expect(mockPause).toHaveBeenCalled());
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });
  });

  describe('§10.1/§10.4/§10.6 快照新字段', () => {
    it('调度列：无调度显示 -；有调度显示下一触发时间+动作 Tag；暂停态覆盖为「调度已暂停」', async () => {
      const nextFire = new Date(NEXT_FIRE_UNIX * 1000).toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      });
      mockFetch.mockResolvedValue(
        resp({
          processes: [
            {
              ...RUNNING,
              name: 'scheduled',
              nextScheduleAtUnix: NEXT_FIRE_UNIX,
              nextScheduleAction: 'restart',
            },
            {
              ...RUNNING,
              name: 'paused',
              nextScheduleAtUnix: NEXT_FIRE_UNIX,
              nextScheduleAction: 'restart',
              scheduleSuspended: true,
            },
            { ...RUNNING, name: 'plain' },
          ],
        }),
      );
      renderDrawer();

      expect(await screen.findByText('scheduled')).toBeInTheDocument();
      const scheduledRow = screen.getByText('scheduled').closest('tr') as HTMLElement;
      const pausedRow = screen.getByText('paused').closest('tr') as HTMLElement;
      const plainRow = screen.getByText('plain').closest('tr') as HTMLElement;
      expect(within(scheduledRow).getByText(nextFire)).toBeInTheDocument();
      expect(within(scheduledRow).getByText('restart')).toBeInTheDocument();
      expect(within(pausedRow).getByText('调度已暂停')).toBeInTheDocument();
      expect(within(plainRow).getByText('-')).toBeInTheDocument();
    });

    it('kind=probe-adapter 显示 adapter:<id> Tag；program 不显示', async () => {
      mockFetch.mockResolvedValue(
        resp({
          processes: [
            { ...RUNNING, name: 'probe', kind: 'probe-adapter', adapterId: 'dbmon-1' },
            RUNNING,
          ],
        }),
      );
      renderDrawer();

      expect(await screen.findByText('probe')).toBeInTheDocument();
      expect(screen.getByText('adapter:dbmon-1')).toBeInTheDocument();
      expect(screen.queryByText('adapter:gameserver')).not.toBeInTheDocument();
    });

    it('fingerprint 显示 fp:<12> 截断 Tag；空指纹不显示且 flags 空时落 -', async () => {
      mockFetch.mockResolvedValue(
        resp({
          processes: [
            { ...RUNNING, name: 'patched', fingerprint: 'abcdef0123456789abcdef0123456789' },
            { ...RUNNING, name: 'fresh', flags: [] },
          ],
        }),
      );
      renderDrawer();

      expect(await screen.findByText('patched')).toBeInTheDocument();
      expect(screen.getByText('fp:abcdef012345')).toBeInTheDocument();
      const freshRow = screen.getByText('fresh').closest('tr') as HTMLElement;
      expect(within(freshRow).getByText('-')).toBeInTheDocument();
    });

    it('事件闭集新类型渲染：schedule_fire/update_applied 等落本地化标签', async () => {
      mockFetch.mockResolvedValue(resp());
      renderDrawer();
      await screen.findByText('gameserver');

      const fire: SupervisorEvent = {
        ...EVENT_BASE,
        seq: 7,
        tsUnix: 1759999600,
        process: 'gameserver',
        event: 'schedule_fire',
        message: 'cron fired',
      };
      const applied: SupervisorEvent = {
        ...EVENT_BASE,
        seq: 6,
        tsUnix: 1759999500,
        process: 'gameserver',
        event: 'update_applied',
        message: 'binary replaced',
      };
      const rolledBack: SupervisorEvent = {
        ...EVENT_BASE,
        seq: 5,
        tsUnix: 1759999400,
        process: 'gameserver',
        event: 'update_rolled_back',
        message: 'rolled back to old',
      };
      mockFetchEvents.mockResolvedValue(eventsResp({ events: [fire, applied, rolledBack] }));
      fireEvent.click(await screen.findByText('事件日志'));

      expect(await screen.findByText('定时触发')).toBeInTheDocument();
      expect(screen.getByText('更新生效')).toBeInTheDocument();
      expect(screen.getByText('更新回滚')).toBeInTheDocument();
    });
  });

  describe('S2 事件日志 tab', () => {
    it('切换 tab 后拉取事件，seq 降序渲染；oomSuspect 行显示疑似 OOM 与 PID 变化', async () => {
      mockFetch.mockResolvedValue(resp());
      renderDrawer();
      await screen.findByText('gameserver');

      await openEventsTab();

      expect(mockFetchEvents).toHaveBeenCalledWith('agent-1');
      // 四条事件全部渲染（闭集标签）
      expect(screen.getByText('自动拉起')).toBeInTheDocument();
      expect(screen.getByText('熔断触发')).toBeInTheDocument();
      expect(screen.getByText('资源超限')).toBeInTheDocument();
      // oomSuspect 翼
      expect(screen.getByText('疑似 OOM')).toBeInTheDocument();
      // 详情/信号翼与 PID 变化
      expect(screen.getByText('process exited unexpectedly')).toBeInTheDocument();
      expect(screen.getByText(/signal=killed/)).toBeInTheDocument();
      expect(screen.getByText('123 → 0')).toBeInTheDocument();
      // 时间列按 zh-CN 本地化格式展示（与 utils/format formatDateTime 同族）
      const expected = new Date(1760000000 * 1000).toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      });
      expect(screen.getByText(expected)).toBeInTheDocument();
      // seq 降序：detect_down(12) 行在 auto_restart(11) 行之前
      const detectCell = screen.getByText('检测到宕机').closest('td');
      const autoCell = screen.getByText('自动拉起').closest('td');
      expect(detectCell).not.toBeNull();
      expect(autoCell).not.toBeNull();
      // 4 = DOCUMENT_POSITION_FOLLOWING：auto 行位于 detect 行之后
      expect(
        (detectCell as HTMLElement).compareDocumentPosition(autoCell as HTMLElement) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    });

    it('事件类型过滤：Select 选 auto_restart 后只剩该类型行', async () => {
      mockFetch.mockResolvedValue(resp());
      renderDrawer();
      await screen.findByText('gameserver');
      await openEventsTab();

      // 事件 pane 内唯一 Select（aria-label=事件），antd6 mouseDown 根 + 可见 option
      // （rc-select 开/关走 message 宏任务，先留真实时间隙——RateLimits 测试同款）
      await new Promise((r) => setTimeout(r, 60));
      const select = document.querySelector('.ant-drawer .ant-select') as HTMLElement;
      expect(select).not.toBeNull();
      fireEvent.mouseDown(select);
      const dropdown = await waitFor(() => {
        const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
          (d) => !d.className.includes('ant-select-dropdown-hidden'),
        ) as HTMLElement;
        expect(visible).not.toBeUndefined();
        return visible;
      });
      // 选项渲染晚于容器去隐藏，须等 option 落地再点（RateLimits 测试同款）
      const option = await waitFor(() => {
        const hit = Array.from(dropdown.querySelectorAll('.ant-select-item-option')).find(
          (o) => o.textContent === '自动拉起',
        ) as HTMLElement;
        expect(hit).not.toBeUndefined();
        return hit;
      });
      fireEvent.click(option);

      // 其余类型行被过滤掉，auto_restart 行仍在。
      // 断言收口在事件表内：下拉 portal（即使收起后残留在 DOM）也含选项文本，
      // screen 级 queryByText 会误命中。
      await waitFor(() => {
        const tables = Array.from(
          document.querySelectorAll('.ant-drawer .ant-table'),
        ) as HTMLElement[];
        const eventsTable = tables.find((t) => (t.textContent ?? '').includes('respawned'));
        expect(eventsTable).toBeDefined();
        expect(eventsTable?.textContent).not.toContain('检测到宕机');
        expect(eventsTable?.textContent).not.toContain('熔断触发');
        expect(eventsTable?.textContent).not.toContain('资源超限');
        expect(eventsTable?.textContent).not.toContain('123 → 0');
      });
      expect(screen.getByText('123 → 124')).toBeInTheDocument();
    });

    it('下载日志链接：href 含 /supervisor/logs 与 agentId，target=_blank', async () => {
      mockFetch.mockResolvedValue(resp());
      renderDrawer();
      await screen.findByText('gameserver');
      await openEventsTab();

      const link = screen.getByText('下载日志').closest('a');
      expect(link).not.toBeNull();
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', 'noreferrer');
      expect(link?.getAttribute('href')).toContain('/api/v1/ops/agents/agent-1/supervisor/logs');
    });

    it('事件拉取失败：空态文案，不崩', async () => {
      mockFetch.mockResolvedValue(resp());
      mockFetchEvents.mockRejectedValue(new Error('tunnel down'));
      renderDrawer();
      await screen.findByText('gameserver');

      fireEvent.click(await screen.findByText('事件日志'));

      expect(await screen.findByText('暂无监管事件')).toBeInTheDocument();
    });

    it('事件为空：空态文案', async () => {
      mockFetch.mockResolvedValue(resp());
      renderDrawer();
      await screen.findByText('gameserver');
      await openEventsTab();
      // openEventsTab 默认有数据，这里单独再验空数组分支
      mockFetchEvents.mockResolvedValue(eventsResp({ events: [] }));
      fireEvent.click(screen.getByText('托管进程'));
      fireEvent.click(await screen.findByText('事件日志'));

      expect(await screen.findByText('暂无监管事件')).toBeInTheDocument();
    });
  });

  describe('S3 崩溃快照', () => {
    it('进程行展示 snapshot:<profile> 标记；none/空档不展示', async () => {
      mockFetch.mockResolvedValue(
        resp({
          processes: [
            RUNNING, // snapshotProfile: jvm
            { ...RUNNING, name: 'plain', flags: [], snapshotProfile: 'none' },
            { ...RUNNING, name: 'unknown', flags: [], snapshotProfile: '' },
          ],
        }),
      );
      renderDrawer();

      expect(await screen.findByText('snapshot:jvm')).toBeInTheDocument();
      // none/空档不渲染快照标记（plain/unknown 行无 snapshot tag）
      expect(screen.queryByText('snapshot:none')).not.toBeInTheDocument();
      expect(screen.queryByText('snapshot:')).not.toBeInTheDocument();
    });

    it('detect_down 行渲染快照产物清单；snapshot_hint 事件落「快照提示」标签', async () => {
      mockFetch.mockResolvedValue(resp());
      renderDrawer();
      await screen.findByText('gameserver');

      const hint: SupervisorEvent = {
        ...EVENT_BASE,
        seq: 8,
        tsUnix: 1759999700,
        event: 'snapshot_hint',
        message: 'core_pattern not pipe mode',
      };
      mockFetchEvents.mockResolvedValue(
        eventsResp({
          events: [
            { ...EV_DETECT, snapshotDir: '/snap/game', snapshotFiles: ['dump.hprof'] },
            hint,
          ],
        }),
      );
      fireEvent.click(await screen.findByText('事件日志'));

      expect(await screen.findByText('快照提示')).toBeInTheDocument();
      expect(screen.getByText('core_pattern not pipe mode')).toBeInTheDocument();
      // detect_down 详情翼带快照目录与产物
      expect(screen.getByText('snapshot: /snap/game (dump.hprof)')).toBeInTheDocument();
    });
  });
});
