/** 批次 C 回归锁定：预览运行时对齐发布运行时的关键数据流。
 * - A1 真实响应归一：invokeFunction 返回 {result}（FunctionInvokeResponse 契约），
 *   预览必须归一为 {data} 页面状态，{{var.data.x}} 才能求值；
 * - A2 行操作 {{row.x}} 表达式原文 → 行字段取值（slice 边界）；
 * - A3 inputAssignments 显式映射求值（page_state 走页面状态 + 字段路径）；
 * - A5 弹窗提交失败保持打开 / 成功关闭；
 * - A6 动作链步骤按 kind 分派（showMessage 主动作 + refreshNode 链步骤）；
 * - A7 表格行点击（onRowClick）/字段卡点击（onClick）事件；
 * - A8 行操作链在打开弹窗后执行（row 作上下文）；
 * - A9 表单当前值写入 values 运行时状态（{{var.values.x}} 求值）。 */
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

const mockedInvoke = invokeFunction as unknown as jest.Mock;

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
        total: { type: 'number' },
      },
    },
  },
  {
    id: 'player.search',
    inputSchema: {
      type: 'object',
      properties: { kw: { type: 'string', title: '关键词' } },
    },
  },
  {
    id: 'player.get',
    inputSchema: {
      type: 'object',
      properties: { uid: { type: 'string', title: '玩家' } },
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

/** 弹窗（行操作/行点击目标）。 */
const mailModal: PageNode = {
  id: 'm1',
  type: 'modal',
  props: { title: '发邮件弹窗', width: 'medium' },
  children: [
    {
      id: 'mailForm1',
      type: 'fnForm',
      props: { functionId: 'mail.send', title: '发邮件', sectionKey: 'mailSendForm' },
    },
  ],
};

/** 自动执行的表格（sectionKey=表达式变量空间入口）。 */
function tableNode(extra: Record<string, unknown> = {}): PageNode {
  return {
    id: 'tbl1',
    type: 'fnTable',
    props: {
      functionId: 'player.list',
      title: '玩家列表',
      sectionKey: 'playerListTable',
      span: 24,
      autoRun: true,
      columns: ['uid', 'nickname'],
      ...extra,
    },
  };
}

/** 详情字段卡（runBinding 目标）。 */
function fieldsNode(extra: Record<string, unknown> = {}): PageNode {
  return {
    id: 'f2',
    type: 'fnFields',
    props: {
      functionId: 'player.get',
      title: '玩家详情',
      sectionKey: 'playerDetail',
      span: 24,
      ...extra,
    },
  };
}

function buttonNode(title: string, onClick: Record<string, unknown>): PageNode {
  return { id: `btn-${title}`, type: 'button', props: { title, onClick } };
}

function renderPreview(nodes: PageNode[]) {
  return render(
    <App>
      <PreviewRuntime tree={nodes} fnById={new Map(descriptors.map((d) => [d.id, d]))} />
    </App>,
  );
}

/** 默认 mock：player.list 返回两行 + total=2（真实响应 {result} 形态）。 */
function mockListOk(extra: Record<string, unknown> = {}) {
  mockedInvoke.mockImplementation(async (fid: string) => {
    if (fid === 'mail.send') return { result: { ok: true } };
    return {
      result: {
        items: [
          { uid: 'u1', nickname: 'alice' },
          { uid: 'u2', nickname: 'bob' },
        ],
        total: 2,
      },
      ...extra,
    };
  });
}

describe('预览对齐发布运行时（批次 C）', () => {
  beforeEach(() => {
    mockedInvoke.mockReset();
    mockListOk();
  });

  it('A1：真实响应 {result} 归一为 {data} 页面状态，{{var.data.total}} 可求值', async () => {
    renderPreview([
      tableNode(),
      fieldsNode(),
      buttonNode('查详情', {
        kind: 'runBinding',
        target: 'f2',
        params: { uid: '{{playerListTable.data.total}}' },
      }),
    ]);
    // autoRun 表格渲染出行数据
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /查详情/ }));
    // total=2（数字）从归一后的 data 求值——修复前存原始响应致恒 undefined
    await waitFor(() =>
      expect(mockedInvoke).toHaveBeenCalledWith('player.get', expect.objectContaining({ uid: 2 })),
    );
  });

  it('A2+A8：行操作 {{row.uid}} 预填弹窗 + 链内 refreshNode 重跑表格', async () => {
    renderPreview([
      tableNode({
        rowActions: [
          {
            label: '发邮件',
            targetSection: 'm1',
            params: { playerId: '{{row.uid}}' },
            chain: [{ kind: 'refreshNode', target: 'tbl1' }],
          },
        ],
      }),
      mailModal,
    ]);
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());
    expect(mockedInvoke).toHaveBeenCalledTimes(1); // 仅 autoRun

    fireEvent.click(screen.getAllByText('发邮件')[0]);
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    // {{row.uid}} 表达式原文 → 第一行 uid（slice 边界修复）
    await waitFor(() => {
      const input = document.querySelector('input[id*="playerId"]') as HTMLInputElement | null;
      expect(input?.value).toBe('u1');
    });
    // 行操作链在打开弹窗后执行 refreshNode
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(2));
    expect(mockedInvoke.mock.calls[1][0]).toBe('player.list');
  });

  it('A3：inputAssignments（page_state）按 selectedRow/uid 路径求值', async () => {
    renderPreview([
      tableNode(),
      fieldsNode({
        inputAssignments: [
          { param: 'uid', kind: 'page_state', sourceNodeId: 'tbl1', field: 'selectedRow/uid' },
        ],
      }),
    ]);
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());

    // 选中第一行（同步写页面状态）→ 执行字段卡
    fireEvent.click(document.querySelectorAll('input[type="radio"]')[0]);
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() =>
      expect(mockedInvoke).toHaveBeenCalledWith(
        'player.get',
        expect.objectContaining({ uid: 'u1' }),
      ),
    );
  });

  it('A5：弹窗提交失败保持打开（可重试），成功才关闭', async () => {
    renderPreview([
      tableNode({
        rowActions: [{ label: '发邮件', targetSection: 'm1', params: { playerId: '{{row.uid}}' } }],
      }),
      mailModal,
    ]);
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());

    // 失败路径：mail.send 拒绝 → 弹窗不关
    mockedInvoke.mockImplementation(async (fid: string) => {
      if (fid === 'mail.send') throw new Error('agent 离线');
      return {
        result: {
          items: [
            { uid: 'u1', nickname: 'alice' },
            { uid: 'u2', nickname: 'bob' },
          ],
          total: 2,
        },
      };
    });
    fireEvent.click(screen.getAllByText('发邮件')[0]);
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledWith('mail.send', expect.anything()));
    expect(screen.getByText('发邮件弹窗')).toBeInTheDocument(); // 仍打开

    // 成功路径：恢复可解析响应 → 提交后弹窗关闭
    mockListOk();
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() => expect(screen.queryByText('发邮件弹窗')).toBeNull());
  });

  it('A6：showMessage 主动作 + refreshNode 链步骤都执行（链按 kind 分派）', async () => {
    renderPreview([
      tableNode(),
      buttonNode('提示并刷新', {
        kind: 'showMessage',
        target: '',
        params: { message: '操作成功' },
        chain: [{ kind: 'refreshNode', target: 'tbl1' }],
      }),
    ]);
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());
    expect(mockedInvoke).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /提示并刷新/ }));
    await waitFor(() => expect(screen.getByText('操作成功')).toBeInTheDocument());
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(2));
  });

  it('A7：表格行点击触发 onRowClick（row 作上下文打开弹窗）', async () => {
    renderPreview([tableNode({ onRowClick: { kind: 'openModal', target: 'm1' } }), mailModal]);
    await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument());

    fireEvent.click(screen.getByText('alice'));
    await waitFor(() => expect(screen.getByText('发邮件弹窗')).toBeInTheDocument());
  });

  it('A9：表单当前值写入 values 状态，{{var.values.kw}} 可求值', async () => {
    renderPreview([
      {
        id: 'sf1',
        type: 'fnForm',
        props: {
          functionId: 'player.search',
          title: '搜索',
          sectionKey: 'searchForm',
          span: 24,
        },
      },
      fieldsNode(),
      buttonNode('按值查详情', {
        kind: 'runBinding',
        target: 'f2',
        params: { uid: '{{searchForm.values.kw}}' },
      }),
    ]);

    fireEvent.change(screen.getByLabelText(/关键词/), { target: { value: 'abc' } });
    fireEvent.click(screen.getByRole('button', { name: /按值查详情/ }));
    await waitFor(() =>
      expect(mockedInvoke).toHaveBeenCalledWith(
        'player.get',
        expect.objectContaining({ uid: 'abc' }),
      ),
    );
  });
});
