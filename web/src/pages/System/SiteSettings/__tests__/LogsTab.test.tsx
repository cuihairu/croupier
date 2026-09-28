/**
 * 日志维护子 Tab 单测（OPEN-ISSUES #54）。
 *
 * 锁定契约：retentionDays 回填与来源徽标、保存提交 'log.retentionDays' 键、
 * 清理预设按钮（7 天前 → 168 小时）+ Popconfirm 确认后提交 (scope, hours)、
 * 服务器日志只读渲染、留痕表体量渲染、加载/清理失败 message 提示。
 *
 * mock 口径沿用同目录 PerformanceTab.test.tsx：services 层 jest.mock、
 * @umijs/max 本地 mock；message 经真实 antd App 渲染进 portal。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import LogsTab from '../LogsTab';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/logsMaintenance', () => ({
  fetchLogsMaintenance: jest.fn(),
  saveLogsSettings: jest.fn(),
  cleanupLogs: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

import {
  cleanupLogs,
  fetchLogsMaintenance,
  saveLogsSettings,
} from '@/services/api/logsMaintenance';

const mFetch = fetchLogsMaintenance as jest.MockedFunction<typeof fetchLogsMaintenance>;
const mSave = saveLogsSettings as jest.MockedFunction<typeof saveLogsSettings>;
const mCleanup = cleanupLogs as jest.MockedFunction<typeof cleanupLogs>;

const baseSnapshot = {
  settings: { retentionDays: 0, sources: {} },
  effective: { executionLogDays: 7, taskLogDays: 7 },
  serverLog: {
    output: 'stdout',
    file: '',
    directory: '',
    maxSizeMB: 0,
    maxBackups: 0,
    maxAgeDays: 0,
    compress: false,
    fileCount: 0,
  },
  tables: [
    { table: 'execution_logs', rows: 2, oldestAt: '2026-08-28T00:00:00Z' },
    { table: 'task_runs', rows: 2, oldestAt: '2026-08-28T00:00:00Z' },
    { table: 'task_events', rows: 0, oldestAt: '' },
  ],
};

function renderTab() {
  return render(
    <App>
      <ConfigProvider>
        <LogsTab />
      </ConfigProvider>
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue(JSON.parse(JSON.stringify(baseSnapshot)));
  mSave.mockResolvedValue(JSON.parse(JSON.stringify(baseSnapshot)));
  mCleanup.mockResolvedValue({
    scope: 'all',
    cutoff: '2026-09-21T00:00:00Z',
    executionLogsDeleted: 1,
    taskRunsDeleted: 1,
    taskEventsDeleted: 1,
  });
});

describe('LogsTab（OPEN-ISSUES #54）', () => {
  it('回填 + 来源徽标 + 服务器日志/表体量渲染', async () => {
    mFetch.mockResolvedValue({
      ...baseSnapshot,
      settings: { retentionDays: 30, sources: { retentionDays: 'database' } },
      effective: { executionLogDays: 30, taskLogDays: 30 },
      serverLog: {
        output: 'file',
        file: '/var/log/croupier/croupier.log',
        directory: '/var/log/croupier',
        maxSizeMB: 100,
        maxBackups: 5,
        maxAgeDays: 28,
        compress: true,
        fileCount: 3,
      },
    });
    renderTab();

    expect(await screen.findByDisplayValue(30)).toBeInTheDocument();
    expect(screen.getByText('UI')).toBeInTheDocument();
    // 生效保留期（L3 覆盖 30 → 两类同值）
    expect(screen.getAllByText('30 天')).toHaveLength(2);
    // 服务器日志只读
    expect(screen.getByText('/var/log/croupier')).toBeInTheDocument();
    expect(screen.getByText('file')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    // 表体量
    expect(screen.getByText('execution_logs')).toBeInTheDocument();
    expect(screen.getByText('task_events')).toBeInTheDocument();
  });

  it('保存：提交 log.retentionDays 键并回填新快照', async () => {
    mSave.mockResolvedValue({
      ...baseSnapshot,
      settings: { retentionDays: 14, sources: { retentionDays: 'database' } },
    });
    renderTab();

    const input = await screen.findByDisplayValue(0);
    fireEvent.change(input, { target: { value: '14' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(mSave).toHaveBeenCalledWith({ 'log.retentionDays': 14 }));
    expect(await screen.findByDisplayValue(14)).toBeInTheDocument();
    expect(await screen.findByText('日志参数已保存')).toBeInTheDocument();
  });

  it('手动清理：预设 7 天前 → 168 小时，Popconfirm 确认后提交 (all, 168)', async () => {
    renderTab();

    fireEvent.click(await screen.findByRole('button', { name: '7 天前' }));
    expect(screen.getByDisplayValue(168)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '清理' }));
    fireEvent.click(await screen.findByText('确认清理'));

    await waitFor(() => expect(mCleanup).toHaveBeenCalledWith('all', 168));
    // 成功提示（mock formatMessage 不替换占位符）
    expect(await screen.findByText(/已清理：执行留痕 \{exec\} 条/)).toBeInTheDocument();
    // 清理后刷新快照
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
  });

  it('生效保留期：0 = 永久', async () => {
    mFetch.mockResolvedValue({
      ...baseSnapshot,
      effective: { executionLogDays: 0, taskLogDays: 0 },
    });
    renderTab();

    const forevers = await screen.findAllByText('永久');
    expect(forevers).toHaveLength(2);
  });

  it('加载失败：message 提示、页面不白屏', async () => {
    mFetch.mockRejectedValue(new Error('boom'));
    renderTab();

    expect(await screen.findByText('boom')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '保存' })).toBeInTheDocument();
  });

  it('清理失败：message 提示、按钮退出 loading', async () => {
    mCleanup.mockRejectedValue(new Error('cleanup down'));
    renderTab();

    fireEvent.click(await screen.findByRole('button', { name: '清理' }));
    fireEvent.click(await screen.findByText('确认清理'));

    expect(await screen.findByText('cleanup down')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: '清理' })).not.toBeDisabled());
  });
});
