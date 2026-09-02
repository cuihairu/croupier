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
});
