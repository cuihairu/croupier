/** 组合页编辑器 V5 T5.1：组件变量名模型——语义命名、自动去重、改名引用同步。
 *
 * 变量名即发布 spec 的区块 key（设计 D1），存储于 `props.sectionKey`
 * （编译器/compiler 已原生支持声明 key 固化与回读）。
 * 详见 docs/dashboard/composite-editor-v5-design.md §3。
 */
import type { ComponentType, PageNode } from './model';

/** 变量名格式：camelCase、ASCII、字母开头（要进表达式语法，禁止中文/点号/空格）。 */
export const VAR_NAME_RE = /^[a-z][a-zA-Z0-9]*$/;

export function isValidVarName(name: string): boolean {
  return VAR_NAME_RE.test(name);
}

/** 非字母数字切段 → camelCase；非 ASCII 字母开头（如中文标题）返回 ''。 */
export function camelize(input: string): string {
  const parts = input.split(/[^a-zA-Z0-9]+/).filter(Boolean);
  if (!parts.length) return '';
  const first = parts[0];
  if (!/^[a-zA-Z]/.test(first)) return '';
  const head = first.charAt(0).toLowerCase() + first.slice(1);
  const tail = parts
    .slice(1)
    .map((p) => (/^[a-zA-Z]/.test(p) ? p.charAt(0).toUpperCase() + p.slice(1) : p))
    .join('');
  const out = head + tail;
  return VAR_NAME_RE.test(out) ? out : '';
}

/** 命名来源：组件类型 + 可选函数契约 id / 标题。 */
export type VarNameSource = {
  type: ComponentType;
  functionId?: string;
  title?: string;
};

/** 函数组件类型后缀（player.list 表格 → playerListTable）。 */
const FN_TYPE_SUFFIX: Partial<Record<ComponentType, string>> = {
  fnTable: 'Table',
  fnForm: 'Form',
  fnFields: 'Fields',
};

/** 语义基名（§3.1，不含去重后缀）。 */
export function baseVarName(src: VarNameSource): string {
  const suffix = FN_TYPE_SUFFIX[src.type];
  if (suffix) {
    const fid = String(src.functionId ?? '').trim();
    if (fid) {
      const [resource, ...opParts] = fid.split('.');
      const opCamel = camelize(opParts.join(' '));
      const op = opCamel ? opCamel.charAt(0).toUpperCase() + opCamel.slice(1) : '';
      const base = `${camelize(resource)}${op}${suffix}`;
      if (VAR_NAME_RE.test(base)) return base;
    }
    // 无函数绑定的回退名
    return suffix.charAt(0).toLowerCase() + suffix.slice(1);
  }
  switch (src.type) {
    case 'staticForm': {
      const t = camelize(String(src.title ?? ''));
      return t ? `${t}Form` : 'filterForm';
    }
    case 'modal': {
      const t = camelize(String(src.title ?? ''));
      return t ? `${t}Modal` : 'modal';
    }
    case 'button': {
      const t = camelize(String(src.title ?? ''));
      return t ? `${t}Button` : 'button';
    }
    case 'container':
      return 'container';
    case 'text':
      return 'text';
    default:
      return 'node';
  }
}

/** 收集页面树中已占用的变量名（props.sectionKey）。 */
export function collectVarNames(nodes: PageNode[]): Set<string> {
  const out = new Set<string>();
  const walk = (list: PageNode[]) => {
    for (const n of list) {
      const declared = typeof n.props.sectionKey === 'string' ? n.props.sectionKey.trim() : '';
      if (declared) out.add(declared);
      if (n.children) walk(n.children);
    }
  };
  walk(nodes);
  return out;
}

/** 生成不重复变量名：base 冲突时追加最小可用数字后缀（base2、base3…）。 */
export function generateVarName(src: VarNameSource, existing: Set<string>): string {
  const base = baseVarName(src);
  if (!existing.has(base)) return base;
  for (let i = 2; ; i += 1) {
    const candidate = `${base}${i}`;
    if (!existing.has(candidate)) return candidate;
  }
}

/** 标识符词边界安全的变量名替换（不重写 oldName2 / xoldName 这类更长标识符）。 */
function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * 树内字符串叶子重写：
 * - `{{ }}` 表达式段内的变量引用（词边界安全）；
 * - 整串裸引用 `oldName` 或 `oldName.路径`（遗留链参数「key.字段」形态）。
 * 其余文本原样保留。
 */
export function rewriteVarRefsInString(s: string, oldName: string, newName: string): string {
  const esc = escapeRegExp(oldName);
  const token = new RegExp(`(?<![A-Za-z0-9_$])${esc}(?![A-Za-z0-9_$])`, 'g');
  let out = s.replace(/\{\{[^}]*\}\}/g, (seg) => seg.replace(token, newName));
  // 裸引用：整串恰为 oldName 或以 oldName. 开头
  if (!/\{\{/.test(out)) {
    out = out.replace(new RegExp(`^${esc}(?=\\.|$)`), newName);
  }
  return out;
}

/** 递归重写 props 中的全部字符串叶子（数组/对象内嵌套同样处理）。 */
function rewriteValue(value: unknown, oldName: string, newName: string): unknown {
  if (typeof value === 'string') return rewriteVarRefsInString(value, oldName, newName);
  if (Array.isArray(value)) return value.map((v) => rewriteValue(v, oldName, newName));
  if (value && typeof value === 'object') {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      out[k] = rewriteValue(v, oldName, newName);
    }
    return out;
  }
  return value;
}

/**
 * 改名同步（纯函数）：节点自身变量名 + 全树表达式/裸引用一并重写。
 * newName 非法或与现有变量冲突时原样返回（调用方负责先校验并提示）。
 */
export function renameVariable(nodes: PageNode[], oldName: string, newName: string): PageNode[] {
  if (!isValidVarName(newName) || oldName === newName) return nodes;
  const existing = collectVarNames(nodes);
  existing.delete(oldName);
  if (existing.has(newName)) return nodes;
  const walk = (list: PageNode[]): PageNode[] =>
    list.map((n) => {
      const props = rewriteValue(n.props, oldName, newName) as PageNode['props'];
      const renamed =
        typeof props.sectionKey === 'string' && props.sectionKey === oldName
          ? { ...props, sectionKey: newName }
          : props;
      return {
        ...n,
        props: renamed,
        ...(n.children ? { children: walk(n.children) } : {}),
      };
    });
  return walk(nodes);
}

/**
 * 落树命名（拖入/模板实例化时调用，传入**新增子树**）：
 * - 未命名的节点按 §3.1 生成语义变量名（含基础组件——可作动作目标，进补全列表）；
 * - 已命名但与页面现有变量冲突的节点重新生成，且子树内部对该旧名的引用
 *   一并重写（模板内部引用随树整体重写，§9）；
 * - 已命名且无冲突的保留（回读旧页面场景）。
 * 实现：冲突节点先记录（from→to）并保持旧 key，最后统一经 renameVariable
 * 翻转——它同时重写子树内引用与节点自身 key（先改 key 会让 renameVariable
 * 误判 newName 被占用而拒绝重写）。
 * 注意：只对新增/冲突子树调用——不要传整棵页面树。existing 会原地累加新名。
 */
export function assignVarNames(nodes: PageNode[], existing: Set<string>): PageNode[] {
  const renames: Array<{ from: string; to: string }> = [];
  const resolve = (list: PageNode[]): PageNode[] =>
    list.map((n) => {
      const declared = typeof n.props.sectionKey === 'string' ? n.props.sectionKey.trim() : '';
      let props = n.props;
      if (!declared || existing.has(declared)) {
        const title =
          typeof n.props.title === 'string'
            ? n.props.title
            : typeof n.props.content === 'string'
              ? n.props.content
              : undefined;
        const name = generateVarName(
          {
            type: n.type,
            functionId: typeof n.props.functionId === 'string' ? n.props.functionId : undefined,
            title,
          },
          existing,
        );
        existing.add(name);
        if (declared) {
          // 冲突节点：保持旧 key，记录改名（引用重写在统一阶段执行）
          renames.push({ from: declared, to: name });
        } else {
          // 全新节点：直接落新名（无旧引用需重写）
          props = { ...n.props, sectionKey: name };
        }
      } else {
        existing.add(declared);
      }
      const children = n.children ? resolve(n.children) : undefined;
      return children && children !== n.children ? { ...n, props, children } : { ...n, props };
    });
  let out = resolve(nodes);
  for (const { from, to } of renames) {
    out = renameVariable(out, from, to);
  }
  return out;
}
