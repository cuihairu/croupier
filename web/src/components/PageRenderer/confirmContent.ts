import { localizedText } from '@/utils/localizedText';
import type { FormFieldSpec, FormPresentationSpec } from '@/types/dashboard';

/** 确认框内容条目：title 为人工标题（字段展示 label > key 人性化），value 为
 * 枚举文案化后的展示值（enumOptions 命中用 label，数组逐项映射）。 */
export interface ConfirmEntry {
  key: string;
  title: string;
  value: string;
}

/** 参数名人性化：snake_case / kebab-case / camelCase → 空格分隔小写词组。
 * 确认框里只显示原始字段 key 是 D2 明确禁止的形态。 */
export function humanizeParamKey(key: string): string {
  return key
    .replace(/[_-]+/g, ' ')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .trim()
    .toLowerCase();
}

/** 字段展示标题：展示层 label（zh-CN）> key 人性化 > 原 key。 */
export function confirmFieldTitle(field: FormFieldSpec | undefined, key: string): string {
  const label = field ? localizedText(field.label, 'zh-CN', '') : '';
  return label || humanizeParamKey(key) || key;
}

/** 值展示：枚举命中映射为选项文案；数组逐项映射后以顿号连接；对象走 JSON。 */
export function confirmValueText(field: FormFieldSpec | undefined, value: unknown): string {
  if (value === undefined || value === null || value === '') return '-';
  const labelOf = (item: unknown): string | undefined => {
    const option = field?.enumOptions?.find((o) => o.value === String(item));
    return option ? localizedText(option.label, 'zh-CN', option.value) : undefined;
  };
  if (Array.isArray(value)) {
    return value.map((item) => labelOf(item) ?? String(item ?? '-')).join('、');
  }
  if (typeof value === 'object') return JSON.stringify(value);
  return labelOf(value) ?? String(value);
}

/** 确认框条目：params 按表单字段声明序排列（未在表单声明的参数按原序追加）。
 * 空值条目保留——「确认内容与实际提交一致」优先于视觉精简。 */
export function describeConfirmEntries(
  spec: FormPresentationSpec | undefined,
  params: Record<string, unknown> | undefined,
): ConfirmEntry[] {
  const entries = Object.entries(params ?? {}).map(([key, value]) => {
    const field = spec?.fields?.find((f) => f.key === key);
    return { key, title: confirmFieldTitle(field, key), value: confirmValueText(field, value) };
  });
  if (!spec?.fields?.length) return entries;
  const order = new Map(spec.fields.map((field, index) => [field.key, index]));
  return [...entries].sort(
    (a, b) =>
      (order.get(a.key) ?? Number.MAX_SAFE_INTEGER) - (order.get(b.key) ?? Number.MAX_SAFE_INTEGER),
  );
}
