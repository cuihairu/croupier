import React, { useMemo, useRef, useState } from 'react';
import { useDraggable } from '@dnd-kit/core';
import { Empty, Input, Space, Typography } from 'antd';
import { SearchOutlined } from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { Tree } from 'antd';
import type { FunctionDescriptor } from '@/services/api/functions';
import { allComponents } from './registry';
import { getScope, setScope, subscribeScope } from '@/stores/scope';
import { getMyGames } from '@/services/api/me';
import { request } from '@umijs/max';
import { listDescriptors } from '@/services/api/functions';
import { defaultView } from './types';
import { viewTypeToComponent } from './components/builtin';

const { Text, Title } = Typography;

export interface AddFnEvent {
  fn: FunctionDescriptor;
  componentType: 'fnTable' | 'fnFields' | 'fnForm';
}

async function probeDescriptorCount(gameId: string, env: string): Promise<number> {
  try {
    const resp = await request<{ functions?: unknown[] } | unknown[]>(
      '/api/v1/functions/descriptors',
      { headers: { 'X-Game-ID': gameId, 'X-Env': env }, skipErrorHandler: true },
    );
    const list = Array.isArray(resp) ? resp : (resp?.functions ?? []);
    return Array.isArray(list) ? list.length : 0;
  } catch {
    return 0;
  }
}

/** 空态 scope 引导（T0.1 行为迁移至此）。 */
function ScopeGuide({ onReload }: { onReload: () => void }) {
  const [scopes, setScopes] = useState<{ gameId: string; env: string; count: number }[]>([]);
  const [probing, setProbing] = useState(true);
  const current = getScope();

  React.useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        const { games } = await getMyGames();
        const tasks: { gameId: string; env: string }[] = [];
        for (const g of games ?? [])
          for (const env of g.envs ?? []) tasks.push({ gameId: g.gameId ?? '', env });
        const results = await Promise.all(
          tasks
            .slice(0, 16)
            .map(async (t) => ({ ...t, count: await probeDescriptorCount(t.gameId, t.env) })),
        );
        if (alive) setScopes(results.filter((r) => r.count > 0 && r.gameId));
      } catch {
        if (alive) setScopes([]);
      } finally {
        if (alive) setProbing(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  return (
    <div style={{ padding: 8 }}>
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={`当前 scope（${current.gameId ?? '?'}/${current.env ?? '?'}）没有函数契约`}
      />
      {probing ? (
        <Text type="secondary" style={{ fontSize: 11 }}>
          正在探测各 scope 函数分布…
        </Text>
      ) : scopes.filter((s) => s.gameId !== current.gameId || s.env !== current.env).length ===
        0 ? (
        <Text type="secondary" style={{ fontSize: 11 }}>
          其他 scope 也没有函数——请先通过 SDK/OpenAPI 注册函数
        </Text>
      ) : (
        <Space direction="vertical" size={4} style={{ width: '100%', marginTop: 8 }}>
          {scopes
            .filter((s) => s.gameId !== current.gameId || s.env !== current.env)
            .map((s) => (
              <a
                key={`${s.gameId}/${s.env}`}
                onClick={() => {
                  setScope({ gameId: s.gameId, env: s.env });
                  onReload();
                }}
                style={{ fontSize: 12 }}
              >
                切换到 {s.gameId}/{s.env}（{s.count} 函数）
              </a>
            ))}
        </Space>
      )}
    </div>
  );
}

/** 组件面板：基础组件网格（scaffold 实例化）+ 函数契约列表（按资源分组）。 */
export default function ComponentPanel({
  onAddBasic,
  onAddFunction,
}: {
  onAddBasic: (type: 'button' | 'modal' | 'container' | 'text') => void;
  onAddFunction: (e: AddFnEvent) => void;
}) {
  const [descriptors, setDescriptors] = useState<FunctionDescriptor[]>([]);
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [reloadKey, setReloadKey] = useState(0);

  React.useEffect(() => {
    void (async () => {
      setLoading(true);
      try {
        setDescriptors(await listDescriptors());
      } catch {
        /* 全局提示 */
      } finally {
        setLoading(false);
      }
    })();
  }, [reloadKey]);

  React.useEffect(() => subscribeScope(() => setReloadKey((k) => k + 1)), []);

  const basics = useMemo(() => allComponents().filter((c) => c.category === 'basic'), []);

  const treeData = useMemo<DataNode[]>(() => {
    const byResource = new Map<string, FunctionDescriptor[]>();
    for (const d of descriptors) {
      const rk = (d.resource || '').trim() || '其他';
      if (!byResource.has(rk)) byResource.set(rk, []);
      byResource.get(rk)!.push(d);
    }
    const q = search.trim().toLowerCase();
    return Array.from(byResource.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([rk, fns]) => ({
        title: `${rk} (${fns.length})`,
        key: rk,
        selectable: false,
        children: fns
          .sort((a, b) => a.id.localeCompare(b.id))
          .filter((f) => !q || f.id.toLowerCase().includes(q))
          .map((f) => ({
            key: f.id,
            title: (
              <PanelDraggable
                data={{
                  source: 'panel',
                  kind: 'fn',
                  fn: f,
                  componentType: viewTypeToComponent(defaultView(f)),
                }}
                label={f.operation || f.id.split('.').pop() || f.id}
                compact
                onClick={() =>
                  onAddFunction({ fn: f, componentType: viewTypeToComponent(defaultView(f)) })
                }
              />
            ),
            isLeaf: true,
          })),
      }))
      .filter((n) => n.children.length > 0);
  }, [descriptors, search]);

  const fnMap = useMemo(() => {
    const m = new Map<string, FunctionDescriptor>();
    for (const d of descriptors) m.set(d.id, d);
    return m;
  }, [descriptors]);

  return (
    <div>
      <Title level={5} style={{ marginTop: 0, marginBottom: 8 }}>
        基础组件
      </Title>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6, marginBottom: 16 }}>
        {basics.map((c) => (
          <a
            key={c.type}
            onClick={() => onAddBasic(c.type as 'button' | 'modal' | 'container' | 'text')}
            style={{
              border: '1px solid #f0f0f0',
              borderRadius: 6,
              padding: '6px 8px',
              fontSize: 12,
              textAlign: 'center',
              background: '#fff',
            }}
          >
            {c.icon} {c.name}
          </a>
        ))}
      </div>

      <Title level={5} style={{ marginBottom: 8 }}>
        函数组件{' '}
        <Text type="secondary" style={{ fontSize: 11 }}>
          {descriptors.length}
        </Text>
      </Title>
      <Input
        size="small"
        allowClear
        prefix={<SearchOutlined style={{ color: '#999' }} />}
        placeholder="搜索函数 / 资源"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        style={{ marginBottom: 8 }}
      />
      {loading ? (
        <Text type="secondary" style={{ fontSize: 11 }}>
          加载中…
        </Text>
      ) : descriptors.length === 0 ? (
        <ScopeGuide onReload={() => setReloadKey((k) => k + 1)} />
      ) : treeData.length === 0 ? (
        <Empty description="无匹配函数" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <Tree
          treeData={treeData}
          defaultExpandAll
          showLine={false}
          blockNode
          onSelect={(keys) => {
            const id = String(keys[0] || '');
            const fn = fnMap.get(id);
            if (fn) onAddFunction({ fn, componentType: viewTypeToComponent(defaultView(fn)) });
          }}
        />
      )}
    </div>
  );
}

/** 面板拖拽源：点击=直接加入画布；拖拽=拖到画布指定落点。 */
function PanelDraggable({
  data,
  label,
  icon,
  compact,
  onClick,
}: {
  data: Record<string, unknown>;
  label: string;
  icon?: React.ReactNode;
  compact?: boolean;
  onClick: () => void;
}) {
  // id 必须稳定：isDragging 触发重渲染时若 id 变化（此前 Math.random()），
  // dnd-kit 的 active 项失效 → onDragEnd 拿不到 data → 拖入画布静默失败（线上实测）。
  const idRef = useRef(`panel:${label}:${Math.random().toString(36).slice(2, 7)}`);
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: idRef.current,
    data,
  });
  return (
    <div
      ref={setNodeRef}
      {...attributes}
      {...listeners}
      onClick={onClick}
      style={{
        border: compact ? 'none' : '1px solid #f0f0f0',
        borderRadius: 6,
        padding: compact ? '1px 4px' : '6px 8px',
        fontSize: 12,
        textAlign: compact ? 'left' : 'center',
        background: '#fff',
        cursor: 'grab',
        opacity: isDragging ? 0.4 : 1,
        display: compact ? 'flex' : 'block',
        gap: 6,
        alignItems: 'center',
        touchAction: 'none',
      }}
    >
      {icon ? <span>{icon}</span> : null}
      <Text code={compact} style={{ fontSize: compact ? 12 : undefined }}>
        {label}
      </Text>
    </div>
  );
}
