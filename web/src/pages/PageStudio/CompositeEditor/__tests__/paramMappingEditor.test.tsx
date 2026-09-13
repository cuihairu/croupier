/** ParamMappingEditor（显式参数映射编辑器）覆盖：无 fn/无参数渲染 null、
 * inputSchema 对象/字符串/坏 JSON 三态解析、required 星标、kind 切换四路
 * （auto 清空/literal 置空串/page_state 取首源/无 sources 空串兜底）、
 * 来源区块与字段双 Select 联动（换源重置首字段/换字段保源保缺省）、
 * 改名提示（field≠param 显示、同名不显示）、缺省值输入与清空置 undefined、
 * literal 值经 ExpressionInput 更新、sources 过滤（坏 schema/未注册函数/
 * 排除自身）。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import ParamMappingEditor, { type InputAssignment } from '../ParamMappingEditor';
import type { PageNode } from '../model';
import type { FunctionDescriptor } from '@/services/api/functions';

// ExpressionInput 替身：受控 input 透传 onChange
jest.mock('../ExpressionInput', () => ({
  __esModule: true,
  default: ({ value, onChange }: { value?: string; onChange: (v: string) => void }) => (
    <input
      type="text"
      data-testid="expr-input"
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));

const banFn: FunctionDescriptor = {
  id: 'player.ban',
  operation: 'update',
  resource: 'player',
  inputSchema: {
    type: 'object',
    required: ['playerId'],
    properties: { playerId: { type: 'string' }, reason: { type: 'string' } },
  },
  outputSchema: {
    type: 'object',
    properties: { uid: { type: 'string' }, nickname: { type: 'string' } },
  },
};

// outputSchema 为字符串形态（覆盖 JSON.parse 分支）
const strFn: FunctionDescriptor = {
  ...banFn,
  id: 'player.query',
  outputSchema: JSON.stringify({
    type: 'object',
    properties: { uid: { type: 'string' }, status: { type: 'number' } },
  }),
};

const fnById = new Map([
  ['player.ban', banFn],
  ['player.query', strFn],
  // outputSchema 坏 JSON：fieldsOf 走 catch 返回空
  ['player.broken', { ...banFn, id: 'player.broken', outputSchema: 'not-json' }],
  // outputSchema 对象但无 properties：?? {} 空侧
  ['player.empty', { ...banFn, id: 'player.empty', outputSchema: { type: 'object' } }],
]);

/** 来源节点：静态表单（对象 schema）/函数表（对象 outputSchema）/函数表
 * （字符串 outputSchema）/坏 JSON 静态表单/未注册函数/字符串 schema 静态表单 */
const nodes = (): PageNode[] => [
  { id: 'self', type: 'fnForm', props: { functionId: 'player.ban' } },
  {
    id: 's1',
    type: 'staticForm',
    props: {
      title: '区块A',
      staticSchema: { type: 'object', properties: { uid: {}, nickname: {} } },
    },
  },
  { id: 's2', type: 'fnTable', props: { title: '区块B', functionId: 'player.ban' } },
  { id: 's3', type: 'fnTable', props: { title: '区块C', functionId: 'player.query' } },
  { id: 's4', type: 'staticForm', props: { title: '坏区块', staticSchema: 'not-json' } },
  { id: 's5', type: 'fnTable', props: { title: '幽灵区块', functionId: 'missing.fn' } },
  { id: 's7', type: 'fnTable', props: { title: '碎区块', functionId: 'player.broken' } },
  {
    id: 's6',
    type: 'staticForm',
    props: { staticSchema: JSON.stringify({ properties: { level: {} } }) },
  },
  // 边角：无 functionId 的函数表（fieldsOf 走 ?? '' → 未命中）、无 properties 的
  // 静态 schema（?? {} 空侧）、children 递归（sectionKey 空 + 与 s1 重复）
  { id: 's8', type: 'fnTable', props: { functionId: 'player.empty' } },
  { id: 's8b', type: 'fnTable', props: {} },
  { id: 's9', type: 'staticForm', props: { staticSchema: { type: 'object' } } },
  {
    id: 'c1',
    type: 'container',
    props: { sectionKey: 'players' },
    children: [
      { id: 'c1a', type: 'text', props: {} },
      { id: 'c1b', type: 'text', props: { sectionKey: 'players' } },
    ],
  },
];

interface RenderOptions {
  /** 显式传 undefined 表示「无 fn」（用 'fn' in options 区分缺省） */
  fn?: FunctionDescriptor;
  value?: InputAssignment[];
  nodeList?: PageNode[];
}

function renderEditor(options: RenderOptions = {}) {
  const onChange = jest.fn();
  const fn = 'fn' in options ? options.fn : banFn;
  const utils = render(
    <ParamMappingEditor
      fn={fn}
      nodes={options.nodeList ?? nodes()}
      selfId="self"
      fnById={fnById}
      value={options.value}
      onChange={onChange}
    />,
  );
  return { onChange, ...utils };
}

/** 匹配 <code>playerId *</code> 整体文本（两个 JSX 文本节点，需按 textContent 匹配）。 */
const codeText = (text: string) => (content: string, el: HTMLElement | null) =>
  el?.tagName === 'CODE' && el.textContent === text;

/** 点击 rc-select 的 option（content div 才派发，aria div 不派发）。 */
const clickOption = async (label: string) => {
  const option = (await screen.findAllByText(label)).find((el) =>
    el.className.includes('ant-select-item-option'),
  ) as HTMLElement;
  fireEvent.mouseDown(option);
  fireEvent.click(option);
};

describe('渲染门槛与参数解析', () => {
  it('fn 缺失 / inputSchema 无 properties / 坏 JSON：渲染 null', () => {
    const { container } = renderEditor({ fn: undefined });
    expect(container.querySelector('.ant-space')).not.toBeInTheDocument();

    // 无 properties
    const empty = render(
      <ParamMappingEditor
        fn={{ ...banFn, inputSchema: { type: 'object' } }}
        nodes={nodes()}
        selfId="self"
        fnById={fnById}
        value={undefined}
        onChange={jest.fn()}
      />,
    );
    expect(empty.container.querySelector('.ant-space')).not.toBeInTheDocument();
    empty.unmount();

    // 坏 JSON 字符串
    const broken = render(
      <ParamMappingEditor
        fn={{ ...banFn, inputSchema: 'not-json' }}
        nodes={nodes()}
        selfId="self"
        fnById={fnById}
        value={undefined}
        onChange={jest.fn()}
      />,
    );
    expect(broken.container.querySelector('.ant-space')).not.toBeInTheDocument();
    broken.unmount();
  });

  it('对象形态：hint + required 星标（playerId *）与普通参数', () => {
    renderEditor();
    expect(screen.getByText(/参数映射：默认取本区块表单值/)).toBeInTheDocument();
    expect(screen.getByText(codeText('playerId *'))).toBeInTheDocument();
    expect(screen.getByText(codeText('reason'))).toBeInTheDocument();
  });

  it('inputSchema 字符串形态：同样解析出参数', () => {
    renderEditor({ fn: { ...banFn, inputSchema: JSON.stringify(banFn.inputSchema) } });
    expect(screen.getByText(codeText('playerId *'))).toBeInTheDocument();
  });
});

describe('kind 切换', () => {
  it('自动 → 固定值：置空串 literal；ExpressionInput 出现并可更新值', async () => {
    const { onChange } = renderEditor();
    fireEvent.mouseDown(screen.getAllByText('自动')[0]);
    await clickOption('固定值');
    expect(onChange).toHaveBeenLastCalledWith([{ param: 'playerId', kind: 'literal', value: '' }]);
  });

  it('literal 赋值：经 ExpressionInput 更新 value', () => {
    const { onChange } = renderEditor({
      value: [{ param: 'reason', kind: 'literal', value: 'x' }],
    });
    const expr = screen.getByTestId('expr-input') as HTMLInputElement;
    expect(expr.value).toBe('x');
    fireEvent.change(expr, { target: { value: '违规发言' } });
    expect(onChange).toHaveBeenLastCalledWith([
      { param: 'reason', kind: 'literal', value: '违规发言' },
    ]);
  });

  it('literal 值为 null：ExpressionInput 收到空串', () => {
    renderEditor({ value: [{ param: 'reason', kind: 'literal', value: null }] });
    expect((screen.getByTestId('expr-input') as HTMLInputElement).value).toBe('');
  });

  it('自动 → 上游区块（有源）：取第一个来源与其首字段', async () => {
    const { onChange } = renderEditor();
    fireEvent.mouseDown(screen.getAllByText('自动')[0]);
    await clickOption('上游区块');
    expect(onChange).toHaveBeenLastCalledWith([
      { param: 'playerId', kind: 'page_state', sourceNodeId: 's1', field: 'uid' },
    ]);
  });

  it('自动 → 上游区块（无可用来源）：空串兜底', async () => {
    const { onChange } = renderEditor({ nodeList: [{ id: 'self', type: 'fnForm', props: {} }] });
    fireEvent.mouseDown(screen.getAllByText('自动')[0]);
    await clickOption('上游区块');
    expect(onChange).toHaveBeenLastCalledWith([
      { param: 'playerId', kind: 'page_state', sourceNodeId: '', field: '' },
    ]);
  });

  it('上游区块 → 自动：清掉该参数的映射', async () => {
    const { onChange } = renderEditor({
      value: [{ param: 'playerId', kind: 'page_state', sourceNodeId: 's1', field: 'uid' }],
    });
    fireEvent.mouseDown(screen.getAllByText('上游区块')[0]);
    await clickOption('自动');
    expect(onChange).toHaveBeenLastCalledWith([]);
  });
});

describe('page_state 双 Select 联动', () => {
  const pageState = (field: string, extra: Partial<InputAssignment> = {}): InputAssignment[] => [
    { param: 'playerId', kind: 'page_state', sourceNodeId: 's1', field, ...extra },
  ];

  it('来源选项：坏 JSON / 未注册函数 / 自身不进列表；换源重置首字段', async () => {
    renderEditor({ value: pageState('uid') });
    fireEvent.mouseDown(screen.getByText('区块A'));
    // 坏区块（坏 schema）、幽灵区块（未注册）、自身均不在；字符串 schema 的 s6 无 title → 用 id
    expect(await screen.findByText('区块B')).toBeInTheDocument();
    expect(screen.getByText('区块C')).toBeInTheDocument();
    expect(screen.getByText('s6')).toBeInTheDocument();
    expect(screen.queryByText('坏区块')).not.toBeInTheDocument();
    expect(screen.queryByText('幽灵区块')).not.toBeInTheDocument();
    expect(screen.queryByText('碎区块')).not.toBeInTheDocument();

    // 换到区块C（player.query 字符串 outputSchema）→ 首字段 uid
    await clickOption('区块C');
    // 直接断言（onChange 不回灌）：换源派发首字段重置
  });

  it('换源派发 {新源 + 首字段}', async () => {
    const { onChange } = renderEditor({ value: pageState('uid') });
    fireEvent.mouseDown(screen.getByText('区块A'));
    await clickOption('区块C');
    expect(onChange).toHaveBeenLastCalledWith([
      { param: 'playerId', kind: 'page_state', sourceNodeId: 's3', field: 'uid' },
    ]);
  });

  it('换字段：保源保缺省值', async () => {
    const { onChange } = renderEditor({ value: pageState('uid', { defaultValue: '0' }) });
    fireEvent.mouseDown(screen.getByText('uid'));
    await clickOption('nickname');
    expect(onChange).toHaveBeenLastCalledWith([
      {
        param: 'playerId',
        kind: 'page_state',
        sourceNodeId: 's1',
        field: 'nickname',
        defaultValue: '0',
      },
    ]);
  });

  it('改名提示：field ≠ param 显示；同名不显示', () => {
    const { rerender } = renderEditor({ value: pageState('uid') });
    expect(screen.getByText('改名 uid → playerId')).toBeInTheDocument();

    rerender(
      <ParamMappingEditor
        fn={banFn}
        nodes={nodes()}
        selfId="self"
        fnById={fnById}
        value={pageState('playerId')}
        onChange={jest.fn()}
      />,
    );
    expect(screen.queryByText(/改名/)).not.toBeInTheDocument();
  });

  it('缺省值：输入更新 defaultValue；清空置 undefined；null 显示空串', () => {
    const { onChange } = renderEditor({ value: pageState('uid', { defaultValue: '0' }) });
    const input = screen.getByPlaceholderText('缺省值（可选）') as HTMLInputElement;
    expect(input.value).toBe('0');
    fireEvent.change(input, { target: { value: '9' } });
    expect(onChange).toHaveBeenLastCalledWith([
      {
        param: 'playerId',
        kind: 'page_state',
        sourceNodeId: 's1',
        field: 'uid',
        defaultValue: '9',
      },
    ]);
    fireEvent.change(input, { target: { value: '' } });
    expect(onChange).toHaveBeenLastCalledWith([
      {
        param: 'playerId',
        kind: 'page_state',
        sourceNodeId: 's1',
        field: 'uid',
        defaultValue: undefined,
      },
    ]);
  });

  it('缺省值为 null：输入框显示空串', () => {
    renderEditor({ value: pageState('uid', { defaultValue: null }) });
    expect((screen.getByPlaceholderText('缺省值（可选）') as HTMLInputElement).value).toBe('');
  });

  it('page_state 缺 sourceNodeId/field：双 Select 走 placeholder；改缺省值兜底空源', async () => {
    const { onChange } = renderEditor({
      value: [{ param: 'playerId', kind: 'page_state' }],
    });
    expect(screen.getByText('来源区块')).toBeInTheDocument();
    expect(screen.getByText('字段')).toBeInTheDocument();
    const input = screen.getByPlaceholderText('缺省值（可选）') as HTMLInputElement;
    fireEvent.change(input, { target: { value: '7' } });
    expect(onChange).toHaveBeenLastCalledWith([
      {
        param: 'playerId',
        kind: 'page_state',
        sourceNodeId: '',
        field: undefined,
        defaultValue: '7',
      },
    ]);

    // 缺源状态下换源：直接选中来源并派发首字段（227 行「换字段兜底空串」为
    // UI 不可达防御——字段 options 依赖已选源，缺源时字段下拉恒空）
    fireEvent.mouseDown(screen.getByText('来源区块'));
    await clickOption('区块A');
    expect(onChange).toHaveBeenLastCalledWith([
      { param: 'playerId', kind: 'page_state', sourceNodeId: 's1', field: 'uid' },
    ]);
  });
});
