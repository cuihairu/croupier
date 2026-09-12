/** U10 条件求值共享模块覆盖：valueAtPointer（JSON Pointer 语义）、
 * matchesCondition（字段级原语义回归）、sectionVisible（区块级 key+path
 * 寻址、key 缺失回落、全部/任一嵌套组合）。 */
import { matchesCondition, sectionVisible, valueAtPointer } from '../sectionCondition';
import type { ConditionSpec, FormValues } from '@/types/dashboard';

describe('valueAtPointer（JSON Pointer 取值）', () => {
  const doc = {
    mode: 'advanced',
    total: 42,
    flags: { 'a/b': 1, 'c~d': 2 },
    items: [{ uid: 'u1' }, { uid: 'u2' }],
  };

  it('对象取值 / ~0 ~1 转义 / 数组整数下标', () => {
    expect(valueAtPointer(doc, '/mode')).toBe('advanced');
    expect(valueAtPointer(doc, '/total')).toBe(42);
    expect(valueAtPointer(doc, '/flags/a~1b')).toBe(1);
    expect(valueAtPointer(doc, '/flags/c~0d')).toBe(2);
    expect(valueAtPointer(doc, '/items/1/uid')).toBe('u2');
  });

  it('越界/类型不符/缺属性 → undefined；非 / 开头 → undefined', () => {
    expect(valueAtPointer(doc, '/items/5')).toBeUndefined();
    expect(valueAtPointer(doc, '/mode/inner')).toBeUndefined();
    expect(valueAtPointer(doc, '/ghost')).toBeUndefined();
    expect(valueAtPointer(doc, 'mode')).toBeUndefined();
    expect(valueAtPointer(undefined, '/mode')).toBeUndefined();
  });
});

describe('matchesCondition（字段级原语义回归）', () => {
  const values: FormValues = { mode: 'advanced', count: 3 };

  it('equals/notEquals/exists 在表单 values 上按相对路径求值', () => {
    const eq: ConditionSpec = { kind: 'equals', path: '/mode', value: 'advanced' };
    const ne: ConditionSpec = { kind: 'notEquals', path: '/mode', value: 'basic' };
    const ex: ConditionSpec = { kind: 'exists', path: '/count' };
    expect(matchesCondition(eq, values)).toBe(true);
    expect(matchesCondition(ne, values)).toBe(true);
    expect(matchesCondition(ex, values)).toBe(true);
    // 值类型不同（字符串 '3' ≠ 数字 3）
    expect(matchesCondition({ kind: 'equals', path: '/count', value: '3' }, values)).toBe(false);
  });

  it('undefined 条件恒真（无条件=显示）', () => {
    expect(matchesCondition(undefined, values)).toBe(true);
  });
});

describe('sectionVisible（区块级条件）', () => {
  const results = {
    filterForm: { data: { mode: 'advanced' }, values: { mode: 'advanced' } },
    playerTable: { data: { total: 7, items: [] } },
  };

  it('key+path 在 results[key] 容器上取值（values/data 段）', () => {
    const c1: ConditionSpec = {
      kind: 'equals',
      key: 'filterForm',
      path: '/values/mode',
      value: 'advanced',
    };
    expect(sectionVisible(c1, results)).toBe(true);
    const c2: ConditionSpec = {
      kind: 'exists',
      key: 'playerTable',
      path: '/data/total',
    };
    expect(sectionVisible(c2, results)).toBe(true);
  });

  it('key 缺失（旧数据/字段级误用）→ 条件不成立', () => {
    const c: ConditionSpec = { kind: 'exists', path: '/mode' };
    expect(sectionVisible(c, results)).toBe(false);
  });

  it('引用不存在的区块或路径 miss → 条件不成立', () => {
    expect(sectionVisible({ kind: 'exists', key: 'ghost', path: '/data/x' }, results)).toBe(false);
    expect(
      sectionVisible(
        { kind: 'equals', key: 'filterForm', path: '/values/env', value: 'prod' },
        results,
      ),
    ).toBe(false);
  });

  it('全部/任一嵌套组合求值', () => {
    const all: ConditionSpec = {
      kind: 'all',
      conditions: [
        { kind: 'equals', key: 'filterForm', path: '/values/mode', value: 'advanced' },
        { kind: 'exists', key: 'playerTable', path: '/data/total' },
      ],
    };
    expect(sectionVisible(all, results)).toBe(true);
    const anyOf: ConditionSpec = {
      kind: 'any',
      conditions: [
        { kind: 'equals', key: 'filterForm', path: '/values/mode', value: 'basic' },
        { kind: 'exists', key: 'playerTable', path: '/data/total' },
      ],
    };
    expect(sectionVisible(anyOf, results)).toBe(true);
    expect(
      sectionVisible(
        {
          kind: 'all',
          conditions: [
            { kind: 'equals', key: 'filterForm', path: '/values/mode', value: 'basic' },
            { kind: 'exists', key: 'playerTable', path: '/data/total' },
          ],
        },
        results,
      ),
    ).toBe(false);
  });

  it('undefined 条件恒真；区块无状态（null）不成立', () => {
    expect(sectionVisible(undefined, results)).toBe(true);
    expect(sectionVisible({ kind: 'exists', key: 'empty', path: '/data/x' }, { empty: null })).toBe(
      false,
    );
  });
});
