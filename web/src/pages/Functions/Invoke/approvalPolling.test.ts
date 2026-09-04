import { startApprovalPolling } from './approvalPolling';

describe('startApprovalPolling', () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });
  afterEach(() => {
    jest.useRealTimers();
  });

  it('立即轮询，pending 期间不回调并持续轮询', async () => {
    const fetcher = jest.fn().mockResolvedValue({ status: 'pending' });
    const onUpdate = jest.fn();
    const stop = startApprovalPolling('ap-1', onUpdate, fetcher, 1000);

    await jest.advanceTimersByTimeAsync(0);
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(onUpdate).not.toHaveBeenCalled();

    await jest.advanceTimersByTimeAsync(1000);
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(onUpdate).not.toHaveBeenCalled();
    stop();
  });

  it('到达终态回调一次并停止轮询', async () => {
    const fetcher = jest.fn().mockResolvedValueOnce({ status: 'approved' });
    const onUpdate = jest.fn();
    const stop = startApprovalPolling('ap-1', onUpdate, fetcher, 1000);

    await jest.advanceTimersByTimeAsync(0);

    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenCalledWith({ status: 'approved', reason: undefined });

    const callsAfterTerminal = fetcher.mock.calls.length;
    await jest.advanceTimersByTimeAsync(5000);
    expect(fetcher.mock.calls.length).toBe(callsAfterTerminal);
    stop();
  });

  it('rejected 终态携带原因回调一次', async () => {
    const fetcher = jest.fn().mockResolvedValueOnce({ status: 'rejected', reason: '越权' });
    const onUpdate = jest.fn();
    const stop = startApprovalPolling('ap-1', onUpdate, fetcher, 1000);

    await jest.advanceTimersByTimeAsync(0);

    expect(onUpdate).toHaveBeenCalledWith({ status: 'rejected', reason: '越权' });
    stop();
  });

  it('stop 后不再轮询也不再回调', async () => {
    const fetcher = jest.fn().mockResolvedValue({ status: 'pending' });
    const onUpdate = jest.fn();
    const stop = startApprovalPolling('ap-1', onUpdate, fetcher, 1000);

    await jest.advanceTimersByTimeAsync(0);
    stop();
    await jest.advanceTimersByTimeAsync(5000);

    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(onUpdate).not.toHaveBeenCalled();
  });

  it('单次查询失败静默重试', async () => {
    const failing = jest.fn().mockRejectedValueOnce(new Error('network')).mockResolvedValue({
      status: 'pending',
    });
    const onUpdate = jest.fn();
    const stop = startApprovalPolling('ap-1', onUpdate, failing, 1000);

    await jest.advanceTimersByTimeAsync(0);
    expect(onUpdate).not.toHaveBeenCalled();
    await jest.advanceTimersByTimeAsync(1000);
    expect(failing).toHaveBeenCalledTimes(2);
    stop();
  });
});
