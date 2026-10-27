/** B1 预览栏：预览消费与保存同一生成链产出的 PageSpec，经正式 PageRenderer 渲染，
 * 仅替换执行适配器（模拟数据）。
 *
 * 覆盖：预览请求命中 preview 端点且请求体与保存同形（sections 由编译产物直接生成）；
 * spec 经真实渲染器出图（autoRun 区块由模拟执行器返回契约样例数据）；
 * 生成质量与诊断就地展示；请求失败与编译阻断均回退旧 PreviewRuntime 且预览不空白。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import PreviewPane from '../PreviewPane';
import { compileTree } from '../compiler';
import type { PageNode } from '../model';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { Diagnostic, PageSpec } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(async () => {
    throw new Error('real invocation must not happen in preview mock mode');
  }),
  listDescriptors: jest.fn(async () => []),
}));

type RequestOpts = { method?: string; data?: unknown; signal?: AbortSignal };

jest.mock('@umijs/max', () => {
  const R = require('react') as typeof React;
  const formatMessage = (
    descriptor: { defaultMessage?: string },
    values?: Record<string, unknown>,
  ): string =>
    Object.entries(values ?? {}).reduce(
      (msg, [key, val]) => msg.split(`{${key}}`).join(String(val)),
      descriptor.defaultMessage ?? '',
    );
  const req = jest.fn(async () => ({}));
  const FormattedMessage = ({ defaultMessage }: { defaultMessage?: React.ReactNode }) =>
    defaultMessage ? R.createElement('span', null, defaultMessage) : null;
  return {
    __esModule: true,
    history: { push: jest.fn() },
    request: req,
    // actions.ts 在模块顶层 getIntl()，useIntl 供组件内使用
    getIntl: () => ({ formatMessage }),
    useIntl: () => ({ formatMessage }),
    FormattedMessage,
    useSearchParams: () => [new URLSearchParams(), jest.fn()],
  };
});

import { request as mockRequestImpl } from '@umijs/max';

const mockedRequest = mockRequestImpl as unknown as jest.Mock<
  (url: string, opts?: RequestOpts) => Promise<unknown>
>;

jest.setTimeout(20000);

const orderListDescriptor: FunctionDescriptor = {
  id: 'order.list',
  outputSchema: {
    type: 'object',
    properties: {
      items: {
        type: 'array',
        items: {
          type: 'object',
          properties: {
            orderId: { type: 'string' },
            amount: { type: 'integer' },
          },
        },
      },
      total: { type: 'integer' },
    },
  },
} as unknown as FunctionDescriptor;

function tree(): PageNode[] {
  return [
    {
      id: 't1',
      type: 'fnFields',
      props: { functionId: 'order.list', title: '订单详情', span: 24, autoRun: true },
    },
  ];
}

function responseOf(
  nodes: PageNode[],
  extra: { quality?: string; diagnostics?: Diagnostic[] } = {},
  view: 'fields' | 'form' = 'fields',
) {
  const sections = compileTree(nodes).sections;
  const spec = {
    pageKey: 'composite--preview',
    type: 'composite',
    title: { 'zh-CN': '预览页' },
    category: { key: 'order' },
    composite: {
      sections: sections.map((s) => ({
        key: s.key,
        bindingId: `b-${s.key}`,
        title: { 'zh-CN': String(s.title ?? '') },
        view,
        autoRun: true,
      })),
    },
    bindings: sections.map((s) => ({
      id: `b-${s.key}`,
      functionId: s.functionId,
      usage: 'detail' as const,
      execution: { mode: 'sync' as const },
    })),
    ...extra,
  } as unknown as PageSpec & { quality?: string; diagnostics?: Diagnostic[] };
  return { pageKey: 'composite--preview', pageSpec: spec };
}

function renderPane(opts?: {
  templates?: Array<{ key: string; digest: string }>;
  pageKey?: string;
}) {
  return render(
    <App>
      <PreviewPane
        tree={tree()}
        fnById={new Map([['order.list', orderListDescriptor]])}
        pageKey={opts?.pageKey ?? 'composite--preview'}
        componentTemplates={opts?.templates}
      />
    </App>,
  );
}

describe('PreviewPane（B1 预览共用运行时）', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
    mockedRequest.mockResolvedValue(undefined);
  });

  it('请求命中 preview 端点，请求体与保存同形且透传模板快照', async () => {
    mockedRequest.mockResolvedValue(responseOf(tree()));
    renderPane({ templates: [{ key: 'tpl--card', digest: 'd1' }] });

    await waitFor(() => expect(screen.getByText('页面规格预览')).toBeInTheDocument());

    const call = mockedRequest.mock.calls[0] as [string, { method?: string; data?: unknown }];
    expect(call[0]).toBe('/api/v1/versioning/pages/composite/preview');
    expect(call[1].method).toBe('POST');
    const data = call[1].data as {
      pageKey: string;
      sections: unknown[];
      componentTemplates?: unknown[];
    };
    expect(data.pageKey).toBe('composite--preview');
    expect(data.sections).toHaveLength(1);
    expect(data.componentTemplates).toEqual([{ key: 'tpl--card', digest: 'd1' }]);

    // 预览消费的是服务端产出的 spec：sections 直接取自编译产物，无客户端再拼装
    expect((data.sections[0] as { functionId: string }).functionId).toBe('order.list');
  });

  it('spec 经正式渲染器出图：autoRun 区块由模拟执行器返回契约样例数据', async () => {
    mockedRequest.mockResolvedValue(responseOf(tree()));
    renderPane();

    // fields 区块把结果对象的键作为 Descriptions 标签渲染——
    // items/total 来自模拟执行器按 outputSchema 合成的数据，证明执行走的是真实渲染链路
    await waitFor(() => expect(screen.getAllByText('items').length).toBeGreaterThan(0));
    expect(screen.getAllByText('total').length).toBeGreaterThan(0);
    expect(screen.getByText('页面规格预览')).toBeInTheDocument();
  });

  it('生成质量与诊断就地展示（质量码本地化，不直接展示英文机器码）', async () => {
    mockedRequest.mockResolvedValue(
      responseOf(tree(), {
        quality: 'needs_review',
        diagnostics: [
          { code: 'selector_warning', severity: 'warning', message: '参数映射未覆盖必填项' },
        ],
      }),
    );
    renderPane();

    expect(await screen.findByText('生成诊断')).toBeInTheDocument();
    expect(screen.getByText('参数映射未覆盖必填项')).toBeInTheDocument();
    expect(screen.getByText('需复核')).toBeInTheDocument();
    expect(screen.queryByText('needs_review')).not.toBeInTheDocument();
  });

  it('预览请求失败：回退旧 PreviewRuntime，并给出可读错误', async () => {
    mockedRequest.mockRejectedValue({ data: { message: 'contract not found: order.list' } });
    renderPane();

    expect(await screen.findByText(/contract not found/)).toBeInTheDocument();
    // 阻断分支不展示生成规格标题
    expect(screen.queryByText('页面规格预览')).not.toBeInTheDocument();
  });

  it('服务端结构化拒绝：details 的字段级原因内联列出（可定位到区块）', async () => {
    mockedRequest.mockRejectedValue({
      data: {
        error: 'validation_failed',
        message: 'function no.such.fn contract not found',
        details: { 'sections[1].functionId': 'function no.such.fn contract not found' },
      },
    });
    renderPane();

    expect(await screen.findByText('sections[1].functionId')).toBeInTheDocument();
    expect(screen.getAllByText(/contract not found/).length).toBeGreaterThan(0);
    // 阻断分支不展示生成规格标题
    expect(screen.queryByText('页面规格预览')).not.toBeInTheDocument();
  });

  it('编译阻断：回退画布结构预览并列出阻断项', async () => {
    mockedRequest.mockResolvedValue(responseOf(tree()));
    render(
      <App>
        <PreviewPane
          tree={[{ id: 'tx', type: 'text', props: { content: '说明文本' } }]}
          fnById={new Map([['order.list', orderListDescriptor]])}
          pageKey="composite--preview"
        />
      </App>,
    );

    expect(await screen.findByText(/回退到画布结构预览/)).toBeInTheDocument();
    expect(screen.getAllByText(/说明文本/).length).toBeGreaterThan(0);
    // 阻断时不得发起预览请求
    expect(mockedRequest).not.toHaveBeenCalled();
  });

  it('页面 Key 为空：回退并提示填写 Key', async () => {
    mockedRequest.mockResolvedValue(responseOf(tree()));
    renderPane({ pageKey: '   ' });

    expect(await screen.findByText('请填写页面 Key')).toBeInTheDocument();
    expect(mockedRequest).not.toHaveBeenCalled();
  });

  it('真实预览：经服务端校验的草稿执行入口执行，请求体与预览端点同形', async () => {
    const executeUrl = '/api/v1/versioning/pages/composite/preview/execute';
    mockedRequest.mockImplementation(async (url: string) => {
      if (String(url).endsWith('/composite/preview')) return responseOf(tree());
      if (String(url) === executeUrl) {
        return { result: { kind: 'sync', requestId: 'req-1', data: { orderId: 'A-1' } } };
      }
      throw new Error(`unexpected request: ${url}`);
    });
    renderPane();

    await waitFor(() => expect(screen.getByText('页面规格预览')).toBeInTheDocument());
    // 默认模拟模式：只发预览请求，不发执行请求
    expect(mockedRequest.mock.calls.filter((c) => String(c[0]) === executeUrl)).toHaveLength(0);

    fireEvent.click(screen.getByRole('switch'));

    // 真实结果渲染出图；模拟数据的 items/total 被替换（同一渲染器、仅换执行适配器）
    expect(await screen.findByText('orderId')).toBeInTheDocument();
    expect(screen.queryByText('items')).not.toBeInTheDocument();

    const call = mockedRequest.mock.calls.find((c) => String(c[0]) === executeUrl) as [
      string,
      {
        method?: string;
        data?: {
          pageKey: string;
          bindingId: string;
          sections: Array<{ functionId: string }>;
          context: unknown;
        };
      },
    ];
    expect(call).toBeTruthy();
    expect(call[1].method).toBe('POST');
    expect(call[1].data?.pageKey).toBe('composite--preview');
    expect(call[1].data?.bindingId).toBe('b-order.list');
    expect(call[1].data?.sections).toHaveLength(1);
    expect(call[1].data?.sections[0].functionId).toBe('order.list');
    // 浏览器不提交 functionId / route / scope；上下文按契约 selector 投影
    expect(call[1].data?.context).toEqual({});
    expect(Object.keys(call[1].data as Record<string, unknown>)).not.toEqual(
      expect.arrayContaining(['functionId', 'route', 'scope']),
    );
  });

  it('真实预览失败：区块内渲染失败反馈（草稿发布级校验的可读诊断）', async () => {
    // form 视图（无表单规格 → 执行按钮形态）渲染执行反馈条
    mockedRequest.mockImplementation(async (url: string) => {
      if (String(url).endsWith('/composite/preview')) return responseOf(tree(), {}, 'form');
      throw { data: { message: 'draft page fails publish validation' } };
    });
    renderPane();

    await waitFor(() => expect(screen.getByText('页面规格预览')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('switch'));

    // 首次 autoRun 已被服务端拒绝（卡片仍在加载，按钮待重绘）；提交出口承载
    // 区块内失败反馈（字段级定位 + 可读信息）
    const execButton = await screen.findByRole('button', { name: /订单详情/ });
    fireEvent.click(execButton);

    expect(await screen.findByText(/执行失败/)).toBeInTheDocument();
    expect(screen.getByText(/draft page fails publish validation/)).toBeInTheDocument();
  });
});
