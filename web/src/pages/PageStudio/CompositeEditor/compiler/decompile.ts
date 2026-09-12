import { getIntl } from '@umijs/max';
import { nodeId, type PageNode } from '../model';
import { parseAction } from '../actions';
import { localizedText } from '@/utils/localizedText';
import {
  isSingleExpression,
  parseExpression,
  pointerToPath,
} from '@/components/PageRenderer/expression';
import type { SpecSectionLike } from './types';
import { SECTION_KEY_RE } from './types';

// 纯函数模块无法 useIntl：经 getIntl 求值诊断文案（先例 services/api/bugs.ts）；
// 回读节点的 title/label 兜底（'常量表单'/'操作'）是编辑树数据，不属于 UI 文案
const intl = getIntl();
// ---------------------------------------------------------------------------
// 回读编辑：CompositeSection → PageNode 树（编译的逆变换）
// ---------------------------------------------------------------------------

/**
 * 反编译：sections → 编辑树。
 * - dialog sections → 各自一个 modal（含 fnForm 子节点）
 * - tab sections（V2）→ 按 group 聚合 tabs 容器（sectionKey 回写组名保
 *   round-trip）、按 tab 标签聚合页 container、区块节点入页 children
 * - inline sections → fnTable/fnFields/fnForm
 * - rowActions/toolbarActions.targetSection（dialog key）→ 映射回 modal 节点 id
 * - onSuccessRefresh（inline key）→ 映射回对应节点 id
 * 返回 [树, 警告]（不可映射的引用降级为警告并丢弃）。
 */
export function decompileToTree(sections: SpecSectionLike[]): [PageNode[], string[]] {
  const warnings: string[] = [];
  const keyToNodeId = new Map<string, string>();
  const dialogKeyToModalId = new Map<string, string>();
  const titleOf = (sec: SpecSectionLike, fallback: string): string =>
    localizedText(sec.title as Record<string, string> | string | undefined, 'zh-CN', fallback);
  const tabLabelOf = (sec: SpecSectionLike, fallback: string): string =>
    localizedText(sec.tab as Record<string, string> | string | undefined, 'zh-CN', fallback);

  // 第一遍：创建节点并登记映射
  const nodes: PageNode[] = [];
  const pendingDialogs: { sec: SpecSectionLike; modal: PageNode }[] = [];
  const groupToModal = new Map<string, PageNode>();
  const dialogGroupToModalId = new Map<string, string>();
  // V2：tab 聚合（group → tabs 节点；组内按 tab 标签 → 页 container）
  const groupToTabs = new Map<string, PageNode>();
  const toolbarButtons: { tableId: string; actions: Array<Record<string, unknown>> }[] = [];
  // 参数映射反查延后（page_state key → 上游节点 id 需完整 keyToNodeId）
  const pendingAssignments: { raw: SpecSectionLike['inputAssignments']; ownerKey: string }[] = [];

  /** tab 区块归位：组无 tabs 则建（sectionKey 回写组名），页无则建
   * （props.title=页签标签），节点入页 children。 */
  const placeIntoTab = (node: PageNode, group: string, tabLabel: string): void => {
    if (!groupToTabs.has(group)) {
      const tabsNode: PageNode = {
        id: nodeId('tabs'),
        type: 'tabs',
        props: SECTION_KEY_RE.test(group) ? { sectionKey: group } : {},
        children: [],
      };
      groupToTabs.set(group, tabsNode);
      nodes.push(tabsNode);
    }
    const tabsNode = groupToTabs.get(group)!;
    let page = (tabsNode.children ?? []).find(
      (p) => String(p.props.title ?? '') === tabLabel && p.type === 'container',
    );
    if (!page) {
      page = { id: nodeId('container'), type: 'container', props: { title: tabLabel, span: 24 } };
      page.children = [node];
      tabsNode.children = [...(tabsNode.children ?? []), page];
      return;
    }
    page.children = [...(page.children ?? []), node];
  };

  /** #94 card 区块归位：组无卡片容器则建（publishAs='card'、
   * sectionKey=group 回写组名、title=cardTitle），节点入 children。 */
  const cardTitleOf = (sec: SpecSectionLike, fallback: string): string =>
    localizedText(sec.cardTitle as Record<string, string> | string | undefined, 'zh-CN', fallback);
  const groupToCard = new Map<string, PageNode>();
  const placeIntoCard = (node: PageNode, group: string, sec: SpecSectionLike): void => {
    if (!groupToCard.has(group)) {
      const cardNode: PageNode = {
        id: nodeId('container'),
        type: 'container',
        props: {
          publishAs: 'card',
          title: cardTitleOf(sec, group),
          span: 24,
          ...(SECTION_KEY_RE.test(group) ? { sectionKey: group } : {}),
        },
        children: [],
      };
      groupToCard.set(group, cardNode);
      nodes.push(cardNode);
    }
    const cardNode = groupToCard.get(group)!;
    cardNode.children = [...(cardNode.children ?? []), node];
  };

  /** U10 回读：spec 叶子条件 → 编辑态 prop {expr,op,value}（key+path 还原
   * 为表达式字面值，同参数映射 V5 多段路径模式）。嵌套组合条件编辑器不
   * 产出——降级为警告丢弃（不静默）。 */
  const conditionPropOf = (sec: SpecSectionLike): Record<string, unknown> | undefined => {
    const c = sec.visibleWhen;
    if (!c) return undefined;
    const key = String(c.key ?? '');
    const path = typeof c.path === 'string' ? c.path : '';
    if ((c.kind !== 'equals' && c.kind !== 'notEquals' && c.kind !== 'exists') || !key || !path) {
      warnings.push(
        intl.formatMessage(
          {
            id: 'pages.pageStudio.compiler.warning.visibleWhenUnmappable',
            defaultMessage: '区块「{key}」的显示条件无法还原为编辑器形态，已丢弃',
          },
          { key: String(sec.key ?? '') },
        ),
      );
      return undefined;
    }
    const segs = pointerToPath(path);
    const dots = segs.map((s) => (typeof s === 'number' ? `[${s}]` : `.${s}`)).join('');
    return {
      expr: `{{${key}${dots}}}`,
      op: c.kind,
      ...(c.kind !== 'exists' ? { value: String(c.value ?? '') } : {}),
    };
  };

  for (const sec of sections) {
    // 常量表单（static）：还原为 staticForm 节点（schema 从 form.jsonSchema）
    if (sec.static === true) {
      const key = String(sec.key ?? '');
      const schema = (sec.form as { jsonSchema?: Record<string, unknown> } | undefined)?.jsonSchema;
      const node: PageNode = {
        id: nodeId('staticForm'),
        type: 'staticForm',
        props: {
          title: titleOf(sec, key || '常量表单'),
          span: Number(sec.span ?? 12) || 12,
          staticSchema: JSON.stringify(schema ?? { type: 'object', properties: {} }, null, 2),
          refreshOn: sec.refreshOn ?? [],
          // U10 条件显示回读（inline/tab staticForm）
          ...(conditionPropOf(sec) ? { visibleWhen: conditionPropOf(sec) } : {}),
          // 固化区块 key（round-trip 稳定，U5）
          sectionKey: key,
        },
      };
      if (sec.display === 'tab') {
        placeIntoTab(node, String(sec.group ?? ''), tabLabelOf(sec, key));
      } else if (sec.display === 'card') {
        placeIntoCard(node, String(sec.group ?? ''), sec);
      } else {
        nodes.push(node);
      }
      keyToNodeId.set(key, node.id);
      continue;
    }
    const fid = String(sec.functionId ?? sec.bindingId ?? '');
    const key = String(sec.key ?? fid);
    if (!fid) {
      warnings.push(
        intl.formatMessage({
          id: 'pages.pageStudio.compiler.warning.sectionMissingFunction',
          defaultMessage: '区块缺少函数绑定，已跳过',
        }),
      );
      continue;
    }
    const view = sec.view === 'table' ? 'fnTable' : sec.view === 'fields' ? 'fnFields' : 'fnForm';
    const fnProps: Record<string, unknown> = {
      functionId: fid,
      title: titleOf(sec, fid),
      span: Number(sec.span ?? 24) || 24,
      autoRun: sec.autoRun === true,
      onSuccessRefresh: undefined,
      // U10 条件显示回读（仅 inline/tab 语义；dialog 区块 spec 不带该字段）
      ...(conditionPropOf(sec) ? { visibleWhen: conditionPropOf(sec) } : {}),
      // 固化区块 key（round-trip 稳定，U5）
      sectionKey: key,
    };
    if (view === 'fnTable') {
      fnProps.columns = (sec.table?.columns ?? []).map((c) => String(c.key ?? '')).filter(Boolean);
      fnProps.rowActions = sec.table?.rowActions ?? [];
      if (sec.toolbar) fnProps.toolbar = sec.toolbar; // 第二遍还原为按钮节点后移除
    }
    if (view === 'fnForm') fnProps.display = 'inline';
    if (Array.isArray(sec.inputAssignments) && sec.inputAssignments.length > 0) {
      // 第二遍统一反查 key → 上游节点 id（keyToNodeId 此时才完整）
      pendingAssignments.push({ raw: sec.inputAssignments, ownerKey: key });
    }

    if (sec.display === 'dialog') {
      const form: PageNode = { id: nodeId(view), type: view, props: fnProps };
      const group = String(sec.group ?? key);
      if (!groupToModal.has(group)) {
        const modal: PageNode = {
          id: nodeId('modal'),
          type: 'modal',
          // 回写 group 名为 modal sectionKey：再编译 group 稳定（round-trip）
          props: {
            title: titleOf(sec, fid),
            width: 'medium',
            ...(SECTION_KEY_RE.test(group) ? { sectionKey: group } : {}),
          },
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
    } else if (sec.display === 'tab') {
      // V2：页签区块 → tabs 容器（组）→ 页 container（标签）→ 区块节点
      const node: PageNode = { id: nodeId(view), type: view, props: fnProps };
      placeIntoTab(node, String(sec.group ?? ''), tabLabelOf(sec, key));
      keyToNodeId.set(key, node.id);
    } else if (sec.display === 'card') {
      // #94：卡片区块 → 卡片容器（publishAs='card'）→ 区块节点
      const node: PageNode = { id: nodeId(view), type: view, props: fnProps };
      placeIntoCard(node, String(sec.group ?? key), sec);
      keyToNodeId.set(key, node.id);
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

  // 第二遍前置：refreshOn 还原（spec key → 上游节点 id → props.refreshOnNode）。
  // 缺失会导致回读→再编译丢失级联联动（round-trip 无损硬要求）。
  // 有节点的依赖 → refreshOnNode（节点 id 引用）；无节点的陈旧依赖 →
  // 保留在 refreshOn 字面量（不静默丢弃）。
  for (const sec of sections) {
    const deps = (sec.refreshOn ?? []).map(String).filter(Boolean);
    if (!deps.length) continue;
    const key = String(sec.key ?? sec.functionId ?? sec.bindingId ?? '');
    const owner = findInNodes(nodes, keyToNodeId.get(key) ?? '');
    if (!owner) continue;
    const nodeIds: string[] = [];
    const literals: string[] = [];
    for (const dep of deps) {
      const depId = keyToNodeId.get(dep);
      if (depId) nodeIds.push(depId);
      else literals.push(dep);
    }
    if (nodeIds.length) {
      const existingNodeDeps = Array.isArray(owner.props.refreshOnNode)
        ? (owner.props.refreshOnNode as unknown[]).map(String)
        : [];
      (owner.props as Record<string, unknown>).refreshOnNode = [
        ...new Set([...existingNodeDeps, ...nodeIds]),
      ];
    }
    if (literals.length) {
      const existingLiterals = Array.isArray(owner.props.refreshOn)
        ? (owner.props.refreshOn as unknown[]).map(String)
        : [];
      (owner.props as Record<string, unknown>).refreshOn = [
        ...new Set([...existingLiterals, ...literals]),
      ];
    }
  }

  // 第二遍前置：参数映射反查（page_state key → 上游节点 id；round-trip 不丢）。
  // V5：多段路径（/data/total、/values/kw、/selectedRow/uid）→ 表达式字面值
  // {{key.路径}}；单段扁平路径维持结构化（来源区块 + 字段）。
  for (const pending of pendingAssignments) {
    const owner = findInNodes(nodes, keyToNodeId.get(pending.ownerKey) ?? '');
    if (!owner) continue;
    (owner.props as Record<string, unknown>).inputAssignments = (
      pending.raw as Array<Record<string, unknown>>
    ).map((m) => {
      const sourceKey = String(m.key ?? '');
      const upstreamId = keyToNodeId.get(sourceKey) ?? '';
      if (m.kind !== 'literal' && sourceKey && !upstreamId) {
        warnings.push(
          intl.formatMessage(
            {
              id: 'pages.pageStudio.compiler.warning.mappingSourceMissing',
              defaultMessage: '区块「{ownerKey}」参数映射来源「{sourceKey}」不存在，已保留字面值',
            },
            { ownerKey: pending.ownerKey, sourceKey },
          ),
        );
      }
      const pointer = typeof m.path === 'string' ? m.path : '';
      const segs = pointerToPath(pointer);
      if (m.kind !== 'literal' && segs.length >= 2) {
        const dots = segs.map((s) => (typeof s === 'number' ? `[${s}]` : `.${s}`)).join('');
        return {
          param: String(m.target ?? '').replace(/^\//, ''),
          kind: 'literal' as const,
          value: `{{${sourceKey}${dots}}}`,
          sourceNodeId: '',
          field: undefined,
        };
      }
      return {
        param: String(m.target ?? '').replace(/^\//, ''),
        kind: m.kind,
        sourceNodeId: upstreamId || sourceKey,
        field: pointer.replace(/^\//, '') || undefined,
        value: m.value,
      };
    });
  }

  // 第二遍：重建引用（events/rowActions/toolbar/onSuccessRefresh）。
  // 事件还原直接按 owner 赋值、rowActions/toolbar 递归遍历整树——
  // dialog 区块的节点在 modal.children 内，只扫根级会让弹窗表单的事件
  // 绑定静默丢失（fnForm 是 modal 唯一合法子节点，即所有弹窗表单事件必丢）。
  const evProp: Record<string, string> = {
    rowClick: 'onRowClick',
    rowSelected: 'onRowSelected',
    click: 'onClick',
    success: 'onSuccess',
    error: 'onError',
  };
  // 事件还原：spec.event 名 → 编辑器事件 prop 名；target=section key → 节点 id
  for (const [secKey, evs] of pendingEvents) {
    const owner = findInNodes(nodes, keyToNodeId.get(secKey) ?? '');
    if (!owner) continue;
    for (const ev of evs) {
      const propName = evProp[ev.event];
      if (!propName) continue;
      const mapTarget = (t: string) => {
        if (!t) return '';
        const nid = keyToNodeId.get(t) ?? dialogGroupToModalId.get(t) ?? '';
        return nid;
      };
      (owner.props as Record<string, unknown>)[propName] = {
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
  // label 提取：后端把 rowActions/toolbar 的 label 包装为 LocalizedText
  // （{"zh-CN": ...}），回读必须提取字符串——原样透传会让再编译
  // String(label) 变 "[object Object]"（数据毁坏）。
  const labelOf = (raw: unknown, fallback: string): string =>
    localizedText(raw as Record<string, string> | string | undefined, 'zh-CN', fallback);
  // rowActions/toolbar 还原（递归：含 dialog 内的 fnTable）
  const restoreTableNode = (node: PageNode): void => {
    if (node.type === 'fnTable') {
      const ras = (node.props.rowActions as Array<Record<string, unknown>>) ?? [];
      node.props.rowActions = ras
        .map((ra) => {
          const t = String(ra.targetSection ?? '');
          const modalId = dialogKeyToModalId.get(t) ?? dialogGroupToModalId.get(t);
          if (!modalId) {
            warnings.push(
              intl.formatMessage(
                {
                  id: 'pages.pageStudio.compiler.warning.rowActionTargetLost',
                  defaultMessage: '行操作「{label}」的弹窗目标 {target} 无法还原，已丢弃',
                },
                { label: labelOf(ra.label, ''), target: t },
              ),
            );
            return null;
          }
          // V5 round-trip：编译产物 row.字段 → 编辑器表达式 {{row.字段}}
          const params: Record<string, unknown> = {};
          for (const [k, v] of Object.entries(
            (ra.params as Record<string, unknown> | undefined) ?? {},
          )) {
            params[k] =
              typeof v === 'string' && v.startsWith('row.') && !v.slice(4).includes('.')
                ? `{{${v}}}`
                : v;
          }
          return {
            ...ra,
            label: labelOf(ra.label, '操作'),
            ...(Object.keys(params).length ? { params } : { params: undefined }),
            targetSection: modalId,
          };
        })
        .filter((x) => x !== null);
      const tas =
        (node.props.toolbar as { actions?: Array<Record<string, unknown>> } | undefined)?.actions ??
        [];
      delete node.props.toolbar;
      if (tas.length) toolbarButtons.push({ tableId: node.id, actions: tas });
    }
    for (const child of node.children ?? []) restoreTableNode(child);
  };
  for (const node of nodes) restoreTableNode(node);
  // onSuccessRefresh（遗留字段）：按 keyToNodeId（dialog 表单或 inline 节点）
  for (const sec of sections) {
    const key = String(sec.key ?? sec.functionId ?? sec.bindingId ?? '');
    const target = sec.onSuccessRefresh?.[0];
    if (!target) continue;
    // 找到 key 对应的 fnForm/inline 节点
    const srcId = keyToNodeId.get(key);
    const tgtId = keyToNodeId.get(target);
    if (srcId && tgtId) {
      const srcNode = findInNodes(nodes, srcId);
      if (srcNode) srcNode.props.onSuccessRefresh = { kind: 'refreshNode', target: tgtId };
    } else {
      warnings.push(
        intl.formatMessage(
          {
            id: 'pages.pageStudio.compiler.warning.successRefreshLost',
            defaultMessage: '「成功后刷新」引用 {target} 无法还原，已丢弃',
          },
          { target },
        ),
      );
    }
  }
  // 顶部按钮还原为独立 button 节点（插到对应表格之后——round-trip 等价）。
  // V2：表格可能在 container/tabs 页内，须插回其所在 children 列表（此前
  // 只扫根级，容器内表格的按钮会漂移到页面末尾）。
  const owningList = (id: string): PageNode[] => {
    const walk = (list: PageNode[]): PageNode[] | undefined => {
      for (const n of list) {
        if (n.id === id) return list;
        if (n.children) {
          const hit = walk(n.children);
          if (hit) return hit;
        }
      }
      return undefined;
    };
    return walk(nodes) ?? nodes;
  };
  for (const { tableId, actions } of toolbarButtons) {
    const list = owningList(tableId);
    let insertAt = list.findIndex((n) => n.id === tableId);
    if (insertAt === -1) insertAt = list.length - 1;
    for (const ta of actions) {
      insertAt += 1;
      const t = String(ta.targetSection ?? '');
      const modalId = t ? (dialogGroupToModalId.get(t) ?? dialogKeyToModalId.get(t)) : undefined;
      const label = labelOf(ta.label, '操作');
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
        warnings.push(
          intl.formatMessage(
            {
              id: 'pages.pageStudio.compiler.warning.buttonTargetLost',
              defaultMessage: '按钮「{label}」的弹窗目标 {target} 无法还原，已丢弃',
            },
            { label, target: t },
          ),
        );
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
            ? {
                // 规范形态：targetSection 隐式 openModal；params=预填初值；
                // chain 全为后续步骤
                onClick: {
                  kind: 'openModal',
                  target: modalId,
                  ...(ta.params && Object.keys(ta.params as object).length
                    ? { params: ta.params }
                    : {}),
                  ...(chain ? { chain } : {}),
                },
              }
            : chain
              ? {
                  onClick: {
                    kind: chain[0].kind,
                    target: chain[0].target,
                    ...(chain[0].params ? { params: chain[0].params } : {}),
                    ...(chain.length > 1 ? { chain: chain.slice(1) } : {}),
                  },
                }
              : {}),
        },
      };
      list.splice(insertAt, 0, btn);
    }
  }
  return [nodes, warnings];
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
