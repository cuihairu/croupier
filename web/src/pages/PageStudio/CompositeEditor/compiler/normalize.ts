import { getIntl } from '@umijs/max';
import {
  isSingleExpression,
  parseExpression,
  ROW_VARIABLE,
} from '@/components/PageRenderer/expression';
import type { PaginationSpec } from '@/types/dashboard';

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

const PAGINATION_MAX_SIZE = 200;

/** 服务端分页归一（D3，编译侧）：只有显式 enabled=true 才透传，其余形态
 * （缺失/关闭/非对象/数组）一律按关闭处理——组合页不推导分页，误启用会让
 * 不消费 page/pageSize 的函数全量返回，出现每页内容相同、页数按上界虚高。
 * defaultSize/pageSizes 按渲染端同一钳制口径（[1,200]）归一：空值与非法值
 * 丢弃，重复值去重，条数升序。
 */
export function normalizePaginationProp(raw: unknown): PaginationSpec | undefined {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return undefined;
  const cfg = raw as { enabled?: unknown; defaultSize?: unknown; pageSizes?: unknown };
  if (cfg.enabled !== true) return undefined;
  const clamp = (n: number): number => Math.min(PAGINATION_MAX_SIZE, Math.max(1, Math.floor(n)));
  const size = Number(cfg.defaultSize);
  const sizes = Array.isArray(cfg.pageSizes)
    ? cfg.pageSizes
        .map((v) => Number(v))
        .filter((v) => Number.isFinite(v) && v > 0)
        .map(clamp)
    : [];
  return {
    enabled: true,
    ...(Number.isFinite(size) && size > 0 ? { defaultSize: clamp(size) } : {}),
    ...(sizes.length > 0 ? { pageSizes: [...new Set(sizes)].sort((a, b) => a - b) } : {}),
  };
}
