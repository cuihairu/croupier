import React, { useMemo } from 'react';
import { Empty, Tree } from 'antd';
import { FormattedMessage } from '@umijs/max';
import type { DataNode } from 'antd/es/tree';
import { getComponent } from './registry';
import type { PageNode } from './model';

/** 节点标签：标题优先，声明区块 key 时附加（实例辨识，U5）。 */
function labelOf(n: PageNode, fallback: string): string {
  const base = String(n.props.title ?? fallback);
  const sk = typeof n.props.sectionKey === 'string' ? n.props.sectionKey.trim() : '';
  return sk ? `${base} (${sk})` : base;
}

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
  // 递归全深（V2：tabs 页 container 内组件也需在大纲可见/可选——页签内
  // 细粒度画布交互受限，大纲是页内节点的主要操作入口）。
  const toData = (n: PageNode): DataNode => ({
    key: n.id,
    title: labelOf(
      n,
      String(n.props.content ?? n.props.functionId ?? getComponent(n.type)?.name ?? n.type),
    ),
    children: n.children?.length ? n.children.map(toData) : undefined,
  });
  const data = useMemo<DataNode[]>(() => tree.map(toData), [tree]);

  if (tree.length === 0) {
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description={
          <FormattedMessage id="pages.pageStudio.editor.outline.empty" defaultMessage="页面为空" />
        }
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
