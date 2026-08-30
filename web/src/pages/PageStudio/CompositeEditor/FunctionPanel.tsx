import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Card, Empty, Input, Space, Spin, Tag, Typography } from 'antd';
import { PlusOutlined, ReloadOutlined, SearchOutlined, SwapOutlined } from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { Tree } from 'antd';
import { request } from '@umijs/max';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';
import { getMyGames } from '@/services/api/me';
import { getScope, setScope, subscribeScope } from '@/stores/scope';
import { VIEW_META, defaultView } from './types';

const { Text } = Typography;

/** 带指定 scope 头探测函数契约数（用于空态引导）。 */
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

/** 空态引导：当前 scope 无函数契约时列出其他 scope 的函数分布并可一键切换。 */
function ScopeGuide({ onReload }: { onReload: () => void }) {
  const [probing, setProbing] = useState(true);
  const [scopes, setScopes] = useState<{ gameId: string; env: string; count: number }[]>([]);
  const [switching, setSwitching] = useState<string | null>(null);
  const current = getScope();

  useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        const { games } = await getMyGames();
        const tasks: { gameId: string; env: string }[] = [];
        for (const g of games ?? []) {
          for (const env of g.envs ?? []) tasks.push({ gameId: g.gameId ?? '', env });
        }
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

  const withFn = scopes.filter((s) => s.gameId !== current.gameId || s.env !== current.env);
  const currentLabel = `${current.gameId ?? '未设置'}/${current.env ?? '未设置'}`;

  return (
    <div style={{ padding: 12 }}>
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={
          <Space direction="vertical" size={4}>
            <Text strong>当前 scope（{currentLabel}）下没有函数契约</Text>
            <Text type="secondary" style={{ fontSize: 12 }}>
              函数按 (game, env) 隔离；切换到有函数的 scope 后即可搭建页面
            </Text>
          </Space>
        }
      />
      <div style={{ marginTop: 8, minHeight: 60 }}>
        {probing ? (
          <div style={{ textAlign: 'center', padding: 16 }}>
            <Spin size="small" />
            <Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
              正在探测各 scope 的函数分布…
            </Text>
          </div>
        ) : withFn.length === 0 ? (
          <Text type="secondary" style={{ fontSize: 12 }}>
            其他 scope 也没有函数契约——请先通过 SDK/OpenAPI 注册函数
          </Text>
        ) : (
          <Space direction="vertical" size={6} style={{ width: '100%' }}>
            {withFn.map((s) => (
              <Button
                key={`${s.gameId}/${s.env}`}
                size="small"
                block
                style={{ textAlign: 'left' }}
                icon={<SwapOutlined />}
                loading={switching === `${s.gameId}/${s.env}`}
                onClick={() => {
                  setSwitching(`${s.gameId}/${s.env}`);
                  setScope({ gameId: s.gameId, env: s.env });
                  onReload();
                  setSwitching(null);
                }}
              >
                {s.gameId}/{s.env}
                <Tag style={{ marginLeft: 8 }} color="blue">
                  {s.count} 函数
                </Tag>
              </Button>
            ))}
          </Space>
        )}
      </div>
    </div>
  );
}

export default function FunctionPanel({
  addedIds,
  onAdd,
}: {
  addedIds: Set<string>;
  onAdd: (fn: FunctionDescriptor) => void;
}) {
  const [descriptors, setDescriptors] = useState<FunctionDescriptor[]>([]);
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState('');
  const [reloadKey, setReloadKey] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setDescriptors(await listDescriptors());
    } catch {
      // 错误已由全局 handler 提示
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load, reloadKey]);

  // scope 切换后自动刷新（空态引导的一键切换 / 顶栏切换器都覆盖）
  useEffect(() => subscribeScope(() => setReloadKey((k) => k + 1)), []);

  const fnById = useMemo(() => {
    const m = new Map<string, FunctionDescriptor>();
    for (const d of descriptors) m.set(d.id, d);
    return m;
  }, [descriptors]);

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
      .map(([rk, fns]) => {
        const children = fns
          .sort((a, b) => a.id.localeCompare(b.id))
          .filter((f) => !q || f.id.toLowerCase().includes(q))
          .map((f) => ({
            key: f.id,
            title: (
              <Space size={6} style={{ fontSize: 12, width: '100%' }}>
                <Text code style={{ fontSize: 11 }}>
                  {f.operation || f.id.split('.').pop()}
                </Text>
                {addedIds.has(f.id) ? (
                  <Text type="success" style={{ fontSize: 11 }}>
                    已添加
                  </Text>
                ) : (
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    {VIEW_META[defaultView(f)].label}
                  </Text>
                )}
              </Space>
            ),
            isLeaf: true,
          }));
        return { title: `${rk} (${children.length})`, key: rk, children, selectable: false };
      })
      .filter((n) => n.children.length > 0);
  }, [descriptors, search, addedIds]);

  return (
    <Card
      size="small"
      title={
        <Space size={4}>
          <Text strong>函数</Text>
          <Text type="secondary">{descriptors.length}</Text>
        </Space>
      }
      extra={
        <Button
          size="small"
          type="text"
          icon={<ReloadOutlined />}
          onClick={() => setReloadKey((k) => k + 1)}
        />
      }
      styles={{ body: { padding: 8, height: 'calc(100vh - 160px)', overflow: 'auto' } }}
    >
      {descriptors.length === 0 && !loading ? (
        <ScopeGuide onReload={() => setReloadKey((k) => k + 1)} />
      ) : (
        <>
          <Input
            size="small"
            allowClear
            prefix={<SearchOutlined style={{ color: '#999' }} />}
            placeholder="搜索函数 / 资源"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ marginBottom: 8 }}
          />
          {treeData.length === 0 ? (
            <Empty description="无匹配函数" image={Empty.PRESENTED_IMAGE_SIMPLE} />
          ) : (
            <Tree
              treeData={treeData}
              defaultExpandAll
              showLine={false}
              blockNode
              onSelect={(keys) => {
                const id = String(keys[0] || '');
                const fn = fnById.get(id);
                if (fn) onAdd(fn);
              }}
            />
          )}
          <div style={{ marginTop: 8 }}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              <PlusOutlined /> 点击函数加入画布
            </Text>
          </div>
        </>
      )}
    </Card>
  );
}
