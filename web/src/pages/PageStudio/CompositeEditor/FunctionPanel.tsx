import React, { useMemo, useState } from 'react';
import { Button, Card, Empty, Input, Space, Typography } from 'antd';
import { PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import { Tree } from 'antd';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';
import { VIEW_META, defaultView } from './types';

const { Text } = Typography;

export type FnPick = {
  functionId: string;
  view: ReturnType<typeof defaultView>;
  title: string;
};

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

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      setDescriptors(await listDescriptors());
    } catch {
      // 错误已由全局 handler 提示
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

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
        <Button size="small" type="text" icon={<ReloadOutlined />} onClick={() => void load()} />
      }
      styles={{ body: { padding: 8, height: 'calc(100vh - 160px)', overflow: 'auto' } }}
      loading={loading}
    >
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
    </Card>
  );
}
