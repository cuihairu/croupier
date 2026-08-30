import { nodeId, type PageNode } from './model';
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
        const forms = (node.children ?? []).filter((c) => c.type === 'fnForm');
        for (const form of forms) emitFnSection(form, 'dialog');
        for (const other of (node.children ?? []).filter((c) => c.type !== 'fnForm')) {
          warnings.push(
            `弹窗「${String(node.props.title ?? node.id)}」内组件 ${other.type} 不参与发布（V1 仅表单）`,
          );
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

// ---------------------------------------------------------------------------
// 回读编辑：CompositeSection → PageNode 树（编译的逆变换）
// ---------------------------------------------------------------------------

/** spec section 的宽松形态（回读自提案/发布 PageSpec）。 */
export interface SpecSectionLike {
  key?: string;
  bindingId?: string;
  functionId?: string;
  view?: string;
  title?: unknown;
  span?: number;
  autoRun?: boolean;
  refreshOn?: string[];
  display?: string;
  onSuccessRefresh?: string[];
  table?: { columns?: Array<{ key?: string }>; rowActions?: Array<Record<string, unknown>> };
  toolbar?: { actions?: Array<Record<string, unknown>> };
}

/**
 * 反编译：sections → 编辑树。
 * - dialog sections → 各自一个 modal（含 fnForm 子节点）
 * - inline sections → fnTable/fnFields/fnForm
 * - rowActions/toolbarActions.targetSection（dialog key）→ 映射回 modal 节点 id
 * - onSuccessRefresh（inline key）→ 映射回对应节点 id
 * 返回 [树, 警告]（不可映射的引用降级为警告并丢弃）。
 */
export function decompileToTree(sections: SpecSectionLike[]): [PageNode[], string[]] {
  const warnings: string[] = [];
  const keyToNodeId = new Map<string, string>();
  const dialogKeyToModalId = new Map<string, string>();
  const titleOf = (sec: SpecSectionLike, fallback: string): string => {
    const t = sec.title;
    if (t && typeof t === 'object') {
      const lt = t as Record<string, unknown>;
      const v = lt['zh-CN'] ?? lt['en-US'];
      if (typeof v === 'string' && v) return v;
    }
    if (typeof t === 'string' && t) return t;
    return fallback;
  };

  // 第一遍：创建节点并登记映射
  const nodes: PageNode[] = [];
  const pendingDialogs: { sec: SpecSectionLike; modal: PageNode }[] = [];
  for (const sec of sections) {
    const fid = String(sec.functionId ?? sec.bindingId ?? '');
    const key = String(sec.key ?? fid);
    if (!fid) {
      warnings.push('区块缺少函数绑定，已跳过');
      continue;
    }
    const view = sec.view === 'table' ? 'fnTable' : sec.view === 'fields' ? 'fnFields' : 'fnForm';
    const fnProps: Record<string, unknown> = {
      functionId: fid,
      title: titleOf(sec, fid),
      span: Number(sec.span ?? 24) || 24,
      autoRun: sec.autoRun === true,
      onSuccessRefresh: undefined,
    };
    if (view === 'fnTable') {
      fnProps.columns = (sec.table?.columns ?? []).map((c) => String(c.key ?? '')).filter(Boolean);
      fnProps.rowActions = sec.table?.rowActions ?? [];
    }
    if (view === 'fnForm') fnProps.display = 'inline';

    if (sec.display === 'dialog') {
      const form: PageNode = { id: nodeId('fnForm'), type: 'fnForm', props: fnProps };
      const modal: PageNode = {
        id: nodeId('modal'),
        type: 'modal',
        props: { title: titleOf(sec, fid), width: 'medium' },
        children: [form],
      };
      keyToNodeId.set(key, form.id);
      dialogKeyToModalId.set(key, modal.id);
      pendingDialogs.push({ sec, modal });
      nodes.push(modal);
    } else {
      const node: PageNode = { id: nodeId(view), type: view, props: fnProps };
      keyToNodeId.set(key, node.id);
      nodes.push(node);
    }
  }

  // 第二遍：重建引用（rowActions/toolbar/onSuccessRefresh）
  for (const node of nodes) {
    if (node.type === 'fnTable') {
      const ras = (node.props.rowActions as Array<Record<string, unknown>>) ?? [];
      node.props.rowActions = ras
        .map((ra) => {
          const t = String(ra.targetSection ?? '');
          const modalId = dialogKeyToModalId.get(t);
          if (!modalId) {
            warnings.push(`行操作「${String(ra.label ?? '')}」的弹窗目标 ${t} 无法还原，已丢弃`);
            return null;
          }
          return { ...ra, targetSection: modalId };
        })
        .filter((x) => x !== null);
      const tas =
        (node.props.toolbar as { actions?: Array<Record<string, unknown>> } | undefined)?.actions ??
        [];
      if (tas.length) {
        const buttons = tas
          .map((ta) => {
            const t = String(ta.targetSection ?? '');
            const modalId = dialogKeyToModalId.get(t);
            if (!modalId) return null;
            return {
              label: String(ta.label ?? ''),
              targetSection: modalId,
              danger: ta.danger === true,
            };
          })
          .filter((x) => x !== null);
        // 顶部按钮在编辑树中不存在独立节点（编译产物）——回读为表格备注，编辑后再编译会以表格属性为准
        void buttons;
        warnings.push('表格顶部按钮为编译产物，回读后请在行操作/按钮重新配置（原始配置已丢弃）');
      }
      delete node.props.toolbar;
    }
    if (node.type === 'fnForm' && node.props.display === 'inline') {
      void node;
    }
  }
  // onSuccessRefresh：按 keyToNodeId（dialog 表单或 inline 节点）
  let i = 0;
  for (const sec of sections) {
    const key = String(sec.key ?? sec.functionId ?? sec.bindingId ?? '');
    const target = sec.onSuccessRefresh?.[0];
    if (!target) {
      i++;
      continue;
    }
    const nodeIdOfSource = nodes.find((n) =>
      n.type === 'modal' ? n.children?.[0] && dialogFormKey(n) === undefined : false,
    );
    void nodeIdOfSource;
    // 找到 key 对应的 fnForm/inline 节点
    const srcId = keyToNodeId.get(key);
    const tgtId = keyToNodeId.get(target);
    if (srcId && tgtId) {
      const srcNode = findInNodes(nodes, srcId);
      if (srcNode) srcNode.props.onSuccessRefresh = { kind: 'refreshNode', target: tgtId };
    } else {
      warnings.push(`「成功后刷新」引用 ${target} 无法还原，已丢弃`);
    }
    i++;
  }
  void i;
  return [nodes, warnings];
}

function dialogFormKey(n: PageNode): string | undefined {
  void n;
  return undefined;
}

function findInNodes(nodes: PageNode[], id: string): PageNode | undefined {
  for (const n of nodes) {
    if (n.id === id) return n;
    if (n.children) {
      const hit = findInNodes(n.children, id);
      if (hit) return hit;
    }
  }
  return undefined;
}
