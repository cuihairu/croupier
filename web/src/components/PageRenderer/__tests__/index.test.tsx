/** PageRenderer 统一入口（类型路由器）覆盖。
 *
 * 覆盖路径：五种页面类型分发到对应子渲染器（含各类型缺配置的错误态）、
 * 未知类型兜底、executeWithPageState 的输出选择器落 page_state 与
 * 下一次执行的注入、onPageStateMerge 的 merge/replace 两模式、
 * pageKey 切换时重置 page_state。子渲染器 mock 为标识 stub 以聚焦路由编排。 */
import React from 'react';
import { act, render, screen } from '@testing-library/react';
import PageRenderer from '../index';
import { CompositeRenderer } from '../CompositeRenderer';
import ResourcePageRenderer from '../ResourcePageRenderer';
import OperationPageRenderer from '../OperationPageRenderer';
import TaskPageRenderer from '../TaskPageRenderer';
import ReportPageRenderer from '../ReportPageRenderer';
import type {
  CompositePageSpec,
  CompositeSection,
  FormPresentationSpec,
  PageExecutionResult,
  PageFunctionBinding,
  PageSpec,
  TaskPageSpec,
} from '@/types/dashboard';

jest.mock('../CompositeRenderer', () => ({
  CompositeRenderer: jest.fn(() => <div data-testid="stub-composite" />),
}));
jest.mock('../ResourcePageRenderer', () => ({
  __esModule: true,
  default: jest.fn(() => <div data-testid="stub-resource" />),
}));
jest.mock('../OperationPageRenderer', () => ({
  __esModule: true,
  default: jest.fn(() => <div data-testid="stub-operation" />),
}));
jest.mock('../TaskPageRenderer', () => ({
  __esModule: true,
  default: jest.fn(() => <div data-testid="stub-task" />),
}));
jest.mock('../ReportPageRenderer', () => ({
  __esModule: true,
  default: jest.fn(() => <div data-testid="stub-report" />),
}));

const mockedComposite = CompositeRenderer as jest.MockedFunction<typeof CompositeRenderer>;
const mockedResource = ResourcePageRenderer as jest.MockedFunction<typeof ResourcePageRenderer>;
const mockedOperation = OperationPageRenderer as jest.MockedFunction<typeof OperationPageRenderer>;
const mockedTask = TaskPageRenderer as jest.MockedFunction<typeof TaskPageRenderer>;
const mockedReport = ReportPageRenderer as jest.MockedFunction<typeof ReportPageRenderer>;

const form: FormPresentationSpec = { fields: [] };
const taskSpec: TaskPageSpec = {
  form,
  taskView: {
    taskIdStateKey: 'taskId',
    statusBindingId: 'b-status',
    statusStatePath: '/status',
  },
};

const base = {
  pageKey: 'p1',
  title: { 'zh-CN': '页面标题' },
  category: { key: 'c', labels: { 'zh-CN': '分类' } },
  bindings: [],
};

const ok = (data?: unknown): PageExecutionResult => ({
  kind: 'invoke',
  requestId: 'r1',
  data,
});

// props 捕获助手：最近一次渲染收到的 props
const lastProps = (mock: { mock: { calls: unknown[][] } }) =>
  mock.mock.calls[mock.mock.calls.length - 1][0] as Record<string, unknown>;

beforeEach(() => {
  jest.clearAllMocks();
});

describe('类型路由分发', () => {
  it('composite：sections 透传子渲染器', () => {
    const sections: CompositeSection[] = [{ key: 's1', bindingId: 'b1', view: 'fields' }];
    const spec = {
      ...base,
      type: 'composite',
      composite: { sections },
    } as PageSpec;
    render(<PageRenderer pageSpec={spec} onExecute={jest.fn()} preview />);
    expect(screen.getByTestId('stub-composite')).toBeInTheDocument();
    expect(lastProps(mockedComposite)).toMatchObject({
      sections,
      preview: true,
    });
  });

  it('composite：缺配置与空 sections 都渲染配置错误', () => {
    const missing = { ...base, type: 'composite', composite: undefined } as unknown as PageSpec;
    const { rerender } = render(<PageRenderer pageSpec={missing} onExecute={jest.fn()} />);
    expect(screen.getByText('配置错误')).toBeInTheDocument();
    expect(screen.getByText('组合页面缺少 composite 配置')).toBeInTheDocument();

    const empty = {
      ...base,
      type: 'composite',
      composite: { sections: [] } satisfies CompositePageSpec,
    } as PageSpec;
    rerender(<PageRenderer pageSpec={empty} onExecute={jest.fn()} />);
    expect(screen.getByText('组合页面缺少 composite 配置')).toBeInTheDocument();
  });

  it('resource：spec/title 透传，缺配置渲染错误态', () => {
    render(
      <PageRenderer
        pageSpec={{ ...base, type: 'resource', resource: { listView: {} } } as PageSpec}
        onExecute={jest.fn()}
      />,
    );
    expect(screen.getByTestId('stub-resource')).toBeInTheDocument();
    expect(lastProps(mockedResource)).toMatchObject({ title: '页面标题' });

    render(
      <PageRenderer
        pageSpec={{ ...base, type: 'resource', resource: undefined } as unknown as PageSpec}
        onExecute={jest.fn()}
      />,
    );
    expect(screen.getByText('资源页面缺少 resource 配置')).toBeInTheDocument();
  });

  it('operation：onQueryApprovalStatus 透传，缺配置渲染错误态', () => {
    const onQueryApprovalStatus = jest.fn();
    render(
      <PageRenderer
        pageSpec={{ ...base, type: 'operation', operation: { form } } as PageSpec}
        onExecute={jest.fn()}
        onQueryApprovalStatus={onQueryApprovalStatus}
      />,
    );
    expect(screen.getByTestId('stub-operation')).toBeInTheDocument();
    expect(lastProps(mockedOperation)).toMatchObject({ onQueryApprovalStatus });

    render(
      <PageRenderer
        pageSpec={{ ...base, type: 'operation', operation: undefined } as unknown as PageSpec}
        onExecute={jest.fn()}
      />,
    );
    expect(screen.getByText('操作页面缺少 operation 配置')).toBeInTheDocument();
  });

  it('task：任务回调透传，缺配置渲染错误态', () => {
    const onQueryStatus = jest.fn();
    const onCancelTask = jest.fn();
    render(
      <PageRenderer
        pageSpec={{ ...base, type: 'task', task: taskSpec } as PageSpec}
        onExecute={jest.fn()}
        onQueryStatus={onQueryStatus}
        onCancelTask={onCancelTask}
      />,
    );
    expect(screen.getByTestId('stub-task')).toBeInTheDocument();
    expect(lastProps(mockedTask)).toMatchObject({ onQueryStatus, onCancelTask, spec: taskSpec });

    render(
      <PageRenderer
        pageSpec={{ ...base, type: 'task', task: undefined } as unknown as PageSpec}
        onExecute={jest.fn()}
      />,
    );
    expect(screen.getByText('任务页面缺少 task 配置')).toBeInTheDocument();
  });

  it('report：onExport 透传，缺配置渲染错误态', () => {
    const onExport = jest.fn();
    render(
      <PageRenderer
        pageSpec={
          {
            ...base,
            type: 'report',
            report: { queryForm: form, dataset: { dimensions: [], metrics: [] } },
          } as PageSpec
        }
        onExecute={jest.fn()}
        onExport={onExport}
      />,
    );
    expect(screen.getByTestId('stub-report')).toBeInTheDocument();
    expect(lastProps(mockedReport)).toMatchObject({ onExport });

    render(
      <PageRenderer
        pageSpec={{ ...base, type: 'report', report: undefined } as unknown as PageSpec}
        onExecute={jest.fn()}
      />,
    );
    expect(screen.getByText('报表页面缺少 report 配置')).toBeInTheDocument();
  });

  it('未知类型渲染兜底错误态', () => {
    render(
      <PageRenderer
        pageSpec={{ ...base, type: 'mystery' } as unknown as PageSpec}
        onExecute={jest.fn()}
      />,
    );
    expect(screen.getByText('未知页面类型')).toBeInTheDocument();
    expect(screen.getByText('不支持的页面类型: mystery')).toBeInTheDocument();
  });
});

describe('page_state 编排', () => {
  // 输入投影只暴露被 path 引用的字段（空 path 会被 projectBindingContext 跳过）：
  // 输入读 page_state.out./y，输出把 result.data./x 落到 out
  const bindingWithIO: PageFunctionBinding = {
    id: 'b1',
    functionId: 'fn',
    usage: 'query',
    execution: { mode: 'sync' },
    selectors: {
      input: {
        assignments: [{ target: '/q', source: { kind: 'page_state', key: 'out', path: '/y' } }],
      },
      output: [{ stateKey: 'out', source: '/x', shape: 'object' }],
    },
  };

  // 输入读 page_state.k 的 a/b 两字段（两条投影对同一 key 深合并，供 merge 用例观察）
  const bindingWithK: PageFunctionBinding = {
    id: 'b1',
    functionId: 'fn',
    usage: 'query',
    execution: { mode: 'sync' },
    selectors: {
      input: {
        assignments: [
          { target: '/qa', source: { kind: 'page_state', key: 'k', path: '/a' } },
          { target: '/qb', source: { kind: 'page_state', key: 'k', path: '/b' } },
          { target: '/qi', source: { kind: 'page_state', key: 'k', path: '/0' } },
        ],
      },
    },
  };

  const compositeSpec = (bindings: PageFunctionBinding[]) =>
    ({
      ...base,
      type: 'composite',
      bindings,
      composite: { sections: [{ key: 's1', bindingId: 'b1', view: 'fields' }] },
    }) as PageSpec;

  it('executeWithPageState：assignments 为 null 的 binding 投影为空上下文，不抛错', async () => {
    // 真实线上形态：服务端 nil slice 序列化为 "assignments":null（上传即成页的
    // 无参数 list 函数）。投影曾在此 TypeError，浏览器端连执行请求都发不出，
    // 页面只剩通用错误 Alert（T11 E2E 实测回归）
    const nullAssignments = {
      id: 'b1',
      functionId: 'fn',
      usage: 'query',
      execution: { mode: 'sync' },
      selectors: { input: { assignments: null } },
    } as PageFunctionBinding;
    const onExecute = jest
      .fn<(b: string, c: unknown) => Promise<PageExecutionResult>>()
      .mockResolvedValue(ok());
    render(<PageRenderer pageSpec={compositeSpec([nullAssignments])} onExecute={onExecute} />);

    const props = lastProps(mockedComposite) as {
      onExecute: (b: string, c: unknown) => Promise<PageExecutionResult>;
    };
    await act(async () => props.onExecute('b1', { form: { page: 1 } }));
    expect(onExecute).toHaveBeenCalledWith('b1', {});
  });

  it('executeWithPageState：输出选择器落 page_state 并注入下一次执行上下文', async () => {
    const onExecute = jest
      .fn<(b: string, c: unknown) => Promise<PageExecutionResult>>()
      .mockResolvedValueOnce(ok({ x: { y: 1 } }));
    render(<PageRenderer pageSpec={compositeSpec([bindingWithIO])} onExecute={onExecute} />);

    const props = lastProps(mockedComposite) as {
      onExecute: (b: string, c: { form?: unknown }) => Promise<PageExecutionResult>;
    };
    // act 包裹：让 setState 的 updater（内含 pageStateRef 同步）被 React flush
    const first = await act(async () => props.onExecute('b1', { form: 1 }));
    expect(first.data).toEqual({ x: { y: 1 } });
    // 第一次：page_state 为空 → 输入投影后无值可注（form 未被 selector 引用也被滤除）
    expect(onExecute).toHaveBeenNthCalledWith(1, 'b1', {});

    onExecute.mockResolvedValueOnce(ok());
    await act(async () => props.onExecute('b1', {}));
    // 第二次：上一次 output 落的 out./y 已按输入投影进入执行上下文
    expect(onExecute).toHaveBeenNthCalledWith(2, 'b1', { pageState: { out: { y: 1 } } });
  });

  it('onPageStateMerge：merge 模式浅合并既有对象，replace 模式整体覆盖', async () => {
    const onExecute = jest
      .fn<(b: string, c: unknown) => Promise<PageExecutionResult>>()
      .mockResolvedValue(ok());
    render(<PageRenderer pageSpec={compositeSpec([bindingWithK])} onExecute={onExecute} />);

    const props = lastProps(mockedComposite) as {
      onExecute: (b: string, c: { form?: unknown }) => Promise<PageExecutionResult>;
      onPageStateMerge: (
        key: string,
        values: Record<string, unknown>,
        mode: 'merge' | 'replace',
      ) => void;
    };

    act(() => {
      props.onPageStateMerge('k', { a: 1, b: 1 }, 'replace');
      props.onPageStateMerge('k', { b: 2 }, 'merge');
    });
    await act(async () => props.onExecute('b1', {}));
    expect(onExecute).toHaveBeenLastCalledWith('b1', {
      pageState: { k: { a: 1, b: 2 } } as never,
    });

    // merge 遇到数组 prev → 不满足对象合并条件，整体覆盖（经 /0 投影观察）
    act(() => {
      props.onPageStateMerge('k', ['old'], 'replace');
      props.onPageStateMerge('k', [1, 2], 'merge');
    });
    await act(async () => props.onExecute('b1', {}));
    expect(onExecute).toHaveBeenLastCalledWith('b1', {
      pageState: { k: { 0: 1 } } as never,
    });
  });

  it('pageKey 变化时重置 page_state', async () => {
    const onExecute = jest
      .fn<(b: string, c: unknown) => Promise<PageExecutionResult>>()
      .mockResolvedValue(ok());
    const { rerender } = render(
      <PageRenderer pageSpec={compositeSpec([bindingWithK])} onExecute={onExecute} />,
    );

    const props = lastProps(mockedComposite) as {
      onExecute: (b: string, c: { form?: unknown }) => Promise<PageExecutionResult>;
      onPageStateMerge: (key: string, values: unknown, mode: 'merge' | 'replace') => void;
    };
    act(() => props.onPageStateMerge('k', { a: 1, b: 0 }, 'replace'));

    rerender(
      <PageRenderer pageSpec={compositeSpec([bindingWithK])} onExecute={onExecute} preview />,
    );
    // pageKey 相同（p1）→ 状态保留；随后切换 pageKey 验证重置
    await act(async () =>
      (lastProps(mockedComposite) as { onExecute: (b: string) => Promise<unknown> }).onExecute(
        'b1',
      ),
    );
    expect(onExecute).toHaveBeenLastCalledWith('b1', {
      pageState: { k: { a: 1, b: 0 } } as never,
    });

    rerender(
      <PageRenderer
        pageSpec={{ ...compositeSpec([bindingWithK]), pageKey: 'p2' } as PageSpec}
        onExecute={onExecute}
      />,
    );
    await act(() =>
      (
        lastProps(mockedComposite) as {
          onExecute: (b: string, c: Record<string, never>) => Promise<unknown>;
        }
      ).onExecute('b1', {}),
    );
    expect(onExecute).toHaveBeenLastCalledWith('b1', {});
  });
});
