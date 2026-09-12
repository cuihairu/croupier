import { getIntl } from '@umijs/max';
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
        title: getIntl().formatMessage({
          id: 'pages.pageStudio.editor.component.fn.prop.functionId',
          defaultMessage: '函数（可换绑）',
        }),
        enum: options.map((o) => o.value),
        enumNames: options.map((o) => o.label),
        ...(fn ? { default: fn.id } : {}),
      },
      title: {
        type: 'string',
        title: getIntl().formatMessage({
          id: 'pages.pageStudio.editor.component.fn.prop.title',
          defaultMessage: '标题',
        }),
      },
      ...extra,
    },
  };
}

export function spanSchema() {
  return {
    type: 'integer',
    title: getIntl().formatMessage({
      id: 'pages.pageStudio.editor.component.fn.prop.span',
      defaultMessage: '宽度（1-24 栅格）',
    }),
    minimum: 4,
    maximum: 24,
    default: 24,
  };
}

/** U10 区块级条件显示（format:'condition' → PropsPanel 分桶渲染
 * ConditionEditor，不走 rjsf 默认对象表单）。 */
export function visibleWhenSchema() {
  return {
    type: 'object',
    format: 'condition',
    title: getIntl().formatMessage({
      id: 'pages.pageStudio.editor.component.visibleWhen.title',
      defaultMessage: '显示条件（按页面状态显隐本区块）',
    }),
  };
}

/** U9 refreshOn 级联失败策略：上游依赖执行失败时本区块的行为
 * （clear 清空数据 / keep 保留旧数据 / pause 停止级联并提示，缺省）。 */
export function cascadePolicySchema() {
  const t = getIntl().formatMessage;
  return {
    type: 'string',
    title: t({
      id: 'pages.pageStudio.editor.component.cascadePolicy.title',
      defaultMessage: '级联失败策略（上游失败时本区块）',
    }),
    enum: ['pause', 'clear', 'keep'],
    enumNames: [
      t({
        id: 'pages.pageStudio.editor.component.cascadePolicy.pause',
        defaultMessage: '暂停级联（默认，提示）',
      }),
      t({
        id: 'pages.pageStudio.editor.component.cascadePolicy.clear',
        defaultMessage: '清空数据',
      }),
      t({
        id: 'pages.pageStudio.editor.component.cascadePolicy.keep',
        defaultMessage: '保留旧数据',
      }),
    ],
  };
}
