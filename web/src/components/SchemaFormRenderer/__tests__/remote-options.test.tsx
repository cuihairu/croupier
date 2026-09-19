/**
 * F9 验收测试：远程选项源（x-options-source）
 */
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import { derivePresentationSpec } from '@/utils/schemaHints';
import {
  clearRemoteOptionsCache,
  optionsFromResult,
  selectByPointer,
  useRemoteOptions,
} from '@/components/SchemaFormRenderer/useRemoteOptions';
import type { FormPresentationSpec, JSONSchema, RemoteOptionsSpec } from '@/types/dashboard';

// 重 DOM 集成用例在 coverage instrumentation 负载下撞默认 5s 用例预算
// （隔离跑恒绿），与 Ops/Jobs 等重 suite 同法放宽
jest.setTimeout(20000);

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(),
}));

const { invokeFunction } = jest.requireMock('@/services/api/functions') as {
  invokeFunction: jest.Mock;
};

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

const PLAYER_RESULT = {
  result: {
    items: [
      { id: 'p1', name: 'Alice' },
      { id: 'p2', name: 'Bob' },
    ],
  },
};

describe('F9: selectByPointer / optionsFromResult', () => {
  test('通配数组段取值', () => {
    const data = { items: [{ id: 'p1' }, { id: 'p2' }] };
    expect(selectByPointer(data as unknown as JSONValue, '/items/*/id')).toEqual(['p1', 'p2']);
    expect(selectByPointer(data as unknown as JSONValue, '/items/0/id')).toBe('p1');
    expect(selectByPointer(data as unknown as JSONValue, '/missing/x')).toBeUndefined();
  });

  test('label/value 映射与缺省兜底', () => {
    const spec: RemoteOptionsSpec = {
      functionId: 'player.list',
      labelPath: '/items/*/name',
      valuePath: '/items/*/id',
    };
    expect(optionsFromResult(PLAYER_RESULT.result as unknown as JSONValue, spec)).toEqual([
      { label: 'Alice', value: 'p1' },
      { label: 'Bob', value: 'p2' },
    ]);
    // label 缺省用 value
    expect(
      optionsFromResult({ items: [{ id: 'p9' }] } as unknown as JSONValue, {
        functionId: 'x',
        labelPath: '/items/*/name',
        valuePath: '/items/*/id',
      }),
    ).toEqual([{ label: 'p9', value: 'p9' }]);
  });

  test('pointer 缺省或无前导斜杠：直接返回 data 本身', () => {
    const data = { items: [{ id: 'p1' }] };
    expect(selectByPointer(data as unknown as JSONValue, undefined)).toBe(data);
    expect(selectByPointer(data as unknown as JSONValue, 'items')).toBe(data);
  });

  test("'*' 通配段但 data 非数组：返回空数组（无可映射项）", () => {
    expect(selectByPointer({ items: {} } as unknown as JSONValue, '/items/*/id')).toEqual([]);
  });

  test('数组索引段非法（非整数/越界/负数）：返回 undefined', () => {
    const data = { items: [{ id: 'p1' }] };
    expect(selectByPointer(data as unknown as JSONValue, '/items/abc')).toBeUndefined();
    expect(selectByPointer(data as unknown as JSONValue, '/items/5')).toBeUndefined();
    expect(selectByPointer(data as unknown as JSONValue, '/items/-1')).toBeUndefined();
  });

  test('valuePath 缺省：value 回退 labelPath 取值', () => {
    expect(
      optionsFromResult({ items: [{ name: 'Alice' }] } as unknown as JSONValue, {
        functionId: 'x',
        labelPath: '/items/*/name',
      }),
    ).toEqual([{ label: 'Alice', value: 'Alice' }]);
  });

  test('labels/values 非数组（单值形态）：按单元素列表参与对齐', () => {
    // labels 非数组：labelList=[3] 与 values 对齐取下标 0
    expect(
      optionsFromResult({ total: 3, items: [{ id: 'x' }] } as unknown as JSONValue, {
        functionId: 'x',
        labelPath: '/total',
        valuePath: '/items/*/id',
      }),
    ).toEqual([{ label: '3', value: 'x' }]);
    // values 非数组：valueList=[2] 单值选项
    expect(
      optionsFromResult({ total: 2, items: [{ id: 'x' }] } as unknown as JSONValue, {
        functionId: 'x',
        labelPath: '/items/*/id',
        valuePath: '/total',
      }),
    ).toEqual([{ label: 'x', value: '2' }]);
  });

  test('values 数组含 null/undefined：跳过不产选项', () => {
    expect(
      optionsFromResult({ items: [{ id: null }, { id: 'y' }] } as unknown as JSONValue, {
        functionId: 'x',
        labelPath: '/items/*/name',
        valuePath: '/items/*/id',
      }),
    ).toEqual([{ label: 'y', value: 'y' }]);
  });

  test('结果为 undefined（label/value 均未命中）：单元素 undefined 跳过，返回空选项', () => {
    // labels/values 非数组且值为 undefined → valueList=[undefined] → continue 分支的
    // undefined 侧（null 侧由上一用例覆盖）
    expect(optionsFromResult(undefined, { functionId: 'x' })).toEqual([]);
  });
});

describe('F9: useRemoteOptions', () => {
  beforeEach(() => {
    invokeFunction.mockReset();
    clearRemoteOptionsCache();
  });

  function HookHarness({ spec, search }: { spec?: RemoteOptionsSpec; search?: string }) {
    const { options, loading } = useRemoteOptions(spec, search);
    return (
      <div>
        <span data-testid="loading">{String(loading)}</span>
        <ul data-testid="options">
          {options.map((option) => (
            <li key={option.value}>{option.label}</li>
          ))}
        </ul>
      </div>
    );
  }

  test('拉取并映射选项；同参数二次挂载命中缓存', async () => {
    invokeFunction.mockResolvedValue(PLAYER_RESULT);
    const spec: RemoteOptionsSpec = {
      functionId: 'player.list',
      labelPath: '/items/*/name',
      valuePath: '/items/*/id',
    };
    const first = render(<HookHarness spec={spec} />);
    await waitFor(() => expect(screen.getByText('Alice')).toBeTruthy());
    expect(invokeFunction).toHaveBeenCalledWith('player.list', {});

    const second = render(<HookHarness spec={spec} />);
    await waitFor(() => expect(second.container.querySelectorAll('li').length).toBe(2));
    expect(invokeFunction).toHaveBeenCalledTimes(1);
    first.unmount();
    second.unmount();
  });

  test('invokeFunction 失败静默降级为空选项，不抛错', async () => {
    invokeFunction.mockRejectedValue(new Error('forbidden'));
    const spec: RemoteOptionsSpec = { functionId: 'player.list', labelPath: '/items/*/name' };
    render(<HookHarness spec={spec} />);
    await waitFor(() => expect(screen.getByTestId('loading').textContent).toBe('false'));
    expect(screen.getAllByTestId('options')[0].children.length).toBe(0);
  });

  test('响应无 result 包装（裸 payload）：整体按数据解析', async () => {
    // 服务端直返业务对象（无 { result: ... } 信封）→ response?.result ?? response 兜底
    invokeFunction.mockResolvedValue({ items: [{ name: 'Alice', id: 'p1' }] });
    const spec: RemoteOptionsSpec = {
      functionId: 'player.list',
      labelPath: '/items/*/name',
      valuePath: '/items/*/id',
    };
    render(<HookHarness spec={spec} />);
    await waitFor(() => expect(screen.getByText('Alice')).toBeTruthy());
  });

  test('searchParam 存在时以关键词重新调用', async () => {
    invokeFunction.mockResolvedValue({ result: { items: [] } });
    const spec: RemoteOptionsSpec = {
      functionId: 'player.search',
      labelPath: '/items/*/name',
      valuePath: '/items/*/id',
      searchParam: 'keyword',
    };
    render(<HookHarness spec={spec} search="ali" />);
    await waitFor(() =>
      expect(invokeFunction).toHaveBeenCalledWith('player.search', { keyword: 'ali' }),
    );
  });

  test('spec 缺省或无 functionId：重置空选项且不发请求', async () => {
    const { rerender } = render(<HookHarness spec={undefined} />);
    await act(async () => {});
    expect(screen.getByTestId('loading').textContent).toBe('false');
    expect(screen.getByTestId('options').children.length).toBe(0);

    rerender(<HookHarness spec={{ functionId: '', labelPath: '/items/*/name' }} search="" />);
    await act(async () => {});
    expect(screen.getByTestId('loading').textContent).toBe('false');
    expect(invokeFunction).not.toHaveBeenCalled();
  });
});

describe('F9: Select 集成渲染', () => {
  beforeEach(() => {
    invokeFunction.mockReset();
    clearRemoteOptionsCache();
  });

  test('x-options-source → Select 渲染远程选项并可选值提交', async () => {
    invokeFunction.mockResolvedValue(PLAYER_RESULT);
    // 走真实 hints 推导链（x-widget/x-options-source → FormPresentationSpec）
    const spec = derivePresentationSpec(
      schemaOf({
        type: 'object',
        properties: {
          playerId: {
            type: 'string',
            title: '玩家',
            'x-widget': 'Select',
            'x-options-source': {
              functionId: 'player.list',
              labelPath: '/items/*/name',
              valuePath: '/items/*/id',
            },
          },
        },
      }) as unknown as JSONSchema,
    );
    const onFinish = jest.fn();
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
    const selector = screen.getByLabelText('玩家').closest('.ant-select') as HTMLElement;
    await act(async () => {});
    fireEvent.mouseDown(selector);
    await waitFor(() => expect(screen.getByText('Alice')).toBeTruthy());
    fireEvent.click(screen.getByText('Alice'));
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ playerId: 'p1' })),
    );
    // 缓存后仅一次调用
    expect(invokeFunction).toHaveBeenCalledTimes(1);
  });

  // U4：服务端派生产物（spec 侧 field.remoteOptions，Go buildFormFields 从
  // x-options-source 派生）→ 渲染器直接消费——不走前端 derivePresentationSpec。
  test('spec 侧 remoteOptions（服务端派生）→ Select 渲染远程选项', async () => {
    invokeFunction.mockResolvedValue(PLAYER_RESULT);
    const remote: RemoteOptionsSpec = {
      functionId: 'player.list',
      labelPath: '/items/*/name',
      valuePath: '/items/*/id',
    };
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        properties: { playerId: { type: 'string', title: '玩家' } },
      }),
      layout: 'vertical',
      fields: [
        { key: 'playerId', widget: 'Select', label: { 'zh-CN': '玩家' }, remoteOptions: remote },
      ],
    };
    const onFinish = jest.fn();
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
    const selector = screen.getByLabelText('玩家').closest('.ant-select') as HTMLElement;
    await act(async () => {});
    fireEvent.mouseDown(selector);
    await waitFor(() => expect(screen.getByText('Bob')).toBeTruthy());
    expect(invokeFunction).toHaveBeenCalledWith('player.list', {});
  });
});
