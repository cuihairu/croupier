import { nodeId, type PageNode } from './model';
import { parseAction } from './actions';

/** 编译产物：与后端 CompositeSectionRequest 对齐（POST /versioning/pages/composite）。 */
export type CompiledSection = {
  /** 区块唯一 key（同函数多实例：fid、fid-2、fid-3…）；引用一律用 key。 */
  key: string;
  /** 弹窗分组名（modal 容器派生）；dialog 区块按 group 聚合渲染。 */
  group?: string;
  /** 通用事件绑定（发布触发点）。 */
  events?: Array<{
    event: string;
    action: { kind: string; target: string; params?: Record<string, string> };
    chain?: Array<{ kind: string; target: string; params?: Record<string, string> }>;
  }>;
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
  chain?: Array<{ kind: string; target: string; params?: Record<string, string> }>;
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

  // 弹窗分组：modal 容器 → group 名，动作目标统一指向 group
  const modalGroup = new Map<string, string>();
  for (const m of tree.filter((n) => n.type === 'modal')) {
    modalGroup.set(m.id, `modal-${m.id.slice(-6)}`);
  }
  const modalFn = modalGroup; // 兼容旧引用

  /** 节点 → 引用目标（modal=group 名；其余=区块 key）。 */
  const sectionKeyOf = (n: PageNode): string | undefined => {
    if (n.type === 'modal') return modalGroup.get(n.id);
    return nodeSectionKey.get(n.id);
  };

  /** 动作链编译：节点 id 引用 → section key/group 引用；params 透传。 */
  const compileChainRef = (
    raw: unknown,
  ):
    | Array<{
        kind: string;
        target: string;
        params?: Record<string, string>;
      }>
    | undefined => {
    if (!Array.isArray(raw) || raw.length === 0) return undefined;
    const out: Array<{ kind: string; target: string; params?: Record<string, string> }> = [];
    for (const step of raw as Array<Record<string, unknown>>) {
      const kind = String(step.kind ?? 'refreshNode');
      if (kind === 'runBinding' || kind === 'refreshNode') {
        const node = typeof step.target === 'string' ? findNode(tree, step.target) : undefined;
        const target = node ? sectionKeyOf(node) : String(step.target ?? '');
        if (target) {
          const params = step.params as Record<string, string> | undefined;
          out.push({ kind, target, ...(params && Object.keys(params).length ? { params } : {}) });
        }
      } else {
        // 无目标动作（navigate/showMessage/closeModal）
        out.push({
          kind,
          target: '',
          ...((step.params as Record<string, string>)
            ? { params: step.params as Record<string, string> }
            : {}),
        });
      }
    }
    return out.length ? out : undefined;
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
        const group = modalGroup.get(node.id) ?? '';
        const kids = node.children ?? [];
        if (kids.length === 0) {
          warnings.push(`弹窗「${String(node.props.title ?? node.id)}」为空，已忽略`);
          continue;
        }
        for (const kid of kids) {
          if (kid.type === 'text') continue; // 文本暂不进弹窗 spec
          emitFnSection(kid, 'dialog', group);
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

  const emitFnSection = (node: PageNode, display: 'inline' | 'dialog', group?: string) => {
    const fid = String(node.props.functionId ?? '');
    if (!fid) {
      warnings.push(`组件「${String(node.props.title ?? node.id)}」没有绑定函数，已忽略`);
      return;
    }
    const section: CompiledSection = {
      key: nodeSectionKey.get(node.id) ?? fid,
      ...(group ? { group } : {}),
      functionId: fid,
      view: VIEW_MAP[node.type],
      title: String(node.props.title ?? fid),
      span: Number(node.props.span ?? 24) || 24,
      autoRun: node.props.autoRun === true,
      display,
    };
    // 成功后刷新：fnForm.onSuccess 事件（refresh 步骤）+ 旧 onSuccessRefresh 兼容
    const collectRefresh = (raw: unknown, source: string) => {
      const a = parseAction(raw);
      if (!a) return;
      const targets: string[] = [];
      if (a.kind === 'refreshNode') {
        const t = findNode(tree, a.target);
        const k = t && sectionKeyOf(t);
        if (k) targets.push(k);
      }
      for (const step of (raw as { chain?: Array<{ kind: string; target: string }> })?.chain ??
        []) {
        if (step.kind !== 'refreshNode') continue;
        const t = findNode(tree, step.target);
        const k = t && sectionKeyOf(t);
        if (k) targets.push(k);
      }
      if (targets.length)
        section.onSuccessRefresh = [...(section.onSuccessRefresh ?? []), ...targets];
      const hasNonRefresh = (raw as { chain?: Array<{ kind: string }> })?.chain?.some(
        (st) => st.kind !== 'refreshNode',
      );
      if (hasNonRefresh) {
        warnings.push(`${fid} 的「${source}」含非刷新动作（V1 发布仅支持刷新），已忽略该部分`);
      }
    };
    if (node.props.onSuccess) collectRefresh(node.props.onSuccess, '执行成功后');
    if (node.props.onSuccessRefresh) collectRefresh(node.props.onSuccessRefresh, '成功后刷新');

    // 通用事件编译：→ section.events（发布触发点）
    // 事件名映射：onRowClick→rowClick；onRowSelected→rowSelected；onClick→click；onSuccess/onError→success/error
    const eventNameMap: Record<string, string> = {
      onRowClick: 'rowClick',
      onRowSelected: 'rowSelected',
      onClick: 'click',
      onSuccess: 'success',
      onError: 'error',
    };
    for (const [propName, eventName] of Object.entries(eventNameMap)) {
      const raw = node.props[propName] as
        | { kind?: string; target?: string; params?: Record<string, string>; chain?: unknown }
        | undefined;
      if (!raw || typeof raw.kind !== 'string') continue;
      const stepNode = raw.target ? findNode(tree, raw.target) : undefined;
      const targetKey = (stepNode && sectionKeyOf(stepNode)) ?? '';
      const chain = compileChainRef(raw.chain);
      section.events = [
        ...(section.events ?? []),
        {
          event: eventName,
          action: {
            kind: raw.kind,
            target: targetKey,
            ...(raw.params && Object.keys(raw.params).length ? { params: raw.params } : {}),
          },
          ...(chain ? { chain } : {}),
        },
      ];
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
        const raChain = compileChainRef(raw.chain);
        if (raChain) ra.chain = raChain;
        ras.push(ra);
      }
      if (ras.length) section.rowActions = ras;
    }
    sections.push(section);
  };

  const compileButton = (node: PageNode) => {
    const act = parseAction(node.props.onClick);
    const extraChain = compileChainRef((node.props.onClick as { chain?: unknown })?.chain);
    if (!act && !extraChain) {
      warnings.push(`按钮「${String(node.props.title ?? '')}」没有配置动作，已忽略`);
      return;
    }
    const targetNode = act?.target ? findNode(tree, act.target) : undefined;
    if (targetNode?.type === 'modal' && !(targetNode.children ?? []).length) {
      warnings.push(`按钮「${String(node.props.title ?? '')}」的弹窗目标无效（空弹窗），已忽略`);
      return;
    }
    // 挂到最近一个表格 section 的 toolbarActions
    const lastTable = [...sections].reverse().find((s) => s.view === 'table');
    if (!lastTable) {
      warnings.push(
        `按钮「${String(node.props.title ?? '')}」需放置在表格之后（编译为表格顶部按钮），已忽略`,
      );
      return;
    }
    const targetKey = (targetNode && sectionKeyOf(targetNode)) ?? '';
    const danger = node.props.btnStyle === 'danger' ? { danger: true } : {};
    const actParams =
      act?.params && Object.keys(act.params).length
        ? (act.params as Record<string, string>)
        : undefined;
    // 主动作分类发布：无目标动作（navigate/showMessage/closeModal）/ 执行刷新链 / 弹窗
    if (
      act &&
      (act.kind === 'navigate' || act.kind === 'showMessage' || act.kind === 'closeModal')
    ) {
      lastTable.toolbarActions = [
        ...(lastTable.toolbarActions ?? []),
        {
          label: String(node.props.title ?? '操作'),
          targetSection: '',
          chain: [
            { kind: act.kind, target: '', ...(actParams ? { params: actParams } : {}) },
            ...(extraChain ?? []),
          ],
          ...danger,
        },
      ];
      return;
    }
    if (act && act.kind !== 'openModal') {
      if (!targetKey) {
        warnings.push(`按钮「${String(node.props.title ?? '')}」动作目标无效，已忽略`);
        return;
      }
      const step = act.kind === 'runBinding' ? 'runBinding' : 'refreshNode';
      lastTable.toolbarActions = [
        ...(lastTable.toolbarActions ?? []),
        {
          label: String(node.props.title ?? '操作'),
          targetSection: '',
          chain: [{ kind: step, target: targetKey }, ...(extraChain ?? [])],
          ...danger,
        },
      ];
      return;
    }
    if (!targetKey) {
      warnings.push(`按钮「${String(node.props.title ?? '')}」的弹窗目标无效，已忽略`);
      return;
    }
    const ta: CompiledAction = {
      label: String(node.props.title ?? '操作'),
      targetSection: targetKey,
    };
    if (node.props.btnStyle === 'danger') ta.danger = true;
    if (extraChain) ta.chain = extraChain;
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
  group?: string;
  events?: Array<{
    event: string;
    action: { kind: string; target: string; params?: Record<string, string> };
    chain?: Array<{ kind: string; target: string; params?: Record<string, string> }>;
  }>;
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
  const groupToModal = new Map<string, PageNode>();
  const dialogGroupToModalId = new Map<string, string>();
  const toolbarButtons: { tableId: string; actions: Array<Record<string, unknown>> }[] = [];
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
      if (sec.toolbar) fnProps.toolbar = sec.toolbar; // 第二遍还原为按钮节点后移除
    }
    if (view === 'fnForm') fnProps.display = 'inline';

    if (sec.display === 'dialog') {
      const form: PageNode = { id: nodeId(view), type: view, props: fnProps };
      const group = String(sec.group ?? key);
      if (!groupToModal.has(group)) {
        const modal: PageNode = {
          id: nodeId('modal'),
          type: 'modal',
          props: { title: titleOf(sec, fid), width: 'medium' },
          children: [form],
        };
        groupToModal.set(group, modal);
        dialogGroupToModalId.set(group, modal.id);
        nodes.push(modal);
      } else {
        groupToModal.get(group)!.children!.push(form);
      }
      keyToNodeId.set(key, form.id);
      dialogKeyToModalId.set(key, groupToModal.get(group)!.id);
    } else {
      const node: PageNode = { id: nodeId(view), type: view, props: fnProps };
      keyToNodeId.set(key, node.id);
      nodes.push(node);
    }
  }

  // 事件绑定暂存（第二遍按 key 映射到节点 props）
  const pendingEvents = new Map<string, NonNullable<SpecSectionLike['events']>>();
  for (const sec of sections) {
    if (sec.events?.length) pendingEvents.set(String(sec.key ?? sec.functionId ?? ''), sec.events);
  }

  // 第二遍：重建引用（rowActions/toolbar/onSuccessRefresh/events）
  for (const node of nodes) {
    // 事件还原：spec.event 名 → 编辑器事件 prop 名；target=section key → 节点 id
    const evProp: Record<string, string> = {
      rowClick: 'onRowClick',
      rowSelected: 'onRowSelected',
      click: 'onClick',
      success: 'onSuccess',
      error: 'onError',
    };
    for (const [secKey, evs] of pendingEvents) {
      const owner = findInNodes(nodes, keyToNodeId.get(secKey) ?? '');
      if (owner !== node) continue;
      for (const ev of evs) {
        const propName = evProp[ev.event];
        if (!propName) continue;
        const mapTarget = (t: string) => {
          if (!t) return '';
          const nid = keyToNodeId.get(t) ?? dialogGroupToModalId.get(t) ?? '';
          return nid;
        };
        (node.props as Record<string, unknown>)[propName] = {
          kind: ev.action.kind,
          target: mapTarget(ev.action.target),
          ...(ev.action.params ? { params: ev.action.params } : {}),
          ...(ev.chain?.length
            ? {
                chain: ev.chain.map((st) => ({
                  kind: st.kind,
                  target: mapTarget(st.target),
                  ...(st.params ? { params: st.params } : {}),
                })),
              }
            : {}),
        };
      }
    }
    if (node.type === 'fnTable') {
      const ras = (node.props.rowActions as Array<Record<string, unknown>>) ?? [];
      node.props.rowActions = ras
        .map((ra) => {
          const t = String(ra.targetSection ?? '');
          const modalId = dialogKeyToModalId.get(t) ?? dialogGroupToModalId.get(t);
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
      delete node.props.toolbar;
      if (tas.length) toolbarButtons.push({ tableId: node.id, actions: tas });
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
  // 顶部按钮还原为独立 button 节点（插到对应表格后——round-trip 等价）
  for (const { tableId, actions } of toolbarButtons) {
    let insertAt = nodes.findIndex((n) => n.id === tableId);
    if (insertAt === -1) insertAt = nodes.length - 1;
    for (const ta of actions) {
      insertAt += 1;
      const t = String(ta.targetSection ?? '');
      const modalId = t ? (dialogGroupToModalId.get(t) ?? dialogKeyToModalId.get(t)) : undefined;
      const rawLabel = ta.label as unknown;
      const label =
        typeof rawLabel === 'string'
          ? rawLabel
          : String(
              ((rawLabel as Record<string, unknown>)?.['zh-CN'] as string | undefined) ??
                (rawLabel as Record<string, unknown>)?.['en-US'] ??
                '操作',
            );
      const chain = Array.isArray(ta.chain)
        ? (
            ta.chain as Array<{ kind: string; target: string; params?: Record<string, string> }>
          ).map((c) => ({
            kind: c.kind,
            target: keyToNodeId.get(String(c.target)) ?? String(c.target),
            ...(c.params ? { params: c.params } : {}),
          }))
        : undefined;
      if (t && !modalId) {
        warnings.push(`按钮「${label}」的弹窗目标 ${t} 无法还原，已丢弃`);
        continue;
      }
      const btn: PageNode = {
        id: nodeId('button'),
        type: 'button',
        props: {
          title: label,
          btnStyle: ta.danger === true ? 'danger' : 'default',
          span: 6,
          ...(modalId
            ? { onClick: { kind: 'openModal', target: modalId, ...(chain ? { chain } : {}) } }
            : chain
              ? { onClick: { kind: chain[0].kind, target: chain[0].target, chain: chain.slice(1) } }
              : {}),
        },
      };
      nodes.splice(insertAt, 0, btn);
    }
  }
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
