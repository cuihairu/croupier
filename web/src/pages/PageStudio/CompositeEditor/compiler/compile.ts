import { getIntl } from '@umijs/max';
import type { PageNode } from '../model';
import { parseAction } from '../actions';
import {
  isSingleExpression,
  parseExpression,
  pathToPointer,
  ROW_VARIABLE,
} from '@/components/PageRenderer/expression';
import type { CompiledSection, CompileResult } from './types';
import { SECTION_KEY_RE, VIEW_MAP } from './types';
import type { CompiledAction } from './types';
import { normalizeRowActionParams } from './normalize';

// 纯函数模块无法 useIntl：经 getIntl 求值诊断文案（SelectLang 切语言整页刷新后
// 重新求值，先例 services/api/bugs.ts）；区块/动作的 title、label 兜底是编译
// 产物 payload 数据，不属于 UI 文案，保持字面量
const intl = getIntl();
/**
 * 编辑树 → 平铺 CompositeSection 列表。
 * 编译规则（V1）：
 * - fnTable/fnFields/fnForm → section（display=dialog 时不占栅格语义，仅弹窗）
 * - modal 容器 → 其唯一 fnForm 编译为 display=dialog 的 section
 * - 根级 button.onClick=openModal → 挂到前一个 fnTable 的 toolbarActions；
 *   页面无表格时警告忽略（V1 独立按钮依赖表格顶部）
 * - fnForm.onSuccessRefresh 动作 target → 目标节点 functionId
 * - text/container 不产生 section（container 子节点平铺；text 忽略并警告）
 * V2 追加：
 * - tabs 容器 → 页（container）内区块平铺为 display='tab' + group（同组
 *   渲染进同一 Tabs；优先 tabs.props.sectionKey，否则 tabs-<id尾6> 去重）
 *   + tab（页 title=页签标签）；页内 button 仍编译到最近表格 toolbar
 */

export function compileTree(tree: PageNode[]): CompileResult {
  const sections: CompiledSection[] = [];
  const warnings: string[] = [];

  // 区块 key 分配（实例命名空间，U5）：
  // 1) 节点声明的 sectionKey（合法且不重复）优先固化——回读/重命名/再实例化
  //    后 key 稳定，不随树顺序漂移；
  // 2) 未声明者同函数多实例依次 fid、fid-2、fid-3……（编辑器允许同一函数
  //    拖多个组件，发布侧按 key 唯一区分）。
  const nodeSectionKey = new Map<string, string>();
  const usedKeys = new Set<string>();
  const allocKey = (fid: string): string => {
    let key = fid;
    let i = 2;
    while (usedKeys.has(key)) key = `${fid}-${i++}`;
    usedKeys.add(key);
    return key;
  };
  const isSectionNode = (n: PageNode): boolean =>
    (!!VIEW_MAP[n.type] && typeof n.props.functionId === 'string' && !!n.props.functionId) ||
    n.type === 'staticForm';
  // 第一遍：登记声明 key（重复/非法 → 忽略并警告，回退自动分配）
  const collectDeclared = (nodes: PageNode[]) => {
    for (const n of nodes) {
      if (n.type === 'modal') {
        collectDeclared(n.children ?? []);
        continue;
      }
      if (isSectionNode(n)) {
        const declared = typeof n.props.sectionKey === 'string' ? n.props.sectionKey.trim() : '';
        if (declared) {
          if (!SECTION_KEY_RE.test(declared) || usedKeys.has(declared)) {
            warnings.push(
              intl.formatMessage(
                {
                  id: 'pages.pageStudio.compiler.warning.sectionKeyInvalid',
                  defaultMessage: '区块「{title}」的 key「{key}」非法或重复，已自动分配',
                },
                { title: String(n.props.title ?? declared), key: declared },
              ),
            );
          } else {
            usedKeys.add(declared);
            nodeSectionKey.set(n.id, declared);
          }
        }
      }
      if (n.children) collectDeclared(n.children);
    }
  };
  collectDeclared(tree);
  // 第二遍：未声明者按函数 id 自动分配
  const assignKeys = (nodes: PageNode[]) => {
    for (const n of nodes) {
      if (n.type === 'modal') {
        assignKeys(n.children ?? []);
        continue;
      }
      if (isSectionNode(n) && !nodeSectionKey.has(n.id)) {
        const fid =
          typeof n.props.functionId === 'string' && n.props.functionId
            ? String(n.props.functionId)
            : String(n.id);
        nodeSectionKey.set(n.id, allocKey(fid));
      }
      if (n.children) assignKeys(n.children);
    }
  };
  assignKeys(tree);
  // V5 表达式变量空间：全部区块 key（表达式 {{var.path}} 的变量必须命中区块）
  const sectionKeySet = new Set(nodeSectionKey.values());

  // 弹窗分组：modal 容器 → group 名，动作目标统一指向 group。
  // 稳定性：优先用 modal 声明的 sectionKey（拖入时自动命名且随树持久），
  // 避免 group 随节点 id 每次回读/再编译漂移（发布 diff 稳定性）。
  const modalGroup = new Map<string, string>();
  const usedGroups = new Set<string>();
  for (const m of tree.filter((n) => n.type === 'modal')) {
    const declared = typeof m.props.sectionKey === 'string' ? m.props.sectionKey.trim() : '';
    const group =
      declared && SECTION_KEY_RE.test(declared) && !usedGroups.has(declared)
        ? declared
        : `modal-${m.id.slice(-6)}`;
    usedGroups.add(group);
    modalGroup.set(m.id, group);
  }
  const modalFn = modalGroup; // 兼容旧引用

  // 页签分组（V2）：tabs 容器 → group 名。同 modal 机制——优先声明
  // sectionKey（round-trip 回写，稳定），否则 tabs-<id尾6> 去重兜底。
  const tabsGroup = new Map<string, string>();
  for (const t of tree.filter((n) => n.type === 'tabs')) {
    const declared = typeof t.props.sectionKey === 'string' ? t.props.sectionKey.trim() : '';
    const group =
      declared && SECTION_KEY_RE.test(declared) && !usedGroups.has(declared)
        ? declared
        : `tabs-${t.id.slice(-6)}`;
    usedGroups.add(group);
    tabsGroup.set(t.id, group);
  }

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
        warnings.push(
          intl.formatMessage(
            {
              id: 'pages.pageStudio.compiler.warning.textSkipped',
              defaultMessage: '文本「{content}」不参与发布（V1）',
            },
            { content: String(node.props.content ?? '') },
          ),
        );
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
          warnings.push(
            intl.formatMessage(
              {
                id: 'pages.pageStudio.compiler.warning.emptyModal',
                defaultMessage: '弹窗「{title}」为空，已忽略',
              },
              { title: String(node.props.title ?? node.id) },
            ),
          );
          continue;
        }
        for (const kid of kids) {
          if (kid.type === 'text') continue; // 文本暂不进弹窗 spec
          emitFnSection(kid, 'dialog', group);
        }
        continue;
      }
      if (node.type === 'tabs') {
        // 页签容器（V2）：children=container（每页 title=页签标签）；页内
        // 区块平铺为 display='tab' + group（同组渲染进同一 Tabs）+ tab（页签）。
        const group = tabsGroup.get(node.id) ?? '';
        const pages = (node.children ?? []).filter((p) => p.type === 'container');
        const emitted = pages.some((p) => (p.children ?? []).some((k) => k.type !== 'text'));
        if (!emitted) {
          warnings.push(
            intl.formatMessage(
              {
                id: 'pages.pageStudio.compiler.warning.emptyTabs',
                defaultMessage: '页签容器「{title}」为空，已忽略',
              },
              { title: String(node.props.sectionKey ?? node.id) },
            ),
          );
          continue;
        }
        pages.forEach((page, pi) => {
          const tabLabel =
            typeof page.props.title === 'string' && page.props.title.trim()
              ? page.props.title
              : `页签 ${pi + 1}`;
          for (const kid of page.children ?? []) {
            if (kid.type === 'text') continue; // 文本暂不进页签 spec（同弹窗）
            if (kid.type === 'staticForm') emitStaticSection(kid, 'tab', group, tabLabel);
            else if (kid.type === 'button') compileButton(kid);
            else emitFnSection(kid, 'tab', group, tabLabel);
          }
        });
        continue;
      }
      if (node.type === 'button') {
        compileButton(node);
        continue;
      }

      if (node.type === 'staticForm') {
        emitStaticSection(node);
        continue;
      }
      if (VIEW_MAP[node.type]) {
        emitFnSection(node, node.props.display === 'dialog' ? 'dialog' : 'inline');
        continue;
      }
      warnings.push(
        intl.formatMessage(
          {
            id: 'pages.pageStudio.compiler.warning.unknownNodeType',
            defaultMessage: '未知组件类型 {type}，已忽略',
          },
          { type: node.type },
        ),
      );
    }
  };

  /** 常量表单（staticForm）：无契约/绑定，schema 由编辑器设计期定义。
   * V2：可落在页签页内（display='tab' + group/tab 标注聚合位置）。 */
  const emitStaticSection = (
    node: PageNode,
    display: 'inline' | 'tab' = 'inline',
    group?: string,
    tab?: string,
  ) => {
    const raw = node.props.staticSchema;
    let jsonSchema: Record<string, unknown> = {};
    if (typeof raw === 'string') {
      try {
        jsonSchema = JSON.parse(raw) as Record<string, unknown>;
      } catch {
        warnings.push(
          intl.formatMessage(
            {
              id: 'pages.pageStudio.compiler.warning.invalidStaticSchema',
              defaultMessage: '常量表单「{title}」的 JSON 定义无效，已跳过',
            },
            { title: String(node.props.title ?? node.id) },
          ),
        );
        return;
      }
    } else if (raw && typeof raw === 'object') {
      jsonSchema = raw as Record<string, unknown>;
    }
    const section: CompiledSection = {
      key: nodeSectionKey.get(node.id) ?? String(node.id),
      functionId: '',
      view: 'form',
      title: String(node.props.title ?? '常量表单'),
      span: Number(node.props.span ?? 12) || 12,
      autoRun: false,
      static: true,
      form: { jsonSchema },
      ...(display === 'tab'
        ? { display, ...(group ? { group } : {}), ...(tab ? { tab } : {}) }
        : {}),
    };
    // refreshOn 透传（字面 section key；回读时写入 props.refreshOn）
    const staticRefreshOn = Array.isArray(node.props.refreshOn)
      ? (node.props.refreshOn as unknown[]).map(String).filter(Boolean)
      : [];
    if (staticRefreshOn.length) section.refreshOn = staticRefreshOn;
    sections.push(section as unknown as (typeof sections)[number]);
  };

  const emitFnSection = (
    node: PageNode,
    display: 'inline' | 'dialog' | 'tab',
    group?: string,
    tab?: string,
  ) => {
    const fid = String(node.props.functionId ?? '');
    if (!fid) {
      warnings.push(
        intl.formatMessage(
          {
            id: 'pages.pageStudio.compiler.warning.missingFunctionBinding',
            defaultMessage: '组件「{title}」没有绑定函数，已忽略',
          },
          { title: String(node.props.title ?? node.id) },
        ),
      );
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
      // 页签标签（display=tab）：渲染端同 group 内按此聚合到 Tabs 对应页
      ...(display === 'tab' && tab ? { tab } : {}),
    };
    // 成功后刷新：events.success 为唯一规范路径（渲染端 runChain 支持
    // refreshNode/runBinding/navigate/showMessage/closeModal，非刷新动作
    // 由事件链编译，不再被忽略）。section.onSuccessRefresh 仅承载遗留
    // prop 中未被 success 事件覆盖的刷新目标——同一目标双写会导致发布端
    // 一次提交触发两次重跑，且回读→再编译数组膨胀（无去重累积）。
    const refreshTargetsOf = (raw: unknown): string[] => {
      const a = parseAction(raw);
      if (!a) return [];
      const targets: string[] = [];
      const push = (target: unknown) => {
        const t = findNode(tree, target as string);
        const k = t && sectionKeyOf(t);
        if (k) targets.push(k);
      };
      if (a.kind === 'refreshNode') push(a.target);
      for (const step of (raw as { chain?: Array<{ kind: string; target: string }> })?.chain ??
        []) {
        if (step.kind === 'refreshNode') push(step.target);
      }
      return targets;
    };
    const successTargets = new Set(refreshTargetsOf(node.props.onSuccess));
    const legacyTargets = refreshTargetsOf(node.props.onSuccessRefresh).filter(
      (k) => !successTargets.has(k),
    );
    if (legacyTargets.length > 0) {
      section.onSuccessRefresh = [...new Set(legacyTargets)];
    }
    // refreshOn：refreshOnNode=模板内部节点 id 引用（实例化时重映射），
    // 解析为本区块的 section key；refreshOn=字面 section key（高级）。
    const refreshOnKeys: string[] = [];
    for (const nid of (Array.isArray(node.props.refreshOnNode)
      ? (node.props.refreshOnNode as unknown[])
      : []) as string[]) {
      const key = nodeSectionKey.get(nid);
      if (key) refreshOnKeys.push(key);
    }
    if (Array.isArray(node.props.refreshOn)) {
      for (const k of node.props.refreshOn as unknown[]) {
        const key = String(k);
        if (key && !refreshOnKeys.includes(key)) refreshOnKeys.push(key);
      }
    }
    if (refreshOnKeys.length > 0) section.refreshOn = refreshOnKeys;

    // 显式参数映射：sourceNodeId → section key（与 refreshOnNode 同机制）
    const rawMappings = Array.isArray(node.props.inputAssignments)
      ? (node.props.inputAssignments as Array<Record<string, unknown>>)
      : [];
    const inputAssignments = rawMappings
      .map(
        (
          m,
        ): {
          target: string;
          kind: 'page_state' | 'literal';
          key?: string;
          path?: string;
          value?: unknown;
        } | null => {
          if (m.kind !== 'literal' && m.kind !== 'page_state') {
            // 未知映射类型：静默归 page_state 会在发布后求值失败，显式警告并跳过
            warnings.push(
              intl.formatMessage(
                {
                  id: 'pages.pageStudio.compiler.warning.unknownMappingKind',
                  defaultMessage: '区块「{title}」参数「{param}」的映射类型「{kind}」未知，已跳过',
                },
                {
                  title: String(section.title ?? node.id),
                  param: String(m.param),
                  kind: String(m.kind),
                },
              ),
            );
            return null;
          }
          const kind: 'page_state' | 'literal' = m.kind;
          const target = String(m.param ?? '').startsWith('/')
            ? String(m.param)
            : `/${String(m.param ?? '')}`;
          if (kind === 'page_state') {
            const key = nodeSectionKey.get(String(m.sourceNodeId ?? ''));
            if (!key) {
              // 来源节点已删/失效：静默 null 会让映射悄悄丢失，显式警告
              warnings.push(
                intl.formatMessage(
                  {
                    id: 'pages.pageStudio.compiler.warning.mappingSourceInvalid',
                    defaultMessage: '区块「{title}」参数「{param}」的来源节点已失效，已跳过',
                  },
                  { title: String(section.title ?? node.id), param: String(m.param) },
                ),
              );
              return null;
            }
            return {
              target,
              kind,
              key,
              path:
                typeof m.field === 'string' && m.field
                  ? `/${m.field.replace(/^\//, '')}`
                  : undefined,
            };
          }
          // V5：字面值为单表达式 {{var.path}} → 编译为 page_state（§6 编译规则）
          if (typeof m.value === 'string' && isSingleExpression(m.value)) {
            const parsed = parseExpression(m.value, sectionKeySet);
            if (parsed.ok && parsed.ref.variable !== ROW_VARIABLE) {
              return {
                target,
                kind: 'page_state',
                key: parsed.ref.variable,
                path: pathToPointer(parsed.ref.path),
              };
            }
            if (!parsed.ok || parsed.ref.variable === ROW_VARIABLE) {
              warnings.push(
                intl.formatMessage(
                  {
                    id: 'pages.pageStudio.compiler.warning.expressionUnknownVariable',
                    defaultMessage:
                      '区块「{title}」参数「{param}」的表达式「{value}」引用未知变量或行上下文，已按字面量保存',
                  },
                  {
                    title: String(section.title ?? node.id),
                    param: String(m.param),
                    value: String(m.value),
                  },
                ),
              );
            }
          }
          return { target, kind, value: m.value };
        },
      )
      .filter((m): m is NonNullable<typeof m> => m !== null);
    if (inputAssignments.length > 0) section.inputAssignments = inputAssignments;

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
          warnings.push(
            intl.formatMessage(
              {
                id: 'pages.pageStudio.compiler.warning.rowActionMissingTarget',
                defaultMessage: '表格「{title}」有未配置目标的行操作，已忽略',
              },
              { title: section.title },
            ),
          );
          continue;
        }
        const ra: CompiledAction = {
          label: String(raw.label ?? ''),
          targetSection: sectionTarget,
          params: normalizeRowActionParams(
            raw.params as Record<string, string> | undefined,
            section.title,
            sectionKeySet,
            warnings,
          ),
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
      warnings.push(
        intl.formatMessage(
          {
            id: 'pages.pageStudio.compiler.warning.buttonNoAction',
            defaultMessage: '按钮「{title}」没有配置动作，已忽略',
          },
          { title: String(node.props.title ?? '') },
        ),
      );
      return;
    }
    const targetNode = act?.target ? findNode(tree, act.target) : undefined;
    if (targetNode?.type === 'modal' && !(targetNode.children ?? []).length) {
      warnings.push(
        intl.formatMessage(
          {
            id: 'pages.pageStudio.compiler.warning.buttonEmptyModalTarget',
            defaultMessage: '按钮「{title}」的弹窗目标无效（空弹窗），已忽略',
          },
          { title: String(node.props.title ?? '') },
        ),
      );
      return;
    }
    // 挂到最近一个表格 section 的 toolbarActions
    const lastTable = [...sections].reverse().find((s) => s.view === 'table');
    if (!lastTable) {
      warnings.push(
        intl.formatMessage(
          {
            id: 'pages.pageStudio.compiler.warning.buttonAfterTable',
            defaultMessage: '按钮「{title}」需放置在表格之后（编译为表格顶部按钮），已忽略',
          },
          { title: String(node.props.title ?? '') },
        ),
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
        warnings.push(
          intl.formatMessage(
            {
              id: 'pages.pageStudio.compiler.warning.buttonTargetInvalid',
              defaultMessage: '按钮「{title}」动作目标无效，已忽略',
            },
            { title: String(node.props.title ?? '') },
          ),
        );
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
      warnings.push(
        intl.formatMessage(
          {
            id: 'pages.pageStudio.compiler.warning.buttonModalTargetInvalid',
            defaultMessage: '按钮「{title}」的弹窗目标无效，已忽略',
          },
          { title: String(node.props.title ?? '') },
        ),
      );
      return;
    }
    const ta: CompiledAction = {
      label: String(node.props.title ?? '操作'),
      targetSection: targetKey,
    };
    // openModal 主动作的 params（弹窗预填初值）——wire 支持此字段，此前被丢弃
    if (actParams) ta.params = actParams;
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
