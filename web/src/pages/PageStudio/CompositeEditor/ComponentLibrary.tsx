import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Button, Empty, Input, Space, Spin, Tag, Typography } from 'antd';
import { AppstoreOutlined, PlusOutlined, SearchOutlined } from '@ant-design/icons';
import { useDraggable } from '@dnd-kit/core';
import { request, useIntl } from '@umijs/max';
import { nodeId, type PageNode } from './model';
import TemplateThumb from './TemplateThumb';
import type { FunctionDescriptor } from '@/services/api/functions';
import { localizedText } from '@/utils/localizedText';
import type { LocalizedText } from '@/types/dashboard';

const { Text, Title } = Typography;

/** 组件模板 DTO（后端 /api/v1/component-templates）。 */
export interface ComponentTemplateDTO {
  key: string;
  name: LocalizedText;
  description?: LocalizedText;
  category?: string;
  icon?: string;
  requiredFunctions?: string[];
  /** 参数定义（U6）：实例化时按值替换对应节点白名单 prop。 */
  params?: ComponentTemplateParam[];
  tree: PageNode[];
  builtin: boolean;
  /** 模板内容指纹（U11 更新提醒）：页面快照与之比对得出「有新版本」。
   * 旧后端/未迁移行可能缺失。 */
  digest?: string;
  /** 最近一次内容更新时间（U11）。 */
  updatedAt?: string;
  /** 契约已变化，builtin 模板需「从契约重新生成」刷新。 */
  stale?: boolean;
}

/** 模板参数定义（与后端 TemplateParam 对齐）。 */
export interface ComponentTemplateParam {
  key: string;
  label?: LocalizedText;
  nodeId: string;
  prop: string;
  default?: unknown;
}

/** 拉取组件模板列表。 */
async function fetchTemplates(): Promise<ComponentTemplateDTO[]> {
  const resp = (await request('/api/v1/component-templates', {
    skipErrorHandler: true,
  })) as { items?: ComponentTemplateDTO[] } | ComponentTemplateDTO[];
  const items = Array.isArray(resp) ? resp : (resp?.items ?? []);
  return items;
}

/** U7：实例化后仍指向模板外的悬空引用（重映射保留旧 id 的分支）。 */
export interface DanglingTemplateRef {
  /** 携带悬空引用的新节点 id */
  nodeId: string;
  /** 节点标题（提示定位；兜底组件类型名） */
  nodeTitle: string;
  /** 引用所在 prop（onClick / refreshOnNode / inputAssignments / rowActions） */
  prop: string;
  /** 引用种类：动作目标 / 联动依赖 / 参数映射来源 / 行操作弹窗目标 */
  kind: 'action' | 'refresh' | 'assignment' | 'rowAction';
  /** 悬空的原始引用值（模板外节点 id，实例化后保留） */
  ref: string;
  /** 落点描述（主动作 / 链第 N 步 / 第 N 个映射 / 第 N 个行操作） */
  detail?: string;
}

/**
 * 实例化组件模板：复制子树 + 重分配 id + 重映射内部引用 + 应用参数值（U6）。
 * 引用形态：onClick/onSuccess/onRowClick 的 target=节点 id；rowActions.targetSection=节点 id。
 * 两遍克隆：先预分配全部新 id（引用重映射与前出现顺序无关），再复制重映射；
 * 嵌套对象拷贝后重映射（不污染模板缓存）。参数按 nodeId→新 id 替换白名单
 * prop（title/span/autoRun），未提供的参数回退 default。
 */
export function instantiateTemplate(
  tpl: ComponentTemplateDTO,
  paramValues?: Record<string, unknown>,
): PageNode[] {
  return instantiateTemplateDetailed(tpl, paramValues).nodes;
}

/**
 * 实例化（含悬空引用检出，U7）：模板内部引用经 idMap 重映射；指向模板外
 * 节点的引用（保存模板时画布上的其他区块）无法重映射，保留旧 id 并登记
 * 进 dangling 清单——编辑器据此提示「联动已断开」并提供快捷重连，
 * 不再静默丢失（此前仅保存时编译警告兜底）。
 */
export function instantiateTemplateDetailed(
  tpl: ComponentTemplateDTO,
  paramValues?: Record<string, unknown>,
): { nodes: PageNode[]; dangling: DanglingTemplateRef[] } {
  const idMap = new Map<string, string>();
  const preassign = (nodes: PageNode[]) => {
    for (const node of nodes) {
      idMap.set(node.id, nodeId(node.type));
      if (node.children) preassign(node.children);
    }
  };
  preassign(tpl.tree ?? []);

  // 参数值按新节点 id 应用（nodeId→paramValues/prop 覆盖，缺省回退 default）
  const applyParams = (node: PageNode, props: Record<string, unknown>) => {
    for (const param of tpl.params ?? []) {
      if (param.nodeId !== node.id) continue;
      if (paramValues && param.key in paramValues) {
        props[param.prop] = paramValues[param.key];
      } else if (param.default !== undefined) {
        props[param.prop] = param.default;
      }
    }
  };

  const dangling: DanglingTemplateRef[] = [];
  const report = (
    node: PageNode,
    prop: string,
    kind: DanglingTemplateRef['kind'],
    ref: string,
    detail?: string,
  ) => {
    dangling.push({
      nodeId: idMap.get(node.id) ?? node.id,
      nodeTitle: String(node.props.title ?? node.type),
      prop,
      kind,
      ref,
      detail,
    });
  };

  const clone = (node: PageNode): PageNode => {
    const props = { ...node.props } as Record<string, unknown>;
    // 区块 key 不随模板复制（实例各自分配，避免多实例冲突；U5）
    delete props.sectionKey;
    applyParams(node, props);
    for (const key of Object.keys(props)) {
      if (key.startsWith('on') && props[key] && typeof props[key] === 'object') {
        const a = props[key] as { target?: string; chain?: Array<{ target?: string }> };
        const copied: { target?: string; chain?: Array<{ target?: string }> } = {
          ...a,
          chain: a.chain?.map((s) => ({ ...s })),
        };
        if (copied.target) {
          if (idMap.has(copied.target)) {
            copied.target = idMap.get(copied.target);
          } else {
            report(node, key, 'action', copied.target);
          }
        }
        for (const [i, step] of (copied.chain ?? []).entries()) {
          if (!step.target) continue;
          if (idMap.has(step.target)) {
            step.target = idMap.get(step.target);
          } else {
            report(node, key, 'action', step.target, `chain ${i + 1}`);
          }
        }
        props[key] = copied;
      }
      // refreshOnNode：模板内部节点 id 引用 → 重映射为新树节点 id
      if (key === 'refreshOnNode' && Array.isArray(props[key])) {
        props[key] = (props[key] as string[]).map((nid) => {
          const mapped = idMap.get(nid);
          if (mapped) return mapped;
          report(node, key, 'refresh', nid);
          return nid;
        });
      }
      // inputAssignments：sourceNodeId 同理重映射
      if (key === 'inputAssignments' && Array.isArray(props[key])) {
        props[key] = (props[key] as Array<{ sourceNodeId?: string } & Record<string, unknown>>).map(
          (m, i) => {
            if (!m.sourceNodeId) return { ...m };
            const mapped = idMap.get(m.sourceNodeId);
            if (mapped) return { ...m, sourceNodeId: mapped };
            report(node, key, 'assignment', m.sourceNodeId, `assignment ${i + 1}`);
            return { ...m };
          },
        );
      }
      if (key === 'rowActions' && Array.isArray(props[key])) {
        props[key] = (
          props[key] as Array<{ targetSection?: string } & Record<string, unknown>>
        ).map((ra, i) => {
          if (!ra.targetSection || idMap.has(ra.targetSection)) {
            return {
              ...ra,
              targetSection:
                ra.targetSection && idMap.has(ra.targetSection)
                  ? idMap.get(ra.targetSection)
                  : ra.targetSection,
            };
          }
          report(node, key, 'rowAction', ra.targetSection, `rowAction ${i + 1}`);
          return { ...ra };
        });
      }
    }
    return {
      ...node,
      id: idMap.get(node.id) ?? node.id,
      props,
      children: node.children?.map(clone),
    };
  };
  return { nodes: (tpl.tree ?? []).map(clone), dangling };
}

/** 一条重连修复（U7）：把携带悬空引用节点里的旧引用值替换为目标节点 id。 */
export interface TemplateRefFix {
  nodeId: string;
  kind: DanglingTemplateRef['kind'];
  prop: string;
  /** 旧引用值（按值匹配替换；未命中的引用不动） */
  ref: string;
  /** 重连目标节点 id */
  target: string;
}

/** 把悬空引用重接到画布节点：按 nodeId 定位节点、按 prop/kind 找引用位、
 * 按旧值（ref）匹配替换。纯函数（不修改入参）；无命中返回原数组。 */
export function reconnectTemplateRefs(nodes: PageNode[], fixes: TemplateRefFix[]): PageNode[] {
  if (fixes.length === 0) return nodes;
  const applyFix = (value: unknown, fix: TemplateRefFix): unknown => {
    switch (fix.kind) {
      case 'action': {
        const a = value as { target?: string; chain?: Array<{ target?: string }> };
        const copied: { target?: string; chain?: Array<{ target?: string }> } = {
          ...a,
          chain: a.chain?.map((s) => ({ ...s })),
        };
        if (copied.target === fix.ref) copied.target = fix.target;
        for (const step of copied.chain ?? []) {
          if (step.target === fix.ref) step.target = fix.target;
        }
        return copied;
      }
      case 'refresh':
        return (value as string[]).map((v) => (v === fix.ref ? fix.target : v));
      case 'assignment':
        return (value as Array<{ sourceNodeId?: string }>).map((m) =>
          m.sourceNodeId === fix.ref ? { ...m, sourceNodeId: fix.target } : m,
        );
      case 'rowAction':
        return (value as Array<{ targetSection?: string }>).map((ra) =>
          ra.targetSection === fix.ref ? { ...ra, targetSection: fix.target } : ra,
        );
      default:
        return value;
    }
  };
  const walk = (list: PageNode[]): PageNode[] =>
    list.map((node) => {
      const mine = fixes.filter((f) => f.nodeId === node.id);
      const props = mine.length > 0 ? { ...(node.props as Record<string, unknown>) } : node.props;
      for (const fix of mine) {
        if (fix.prop in props) props[fix.prop] = applyFix(props[fix.prop], fix);
      }
      return node.children
        ? { ...node, props, children: walk(node.children) }
        : mine.length > 0
          ? { ...node, props }
          : node;
    });
  return walk(nodes);
}

/** 组件库面板：浏览/搜索/点击拖入组件模板。 */
export default function ComponentLibrary({
  availableFnIds,
  onInsert,
  onCreateFromCanvas,
}: {
  /** 当前 scope 可用的函数 id 集合（检查组件依赖）。 */
  availableFnIds: Set<string>;
  onInsert: (
    nodes: PageNode[],
    template: ComponentTemplateDTO,
    dangling?: DanglingTemplateRef[],
  ) => void;
  /** 「从画布选中创建」入口（发现性 V1）：编辑器接线保存流程。 */
  onCreateFromCanvas?: () => void;
}) {
  const intl = useIntl();
  const [templates, setTemplates] = useState<ComponentTemplateDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState('');

  useEffect(() => {
    setLoading(true);
    fetchTemplates()
      .then(setTemplates)
      .catch(() => setTemplates([]))
      .finally(() => setLoading(false));
  }, []);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return templates;
    return templates.filter((t) => {
      // 搜索匹配与展示同源：走 localizedText 统一回退
      const name = localizedText(t.name, 'zh-CN', t.key);
      return (
        name.toLowerCase().includes(q) ||
        (t.category ?? '').toLowerCase().includes(q) ||
        t.key.toLowerCase().includes(q)
      );
    });
  }, [templates, search]);

  const grouped = useMemo(() => {
    const byCat = new Map<string, ComponentTemplateDTO[]>();
    // 分类缺失时的展示兜底：内置/自定义（与 templates.tag.builtin 同语义的分组名）
    const builtinCat = intl.formatMessage({
      id: 'pages.pageStudio.editor.library.category.builtin',
      defaultMessage: '内置',
    });
    const customCat = intl.formatMessage({
      id: 'pages.pageStudio.editor.library.category.custom',
      defaultMessage: '自定义',
    });
    for (const t of filtered) {
      const cat = t.category || (t.builtin ? builtinCat : customCat);
      if (!byCat.has(cat)) byCat.set(cat, []);
      byCat.get(cat)!.push(t);
    }
    return Array.from(byCat.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [filtered, intl]);

  const checkAvailable = useCallback(
    (tpl: ComponentTemplateDTO): { ok: boolean; missing: string[] } => {
      const missing = (tpl.requiredFunctions ?? []).filter((fid) => !availableFnIds.has(fid));
      return { ok: missing.length === 0, missing };
    },
    [availableFnIds],
  );

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 40 }}>
        <Spin size="small" />
        <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
          {intl.formatMessage({
            id: 'pages.pageStudio.editor.library.loading',
            defaultMessage: '加载组件库…',
          })}
        </Text>
      </div>
    );
  }

  if (templates.length === 0) {
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={
          <Space orientation="vertical" size={4}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {intl.formatMessage({
                id: 'pages.pageStudio.editor.library.empty',
                defaultMessage: '暂无组件模板',
              })}
            </Text>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {intl.formatMessage({
                id: 'pages.pageStudio.editor.library.emptyHint',
                defaultMessage: '选中画布多个节点 → 顶栏「保存为组件」可创建',
              })}
            </Text>
          </Space>
        }
        style={{ marginTop: 40 }}
      />
    );
  }

  return (
    <div>
      {onCreateFromCanvas && (
        <Button
          size="small"
          block
          icon={<PlusOutlined />}
          onClick={onCreateFromCanvas}
          style={{ marginBottom: 8 }}
        >
          {intl.formatMessage({
            id: 'pages.pageStudio.editor.library.createFromCanvas',
            defaultMessage: '从画布选中创建',
          })}
        </Button>
      )}
      <Input
        size="small"
        allowClear
        prefix={<SearchOutlined style={{ color: '#999' }} />}
        placeholder={intl.formatMessage({
          id: 'pages.pageStudio.editor.library.searchPlaceholder',
          defaultMessage: '搜索组件',
        })}
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        style={{ marginBottom: 8 }}
      />
      {grouped.map(([category, items]) => (
        <div key={category} style={{ marginBottom: 12 }}>
          <Title level={5} style={{ fontSize: 12, marginBottom: 4, color: '#666' }}>
            {category}
          </Title>
          {items.map((tpl) => {
            const { ok, missing } = checkAvailable(tpl);
            const name = localizedText(tpl.name, 'zh-CN', tpl.key);
            const desc = localizedText(tpl.description, 'zh-CN');
            return (
              <TemplateDraggable
                key={tpl.key}
                tpl={tpl}
                missing={ok ? [] : missing}
                onInsert={onInsert}
              >
                <Space size={6}>
                  <AppstoreOutlined style={{ color: '#1677ff' }} />
                  <Text strong style={{ fontSize: 12 }}>
                    {name}
                  </Text>
                  {tpl.builtin && (
                    <Tag style={{ marginRight: 0, fontSize: 10 }}>
                      {intl.formatMessage({
                        id: 'pages.pageStudio.templates.tag.builtin',
                        defaultMessage: '内置',
                      })}
                    </Tag>
                  )}
                  {tpl.stale && (
                    <Tag color="orange" style={{ marginRight: 0, fontSize: 10 }}>
                      {intl.formatMessage({
                        id: 'pages.pageStudio.templates.tag.stale',
                        defaultMessage: '已过期',
                      })}
                    </Tag>
                  )}
                </Space>
                {desc && (
                  <div>
                    <Text type="secondary" style={{ fontSize: 11 }}>
                      {desc}
                    </Text>
                  </div>
                )}
                <TemplateThumb tree={tpl.tree ?? []} />
                {!ok && (
                  <div>
                    <Text type="danger" style={{ fontSize: 11 }}>
                      {intl.formatMessage(
                        {
                          id: 'pages.pageStudio.editor.library.missingFunctions',
                          defaultMessage: '缺少函数：{fns}',
                        },
                        { fns: missing.join(', ') },
                      )}
                    </Text>
                  </div>
                )}
              </TemplateDraggable>
            );
          })}
        </div>
      ))}
      <div style={{ marginTop: 8, textAlign: 'center' }}>
        <a
          href="/functions/component-templates"
          target="_blank"
          rel="noreferrer"
          style={{ fontSize: 12 }}
        >
          {intl.formatMessage({
            id: 'pages.pageStudio.editor.library.manageLink',
            defaultMessage: '管理组件模板 →',
          })}
        </a>
      </div>
    </div>
  );
}

function TemplateDraggable({
  tpl,
  missing,
  onInsert,
  children,
}: {
  tpl: ComponentTemplateDTO;
  missing: string[];
  onInsert: (
    nodes: PageNode[],
    template: ComponentTemplateDTO,
    dangling?: DanglingTemplateRef[],
  ) => void;
  children: React.ReactNode;
}) {
  // id 必须稳定（同 PanelDraggable 注释）：isDragging 重渲染时 id 变化
  // 会导致 onDragEnd 拿不到 data，拖入静默失败。
  const idRef = useRef(`panel:tpl:${tpl.key}`);
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: idRef.current,
    data: { source: 'panel', kind: 'template', tpl, missing },
  });
  const ok = missing.length === 0;
  return (
    <div
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      onClick={() => {
        if (!ok) return;
        // 带参数模板（U6）：抛给父级弹参数表单（onInsert([], tpl)），确认后实例化
        if (tpl.params?.length) {
          onInsert([], tpl);
          return;
        }
        const { nodes, dangling } = instantiateTemplateDetailed(tpl);
        onInsert(nodes, tpl, dangling);
      }}
      style={{
        border: '1px solid #f0f0f0',
        borderRadius: 6,
        padding: '8px 10px',
        marginBottom: 6,
        cursor: ok ? 'grab' : 'not-allowed',
        opacity: isDragging ? 0.4 : ok ? 1 : 0.5,
        background: '#fff',
        touchAction: 'none',
      }}
    >
      {children}
    </div>
  );
}
