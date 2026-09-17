import React, { useEffect, useMemo, useState } from 'react';
import { useIntl } from '@umijs/max';
import { Button, Popconfirm, Space, Tag, Tooltip, Tree } from 'antd';
import type { DataNode } from 'antd/es/tree';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import type { MenuItem } from '@/services/api/menu';
import { resolveMenuIcon } from '@/utils/menuIcon';
import { localizedText } from '@/utils/localizedText';
import { computeDragUpdates, type SortUpdate } from './sortUtils';

export interface MenuTreeProps {
  items: MenuItem[];
  canManage: boolean;
  onEdit: (menu: MenuItem, mode: 'edit' | 'createChild') => void;
  onDelete: (menu: MenuItem) => void;
  onMove: (updates: SortUpdate[]) => void;
}

function collectKeys(items: MenuItem[]): React.Key[] {
  const out: React.Key[] = [];
  const walk = (nodes: MenuItem[]) => {
    for (const node of nodes) {
      out.push(String(node.id));
      walk(node.children);
    }
  };
  walk(items);
  return out;
}

/** antd Tree onDrop 的 info 形状（仅用到字段）。 */ export interface TreeDropInfo {
  node: { key: React.Key; pos?: string };
  dragNode: { key: React.Key };
  dropPosition: number;
  dropToGap: boolean;
}

/**
 * 构造 antd Tree 的 onDrop 处理器：pos 解析 → before/after/inside 归一 →
 * computeDragUpdates → onMove。导出以便单元测试覆盖胶水逻辑。
 */
export function buildDropHandler(
  items: MenuItem[],
  onMove: (updates: SortUpdate[]) => void,
): (info: TreeDropInfo) => void {
  return (info) => {
    const dropPos = String(info.node.pos ?? '').split('-');
    const relative = info.dropPosition - Number(dropPos[dropPos.length - 1]);
    const position = !info.dropToGap ? 'inside' : relative === -1 ? 'before' : 'after';
    const updates = computeDragUpdates(items, {
      dragKey: Number(info.dragNode.key),
      dropKey: Number(info.node.key),
      position,
    });
    if (updates && updates.length > 0) {
      onMove(updates);
    }
  };
}

/** MenuTree 渲染层级菜单树：图标+名称+标识+权限/隐藏标记，支持拖拽排序。 */
export default function MenuTree({ items, canManage, onEdit, onDelete, onMove }: MenuTreeProps) {
  const intl = useIntl();
  const fmt = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  // defaultExpandAll 不作用于异步后到的 treeData：数据变化时受控全展开
  const allKeys = useMemo(() => collectKeys(items), [items]);
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  useEffect(() => {
    setExpandedKeys(allKeys);
  }, [allKeys]);

  const buildTreeData = (nodes: MenuItem[]): DataNode[] =>
    nodes.map((node) => ({
      key: String(node.id),
      title: (
        <Space size="middle" style={{ flex: 1, minWidth: 0 }}>
          <Space size={4}>
            {resolveMenuIcon(node.icon)}
            <span>{localizedText(node.labels, intl.locale, node.menuKey)}</span>
            <Tag style={{ marginInlineEnd: 0 }}>{node.menuKey}</Tag>
          </Space>
          <Space size={4}>
            {node.permission ? (
              <Tooltip title={fmt('pages.menuManagement.tag.permission', '权限')}>
                <Tag color="geekblue" style={{ marginInlineEnd: 0 }}>
                  {node.permission}
                </Tag>
              </Tooltip>
            ) : null}
            {!node.isVisible ? (
              <Tag color="default" style={{ marginInlineEnd: 0 }}>
                {fmt('pages.menuManagement.tag.hidden', '隐藏')}
              </Tag>
            ) : null}
          </Space>
          {canManage ? (
            <Space size={0} onClick={(e) => e.stopPropagation()}>
              <Button
                type="link"
                size="small"
                icon={<EditOutlined />}
                onClick={(e) => {
                  e.stopPropagation();
                  onEdit(node, 'edit');
                }}
              >
                {fmt('pages.menuManagement.action.edit', '编辑')}
              </Button>
              <Button
                type="link"
                size="small"
                icon={<PlusOutlined />}
                onClick={(e) => {
                  e.stopPropagation();
                  onEdit(node, 'createChild');
                }}
              >
                {fmt('pages.menuManagement.action.addChild', '加子菜单')}
              </Button>
              <Popconfirm
                title={fmt(
                  'pages.menuManagement.delete.confirm',
                  '删除该菜单及其全部子菜单，并解除页面挂载？',
                )}
                onConfirm={(e) => {
                  e?.stopPropagation();
                  onDelete(node);
                }}
                onCancel={(e) => e?.stopPropagation()}
              >
                <Button
                  type="link"
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={(e) => e.stopPropagation()}
                >
                  {fmt('pages.menuManagement.action.delete', '删除')}
                </Button>
              </Popconfirm>
            </Space>
          ) : null}
        </Space>
      ),
      children: node.children.length ? buildTreeData(node.children) : undefined,
    }));

  return (
    <Tree
      blockNode
      expandedKeys={expandedKeys}
      onExpand={(keys) => setExpandedKeys(keys)}
      treeData={buildTreeData(items)}
      draggable={canManage ? { icon: false } : false}
      onDrop={buildDropHandler(items, onMove)}
    />
  );
}
