/** 组合页 V5 T5.4：ExpressionInput——表达式输入 + 两级补全 + 即时校验。
 *
 * 设计 §5.1：
 * - 输入 `{{` 后弹出变量补全（变量名 + 标题 + 类型）；选中变量后继续补全
 *   路径（分支 → schema 字段，逐级深入）；
 * - 即时校验：未知变量/语法非法 → error（红框）；路径字段不在 schema →
 *   warning（黄框，schema 可能不完整，不阻断）；
 * - 不以 `{{` 开头 = 字面量（无校验）。
 * 补全按「文本尾部正在编辑的 token」计算（追加式插入，光标居尾的常规用法）。
 */
import React, { useMemo, useState } from 'react';
import { AutoComplete, Input, Typography } from 'antd';
import {
  isSingleExpression,
  matchVariable,
  parseExpression,
  ROW_VARIABLE,
} from '@/components/PageRenderer/expression';
import type { ExprPathNode, ExprVariable } from './exprVariables';

const { Text } = Typography;

export type ExpressionSuggestion = {
  /** 插入的补全文本（替换尾部未完成 token）。 */
  insert: string;
  /** 展示文本。 */
  label: string;
  /** 展示副文本（标题/类型）。 */
  hint?: string;
};

/** 计算文本尾部 token 的补全候选。 */
export function computeSuggestions(
  text: string,
  variables: ExprVariable[],
  rootsOf: (variable: string) => ExprPathNode[],
  rowFields?: string[],
): ExpressionSuggestion[] {
  // 未闭合的 {{ —— 正在输入表达式
  const open = text.lastIndexOf('{{');
  if (open < 0 || text.indexOf('}}', open) >= 0) return [];
  const inner = text.slice(open + 2);
  if (inner.includes('}}')) return [];

  const names = new Set(variables.map((v) => v.name));
  const known = [...names, ...(rowFields?.length ? [ROW_VARIABLE] : [])];
  const variable = matchVariable(inner, new Set(known));
  if (!variable) {
    // 还在敲变量名：按前缀过滤变量列表
    const prefix = inner.toLowerCase();
    return variables
      .filter((v) => !prefix || v.name.toLowerCase().startsWith(prefix))
      .slice(0, 12)
      .concat(
        rowFields?.length && ROW_VARIABLE.startsWith(prefix)
          ? [{ name: ROW_VARIABLE, kind: 'row' as const, title: '当前行（行操作上下文）' }]
          : [],
      )
      .map((v) => ({
        insert: `{{${v.name}`,
        label: v.name,
        hint: v.title ? `${v.title}（${v.kind}）` : v.kind,
      }));
  }

  // 变量已确定：补全路径。按已敲的完整段走树，尾部未完成段过滤。
  const rest = inner.slice(variable.length);
  let nodes: ExprPathNode[] =
    variable === ROW_VARIABLE ? (rowFields ?? []).map((f) => ({ segment: f })) : rootsOf(variable);
  const tokens = rest.split('.');
  const partial = tokens[tokens.length - 1] ?? '';
  const walked = tokens.slice(0, -1).filter((t) => t.trim() !== '');
  for (const seg of walked) {
    const clean = seg.replace(/\[\d+\]/g, '');
    const hit = nodes.find((n) => n.segment === clean);
    nodes = hit?.children ?? [];
    if (!nodes.length) break;
  }
  const lower = partial.toLowerCase();
  return nodes
    .filter((n) => !lower || n.segment.toLowerCase().startsWith(lower))
    .slice(0, 16)
    .map((n) => {
      const head = tokens.slice(0, -1).filter(Boolean).join('.');
      const insert = `{{${variable}${head ? `.${head}` : ''}.${n.segment}}}`;
      return { insert, label: n.segment, hint: n.children?.length ? '▸' : undefined };
    });
}

/** 校验：error = 语法/未知变量；warning = 路径字段不在 schema。 */
export function validateExpressionInput(
  text: string,
  variables: Set<string>,
  rowFields: string[] | undefined,
  rootsOf: (variable: string) => ExprPathNode[],
): { level: 'error' | 'warning'; message: string } | undefined {
  if (!isSingleExpression(text)) return undefined;
  const parsed = parseExpression(text, variables);
  if (!parsed.ok) {
    // row 上下文未提供时不算未知变量错误（可能在行操作里）
    if (text.trim().startsWith(`{{${ROW_VARIABLE}.`) && !rowFields?.length) return undefined;
    return { level: 'error', message: parsed.error };
  }
  const { variable, path } = parsed.ref;
  if (variable === ROW_VARIABLE) {
    if (!rowFields?.length) return undefined;
    const field = String(path[0] ?? '');
    if (path.length === 1 && field && !rowFields.includes(field)) {
      return { level: 'warning', message: `行字段「${field}」不在当前表格输出 schema 中` };
    }
    return undefined;
  }
  // 路径走树：缺段 → warning（schema 可能不完整，不阻断保存）
  let nodes = rootsOf(variable);
  for (const seg of path) {
    const name = typeof seg === 'number' ? '' : seg;
    const hit = nodes.find((n) => n.segment === name);
    if (!hit) {
      return nodes.length
        ? { level: 'warning', message: `路径段「${name}」不在 ${variable} 的 schema 候选中` }
        : undefined;
    }
    nodes = hit.children ?? [];
  }
  return undefined;
}

const ExpressionInput: React.FC<{
  value: string;
  onChange: (v: string) => void;
  /** 页面变量列表（树内已命名组件）。 */
  variables: ExprVariable[];
  /** 变量 → 路径树根。 */
  rootsOf: (variable: string) => ExprPathNode[];
  /** 行上下文字段（行操作/行事件编辑时传入）。 */
  rowFields?: string[];
  placeholder?: string;
  style?: React.CSSProperties;
  size?: 'small' | 'middle';
}> = ({ value, onChange, variables, rootsOf, rowFields, placeholder, style, size }) => {
  const [focused, setFocused] = useState(false);
  const nameSet = useMemo(() => new Set(variables.map((v) => v.name)), [variables]);
  const suggestions = useMemo(
    () => computeSuggestions(value, variables, rootsOf, rowFields),
    [value, variables, rootsOf, rowFields],
  );
  const diagnostic = useMemo(
    () => validateExpressionInput(value, nameSet, rowFields, rootsOf),
    [value, nameSet, rowFields, rootsOf],
  );

  return (
    <div style={style}>
      <AutoComplete
        size={size}
        style={{ width: '100%' }}
        open={focused && suggestions.length > 0}
        value={value}
        onSearch={(text) => onChange(text)}
        onChange={(text) => onChange(text)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        options={suggestions.map((s) => ({
          value: s.insert,
          label: (
            <span>
              <Text code style={{ fontSize: 12 }}>
                {s.label}
              </Text>
              {s.hint && (
                <Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
                  {s.hint}
                </Text>
              )}
            </span>
          ),
        }))}
      >
        <Input
          size={size}
          placeholder={placeholder ?? '字面量，或 {{ 选择变量 }}'}
          allowClear
          status={diagnostic?.level === 'error' ? 'error' : undefined}
          suffix={
            diagnostic ? (
              <Text
                type={diagnostic.level === 'error' ? 'danger' : 'warning'}
                style={{ fontSize: 11 }}
                title={diagnostic.message}
              >
                {diagnostic.level === 'error' ? '✕' : '⚠'}
              </Text>
            ) : undefined
          }
        />
      </AutoComplete>
      {diagnostic && (
        <Text
          type={diagnostic.level === 'error' ? 'danger' : 'warning'}
          style={{ fontSize: 11, display: 'block', marginTop: 2 }}
        >
          {diagnostic.message}
        </Text>
      )}
    </div>
  );
};

export default ExpressionInput;
