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

// 重 DOM 套件（PreviewRuntime 全量渲染）在 coverage instrumentation 负载下
// 撞默认 5s 用例预算（隔离跑恒绿），与 Ops/Jobs 等重 suite 同法放宽
jest.setTimeout(20000);

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
  it('默认即模拟模式：生成假数据且不调用真实函数', async () => {
    renderPreview(tree());
    expect(screen.getByText('数据来源')).toBeInTheDocument();

    // 默认开（安全预览）→ 点执行 → 假数据渲染（radio 选择列出现），真实调用未发生
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
    expect(mockedInvoke).not.toHaveBeenCalled();
  });

  it('关闭模拟后回退真实调用', async () => {
    mockedInvoke.mockResolvedValue({ data: { items: [{ uid: 'real-1' }], total: 1 } });
    renderPreview(tree());

    // 默认模拟开 → 先关，走真实调用路径
    fireEvent.click(screen.getByRole('switch')); // 关
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
  it('无 outputSchema：仍生成兜底假数据（不再提示空态）', async () => {
    mockedInvoke.mockClear();
    const bare = { id: 'player.list' } as unknown as FunctionDescriptor;
    render(
      <App>
        <PreviewRuntime tree={tree()} fnById={new Map([['player.list', bare]])} />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    // 兜底假数据：player.list 匹配 *.list 模式 → 生成 items 数组 → 表格渲染行
    await waitFor(() => {
      expect(document.querySelectorAll('input[type="radio"]').length).toBeGreaterThan(0);
    });
    expect(mockedInvoke).not.toHaveBeenCalled();
  });
});

// BUG-033 回归：内置组件模板（服务端 generator 产物）fnTable 固定 columns: []，
// mock 假数据已生成但渲染消费端取不到/取错——此前三个病灶：
// ① 列回退取 outputSchema 顶层属性（列表形态取到 items/total，items 列裸渲染
//    对象数组崩）；② 行提取只认 payload.items（数组字段名非 items 时永远取不到）；
// ③ 函数未注册时 schemaProperties(undefined)=[] 零列空表。
describe('PreviewRuntime 组件模板内置形态（BUG-033）', () => {
  // 服务端 generator.go 内置模板的 fnTable 形态：columns:[] + autoRun:true
  function bareTableTree(): PageNode[] {
    return [
      {
        id: 'tbl1',
        type: 'fnTable',
        props: {
          functionId: 'player.list',
          title: '玩家列表',
          span: 24,
          autoRun: true,
          columns: [],
          rowActions: [],
        },
      },
    ];
  }

  const listSchemaDescriptor = {
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

  it('内置模板 columns:[] + 列表形态 schema：autoRun 直接渲染假数据行，列为行字段', async () => {
    render(
      <App>
        <PreviewRuntime
          tree={bareTableTree()}
          fnById={new Map([['player.list', listSchemaDescriptor]])}
        />
      </App>,
    );
    // mock 行 0 的 uid = u-1001（mockStringByField 启发式）
    await waitFor(() => {
      expect(screen.getByText('u-1001')).toBeInTheDocument();
    });
    // 列头是行字段 uid/nickname，而非顶层 items/total
    expect(screen.getByText('uid')).toBeInTheDocument();
    expect(screen.getByText('nickname')).toBeInTheDocument();
  });

  it('函数未注册：fallback 行数据可见（此前零列空表）', async () => {
    render(
      <App>
        <PreviewRuntime tree={bareTableTree()} fnById={new Map()} />
      </App>,
    );
    // generateFallbackMockData：player.list 命中 *.list → items 3 行，id=mock-1001…
    await waitFor(() => {
      expect(screen.getByText('mock-1001')).toBeInTheDocument();
    });
  });

  it('数组字段名非 items（如 players）：行提取兜底首个对象数组字段', async () => {
    const playersDescriptor = {
      id: 'player.list',
      outputSchema: {
        type: 'object',
        properties: {
          players: {
            type: 'array',
            items: { type: 'object', properties: { uid: { type: 'string' } } },
          },
        },
      },
    } as unknown as FunctionDescriptor;
    render(
      <App>
        <PreviewRuntime
          tree={bareTableTree()}
          fnById={new Map([['player.list', playersDescriptor]])}
        />
      </App>,
    );
    await waitFor(() => {
      expect(screen.getByText('u-1001')).toBeInTheDocument();
    });
  });

  it('行内含对象字段：列取行字段并把对象值 JSON 序列化渲染（此前取顶层 items 列，整表空）', async () => {
    const nestedSchemaDescriptor = {
      id: 'player.list',
      outputSchema: {
        type: 'object',
        properties: {
          items: {
            type: 'array',
            items: {
              type: 'object',
              properties: {
                uid: { type: 'string' },
                // 行内嵌套对象：rowFieldsOf 把它列为列，单元格走 JSON 序列化
                meta: { type: 'object', properties: { zone: { type: 'string' } } },
              },
            },
          },
        },
      },
    } as unknown as FunctionDescriptor;
    render(
      <App>
        <PreviewRuntime
          tree={bareTableTree()}
          fnById={new Map([['player.list', nestedSchemaDescriptor]])}
        />
      </App>,
    );
    await waitFor(() => {
      expect(screen.getByText('u-1001')).toBeInTheDocument();
    });
    // meta 列对象值 → JSON 字符串（含 zone），不再是 React 非法子节点（3 行各一）
    expect(screen.getAllByText(/"zone"/).length).toBe(3);
  });
});
