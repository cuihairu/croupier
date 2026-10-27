/** 操作页多阶段状态反馈 Steps（D1 状态反馈）覆盖。
 *
 * 阶段条由结果卡同源状态派生、追加展示：断言阶段标题顺序与 antd Steps 的
 * item 状态类（ant-steps-item-<status>），覆盖同步成功/任务进行中/审批
 * （未查询·拒绝·通过转任务）/提交失败/确认链/重置各形态，以及未执行不渲染。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import OperationPageRenderer from '../OperationPageRenderer';
import type {
  ApprovalStatusResult,
  OperationPageSpec,
  PageExecutionResult,
  PageFunctionBinding,
} from '@/types/dashboard';

// SchemaFormRenderer 替身：渲染提交按钮直接触发 onFinish
jest.mock('@/components/SchemaFormRenderer', () => ({
  __esModule: true,
  default: jest.fn(({ onFinish }: { onFinish: (v: Record<string, unknown>) => void }) => (
    <button type="button" data-testid="form-submit" onClick={() => onFinish({ amount: 10 })}>
      form-submit
    </button>
  )),
}));

type ExecuteMock = jest.Mock<Promise<PageExecutionResult>, [string, unknown]>;

const ok = (data?: unknown, extra?: Partial<PageExecutionResult>): PageExecutionResult => ({
  kind: 'sync',
  requestId: 'r1',
  data,
  ...extra,
});

const actionBinding = (): PageFunctionBinding => ({
  id: 'b-act',
  functionId: 'fn-act',
  usage: 'action',
  execution: { mode: 'sync' },
});

const spec = (overrides: Partial<OperationPageSpec> = {}): OperationPageSpec => ({
  form: { jsonSchema: { type: 'object' } },
  ...overrides,
});

const confirmSpec = spec({
  confirm: {
    title: { 'zh-CN': '危险操作' },
    description: { 'zh-CN': '请再次确认' },
    confirmText: { 'zh-CN': '执行' },
    cancelText: { 'zh-CN': '返回' },
    bindingId: 'b-act',
  },
});

interface RenderOptions {
  spec?: OperationPageSpec;
  onQueryApprovalStatus?: (approvalId: string) => Promise<ApprovalStatusResult>;
}

function renderOp(options: RenderOptions = {}) {
  const onExecute = jest.fn() as ExecuteMock;
  const utils = render(
    <App>
      <OperationPageRenderer
        spec={options.spec ?? spec()}
        bindings={[actionBinding()]}
        onExecute={onExecute as never}
        onQueryApprovalStatus={options.onQueryApprovalStatus}
      />
    </App>,
  );
  return { onExecute, ...utils };
}

const submit = () => fireEvent.click(screen.getByTestId('form-submit'));

/** 阶段标题所在的 Steps item 节点；className 携带 ant-steps-item-<status>。 */
const stage = (title: string): HTMLElement =>
  screen.getByText(title).closest('.ant-steps-item') as HTMLElement;

const expectStage = (title: string, status: string) => {
  expect(stage(title).className).toContain(`ant-steps-item-${status}`);
};

describe('操作页阶段条（D1 状态反馈）', () => {
  it('未执行不渲染阶段条', () => {
    renderOp();
    expect(screen.queryByText('提交操作')).not.toBeInTheDocument();
    expect(screen.queryByText('审批')).not.toBeInTheDocument();
    expect(screen.queryByText('执行')).not.toBeInTheDocument();
  });

  it('同步执行成功：提交与执行均完成', async () => {
    const { onExecute } = renderOp();
    onExecute.mockResolvedValueOnce(ok({ total: 5 }));
    submit();
    await waitFor(() => expect(screen.getByText('操作成功')).toBeInTheDocument());
    expectStage('提交操作', 'finish');
    expectStage('执行', 'finish');
    expect(screen.getByText('执行结果')).toBeInTheDocument();
    expect(document.querySelectorAll('.ant-steps-item-error')).toHaveLength(0);
  });

  it('任务已提交：执行阶段保持进行中', async () => {
    const { onExecute } = renderOp();
    onExecute.mockResolvedValueOnce(ok(undefined, { kind: 'task', taskId: 'tk-1' }));
    submit();
    await waitFor(() => expect(screen.getByText('任务已提交')).toBeInTheDocument());
    expectStage('提交操作', 'finish');
    expectStage('执行', 'process');
    expect(document.querySelector('.ant-steps-item-icon-number')).toBeInTheDocument();
  });

  it('已提交审批尚未查询状态：审批进行中、执行待审批', async () => {
    const { onExecute } = renderOp();
    onExecute.mockResolvedValueOnce(ok(undefined, { kind: 'approval', approvalId: 'apr-1' }));
    submit();
    await waitFor(() => expect(screen.getByText('等待审批')).toBeInTheDocument());
    expectStage('提交操作', 'finish');
    expectStage('审批', 'process');
    expectStage('执行', 'wait');
  });

  it('审批被拒绝：审批阶段 error，不推进执行', async () => {
    const onQueryApprovalStatus = jest.fn<() => Promise<ApprovalStatusResult>>();
    const { onExecute } = renderOp({ onQueryApprovalStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { kind: 'approval', approvalId: 'apr-1' }));
    submit();
    await waitFor(() => expect(screen.getByText('等待审批')).toBeInTheDocument());
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'rejected',
      reason: '被拒绝',
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText(/审批状态：rejected/)).toBeInTheDocument());
    expectStage('审批', 'error');
    expectStage('执行', 'wait');
    expect(document.querySelector('.ant-alert-error')).toBeInTheDocument();
  });

  it('审批通过并转任务执行：审批完成、执行进行中', async () => {
    const onQueryApprovalStatus = jest.fn<() => Promise<ApprovalStatusResult>>();
    const { onExecute } = renderOp({ onQueryApprovalStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { kind: 'approval', approvalId: 'apr-1' }));
    submit();
    await waitFor(() => expect(screen.getByText('等待审批')).toBeInTheDocument());
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'approved',
      continuation: true,
      resultKind: 'task',
      taskId: 'tk-after',
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText('审批已通过，任务已启动')).toBeInTheDocument());
    expectStage('审批', 'finish');
    expectStage('执行', 'process');
  });

  it('提交失败：终态停在提交阶段 error，不渲染未到达的阶段', async () => {
    const { onExecute } = renderOp();
    onExecute.mockRejectedValueOnce(new Error('余额不足'));
    submit();
    await waitFor(() => expect(screen.getByText('余额不足')).toBeInTheDocument());
    expectStage('提交操作', 'error');
    expect(screen.queryByText('审批')).not.toBeInTheDocument();
    expect(screen.queryByText('执行')).not.toBeInTheDocument();
    expect(document.querySelectorAll('.ant-steps-item-error')).toHaveLength(1);
  });

  it('确认后执行：确认链同样产出阶段条', async () => {
    const { onExecute } = renderOp({ spec: confirmSpec });
    submit();
    await waitFor(() => expect(screen.getByText('危险操作')).toBeInTheDocument());
    expect(screen.queryByText('提交操作')).not.toBeInTheDocument();
    onExecute.mockResolvedValueOnce(ok({ amount: 10 }));
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(screen.getAllByText('操作成功').length).toBeGreaterThanOrEqual(1));
    expectStage('提交操作', 'finish');
    expectStage('执行', 'finish');
  });

  it('重置后阶段条一并清除', async () => {
    const { onExecute } = renderOp();
    onExecute.mockResolvedValueOnce(ok({ a: 1 }));
    submit();
    await waitFor(() => expectStage('提交操作', 'finish'));
    fireEvent.click(screen.getByRole('button', { name: /重置结果/ }));
    await waitFor(() => expect(screen.queryByText('提交操作')).not.toBeInTheDocument());
    expect(screen.queryByText('执行')).not.toBeInTheDocument();
  });
});
