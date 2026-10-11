import { request } from '@umijs/max';
import {
  getCaptureGates,
  listCaptureGatesVersions,
  publishCaptureGates,
  rollbackCaptureGates,
} from './opsCapture';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// Authorization 由 requestErrorConfig 的请求拦截器统一注入；顶栏 game/env
// scope 头同样走全局拦截器——适配层只负责 URL/method/参数形状。
describe('opsCapture service', () => {
  it('getCaptureGates: GET /api/v1/ops/capture/gates', async () => {
    mockedRequest.mockResolvedValueOnce({
      key: 'capture.gates',
      version: 2,
      content: '{}',
    });
    await expect(getCaptureGates()).resolves.toEqual({
      key: 'capture.gates',
      version: 2,
      content: '{}',
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/capture/gates');
  });

  it('publishCaptureGates: PUT with content+message', async () => {
    mockedRequest.mockResolvedValueOnce({ version: 3 });
    await expect(
      publishCaptureGates({ content: '{"reconcile":{"rules":[]}}', message: 'first' }),
    ).resolves.toEqual({ version: 3 });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/capture/gates', {
      method: 'PUT',
      data: { content: '{"reconcile":{"rules":[]}}', message: 'first' },
    });
  });

  it('publishCaptureGates: message omitted stays undefined', async () => {
    mockedRequest.mockResolvedValueOnce({ version: 4 });
    await publishCaptureGates({ content: '{}' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/capture/gates', {
      method: 'PUT',
      data: { content: '{}', message: undefined },
    });
  });

  it('publishCaptureGates: baseVersion optimistic-lock baseline passes through', async () => {
    mockedRequest.mockResolvedValueOnce({ version: 5 });
    await publishCaptureGates({ content: '{}', baseVersion: 4 });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/capture/gates', {
      method: 'PUT',
      data: { content: '{}', baseVersion: 4 },
    });
  });

  it('listCaptureGatesVersions: GET /versions', async () => {
    mockedRequest.mockResolvedValueOnce([
      {
        version: 3,
        message: 'rollback from version 1',
        createdBy: 'admin',
        updatedAt: '2026-10-11 10:00:00',
      },
    ]);
    await expect(listCaptureGatesVersions()).resolves.toHaveLength(1);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/capture/gates/versions');
  });

  it('rollbackCaptureGates: POST /rollback with version', async () => {
    mockedRequest.mockResolvedValueOnce({ version: 4 });
    await expect(rollbackCaptureGates({ version: 1 })).resolves.toEqual({ version: 4 });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/capture/gates/rollback', {
      method: 'POST',
      data: { version: 1 },
    });
  });
});
