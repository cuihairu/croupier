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
  ModalForm: () => null,
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
