import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Empty, Input, Space, Spin, Tag, Typography } from 'antd';
import { AppstoreOutlined, SearchOutlined } from '@ant-design/icons';
import { useDraggable } from '@dnd-kit/core';
import { request } from '@umijs/max';
import { nodeId, type PageNode } from './model';
import type { FunctionDescriptor } from '@/services/api/functions';

const { Text, Title } = Typography;

/** 组件模板 DTO（后端 /api/v1/component-templates）。 */
export interface ComponentTemplateDTO {
  key: string;
  name: { 'zh-CN'?: string; 'en-US'?: string } | Record<string, unknown>;
  description?: { 'zh-CN'?: string } | Record<string, unknown>;
  category?: string;
  icon?: string;
  requiredFunctions?: string[];
  tree: PageNode[];
  builtin: boolean;
  /** 契约已变化，builtin 模板需「从契约重新生成」刷新。 */
  stale?: boolean;
}

/** 拉取组件模板列表。 */
async function fetchTemplates(): Promise<ComponentTemplateDTO[]> {
  const resp = (await request('/api/v1/component-templates', {
    skipErrorHandler: true,
  })) as { items?: ComponentTemplateDTO[] } | ComponentTemplateDTO[];
  const items = Array.isArray(resp) ? resp : (resp?.items ?? []);
  return items;
}

/**
 * 实例化组件模板：复制子树 + 重分配 id + 重映射内部引用。
 * 引用形态：onClick/onSuccess/onRowClick 的 target=节点 id；rowActions.targetSection=节点 id。
 */
export function instantiateTemplate(tpl: ComponentTemplateDTO): PageNode[] {
  const idMap = new Map<string, string>();
  const clone = (node: PageNode): PageNode => {
    const newId = nodeId(node.type);
    idMap.set(node.id, newId);
    const props = { ...node.props } as Record<string, unknown>;
    // 重映射事件引用
    for (const key of Object.keys(props)) {
      if (key.startsWith('on') && props[key] && typeof props[key] === 'object') {
        const action = props[key] as { target?: string; chain?: Array<{ target?: string }> };
        if (action.target && idMap.has(action.target)) {
          action.target = idMap.get(action.target);
        }
        for (const step of action.chain ?? []) {
          if (step.target && idMap.has(step.target)) {
            step.target = idMap.get(step.target);
          }
        }
      }
      // refreshOnNode：模板内部节点 id 引用 → 重映射为新树节点 id
      if (key === 'refreshOnNode' && Array.isArray(props[key])) {
        props[key] = (props[key] as string[]).map((nid) => idMap.get(nid) ?? nid);
      }
      // inputAssignments：sourceNodeId 同理重映射
      if (key === 'inputAssignments' && Array.isArray(props[key])) {
        props[key] = (props[key] as Array<{ sourceNodeId?: string } & Record<string, unknown>>).map(
          (m) => ({ ...m, sourceNodeId: idMap.get(m.sourceNodeId ?? '') ?? m.sourceNodeId }),
        );
      }
      if (key === 'rowActions' && Array.isArray(props[key])) {
        for (const ra of props[key] as Array<{ targetSection?: string }>) {
          if (ra.targetSection && idMap.has(ra.targetSection)) {
            ra.targetSection = idMap.get(ra.targetSection);
          }
        }
      }
    }
    return {
      ...node,
      id: newId,
      props,
      children: node.children?.map(clone),
    };
  };
  return (tpl.tree ?? []).map(clone);
}

/** 组件库面板：浏览/搜索/点击拖入组件模板。 */
export default function ComponentLibrary({
  availableFnIds,
  onInsert,
}: {
  /** 当前 scope 可用的函数 id 集合（检查组件依赖）。 */
  availableFnIds: Set<string>;
  onInsert: (nodes: PageNode[], template: ComponentTemplateDTO) => void;
}) {
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
      const name = (t.name as Record<string, string>)?.['zh-CN'] ?? t.key;
      return (
        name.toLowerCase().includes(q) ||
        (t.category ?? '').toLowerCase().includes(q) ||
        t.key.toLowerCase().includes(q)
      );
    });
  }, [templates, search]);

  const grouped = useMemo(() => {
    const byCat = new Map<string, ComponentTemplateDTO[]>();
    for (const t of filtered) {
      const cat = t.category || (t.builtin ? '内置' : '自定义');
      if (!byCat.has(cat)) byCat.set(cat, []);
      byCat.get(cat)!.push(t);
    }
    return Array.from(byCat.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [filtered]);

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
          加载组件库…
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
              暂无组件模板
            </Text>
            <Text type="secondary" style={{ fontSize: 11 }}>
              选中画布多个节点 → 右键「保存为组件」可创建
            </Text>
          </Space>
        }
        style={{ marginTop: 40 }}
      />
    );
  }

  return (
    <div>
      <Input
        size="small"
        allowClear
        prefix={<SearchOutlined style={{ color: '#999' }} />}
        placeholder="搜索组件"
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
            const name = (tpl.name as Record<string, string>)?.['zh-CN'] ?? tpl.key;
            const desc = (tpl.description as Record<string, string>)?.['zh-CN'] ?? '';
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
                  {tpl.builtin && <Tag style={{ marginRight: 0, fontSize: 10 }}>内置</Tag>}
                  {tpl.stale && (
                    <Tag color="orange" style={{ marginRight: 0, fontSize: 10 }}>
                      已过期
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
                {!ok && (
                  <div>
                    <Text type="danger" style={{ fontSize: 11 }}>
                      缺少函数：{missing.join(', ')}
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
          管理组件模板 →
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
  onInsert: (nodes: PageNode[], template: ComponentTemplateDTO) => void;
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
        onInsert(instantiateTemplate(tpl), tpl);
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
