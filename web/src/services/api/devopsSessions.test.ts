import { request } from '@umijs/max';
import { createEventSource } from '../core/http';
import {
  closeDevopsSession,
  listDevopsSessions,
  openDevopsSession,
  openDevopsSessionStream,
  resizeDevopsSession,
  sendDevopsSessionInput,
} from './devopsSessions';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));
jest.mock('../core/http', () => ({ createEventSource: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;
const mockedCreate = createEventSource as jest.MockedFunction<typeof createEventSource>;

describe('devopsSessions service', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
    mockedCreate.mockReset();
  });

  it('open posts the session request to the relay path', async () => {
    mockedRequest.mockResolvedValueOnce({ sessionId: 's1', agentId: 'a1' } as never);
    const resp = await openDevopsSession({
      agentId: 'a1',
      protocol: 'ssh',
      host: '10.0.0.11',
      port: 22,
      cols: 120,
      rows: 32,
    });
    expect(resp).toEqual({ sessionId: 's1', agentId: 'a1' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/devops/sessions', {
      method: 'POST',
      data: { agentId: 'a1', protocol: 'ssh', host: '10.0.0.11', port: 22, cols: 120, rows: 32 },
    });
  });

  it('list returns sessions and tolerates a missing field', async () => {
    mockedRequest.mockResolvedValueOnce({
      sessions: [{ sessionId: 's1', agentId: 'a1', operator: 'admin' }],
    } as never);
    await expect(listDevopsSessions()).resolves.toHaveLength(1);
    mockedRequest.mockResolvedValueOnce({} as never);
    await expect(listDevopsSessions()).resolves.toEqual([]);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/devops/sessions');
  });

  it('input/resize skip the global error handler (transient flow control)', async () => {
    mockedRequest.mockResolvedValueOnce(undefined as never);
    await sendDevopsSessionInput('s/1', 'ls\n');
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/devops/sessions/s%2F1/input', {
      method: 'POST',
      data: { data: 'ls\n' },
      skipErrorHandler: true,
    });

    mockedRequest.mockResolvedValueOnce(undefined as never);
    await resizeDevopsSession('s/1', 120, 32);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/devops/sessions/s%2F1/resize', {
      method: 'POST',
      data: { cols: 120, rows: 32 },
      skipErrorHandler: true,
    });
  });

  it('close deletes the session with an audit reason', async () => {
    mockedRequest.mockResolvedValueOnce(undefined as never);
    await closeDevopsSession('s1', 'operator request');
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/devops/sessions/s1', {
      method: 'DELETE',
      data: { reason: 'operator request' },
    });

    mockedRequest.mockResolvedValueOnce(undefined as never);
    await closeDevopsSession('s1');
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/ops/devops/sessions/s1', {
      method: 'DELETE',
      data: { reason: 'panel close' },
    });
  });

  it('stream opens a fetch-SSE on the session stream endpoint', () => {
    openDevopsSessionStream('s/1');
    expect(mockedCreate).toHaveBeenCalledWith('/api/v1/ops/devops/sessions/s%2F1/stream');
  });
});
