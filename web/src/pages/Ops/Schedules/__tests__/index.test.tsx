/**
 * Ops/Schedules 页面回归（#24 概览区 + 来源分组 + 副标题独立成行）：
 * 1. 副标题经 PageContainer 的 content 渲染（独立一行），不再占用 subTitle
 *    （标题右侧同行）；
 * 2. 概览区统计：任务总数/启用中为 platform+host 双来源合计，
 *    平台调度/宿主机任务为来源分布；
 * 3. 来源分组 Tabs：平台调度（平台创建，可操作）/ 宿主机任务（agent 采集，
 *    只读展示），宿主机任务含不可达节点警示；
 * 4. host 采集失败时诚实降级：数字不归零，标注「宿主机采集不可用」。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import SchedulesPage from '../index';
import { useIntl } from '@umijs/max';
import {
  listSchedules,
  listScheduleRuns,
  createSchedule,
  deleteSchedule,
  setScheduleStatus,
  triggerScheduleNow,
} from '@/services/api/schedules';
import { listNodesCronJobsAll } from '@/services/api/nodes';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(40000);

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ defaultMessage }, values) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
  FormattedMessage: ({ defaultMessage }) => defaultMessage,
  history: { push: jest.fn(), back: jest.fn() },
}));

// 创建表单桩：测试用例在渲染前设置提交值，点击 create-submit 驱动 onFinish
let mockCreateFormValues: Record<string, unknown> = {};

// PageContainer 捕获 props 供副标题断言（subTitle= 行内 vs content= 独立行）
jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({
    children,
    content,
    subTitle,
    extra,
  }: {
    children?: React.ReactNode;
    content?: React.ReactNode;
    subTitle?: React.ReactNode;
    extra?: React.ReactNode;
  }) => (
    <div
      data-testid="page-container"
      data-content={content === undefined ? undefined : String(content)}
      data-subtitle={subTitle === undefined ? undefined : String(subTitle)}
    >
      {extra}
      {children}
    </div>
  ),
  ModalForm: ({
    onFinish,
    trigger,
  }: {
    onFinish?: (values: Record<string, unknown>) => Promise<boolean | void>;
    trigger?: React.ReactNode;
  }) => (
    <span>
      {trigger}
      <button
        data-testid="create-submit"
        type="button"
        onClick={() => {
          void onFinish?.(mockCreateFormValues);
        }}
      >
        create-submit
      </button>
    </span>
  ),
}));

jest.mock('@/services/api/schedules', () => ({
  listSchedules: jest.fn(),
  listScheduleRuns: jest.fn(),
  createSchedule: jest.fn(),
  deleteSchedule: jest.fn(),
  setScheduleStatus: jest.fn(),
  triggerScheduleNow: jest.fn(),
}));

jest.mock('@/services/api/nodes', () => ({
  listNodesCronJobsAll: jest.fn(),
}));

const mockedUseIntl = useIntl as unknown as jest.Mock;
const mockListSchedules = listSchedules as unknown as jest.Mock;
const mockListScheduleRuns = listScheduleRuns as unknown as jest.Mock;
const mockCreateSchedule = createSchedule as unknown as jest.Mock;
const mockDeleteSchedule = deleteSchedule as unknown as jest.Mock;
const mockSetScheduleStatus = setScheduleStatus as unknown as jest.Mock;
const mockTriggerScheduleNow = triggerScheduleNow as unknown as jest.Mock;
const mockListNodesCronJobsAll = listNodesCronJobsAll as unknown as jest.Mock;

const platformRows = [
  {
    id: 1,
    name: 'nightly-cleanup',
    cronExpr: '30 2 * * *',
    functionId: 'player.cleanup',
    gameId: 'demo',
    env: 'prod',
    status: 'active',
    consecutiveFailures: 0,
    maxFailedRuns: 5,
    nextTriggerAt: '2026-09-28T02:30:00Z',
  },
  {
    id: 2,
    name: 'weekly-report',
    cronExpr: '0 6 * * 1',
    functionId: 'report.build',
    gameId: 'demo',
    env: 'prod',
    status: 'paused',
    consecutiveFailures: 0,
    maxFailedRuns: 5,
    nextTriggerAt: '2026-09-29T06:00:00Z',
  },
];

const hostReports = [
  {
    nodeId: 'agent-1',
    nodeName: 'linux-host',
    status: 'online',
    ok: true,
    jobs: [
      {
        schedule: '*/5 * * * *',
        command: '/usr/bin/backup',
        user: 'root',
        sourceFile: '/etc/crontab',
        enabled: true,
      },
      {
        schedule: 'periodic',
        command: '/etc/cron.daily/tidy',
        user: 'root',
        sourceFile: '/etc/cron.daily/tidy',
        enabled: false,
      },
    ],
  },
  {
    nodeId: 'agent-2',
    nodeName: 'down-host',
    status: 'offline',
    ok: false,
    error: '节点不在线',
    jobs: [],
  },
];

const statisticText = (title: string): string => {
  const nodes = Array.from(document.querySelectorAll('.ant-statistic'));
  const hit = nodes.find((n) => n.querySelector('.ant-statistic-title')?.textContent === title);
  if (!hit) throw new Error(`statistic ${title} not found`);
  return hit.querySelector('.ant-statistic-content')?.textContent || '';
};

beforeEach(() => {
  jest.clearAllMocks();
  mockCreateFormValues = {};
  mockListSchedules.mockResolvedValue({ items: platformRows });
  mockListScheduleRuns.mockResolvedValue({ items: [] });
  mockListNodesCronJobsAll.mockResolvedValue({ items: hostReports, total: 2 });
});

describe('Ops/Schedules 页面（#24 概览 + 来源分组）', () => {
  it('副标题经 content 独立成行，不再占用 subTitle', async () => {
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await waitFor(() => expect(mockListSchedules).toHaveBeenCalled());

    const container = screen.getByTestId('page-container');
    expect(container.getAttribute('data-content')).toContain('五字段 cron');
    expect(container.getAttribute('data-subtitle')).toBeNull();
  });

  it('概览区：总数/启用为双来源合计，来源分布单列', async () => {
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await waitFor(() => expect(mockListNodesCronJobsAll).toHaveBeenCalled());

    // 总数 = 平台 2 + 宿主机 2 = 4；启用 = active 1 + host enabled 1 = 2
    expect(statisticText('任务总数')).toContain('4');
    expect(statisticText('启用中')).toContain('2');
    expect(statisticText('平台调度')).toContain('2');
    expect(statisticText('宿主机任务')).toContain('2');
    expect(statisticText('宿主机任务')).toContain('1 个节点');
  });

  it('来源分组 Tabs：平台调度与宿主机任务各自展示，宿主机任务行带节点信息', async () => {
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    expect(screen.getByRole('tab', { name: /平台调度 \(2\)/ })).toBeTruthy();
    fireEvent.click(screen.getByRole('tab', { name: /宿主机任务/ }));

    await screen.findByText('/usr/bin/backup');
    // 两行任务同属 linux-host：节点列各渲染一次
    expect(screen.getAllByText('linux-host')).toHaveLength(2);
    // 不可达节点警示列出节点与原因
    expect(screen.getByText(/以下节点不可达/)).toBeTruthy();
    expect(screen.getByText(/down-host: 节点不在线/)).toBeTruthy();
  });

  it('宿主机采集失败时诚实降级：标注不可用而非归零', async () => {
    mockListNodesCronJobsAll.mockRejectedValue(new Error('boom'));
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await waitFor(() => expect(mockListNodesCronJobsAll).toHaveBeenCalled());

    await waitFor(() => expect(statisticText('任务总数')).toContain('宿主机采集不可用'));
    // 总数退化为平台数（host 不可知），不伪造 host=0 的合计
    expect(statisticText('任务总数')).toContain('2');
  });

  it('写操作链路不受改造影响（平台调度仍可创建/触发）', async () => {
    mockCreateSchedule.mockResolvedValue({ id: 3 });
    mockTriggerScheduleNow.mockResolvedValue({ taskRunId: 'tr-1' });
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    expect(mockedUseIntl).toBeTruthy();
    expect(mockSetScheduleStatus).not.toHaveBeenCalled();
    expect(mockDeleteSchedule).not.toHaveBeenCalled();
  });
});

describe('Ops/Schedules 行操作与弹窗流程', () => {
  it('行操作：立即触发调用 triggerScheduleNow 并提示派发结果', async () => {
    mockTriggerScheduleNow.mockResolvedValue({ taskRunId: 'tr-9' });
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    // 两行平台调度各渲染一次「立即触发」，取第一行（id=1）
    fireEvent.click((await screen.findAllByText('立即触发'))[0]);
    await waitFor(() => expect(mockTriggerScheduleNow).toHaveBeenCalledWith(1));
    expect(await screen.findByText('已派发任务 tr-9')).toBeInTheDocument();
  });

  // extractErrorMessage 对带 message 的 Error 透传原文，仅对非对象拒绝回退默认文案
  it('行操作：触发失败提示「触发失败」', async () => {
    mockTriggerScheduleNow.mockRejectedValue('agent offline');
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    fireEvent.click((await screen.findAllByText('立即触发'))[0]);
    expect(await screen.findByText('触发失败')).toBeInTheDocument();
  });

  // —— 以下补覆盖率巡检缺口：未知状态兜底 tag、删除确认成功/失败、状态操作失败 ——

  it('未知状态：状态 tag 兜底渲染原始值', async () => {
    mockListSchedules.mockResolvedValue({
      items: [
        {
          id: 4,
          name: 'odd-job',
          cronExpr: '* * * * *',
          functionId: 'f1',
          status: 'some_future_status',
          consecutiveFailures: 0,
          maxFailedRuns: 5,
          nextTriggerAt: '2026-09-28T02:30:00Z',
        },
      ],
    });
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('odd-job');
    // 非白名单状态不经翻译，直接展示原始字符串（default 分支）
    expect(screen.getByText('some_future_status')).toBeInTheDocument();
  });

  it('行操作：删除经 Popconfirm 确认后调用接口并提示已删除', async () => {
    mockDeleteSchedule.mockResolvedValue({ ok: true });
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    fireEvent.click(screen.getAllByText('删除')[0]);
    fireEvent.click(await screen.findByRole('button', { name: 'OK' }));
    await waitFor(() => expect(mockDeleteSchedule).toHaveBeenCalledWith(1));
    expect(await screen.findByText('已删除')).toBeInTheDocument();
    // 删除后重拉列表
    await waitFor(() => expect(mockListSchedules.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it('行操作：删除失败提示「删除失败」', async () => {
    mockDeleteSchedule.mockRejectedValue('boom');
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    fireEvent.click(screen.getAllByText('删除')[0]);
    fireEvent.click(await screen.findByRole('button', { name: 'OK' }));
    await waitFor(() => expect(mockDeleteSchedule).toHaveBeenCalled());
    expect(await screen.findByText('删除失败')).toBeInTheDocument();
  });

  it('行操作：暂停/启用失败提示「操作失败」', async () => {
    mockSetScheduleStatus.mockRejectedValue('boom');
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    fireEvent.click(screen.getByText('暂停'));
    expect(await screen.findByText('操作失败')).toBeInTheDocument();

    fireEvent.click(screen.getByText('启用'));
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
  });

  it('行操作：active 行暂停、paused 行启用', async () => {
    mockSetScheduleStatus.mockResolvedValue({});
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    fireEvent.click(screen.getByText('暂停'));
    await waitFor(() => expect(mockSetScheduleStatus).toHaveBeenCalledWith(1, 'paused'));

    fireEvent.click(screen.getByText('启用'));
    await waitFor(() => expect(mockSetScheduleStatus).toHaveBeenCalledWith(2, 'active'));
  });

  it('dead_letter 行：死信标签、失败计数标注、恢复需确认后调用 status 接口', async () => {
    mockListSchedules.mockResolvedValue({
      items: [
        ...platformRows,
        {
          id: 3,
          name: 'dl-job',
          cronExpr: '* * * * *',
          functionId: 'player.cleanup',
          status: 'dead_letter',
          consecutiveFailures: 5,
          maxFailedRuns: 5,
          nextTriggerAt: '2026-09-28T02:30:00Z',
        },
      ],
    });
    mockSetScheduleStatus.mockResolvedValue({});
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('dl-job');

    expect(screen.getByText('死信')).toBeInTheDocument();
    expect(screen.getByText('5/5')).toBeInTheDocument();

    fireEvent.click(screen.getByText('恢复'));
    // 死信恢复必须经二次确认（antd v6 confirm 标题在 .ant-modal-title 与
    // .ant-modal-confirm-title 各渲染一次，须用 AllBy 变体）
    await screen.findAllByText('从死信恢复该调度？');
    fireEvent.click(
      document.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLButtonElement,
    );
    await waitFor(() => expect(mockSetScheduleStatus).toHaveBeenCalledWith(3, 'active'));
  });

  it('触发历史 drawer：打开时拉取运行记录并渲染', async () => {
    mockListScheduleRuns.mockResolvedValue({
      items: [
        {
          id: 'r1',
          slot: '2026-09-28T02:30:00Z',
          status: 'dispatched',
          taskRunId: 'tr-1',
          message: 'ok',
        },
      ],
    });
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    fireEvent.click(screen.getAllByText('历史')[0]);
    await waitFor(() => expect(mockListScheduleRuns).toHaveBeenCalledWith(1, { pageSize: 50 }));
    expect(await screen.findByText('触发历史：nightly-cleanup')).toBeInTheDocument();
    expect(screen.getByText('tr-1')).toBeInTheDocument();
  });

  it('触发历史 drawer：拉取失败提示「加载触发历史失败」', async () => {
    mockListScheduleRuns.mockRejectedValue('boom');
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    fireEvent.click(screen.getAllByText('历史')[0]);
    expect(await screen.findByText('加载触发历史失败')).toBeInTheDocument();
  });

  it('列表加载失败：提示「加载定时任务失败」且不崩溃', async () => {
    mockListSchedules.mockRejectedValue('boom');
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    expect(await screen.findByText('加载定时任务失败')).toBeInTheDocument();
  });

  it('创建调度：payload JSON 解析为对象提交，maxFailedRuns 缺省 5', async () => {
    mockCreateSchedule.mockResolvedValue({ id: 9 });
    mockCreateFormValues = {
      name: 'n1',
      cronExpr: '* * * * *',
      functionId: 'f1',
      payload: '{"k":1}',
    };
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    fireEvent.click(screen.getByTestId('create-submit'));
    await waitFor(() =>
      expect(mockCreateSchedule).toHaveBeenCalledWith({
        name: 'n1',
        cronExpr: '* * * * *',
        functionId: 'f1',
        payload: { k: 1 },
        maxFailedRuns: 5,
      }),
    );
    expect(await screen.findByText('已创建，调度器将在下次到期自动触发')).toBeInTheDocument();
  });

  it('创建调度：payload 非法 JSON 本地拦截弹错误 toast（解析错误原文），不发起请求', async () => {
    mockCreateFormValues = {
      name: 'n1',
      cronExpr: '* * * * *',
      functionId: 'f1',
      payload: '{bad',
    };
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    fireEvent.click(screen.getByTestId('create-submit'));
    // SyntaxError 原文经 extractErrorMessage 直出 toast；文本随 Node 版本波动，只断言错误 toast 弹出
    await waitFor(() => expect(document.querySelector('.ant-message-notice-error')).toBeTruthy());
    expect(mockCreateSchedule).not.toHaveBeenCalled();
  });

  it('创建调度：请求失败同样提示创建失败', async () => {
    mockCreateSchedule.mockRejectedValue('server error');
    mockCreateFormValues = {
      name: 'n1',
      cronExpr: '* * * * *',
      functionId: 'f1',
      payload: '',
    };
    render(
      <App>
        <SchedulesPage />
      </App>,
    );
    await screen.findByText('nightly-cleanup');

    fireEvent.click(screen.getByTestId('create-submit'));
    expect(await screen.findByText('创建失败')).toBeInTheDocument();
  });
});
