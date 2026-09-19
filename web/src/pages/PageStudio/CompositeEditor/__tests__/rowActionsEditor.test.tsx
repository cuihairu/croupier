/** RowActionsEditor（行操作编辑器）覆盖：弹窗选项构造（title 缺省回退/
 * functionId 透出）、增删改行操作、targetSection 换选重置 params、danger
 * 开关、无弹窗时添加按钮禁用提示、参数映射（Select 形态/手输形态改名
 * 校验：空·同名·撞名不生效、失焦与回车提交、行字段值更新、条目删除、
 * 添加映射：无行字段禁用·全占用不加·空闲字段自动带入）、契约参数提取
 * （functionId 缺失/未注册回退空）。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { App } from 'antd';
import RowActionsEditor from '../RowActionsEditor';
import type { PageNode } from '../model';
import type { FunctionDescriptor } from '@/services/api/functions';

// ExpressionInput 替身：受控 input 透传 onChange；挂载时调用 rootsOf 探针
// （覆盖 RowActionsEditor 向 ParamMapping 注入的 rootsOf={() => []} 函数路径）
jest.mock('../ExpressionInput', () => {
  // 大写命名：react-hooks/rules-of-hooks 按函数名识别组件
  const MockExpressionInput: React.FC<{
    value?: string;
    onChange: (v: string) => void;
    rootsOf?: (name: string) => string[];
  }> = ({ value, onChange, rootsOf }) => {
    const { useEffect } = require('react') as typeof React;
    useEffect(() => {
      rootsOf?.('__probe__');
    }, [rootsOf]);
    return (
      <input
        type="text"
        data-testid="expr-input"
        value={value ?? ''}
        onChange={(e) => onChange(e.target.value)}
      />
    );
  };
  return { __esModule: true, default: MockExpressionInput };
});

const banFn: FunctionDescriptor = {
  id: 'player.ban',
  operation: 'update',
  resource: 'player',
  inputSchema: {
    type: 'object',
    required: ['playerId'],
    properties: { playerId: { type: 'string' }, reason: { type: 'string' } },
  },
  outputSchema: { type: 'object', properties: { ok: { type: 'boolean' } } },
};

const modalNode = (id: string, title: string | undefined, functionId: string): PageNode => ({
  id,
  type: 'modal',
  props: title === undefined ? {} : { title },
  children: [{ id: `${id}-f`, type: 'fnForm', props: { functionId } }],
});

const nodes = (): PageNode[] => [
  // 命中 fnById 的弹窗 + 无 title 弹窗 + 空 functionId 弹窗 + 非 modal 节点
  modalNode('m1', '封禁弹窗', 'player.ban'),
  modalNode('m2', undefined, 'player.ban'),
  modalNode('m3', '空函数', ''),
  { id: 't1', type: 'fnTable', props: {} },
  // 无 fnForm 子的 modal（不进选项）
  { id: 'm4', type: 'modal', props: { title: '坏弹窗' }, children: [] },
  // fnForm 无 functionId 键（?? '' 的 undefined 侧；m3 是空串走左侧）
  {
    id: 'm5',
    type: 'modal',
    props: { title: '缺函数键' },
    children: [{ id: 'm5-f', type: 'fnForm', props: {} }],
  },
];

const fnById = new Map([['player.ban', banFn]]);

interface RenderOptions {
  value?: unknown;
  rowFields?: string[];
}

function renderEditor(options: RenderOptions = {}) {
  const onChange = jest.fn();
  const utils = render(
    <App>
      <RowActionsEditor
        value={options.value}
        nodes={nodes()}
        fnById={fnById}
        rowFields={options.rowFields ?? ['uid', 'nickname']}
        onChange={onChange as never}
      />
    </App>,
  );
  return { onChange, ...utils };
}

describe('行操作编辑主体', () => {
  it('value 非数组与空：无条目，添加按钮可用（有弹窗）', () => {
    const { onChange } = renderEditor({ value: 'not-array' });
    expect(screen.getByRole('button', { name: /添加行操作/ })).toBeEnabled();

    fireEvent.click(screen.getByRole('button', { name: /添加行操作/ }));
    expect(onChange).toHaveBeenCalledWith([
      { label: '', targetSection: '', params: {}, danger: false },
    ]);
  });

  it('弹窗选项：title 缺省回退「弹窗」；含 fnForm 的 modal 才进选项', async () => {
    renderEditor({
      value: [{ label: '封禁', targetSection: '', params: {}, danger: false }],
    });
    fireEvent.mouseDown(screen.getByText('打开弹窗'));
    // m1 带标题；m2 无标题回退；m3 空 functionId 括号空串；m4 不在
    expect(await screen.findByText('封禁弹窗（player.ban）')).toBeInTheDocument();
    expect(screen.getByText('弹窗（player.ban）')).toBeInTheDocument();
    expect(screen.getByText('空函数（）')).toBeInTheDocument();
    expect(screen.queryByText('坏弹窗')).not.toBeInTheDocument();
    // m5：fnForm props 无 functionId 键 → functionId ?? '' 兜底
    expect(screen.getByText('缺函数键（）')).toBeInTheDocument();
  });

  it('多条行操作编辑非首条：patch map 的其余条目原样保位', () => {
    const first = { label: '甲', targetSection: 'm1', params: {}, danger: false };
    const second = { label: '乙', targetSection: 'm2', params: {}, danger: false };
    const { onChange } = renderEditor({ value: [first, second] });
    fireEvent.change(screen.getByDisplayValue('乙'), { target: { value: '丙' } });
    // 非编辑条目（idx 0）走 map else 侧原样返回
    expect(onChange).toHaveBeenCalledWith([first, { ...second, label: '丙' }]);
  });

  it('无可用弹窗：添加禁用并提示', () => {
    const onChange = jest.fn();
    render(
      <App>
        <RowActionsEditor
          value={[]}
          nodes={[{ id: 't1', type: 'fnTable', props: {} }]}
          fnById={fnById}
          rowFields={['uid']}
          onChange={onChange as never}
        />
      </App>,
    );
    const add = screen.getByRole('button', { name: /先创建含表单的弹窗/ });
    expect(add).toBeDisabled();
    fireEvent.click(add);
    expect(onChange).not.toHaveBeenCalled();
  });

  it('编辑 label / danger / 删除；换选弹窗重置 params', async () => {
    const { onChange } = renderEditor({
      value: [{ label: '封禁', targetSection: 'm1', params: { playerId: 'uid' }, danger: false }],
    });
    // label 输入
    fireEvent.change(screen.getByDisplayValue('封禁'), { target: { value: '禁言' } });
    expect(onChange).toHaveBeenLastCalledWith([expect.objectContaining({ label: '禁言' })]);

    // danger 开关
    fireEvent.click(screen.getByRole('switch'));
    expect(onChange).toHaveBeenLastCalledWith([expect.objectContaining({ danger: true })]);

    // 换选弹窗：params 重置
    fireEvent.mouseDown(screen.getByText('封禁弹窗（player.ban）'));
    const option = await screen.findAllByText('弹窗（player.ban）');
    fireEvent.click(option[0]);
    expect(onChange).toHaveBeenLastCalledWith([
      expect.objectContaining({ targetSection: 'm2', params: {} }),
    ]);

    // 删除行操作
    fireEvent.click(screen.getByRole('button', { name: /删\s*除/ }));
    expect(onChange).toHaveBeenLastCalledWith([]);
  });
});

describe('参数映射（ParamMapping）', () => {
  const withMapping = (
    params: Record<string, string>,
    rowFields: string[] = ['uid', 'nickname'],
    target = 'm1',
  ) => {
    const onChange = jest.fn();
    render(
      <App>
        <RowActionsEditor
          value={[{ label: '封禁', targetSection: target, params, danger: false }]}
          nodes={nodes()}
          fnById={fnById}
          rowFields={rowFields}
          onChange={onChange as never}
        />
      </App>,
    );
    return { onChange };
  };

  it('契约参数字段 → Select 形态；行字段值经 ExpressionInput 更新', async () => {
    const { onChange } = withMapping({ playerId: 'uid' });
    // m1 → player.ban 契约有 playerId/reason → Select 显示参数名
    expect(await screen.findByText('参数带入（表单参数 ← 行字段）')).toBeInTheDocument();

    const expr = screen.getByTestId('expr-input') as HTMLInputElement;
    expect(expr.value).toBe('uid');
    fireEvent.change(expr, { target: { value: 'row.uid' } });
    expect(onChange).toHaveBeenLastCalledWith([
      expect.objectContaining({ params: { playerId: 'row.uid' } }),
    ]);

    // Select 换参数名：保位改名（option 点击目标须为 content div——aria div 不派发）
    fireEvent.mouseDown(screen.getByText('playerId'));
    const reasonOption = (await screen.findAllByText('reason')).find((el) =>
      el.className.includes('ant-select-item-option'),
    ) as HTMLElement;
    fireEvent.mouseDown(reasonOption);
    fireEvent.click(reasonOption);
    // 保位改名：值沿用原条目（ExpressionInput 的修改未回灌组件，仍是初始 uid）
    expect(onChange).toHaveBeenLastCalledWith([
      expect.objectContaining({ params: { reason: 'uid' } }),
    ]);
  });

  it('契约缺失（functionId 空）→ 手输参数名：合法改名/空名不生效/撞名不生效', async () => {
    const { onChange } = withMapping({ foo: 'uid' }, ['uid', 'nickname'], 'm3');
    await screen.findByText('参数带入（表单参数 ← 行字段）');
    const nameInput = screen.getByDisplayValue('foo') as HTMLInputElement;

    // 空名：失焦不生效
    fireEvent.change(nameInput, { target: { value: '   ' } });
    fireEvent.blur(nameInput);
    expect(onChange).not.toHaveBeenCalled();

    // 同名：不生效
    fireEvent.change(nameInput, { target: { value: ' foo ' } });
    fireEvent.blur(nameInput);
    expect(onChange).not.toHaveBeenCalled();

    // 合法改名：保位（顺序不变）
    fireEvent.change(nameInput, { target: { value: 'bar' } });
    fireEvent.keyDown(nameInput, { key: 'Enter' });
    expect(onChange).toHaveBeenLastCalledWith([
      expect.objectContaining({ params: { bar: 'uid' } }),
    ]);
  });

  it('手输形态撞名标红且不生效；删除条目', async () => {
    // m3（functionId 空）→ paramFields=[]，参数名手输
    const { onChange } = withMapping({ a: 'uid', b: 'nickname' }, ['uid', 'nickname'], 'm3');
    await screen.findByText('参数带入（表单参数 ← 行字段）');

    // 第一个参数名改成 b（已存在）→ error 状态 + 失焦不生效
    const first = screen.getByDisplayValue('a') as HTMLInputElement;
    fireEvent.change(first, { target: { value: 'b' } });
    expect(first.className).toContain('status-error');
    fireEvent.blur(first);
    expect(onChange).not.toHaveBeenCalled();

    // blur 从未输入过的参数名框：无草稿（draft undefined）直接 return
    const second = screen.getByDisplayValue('b') as HTMLInputElement;
    fireEvent.blur(second);
    expect(onChange).not.toHaveBeenCalled();

    // 合法改名 a→c：另一键 b 保位（rename map 的 else 侧）
    fireEvent.change(first, { target: { value: 'c' } });
    fireEvent.blur(first);
    expect(onChange).toHaveBeenLastCalledWith([
      expect.objectContaining({ params: { c: 'uid', b: 'nickname' } }),
    ]);

    // 删除第二条（× 按钮）——受控组件未回喂改名结果，仍基于初始 {a,b}
    const removes = screen.getAllByRole('button', { name: '×' });
    fireEvent.click(removes[1]);
    expect(onChange).toHaveBeenLastCalledWith([expect.objectContaining({ params: { a: 'uid' } })]);
  });

  it('rowFields 空：添加映射禁用', () => {
    withMapping({}, []);
    expect(screen.getByRole('button', { name: '+ 添加映射' })).toBeDisabled();
  });

  it('有空闲行字段：添加映射自动带入（参数名=行字段名）', () => {
    const { onChange } = withMapping({ playerId: 'uid' });
    fireEvent.click(screen.getByRole('button', { name: '+ 添加映射' }));
    // 空闲行字段 = uid（used 只含参数名 playerId）
    expect(onChange).toHaveBeenLastCalledWith([
      expect.objectContaining({ params: { playerId: 'uid', uid: 'uid' } }),
    ]);
  });

  it('行字段全被参数名占用：添加映射不产生调用', () => {
    const { onChange } = withMapping({ uid: 'uid', nickname: 'nickname' });
    fireEvent.click(screen.getByRole('button', { name: '+ 添加映射' }));
    expect(onChange).not.toHaveBeenCalled();
  });
});
