/** V5 T5.4：ExpressionInput 补全候选与即时校验测试。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import ExpressionInput, { computeSuggestions, validateExpressionInput } from '../ExpressionInput';
import type { ExprVariable } from '../exprVariables';

const variables: ExprVariable[] = [
  { name: 'playerListTable', title: '玩家列表', kind: 'fnTable' },
  { name: 'filterForm', title: '筛选', kind: 'staticForm' },
];
const rootsOf = (v: string) =>
  v === 'playerListTable'
    ? [
        { segment: 'data', children: [{ segment: 'total' }] },
        { segment: 'selectedRow', children: [{ segment: 'uid' }, { segment: 'nickname' }] },
      ]
    : v === 'filterForm'
      ? [{ segment: 'values', children: [{ segment: 'keyword' }] }]
      : [];
const varNames = new Set(variables.map((v) => v.name));

describe('computeSuggestions', () => {
  it('输入 {{ 后列出全部变量（含标题）', () => {
    const out = computeSuggestions('{{', variables, rootsOf);
    expect(out.map((s) => s.label)).toEqual(['playerListTable', 'filterForm']);
    expect(out[0].insert).toBe('{{playerListTable');
    expect(out[0].hint).toContain('玩家列表');
  });

  it('变量名前缀过滤', () => {
    const out = computeSuggestions('{{player', variables, rootsOf);
    expect(out.map((s) => s.label)).toEqual(['playerListTable']);
  });

  it('选中变量后补全路径分支', () => {
    const out = computeSuggestions('{{playerListTable.', variables, rootsOf);
    expect(out.map((s) => s.label)).toEqual(['data', 'selectedRow']);
    expect(out[0].insert).toBe('{{playerListTable.data}}');
  });

  it('逐级深入：分支后补全 schema 字段', () => {
    const out = computeSuggestions('{{playerListTable.selectedRow.n', variables, rootsOf);
    expect(out.map((s) => s.label)).toEqual(['nickname']);
    expect(out[0].insert).toBe('{{playerListTable.selectedRow.nickname}}');
  });

  it('行上下文：rowFields 提供时 row 进候选并补全行字段', () => {
    const out = computeSuggestions('{{', variables, rootsOf, ['uid', 'nickname']);
    expect(out.map((s) => s.label)).toContain('row');
    const fields = computeSuggestions('{{row.', variables, rootsOf, ['uid', 'nickname']);
    expect(fields.map((s) => s.label)).toEqual(['uid', 'nickname']);
  });

  it('已闭合表达式/字面量无候选', () => {
    expect(computeSuggestions('{{playerListTable.data}}', variables, rootsOf)).toEqual([]);
    expect(computeSuggestions('plain', variables, rootsOf)).toEqual([]);
  });

  it('无标题变量：hint 回退 kind；深入无 children 节点后无候选（break）', () => {
    const bare: ExprVariable[] = [{ name: 'rawVar', kind: 'fnFields' }];
    const [s] = computeSuggestions('{{', bare, () => []);
    expect(s.hint).toBe('fnFields');

    // 走到无 children 的节点（hit?.children ?? []）后继续深入 → break → 空候选
    const flatRoots = () => [{ segment: 'bag', children: [{ segment: 'inner' }] }];
    const deep = computeSuggestions('{{rawVar.bag.inner.', bare, flatRoots);
    expect(deep).toEqual([]);
  });
});

describe('validateExpressionInput', () => {
  it('未知变量 → error', () => {
    const d = validateExpressionInput('{{nope.uid}}', varNames, undefined, rootsOf);
    expect(d?.level).toBe('error');
    expect(d?.message).toContain('未知变量');
  });

  it('语法非法 → error', () => {
    const d = validateExpressionInput('{{playerListTable..x}}', varNames, undefined, rootsOf);
    expect(d?.level).toBe('error');
  });

  it('路径段不在 schema → warning（不阻断）', () => {
    const d = validateExpressionInput(
      '{{playerListTable.selectedRow.noSuchField}}',
      varNames,
      undefined,
      rootsOf,
    );
    expect(d?.level).toBe('warning');
  });

  it('合法路径无诊断；字面量无诊断', () => {
    expect(
      validateExpressionInput('{{playerListTable.selectedRow.uid}}', varNames, undefined, rootsOf),
    ).toBeUndefined();
    expect(validateExpressionInput('hello', varNames, undefined, rootsOf)).toBeUndefined();
  });

  it('row 字段不在行 schema → warning；无 rowFields 时不诊断', () => {
    const d = validateExpressionInput('{{row.nope}}', varNames, ['uid'], rootsOf);
    expect(d?.level).toBe('warning');
    expect(validateExpressionInput('{{row.uid}}', varNames, undefined, rootsOf)).toBeUndefined();
  });

  it('row 多段路径/无字段路径：不校验深层结构，返回 undefined', () => {
    // path.length > 1：深层路径不做行字段校验（只校验第一段）
    expect(
      validateExpressionInput('{{row.uid.meta.x}}', varNames, ['uid'], rootsOf),
    ).toBeUndefined();
    // 无字段（裸 row）：field 空串，不构成 warning
    expect(validateExpressionInput('{{row}}', varNames, ['uid'], rootsOf)).toBeUndefined();
  });

  it('路径中间段缺失后无候选（nodes 空）：不警告（schema 未知不阻断）', () => {
    // 走到 data（无 children）后继续找不到段 → nodes.length === 0 → undefined
    expect(
      validateExpressionInput('{{playerListTable.data.total.x}}', varNames, undefined, rootsOf),
    ).toBeUndefined();
  });

  it('row 开头但语法非法且无行上下文：不诊断（可能在行操作里）', () => {
    expect(validateExpressionInput('{{row..x}}', varNames, undefined, rootsOf)).toBeUndefined();
    expect(validateExpressionInput('{{row..x}}', varNames, [], rootsOf)).toBeUndefined();
  });

  it('数组下标段（number）在 schema 树中不匹配字符串段 → warning', () => {
    const d = validateExpressionInput(
      '{{playerListTable.data[0].x}}',
      varNames,
      undefined,
      rootsOf,
    );
    expect(d?.level).toBe('warning');
  });
});

describe('ExpressionInput 组件', () => {
  it('未知变量显示红标错误信息', () => {
    render(
      <ExpressionInput
        value="{{unknownVar.x}}"
        onChange={jest.fn()}
        variables={variables}
        rootsOf={rootsOf}
      />,
    );
    expect(screen.getByText(/未知变量/)).toBeInTheDocument();
  });

  it('字面量不显示诊断', () => {
    render(
      <ExpressionInput value="abc" onChange={jest.fn()} variables={variables} rootsOf={rootsOf} />,
    );
    expect(screen.queryByText(/未知变量/)).toBeNull();
  });

  it('输入变化回传', () => {
    const onChange = jest.fn();
    render(
      <ExpressionInput value="" onChange={onChange} variables={variables} rootsOf={rootsOf} />,
    );
    fireEvent.change(document.querySelector('input')!, { target: { value: '{{filterForm' } });
    expect(onChange).toHaveBeenCalledWith('{{filterForm');
  });

  it('warning 形态：⚠ 提示与黄色诊断文本', () => {
    render(
      <ExpressionInput
        value="{{playerListTable.selectedRow.noSuchField}}"
        onChange={jest.fn()}
        variables={variables}
        rootsOf={rootsOf}
      />,
    );
    expect(screen.getByText('⚠')).toBeInTheDocument();
    expect(screen.getByText(/路径段/)).toBeInTheDocument();
  });

  it('聚焦后渲染补全下拉：分支节点带 ▸ 提示、叶子节点无提示', async () => {
    const { container } = render(
      <ExpressionInput
        value="{{playerListTable."
        onChange={jest.fn()}
        variables={variables}
        rootsOf={rootsOf}
      />,
    );
    fireEvent.focus(container.querySelector('input')!);
    // 补全候选渲染在 dropdown（挂 body）：data/selectedRow 均有子级 → ▸
    expect(await screen.findAllByText('data')).toHaveLength(1);
    expect(screen.getByText('selectedRow')).toBeInTheDocument();
    expect(screen.getAllByText('▸').length).toBeGreaterThanOrEqual(2);
    fireEvent.blur(container.querySelector('input')!);

    // 深入一层：data 下只有叶子 total（无 ▸）
    const { container: c2 } = render(
      <ExpressionInput
        value="{{playerListTable.data."
        onChange={jest.fn()}
        variables={variables}
        rootsOf={rootsOf}
      />,
    );
    fireEvent.focus(c2.querySelector('input')!);
    expect(await screen.findAllByText('total')).toHaveLength(1);
  });
});
