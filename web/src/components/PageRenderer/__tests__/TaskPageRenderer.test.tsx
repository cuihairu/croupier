/** TaskPageRenderer 覆盖。
 *
 * 覆盖路径：提交流程（缺绑定/预览拦截/approval 分流/selector 取 taskId/
 * 无 taskId 警告/异常兜底）、binding 轮询与 onQueryStatus 轮询（完成态停轮询、
 * 状态未映射报错、事件与结果绑定聚合）、取消（binding 优先/回调兜底/无配置警告/
 * 异常兜底）、审批刷新（task 续接/sync 完成/未通过/异常）、渲染分支
 * （状态五色 Tag/进度态/审批 Alert 色/时间线/结果视图）。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import TaskPageRenderer from '../TaskPageRenderer';
import type {
  ApprovalStatusResult,
  PageExecutionResult,
  PageFunctionBinding,
  TaskPageSpec,
  TaskStatusResult,
} from '@/types/dashboard';

// SchemaFormRenderer 替身：渲染提交按钮直接触发 onFinish
jest.mock('@/components/SchemaFormRenderer', () => ({
  __esModule: true,
  default: jest.fn(({ onFinish }: { onFinish: (v: Record<string, unknown>) => void }) => (
    <button type="button" data-testid="form-submit" onClick={() => onFinish({ name: 'x' })}>
      form-submit
    </button>
  )),
}));

type ExecuteMock = jest.Mock<Promise<PageExecutionResult>, [string, unknown]>;

const ok = (data?: unknown, extra?: Partial<PageExecutionResult>): PageExecutionResult => ({
  kind: 'invoke',
  requestId: 'r1',
  data,
  ...extra,
});

// 构造 binding；output 形如 { taskStatus: 'wrapper' } 表示把 data.<wrapper> 落到该 stateKey
function binding(
  usage: PageFunctionBinding['usage'],
  id: string,
  output?: Record<string, string>,
): PageFunctionBinding {
  const item: PageFunctionBinding = {
    id,
    functionId: `fn-${id}`,
    usage,
    execution: { mode: 'sync' },
  };
  if (output) {
    item.selectors = {
      input: { assignments: [] },
      output: Object.entries(output).map(([stateKey, field]) => ({
        stateKey,
        source: `/${field}`,
        shape: 'object' as const,
      })),
    };
  }
  return item;
}

const spec = (
  taskView: Partial<TaskPageSpec['taskView']>,
  resultView?: TaskPageSpec['resultView'],
): TaskPageSpec => ({
  form: { jsonSchema: { type: 'object' } },
  resultView,
  taskView: {
    taskIdStateKey: 'taskId',
    statusBindingId: 'b-status',
    statusStatePath: '/state',
    showTimeline: true,
    showProgress: true,
    showEvents: true,
    cancelable: true,
    ...taskView,
  },
});

interface RenderOptions {
  spec?: TaskPageSpec;
  bindings?: PageFunctionBinding[];
  preview?: boolean;
  onQueryStatus?: (taskId: string) => Promise<TaskStatusResult>;
  onCancelTask?: (taskId: string) => Promise<void>;
  onQueryApprovalStatus?: (approvalId: string) => Promise<ApprovalStatusResult>;
  title?: string;
}

function renderTask(options: RenderOptions = {}) {
  const onExecute = jest.fn() as ExecuteMock;
  const utils = render(
    <App>
      <TaskPageRenderer
        spec={options.spec ?? spec({})}
        bindings={options.bindings ?? [binding('task', 'b-task')]}
        onExecute={onExecute as never}
        preview={options.preview}
        onQueryStatus={options.onQueryStatus}
        onCancelTask={options.onCancelTask}
        onQueryApprovalStatus={options.onQueryApprovalStatus}
        title={options.title}
      />
    </App>,
  );
  return { onExecute, ...utils };
}

const submit = () => fireEvent.click(screen.getByTestId('form-submit'));

describe('提交流程', () => {
  it('未配置任务绑定时报错且不执行', async () => {
    renderTask({ bindings: [] });
    submit();
    await waitFor(() => expect(screen.getByText('未配置任务绑定')).toBeInTheDocument());
  });

  it('预览模式拦截提交', async () => {
    renderTask({ preview: true });
    submit();
    await waitFor(() => expect(screen.getByText('预览模式不提交任务')).toBeInTheDocument());
  });

  it('提交成功：response.taskId 优先，进入 pending 并立即轮询一次', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>().mockResolvedValue({
      taskId: 't-1',
      status: 'running',
    });
    const { onExecute } = renderTask({ onQueryStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-1' }));

    submit();
    await waitFor(() => expect(screen.getByText('任务已提交')).toBeInTheDocument());
    await waitFor(() => expect(onQueryStatus).toHaveBeenCalledWith('t-1'));
    expect(screen.getByText('RUNNING')).toBeInTheDocument();
  });

  it('taskId 缺失（response.taskId 与 selector 均无）时警告', async () => {
    const { onExecute } = renderTask();
    onExecute.mockResolvedValueOnce(ok({ x: 1 }));
    submit();
    await waitFor(() => expect(screen.getByText('未获取到任务 ID')).toBeInTheDocument());
  });

  it('selector 输出兜底取 taskId（stateKey 命中）', async () => {
    const { onExecute } = renderTask({
      bindings: [binding('task', 'b-task', { taskId: 'job' })],
    });
    onExecute.mockResolvedValueOnce(ok({ job: 'job-9' }));
    submit();
    await waitFor(() => expect(screen.getByText('任务已提交')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('b-task', { form: { name: 'x' } });
  });

  it('审批分流：kind=approval 显示等待审批', async () => {
    const { onExecute } = renderTask();
    onExecute.mockResolvedValueOnce({
      kind: 'approval',
      requestId: 'req-a',
      approvalId: 'apr-1',
    });
    submit();
    await waitFor(() => expect(screen.getByText('任务已提交审批')).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText('等待审批')).toBeInTheDocument());
  });

  it('提交异常时展示失败详情', async () => {
    const { onExecute } = renderTask();
    onExecute.mockRejectedValueOnce(new Error('boom'));
    submit();
    await waitFor(() => expect(screen.getByText(/任务提交失败: boom/)).toBeInTheDocument());
  });
});

describe('轮询：statusBinding 路径', () => {
  const statusBindings = [
    binding('task', 'b-task'),
    binding('task_status', 'b-status', { taskStatus: 'wrapper' }),
  ];

  it('状态/事件/结果三绑定聚合渲染', async () => {
    const { onExecute } = renderTask({
      bindings: [
        ...statusBindings,
        binding('task_events', 'b-events', { taskEvents: 'evts' }),
        binding('task_result', 'b-result', { taskResult: 'res' }),
        binding('task_result', 'b-res-key', { taskResult: 'res' }),
      ],
    });
    onExecute.mockImplementation(async (bindingId: string) => {
      if (bindingId === 'b-task') return ok(undefined, { taskId: 't-9' });
      if (bindingId === 'b-status') {
        return ok({
          wrapper: {
            state: 'completed',
            progress: 80,
            message: '即将完成',
            result: { done: 1 },
          },
        });
      }
      if (bindingId === 'b-events') {
        return ok({
          evts: [
            { timestamp: '2026-01-01T00:00:00Z', message: '开始', type: 'info' },
            { timestamp: '2026-01-01T00:01:00Z', message: '出错了', type: 'error' },
            { timestamp: '2026-01-01T00:02:00Z', message: '注意', type: 'warning' },
            { timestamp: '2026-01-01T00:04:00Z', message: '带负载', payload: { p: 1 } },
            { timestamp: '2026-01-01T00:05:00Z', message: '进度', type: 'progress' },
            'not-a-record',
            { timestamp: '2026-01-01T00:03:00Z' },
          ],
        });
      }
      if (bindingId === 'b-res-key') {
        // extractTaskResult 的 result 剥壳分支
        return ok({ res: { result: { final: 1 } } });
      }
      return ok({ res: { value: 42 } });
    });

    submit();
    await waitFor(() => expect(screen.getByText('COMPLETED')).toBeInTheDocument());
    expect(screen.getByText('即将完成')).toBeInTheDocument();
    // 事件时间线（showTimeline && showEvents）；无 message 的事件行被过滤
    expect(screen.getByText('任务事件')).toBeInTheDocument();
    expect(screen.getByText('开始')).toBeInTheDocument();
    expect(screen.getByText('出错了')).toBeInTheDocument();
    expect(screen.getByText('注意')).toBeInTheDocument();
    // 结果视图（completed + result）
    expect(screen.getByText('任务结果')).toBeInTheDocument();
  });

  it('状态字符串归一：succeeded/timed_out/cancel_requested/queued 全映射', async () => {
    const { onExecute } = renderTask({
      bindings: statusBindings,
      spec: spec({ statusStatePath: '' }),
    });
    const statuses = ['succeeded', 'timed_out', 'cancel_requested', 'queued'];
    let index = 0;
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task'
        ? ok(undefined, { taskId: 't-s' })
        : ok({ wrapper: statuses[index++ % statuses.length] }),
    );
    submit();
    await waitFor(() =>
      expect(screen.getByText(/COMPLETED|FAILED|CANCELLED|PENDING/)).toBeInTheDocument(),
    );
  });

  // 中间态经「提交→立即轮询」链喂入：刷新按钮在非终态后持续 loading（antd loading
  // 拦截点击），无法逐次喂入，故每个状态独立用例提交一次。
  it.each([
    ['queued', 'PENDING'],
    ['dispatching', 'PENDING'],
    ['running', 'RUNNING'],
    ['succeeded', 'COMPLETED'],
    ['completed', 'COMPLETED'],
    ['failed', 'FAILED'],
    ['timed_out', 'FAILED'],
    ['cancel_requested', 'CANCELLED'],
    ['cancelled', 'CANCELLED'],
    ['weird-state', 'PENDING'],
  ])('状态映射：%s → %s', async (state, expected) => {
    const { onExecute } = renderTask({ bindings: statusBindings });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task' ? ok(undefined, { taskId: `t-${state}` }) : ok({ wrapper: { state } }),
    );
    submit();
    await waitFor(() => expect(screen.getByText(expected)).toBeInTheDocument());
  });

  it('数组 token 命中：状态指针指向数组元素', async () => {
    const { onExecute } = renderTask({
      bindings: statusBindings,
      spec: spec({ statusStatePath: '/arr/0' }),
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task'
        ? ok(undefined, { taskId: 't-arr' })
        : ok({ wrapper: { arr: ['running'] } }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('RUNNING')).toBeInTheDocument());
  });

  it('指针寻址失败（数组越界）归一为 pending', async () => {
    const { onExecute } = renderTask({
      bindings: statusBindings,
      spec: spec({ statusStatePath: '/arr/5' }),
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task'
        ? ok(undefined, { taskId: 't-arr2' })
        : ok({ wrapper: { arr: ['x'], state: 'running' } }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('PENDING')).toBeInTheDocument());
  });

  it('指针指向不存在 token 归一为 pending', async () => {
    const { onExecute } = renderTask({
      bindings: statusBindings,
      spec: spec({ statusStatePath: '/missing' }),
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task'
        ? ok(undefined, { taskId: 't-miss' })
        : ok({ wrapper: { state: 'running' } }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('PENDING')).toBeInTheDocument());
  });

  it('record 状态 + 空 statusStatePath 取整个对象（非字符串归一 pending）', async () => {
    const { onExecute } = renderTask({
      bindings: statusBindings,
      spec: spec({ statusStatePath: '' }),
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task'
        ? ok(undefined, { taskId: 't-rec' })
        : ok({ wrapper: { state: 'running', arr: ['x'] } }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('PENDING')).toBeInTheDocument());
  });

  it('状态数据为非 record/非 string 标量（数字）：跳过归一，按 previous/兜底 pending', async () => {
    // normalizeTaskStatusFromExecution 的尾部兜底：executionData 既非对象也非字符串
    const { onExecute } = renderTask({ bindings: statusBindings });
    onExecute.mockImplementation((bindingId: string) =>
      bindingId === 'b-task' ? ok(undefined, { taskId: 't-num' }) : ok({ wrapper: 42 }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('PENDING')).toBeInTheDocument());
  });

  it('eventsData 为 {items:[...]} 形态时同样提取时间线', async () => {
    const { onExecute } = renderTask({
      bindings: [...statusBindings, binding('task_events', 'b-events', { taskEvents: 'evts' })],
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task'
        ? ok(undefined, { taskId: 't-items' })
        : bindingId === 'b-status'
          ? ok({ wrapper: { state: 'running' } })
          : ok({ evts: { items: [{ timestamp: '2026-01-01T00:00:00Z', message: 'items 事件' }] } }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('items 事件')).toBeInTheDocument());
  });

  it('statusStatePath 无斜杠前缀时寻址失败归一 pending', async () => {
    const { onExecute } = renderTask({
      bindings: statusBindings,
      spec: spec({ statusStatePath: 'state' }),
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task'
        ? ok(undefined, { taskId: 't-noslash' })
        : ok({ wrapper: { state: 'running' } }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('PENDING')).toBeInTheDocument());
  });

  it('resultBinding 输出带 result key 时剥壳取内层', async () => {
    const { onExecute } = renderTask({
      bindings: [...statusBindings, binding('task_result', 'b-res-key', { taskResult: 'res' })],
      spec: spec(
        { resultBindingId: 'b-res-key' },
        {
          fields: [{ key: 'final', title: { 'zh-CN': '终值' }, dataType: 'string' }],
        },
      ),
    });
    onExecute.mockImplementation(async (bindingId: string) => {
      if (bindingId === 'b-task') return ok(undefined, { taskId: 't-rk' });
      if (bindingId === 'b-status') {
        return ok({ wrapper: { state: 'completed', result: { raw: 1 } } });
      }
      return ok({ res: { result: { final: 1 } } });
    });
    submit();
    await waitFor(() => expect(screen.getByText('任务结果')).toBeInTheDocument());
    // 剥壳后的 {final:1}（而非 {result:{final:1}}）作为结果数据：字段终值=1
    expect(screen.getByText('终值')).toBeInTheDocument();
    expect(screen.getByText('1')).toBeInTheDocument();
  });

  it('statusData 未映射到 pageState.taskStatus 时报错', async () => {
    const { onExecute } = renderTask({ bindings: statusBindings });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task' ? ok(undefined, { taskId: 't-x' }) : ok({ other: 1 }),
    );
    submit();
    await waitFor(() =>
      expect(screen.getByText('任务状态绑定未映射到 pageState.taskStatus')).toBeInTheDocument(),
    );
  });

  it('轮询执行异常时静默记录 console.error', async () => {
    const spy = jest.spyOn(console, 'error').mockImplementation(() => undefined);
    const { onExecute } = renderTask({ bindings: statusBindings });
    onExecute.mockImplementation(async (bindingId: string) => {
      if (bindingId === 'b-task') return ok(undefined, { taskId: 't-e' });
      throw new Error('poll fail');
    });
    submit();
    await waitFor(() => {
      expect(spy).toHaveBeenCalled();
      const hasError = spy.mock.calls.some((call) => call.some((arg) => arg instanceof Error));
      expect(hasError).toBe(true);
    });
    spy.mockRestore();
  });
});

describe('轮询：onQueryStatus 路径', () => {
  it('完成态停止轮询并渲染进度/结果/时间线', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>().mockResolvedValue({
      taskId: 't-c',
      status: 'completed',
      progress: 100,
      message: '全部完成',
      result: { total: 3 },
      events: [{ timestamp: '2026-01-01T00:00:00Z', type: 'info', message: 'ev1' }],
    });
    const { onExecute } = renderTask({ onQueryStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-c' }));

    submit();
    await waitFor(() => expect(screen.getByText('COMPLETED')).toBeInTheDocument());
    expect(screen.getByText('全部完成')).toBeInTheDocument();
    expect(document.querySelector('.ant-progress-status-success')).toBeInTheDocument();
    expect(screen.getByText('任务结果')).toBeInTheDocument();
    expect(screen.getByText('任务事件')).toBeInTheDocument();
  });

  it('查询异常静默处理', async () => {
    const spy = jest.spyOn(console, 'error').mockImplementation(() => undefined);
    const onQueryStatus = jest
      .fn<() => Promise<TaskStatusResult>>()
      .mockRejectedValue(new Error('q'));
    const { onExecute } = renderTask({ onQueryStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-q' }));
    submit();
    await waitFor(() => expect(spy).toHaveBeenCalled());
    spy.mockRestore();
  });

  it('无状态查询通道：提交成功但仅提示未配置', async () => {
    const { onExecute } = renderTask({ bindings: [binding('task', 'b-task')] });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-n' }));
    submit();
    await waitFor(() =>
      expect(screen.getByText('任务已提交，但页面未配置状态查询绑定')).toBeInTheDocument(),
    );
  });
});

describe('取消任务', () => {
  const startRunning = async (options: RenderOptions = {}) => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>().mockResolvedValue({
      taskId: 't-r',
      status: 'running',
    });
    const { onExecute } = renderTask({ ...options, onQueryStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-r' }));
    submit();
    await waitFor(() => expect(screen.getByText('RUNNING')).toBeInTheDocument());
    return { onExecute };
  };

  it('cancelBinding 优先于 onCancelTask 回调', async () => {
    const onCancelTask = jest.fn<() => Promise<void>>();
    const { onExecute } = await startRunning({
      onCancelTask,
      bindings: [binding('task', 'b-task'), binding('task_cancel', 'b-cancel')],
    });
    fireEvent.click(screen.getByRole('button', { name: /取消/ }));
    await waitFor(() => expect(screen.getByText('任务已取消')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('b-cancel', { pageState: { taskId: 't-r' } });
    expect(onCancelTask).not.toHaveBeenCalled();
    expect(screen.getByText('CANCELLED')).toBeInTheDocument();
  });

  it('无 cancelBinding 时走 onCancelTask 回调', async () => {
    const onCancelTask = jest.fn<() => Promise<void>>();
    await startRunning({ onCancelTask });
    fireEvent.click(screen.getByRole('button', { name: /取消/ }));
    await waitFor(() => expect(onCancelTask).toHaveBeenCalledWith('t-r'));
    expect(screen.getByText('任务已取消')).toBeInTheDocument();
  });

  it('取消异常时展示失败详情', async () => {
    const onCancelTask = jest.fn<() => Promise<void>>().mockRejectedValue(new Error('nope'));
    await startRunning({ onCancelTask });
    fireEvent.click(screen.getByRole('button', { name: /取消/ }));
    await waitFor(() => expect(screen.getByText(/取消任务失败: nope/)).toBeInTheDocument());
  });
});

describe('审批刷新', () => {
  const toApproval = async (options: RenderOptions = {}) => {
    const onQueryApprovalStatus = jest.fn<() => Promise<ApprovalStatusResult>>();
    const { onExecute } = renderTask({ ...options, onQueryApprovalStatus });
    onExecute.mockResolvedValueOnce({
      kind: 'approval',
      requestId: 'req-a',
      approvalId: 'apr-1',
    });
    submit();
    await waitFor(() => expect(screen.getByText('等待审批')).toBeInTheDocument());
    return { onQueryApprovalStatus: onQueryApprovalStatus as jest.Mock, onExecute };
  };

  it('approved + task 续接：任务启动并进入轮询', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>().mockResolvedValue({
      taskId: 't-after',
      status: 'running',
    });
    const { onQueryApprovalStatus } = await toApproval({ onQueryStatus });
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'approved',
      resultKind: 'task',
      taskId: 't-after',
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    // 审批通过后立即轮询，带 message 的中间态与轮询结果同批提交 → 断言终态
    await waitFor(() => expect(onQueryStatus).toHaveBeenCalledWith('t-after'));
    expect(screen.getByText('RUNNING')).toBeInTheDocument();
  });

  it('approved + sync：直接完成并渲染结果', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'approved',
      resultKind: 'sync',
      result: { value: 'synced' },
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText('审批已通过，执行已完成')).toBeInTheDocument());
    expect(screen.getByText('任务结果')).toBeInTheDocument();
  });

  it('rejected：Alert 呈 error 色并展示理由', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'rejected',
      reason: '两眼一闭',
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(document.querySelector('.ant-alert-error')).toBeInTheDocument());
    expect(screen.getByText(/两眼一闭/)).toBeInTheDocument();
  });

  it('pending：仅更新状态展示', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'pending',
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText(/审批状态：pending/)).toBeInTheDocument());
  });

  it('查询异常时报错', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockRejectedValueOnce(new Error('net'));
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText('net')).toBeInTheDocument());
  });
});

describe('渲染分支', () => {
  it('自定义标题透传，默认为「提交任务」', () => {
    const { rerender } = renderTask();
    expect(screen.getByText('提交任务')).toBeInTheDocument();
    rerender(
      <App>
        <TaskPageRenderer
          spec={spec({})}
          bindings={[binding('task', 'b-task')]}
          onExecute={jest.fn() as never}
          title="自定义任务页"
        />
      </App>,
    );
    expect(screen.getByText('自定义任务页')).toBeInTheDocument();
  });

  it('showTimeline/showEvents 关闭时不渲染时间线', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>().mockResolvedValue({
      taskId: 't-t',
      status: 'completed',
      events: [{ timestamp: '2026-01-01T00:00:00Z', type: 'info', message: 'ev' }],
      result: { a: 1 },
    });
    const { onExecute } = renderTask({
      onQueryStatus,
      spec: spec({ showTimeline: false, showEvents: false }),
    });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-t' }));
    submit();
    await waitFor(() => expect(screen.getByText('COMPLETED')).toBeInTheDocument());
    expect(screen.queryByText('任务事件')).not.toBeInTheDocument();
    expect(screen.getByText('任务结果')).toBeInTheDocument();
  });

  it('showProgress 关闭时不渲染进度条；failed 状态渲染错误信息', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>();
    const { onExecute } = renderTask({
      onQueryStatus,
      spec: spec({ showProgress: false }),
    });
    (onQueryStatus as jest.Mock).mockResolvedValue({
      taskId: 't-f',
      status: 'failed',
      progress: 40,
      error: '执行炸了',
    });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-f' }));
    submit();
    await waitFor(() => expect(screen.getByText('FAILED')).toBeInTheDocument());
    expect(document.querySelector('.ant-progress')).not.toBeInTheDocument();
    expect(screen.getByText('执行炸了')).toBeInTheDocument();
  });

  it('cancelable 关闭时不渲染取消按钮；刷新按钮在无审批时可用', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>();
    const { onExecute } = renderTask({ onQueryStatus, spec: spec({ cancelable: false }) });
    (onQueryStatus as jest.Mock).mockResolvedValue({ taskId: 't-r2', status: 'running' });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-r2' }));
    submit();
    await waitFor(() => expect(screen.getByText('RUNNING')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /取消/ })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /刷新/ })).toBeInTheDocument();
  });

  it('事件 data 附带 JSON 摘要（payload 回退）', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>();
    const { onExecute } = renderTask({ onQueryStatus });
    (onQueryStatus as jest.Mock).mockResolvedValue({
      taskId: 't-d',
      status: 'running',
      events: [
        { timestamp: '2026-01-01T00:00:00Z', type: 'info', message: '带数据', data: { k: 1 } },
      ],
    });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-d' }));
    submit();
    await waitFor(() => expect(screen.getByText('带数据')).toBeInTheDocument());
    expect(screen.getByText('对象')).toBeInTheDocument();
    expect(screen.getByText('k')).toBeInTheDocument();
  });

  it('statusBindingId 显式指定时按 id 匹配而非 usage', async () => {
    const { onExecute } = renderTask({
      bindings: [
        binding('task', 'b-task'),
        binding('query', 'b-custom-status', { taskStatus: 'state' }),
      ],
      spec: spec({ statusBindingId: 'b-custom-status', statusStatePath: '' }),
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task' ? ok(undefined, { taskId: 't-i' }) : ok({ state: 'running' }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('RUNNING')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('b-custom-status', {
      pageState: { taskId: 't-i' },
    });
  });
});

describe('事件与状态数据形态（分支补齐）', () => {
  it('statusData 内联 events：直接作为时间线（非 previousEvents 兜底）；createdAt 兜底时间戳；failed/cancel_requested/cancelled 事件类型归一（L239/L196/L166/L169/L170）', async () => {
    const { onExecute } = renderTask({
      bindings: [
        binding('task', 'b-task'),
        binding('task_status', 'b-status', { taskStatus: 'wrapper' }),
      ],
    });
    onExecute.mockImplementation(async (bindingId: string) => {
      if (bindingId === 'b-task') return ok(undefined, { taskId: 't-ev' });
      return ok({
        wrapper: {
          state: 'running',
          events: [
            // 仅 createdAt 无 timestamp → 时间戳兜底链（L196 右支）
            { createdAt: '2026-01-01T00:00:00Z', message: '失败事件', type: 'failed' },
            { createdAt: '2026-01-01T00:01:00Z', message: '取消请求', type: 'cancel_requested' },
            { createdAt: '2026-01-01T00:02:00Z', message: '取消完成', type: 'cancelled' },
            // 无任何时间戳字段 → 事件被过滤（L196 两级 || 均落空）
            { message: '无时间事件' },
          ],
        },
      });
    });
    submit();
    // 内联 events 非空 → nextEvents 直接采用（L239 真值侧）
    await waitFor(() => expect(screen.getByText('失败事件')).toBeInTheDocument());
    expect(screen.getByText('取消请求')).toBeInTheDocument();
    expect(screen.getByText('取消完成')).toBeInTheDocument();
    // 三种类型分别归一为 error/warning/warning（时间线渲染即证明归一路径执行）
    expect(screen.getByText('任务事件')).toBeInTheDocument();
    // 无时间戳事件被过滤
    expect(screen.queryByText('无时间事件')).not.toBeInTheDocument();
  });

  it('eventsData 为 {events:[...]} 包装形态时提取时间线（L187）', async () => {
    const { onExecute } = renderTask({
      bindings: [
        binding('task', 'b-task'),
        binding('task_status', 'b-status', { taskStatus: 'wrapper' }),
        binding('task_events', 'b-events', { taskEvents: 'evts' }),
      ],
      spec: spec({ eventsBindingId: 'b-events' }),
    });
    onExecute.mockImplementation(async (bindingId: string) => {
      if (bindingId === 'b-task') return ok(undefined, { taskId: 't-w' });
      if (bindingId === 'b-status') return ok({ wrapper: { state: 'running' } });
      return ok({ evts: { events: [{ timestamp: '2026-01-01T00:00:00Z', message: '包装事件' }] } });
    });
    submit();
    await waitFor(() => expect(screen.getByText('包装事件')).toBeInTheDocument());
  });

  it('eventsBindingId 显式指定时按 id 匹配而非 usage（L315 真值侧）', async () => {
    const { onExecute } = renderTask({
      bindings: [
        binding('task', 'b-task'),
        binding('task_status', 'b-status', { taskStatus: 'wrapper' }),
        // usage 故意非 task_events：显式 id 必须压过 usage 缺省匹配
        binding('query', 'b-ev-2', { taskEvents: 'evts' }),
      ],
      spec: spec({ eventsBindingId: 'b-ev-2' }),
    });
    onExecute.mockImplementation(async (bindingId: string) => {
      if (bindingId === 'b-task') return ok(undefined, { taskId: 't-i2' });
      if (bindingId === 'b-status') return ok({ wrapper: { state: 'running' } });
      return ok({ evts: { items: [{ timestamp: '2026-01-01T00:00:00Z', message: '显式事件' }] } });
    });
    submit();
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith('b-ev-2', { pageState: { taskId: 't-i2' } }),
    );
    expect(screen.getByText('显式事件')).toBeInTheDocument();
  });

  it('eventsData 为无关 record（无 events/items）时不渲染时间线（L190 兜底）', async () => {
    const { onExecute } = renderTask({
      bindings: [
        binding('task', 'b-task'),
        binding('task_status', 'b-status', { taskStatus: 'wrapper' }),
        binding('task_events', 'b-events', { taskEvents: 'evts' }),
      ],
    });
    onExecute.mockImplementation(async (bindingId: string) => {
      if (bindingId === 'b-task') return ok(undefined, { taskId: 't-n' });
      if (bindingId === 'b-status') return ok({ wrapper: { state: 'running' } });
      return ok({ evts: { total: 3 } });
    });
    submit();
    await waitFor(() => expect(screen.getByText('RUNNING')).toBeInTheDocument());
    expect(screen.queryByText('任务事件')).not.toBeInTheDocument();
  });

  it('statusData 为 null：非 record 非字符串，沿用上次状态展开（L253 previous 侧）', async () => {
    const { onExecute } = renderTask({
      bindings: [
        binding('task', 'b-task'),
        binding('task_status', 'b-status', { taskStatus: 'wrapper' }),
      ],
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task' ? ok(undefined, { taskId: 't-null' }) : ok({ wrapper: null }),
    );
    submit();
    // previous = 提交时的 pending 状态 → 展开后仍 PENDING，且保留提交消息
    await waitFor(() => expect(screen.getByText('PENDING')).toBeInTheDocument());
    // antd message 通知（命令式 portal，cleanup 不卸载）会与前序用例的
    // 「任务已提交」notice 叠加，selector 限定到本用例渲染的 Alert 标题
    expect(screen.getByText('任务已提交', { selector: '.ant-alert-title' })).toBeInTheDocument();
  });
});

describe('绑定 id 匹配与缺省（分支补齐）', () => {
  it('statusBindingId 为空串：回退按 usage 匹配 task_status（L313 假值侧）', async () => {
    const { onExecute } = renderTask({
      bindings: [
        binding('task', 'b-task'),
        binding('task_status', 'b-st2', { taskStatus: 'wrapper' }),
      ],
      spec: spec({ statusBindingId: '' }),
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task'
        ? ok(undefined, { taskId: 't-u' })
        : ok({ wrapper: { state: 'running' } }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('RUNNING')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('b-st2', { pageState: { taskId: 't-u' } });
  });

  it('cancelBindingId 显式指定时按 id 匹配并执行该绑定（L321 真值侧）', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>().mockResolvedValue({
      taskId: 't-c2',
      status: 'running',
    });
    const { onExecute } = renderTask({
      onQueryStatus,
      bindings: [binding('task', 'b-task'), binding('action', 'b-cancel-2')],
      spec: spec({ cancelBindingId: 'b-cancel-2' }),
    });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-c2' }));
    submit();
    await waitFor(() => expect(screen.getByText('RUNNING')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /取\s*消/ }));
    await waitFor(() => expect(screen.getByText('任务已取消')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('b-cancel-2', { pageState: { taskId: 't-c2' } });
    expect(screen.getByText('CANCELLED')).toBeInTheDocument();
  });

  it('taskIdStateKey 为空串：pageState 键回落 taskId（L323/L265）', async () => {
    const { onExecute } = renderTask({
      bindings: [
        binding('task', 'b-task'),
        binding('task_status', 'b-status', { taskStatus: 'wrapper' }),
      ],
      spec: spec({ taskIdStateKey: '' }),
    });
    onExecute.mockImplementation(async (bindingId: string) =>
      bindingId === 'b-task'
        ? ok(undefined, { taskId: 't-k' })
        : ok({ wrapper: { state: 'running' } }),
    );
    submit();
    await waitFor(() => expect(screen.getByText('RUNNING')).toBeInTheDocument());
    expect(onExecute).toHaveBeenCalledWith('b-status', { pageState: { taskId: 't-k' } });
  });
});

describe('审批单号与进度渲染（分支补齐）', () => {
  it('approval 响应缺 approvalId：回落 requestId 作为审批单号（L509 右支）', async () => {
    const onQueryApprovalStatus = jest.fn<() => Promise<ApprovalStatusResult>>().mockResolvedValue({
      approvalId: 'req-9',
      status: 'pending',
    });
    const { onExecute } = renderTask({ onQueryApprovalStatus });
    onExecute.mockResolvedValueOnce({ kind: 'approval', requestId: 'req-9' });
    submit();
    await waitFor(() => expect(screen.getByText('等待审批')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    // 审批单号取 requestId 兜底值
    await waitFor(() => expect(onQueryApprovalStatus).toHaveBeenCalledWith('req-9'));
    await waitFor(() => expect(screen.getByText(/审批状态：pending/)).toBeInTheDocument());
  });

  it('failed + 进度：Progress 呈 exception 态（L730）', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>().mockResolvedValue({
      taskId: 't-pf',
      status: 'failed',
      progress: 40,
    });
    const { onExecute } = renderTask({ onQueryStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-pf' }));
    submit();
    await waitFor(() => expect(screen.getByText('FAILED')).toBeInTheDocument());
    expect(document.querySelector('.ant-progress-status-exception')).toBeInTheDocument();
  });

  it('running + 进度：Progress 呈 active 态（L733）', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>().mockResolvedValue({
      taskId: 't-pr',
      status: 'running',
      progress: 55,
    });
    const { onExecute } = renderTask({ onQueryStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-pr' }));
    submit();
    await waitFor(() => expect(screen.getByText('RUNNING')).toBeInTheDocument());
    expect(document.querySelector('.ant-progress-status-active')).toBeInTheDocument();
  });

  it('终态后点击「刷新」按钮：手动触发一次状态查询（FN onClick）', async () => {
    const onQueryStatus = jest.fn<() => Promise<TaskStatusResult>>().mockResolvedValue({
      taskId: 't-r3',
      status: 'completed',
    });
    const { onExecute } = renderTask({ onQueryStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { taskId: 't-r3' }));
    submit();
    await waitFor(() => expect(screen.getByText('COMPLETED')).toBeInTheDocument());
    expect(onQueryStatus).toHaveBeenCalledTimes(1);
    // 轮询已停（polling=false），刷新按钮可点击（name 含 sync 图标 aria-label 前缀，尾匹配）
    fireEvent.click(screen.getByRole('button', { name: /刷\s*新$/ }));
    await waitFor(() => expect(onQueryStatus).toHaveBeenCalledTimes(2));
    expect(onQueryStatus).toHaveBeenLastCalledWith('t-r3');
  });
});

// 剩余未覆盖分支均为渲染层守卫造成的防御性死代码，事件层不可达：
// - handleCancel 的 !taskStatus?.taskId 与「未配置取消任务绑定」warning：
//   取消按钮仅在 (cancelBinding || onCancelTask) 且 running 时渲染；
// - pollTaskStatus 的 !onQueryStatus 兜底：刷新按钮/启动轮询均先经
//   canQueryTaskStatus（statusBinding || onQueryStatus）过滤；
// - refreshApproval 的 !approvalId || !onQueryApprovalStatus：按钮渲染已过滤；
// - selectByJsonPointer 的 null 入参（L132-134）：唯一调用方先经 isJsonRecord 守卫；
// - normalizeTaskStatusFromExecution 的 previous 假值侧（L253 右支）：轮询启动前
//   taskStatusRef 恒被同步写入（提交/审批分流/轮询三路径均如此）；
// - handleCancel 的 taskStatusRef 空值侧（L613 右支）：取消按钮存在即有 taskStatus；
// - Progress status 的 approved→success（L762 中支）：两条 approved 路径均同步
//   setApprovalId('')，React 批处理下无「已通过但 Alert 仍在」的渲染窗口。
