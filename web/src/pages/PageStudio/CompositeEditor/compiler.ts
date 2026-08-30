import type { PageNode } from './model';
import { parseAction } from './actions';

/** 编译产物：与后端 CompositeSectionRequest 对齐（POST /versioning/pages/composite）。 */
export type CompiledSection = {
  /** 区块唯一 key（同函数多实例：fid、fid-2、fid-3…）；引用一律用 key。 */
  key: string;
  functionId: string;
  view: 'table' | 'fields' | 'form';
  title: string;
  span: number;
  autoRun: boolean;
  refreshOn?: string[];
  display?: 'inline' | 'dialog';
  rowActions?: CompiledAction[];
  toolbarActions?: CompiledAction[];
  onSuccessRefresh?: string[];
};

export type CompiledAction = {
  label: string;
  targetSection: string;
  params?: Record<string, string>;
  danger?: boolean;
};

export interface CompileResult {
  sections: CompiledSection[];
  warnings: string[];
}

const VIEW_MAP: Record<string, 'table' | 'fields' | 'form'> = {
  fnTable: 'table',
  fnFields: 'fields',
  fnForm: 'form',
};

/**
 * 编辑树 → 平铺 CompositeSection 列表。
 * 编译规则（V1）：
 * - fnTable/fnFields/fnForm → section（display=dialog 时不占栅格语义，仅弹窗）
 * - modal 容器 → 其唯一 fnForm 编译为 display=dialog 的 section
 * - 根级 button.onClick=openModal → 挂到前一个 fnTable 的 toolbarActions；
 *   页面无表格时警告忽略（V1 独立按钮依赖表格顶部）
 * - fnForm.onSuccessRefresh 动作 target → 目标节点 functionId
 * - text/container 不产生 section（container 子节点平铺；text 忽略并警告）
 */
export function compileTree(tree: PageNode[]): CompileResult {
  const sections: CompiledSection[] = [];
  const warnings: string[] = [];

  // 区块 key 分配：同函数多实例依次 fid、fid-2、fid-3……
  //（编辑器允许同一函数拖多个组件，发布侧按 key 唯一区分）。
  const nodeSectionKey = new Map<string, string>();
  const usedKeys = new Set<string>();
  const allocKey = (fid: string): string => {
    let key = fid;
    let i = 2;
    while (usedKeys.has(key)) key = `${fid}-${i++}`;
    usedKeys.add(key);
    return key;
  };
  const assignKeys = (nodes: PageNode[]) => {
    for (const n of nodes) {
      if (n.type === 'modal') {
        assignKeys(n.children ?? []);
        continue;
      }
      if (VIEW_MAP[n.type] && typeof n.props.functionId === 'string' && n.props.functionId) {
        nodeSectionKey.set(n.id, allocKey(String(n.props.functionId)));
      }
      if (n.children) assignKeys(n.children);
    }
  };
  assignKeys(tree);

  // 收集 modal 目标映射（modalId → 弹窗内 fnForm 的区块 key）
  const modalFn = new Map<string, string>();
  for (const m of tree.filter((n) => n.type === 'modal')) {
    const form = m.children?.find((c) => c.type === 'fnForm');
    const key = form ? nodeSectionKey.get(form.id) : undefined;
    if (key) modalFn.set(m.id, key);
    else warnings.push(`弹窗「${String(m.props.title ?? m.id)}」内没有函数表单，已忽略`);
  }

  /** 节点 → 区块 key（引用统一语义）。 */
  const sectionKeyOf = (n: PageNode): string | undefined => {
    if (n.type === 'modal') return modalFn.get(n.id);
    return nodeSectionKey.get(n.id);
  };

  const walk = (nodes: PageNode[]) => {
    for (const node of nodes) {
      if (node.type === 'text') {
        warnings.push(`文本「${String(node.props.content ?? '')}」不参与发布（V1）`);
        continue;
      }
      if (node.type === 'container') {
        walk(node.children ?? []);
        continue;
      }
      if (node.type === 'modal') {
        const form = node.children?.find((c) => c.type === 'fnForm');
        if (form) {
          emitFnSection(form, 'dialog');
        }
        continue;
      }
      if (node.type === 'button') {
        compileButton(node);
        continue;
      }
      if (VIEW_MAP[node.type]) {
        emitFnSection(node, node.props.display === 'dialog' ? 'dialog' : 'inline');
        continue;
      }
      warnings.push(`未知组件类型 ${node.type}，已忽略`);
    }
  };

  const emitFnSection = (node: PageNode, display: 'inline' | 'dialog') => {
    const fid = String(node.props.functionId ?? '');
    if (!fid) {
      warnings.push(`组件「${String(node.props.title ?? node.id)}」没有绑定函数，已忽略`);
      return;
    }
    const section: CompiledSection = {
      key: nodeSectionKey.get(node.id) ?? fid,
      functionId: fid,
      view: VIEW_MAP[node.type],
      title: String(node.props.title ?? fid),
      span: Number(node.props.span ?? 24) || 24,
      autoRun: node.props.autoRun === true,
      display,
    };
    // onSuccessRefresh：动作 target → 目标节点区块 key
    const act = parseAction(node.props.onSuccessRefresh);
    if (act?.kind === 'refreshNode') {
      const target = findNode(tree, act.target);
      const tkey = target && sectionKeyOf(target);
      if (tkey) section.onSuccessRefresh = [tkey];
      else warnings.push(`${fid} 的「成功后刷新」目标无效，已忽略`);
    }
    // 行操作（表格属性面板直接编辑，目标弹窗→函数 id）
    if (node.type === 'fnTable' && Array.isArray(node.props.rowActions)) {
      const ras: CompiledAction[] = [];
      for (const raw of node.props.rowActions as Array<Record<string, unknown>>) {
        const target = typeof raw.targetSection === 'string' ? raw.targetSection : '';
        const targetNode = findNode(tree, target);
        const sectionTarget = (targetNode && sectionKeyOf(targetNode)) ?? '';
        if (!sectionTarget) {
          warnings.push(`表格「${section.title}」有未配置目标的行操作，已忽略`);
          continue;
        }
        const ra: CompiledAction = {
          label: String(raw.label ?? ''),
          targetSection: sectionTarget,
          params: (raw.params as Record<string, string>) ?? undefined,
        };
        if (raw.danger === true) ra.danger = true;
        ras.push(ra);
      }
      if (ras.length) section.rowActions = ras;
    }
    sections.push(section);
  };

  const compileButton = (node: PageNode) => {
    const act = parseAction(node.props.onClick);
    if (!act) {
      warnings.push(`按钮「${String(node.props.title ?? '')}」没有配置动作，已忽略`);
      return;
    }
    if (act.kind !== 'openModal') {
      warnings.push(
        `按钮「${String(node.props.title ?? '')}」动作 ${act.kind} 不参与发布（V1 仅支持打开弹窗），已忽略`,
      );
      return;
    }
    const targetNode = findNode(tree, act.target);
    const targetKey = (targetNode && sectionKeyOf(targetNode)) ?? modalFn.get(act.target);
    if (!targetKey) {
      warnings.push(`按钮「${String(node.props.title ?? '')}」的弹窗目标无效，已忽略`);
      return;
    }
    // 挂到最近一个表格 section 的 toolbarActions
    const lastTable = [...sections].reverse().find((s) => s.view === 'table');
    if (!lastTable) {
      warnings.push(
        `按钮「${String(node.props.title ?? '')}」需放置在表格之后（V1 编译为表格顶部按钮），已忽略`,
      );
      return;
    }
    const ta: CompiledAction = {
      label: String(node.props.title ?? '操作'),
      targetSection: targetKey,
    };
    if (node.props.btnStyle === 'danger') ta.danger = true;
    lastTable.toolbarActions = [...(lastTable.toolbarActions ?? []), ta];
  };

  walk(tree);
  return { sections, warnings };
}

function findNode(nodes: PageNode[], id: string): PageNode | undefined {
  for (const n of nodes) {
    if (n.id === id) return n;
    if (n.children) {
      const hit = findNode(n.children, id);
      if (hit) return hit;
    }
  }
  return undefined;
}
