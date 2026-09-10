/** 发布渲染端：编辑器把按钮编译到 table section 的 toolbar（compiler
 * compileButton），渲染端必须在表格卡片头部消费——否则按钮发布后消失。
 * 同时覆盖 dialog 分组聚合：toolbar 按钮 targetSection=group 名。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../index';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

const sections: CompositeSection[] = [
  {
    key: 'playerListTable',
    bindingId: 'b-table',
    view: 'table',
    title: { 'zh-CN': '玩家列表' },
    autoRun: true,
    table: {
      columns: [
        { key: 'uid', title: { 'zh-CN': 'UID' } },
        { key: 'nickname', title: { 'zh-CN': '昵称' } },
      ],
    },
    toolbar: {
      actions: [
        // openModal 型：targetSection=弹窗 group 名
        { label: { 'zh-CN': '发邮件' }, targetSection: 'mailModal' },
        // 刷新型：chain 内 refreshNode
        {
          label: { 'zh-CN': '刷新列表' },
          chain: [{ kind: 'refreshNode', target: 'playerListTable' }],
        },
      ],
    },
  },
  {
    key: 'mailForm',
    bindingId: 'b-form',
    view: 'form',
    title: { 'zh-CN': '发邮件表单' },
    display: 'dialog',
    group: 'mailModal',
    form: {
      jsonSchema: {
        type: 'object',
        properties: { to: { type: 'string', title: '收件人' } },
      },
    },
  },
];

function renderComposite(onExecute: jest.Mock) {
  return render(
    <App>
      <CompositeRenderer
        sections={sections}
        bindings={[]}
        onExecute={onExecute as never}
        preview={false}
      />
    </App>,
  );
}

describe('CompositeRenderer 表格工具栏按钮', () => {
  it('view=table 区块的 toolbar.actions 在表格卡片头部渲染', async () => {
    const onExecute = jest.fn().mockResolvedValue({
      data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 },
    });
    renderComposite(onExecute);
    await waitFor(() => expect(screen.getByText('bob')).toBeInTheDocument());

    expect(screen.getByRole('button', { name: '发邮件' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '刷新列表' })).toBeInTheDocument();
  });

  it('openModal 型按钮按 group 打开弹窗', async () => {
    const onExecute = jest.fn().mockResolvedValue({
      data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 },
    });
    renderComposite(onExecute);
    await waitFor(() => expect(screen.getByText('bob')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: '发邮件' }));
    // 弹窗标题来自 group 命中的第一个 dialog section
    await waitFor(() => expect(screen.getByText('发邮件表单')).toBeInTheDocument());
  });

  it('刷新型按钮执行 chain 内 refreshNode', async () => {
    const onExecute = jest.fn().mockResolvedValue({
      data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 },
    });
    renderComposite(onExecute);
    // autoRun 首次执行
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-table', expect.anything()));
    expect(onExecute).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: '刷新列表' }));
    await waitFor(() => expect(onExecute).toHaveBeenCalledTimes(2));
  });
});
