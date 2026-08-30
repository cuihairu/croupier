import React, { useMemo } from 'react';
import { Empty, Tree } from 'antd';
import type { DataNode } from 'antd/es/tree';
import { getComponent } from './registry';
import type { PageNode } from './model';

/** 大纲树：组件树导航（点击定位选中；与画布双向同步）。 */
export default function OutlinePanel({
  tree,
  selectedId,
  onSelect,
}: {
  tree: PageNode[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}) {
  const data = useMemo<DataNode[]>(
    () =>
      tree.map((n) => ({
        key: n.id,
        title: `${String(n.props.title ?? n.props.content ?? getComponent(n.type)?.name ?? n.type)}`,
        children: n.children?.length
          ? n.children.map((c) => ({
              key: c.id,
              title: String(c.props.functionId ?? c.type),
            }))
          : undefined,
      })),
    [tree],
  );

  if (tree.length === 0) {
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description="页面为空"
        style={{ marginTop: 40 }}
      />
    );
  }

  return (
    <Tree
      treeData={data}
      defaultExpandAll
      blockNode
      selectedKeys={selectedId ? [selectedId] : []}
      onSelect={(keys) => {
        const id = String(keys[0] || '');
        if (id) onSelect(id);
      }}
    />
  );
}
