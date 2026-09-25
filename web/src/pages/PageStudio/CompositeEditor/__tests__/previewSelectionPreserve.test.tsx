/** PreviewRuntime 选中态跨重跑保留（对齐发布端表格选中不随刷新丢失）：
 * mock / 真实两条响应分支各自维护 selectedRow/selectedRows，
 * 重跑后 selectedRow 仍可被 {{var.selectedRow.uid}} 表达式取到。
 * 覆盖 PreviewRuntime.tsx L184-194（模拟分支）与 L203-215（真实分支）。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { jest } from '@jest/globals';
import PreviewRuntime from '../PreviewRuntime';
import { invokeFunction } from '@/services/api/functions';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(async () => ({
    result: {
      items: [
        { uid: 'u-1001', nickname: '甲' },
        { uid: 'u-1002', nickname: '乙' },
      ],
    },
  })),
  listDescriptors: jest.fn(async () => []),
}));

jest.setTimeout(30000);

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
      properties: { playerId: { type: 'string', title: '玩家' } },
    },
  },
] as unknown as FunctionDescriptor[];

function tree(): PageNode[] {
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
      onRowSelected: {
        kind: 'openModal',
        target: 'modal1',
        params: { playerId: '{{playerListTable.selectedRow.uid}}' },
      },
    },
  };
  // 重跑后读取选中态的观察点：表达式命中 → 预填 u-1001
  const probe: PageNode = {
    id: 'btn1',
    type: 'button',
    props: {
      title: '打开弹窗',
      onClick: {
        kind: 'openModal',
        target: 'modal1',
        params: { playerId: '{{playerListTable.selectedRow.uid}}' },
      },
    },
  };
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
  const modal: PageNode = {
    id: 'modal1',
    type: 'modal',
    props: { title: '发邮件弹窗', width: 'medium' },
    children: [form],
  };
  return [table, probe, modal];
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

async function runTable(): Promise<void> {
  fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
  await waitFor(
    () => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    },
    { timeout: 5000 },
  );
}

async function selectFirstRow(): Promise<void> {
  fireEvent.click(document.querySelectorAll('input[type="radio"]')[0]);
  await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
  await waitFor(() => expect(playerIdValue()).toBe('u-1001'));
}

function closeModal(): void {
  const closeBtn = document.querySelector('.ant-modal-close') as HTMLButtonElement | null;
  expect(closeBtn).not.toBeNull();
  fireEvent.click(closeBtn!);
}

async function expectSelectionProbe(): Promise<void> {
  fireEvent.click(screen.getByRole('button', { name: /打开弹窗/ }));
  await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
  await waitFor(() => expect(playerIdValue()).toBe('u-1001'));
}

describe('PreviewRuntime 选中态跨重跑保留', () => {
  const mockedInvoke = invokeFunction as unknown as jest.Mock;

  beforeEach(() => {
    mockedInvoke.mockClear();
  });

  it('模拟模式：选中 → 重跑 → selectedRow 仍可被表达式取到', async () => {
    renderPreview();
    // 未选中重跑：prev 存在但无选中态 → 不进入保留分支
    await runTable();
    await runTable();
    await selectFirstRow();
    closeModal();

    await runTable();
    await expectSelectionProbe();
  });

  it('真实模式：选中 → 重跑 → selectedRow 仍可被表达式取到', async () => {
    renderPreview();
    fireEvent.click(screen.getByRole('switch')); // 关模拟 → 真实调用
    await waitFor(() => expect(screen.getByText('真实调用')).toBeInTheDocument());

    await runTable();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(1));
    await selectFirstRow();
    closeModal();

    await runTable();
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(2));
    await expectSelectionProbe();
  });
});
