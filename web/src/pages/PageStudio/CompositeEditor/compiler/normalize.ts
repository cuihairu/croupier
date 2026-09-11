import { getIntl } from '@umijs/max';
import {
  isSingleExpression,
  parseExpression,
  ROW_VARIABLE,
} from '@/components/PageRenderer/expression';

// 纯函数模块无法 useIntl：经 getIntl 求值诊断文案（先例 services/api/bugs.ts）
const intl = getIntl();

/** 行操作参数归一（编译侧独立步骤）。 */
/**
 * 行操作参数归一（V5 §6）：{{row.字段}} → row.字段（沿用现状 row.字段 语义）；
 * 行上下文之外的变量表达式不被运行时支持（行操作参数只读当前行）——
 * 警告并按字面量保留，不静默丢弃。
 */
export function normalizeRowActionParams(
  params: Record<string, string> | undefined,
  tableTitle: unknown,
  sectionKeys: Set<string>,
  warnings: string[],
): Record<string, string> | undefined {
  if (!params || typeof params !== 'object') return undefined;
  const out: Record<string, string> = {};
  for (const [param, value] of Object.entries(params)) {
    if (typeof value === 'string' && isSingleExpression(value)) {
      const parsed = parseExpression(value, sectionKeys);
      if (parsed.ok && parsed.ref.variable === ROW_VARIABLE) {
        const [head, ...rest] = parsed.ref.path;
        if (rest.length === 0) {
          out[param] = `row.${String(head)}`;
        } else {
          // 嵌套行路径（{{row.a.b}}）：发布端只支持单段 row.字段，多段求值恒 undefined
          warnings.push(
            intl.formatMessage(
              {
                id: 'pages.pageStudio.compiler.warning.rowActionParamNestedPath',
                defaultMessage:
                  '表格「{title}」行操作参数「{param}」的嵌套字段「{path}」发布后无法求值，已按字面量保存',
              },
              {
                title: String(tableTitle ?? ''),
                param,
                path: parsed.ref.path.map(String).join('.'),
              },
            ),
          );
          out[param] = value;
        }
        continue;
      }
      warnings.push(
        intl.formatMessage(
          {
            id: 'pages.pageStudio.compiler.warning.rowActionParamRowOnly',
            // defaultMessage 里字面量 {{ }} 按 ICU 语法转义为 '{{' '}}'（渲染结果与原文一致）
            defaultMessage:
              "表格「{title}」行操作参数「{param}」的表达式「{value}」仅支持 '{{'row.字段'}}'，已按字面量保存",
          },
          { title: String(tableTitle ?? ''), param, value },
        ),
      );
    }
    out[param] = String(value);
  }
  return Object.keys(out).length ? out : undefined;
}
