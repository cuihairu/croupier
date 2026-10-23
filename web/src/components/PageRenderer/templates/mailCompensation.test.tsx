/** mailCompensation 模板覆盖：PageSpec 结构 + demo 执行器主要分支。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import MailCompensationTemplate, {
  createMailCompensationDemoExecute,
  mailCompensationPageSpec,
} from './mailCompensation';
import { createGameOpsDemoStore } from './gameOpsDemoData';
import type { BindingExecutionContext, JSONValue } from '@/types/dashboard';

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ title, children }: { title?: React.ReactNode; children?: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}));
jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="page-renderer" />,
}));

const run = (
  execute: ReturnType<typeof createMailCompensationDemoExecute>,
  bindingId: string,
  context: BindingExecutionContext = {},
) => execute(bindingId, context) as Promise<{ kind: string; data: JSONValue }>;

describe('mailCompensation PageSpec', () => {
  it('归属 mail 分类，list 绑定带 items/total 输出，动作绑定齐备', () => {
    expect(mailCompensationPageSpec.category.key).toBe('mail');
    expect(mailCompensationPageSpec.type).toBe('resource');
    if (mailCompensationPageSpec.type !== 'resource') return;
    const list = mailCompensationPageSpec.bindings.find((b) => b.id === 'list');
    expect(list?.selectors?.output?.map((o) => o.stateKey)).toEqual(['items', 'total']);
    for (const action of mailCompensationPageSpec.resource.listView?.rowActions ?? []) {
      expect(mailCompensationPageSpec.bindings.some((b) => b.id === action.bindingId)).toBe(true);
    }
  });
});

describe('mailCompensation demo 执行器', () => {
  it('list：分页/类型筛选/默认兜底', async () => {
    const execute = createMailCompensationDemoExecute();
    const page1 = await run(execute, 'list', { form: { current: 1, pageSize: 2 } });
    expect(page1.data).toMatchObject({ total: 4 });
    const compensationOnly = await run(execute, 'list', {
      form: { current: 1, pageSize: 10, type: 'compensation' },
    });
    expect(compensationOnly.data).toMatchObject({ total: 2 });
    const empty = await run(execute, 'list', {
      form: { current: 9, pageSize: 10, status: 'failed' },
    });
    expect(empty.data).toMatchObject({ items: [], total: 0 });
  });

  it('detail：命中与未命中', async () => {
    const execute = createMailCompensationDemoExecute();
    const hit = await run(execute, 'detail', { row: { id: 'task-mail-1001' } });
    expect(hit.data).toMatchObject({ title: 'S3 赛季结算奖励' });
    const miss = await run(execute, 'detail', { row: { id: 'nope' } });
    expect(miss.data).toBeNull();
  });

  it('create：邮件直接发送完成，补偿进入待审批并落操作记录', async () => {
    const store = createGameOpsDemoStore();
    const execute = createMailCompensationDemoExecute(store);
    const mail = await run(execute, 'create', {
      form: { type: 'mail', title: '周常奖励', detail: '发放' },
    });
    expect(mail.data).toMatchObject({ approvalId: null });
    const comp = await run(execute, 'create', {
      form: { type: 'compensation', title: '事故补偿', perAmount: 100 },
    });
    expect((comp.data as { approvalId: string }).approvalId).toBeTruthy();
    const list = await run(execute, 'list', { form: { current: 1, pageSize: 10 } });
    expect(list.data).toMatchObject({ total: 6 });
    expect(store.opRecords[0].result).toBe('pending_approval');
  });

  it('retryFailed：无失败目标报错，成功后失败清零', async () => {
    const execute = createMailCompensationDemoExecute();
    await expect(
      run(execute, 'action.retryFailed', { row: { id: 'task-mail-1001' } }),
    ).rejects.toThrow('no_failed_targets');
    const result = await run(execute, 'action.retryFailed', {
      row: { id: 'task-mail-1002' },
    });
    expect(result.data).toMatchObject({ retried: 2 });
    const detail = await run(execute, 'detail', { row: { id: 'task-mail-1002' } });
    expect(detail.data).toMatchObject({ failedCount: 0, status: 'sent' });
    await expect(run(execute, 'action.retryFailed', { row: { id: 'nope' } })).rejects.toThrow(
      'task_not_found',
    );
  });

  it('cancelTask：已完成任务拒绝撤回，进行中任务可撤回', async () => {
    const execute = createMailCompensationDemoExecute();
    await expect(
      run(execute, 'action.cancelTask', { row: { id: 'task-mail-1001' } }),
    ).rejects.toThrow('task_already_finished');
    const result = await run(execute, 'action.cancelTask', {
      row: { id: 'task-comp-2001' },
    });
    expect(result.data).toMatchObject({ success: true });
    await expect(run(execute, 'action.cancelTask', { row: { id: 'nope' } })).rejects.toThrow(
      'task_not_found',
    );
  });

  it('未知 binding 走默认成功分支，requestId 递增', async () => {
    const execute = createMailCompensationDemoExecute();
    const first = await run(execute, 'unknown');
    const second = await run(execute, 'unknown');
    expect(first.requestId).not.toBe(second.requestId);
    expect(first.data).toMatchObject({ success: true });
  });
});

describe('MailCompensationTemplate 组件', () => {
  it('挂载渲染标题与 PageRenderer', () => {
    render(<MailCompensationTemplate />);
    expect(screen.getByRole('heading', { name: '邮件与补偿' })).toBeInTheDocument();
    expect(screen.getByTestId('page-renderer')).toBeInTheDocument();
  });
});
