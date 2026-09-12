import type { ConditionSpec, FormValues, JSONValue } from '@/types/dashboard';

/** 受限条件求值（equals/notEquals/exists 叶子 + 全部/任一嵌套组合）——
 * SchemaFormRenderer 字段级 visibleWhen 与 CompositeRenderer 区块级
 * visibleWhen 共用一份实现（U10 抽取，语义单一来源）。 */

/** JSON Pointer 取值（~0/~1 转义；容器为数组时按整数下标）。 */
export function valueAtPointer(
  value: JSONValue | undefined,
  pointer: string,
): JSONValue | undefined {
  if (value === undefined || !pointer.startsWith('/')) return undefined;
  let current = value;
  for (const token of pointer.slice(1).split('/')) {
    const key = token.replace(/~1/g, '/').replace(/~0/g, '~');
    if (Array.isArray(current)) {
      const index = Number(key);
      if (!Number.isInteger(index) || index < 0 || index >= current.length) return undefined;
      current = current[index];
      continue;
    }
    if (
      typeof current !== 'object' ||
      current === null ||
      !Object.prototype.hasOwnProperty.call(current, key)
    ) {
      return undefined;
    }
    current = (current as Record<string, JSONValue>)[key];
  }
  return current;
}

function sameJsonValue(left: JSONValue | undefined, right: JSONValue): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

type LeafCondition = Extract<ConditionSpec, { path: string }>;

/** 按叶子取值函数求值（resolve 决定数据源——字段级=表单 values，
 * 区块级=页面状态容器，见下）。 */
function matchesConditionWith(
  condition: ConditionSpec | undefined,
  resolve: (leaf: LeafCondition) => JSONValue | undefined,
): boolean {
  if (!condition) return true;
  switch (condition.kind) {
    case 'equals':
      return sameJsonValue(resolve(condition), condition.value);
    case 'notEquals':
      return !sameJsonValue(resolve(condition), condition.value);
    case 'exists':
      return resolve(condition) !== undefined;
    case 'all':
      return condition.conditions.every((item) => matchesConditionWith(item, resolve));
    case 'any':
      return condition.conditions.some((item) => matchesConditionWith(item, resolve));
  }
}

/** 字段级条件（FormPresentation 字段 visibleWhen 原语义）：
 * 表单内相对路径，在表单 values 上取值。 */
export function matchesCondition(
  condition: ConditionSpec | undefined,
  values: FormValues,
): boolean {
  return matchesConditionWith(condition, (leaf) => valueAtPointer(values, leaf.path));
}

/** 区块运行时状态（与 page_state 同构）：键=区块 key，值含
 * data（函数输出）/ selectedRow / selectedRows / values（表单当前值）。 */
export type SectionRuntimeState = Record<string, unknown>;

/** 区块级条件（CompositeSection.visibleWhen）：叶子 key=来源区块 key，
 * 在 results[key] 容器上按 path 取值（如 /values/mode、/data/total）。
 * key 缺失（旧数据/字段级误用）视为不可求值 → 条件不成立。 */
export function sectionVisible(
  condition: ConditionSpec | undefined,
  results: SectionRuntimeState,
): boolean {
  return matchesConditionWith(condition, (leaf) => {
    if (!leaf.key) return undefined;
    const state = results[leaf.key];
    if (state === undefined || state === null) return undefined;
    return valueAtPointer(state as JSONValue, leaf.path);
  });
}
