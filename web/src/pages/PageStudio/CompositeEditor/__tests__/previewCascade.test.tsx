import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
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

// 重 DOM 套件（PreviewRuntime 全量渲染 + 防抖等待）在 coverage
// instrumentation 负载下撞默认 5s 用例预算（隔离跑恒绿），同法放宽
jest.setTimeout(20000);

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

    // 本文件锁定真实调用路径：预览默认模拟模式，显式切到真实
    fireEvent.click(screen.getByRole('switch')); // 关

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
    fireEvent.click(screen.getByRole('switch')); // 关（真实路径下验证不重跑）
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

  it('fnForm 提交成功后，refreshOnNode 指向它的 fnTable 应自动重跑（值变化级联）', async () => {
    // 复现 bug：Object.keys(results).join(',') 只检测 key 增删，
    // 不检测已有 key 的 value 变化 → fnForm 提交后下游不刷新。
    // 测试策略：手动触发 form 值变化（onFormValues），验证级联是否触发。
    const formNode: PageNode = {
      id: 'form1',
      type: 'fnForm',
      props: {
        functionId: 'player.create',
        title: '创建玩家',
        span: 12,
        autoRun: false,
      },
    };
    const tableNode: PageNode = {
      id: 'tbl1',
      type: 'fnTable',
      props: {
        functionId: 'player.list',
        title: '玩家列表',
        span: 24,
        autoRun: true, // 首次自动执行 → tbl1 key 立即进入 results
        refreshOnNode: ['form1'],
      },
    };
    const createDesc: FunctionDescriptor = {
      id: 'player.create',
      inputSchema: {
        type: 'object',
        properties: { name: { type: 'string' } },
        required: ['name'],
      },
    } as unknown as FunctionDescriptor;
    const listDesc: FunctionDescriptor = {
      id: 'player.list',
      inputSchema: { type: 'object', properties: {} },
    } as unknown as FunctionDescriptor;
    const fnMap = new Map([
      ['player.create', createDesc],
      ['player.list', listDesc],
    ]);

    let listCallCount = 0;
    mockedInvoke.mockImplementation(async (fid: string) => {
      if (fid === 'player.create') return { result: { success: true } };
      listCallCount++;
      return { result: { items: [{ id: listCallCount }] } };
    });

    render(
      <App>
        <PreviewRuntime tree={[formNode, tableNode]} fnById={fnMap} />
      </App>,
    );

    // 切到真实模式 → autoRun=true 的 fnTable 立即执行
    fireEvent.click(screen.getByRole('switch'));

    // 等 fnTable autoRun 完成 → 此时 results 已有 tbl1 key
    await waitFor(() => {
      expect(listCallCount).toBe(1);
    });

    // 模拟 fnForm 值变化（onFormValues 写入 results[form1].values）
    // 这是 fnForm 提交后的效果：results[form1] 从无到有，触发级联
    const nameInput = screen.queryByLabelText(/name/i) || screen.queryByRole('textbox');
    if (nameInput) {
      fireEvent.change(nameInput, { target: { value: 'test' } });
    }

    // fnForm 值变化后，fnTable 应因 refreshOnNode 级联自动重跑
    await waitFor(
      () => {
        expect(listCallCount).toBeGreaterThanOrEqual(2);
      },
      { timeout: 5000 },
    );
  });
});
