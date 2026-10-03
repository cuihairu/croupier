/**
 * 开放范围统计纯函数（computeAssignmentScopes）：白名单 map → 逐函数
 * open/total 计数。语义对齐后端执行闸门 EnsureFunctionAssigned：
 * - undefined（拉取失败）→ undefined（未知态）；
 * - 空 map（从未保存白名单）→ total=0（默认开放）；
 * - 重复函数 ID 跨环境累加；非数组/null 值安全跳过。
 */
import { computeAssignmentScopes } from '../assignmentScope';

describe('computeAssignmentScopes', () => {
  it('undefined 入参（拉取失败）→ undefined', () => {
    expect(computeAssignmentScopes(undefined)).toBeUndefined();
  });

  it('空 map → total=0、openByFunction 空（默认开放）', () => {
    expect(computeAssignmentScopes({})).toEqual({ total: 0, openByFunction: new Map() });
  });

  it('跨环境累加 open、total=环境键数；不在白名单的函数不出现在索引', () => {
    const idx = computeAssignmentScopes({
      'demo|prod': ['fn.a', 'fn.b'],
      'demo|dev': ['fn.a'],
      'demo|test': [],
    });
    expect(idx?.total).toBe(3);
    expect(idx?.openByFunction.get('fn.a')).toBe(2);
    expect(idx?.openByFunction.get('fn.b')).toBe(1);
    expect(idx?.openByFunction.has('fn.c')).toBe(false);
  });

  it('null/空串条目安全跳过（旧版化石值防御）', () => {
    const idx = computeAssignmentScopes({
      'demo|prod': [null as unknown as string, '', 'fn.a'],
    });
    expect(idx?.total).toBe(1);
    expect(idx?.openByFunction.get('fn.a')).toBe(1);
    expect(idx?.openByFunction.size).toBe(1);
  });

  it('值非数组（异常 payload）→ 跳过该键不崩溃', () => {
    const idx = computeAssignmentScopes({
      'demo|prod': 'fn.a' as unknown as string[],
      'demo|dev': ['fn.b'],
    });
    expect(idx?.total).toBe(2);
    expect(idx?.openByFunction.get('fn.b')).toBe(1);
    expect(idx?.openByFunction.has('fn.a')).toBe(false);
  });
});
