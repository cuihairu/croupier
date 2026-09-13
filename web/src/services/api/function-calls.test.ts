import { request } from '@umijs/max';
import { getFunctionCallDetail, getFunctionCallStats, listFunctionCalls } from './function-calls';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('function-calls API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  describe('listFunctionCalls', () => {
    it('passes every list filter as query params and normalizes raw records', async () => {
      mockedRequest.mockResolvedValue({
        calls: [
          {
            id: 'call-1',
            taskId: 'task-1',
            functionId: 'player.ban',
            gameId: 'demo',
            env: 'prod',
            actorId: 'admin',
            actorType: 'admin',
            status: 'success',
            agentId: 'agent-1',
            serviceId: 'service-1',
            startedAt: '2026-01-01T00:00:01Z',
            finishedAt: '2026-01-01T00:00:02Z',
            durationMs: 12,
            payload: { op: 'ban' },
            result: { ok: true },
            errorMsg: null,
            retryCount: 1,
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
        total: 11,
        page: 2,
        pageSize: 50,
      });

      const res = await listFunctionCalls({
        functionId: 'player.ban',
        gameId: 'demo',
        env: 'prod',
        status: 'success',
        actorId: 'admin',
        agentId: 'agent-1',
        startTime: '2026-01-01T00:00:00Z',
        endTime: '2026-01-02T00:00:00Z',
        page: 2,
        pageSize: 50,
      });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function-calls', {
        params: {
          functionId: 'player.ban',
          gameId: 'demo',
          env: 'prod',
          status: 'success',
          actorId: 'admin',
          agentId: 'agent-1',
          startTime: '2026-01-01T00:00:00Z',
          endTime: '2026-01-02T00:00:00Z',
          page: 2,
          pageSize: 50,
        },
      });
      expect(res).toEqual({
        calls: [
          {
            id: 'call-1',
            taskId: 'task-1',
            functionId: 'player.ban',
            gameId: 'demo',
            env: 'prod',
            actorId: 'admin',
            actorType: 'admin',
            status: 'success',
            agentId: 'agent-1',
            serviceId: 'service-1',
            startedAt: '2026-01-01T00:00:01Z',
            finishedAt: '2026-01-01T00:00:02Z',
            durationMs: 12,
            payload: { op: 'ban' },
            result: { ok: true },
            errorMessage: null,
            retryCount: 1,
            createdAt: '2026-01-01T00:00:00Z',
          },
        ],
        total: 11,
        page: 2,
        pageSize: 50,
      });
    });

    it('defaults every filter and pagination field when called without params', async () => {
      mockedRequest.mockResolvedValue({ calls: [] });

      await expect(listFunctionCalls()).resolves.toEqual({
        calls: [],
        total: 0,
        page: 1,
        pageSize: 20,
      });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function-calls', {
        params: {
          functionId: undefined,
          gameId: undefined,
          env: undefined,
          status: undefined,
          actorId: undefined,
          agentId: undefined,
          startTime: undefined,
          endTime: undefined,
          page: undefined,
          pageSize: undefined,
        },
      });
    });

    it('falls back pageSize to params.pageSize before the built-in default', async () => {
      mockedRequest.mockResolvedValue({});

      const withParamPageSize = await listFunctionCalls({ pageSize: 30 });
      expect(withParamPageSize.pageSize).toBe(30);
    });

    it('normalizes missing identity fields to empty strings', async () => {
      mockedRequest.mockResolvedValue({ calls: [{ id: 'call-2', status: 'failed' }] });

      const { calls } = await listFunctionCalls();

      expect(calls[0]).toEqual({
        id: 'call-2',
        taskId: '',
        functionId: '',
        gameId: undefined,
        env: undefined,
        actorId: undefined,
        actorType: undefined,
        status: 'failed',
        agentId: undefined,
        serviceId: undefined,
        startedAt: undefined,
        finishedAt: undefined,
        durationMs: undefined,
        payload: undefined,
        result: undefined,
        errorMessage: undefined,
        retryCount: undefined,
        createdAt: '',
      });
    });
  });

  describe('getFunctionCallDetail', () => {
    it('URL-encodes the call id and maps errorMsg to errorMessage', async () => {
      mockedRequest.mockResolvedValue({
        id: 'call/3',
        status: 'running',
        errorMsg: 'pending',
      });

      const detail = await getFunctionCallDetail('call/3');

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function-calls/call%2F3', {
        method: 'GET',
      });
      expect(detail).toEqual({
        id: 'call/3',
        taskId: '',
        functionId: '',
        gameId: undefined,
        env: undefined,
        actorId: undefined,
        actorType: undefined,
        status: 'running',
        agentId: undefined,
        serviceId: undefined,
        startedAt: undefined,
        finishedAt: undefined,
        durationMs: undefined,
        payload: undefined,
        result: undefined,
        errorMessage: 'pending',
        retryCount: undefined,
        createdAt: '',
      });
    });
  });

  describe('getFunctionCallStats', () => {
    it('passes scope filters through and returns the raw counters', async () => {
      mockedRequest.mockResolvedValue({
        total: 100,
        succeeded: 90,
        failed: 5,
        running: 2,
        cancelled: 2,
        timeout: 1,
        other: 0,
        avgDurationMs: 42,
      });

      const stats = await getFunctionCallStats({
        functionId: 'player.ban',
        gameId: 'demo',
        env: 'prod',
        actorId: 'admin',
        startTime: '2026-01-01T00:00:00Z',
        endTime: '2026-01-02T00:00:00Z',
      });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function-calls/stats', {
        params: {
          functionId: 'player.ban',
          gameId: 'demo',
          env: 'prod',
          actorId: 'admin',
          startTime: '2026-01-01T00:00:00Z',
          endTime: '2026-01-02T00:00:00Z',
        },
      });
      expect(stats).toEqual({
        total: 100,
        succeeded: 90,
        failed: 5,
        running: 2,
        cancelled: 2,
        timeout: 1,
        other: 0,
        avgDurationMs: 42,
      });
    });

    it('defaults every counter to 0 when the response is empty', async () => {
      mockedRequest.mockResolvedValue({});

      await expect(getFunctionCallStats()).resolves.toEqual({
        total: 0,
        succeeded: 0,
        failed: 0,
        running: 0,
        cancelled: 0,
        timeout: 0,
        other: 0,
        avgDurationMs: 0,
      });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function-calls/stats', {
        params: {
          functionId: undefined,
          gameId: undefined,
          env: undefined,
          actorId: undefined,
          startTime: undefined,
          endTime: undefined,
        },
      });
    });
  });
});
