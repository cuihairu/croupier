/** ConditionEditor（U10 区块条件显示编辑器）覆盖：无值态（dashed 按钮
 * → onChange 初始条件）、有值态（提示/表达式接线/三运算符切换/比较值
 * Input 仅非 exists 显示/patch 合并）、移除条件 → undefined、rootsOf 经
 * sectionKey 索引（trim/缺省回落）与子树递归。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import ConditionEditor, { type VisibleWhenProp } from '../ConditionEditor';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

// 轻替身：表达式输入透传 value/onChange，暴露 rootsOf 供断言
jest.mock('../ExpressionInput', () => {
  const Mock = (props: {
    value: string;
    onChange: (v: string) => void;
    rootsOf: (name: string) => unknown;
    placeholder: string;
  }) => (
    <input
      aria-label="expr"
      value={props.value}
      placeholder={props.placeholder}
      onChange={(e) => props.onChange(e.target.value)}
      onMouseDown={() => void props.rootsOf('probe')}
    />
  );
  return { __esModule: true, default: Mock };
});

const fnById = new Map<string, FunctionDescriptor>();
const nodes: PageNode[] = [
  {
    id: 'n1',
    type: 'fnFields',
    props: { sectionKey: ' source ' },
    children: [{ id: 'n1x', type: 'text', props: { sectionKey: 'nested' } }],
  },
  { id: 'n2', type: 'fnTable', props: {} },
];

function renderEditor(value?: VisibleWhenProp) {
  const onChange = jest.fn();
  const utils = render(
    <ConditionEditor
      value={value}
      onChange={onChange}
      nodes={nodes}
      selfId="self"
      fnById={fnById}
    />,
  );
  return { onChange, ...utils };
}

const clickOption = (label: string) => {
  fireEvent.mouseDown(screen.getByRole('combobox'));
  fireEvent.click(screen.getByTitle(label));
};

// 受控包装：onChange 回灌 state，验证受控重渲染行为
function Controlled({ initial }: { initial?: VisibleWhenProp }) {
  const [v, setV] = React.useState<VisibleWhenProp | undefined>(initial);
  return <ConditionEditor value={v} onChange={setV} nodes={nodes} selfId="self" fnById={fnById} />;
}

describe('ConditionEditor', () => {
  it('无值：dashed 按钮 → 初始条件', () => {
    const { onChange } = renderEditor(undefined);
    fireEvent.click(screen.getByRole('button', { name: '+ 设置显示条件' }));
    expect(onChange).toHaveBeenCalledWith({ expr: '', op: 'equals', value: '' });
  });

  it('有值：提示 + 表达式 patch', () => {
    const { onChange } = renderEditor({ expr: '{{var.values.a}}', op: 'equals', value: 'x' });
    expect(screen.getByText('条件不满足时本区块不渲染（执行/联动不受影响）')).toBeInTheDocument();
    const expr = screen.getByLabelText('expr') as HTMLInputElement;
    expect(expr.value).toBe('{{var.values.a}}');
    fireEvent.change(expr, { target: { value: '{{var.values.b}}' } });
    expect(onChange).toHaveBeenLastCalledWith({
      expr: '{{var.values.b}}',
      op: 'equals',
      value: 'x',
    });
  });

  it('op 缺省回退 equals：比较值显示 + 输入 patch', () => {
    const { onChange } = renderEditor({ expr: '', value: '' });
    const input = screen.getByPlaceholderText('比较值') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'vip' } });
    // 显示回退 equals，但 patch 展开保持原 value 形态（无 op 键）
    expect(onChange).toHaveBeenLastCalledWith({ expr: '', value: 'vip' });
  });

  it('受控切换 exists：重渲染后比较值隐藏', () => {
    render(<Controlled initial={{ expr: '', op: 'notEquals', value: 'a' }} />);
    clickOption('有值');
    expect(screen.queryByPlaceholderText('比较值')).not.toBeInTheDocument();
  });

  it('expr/value 键缺省：?? 空串兜底（仅 op）', () => {
    renderEditor({ op: 'notEquals' });
    expect((screen.getByLabelText('expr') as HTMLInputElement).value).toBe('');
    expect((screen.getByPlaceholderText('比较值') as HTMLInputElement).value).toBe('');
  });

  it('切换到不等于：比较值仍显示', () => {
    const { onChange } = renderEditor({ expr: '', op: 'equals', value: '' });
    clickOption('不等于');
    expect(onChange).toHaveBeenLastCalledWith({ expr: '', op: 'notEquals', value: '' });
    expect(screen.getByPlaceholderText('比较值')).toBeInTheDocument();
  });

  it('移除条件 → undefined', () => {
    const { onChange } = renderEditor({ expr: '', op: 'equals', value: '' });
    fireEvent.click(screen.getByRole('button', { name: /移除条件/ }));
    expect(onChange).toHaveBeenCalledWith(undefined);
  });

  it('rootsOf：sectionKey trim 索引（缺省/子树均可解析不炸）', () => {
    renderEditor({ expr: '', op: 'equals', value: '' });
    // mock 的表达式输入 mousedown 即触发 rootsOf('probe')——未知变量
    // 返回空路径数组（nodeByVar 无命中），不应抛错
    fireEvent.mouseDown(screen.getByLabelText('expr'));
    expect(screen.getByLabelText('expr')).toBeInTheDocument();
  });
});
