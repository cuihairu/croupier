/** 组合页 V5 T5.2：受限绑定表达式——解析、求值、JSON Pointer 互转（纯函数）。
 *
 * 文法（docs/dashboard/composite-editor-v5-design.md §4）：
 *   expression = variable , { segment }
 *   variable   = <页面变量集合最长前缀匹配> | "row"
 *   segment    = "." identifier | "[" integer "]"
 *
 * 不做 JS eval / 运算符 / 函数调用（V5 边界）。
 */

import { getIntl } from '@umijs/max';

/**
 * 错误消息本地化：纯函数模块无法 useIntl，在错误产生时经 getIntl 求值
 * （消费方 ExpressionInput 将 error 直接渲染为编辑器诊断）。umi 运行时外
 * （单测/异常）回退 defaultMessage，与 currentLocale() 的防御先例一致。
 */
function exprError(
  id: string,
  defaultMessage: string,
  values?: Record<string, string | number>,
): string {
  try {
    return getIntl().formatMessage({ id, defaultMessage }, values);
  } catch {
    return defaultMessage;
  }
}

/** 表达式引用：变量名 + 路径段（字符串属性 / 数字下标）。 */
export type ExprRef = {
  /** 变量名：页面区块变量（== 发布 spec 区块 key）或保留字 row（行上下文）。 */
  variable: string;
  path: Array<string | number>;
};

/** 解析结果：ok=false 时 error 为面向用户的可读原因。 */
export type ParseResult = { ok: true; ref: ExprRef } | { ok: false; error: string };

/** 行上下文保留变量（行操作 / 行点击 / 行选中事件内可用）。 */
export const ROW_VARIABLE = 'row';

const EXPR_RE = /^\{\{([\s\S]*)\}\}$/;
const IDENT_RE = /^[a-zA-Z_][a-zA-Z0-9_]*$/;

/** 是否为单表达式形态（整串就是一对 {{ }}；模板混排属 V5.1 边界，返回 false 按字面量处理）。 */
export function isSingleExpression(text: string): boolean {
  const m = EXPR_RE.exec(text.trim());
  return !!m && m[1].trim().length > 0;
}

/**
 * 变量名最长前缀匹配：inner 以某个已知变量名开头（后跟 `.`/`[` 或结束）时取最长者。
 * 旧页面 key 含点号（player.list）也能命中（§3.3 回读兼容）。
 */
export function matchVariable(inner: string, variables: Set<string>): string | undefined {
  let best: string | undefined;
  for (const name of variables) {
    if (!name || !inner.startsWith(name)) continue;
    const next = inner.charAt(name.length);
    if (next !== '' && next !== '.' && next !== '[') continue;
    if (!best || name.length > best.length) best = name;
  }
  return best;
}

/** 解析 `{{expr}}` 为 ExprRef；非单表达式形态 / 语法非法 / 未知变量返回 error。 */
export function parseExpression(text: string, variables: Set<string>): ParseResult {
  const trimmed = text.trim();
  const m = EXPR_RE.exec(trimmed);
  if (!m) {
    // defaultMessage 含字面量 {{ }}，按 ICU 语法转义（与 locale 键值一致）
    return {
      ok: false,
      error: exprError(
        'component.pageRenderer.expression.error.notExpression',
        "不是表达式（应以 '{{' 开头、'}}' 结尾）",
      ),
    };
  }
  const inner = m[1].trim();
  if (!inner) {
    return {
      ok: false,
      error: exprError('component.pageRenderer.expression.error.empty', '表达式为空'),
    };
  }

  let variable = matchVariable(inner, variables);
  if (
    !variable &&
    (inner === ROW_VARIABLE ||
      inner.startsWith(`${ROW_VARIABLE}.`) ||
      inner.startsWith(`${ROW_VARIABLE}[`))
  ) {
    variable = ROW_VARIABLE;
  }
  if (!variable) {
    const unknown = inner.split(/[.[\]]/)[0];
    return {
      ok: false,
      error: exprError(
        'component.pageRenderer.expression.error.unknownVariable',
        `未知变量：${unknown}`,
        { variable: unknown },
      ),
    };
  }

  const path: Array<string | number> = [];
  let i = variable.length;
  while (i < inner.length) {
    const ch = inner[i];
    if (ch === '.') {
      const start = i + 1;
      let j = start;
      while (j < inner.length && inner[j] !== '.' && inner[j] !== '[') j += 1;
      const seg = inner.slice(start, j);
      if (!IDENT_RE.test(seg)) {
        return {
          ok: false,
          error: exprError(
            'component.pageRenderer.expression.error.illegalSegment',
            `非法路径段「${seg}」`,
            { segment: seg },
          ),
        };
      }
      path.push(seg);
      i = j;
      continue;
    }
    if (ch === '[') {
      const end = inner.indexOf(']', i);
      if (end < 0) {
        return {
          ok: false,
          error: exprError(
            'component.pageRenderer.expression.error.arrayIndexUnterminated',
            '数组下标缺少 ]',
          ),
        };
      }
      const raw = inner.slice(i + 1, end);
      if (!/^\d+$/.test(raw)) {
        return {
          ok: false,
          error: exprError(
            'component.pageRenderer.expression.error.illegalIndex',
            `非法数组下标「${raw}」`,
            { index: raw },
          ),
        };
      }
      path.push(Number(raw));
      i = end + 1;
      continue;
    }
    return {
      ok: false,
      error: exprError(
        'component.pageRenderer.expression.error.illegalCharacter',
        `非法字符「${ch}」`,
        { char: ch },
      ),
    };
  }
  return { ok: true, ref: { variable, path } };
}

/** ExprRef → JSON Pointer（/data/items/0/total；~ 与 / 按 RFC6901 转义）。 */
export function pathToPointer(path: Array<string | number>): string {
  if (!path.length) return '';
  return '/' + path.map((seg) => String(seg).replace(/~/g, '~0').replace(/\//g, '~1')).join('/');
}

/** JSON Pointer → 路径段（纯数字段还原为下标；schema 字段名不会是纯数字，见设计 §6 注）。 */
export function pointerToPath(pointer: string): Array<string | number> {
  if (!pointer || pointer === '/') return [];
  return pointer
    .replace(/^\//, '')
    .split('/')
    .map((token) => {
      const seg = token.replace(/~1/g, '/').replace(/~0/g, '~');
      return /^\d+$/.test(seg) ? Number(seg) : seg;
    });
}

/** ExprRef → 展示用点路径文本（playerListTable.data.items[0].uid）。 */
export function refToText(ref: ExprRef): string {
  let out = `{{${ref.variable}`;
  for (const seg of ref.path) {
    out += typeof seg === 'number' ? `[${seg}]` : `.${seg}`;
  }
  return `${out}}}`;
}

/** 状态按 ExprRef 求值：中途任何一段缺失返回 undefined（不抛错）。 */
export function resolveRef(
  ref: ExprRef,
  state: Record<string, unknown>,
  ctx?: Record<string, unknown>,
): unknown {
  let current: unknown = ref.variable === ROW_VARIABLE ? ctx : state[ref.variable];
  for (const seg of ref.path) {
    if (current === null || current === undefined) return undefined;
    if (typeof seg === 'number') {
      if (!Array.isArray(current) || seg >= current.length) return undefined;
      current = current[seg];
      continue;
    }
    if (typeof current !== 'object' || Array.isArray(current)) return undefined;
    current = (current as Record<string, unknown>)[seg];
  }
  return current;
}

/** 绑定字符串求值：单表达式 → 求值；其余（字面量 / 模板混排）原样返回。 */
export function resolveExpression(
  text: string,
  state: Record<string, unknown>,
  ctx?: Record<string, unknown>,
  variables?: Set<string>,
): unknown {
  if (!isSingleExpression(text)) return text;
  const parsed = parseExpression(text, variables ?? new Set(Object.keys(state)));
  if (!parsed.ok) return text; // 求值期宽容：解析失败回退字面量（编辑期由 parseExpression 诊断拦截）
  return resolveRef(parsed.ref, state, ctx);
}
