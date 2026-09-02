/**
 * F11 验收测试：异步任务进度内嵌面板
 */
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import TaskProgressPanel from '@/pages/Functions/Invoke/TaskProgressPanel';

jest.mock('@/services/api/functions', () => ({
  fetchTaskResult: jest.fn(),
  cancelTask: jest.fn(),
}));

const { fetchTaskResult, cancelTask } = jest.requireMock('@/services/api/functions') as {
  fetchTaskResult: jest.Mock;
  cancelTask: jest.Mock;
};

beforeEach(() => {
  fetchTaskResult.mockReset();
  cancelTask.mockReset();
});

describe('F11: TaskProgressPanel', () => {
  test('轮询至 succeeded 并回调结果', async () => {
    jest.useFakeTimers();
    fetchTaskResult
      .mockResolvedValueOnce({ state: 'queued' })
      .mockResolvedValueOnce({ state: 'running' })
      .mockResolvedValue({ state: 'succeeded', payload: { done: true } });

    const onCompleted = jest.fn();
    render(<TaskProgressPanel taskId="t1" onCompleted={onCompleted} />);

    await act(async () => {});
    expect(screen.getByTestId('task-status').textContent).toBe('排队中');

    await act(async () => {
      jest.advanceTimersByTime(2100);
    });
    expect(screen.getByTestId('task-status').textContent).toBe('执行中');

    await act(async () => {
      jest.advanceTimersByTime(2100);
    });
    expect(screen.getByTestId('task-status').textContent).toBe('已完成');
    expect(onCompleted).toHaveBeenCalledWith({ done: true });
    // 终态后停止轮询
    const calls = fetchTaskResult.mock.calls.length;
    await act(async () => {
      jest.advanceTimersByTime(6000);
    });
    expect(fetchTaskResult.mock.calls.length).toBe(calls);
    jest.useRealTimers();
  });

  test('failed 展示错误详情', async () => {
    fetchTaskResult.mockResolvedValue({ state: 'failed', error: 'game server exploded' });
    render(<TaskProgressPanel taskId="t2" />);
    await waitFor(() => expect(screen.getByText('任务执行失败')).toBeTruthy());
    expect(screen.getByText(/game server exploded/)).toBeTruthy();
  });

  test('取消按钮调用 cancelTask', async () => {
    fetchTaskResult.mockResolvedValue({ state: 'running' });
    cancelTask.mockResolvedValue(undefined);
    render(<TaskProgressPanel taskId="t3" />);
    await act(async () => {});
    fireEvent.click(screen.getByTestId('task-cancel'));
    await waitFor(() => expect(cancelTask).toHaveBeenCalledWith('t3'));
  });

  test('终态后不渲染取消/刷新按钮', async () => {
    fetchTaskResult.mockResolvedValue({ state: 'cancelled' });
    render(<TaskProgressPanel taskId="t4" />);
    await waitFor(() => expect(screen.getByTestId('task-status').textContent).toBe('已取消'));
    expect(screen.queryByTestId('task-cancel')).toBeNull();
  });

  test('轮询单次失败不打断（下一轮恢复）', async () => {
    jest.useFakeTimers();
    fetchTaskResult
      .mockRejectedValueOnce(new Error('network blip'))
      .mockResolvedValueOnce({ state: 'succeeded', payload: null });
    const onCompleted = jest.fn();
    render(<TaskProgressPanel taskId="t5" onCompleted={onCompleted} />);
    // 第一轮 rejected → catch 内重新安排 timer；需要先 flush 微任务让
    // rejection 续体注册 timer，再推进时间触发第二轮
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
      jest.advanceTimersByTime(2100);
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });
    await waitFor(() => expect(onCompleted).toHaveBeenCalledWith(null));
    jest.useRealTimers();
  });
});
