import { request } from '@umijs/max';
import {
  batchUpdateFunctions,
  cancelTask,
  copyFunction,
  deleteAllFunctionWarnings,
  deleteFunction,
  deleteFunctionWarning,
  disableFunction,
  enableFunction,
  fetchTaskResult,
  getFunctionAnalytics,
  getFunctionDetail,
  getFunctionHistory,
  getFunctionOpenAPI,
  getFunctionPermissions,
  invokeFunction,
  listDescriptors,
  listFunctionInstances,
  listFunctionWarnings,
  markAllFunctionWarningsRead,
  markFunctionWarningRead,
  normalizeFunctionDescriptor,
  normalizeFunctionDetail,
  startTask,
  subscribeTaskEvents,
  updateFunction,
  updateFunctionPermissions,
  updateFunctionStatus,
  type FunctionInvokeResponse,
  type TaskEvent,
} from './functions';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

const evt = (seq: number): TaskEvent => ({
  seq,
  type: 'log',
  progress: seq * 10,
  message: `event-${seq}`,
  payload: null,
  createdAt: '2026-01-01T00:00:00Z',
});

describe('normalizeFunctionDescriptor', () => {
  it('should map input field to inputSchema', () => {
    const raw = {
      id: 'test-function',
      input: '{"type":"object","properties":{"name":{"type":"string"}}}',
      output: '{"type":"object","properties":{"result":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.inputSchema).toBe(raw.input);
    expect(result.outputSchema).toBe(raw.output);
  });

  it('should prefer inputSchema over input', () => {
    const raw = {
      id: 'test-function',
      inputSchema: '{"type":"object"}',
      input: '{"type":"object","properties":{"other":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.inputSchema).toBe(raw.inputSchema);
  });

  it('should prefer input_schema over input', () => {
    const raw = {
      id: 'test-function',
      inputSchema: '{"type":"object"}',
      input: '{"type":"object","properties":{"other":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.inputSchema).toBe(raw.inputSchema);
  });

  it('should map output field to outputSchema', () => {
    const raw = {
      id: 'test-function',
      output: '{"type":"object","properties":{"data":{"type":"array"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.outputSchema).toBe(raw.output);
  });

  it('should prefer outputSchema over output', () => {
    const raw = {
      id: 'test-function',
      outputSchema: '{"type":"object"}',
      output: '{"type":"object","properties":{"other":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.outputSchema).toBe(raw.outputSchema);
  });

  it('should prefer outputSchema over output', () => {
    const raw = {
      id: 'test-function',
      outputSchema: '{"type":"object"}',
      output: '{"type":"object","properties":{"other":{"type":"string"}}}',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.outputSchema).toBe(raw.outputSchema);
  });

  it('should handle undefined input/output', () => {
    const raw = {
      id: 'test-function',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.inputSchema).toBeUndefined();
    expect(result.outputSchema).toBeUndefined();
  });

  it('should normalize displayName from string', () => {
    const raw = {
      id: 'test-function',
      displayName: 'Test Function',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.displayName).toEqual({ 'zh-CN': 'Test Function' });
  });

  it('should normalize displayName from object', () => {
    const raw = {
      id: 'test-function',
      displayName: { 'en-US': 'Test Function', 'zh-CN': '测试函数' },
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.displayName).toEqual({ 'en-US': 'Test Function', 'zh-CN': '测试函数' });
  });

  it('should normalize BCP-47 locale keys from registered descriptors', () => {
    const result = normalizeFunctionDescriptor({
      id: 'test-function',
      displayName: { 'zh-CN': '测试函数', 'en-US': 'Test Function' },
    });

    expect(result.displayName).toEqual({ 'en-US': 'Test Function', 'zh-CN': '测试函数' });
  });

  it('should normalize displayName string to LocalizedText', () => {
    const raw = {
      id: 'test-function',
      displayName: 'Test Function',
    };

    const result = normalizeFunctionDescriptor(raw);

    expect(result.displayName).toEqual({ 'zh-CN': 'Test Function' });
  });

  it('should flatten the nested detail descriptor and map name to displayName', () => {
    const result = normalizeFunctionDetail({
      id: 'nested-function',
      name: 'Nested Function',
      description: 'Function description',
      descriptor: {
        input: {
          type: 'object',
          properties: { playerId: { type: 'string' } },
          required: ['playerId'],
        },
        output: { type: 'object', properties: { ok: { type: 'boolean' } } },
        schema: { type: 'object' },
      },
    });

    expect(result.displayName).toEqual({ 'zh-CN': 'Nested Function' });
    expect(result.summary).toEqual({ 'zh-CN': 'Function description' });
    expect(result.inputSchema).toMatchObject({
      type: 'object',
      required: ['playerId'],
    });
    expect(result.outputSchema).toMatchObject({ type: 'object' });
    expect(result.schema).toEqual({ type: 'object' });
  });
});

describe('functions API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  describe('listDescriptors', () => {
    it('normalizes the { functions } envelope', async () => {
      mockedRequest.mockResolvedValue({
        functions: [{ id: 'player.ban', displayName: '封禁玩家' }],
      });

      const result = await listDescriptors();

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/descriptors');
      expect(result).toEqual([
        expect.objectContaining({
          id: 'player.ban',
          displayName: { 'zh-CN': '封禁玩家' },
        }),
      ]);
    });

    it('accepts a bare array response', async () => {
      mockedRequest.mockResolvedValue([{ id: 'a' }, { id: 'b' }]);

      const result = await listDescriptors();

      expect(result.map((f) => f.id)).toEqual(['a', 'b']);
    });

    it('returns an empty list when the envelope has no functions', async () => {
      mockedRequest.mockResolvedValue({});

      await expect(listDescriptors()).resolves.toEqual([]);
    });
  });

  describe('function warnings', () => {
    it('deleteFunctionWarning URL-encodes the key', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await deleteFunctionWarning('warn/1');

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function/warnings/warn%2F1', {
        method: 'DELETE',
      });
    });

    it('markFunctionWarningRead posts the read marker', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await markFunctionWarningRead('warn/1');

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/function/warnings/warn%2F1/read', {
        method: 'POST',
      });
    });

    it('markAllFunctionWarningsRead returns the marked count', async () => {
      mockedRequest.mockResolvedValue({ marked: 3 });

      await expect(markAllFunctionWarningsRead()).resolves.toEqual({ marked: 3 });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/warnings/read-all', {
        method: 'POST',
      });
    });

    it('listFunctionWarnings passes filters and normalizes warning rows', async () => {
      mockedRequest.mockResolvedValue({
        items: [
          {
            key: 'w-1',
            gameId: 'demo',
            env: 'prod',
            agentId: 'agent-1',
            functionId: 'player.ban',
            version: '1.0.0',
            code: 'input_schema_stale',
            message: 'stale schema',
            count: 2,
            firstSeen: '2026-01-01T00:00:00Z',
            lastSeen: '2026-01-02T00:00:00Z',
          },
          { key: 'w-2', code: 'agent_lost', message: 'agent lost', count: 1 },
        ],
      });

      const res = await listFunctionWarnings({
        functionId: 'player.ban',
        agentId: 'agent-1',
        code: 'agent_lost',
        limit: 10,
      });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/warnings', {
        method: 'GET',
        params: { functionId: 'player.ban', agentId: 'agent-1', code: 'agent_lost', limit: 10 },
      });
      expect(res.items[0]).toEqual({
        key: 'w-1',
        gameId: 'demo',
        env: 'prod',
        agentId: 'agent-1',
        functionId: 'player.ban',
        version: '1.0.0',
        code: 'input_schema_stale',
        message: 'stale schema',
        count: 2,
        firstSeen: '2026-01-01T00:00:00Z',
        lastSeen: '2026-01-02T00:00:00Z',
      });
      expect(res.items[1]).toEqual({
        key: 'w-2',
        gameId: '',
        env: '',
        agentId: '',
        functionId: '',
        version: undefined,
        code: 'agent_lost',
        message: 'agent lost',
        count: 1,
        firstSeen: '',
        lastSeen: '',
      });
    });

    it('listFunctionWarnings defaults filters and tolerates a missing items field', async () => {
      mockedRequest.mockResolvedValue({});

      await expect(listFunctionWarnings()).resolves.toEqual({ items: [] });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/warnings', {
        method: 'GET',
        params: { functionId: undefined, agentId: undefined, code: undefined, limit: undefined },
      });
    });

    it('deleteAllFunctionWarnings returns the deleted count', async () => {
      mockedRequest.mockResolvedValue({ deleted: 5 });

      await expect(deleteAllFunctionWarnings()).resolves.toEqual({ deleted: 5 });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/warnings', {
        method: 'DELETE',
      });
    });
  });

  describe('invokeFunction and startTask', () => {
    it('posts only the payload when no options are given', async () => {
      const resp: FunctionInvokeResponse = { result: { ok: true }, traceId: 't-1' };
      mockedRequest.mockResolvedValue(resp);

      await expect(invokeFunction('player/ban', { op: 'ban' })).resolves.toBe(resp);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player%2Fban/invoke', {
        method: 'POST',
        data: { payload: { op: 'ban' } },
      });
    });

    it('spreads routing options onto the request body', async () => {
      mockedRequest.mockResolvedValue({});

      await invokeFunction('fn-1', null, {
        route: 'targeted',
        targetServiceId: 'svc-1',
        hashKey: 'player-1',
        mode: 'async',
      });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1/invoke', {
        method: 'POST',
        data: {
          payload: null,
          route: 'targeted',
          targetServiceId: 'svc-1',
          hashKey: 'player-1',
          mode: 'async',
        },
      });
    });

    it('startTask forces async mode', async () => {
      mockedRequest.mockResolvedValue({ taskId: 'task-1' });

      await startTask('fn-1', { op: 1 });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1/invoke', {
        method: 'POST',
        data: { payload: { op: 1 }, mode: 'async' },
      });
    });

    it('startTask keeps routing options', async () => {
      mockedRequest.mockResolvedValue({});

      await startTask('fn-1', null, { route: 'hash', hashKey: 'k', targetServiceId: 'svc' });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1/invoke', {
        method: 'POST',
        data: {
          payload: null,
          mode: 'async',
          route: 'hash',
          hashKey: 'k',
          targetServiceId: 'svc',
        },
      });
    });
  });

  describe('task API', () => {
    it('cancelTask posts to the task cancel route with an encoded id', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await cancelTask('task/9');

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tasks/task%2F9/cancel', {
        method: 'POST',
      });
    });

    it('fetchTaskResult projects the task detail into { state, payload, error }', async () => {
      mockedRequest.mockResolvedValue({ status: 'succeeded', result: { ok: 1 }, error: '' });

      await expect(fetchTaskResult('t-1')).resolves.toEqual({
        state: 'succeeded',
        payload: { ok: 1 },
        error: '',
      });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tasks/t-1', { method: 'GET' });
    });
  });

  describe('listFunctionInstances', () => {
    it('prefers items over instances and normalizes entries', async () => {
      mockedRequest.mockResolvedValue({
        items: [{ functionId: 'f-1', status: 'active' }],
        instances: [{ functionId: 'wrong' }],
      });

      const res = await listFunctionInstances({ functionId: 'f-1' });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/f-1/instances', {
        method: 'GET',
      });
      expect(res.instances).toEqual([
        expect.objectContaining({ functionId: 'f-1', status: 'running' }),
      ]);
    });

    it('falls back to the instances envelope', async () => {
      mockedRequest.mockResolvedValue({
        instances: [{ functionId: 'f-2', status: 'inactive' }],
      });

      const res = await listFunctionInstances({ functionId: 'f-2' });

      expect(res.instances).toEqual([
        expect.objectContaining({ functionId: 'f-2', status: 'stopped' }),
      ]);
    });

    it('returns an empty list when the envelope carries neither field', async () => {
      mockedRequest.mockResolvedValue({});

      await expect(listFunctionInstances({ functionId: 'f-3' })).resolves.toEqual({
        instances: [],
      });
    });
  });

  describe('function permissions', () => {
    it('getFunctionPermissions GETs permissions with an encoded id', async () => {
      const perms = { items: [{ resource: 'player', actions: ['read'], roles: ['gm'] }] };
      mockedRequest.mockResolvedValue(perms);

      await expect(getFunctionPermissions('player/ban')).resolves.toBe(perms);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player%2Fban/permissions', {
        method: 'GET',
      });
    });

    it('updateFunctionPermissions PUTs the permission list', async () => {
      const permissions = [{ resource: 'player', actions: ['read'], roles: ['gm'] }];
      mockedRequest.mockResolvedValue({});

      await updateFunctionPermissions('player.ban', permissions);

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player.ban/permissions', {
        method: 'PUT',
        data: { permissions },
      });
    });
  });

  describe('status and batch operations', () => {
    it('updateFunctionStatus PUTs the enabled flag', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await updateFunctionStatus('fn-1', { enabled: false });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1/status', {
        method: 'PUT',
        data: { enabled: false },
      });
    });

    it('enableFunction and disableFunction POST their toggle routes', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await enableFunction('fn-1');
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1/enable', {
        method: 'POST',
      });

      await disableFunction('fn-1');
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1/disable', {
        method: 'POST',
      });
    });

    it('batchUpdateFunctions posts ids and the enabled flag', async () => {
      mockedRequest.mockResolvedValue({ updated: 2, failed: [] });

      await batchUpdateFunctions({ functionIds: ['a', 'b'], enabled: false });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/batch-update', {
        method: 'POST',
        data: { functionIds: ['a', 'b'], enabled: false },
      });
    });

    it('copyFunction returns the id pair from the response', async () => {
      mockedRequest.mockResolvedValue({ functionId: 'old', newId: 'new-1' });

      await expect(copyFunction('old')).resolves.toEqual({ functionId: 'old', newId: 'new-1' });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/old/copy', {
        method: 'POST',
      });
    });

    it('deleteFunction DELETEs with an encoded id', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await deleteFunction('player/ban');

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player%2Fban', {
        method: 'DELETE',
      });
    });
  });

  describe('getFunctionDetail (detail endpoint)', () => {
    it('flattens the nested descriptor and falls back displayName to name', async () => {
      mockedRequest.mockResolvedValue({
        id: 'fn-1',
        name: '批量邮件',
        descriptor: {
          input: { type: 'object' },
          output: { type: 'array' },
          schema: { type: 'object' },
        },
      });

      const detail = await getFunctionDetail('fn-1');

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1');
      expect(detail.displayName).toEqual({ 'zh-CN': '批量邮件' });
      expect(detail.inputSchema).toEqual({ type: 'object' });
      expect(detail.outputSchema).toEqual({ type: 'array' });
      expect(detail.schema).toEqual({ type: 'object' });
    });

    it('keeps an explicit displayName without the name fallback', async () => {
      mockedRequest.mockResolvedValue({
        id: 'fn-2',
        name: '旧名字',
        displayName: '正式名字',
        summary: { 'zh-CN': '摘要', 'en-US': 'Summary' },
      });

      const detail = await getFunctionDetail('fn-2');

      expect(detail.displayName).toEqual({ 'zh-CN': '正式名字' });
      expect(detail.summary).toEqual({ 'zh-CN': '摘要', 'en-US': 'Summary' });
    });
  });

  describe('getFunctionHistory', () => {
    it('passes pagination params and returns items with total', async () => {
      const items = [{ id: 'h-1', action: 'update', timestamp: '2026-01-01T00:00:00Z' }];
      mockedRequest.mockResolvedValue({ items, total: 7 });

      const res = await getFunctionHistory('fn-1', { limit: 10, offset: 20 });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1/history', {
        params: { limit: 10, offset: 20 },
      });
      expect(res).toEqual({ items, total: 7 });
    });

    it('falls back total to items.length', async () => {
      mockedRequest.mockResolvedValue({
        items: [{ id: 'h-1', action: 'update', timestamp: 't' }],
      });

      await expect(getFunctionHistory('fn-1')).resolves.toEqual({
        items: [{ id: 'h-1', action: 'update', timestamp: 't' }],
        total: 1,
      });
    });

    it('returns an empty page when the response has no items', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await expect(getFunctionHistory('fn-1')).resolves.toEqual({ items: [], total: 0 });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1/history', {
        params: undefined,
      });
    });
  });

  describe('analytics and updates', () => {
    it('getFunctionAnalytics GETs the analytics payload', async () => {
      const analytics = {
        totalCalls: 10,
        successRate: 0.9,
        avgLatency: 5,
        callsToday: 1,
        callsThisWeek: 2,
        callsThisMonth: 3,
      };
      mockedRequest.mockResolvedValue(analytics);

      await expect(getFunctionAnalytics('fn-1')).resolves.toBe(analytics);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1/analytics');
    });

    it('updateFunction PUTs the editable fields', async () => {
      mockedRequest.mockResolvedValue(undefined);
      const data = {
        name: '新名字',
        description: '描述',
        resource: 'player',
        tags: ['gm'],
        enabled: true,
      };

      await updateFunction('fn-1', data);

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/fn-1', {
        method: 'PUT',
        data,
      });
    });
  });

  describe('getFunctionOpenAPI (functions adapter)', () => {
    it('unwraps the spec envelope', async () => {
      mockedRequest.mockResolvedValue({ spec: { operationId: 'player.ban' } });

      await expect(getFunctionOpenAPI('player.ban')).resolves.toEqual({
        operationId: 'player.ban',
      });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/functions/player.ban/openapi');
    });
  });
});

describe('subscribeTaskEvents', () => {
  beforeEach(() => {
    jest.useFakeTimers();
    mockedRequest.mockReset();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it('replays events, advances afterSeq and stops on done', async () => {
    const onEvent = jest.fn();
    const onDone = jest.fn();
    mockedRequest
      .mockResolvedValueOnce({ items: [evt(1)], nextSeq: 1, done: false })
      .mockResolvedValueOnce({ items: [evt(2)], nextSeq: 2, done: true });

    const sub = subscribeTaskEvents('task/1', { onEvent, onDone });
    await jest.advanceTimersByTimeAsync(0);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tasks/task%2F1/events', {
      params: { afterSeq: 0 },
    });
    expect(onEvent).toHaveBeenCalledWith(evt(1));

    await jest.advanceTimersByTimeAsync(1500);

    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/tasks/task%2F1/events', {
      params: { afterSeq: 1 },
    });
    expect(onEvent).toHaveBeenCalledWith(evt(2));
    expect(onDone).toHaveBeenCalled();

    // done 之后不再轮询
    await jest.advanceTimersByTimeAsync(10000);
    expect(mockedRequest).toHaveBeenCalledTimes(2);
    sub.close();
  });

  it('keeps afterSeq when nextSeq is missing', async () => {
    mockedRequest
      .mockResolvedValueOnce({ items: [] })
      .mockResolvedValueOnce({ items: [], nextSeq: 2, done: true });

    const sub = subscribeTaskEvents('t-1', {});
    await jest.advanceTimersByTimeAsync(0);
    await jest.advanceTimersByTimeAsync(1500);

    expect(mockedRequest).toHaveBeenCalledTimes(2);
    expect(mockedRequest.mock.calls[1][1]).toEqual({ params: { afterSeq: 0 } });
    sub.close();
  });

  it('reports poll errors and stops retrying', async () => {
    const err = new Error('network down');
    mockedRequest.mockRejectedValueOnce(err);
    const onError = jest.fn();

    const sub = subscribeTaskEvents('t-1', { onError });
    await jest.advanceTimersByTimeAsync(0);

    expect(onError).toHaveBeenCalledWith(err);
    await jest.advanceTimersByTimeAsync(10000);
    expect(mockedRequest).toHaveBeenCalledTimes(1);
    sub.close();
  });

  it('stops polling after close clears the pending timer', async () => {
    mockedRequest.mockResolvedValue({ items: [], nextSeq: 5, done: false });
    const onEvent = jest.fn();

    const sub = subscribeTaskEvents('t-1', { onEvent });
    await jest.advanceTimersByTimeAsync(0);
    expect(mockedRequest).toHaveBeenCalledTimes(1);

    sub.close();
    await jest.advanceTimersByTimeAsync(10000);
    expect(mockedRequest).toHaveBeenCalledTimes(1);
  });

  it('ignores resolutions that land after close', async () => {
    let resolvePoll!: (value: { items?: TaskEvent[]; nextSeq?: number; done?: boolean }) => void;
    mockedRequest.mockImplementationOnce(
      () =>
        new Promise<{ items?: TaskEvent[]; nextSeq?: number; done?: boolean }>((resolve) => {
          resolvePoll = resolve;
        }),
    );
    const onEvent = jest.fn();
    const onDone = jest.fn();

    const sub = subscribeTaskEvents('t-1', { onEvent, onDone });
    sub.close();
    await jest.advanceTimersByTimeAsync(0);
    resolvePoll({ items: [evt(9)], nextSeq: 9, done: false });
    await jest.advanceTimersByTimeAsync(5000);

    expect(onEvent).not.toHaveBeenCalled();
    expect(onDone).not.toHaveBeenCalled();
    expect(mockedRequest).toHaveBeenCalledTimes(1);
  });

  it('swallows errors that land after close', async () => {
    let rejectPoll!: (reason: Error) => void;
    mockedRequest.mockImplementationOnce(
      () =>
        new Promise<never>((_resolve, reject) => {
          rejectPoll = reject;
        }),
    );
    const onError = jest.fn();

    const sub = subscribeTaskEvents('t-1', { onError });
    sub.close();
    await jest.advanceTimersByTimeAsync(0);
    rejectPoll(new Error('late failure'));
    await jest.advanceTimersByTimeAsync(5000);

    expect(onError).not.toHaveBeenCalled();
    expect(mockedRequest).toHaveBeenCalledTimes(1);
  });

  it('tolerates missing handlers and stops on done', async () => {
    mockedRequest.mockResolvedValueOnce({ items: [evt(1)], nextSeq: 1, done: true });

    const sub = subscribeTaskEvents('t-1');
    await jest.advanceTimersByTimeAsync(0);
    expect(mockedRequest).toHaveBeenCalledTimes(1);

    await jest.advanceTimersByTimeAsync(3000);
    expect(mockedRequest).toHaveBeenCalledTimes(1);
    sub.close();
  });

  it('treats a response without items as an empty batch', async () => {
    mockedRequest
      .mockResolvedValueOnce({ nextSeq: 1, done: false })
      .mockResolvedValueOnce({ done: true });

    const onEvent = jest.fn();
    const sub = subscribeTaskEvents('t-1', { onEvent });
    await jest.advanceTimersByTimeAsync(0);
    await jest.advanceTimersByTimeAsync(1500);

    expect(onEvent).not.toHaveBeenCalled();
    expect(mockedRequest).toHaveBeenCalledTimes(2);
    sub.close();
  });

  it('stops rescheduling when close runs inside an event handler', async () => {
    mockedRequest.mockResolvedValue({ items: [evt(1)], nextSeq: 1, done: false });
    let closeFn: () => void = () => {};
    const onEvent = jest.fn(() => closeFn());

    const sub = subscribeTaskEvents('t-1', { onEvent });
    closeFn = () => sub.close();

    await jest.advanceTimersByTimeAsync(0);
    expect(onEvent).toHaveBeenCalledWith(evt(1));

    await jest.advanceTimersByTimeAsync(10000);
    expect(mockedRequest).toHaveBeenCalledTimes(1);
  });
});
