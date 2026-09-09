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
});
