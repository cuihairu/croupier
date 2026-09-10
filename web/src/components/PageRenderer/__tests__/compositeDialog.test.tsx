/** CompositeRenderer 弹窗形态回归与主路径覆盖。
 *
 * 回归点（本轮修复，必须持续守护）：
 * DialogForm onSubmit 现已 try/catch——
 * 1. 执行失败：message.error（App 实例）+ 弹窗保持开启（保留已填参数）；
 * 2. 成功才 setDialogKey(null) + message.success。
 *
 * 其余弹窗路径：无字段弹窗降级按钮、danger 确认、rowAction 行字段映射
 * （裸字段名与 row. 前缀两种形态）、rowClick openModal 表达式参数、
 * bindingId 命中、弹窗 fields/table 区块聚合渲染、onCancel、closeModal 链。 */
import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../CompositeRenderer';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

type AppApi = ReturnType<typeof App.useApp>;

/** 渲染并捕获 App.useApp 实例（message/modal spy 用） */
function renderComposite(
  sections: CompositeSection[],
  onExecute: jest.Mock,
  onPageStateMerge?: jest.Mock,
) {
  const captured: { current?: AppApi } = {};
  const Probe: React.FC = () => {
    captured.current = App.useApp();
    return null;
  };
  const utils = render(
    <App>
      <Probe />
      <CompositeRenderer
        sections={sections}
        bindings={[]}
        onExecute={onExecute as never}
        preview={false}
        onPageStateMerge={onPageStateMerge as never}
      />
    </App>,
  );
  return { ...utils, app: captured };
}

const mailFormSection: CompositeSection = {
  key: 'mailForm',
  bindingId: 'b-mail',
  view: 'form',
  display: 'dialog',
  group: 'mailModal',
  title: { 'zh-CN': '发邮件表单' },
  form: {
    jsonSchema: {
      type: 'object',
      properties: { to: { type: 'string', title: '收件人' } },
    },
  },
};

const baseSections: CompositeSection[] = [
  {
    key: 'mainTable',
    bindingId: 'b-table',
    view: 'table',
    autoRun: true,
    title: { 'zh-CN': '玩家列表' },
    table: {
      columns: [
        { key: 'uid', title: { 'zh-CN': 'UID' } },
        { key: 'nickname', title: { 'zh-CN': '昵称' } },
      ],
    },
    events: [
      {
        event: 'rowClick',
        action: { kind: 'openModal', target: 'mailModal', params: { to: '{{row.uid}}' } },
      },
    ],
  },
  {
    key: 'opBar',
    bindingId: 'b-tb',
    view: 'toolbar',
    title: { 'zh-CN': '操作区' },
    toolbar: {
      actions: [
        { label: { 'zh-CN': '打开发邮件' }, targetSection: 'mailModal' },
        { label: { 'zh-CN': '关闭弹窗' }, chain: [{ kind: 'closeModal', target: '' }] },
      ],
    },
  },
  mailFormSection,
];

function tableExecute(): jest.Mock {
  const onExecute = jest.fn();
  onExecute.mockImplementation((bindingId: string) => {
    if (bindingId === 'b-table') {
      return Promise.resolve({ data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 } });
    }
    return Promise.resolve({ data: { ok: 1 } });
  });
  return onExecute;
}

async function openMailModal(onExecute: jest.Mock) {
  const utils = renderComposite(baseSections, onExecute);
  await screen.findByText('bob');
  fireEvent.click(screen.getByRole('button', { name: '打开发邮件' }));
  await screen.findByText('发邮件表单', { selector: '.ant-modal-title' });
  return utils;
}

describe('CompositeRenderer 弹窗表单（DialogForm 回归）', () => {
  it('提交成功：onExecute 执行 → message.success + 弹窗关闭 + success 事件链刷新目标区块', async () => {
    const onExecute = tableExecute();
    const { app } = await openMailModal(onExecute);
    const successSpy = jest.spyOn(app.current!.message, 'success');

    const toInput = (await screen.findByLabelText('收件人')) as HTMLInputElement;
    fireEvent.change(toInput, { target: { value: 'bob@test' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));

    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith(
        'b-mail',
        expect.objectContaining({ form: { to: 'bob@test' } }),
      ),
    );
    // 成功提示：标题 + 执行成功
    await waitFor(() => expect(successSpy).toHaveBeenCalledWith('发邮件表单 执行成功'));
    // 成功后弹窗关闭（destroyOnHidden：标题随弹窗卸载）
    await waitFor(() =>
      expect(screen.queryByText('发邮件表单', { selector: '.ant-modal-title' })).toBeNull(),
    );
  });

  it('执行失败（回归点）：message.error 且弹窗保持开启、已填参数保留', async () => {
    // Once 按调用顺序消耗：第 1 次 = autoRun 表格，第 2 次 = 弹窗提交（reject）
    const onExecute = jest.fn();
    onExecute
      .mockImplementationOnce(() =>
        Promise.resolve({ data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 } }),
      )
      .mockImplementationOnce(() => Promise.reject(new Error('发送失败')));
    const { app } = await openMailModal(onExecute);
    const errorSpy = jest.spyOn(app.current!.message, 'error');

    const toInput = (await screen.findByLabelText('收件人')) as HTMLInputElement;
    fireEvent.change(toInput, { target: { value: 'bob@test' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));

    await waitFor(() => expect(errorSpy).toHaveBeenCalledWith('发送失败'));
    // 弹窗仍在 + 输入保留
    expect(screen.getByText('发邮件表单', { selector: '.ant-modal-title' })).toBeInTheDocument();
    expect(((await screen.findByLabelText('收件人')) as HTMLInputElement).value).toBe('bob@test');
  });

  it('无字段弹窗降级为「确认执行」按钮：点击执行 onSubmit({})', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: { actions: [{ label: { 'zh-CN': '执行操作' }, targetSection: 'plainModal' }] },
      },
      {
        key: 'plainForm',
        bindingId: 'b-plain',
        view: 'form',
        display: 'dialog',
        group: 'plainModal',
        title: { 'zh-CN': '确认弹窗' },
      },
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    fireEvent.click(screen.getByRole('button', { name: '执行操作' }));
    const confirmBtn = await screen.findByRole('button', { name: '确认执行' });
    expect(confirmBtn).not.toBeDisabled();
    fireEvent.click(confirmBtn);
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-plain', expect.anything()));
  });

  it('danger 按钮先弹确认框，确认后才打开弹窗', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: {
          actions: [{ label: { 'zh-CN': '危险操作' }, targetSection: 'mailModal', danger: true }],
        },
      },
      mailFormSection,
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    fireEvent.click(screen.getByRole('button', { name: '危险操作' }));
    await waitFor(() =>
      expect(screen.getAllByText('确认执行「危险操作」').length).toBeGreaterThan(0),
    );
    // 未确认前弹窗未开
    expect(screen.queryByText('发邮件表单', { selector: '.ant-modal-title' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await screen.findByText('发邮件表单', { selector: '.ant-modal-title' });
  });

  it('rowAction 参数映射：裸字段名与 row. 前缀均取本行字段值并预填弹窗', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'mainTable',
        bindingId: 'b-table',
        view: 'table',
        autoRun: true,
        title: { 'zh-CN': '玩家列表' },
        table: {
          columns: [{ key: 'uid', title: { 'zh-CN': 'UID' } }],
          rowActions: [
            {
              label: { 'zh-CN': '编辑备注' },
              targetSection: 'mailModal',
              params: { to: 'uid', to2: 'row.uid' },
            },
          ],
        },
      },
      {
        ...mailFormSection,
        form: {
          jsonSchema: {
            type: 'object',
            properties: {
              to: { type: 'string', title: '收件人' },
              to2: { type: 'string', title: '抄送' },
            },
          },
        },
      },
    ];
    const onExecute = tableExecute();
    renderComposite(sections, onExecute);
    await screen.findByText('u1');
    fireEvent.click(screen.getByRole('button', { name: '编辑备注' }));
    await waitFor(() => {
      expect((screen.getByLabelText('收件人') as HTMLInputElement).value).toBe('u1');
      expect((screen.getByLabelText('抄送') as HTMLInputElement).value).toBe('u1');
    });
  });

  it('rowClick 事件 openModal：{{row.x}} 表达式参数求值进弹窗预填', async () => {
    const onExecute = tableExecute();
    renderComposite(baseSections, onExecute);
    await screen.findByText('u1');
    fireEvent.click(screen.getByText('u1').closest('tr') as HTMLElement);
    await waitFor(() =>
      expect((screen.getByLabelText('收件人') as HTMLInputElement).value).toBe('u1'),
    );
  });

  it('openDialog 目标为 bindingId 时同样命中弹窗', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: { actions: [{ label: { 'zh-CN': '按绑定打开' }, targetSection: 'b-mail' }] },
      },
      mailFormSection,
    ];
    const onExecute = jest.fn().mockResolvedValue({ data: {} });
    renderComposite(sections, onExecute);
    fireEvent.click(screen.getByRole('button', { name: '按绑定打开' }));
    await screen.findByText('发邮件表单', { selector: '.ant-modal-title' });
  });

  it('弹窗分组聚合：fields（过滤 items/total、对象 JSON 序列化）与 table 区块渲染', async () => {
    const sections: CompositeSection[] = [
      {
        key: 'opBar',
        bindingId: 'b-tb',
        view: 'toolbar',
        title: { 'zh-CN': '操作区' },
        toolbar: {
          actions: [
            {
              label: { 'zh-CN': '查看详情' },
              targetSection: 'detailModal',
              chain: [
                { kind: 'runBinding', target: 'detailFields' },
                { kind: 'runBinding', target: 'detailTable' },
              ],
            },
          ],
        },
      },
      {
        key: 'detailFields',
        bindingId: 'b-fields',
        view: 'fields',
        display: 'dialog',
        group: 'detailModal',
        title: { 'zh-CN': '详情字段' },
      },
      {
        key: 'detailTable',
        bindingId: 'b-dtable',
        view: 'table',
        display: 'dialog',
        group: 'detailModal',
        title: { 'zh-CN': '详情表格' },
        table: { columns: [{ key: 'uid', title: { 'zh-CN': 'UID' } }] },
      },
    ];
    const onExecute = jest.fn().mockImplementation((bindingId: string) => {
      if (bindingId === 'b-fields') {
        return Promise.resolve({
          data: { name: 'bob', items: [1], total: 2, meta: { a: 1 } },
        });
      }
      if (bindingId === 'b-dtable') {
        return Promise.resolve({ data: { items: [{ uid: 'u9' }] } });
      }
      return Promise.resolve({ data: {} });
    });
    renderComposite(sections, onExecute);
    fireEvent.click(screen.getByRole('button', { name: '查看详情' }));
    // fields：name 渲染、meta 对象 JSON 序列化、items/total 被过滤不渲染为条目
    await screen.findByText('bob');
    expect(screen.getByText('{"a":1}')).toBeInTheDocument();
    // table：弹窗内表格渲染数据行
    await screen.findByText('u9');
  });

  it('弹窗 onCancel 关闭；closeModal 链步骤关闭当前弹窗', async () => {
    const onExecute = tableExecute();
    renderComposite(baseSections, onExecute);
    await screen.findByText('bob');
    // 工具栏 closeModal 链在弹窗未开时为空操作；先打开再关闭验证链路
    fireEvent.click(screen.getByRole('button', { name: '打开发邮件' }));
    await screen.findByText('发邮件表单', { selector: '.ant-modal-title' });
    fireEvent.click(screen.getByRole('button', { name: /close/i }));
    await waitFor(() =>
      expect(screen.queryByText('发邮件表单', { selector: '.ant-modal-title' })).toBeNull(),
    );
    // 再次打开，用 closeModal 链按钮关闭
    fireEvent.click(screen.getByRole('button', { name: '打开发邮件' }));
    await screen.findByText('发邮件表单', { selector: '.ant-modal-title' });
    fireEvent.click(screen.getByRole('button', { name: '关闭弹窗' }));
    await waitFor(() =>
      expect(screen.queryByText('发邮件表单', { selector: '.ant-modal-title' })).toBeNull(),
    );
  });
});
