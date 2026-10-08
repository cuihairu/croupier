/**
 * logsMaintenance API adapter 覆盖收口（web 覆盖率巡检：saveLogsSettings /
 * cleanupLogs 此前零触达；fetchLogsMaintenance 的快照契约一并锁定）。
 */
import { request } from '@umijs/max';
import {
  cleanupLogs,
  fetchLogsMaintenance,
  saveLogsSettings,
  type LogsSnapshot,
} from './logsMaintenance';

jest.mock('@umijs/max', () => ({
  request: jest.fn(),
}));
const mockedRequest = request as jest.MockedFunction<typeof request>;

const SNAPSHOT: LogsSnapshot = {
  settings: { retentionDays: 0, sources: {} },
  effective: { executionLogDays: 30, taskLogDays: 14 },
  serverLog: {
    output: 'file',
    file: 'croupier.log',
    directory: 'logs',
    maxSizeMB: 100,
    maxBackups: 3,
    maxAgeDays: 28,
    compress: false,
    fileCount: 1,
  },
  tables: [],
};

beforeEach(() => {
  mockedRequest.mockReset();
});

describe('logs maintenance adapters', () => {
  it('fetchLogsMaintenance GETs the snapshot without a method override', async () => {
    mockedRequest.mockResolvedValue(SNAPSHOT);

    await expect(fetchLogsMaintenance()).resolves.toEqual(SNAPSHOT);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/logs', { skipErrorHandler: true });
  });

  it('saveLogsSettings PUTs the numeric patch and returns the updated snapshot', async () => {
    mockedRequest.mockResolvedValue(SNAPSHOT);

    await expect(saveLogsSettings({ 'log.maxAge': 30 })).resolves.toEqual(SNAPSHOT);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/logs', {
      method: 'PUT',
      data: { 'log.maxAge': 30 },
      skipErrorHandler: true,
    });
  });

  it('cleanupLogs POSTs scope+beforeHours and returns deletion counts', async () => {
    const result = {
      scope: 'execution' as const,
      cutoff: '2026-10-08T00:00:00Z',
      executionLogsDeleted: 12,
      taskRunsDeleted: 3,
      taskEventsDeleted: 40,
    };
    mockedRequest.mockResolvedValue(result);

    await expect(cleanupLogs('execution', 72)).resolves.toEqual(result);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/logs/cleanup', {
      method: 'POST',
      data: { scope: 'execution', beforeHours: 72 },
      skipErrorHandler: true,
    });
  });
});
