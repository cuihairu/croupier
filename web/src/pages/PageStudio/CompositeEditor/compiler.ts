import type { PageNode } from './model';
import { parseAction } from './actions';

/** 编译产物：与后端 CompositeSectionRequest 对齐（POST /versioning/pages/composite）。 */
export type CompiledSection = {
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

/** modal 容器 → 其内 fnForm 的 functionId（弹窗动作目标引用）。 */
function modalTargetFunction(modal: PageNode): string | undefined {
  return modal.children?.find((c) => c.type === 'fnForm')?.props.functionId as string | undefined;
}

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

  // 收集 modal 目标映射（modalId → fnForm functionId）
  const modalFn = new Map<string, string>();
  for (const m of tree.filter((n) => n.type === 'modal')) {
    const fid = modalTargetFunction(m);
    if (fid) modalFn.set(m.id, fid);
    else warnings.push(`弹窗「${String(m.props.title ?? m.id)}」内没有函数表单，已忽略`);
  }

  const sectionFunctionId = (n: PageNode): string | undefined => {
    if (n.type === 'modal') return modalFn.get(n.id);
    const fid = n.props.functionId;
    return typeof fid === 'string' && fid ? fid : undefined;
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
      functionId: fid,
      view: VIEW_MAP[node.type],
      title: String(node.props.title ?? fid),
      span: Number(node.props.span ?? 24) || 24,
      autoRun: node.props.autoRun === true,
      display,
    };
    // onSuccessRefresh：动作 target → 目标节点 functionId
    const act = parseAction(node.props.onSuccessRefresh);
    if (act?.kind === 'refreshNode') {
      const target = findNode(tree, act.target);
      const tfid = target && sectionFunctionId(target);
      if (tfid) section.onSuccessRefresh = [tfid];
      else warnings.push(`${fid} 的「成功后刷新」目标无效，已忽略`);
    }
    // 行操作（表格属性面板直接编辑，目标弹窗→函数 id）
    if (node.type === 'fnTable' && Array.isArray(node.props.rowActions)) {
      const ras: CompiledAction[] = [];
      for (const raw of node.props.rowActions as Array<Record<string, unknown>>) {
        const target = typeof raw.targetSection === 'string' ? raw.targetSection : '';
        const sectionTarget = modalFn.get(target) ?? target; // modal 节点 id → 函数 id
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
    if (!sections.some((s) => s.functionId === fid)) sections.push(section);
    else warnings.push(`函数 ${fid} 被多个组件使用，仅保留第一个（V1 一个函数一区块）`);
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
    const targetFn = modalFn.get(act.target);
    if (!targetFn) {
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
      targetSection: targetFn,
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
