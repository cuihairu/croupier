import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { jest } from '@jest/globals';
import PreviewRuntime from '../PreviewRuntime';
import { invokeFunction } from '@/services/api/functions';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(async () => ({ data: { items: [] } })),
  listDescriptors: jest.fn(async () => []),
}));

const mockedInvoke = invokeFunction as unknown as jest.Mock;

function tree(): PageNode[] {
  return [
    {
      id: 'sf1',
      type: 'staticForm',
      props: {
        title: '过滤条件',
        span: 12,
        staticSchema: JSON.stringify({
          type: 'object',
          properties: { keyword: { type: 'string', title: '关键词' } },
        }),
      },
    },
    {
      id: 'tbl1',
      type: 'fnTable',
      props: {
        functionId: 'player.list',
        title: '玩家列表',
        span: 24,
        autoRun: false,
        refreshOnNode: ['sf1'],
      },
    },
  ];
}

const playerListDescriptor: FunctionDescriptor = {
  id: 'player.list',
  inputSchema: {
    type: 'object',
    properties: { keyword: { type: 'string', title: '关键词' } },
  },
} as unknown as FunctionDescriptor;

describe('PreviewRuntime refreshOnNode 级联（对齐发布运行时）', () => {
  beforeEach(() => {
    mockedInvoke.mockClear();
  });

  it('staticForm 值变化 → refreshOnNode 指向的 fnTable 自动重跑并带入同名字段', async () => {
    render(
      <App>
        <PreviewRuntime tree={tree()} fnById={new Map([['player.list', playerListDescriptor]])} />
      </App>,
    );

    // autoRun=false：初始不执行
    expect(mockedInvoke).not.toHaveBeenCalled();

    // 常量表单输入 → 防抖后并入 results → 级联重跑 fnTable
    fireEvent.change(screen.getByLabelText(/关键词/), { target: { value: '张三' } });

    await waitFor(
      () => {
        expect(mockedInvoke).toHaveBeenCalledWith(
          'player.list',
          expect.objectContaining({ keyword: '张三' }),
        );
      },
      { timeout: 2000 },
    );
  });

  it('无 refreshOnNode 的节点不因 staticForm 变化而重跑', async () => {
    const nodes: PageNode[] = [
      tree()[0],
      {
        id: 'tbl2',
        type: 'fnTable',
        props: { functionId: 'player.list', title: '无联动表格', span: 24, autoRun: false },
      },
    ];
    render(
      <App>
        <PreviewRuntime tree={nodes} fnById={new Map([['player.list', playerListDescriptor]])} />
      </App>,
    );
    fireEvent.change(screen.getByLabelText(/关键词/), { target: { value: '李四' } });
    await waitFor(
      () => {
        expect(screen.getByText('无联动表格')).toBeInTheDocument();
      },
      { timeout: 1000 },
    );
    // 等过防抖窗口后仍不应有调用
    await new Promise((r) => setTimeout(r, 600));
    expect(mockedInvoke).not.toHaveBeenCalled();
  });
});
