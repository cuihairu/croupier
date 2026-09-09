/** V5 T5.3 验收：选中行 → 动作参数取值（CompositeRenderer 组件级联动）。 */
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
    events: [
      {
        event: 'rowSelected',
        action: {
          kind: 'runBinding',
          target: 'mailForm',
          params: { nickname: '{{playerListTable.selectedRow.nickname}}' },
        },
      },
    ],
  },
  {
    key: 'mailForm',
    bindingId: 'b-form',
    view: 'form',
    title: { 'zh-CN': '发邮件' },
    form: {
      jsonSchema: {
        type: 'object',
        properties: { nickname: { type: 'string', title: '昵称' } },
      },
    },
  },
];

function renderComposite(onExecute: jest.Mock) {
  const utils = render(
    <App>
      <CompositeRenderer
        sections={sections}
        bindings={[]}
        onExecute={onExecute as never}
        preview={false}
      />
    </App>,
  );
  return utils;
}

describe('CompositeRenderer 选中行联动（V5）', () => {
  it('选中表格行后，rowSelected 事件的 {{var.selectedRow.x}} 参数正确取值', async () => {
    const onExecute = jest.fn().mockResolvedValue({
      data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 },
    });
    renderComposite(onExecute);

    // autoRun：表格加载即执行
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-table', expect.anything()));
    await waitFor(() => expect(screen.getByText('bob')).toBeInTheDocument());

    // 选中第一行（radio）
    const radios = document.querySelectorAll('input[type="radio"]');
    expect(radios.length).toBeGreaterThan(0);
    fireEvent.click(radios[0]);

    // rowSelected → runBinding mailForm，nickname 取自 selectedRow
    await waitFor(() =>
      expect(onExecute).toHaveBeenCalledWith(
        'b-form',
        expect.objectContaining({ form: { nickname: 'bob' } }),
      ),
    );
  });
});
