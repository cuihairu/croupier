/**
 * thirdPartyProbe API adapter 覆盖收口（web 覆盖率巡检：probeThirdParty
 * 此前零测试触达）。契约：POST /api/v1/ops/probes/:channel + skipErrorHandler。
 */
import { request } from '@umijs/max';
import { probeThirdParty, type ThirdPartyProbeView } from './thirdPartyProbe';

jest.mock('@umijs/max', () => ({
  request: jest.fn(),
}));
const mockedRequest = request as jest.MockedFunction<typeof request>;

const VIEW: ThirdPartyProbeView = {
  channel: 'smtp',
  configured: true,
  ok: true,
  status: 250,
  latencyMs: 42,
};

beforeEach(() => {
  mockedRequest.mockReset();
});

describe('probeThirdParty', () => {
  it('POSTs to the per-channel probe endpoint read-only with empty body', async () => {
    mockedRequest.mockResolvedValue(VIEW);

    await expect(probeThirdParty('smtp')).resolves.toEqual(VIEW);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/probes/smtp', {
      method: 'POST',
      skipErrorHandler: true,
      data: {},
    });
  });

  it('rejects when the backend reports a failed probe (skipErrorHandler keeps raw error)', async () => {
    mockedRequest.mockRejectedValue(new Error('connect timeout'));

    await expect(probeThirdParty('dingtalk')).rejects.toThrow('connect timeout');
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/probes/dingtalk', {
      method: 'POST',
      skipErrorHandler: true,
      data: {},
    });
  });
});
