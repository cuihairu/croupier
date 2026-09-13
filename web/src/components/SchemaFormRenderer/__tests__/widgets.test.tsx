/**
 * widgets.tsx 本体分支全覆盖测试：
 * - TreeSelectWidget：treeData 归一化（label/title/value 兜底、disabled、children）、
 *   单/多选判定与值语义、远程选项源（F9，含 searchParam 搜索透传）
 * - CascaderWidget：cascaderOptions 归一化、changeOnSelect、取最后一级值
 * - RateWidget：count/allowHalf/value 缺省、onChange+onBlur 联动
 * - RemoteSelectWidget：无 remoteOptions 委托内置 SelectWidget、远程模式值归一化
 *
 * 测试手法：mock antd 的 Select/TreeSelect/Cascader 为记录 props 的桩组件
 * （widgets.tsx 只透传 props 给它们，不依赖其内部交互），Rate 用真实组件。
 */
import React from 'react';
import { act, fireEvent, render, waitFor } from '@testing-library/react';
import type { RJSFSchema, WidgetProps } from '@rjsf/utils';
import {
  CascaderWidget,
  RateWidget,
  RemoteSelectWidget,
  TreeSelectWidget,
  customWidgets,
} from '../widgets';
import { clearRemoteOptionsCache } from '../useRemoteOptions';
import type { RemoteOptionsSpec } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(),
}));

const { invokeFunction } = jest.requireMock('@/services/api/functions') as {
  invokeFunction: jest.Mock;
};

type StubProps = Record<string, unknown>;

type StubComponent = ((props: StubProps) => React.ReactElement) & {
  __registry: StubProps[];
};

// antd 桩：记录每次渲染收到的 props，供断言与直接回调触发
jest.mock('antd', () => {
  const actual = jest.requireActual('antd');
  const registry: StubProps[] = [];
  const Stub = (props: StubProps) => {
    registry.push(props);
    const testid = typeof props['data-testid'] === 'string' ? props['data-testid'] : 'antd-stub';
    return <div data-testid={testid} />;
  };
  Stub.__registry = registry;
  return { ...actual, Select: Stub, TreeSelect: Stub, Cascader: Stub };
});

// @rjsf/antd 桩：SelectWidget 可被测试临时置空（覆盖 Default 缺失的兜底分支）
jest.mock('@rjsf/antd', () => {
  const actual = jest.requireActual('@rjsf/antd');
  const widgets = { ...actual.Widgets };
  const state = { selectWidget: actual.Widgets.SelectWidget as unknown };
  Object.defineProperty(widgets, 'SelectWidget', {
    configurable: true,
    get() {
      return state.selectWidget;
    },
  });
  return { ...actual, Widgets: widgets, __selectWidgetState: state };
});

const antdMock = jest.requireMock('antd') as unknown as {
  Select: StubComponent;
  TreeSelect: StubComponent;
  Cascader: StubComponent;
};

const rjsfActual = jest.requireActual('@rjsf/antd') as typeof import('@rjsf/antd');
const rjsfMock = jest.requireMock('@rjsf/antd') as typeof import('@rjsf/antd') & {
  __selectWidgetState: { selectWidget: unknown };
};

function stubProps(testid: string): StubProps {
  const found = antdMock.Select.__registry.filter((props) => props['data-testid'] === testid);
  if (!found.length) throw new Error(`no stub render captured for ${testid}`);
  return found[found.length - 1];
}

type WidgetOverrides = Partial<WidgetProps> & Record<string, unknown>;

function makeWidgetProps(overrides: WidgetOverrides = {}): WidgetProps {
  return {
    id: 'root_field',
    schema: { type: 'string' } as RJSFSchema,
    uiSchema: {},
    options: {},
    value: undefined,
    required: false,
    disabled: false,
    readonly: false,
    autofocus: false,
    placeholder: '',
    registry: { formContext: {} } as never,
    formContext: {},
    rawErrors: [],
    hideError: false,
    label: '字段',
    multiple: undefined,
    onChange: jest.fn(),
    onBlur: jest.fn(),
    onFocus: jest.fn(),
    ...overrides,
  } as WidgetProps;
}

const RAW_NODES = [
  { label: 'A', value: 'a' },
  { title: 'B', value: 'b' },
  { value: 'c' },
  null,
  { label: 'D', value: 'd', disabled: true, children: [{ label: 'D1', value: 'd1' }] },
  { label: 'E', value: 'e', children: [] },
];

describe('TreeSelectWidget 本地模式', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    antdMock.Select.__registry.length = 0;
  });

  it('treeData 归一化：label → title → value 兜底，disabled/children 透传，空 children 丢弃', () => {
    render(
      <TreeSelectWidget
        {...makeWidgetProps({ id: 'ts-norm', options: { treeData: RAW_NODES } })}
      />,
    );
    expect(stubProps('ts-norm').treeData).toEqual([
      { title: 'A', value: 'a' },
      { title: 'B', value: 'b' },
      { title: 'c', value: 'c' },
      { title: '', value: '' },
      {
        title: 'D',
        value: 'd',
        disabled: true,
        children: [{ title: 'D1', value: 'd1' }],
      },
      { title: 'E', value: 'e' },
    ]);
    expect(stubProps('ts-norm').treeNodeFilterProp).toBe('title');
  });

  it('options 缺省（undefined）时 treeData 兜底为空数组', () => {
    render(<TreeSelectWidget {...makeWidgetProps({ id: 'ts-noopt', options: undefined })} />);
    expect(stubProps('ts-noopt').treeData).toEqual([]);
  });

  it('treeData 非数组（undefined）时兜底为空数组', () => {
    render(
      <TreeSelectWidget
        {...makeWidgetProps({ id: 'ts-noarr', options: { treeData: undefined } })}
      />,
    );
    expect(stubProps('ts-noarr').treeData).toEqual([]);
  });

  it('多选判定：options.multiple 布尔值优先（显式 false 覆盖 array schema），缺省回退 schema.type=array', () => {
    render(<TreeSelectWidget {...makeWidgetProps({ id: 'ts-m1', options: { multiple: true } })} />);
    expect(stubProps('ts-m1').multiple).toBe(true);

    render(
      <TreeSelectWidget
        {...makeWidgetProps({
          id: 'ts-m2',
          options: { multiple: false },
          schema: { type: 'array' } as RJSFSchema,
        })}
      />,
    );
    expect(stubProps('ts-m2').multiple).toBe(false);

    render(
      <TreeSelectWidget {...makeWidgetProps({ id: 'ts-m3', options: { multiple: false } })} />,
    );
    expect(stubProps('ts-m3').multiple).toBe(false);

    render(
      <TreeSelectWidget
        {...makeWidgetProps({
          id: 'ts-m4',
          schema: { type: 'array' } as RJSFSchema,
        })}
      />,
    );
    expect(stubProps('ts-m4').multiple).toBe(true);
  });

  it('值归一化：单选 string 原样 / 非 string 为 undefined；多选统一 string[]', () => {
    render(<TreeSelectWidget {...makeWidgetProps({ id: 'ts-v1', value: 'east' })} />);
    expect(stubProps('ts-v1').value).toBe('east');

    render(<TreeSelectWidget {...makeWidgetProps({ id: 'ts-v2', value: 42 })} />);
    expect(stubProps('ts-v2').value).toBeUndefined();

    render(
      <TreeSelectWidget
        {...makeWidgetProps({
          id: 'ts-v3',
          value: ['a', 1],
          options: { multiple: true },
        })}
      />,
    );
    expect(stubProps('ts-v3').value).toEqual(['a', '1']);

    render(
      <TreeSelectWidget
        {...makeWidgetProps({ id: 'ts-v4', value: 'solo', options: { multiple: true } })}
      />,
    );
    expect(stubProps('ts-v4').value).toEqual(['solo']);

    render(
      <TreeSelectWidget
        {...makeWidgetProps({ id: 'ts-v5', value: '', options: { multiple: true } })}
      />,
    );
    expect(stubProps('ts-v5').value).toEqual([]);

    render(
      <TreeSelectWidget
        {...makeWidgetProps({ id: 'ts-v6', value: undefined, options: { multiple: true } })}
      />,
    );
    expect(stubProps('ts-v6').value).toEqual([]);
  });

  it('disabled 与 readonly 任一为真即禁用', () => {
    render(<TreeSelectWidget {...makeWidgetProps({ id: 'ts-d1', disabled: true })} />);
    expect(stubProps('ts-d1').disabled).toBe(true);

    render(<TreeSelectWidget {...makeWidgetProps({ id: 'ts-d2', readonly: true })} />);
    expect(stubProps('ts-d2').disabled).toBe(true);

    render(<TreeSelectWidget {...makeWidgetProps({ id: 'ts-d3' })} />);
    expect(stubProps('ts-d3').disabled).toBe(false);
  });

  it('onChange：单选 string 原样 / 清空得空串；多选归一 string[]', () => {
    const single = makeWidgetProps({ id: 'ts-c1' });
    render(<TreeSelectWidget {...single} />);
    const p1 = stubProps('ts-c1') as { onChange: (v: unknown) => void };
    p1.onChange('east');
    expect(single.onChange).toHaveBeenCalledWith('east');
    p1.onChange(undefined);
    expect(single.onChange).toHaveBeenCalledWith('');

    const multi = makeWidgetProps({ id: 'ts-c2', options: { multiple: true } });
    render(<TreeSelectWidget {...multi} />);
    const p2 = stubProps('ts-c2') as { onChange: (v: unknown) => void };
    p2.onChange(['a', 1]);
    expect(multi.onChange).toHaveBeenCalledWith(['a', '1']);
    p2.onChange('solo');
    expect(multi.onChange).toHaveBeenCalledWith(['solo']);
    p2.onChange(undefined);
    expect(multi.onChange).toHaveBeenCalledWith([]);
  });

  it('onBlur/onFocus 回传 id 与当前值', () => {
    const props = makeWidgetProps({ id: 'ts-f1', value: 'east' });
    render(<TreeSelectWidget {...props} />);
    const stub = stubProps('ts-f1') as {
      onBlur: (e: unknown) => void;
      onFocus: (e: unknown) => void;
    };
    stub.onBlur({ type: 'blur' });
    expect(props.onBlur).toHaveBeenCalledWith('ts-f1', 'east');
    stub.onFocus({ type: 'focus' });
    expect(props.onFocus).toHaveBeenCalledWith('ts-f1', 'east');
  });
});

describe('TreeSelectWidget 远程模式（F9）', () => {
  const REMOTE: RemoteOptionsSpec = {
    functionId: 'opt.list',
    labelPath: '/items/*/name',
    valuePath: '/items/*/id',
  };

  const REMOTE_RESULT = {
    result: {
      items: [
        { name: 'Alice', id: 'p1' },
        { name: 'Bob', id: 'p2' },
      ],
    },
  };

  beforeEach(() => {
    jest.clearAllMocks();
    antdMock.Select.__registry.length = 0;
    clearRemoteOptionsCache();
    invokeFunction.mockResolvedValue(REMOTE_RESULT);
  });

  it('远程选项映射为平铺树；无 searchParam 时本地过滤、无 onSearch', async () => {
    render(
      <TreeSelectWidget
        {...makeWidgetProps({ id: 'rts-1', options: { remoteOptions: REMOTE } })}
      />,
    );
    await waitFor(() =>
      expect(stubProps('rts-1').treeData).toEqual([
        { title: 'Alice', value: 'p1' },
        { title: 'Bob', value: 'p2' },
      ]),
    );
    expect(stubProps('rts-1').loading).toBe(false);
    expect(stubProps('rts-1').filterTreeNode).toBe(true);
    expect(stubProps('rts-1').onSearch).toBeUndefined();
    expect(invokeFunction).toHaveBeenCalledWith('opt.list', {});
  });

  it('searchParam 存在时关闭本地过滤并透传搜索词（null 归一为空串）', async () => {
    const remote: RemoteOptionsSpec = { ...REMOTE, searchParam: 'keyword' };
    render(
      <TreeSelectWidget
        {...makeWidgetProps({ id: 'rts-2', options: { remoteOptions: remote } })}
      />,
    );
    await waitFor(() => expect(stubProps('rts-2').loading).toBe(false));
    expect(stubProps('rts-2').filterTreeNode).toBe(false);
    const onSearch = stubProps('rts-2').onSearch as (v: string | null) => void;
    expect(typeof onSearch).toBe('function');

    await act(async () => {
      onSearch('ali');
    });
    await waitFor(() =>
      expect(invokeFunction).toHaveBeenCalledWith('opt.list', { keyword: 'ali' }),
    );
    const callsBefore = invokeFunction.mock.calls.length;
    await act(async () => {
      onSearch(null);
    });
    // 空串关键词命中会话缓存：不重复调用
    expect(invokeFunction.mock.calls.length).toBe(callsBefore);
  });

  it('远程模式值与 onChange 语义与本地一致', () => {
    const single = makeWidgetProps({
      id: 'rts-3',
      value: 'p1',
      options: { remoteOptions: REMOTE },
    });
    render(<TreeSelectWidget {...single} />);
    expect(stubProps('rts-3').value).toBe('p1');
    const p = stubProps('rts-3') as { onChange: (v: unknown) => void };
    p.onChange('p2');
    expect(single.onChange).toHaveBeenCalledWith('p2');
    p.onChange(undefined);
    expect(single.onChange).toHaveBeenCalledWith('');

    const multi = makeWidgetProps({
      id: 'rts-4',
      value: ['p1', 2],
      options: { multiple: true, remoteOptions: REMOTE },
    });
    render(<TreeSelectWidget {...multi} />);
    expect(stubProps('rts-4').value).toEqual(['p1', '2']);
    const pm = stubProps('rts-4') as { onChange: (v: unknown) => void };
    pm.onChange(['p1', 3]);
    expect(multi.onChange).toHaveBeenCalledWith(['p1', '3']);
  });

  it('远程模式 disabled/readonly 与 onBlur/onFocus 透传', () => {
    const props = makeWidgetProps({
      id: 'rts-5',
      value: 'p1',
      readonly: true,
      options: { remoteOptions: REMOTE },
    });
    render(<TreeSelectWidget {...props} />);
    expect(stubProps('rts-5').disabled).toBe(true);
    const stub = stubProps('rts-5') as {
      onBlur: (e: unknown) => void;
      onFocus: (e: unknown) => void;
    };
    stub.onBlur({ type: 'blur' });
    expect(props.onBlur).toHaveBeenCalledWith('rts-5', 'p1');
    stub.onFocus({ type: 'focus' });
    expect(props.onFocus).toHaveBeenCalledWith('rts-5', 'p1');
  });

  it('remoteOptions 无 functionId 时走本地分支（读 props.treeData）', () => {
    render(
      <TreeSelectWidget
        {...makeWidgetProps({
          id: 'rts-6',
          options: {
            treeData: [{ label: '本地', value: 'local' }],
            remoteOptions: { functionId: '' },
          },
        })}
      />,
    );
    expect(stubProps('rts-6').treeData).toEqual([{ title: '本地', value: 'local' }]);
    expect(stubProps('rts-6').filterTreeNode).toBeUndefined();
    expect(invokeFunction).not.toHaveBeenCalled();
  });
});

describe('CascaderWidget', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    antdMock.Select.__registry.length = 0;
  });

  it('cascaderOptions 归一化：label → title → value 兜底，disabled/children 透传', () => {
    render(
      <CascaderWidget
        {...makeWidgetProps({ id: 'cas-1', options: { cascaderOptions: RAW_NODES } })}
      />,
    );
    expect(stubProps('cas-1').options).toEqual([
      { label: 'A', value: 'a' },
      { label: 'B', value: 'b' },
      { label: 'c', value: 'c' },
      { label: '', value: '' },
      { label: 'D', value: 'd', disabled: true, children: [{ label: 'D1', value: 'd1' }] },
      { label: 'E', value: 'e' },
    ]);
  });

  it('options 缺省（undefined）时 options 兜底为空数组', () => {
    render(<CascaderWidget {...makeWidgetProps({ id: 'cas-2', options: undefined })} />);
    expect(stubProps('cas-2').options).toEqual([]);
  });

  it('cascaderOptions 非数组时兜底为空数组', () => {
    render(
      <CascaderWidget {...makeWidgetProps({ id: 'cas-3', options: { cascaderOptions: 'bad' } })} />,
    );
    expect(stubProps('cas-3').options).toEqual([]);
  });

  it('changeOnSelect 仅在显式为 true 时透传', () => {
    render(
      <CascaderWidget
        {...makeWidgetProps({
          id: 'cas-4',
          options: { cascaderOptions: [], changeOnSelect: true },
        })}
      />,
    );
    expect(stubProps('cas-4').changeOnSelect).toBe(true);

    render(
      <CascaderWidget
        {...makeWidgetProps({
          id: 'cas-5',
          options: { cascaderOptions: [], changeOnSelect: false },
        })}
      />,
    );
    expect(stubProps('cas-5').changeOnSelect).toBe(false);
  });

  it('值归一化：string 转为单段路径数组，非 string 为 undefined', () => {
    render(<CascaderWidget {...makeWidgetProps({ id: 'cas-6', value: 'official' })} />);
    expect(stubProps('cas-6').value).toEqual(['official']);

    render(<CascaderWidget {...makeWidgetProps({ id: 'cas-7', value: 42 })} />);
    expect(stubProps('cas-7').value).toBeUndefined();
  });

  it('onChange 取最后一级；清空（undefined/空数组）得空串', () => {
    const props = makeWidgetProps({ id: 'cas-8', value: 'official' });
    render(<CascaderWidget {...props} />);
    const stub = stubProps('cas-8') as { onChange: (v: unknown) => void };
    stub.onChange(['android', 'official']);
    expect(props.onChange).toHaveBeenCalledWith('official');
    stub.onChange(undefined);
    expect(props.onChange).toHaveBeenCalledWith('');
    stub.onChange([]);
    expect(props.onChange).toHaveBeenCalledWith('');
  });

  it('onBlur/onFocus 回传 id 与当前值', () => {
    const props = makeWidgetProps({ id: 'cas-9', value: 'official' });
    render(<CascaderWidget {...props} />);
    const stub = stubProps('cas-9') as {
      onBlur: () => void;
      onFocus: () => void;
    };
    stub.onBlur();
    expect(props.onBlur).toHaveBeenCalledWith('cas-9', 'official');
    stub.onFocus();
    expect(props.onFocus).toHaveBeenCalledWith('cas-9', 'official');
  });
});

describe('RateWidget（真实 antd Rate）', () => {
  it('count 为数字时生效，缺省 5 颗星', () => {
    const { container: c7 } = render(
      <RateWidget {...makeWidgetProps({ id: 'rate-1', options: { count: 7 }, value: 3 })} />,
    );
    expect(c7.querySelectorAll('.ant-rate-star').length).toBe(7);
    expect(c7.querySelectorAll('.ant-rate-star-full').length).toBe(3);

    const { container: c5 } = render(
      <RateWidget {...makeWidgetProps({ id: 'rate-2', options: {} })} />,
    );
    expect(c5.querySelectorAll('.ant-rate-star').length).toBe(5);
    expect(c5.querySelectorAll('.ant-rate-star-full').length).toBe(0);
  });

  it('count 非数字或缺省（options undefined）时回退 5', () => {
    const { container } = render(
      <RateWidget {...makeWidgetProps({ id: 'rate-3', options: { count: '9' } })} />,
    );
    expect(container.querySelectorAll('.ant-rate-star').length).toBe(5);

    const { container: bare } = render(
      <RateWidget {...makeWidgetProps({ id: 'rate-3b', options: undefined })} />,
    );
    expect(bare.querySelectorAll('.ant-rate-star').length).toBe(5);
  });

  it('allowHalf 为 true 时透传半星支持', () => {
    const { container } = render(
      <RateWidget {...makeWidgetProps({ id: 'rate-4', options: { allowHalf: true }, value: 1 })} />,
    );
    expect(container.querySelectorAll('.ant-rate-star').length).toBe(5);
    expect(container.querySelector('.ant-rate')).toBeTruthy();
  });

  it('点击星标：onChange 产出 number 并联动 onBlur', () => {
    const props = makeWidgetProps({ id: 'rate-5', options: { count: 5 } });
    const { container } = render(<RateWidget {...props} />);
    const stars = container.querySelectorAll('.ant-rate-star');
    fireEvent.click(stars[3].querySelector('[role="radio"]') as Element);
    expect(props.onChange).toHaveBeenCalledWith(4);
    expect(props.onBlur).toHaveBeenCalledWith('rate-5', 4);
  });

  it('聚焦触发 onFocus 回传 id 与当前值', () => {
    const props = makeWidgetProps({ id: 'rate-6', value: 2 });
    const { container } = render(<RateWidget {...props} />);
    fireEvent.focus(container.querySelector('.ant-rate') as Element);
    expect(props.onFocus).toHaveBeenCalledWith('rate-6', 2);
  });

  it('disabled/readonly 任一为真时禁用', () => {
    const { container: cd } = render(
      <RateWidget {...makeWidgetProps({ id: 'rate-7', disabled: true })} />,
    );
    expect(cd.querySelector('.ant-rate-disabled')).toBeTruthy();

    const { container: cr } = render(
      <RateWidget {...makeWidgetProps({ id: 'rate-8', readonly: true })} />,
    );
    expect(cr.querySelector('.ant-rate-disabled')).toBeTruthy();
  });
});

describe('RemoteSelectWidget', () => {
  const REMOTE: RemoteOptionsSpec = {
    functionId: 'opt.list',
    labelPath: '/items/*/name',
    valuePath: '/items/*/id',
  };

  beforeEach(() => {
    jest.clearAllMocks();
    antdMock.Select.__registry.length = 0;
    clearRemoteOptionsCache();
    rjsfMock.__selectWidgetState.selectWidget = rjsfActual.Widgets.SelectWidget;
    invokeFunction.mockResolvedValue({
      result: {
        items: [
          { name: 'Alice', id: 'p1' },
          { name: 'Bob', id: 'p2' },
        ],
      },
    });
  });

  it('无 remoteOptions 时委托内置 SelectWidget 渲染', () => {
    const { container } = render(
      <RemoteSelectWidget {...makeWidgetProps({ id: 'rs-delegate' })} />,
    );
    expect(container.querySelector('[data-testid="antd-stub"]')).toBeTruthy();
    expect(invokeFunction).not.toHaveBeenCalled();
  });

  it('remoteOptions 无 functionId 时同样委托内置实现', () => {
    const { container } = render(
      <RemoteSelectWidget
        {...makeWidgetProps({
          id: 'rs-delegate2',
          options: { remoteOptions: { functionId: '' } },
        })}
      />,
    );
    expect(container.querySelector('[data-testid="antd-stub"]')).toBeTruthy();
    expect(invokeFunction).not.toHaveBeenCalled();
  });

  it('内置 SelectWidget 缺失时渲染 null（不抛错）；options 缺省走空对象兜底', () => {
    rjsfMock.__selectWidgetState.selectWidget = undefined;
    const { container } = render(
      <RemoteSelectWidget {...makeWidgetProps({ id: 'rs-null', options: undefined })} />,
    );
    expect(container.querySelector('[data-testid]')).toBeNull();
  });

  it('单选：string/number 值归一为 string，其余 undefined', () => {
    render(
      <RemoteSelectWidget
        {...makeWidgetProps({ id: 'rs-s1', value: 'p1', options: { remoteOptions: REMOTE } })}
      />,
    );
    expect(stubProps('rs-s1').value).toBe('p1');

    render(
      <RemoteSelectWidget
        {...makeWidgetProps({ id: 'rs-s2', value: 7, options: { remoteOptions: REMOTE } })}
      />,
    );
    expect(stubProps('rs-s2').value).toBe('7');

    render(
      <RemoteSelectWidget
        {...makeWidgetProps({ id: 'rs-s3', value: null, options: { remoteOptions: REMOTE } })}
      />,
    );
    expect(stubProps('rs-s3').value).toBeUndefined();
  });

  it('多选：数组归一 string[]，非数组为 undefined；mode=multiple', () => {
    render(
      <RemoteSelectWidget
        {...makeWidgetProps({
          id: 'rs-m1',
          multiple: true,
          value: ['p1', 2],
          options: { remoteOptions: REMOTE },
        })}
      />,
    );
    const p = stubProps('rs-m1');
    expect(p.value).toEqual(['p1', '2']);
    expect(p.mode).toBe('multiple');

    render(
      <RemoteSelectWidget
        {...makeWidgetProps({
          id: 'rs-m2',
          multiple: true,
          value: 'p1',
          options: { remoteOptions: REMOTE },
        })}
      />,
    );
    expect(stubProps('rs-m2').value).toBeUndefined();
    expect(stubProps('rs-m2').mode).toBe('multiple');

    render(
      <RemoteSelectWidget
        {...makeWidgetProps({ id: 'rs-m3', options: { remoteOptions: REMOTE } })}
      />,
    );
    expect(stubProps('rs-m3').mode).toBeUndefined();
  });

  it('远程选项挂载拉取并透传给 Select', async () => {
    render(
      <RemoteSelectWidget
        {...makeWidgetProps({ id: 'rs-opts', options: { remoteOptions: REMOTE } })}
      />,
    );
    await waitFor(() =>
      expect(stubProps('rs-opts').options).toEqual([
        { label: 'Alice', value: 'p1' },
        { label: 'Bob', value: 'p2' },
      ]),
    );
    expect(stubProps('rs-opts').loading).toBe(false);
    expect(invokeFunction).toHaveBeenCalledWith('opt.list', {});
  });

  it('searchParam：关闭本地过滤、透传搜索词，null 归一空串', async () => {
    const remote: RemoteOptionsSpec = { ...REMOTE, searchParam: 'keyword' };
    render(
      <RemoteSelectWidget
        {...makeWidgetProps({ id: 'rs-search', options: { remoteOptions: remote } })}
      />,
    );
    await waitFor(() => expect(stubProps('rs-search').loading).toBe(false));
    expect(stubProps('rs-search').filterOption).toBe(false);
    const onSearch = stubProps('rs-search').onSearch as (v: string | null) => void;
    expect(typeof onSearch).toBe('function');

    await act(async () => {
      onSearch('bo');
    });
    await waitFor(() => expect(invokeFunction).toHaveBeenCalledWith('opt.list', { keyword: 'bo' }));
    const before = invokeFunction.mock.calls.length;
    await act(async () => {
      onSearch(null);
    });
    expect(invokeFunction.mock.calls.length).toBe(before);

    // 无 searchParam：本地过滤、无 onSearch
    render(
      <RemoteSelectWidget
        {...makeWidgetProps({ id: 'rs-nosearch', options: { remoteOptions: REMOTE } })}
      />,
    );
    await waitFor(() => expect(stubProps('rs-nosearch').loading).toBe(false));
    expect(stubProps('rs-nosearch').filterOption).toBe(true);
    expect(stubProps('rs-nosearch').onSearch).toBeUndefined();
  });

  it('单选 onChange：string 原样、非 string 得空串，并联动 onBlur', () => {
    const props = makeWidgetProps({
      id: 'rs-c1',
      value: 'p1',
      options: { remoteOptions: REMOTE },
    });
    render(<RemoteSelectWidget {...props} />);
    const stub = stubProps('rs-c1') as { onChange: (v: unknown) => void };
    stub.onChange('p2');
    expect(props.onChange).toHaveBeenCalledWith('p2');
    expect(props.onBlur).toHaveBeenCalledWith('rs-c1', 'p2');
    stub.onChange(undefined);
    expect(props.onChange).toHaveBeenCalledWith('');
    expect(props.onBlur).toHaveBeenCalledWith('rs-c1', undefined);
  });

  it('多选 onChange：数组归一 string[]，非数组兜底 []，并联动 onBlur', () => {
    const props = makeWidgetProps({
      id: 'rs-c2',
      multiple: true,
      options: { remoteOptions: REMOTE },
    });
    render(<RemoteSelectWidget {...props} />);
    const stub = stubProps('rs-c2') as { onChange: (v: unknown) => void };
    stub.onChange(['p1', 2]);
    expect(props.onChange).toHaveBeenCalledWith(['p1', '2']);
    expect(props.onBlur).toHaveBeenCalledWith('rs-c2', ['p1', 2]);
    stub.onChange('not-array');
    expect(props.onChange).toHaveBeenCalledWith([]);
  });

  it('onFocus 回传 id 与当前值；disabled/readonly 禁用', () => {
    const props = makeWidgetProps({
      id: 'rs-f1',
      value: 'p1',
      readonly: true,
      options: { remoteOptions: REMOTE },
    });
    render(<RemoteSelectWidget {...props} />);
    expect(stubProps('rs-f1').disabled).toBe(true);
    const stub = stubProps('rs-f1') as { onFocus: () => void };
    stub.onFocus();
    expect(props.onFocus).toHaveBeenCalledWith('rs-f1', 'p1');
  });
});

describe('customWidgets 注册表', () => {
  it('treeSelect/cascader/rate/select 指向对应实现', () => {
    expect(customWidgets.treeSelect).toBe(TreeSelectWidget);
    expect(customWidgets.cascader).toBe(CascaderWidget);
    expect(customWidgets.rate).toBe(RateWidget);
    expect(customWidgets.select).toBe(RemoteSelectWidget);
  });
});
