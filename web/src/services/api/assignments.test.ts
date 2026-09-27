import { request } from '@umijs/max';
import { fetchAssignments, fetchAssignmentsHistory, setAssignments } from './assignments';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('assignments API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  describe('fetchAssignments', () => {
    it('GETs /api/v1/assignments with scope params and passes the payload through', async () => {
      const payload = {
        assignments: { prod: ['player.ban'] },
        total: 1,
        page: 1,
        pageSize: 20,
      };
      mockedRequest.mockResolvedValue(payload);

      await expect(fetchAssignments({ gameId: 'demo', env: 'prod' })).resolves.toBe(payload);

      expect(mockedRequest).toHaveBeenCalledTimes(1);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/assignments', {
        params: { gameId: 'demo', env: 'prod' },
      });
    });

    it('forwards params: undefined when called without arguments', async () => {
      mockedRequest.mockResolvedValue({ assignments: {} });

      await fetchAssignments();

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/assignments', { params: undefined });
    });

    it('falls back to an empty pager shape on empty response', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(fetchAssignments()).resolves.toEqual({ total: 0, page: 1, pageSize: 20 });
    });
  });

  describe('setAssignments', () => {
    it('PUTs the mutation body and passes the response through', async () => {
      const payload = { ok: true, unknown: ['fn.missing'], assignments: { prod: ['a', 'b'] } };
      mockedRequest.mockResolvedValue(payload);

      const body = { action: 'assign' as const, targetEnv: 'prod', functions: ['a', 'b'] };
      await expect(setAssignments(body)).resolves.toBe(payload);

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/assignments', {
        method: 'PUT',
        data: body,
      });
    });

    it('falls back to a failed-update shape on empty response', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(setAssignments({ functions: [] })).resolves.toEqual({
        ok: false,
        unknown: [],
        assignments: {},
      });
    });
  });

  describe('fetchAssignmentsHistory', () => {
    it('GETs history with filters and passes the payload through', async () => {
      const payload = {
        items: [
          {
            id: 'h1',
            gameId: 'demo',
            env: 'prod',
            functionId: 'player.ban',
            action: 'assign',
            count: 2,
            operatedBy: 'admin',
            operatedAt: '2026-09-01T00:00:00Z',
            details: { source: 'ui' },
          },
        ],
        total: 1,
        page: 1,
        pageSize: 20,
      };
      mockedRequest.mockResolvedValue(payload);

      const params = { gameId: 'demo', env: 'prod', action: 'assign', page: 1, pageSize: 20 };
      await expect(fetchAssignmentsHistory(params)).resolves.toBe(payload);

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/assignments/history', { params });
    });

    it('keeps caller paging when the response is empty', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(fetchAssignmentsHistory({ page: 5, pageSize: 50 })).resolves.toEqual({
        items: [],
        total: 0,
        page: 5,
        pageSize: 50,
      });
    });

    it('falls back to default paging when called without arguments', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(fetchAssignmentsHistory()).resolves.toEqual({
        items: [],
        total: 0,
        page: 1,
        pageSize: 20,
      });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/assignments/history', {
        params: undefined,
      });
    });

    it('falls back to default paging when params omit page/pageSize', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(fetchAssignmentsHistory({ gameId: 'demo' })).resolves.toEqual({
        items: [],
        total: 0,
        page: 1,
        pageSize: 20,
      });
    });
  });
});
