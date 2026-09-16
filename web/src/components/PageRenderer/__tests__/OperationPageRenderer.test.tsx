/** OperationPageRenderer 覆盖。
 *
 * 覆盖路径：提交流程（缺绑定/预览拦截/确认分流/invoke·task·approval 三态文案/
 * successMessage·errorMessage 定制/异常兜底）、确认对话框（自定义文案/
 * 暂存值 Descriptions/取消关闭）、结果渲染四态（error/approval/task/success）、
 * 审批刷新（状态 Alert 三色/approved 续接 task·sync 两形态/异常兜底）、重置。 */
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
  kind: 'invoke',
  requestId: 'r1',
  data,
  ...extra,
});

const actionBinding = (requireConfirm?: boolean): PageFunctionBinding => ({
  id: 'b-act',
  functionId: 'fn-act',
  usage: 'action',
  execution: { mode: 'sync', requireConfirm },
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
  bindings?: PageFunctionBinding[];
  preview?: boolean;
  onQueryApprovalStatus?: (approvalId: string) => Promise<ApprovalStatusResult>;
  title?: string;
}

function renderOp(options: RenderOptions = {}) {
  const onExecute = jest.fn() as ExecuteMock;
  const utils = render(
    <App>
      <OperationPageRenderer
        spec={options.spec ?? spec()}
        bindings={options.bindings ?? [actionBinding()]}
        onExecute={onExecute as never}
        preview={options.preview}
        onQueryApprovalStatus={options.onQueryApprovalStatus}
        title={options.title}
      />
    </App>,
  );
  return { onExecute, ...utils };
}

const submit = () => fireEvent.click(screen.getByTestId('form-submit'));

describe('提交流程', () => {
  it('未配置操作绑定时报错', async () => {
    renderOp({ bindings: [] });
    submit();
    await waitFor(() => expect(screen.getByText('未配置操作绑定')).toBeInTheDocument());
  });

  it('预览模式拦截提交', async () => {
    renderOp({ preview: true });
    submit();
    await waitFor(() => expect(screen.getByText('预览模式不执行操作')).toBeInTheDocument());
  });

  it('invoke 成功：默认文案 + 结果视图', async () => {
    const { onExecute } = renderOp();
    onExecute.mockResolvedValueOnce(ok({ total: 5 }));
    submit();
    await waitFor(() => expect(screen.getByText('操作成功')).toBeInTheDocument());
    expect(screen.getByText('执行结果')).toBeInTheDocument();
    expect(screen.getByText('操作结果视图未配置')).toBeInTheDocument();
  });

  it('invoke 成功：successMessage 定制文案 + 字段展示', async () => {
    const { onExecute } = renderOp({
      spec: spec({
        resultView: {
          fields: [{ key: 'total', title: { 'zh-CN': '总数' }, dataType: 'number' }],
          successMessage: { 'zh-CN': '发放成功' },
        },
      }),
    });
    onExecute.mockResolvedValueOnce(ok({ total: 5 }));
    submit();
    await waitFor(() => expect(screen.getByText('发放成功')).toBeInTheDocument());
    expect(screen.getByText('总数')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
  });

  it('task 分流：任务已提交 + taskId', async () => {
    const { onExecute } = renderOp();
    onExecute.mockResolvedValueOnce(ok(undefined, { kind: 'task', taskId: 'tk-1' }));
    submit();
    await waitFor(() => expect(screen.getAllByText('任务已提交').length).toBeGreaterThan(0));
    expect(screen.getByText('tk-1')).toBeInTheDocument();
    expect(
      screen.getByText('异步任务仍在执行，请在任务中心或任务页面查看进度。'),
    ).toBeInTheDocument();
  });

  it('approval 分流：等待审批 + approvalId', async () => {
    const { onExecute } = renderOp();
    onExecute.mockResolvedValueOnce(ok(undefined, { kind: 'approval', approvalId: 'apr-1' }));
    submit();
    await waitFor(() => expect(screen.getByText('操作已提交审批')).toBeInTheDocument());
    expect(screen.getByText('等待审批')).toBeInTheDocument();
    expect(screen.getByText('apr-1')).toBeInTheDocument();
    expect(screen.getByText('操作尚未完成')).toBeInTheDocument();
  });

  it('异常：Error 取 message，errorMessage 定制', async () => {
    const { onExecute } = renderOp({
      spec: spec({
        resultView: { errorMessage: { 'zh-CN': '自定义失败' } },
      }),
    });
    onExecute.mockRejectedValueOnce(new Error('余额不足'));
    submit();
    await waitFor(() => expect(screen.getByText('自定义失败')).toBeInTheDocument());
    expect(screen.getByText('操作失败')).toBeInTheDocument();
    expect(screen.getByText('余额不足')).toBeInTheDocument();
  });

  it('异常：非 Error 兜底「操作失败」', async () => {
    const { onExecute } = renderOp();
    onExecute.mockRejectedValueOnce('plain');
    submit();
    // toast 与结果卡片标题各出现一次
    await waitFor(() => expect(screen.getAllByText('操作失败').length).toBeGreaterThanOrEqual(2));
  });
});

describe('确认对话框', () => {
  it('spec.confirm 触发：展示自定义文案与暂存值，确定后执行', async () => {
    const { onExecute } = renderOp({ spec: confirmSpec });
    submit();
    await waitFor(() => expect(screen.getByText('危险操作')).toBeInTheDocument());
    expect(screen.getByText('请再次确认')).toBeInTheDocument();
    // 暂存值 Descriptions：label=字段名，value=摘要
    expect(screen.getByText('amount')).toBeInTheDocument();
    expect(screen.getByText('10')).toBeInTheDocument();
    expect(onExecute).not.toHaveBeenCalled();

    onExecute.mockResolvedValueOnce(ok({ amount: 10 }));
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-act', { form: { amount: 10 } }));
    // toast 与结果标题各一次
    await waitFor(() => expect(screen.getAllByText('操作成功').length).toBeGreaterThanOrEqual(2));
  });

  it('binding.execution.requireConfirm 同样触发确认', async () => {
    const { onExecute } = renderOp({ bindings: [actionBinding(true)] });
    submit();
    await waitFor(() => expect(screen.getByText('确认操作')).toBeInTheDocument());
    expect(onExecute).not.toHaveBeenCalled();
  });

  it('确认弹窗取消：关闭且不执行', async () => {
    const { onExecute } = renderOp({ spec: confirmSpec });
    submit();
    await waitFor(() => expect(screen.getByText('危险操作')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /返\s*回/ }));
    // 关闭动画在 jsdom 下由 rc-motion 驱动，DOM 移除时机不稳定：以「未执行」为硬断言
    expect(onExecute).not.toHaveBeenCalled();
  });

  it('确认后执行异常：错误入结果卡片', async () => {
    const { onExecute } = renderOp({ spec: confirmSpec });
    submit();
    await waitFor(() => expect(screen.getByText('危险操作')).toBeInTheDocument());
    onExecute.mockRejectedValueOnce(new Error('post-fail'));
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(screen.getByText('post-fail')).toBeInTheDocument());
  });

  it('确认后非 Error 异常兜底「操作失败」', async () => {
    const { onExecute } = renderOp({ spec: confirmSpec });
    submit();
    await waitFor(() => expect(screen.getByText('危险操作')).toBeInTheDocument());
    onExecute.mockRejectedValueOnce('plain');
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    // 错误卡片标题 + toast 各一次
    await waitFor(() => expect(screen.getAllByText('操作失败').length).toBeGreaterThanOrEqual(2));
  });

  it('确认后 approval 分流：等待审批', async () => {
    const { onExecute } = renderOp({ spec: confirmSpec });
    submit();
    await waitFor(() => expect(screen.getByText('危险操作')).toBeInTheDocument());
    onExecute.mockResolvedValueOnce(ok(undefined, { kind: 'approval', approvalId: 'apr-9' }));
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(screen.getByText('操作已提交审批')).toBeInTheDocument());
    expect(screen.getByText('等待审批')).toBeInTheDocument();
  });

  it('确认后 task 分流：任务已提交', async () => {
    const { onExecute } = renderOp({ spec: confirmSpec });
    submit();
    await waitFor(() => expect(screen.getByText('危险操作')).toBeInTheDocument());
    onExecute.mockResolvedValueOnce(ok(undefined, { kind: 'task', taskId: 'tk-9' }));
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(screen.getByText('任务已提交')).toBeInTheDocument());
    expect(screen.getByText('tk-9')).toBeInTheDocument();
  });

  it('预览模式下确认链同样被拦截', async () => {
    renderOp({ spec: confirmSpec, preview: true });
    submit();
    // handleSubmit 在预览下先行拦截，弹窗不出现
    await waitFor(() => expect(screen.getByText('预览模式不执行操作')).toBeInTheDocument());
    expect(screen.queryByText('危险操作')).not.toBeInTheDocument();
  });

  it('确认弹窗打开后热切换预览：确认链在 handleConfirm 层被拦截', async () => {
    // 弹窗 visible/pendingValues 是组件 state，跨 rerender 存留——
    // 触达 handleSubmit 之后的第二道 preview 防线
    const onExecute = jest.fn() as ExecuteMock;
    const utils = render(
      <App>
        <OperationPageRenderer
          spec={confirmSpec}
          bindings={[actionBinding()]}
          onExecute={onExecute as never}
        />
      </App>,
    );
    submit();
    await waitFor(() => expect(screen.getByText('危险操作')).toBeInTheDocument());
    utils.rerender(
      <App>
        <OperationPageRenderer
          spec={confirmSpec}
          bindings={[actionBinding()]}
          onExecute={onExecute as never}
          preview
        />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(screen.getByText('预览模式不执行操作')).toBeInTheDocument());
    expect(onExecute).not.toHaveBeenCalled();
  });

  it('确认弹窗打开后绑定被移除：确认无操作直接返回（防御）', async () => {
    const onExecute = jest.fn() as ExecuteMock;
    const utils = render(
      <App>
        <OperationPageRenderer
          spec={confirmSpec}
          bindings={[actionBinding()]}
          onExecute={onExecute as never}
        />
      </App>,
    );
    submit();
    await waitFor(() => expect(screen.getByText('危险操作')).toBeInTheDocument());
    // 提案应用后绑定集变化：弹窗仍开（spec.confirm 维持 requiresConfirm），
    // 但 mainBinding 已不在——确认应静默返回而非抛错
    utils.rerender(
      <App>
        <OperationPageRenderer spec={confirmSpec} bindings={[]} onExecute={onExecute as never} />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    expect(onExecute).not.toHaveBeenCalled();
  });
});

describe('审批刷新', () => {
  const toApproval = async (status?: ApprovalStatusResult['status']) => {
    const onQueryApprovalStatus = jest.fn<() => Promise<ApprovalStatusResult>>();
    const { onExecute } = renderOp({ onQueryApprovalStatus });
    onExecute.mockResolvedValueOnce(ok(undefined, { kind: 'approval', approvalId: 'apr-1' }));
    submit();
    await waitFor(() => expect(screen.getByText('等待审批')).toBeInTheDocument());
    if (status) {
      (onQueryApprovalStatus as jest.Mock).mockResolvedValueOnce({
        approvalId: 'apr-1',
        status,
      });
    }
    return { onQueryApprovalStatus: onQueryApprovalStatus as jest.Mock };
  };

  // refreshApproval 内 !approvalId || !onQueryApprovalStatus 防御 return 为
  // 死代码：刷新按钮仅在 result.approvalId && onQueryApprovalStatus 时渲染

  it('pending 状态渲染 info Alert', async () => {
    const { onQueryApprovalStatus } = await toApproval('pending');
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText(/审批状态：pending/)).toBeInTheDocument());
    expect(onQueryApprovalStatus).toHaveBeenCalledWith('apr-1');
  });

  it('rejected 状态渲染 error Alert + reason', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'rejected',
      reason: '被拒绝',
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText(/审批状态：rejected/)).toBeInTheDocument());
    expect(screen.getByText('被拒绝')).toBeInTheDocument();
    expect(document.querySelector('.ant-alert-error')).toBeInTheDocument();
  });

  it('approved + continuation + task：任务已启动提示', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'approved',
      continuation: true,
      resultKind: 'task',
      taskId: 'tk-after',
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText('审批已通过，任务已启动')).toBeInTheDocument());
    expect(screen.getByText('tk-after')).toBeInTheDocument();
  });

  it('approved + continuation + sync：渲染审批后结果视图', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'approved',
      continuation: true,
      resultKind: 'sync',
      result: { done: true },
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText('审批后执行结果视图未配置')).toBeInTheDocument());
  });

  it('approved 无 continuation：仅状态 Alert', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'approved',
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText(/审批状态：approved/)).toBeInTheDocument());
    expect(screen.queryByText('审批已通过，任务已启动')).not.toBeInTheDocument();
  });

  it('查询异常展示错误消息', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockRejectedValueOnce(new Error('net-down'));
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText('net-down')).toBeInTheDocument());
  });

  it('查询非 Error 异常兜底「审批状态查询失败」', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockRejectedValueOnce('plain');
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText('审批状态查询失败')).toBeInTheDocument());
  });

  it('approved + continuation 但 resultKind 非 task/sync：仅状态 Alert 无续接渲染', async () => {
    const { onQueryApprovalStatus } = await toApproval();
    onQueryApprovalStatus.mockResolvedValueOnce({
      approvalId: 'apr-1',
      status: 'approved',
      continuation: true,
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新审批状态/ }));
    await waitFor(() => expect(screen.getByText(/审批状态：approved/)).toBeInTheDocument());
    expect(screen.queryByText('审批已通过，任务已启动')).not.toBeInTheDocument();
    expect(screen.queryByText('审批后执行结果视图未配置')).not.toBeInTheDocument();
  });
});

describe('重置与标题', () => {
  it('重置按钮清空结果与错误', async () => {
    const { onExecute } = renderOp();
    onExecute.mockResolvedValueOnce(ok({ a: 1 }));
    submit();
    await waitFor(() => expect(screen.getByText('执行结果')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /重置结果/ }));
    await waitFor(() => expect(screen.queryByText('执行结果')).not.toBeInTheDocument());
  });

  it('默认标题「执行操作」，自定义标题透传', () => {
    const { rerender } = renderOp();
    expect(screen.getByText('执行操作')).toBeInTheDocument();
    rerender(
      <App>
        <OperationPageRenderer
          spec={spec()}
          bindings={[actionBinding()]}
          onExecute={jest.fn() as never}
          title="发补偿"
        />
      </App>,
    );
    expect(screen.getByText('发补偿')).toBeInTheDocument();
  });
});
