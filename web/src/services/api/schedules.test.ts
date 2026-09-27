/**
 * /ops/schedules 适配层回归（#24 覆盖率补缺）：
 * 页面套件整模块 mock 了 '@/services/api/schedules'，适配器本身此前 0% 覆盖。
 * 六个函数均为纯透传（无分支），用例断言路由/方法/载荷逐一触达即全量覆盖。
 */
import { request } from '@umijs/max';
import {
  createSchedule,
  deleteSchedule,
  listScheduleRuns,
  listSchedules,
  setScheduleStatus,
  triggerScheduleNow,
} from './schedules';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('schedules adapters (#24)', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('listSchedules passes filter params through', async () => {
    const payload = { items: [], total: 0 };
    mockedRequest.mockResolvedValue(payload);
    const params = { status: 'active', pageSize: 100 };

    await expect(listSchedules(params)).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/schedules', { params });
  });

  it('createSchedule POSTs the payload', async () => {
    const body = { name: 'n1', cronExpr: '* * * * *', functionId: 'f1', maxFailedRuns: 5 };
    mockedRequest.mockResolvedValue({ item: { id: 9 } });

    await expect(createSchedule(body)).resolves.toEqual({ item: { id: 9 } });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/schedules', { method: 'POST', data: body });
  });

  it('setScheduleStatus PUTs the status body', async () => {
    mockedRequest.mockResolvedValue({ item: { id: 2 } });

    await setScheduleStatus(2, 'paused');
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/schedules/2/status', {
      method: 'PUT',
      data: { status: 'paused' },
    });
  });

  it('triggerScheduleNow POSTs the trigger route', async () => {
    mockedRequest.mockResolvedValue({ taskRunId: 'tr-9' });

    await expect(triggerScheduleNow(1)).resolves.toEqual({ taskRunId: 'tr-9' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/schedules/1/trigger', { method: 'POST' });
  });

  it('deleteSchedule DELETEs the id route', async () => {
    mockedRequest.mockResolvedValue({ ok: true });

    await deleteSchedule(3);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/schedules/3', { method: 'DELETE' });
  });

  it('listScheduleRuns passes paging params through', async () => {
    const payload = { items: [], total: 0 };
    mockedRequest.mockResolvedValue(payload);

    await expect(listScheduleRuns(1, { pageSize: 50 })).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/schedules/1/runs', {
      params: { pageSize: 50 },
    });
  });
});
