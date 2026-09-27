import { request } from '@umijs/max';
import {
  fetchAssignments,
  fetchAssignmentsHistory,
  setAssignments,
} from './assignments';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

beforeEach(() => mockedRequest.mockReset());

describe('assignments adapters', () => {
  it('fetchAssignments 透传 game/env 查询参数', async () => {
    mockedRequest.mockResolvedValue({
      assignments: { 'demo|prod': ['fn.a'] },
      total: 1,
      page: 1,
      pageSize: 20,
    });

    await expect(fetchAssignments({ gameId: 'demo', env: 'prod' })).resolves.toEqual({
      assignments: { 'demo|prod': ['fn.a'] },
      total: 1,
      page: 1,
      pageSize: 20,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/assignments', {
      params: { gameId: 'demo', env: 'prod' },
    });
  });

  it('fetchAssignments 响应缺失时回退分页兜底对象', async () => {
    // mockReset 后 request 返回 undefined：整个响应缺失也要给出兜底
    await expect(fetchAssignments()).resolves.toEqual({
      total: 0,
      page: 1,
      pageSize: 20,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/assignments', { params: undefined });
  });

  it('setAssignments 以 PUT 提交动作与函数清单', async () => {
    mockedRequest.mockResolvedValue({ ok: true, unknown: [], assignments: {} });

    await expect(
      setAssignments({ action: 'assign', targetEnv: 'prod', functions: ['fn.a', 'fn.b'] }),
    ).resolves.toEqual({ ok: true, unknown: [], assignments: {} });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/assignments', {
      method: 'PUT',
      data: { action: 'assign', targetEnv: 'prod', functions: ['fn.a', 'fn.b'] },
    });
  });

  it('setAssignments 响应缺失时回退失败兜底对象', async () => {
    await expect(setAssignments({ functions: [] })).resolves.toEqual({
      ok: false,
      unknown: [],
      assignments: {},
    });
  });

  it('fetchAssignmentsHistory 透传筛选与分页参数', async () => {
    mockedRequest.mockResolvedValue({
      items: [
        {
          id: 'h1',
          gameId: 'demo',
          env: 'prod',
          functionId: 'all',
          action: 'assign',
          count: 2,
          operatedBy: 'alice',
          operatedAt: '2026-09-27T08:00:00Z',
          details: { before: ['a'], after: ['a', 'b'], added: ['b'], removed: [], unknown: [] },
        },
      ],
      total: 1,
      page: 2,
      pageSize: 50,
    });

    const resp = await fetchAssignmentsHistory({ gameId: 'demo', action: 'assign', page: 2, pageSize: 50 });
    expect(resp.items).toHaveLength(1);
    // details 保持 unknown 值类型：数组不被标量 Record 收窄（#37 diff 渲染依赖）
    expect(resp.items[0].details?.added).toEqual(['b']);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/assignments/history', {
      params: { gameId: 'demo', action: 'assign', page: 2, pageSize: 50 },
    });
  });

  it('fetchAssignmentsHistory 响应缺失时回退空列表与入参分页', async () => {
    await expect(fetchAssignmentsHistory({ page: 3, pageSize: 10 })).resolves.toEqual({
      items: [],
      total: 0,
      page: 3,
      pageSize: 10,
    });
    // 完全无入参时 page/pageSize 走默认值
    await expect(fetchAssignmentsHistory()).resolves.toEqual({
      items: [],
      total: 0,
      page: 1,
      pageSize: 20,
    });
  });
});
