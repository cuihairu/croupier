/** 预览模拟数据模式：开启时不调用真实函数，按 outputSchema 渲染假数据。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { jest } from '@jest/globals';
import PreviewRuntime from '../PreviewRuntime';
import { invokeFunction } from '@/services/api/functions';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(async () => {
    throw new Error('real invocation should not happen in mock mode');
  }),
  listDescriptors: jest.fn(async () => []),
}));

const mockedInvoke = invokeFunction as unknown as jest.Mock;

const playerListDescriptor: FunctionDescriptor = {
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
      total: { type: 'integer' },
    },
  },
} as unknown as FunctionDescriptor;

function tree(): PageNode[] {
  return [
    {
      id: 'tbl1',
      type: 'fnTable',
      props: {
        functionId: 'player.list',
        title: '玩家列表',
        span: 24,
        autoRun: false,
        columns: ['uid'],
      },
    },
  ];
}

function renderPreview(nodes: PageNode[]) {
  return render(
    <App>
      <PreviewRuntime tree={nodes} fnById={new Map([['player.list', playerListDescriptor]])} />
    </App>,
  );
}

describe('PreviewRuntime 模拟数据模式', () => {
  it('开启模拟：生成假数据且不调用真实函数', async () => {
    renderPreview(tree());
    expect(screen.getByText('数据来源')).toBeInTheDocument();

    // 打开模拟开关 → 点执行 → 假数据渲染（radio 选择列出现），真实调用未发生
    fireEvent.click(screen.getByRole('switch'));
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
    expect(mockedInvoke).not.toHaveBeenCalled();
  });

  it('关闭模拟后回退真实调用', async () => {
    mockedInvoke.mockResolvedValue({ data: { items: [{ uid: 'real-1' }], total: 1 } });
    renderPreview(tree());

    // 真实调用路径（模拟关）
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(screen.getByText('real-1')).toBeInTheDocument());
    expect(mockedInvoke).toHaveBeenCalledTimes(1);

    // 开 → 模拟执行（假数据替换，真实调用不增加）
    fireEvent.click(screen.getByRole('switch')); // 开
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
    expect(mockedInvoke).toHaveBeenCalledTimes(1);

    // 关 → 回真实调用
    fireEvent.click(screen.getByRole('switch')); // 关
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(mockedInvoke).toHaveBeenCalledTimes(2));
  });
});

describe('PreviewRuntime 模拟数据空态提示', () => {
  it('无 outputSchema：模拟执行提示「模拟数据为空」且每节点只提示一次', async () => {
    const bare = { id: 'player.list' } as unknown as FunctionDescriptor;
    render(
      <App>
        <PreviewRuntime tree={tree()} fnById={new Map([['player.list', bare]])} />
      </App>,
    );
    fireEvent.click(screen.getByRole('switch'));
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => {
      expect(screen.getByText(/「玩家列表」无可用 outputSchema.*模拟数据为空/)).toBeInTheDocument();
    });
    // 再次执行：同节点不重复弹（mockWarnedRef 每节点一次）
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /执\s*行/ })).toBeEnabled();
    });
    expect(screen.getAllByText(/模拟数据为空/)).toHaveLength(1);
  });
});
