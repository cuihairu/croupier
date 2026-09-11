/** V5 预览动作数据流：行操作 → 弹窗预填、选中事件 → 表达式参数求值。
 * 场景对齐设计 §11 验收（选中行 → 发邮件 → 弹窗预填）的预览等价物。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { jest } from '@jest/globals';
import PreviewRuntime from '../PreviewRuntime';
import { invokeFunction } from '@/services/api/functions';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(async () => ({ result: { items: [] } })),
  listDescriptors: jest.fn(async () => []),
}));

const descriptors: FunctionDescriptor[] = [
  {
    id: 'player.list',
    outputSchema: {
      type: 'object',
      properties: {
        items: {
          type: 'array',
          items: {
            type: 'object',
            properties: { uid: { type: 'string' }, nickname: { type: 'string' } },
          },
        },
      },
    },
  },
  {
    id: 'mail.send',
    inputSchema: {
      type: 'object',
      properties: {
        playerId: { type: 'string', title: '玩家' },
        title: { type: 'string', title: '标题' },
      },
    },
  },
] as unknown as FunctionDescriptor[];

function tree(): PageNode[] {
  const form: PageNode = {
    id: 'form1',
    type: 'fnForm',
    props: {
      functionId: 'mail.send',
      title: '发邮件',
      sectionKey: 'mailSendForm',
      display: 'dialog',
    },
  };
  const table: PageNode = {
    id: 'tbl1',
    type: 'fnTable',
    props: {
      functionId: 'player.list',
      title: '玩家列表',
      sectionKey: 'playerListTable',
      span: 24,
      autoRun: false,
      columns: ['uid', 'nickname'],
      rowActions: [
        {
          label: '发邮件',
          targetSection: 'modal1',
          params: { playerId: 'row.uid', title: '{{row.nickname}} 的邮件' },
        },
      ],
      // 选中事件：表达式参数求值（变量空间 = sectionKey → 运行时状态）
      onRowSelected: {
        kind: 'openModal',
        target: 'modal1',
        params: { playerId: '{{playerListTable.selectedRow.uid}}' },
      },
    },
  };
  const modal: PageNode = {
    id: 'modal1',
    type: 'modal',
    props: { title: '发邮件弹窗', width: 'medium' },
    children: [form],
  };
  // 无参 openModal 按钮：打开同一弹窗但不得残留任何预填
  const plainButton: PageNode = {
    id: 'btn1',
    type: 'button',
    props: { title: '打开弹窗', onClick: { kind: 'openModal', target: 'modal1' } },
  };
  return [table, plainButton, modal];
}

function playerIdValue(): string {
  const input = document.querySelector('input[id*="playerId"]') as HTMLInputElement | null;
  return input?.value ?? '';
}

function renderPreview() {
  const fnMap = new Map(descriptors.map((d) => [d.id, d]));
  return render(
    <App>
      <PreviewRuntime tree={tree()} fnById={fnMap} />
    </App>,
  );
}

const mockedInvoke = invokeFunction as unknown as jest.Mock;

async function executeTableWithMockRows() {
  // 预览默认即模拟模式（无需切开关）——mock 行 uid 固定 u-1001 起步
  fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
  await waitFor(() => {
    expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
  });
}

describe('预览 V5 动作数据流', () => {
  beforeEach(() => {
    mockedInvoke.mockClear();
  });

  it('行操作：行字段映射 → 弹窗表单预填（row.uid / {{row.nickname}}）', async () => {
    renderPreview();
    await executeTableWithMockRows();

    // 点击第一行的「发邮件」
    fireEvent.click(screen.getAllByText('发邮件')[0]);

    // 弹窗打开且 playerId 预填为第一行 mock uid（u-1001）
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    await waitFor(() => {
      const input = document.querySelector('input[id*="playerId"]') as HTMLInputElement | null;
      expect(input?.value).toBe('u-1001');
    });
  });

  it('无参动作打开同一弹窗：不残留上一次的预填（替换语义）', async () => {
    renderPreview();
    await executeTableWithMockRows();

    // 行操作预填 playerId=u-1001
    fireEvent.click(screen.getAllByText('发邮件')[0]);
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    await waitFor(() => expect(playerIdValue()).toBe('u-1001'));

    // 关闭弹窗（右上角 X）→ 无参按钮再次打开 → 输入应为空
    const closeBtn = document.querySelector('.ant-modal-close') as HTMLButtonElement | null;
    expect(closeBtn).not.toBeNull();
    fireEvent.click(closeBtn!);
    fireEvent.click(screen.getByRole('button', { name: /打开弹窗/ }));
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    expect(playerIdValue()).toBe('');
  });

  it('选中事件：{{var.selectedRow.uid}} 表达式参数求值 → 弹窗预填', async () => {
    renderPreview();
    await executeTableWithMockRows();

    // 选中第一行 → onRowSelected 动作 → 弹窗预填 selectedRow.uid
    fireEvent.click(document.querySelectorAll('input[type="radio"]')[0]);
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    await waitFor(() => {
      const input = document.querySelector('input[id*="playerId"]') as HTMLInputElement | null;
      expect(input?.value).toBe('u-1001');
    });
  });
});
