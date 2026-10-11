import { request } from '@umijs/max';
import { getCaptureGates, publishCaptureGates } from './opsCapture';

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
});
