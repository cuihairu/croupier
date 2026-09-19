import React, { useEffect, useMemo, useState } from 'react';
import { useIntl } from '@umijs/max';
import { Button, Popconfirm, Space, Tag, Tooltip, Tree, Typography } from 'antd';
import type { DataNode } from 'antd/es/tree';
import {
  DeleteOutlined,
  EditOutlined,
  FileTextOutlined,
  HolderOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import type { MenuItem } from '@/services/api/menu';
import type { LocalizedText, PageDraftStatus, PageType } from '@/types/dashboard';
import { resolveMenuIcon } from '@/utils/menuIcon';
import { localizedText } from '@/utils/localizedText';
import { computeDragUpdates, type SortUpdate } from './sortUtils';

/** 挂在菜单节点下的页面摘要（来自 listPageDrafts 的 menuId 关联）。 */
export interface MenuMountedPage {
  pageKey: string;
  type: PageType;
  menuId: number;
  title: LocalizedText;
  status: PageDraftStatus;
  order?: number;
}

export interface MenuTreeProps {
  items: MenuItem[];
  pages: MenuMountedPage[];
  canManage: boolean;
  onEdit: (menu: MenuItem, mode: 'edit' | 'createChild') => void;
  onDelete: (menu: MenuItem) => void;
  onMove: (updates: SortUpdate[]) => void;
  onEditPage?: (pageKey: string) => void;
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

/** antd Tree onDrop 的 info 形状（仅用到字段）。 */
export interface TreeDropInfo {
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

const dragHandleStyle: React.CSSProperties = {
  color: 'rgba(0,0,0,0.25)',
  cursor: 'grab',
  marginRight: 2,
};

/** MenuTree 渲染层级菜单树：图标+名称+标识+权限/隐藏标记，支持拖拽排序；
 *  每个菜单节点下以只读叶子展示挂载的页面（状态徽标区分 draft/published）。 */
export default function MenuTree({
  items,
  pages,
  canManage,
  onEdit,
  onDelete,
  onMove,
  onEditPage,
}: MenuTreeProps) {
  const intl = useIntl();
  const fmt = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  // defaultExpandAll 不作用于异步后到的 treeData：数据变化时受控全展开
  const allKeys = useMemo(() => collectKeys(items), [items]);
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]);
  useEffect(() => {
    setExpandedKeys(allKeys);
  }, [allKeys]);

  const pagesByMenu = useMemo(() => {
    const map = new Map<number, MenuMountedPage[]>();
    for (const p of pages) {
      const list = map.get(p.menuId) ?? [];
      list.push(p);
      map.set(
        p.menuId,
        list.sort((a, b) => (a.order ?? 0) - (b.order ?? 0) || a.pageKey.localeCompare(b.pageKey)),
      );
    }
    return map;
  }, [pages]);

  const pageLeaf = (page: MenuMountedPage): DataNode => ({
    key: `page:${page.pageKey}`,
    selectable: false,
    title: (
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0, paddingBlock: 2 }}>
        <FileTextOutlined style={{ color: 'rgba(0,0,0,0.35)' }} />
        <Typography.Text ellipsis style={{ maxWidth: 260 }}>
          {localizedText(page.title, intl.locale, page.pageKey)}
        </Typography.Text>
        <Tag
          color={
            page.status === 'published' ? 'green' : page.status === 'draft' ? 'orange' : 'default'
          }
          style={{ marginInlineEnd: 0 }}
        >
          {page.status === 'published'
            ? fmt('pages.menuManagement.page.statusPublished', '已发布')
            : page.status === 'draft'
              ? fmt('pages.menuManagement.page.statusDraft', '草稿')
              : fmt('pages.menuManagement.page.statusArchived', '已下架')}
        </Tag>
        {page.status === 'draft' ? (
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {fmt('pages.menuManagement.page.draftHint', '发布后才会出现在控制台导航')}
          </Typography.Text>
        ) : null}
        {page.order ? (
          <Tag style={{ marginInlineEnd: 0 }}>
            {fmt('pages.menuManagement.page.order', '排序 {order}').replace(
              '{order}',
              String(page.order),
            )}
          </Tag>
        ) : null}
        {onEditPage ? (
          <Button
            type="link"
            size="small"
            style={{ paddingInline: 4 }}
            onClick={(e) => {
              e.stopPropagation();
              onEditPage(page.pageKey);
            }}
          >
            {fmt('pages.menuManagement.page.edit', '编辑页面')}
          </Button>
        ) : null}
      </div>
    ),
  });

  const buildTreeData = (nodes: MenuItem[]): DataNode[] =>
    nodes.map((node) => {
      const childMenus = node.children.length ? buildTreeData(node.children) : [];
      const mountedPages = (pagesByMenu.get(node.id) ?? []).map(pageLeaf);
      return {
        key: String(node.id),
        title: (
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 0 }}>
            <Space size={6} style={{ minWidth: 0, flex: '0 1 auto' }}>
              {canManage ? (
                <Tooltip title={fmt('pages.menuManagement.dragHint', '按住拖拽可调整同级顺序')}>
                  <HolderOutlined style={dragHandleStyle} />
                </Tooltip>
              ) : null}
              {resolveMenuIcon(node.icon)}
              <Typography.Text strong ellipsis style={{ maxWidth: 220 }}>
                {localizedText(node.labels, intl.locale, node.menuKey)}
              </Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {node.menuKey}
              </Typography.Text>
              {node.sortOrder ? (
                <Tag style={{ marginInlineEnd: 0 }}>
                  {fmt('pages.menuManagement.page.order', '排序 {order}').replace(
                    '{order}',
                    String(node.sortOrder),
                  )}
                </Tag>
              ) : null}
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
              <Space size={0} style={{ marginLeft: 'auto' }} onClick={(e) => e.stopPropagation()}>
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
                    '删除该菜单及其全部子菜单，并解除挂在下面的页面挂载？',
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
          </div>
        ),
        children:
          childMenus.length > 0 || mountedPages.length > 0
            ? [...childMenus, ...mountedPages]
            : undefined,
      };
    });

  return (
    <Tree
      blockNode
      showLine
      expandedKeys={expandedKeys}
      onExpand={(keys) => setExpandedKeys(keys)}
      treeData={buildTreeData(items)}
      draggable={
        canManage
          ? {
              icon: false,
              // 页面叶子只读展示挂载关系，不可拖拽（排序经页面编辑器「页面排序」）
              nodeDraggable: (node: DataNode) => !String(node.key ?? '').startsWith('page:'),
            }
          : false
      }
      onDrop={buildDropHandler(items, onMove)}
    />
  );
}
