import type { FunctionDescriptor } from '@/services/api/functions';
import { localizedText } from '@/utils/localizedText';
import type { JSONSchema } from '@/types/dashboard';

/** functionId 只读展示 + 通用字段（标题/宽度/自动执行）的公共 schema 片段。 */
/** functionId 全量可换绑（当前 scope 所有函数）。 */
export function commonFnSchema(
  fn: FunctionDescriptor | undefined,
  allFns: FunctionDescriptor[],
  extra: Record<string, unknown> = {},
): JSONSchema {
  const pool = allFns ?? [];
  const options = (pool.length ? pool : fn ? [fn] : []).map((f) => {
    const summary = localizedText(f.summary, 'zh-CN');
    return { value: f.id, label: summary ? `${f.id}（${summary}）` : f.id };
  });
  return {
    type: 'object',
    properties: {
      functionId: {
        type: 'string',
        title: '函数（可换绑）',
        enum: options.map((o) => o.value),
        enumNames: options.map((o) => o.label),
        ...(fn ? { default: fn.id } : {}),
      },
      title: { type: 'string', title: '标题' },
      ...extra,
    },
  };
}

export function spanSchema() {
  return { type: 'integer', title: '宽度（1-24 栅格）', minimum: 4, maximum: 24, default: 24 };
}
