import React, { useCallback, useEffect, useState } from 'react';
import { history, useAccess, useIntl } from '@umijs/max';
import { App, Button, Card, Empty, Spin } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import {
  createMenu,
  deleteMenu,
  listMenus,
  updateMenu,
  updateMenuSort,
  type MenuItem,
} from '@/services/api/menu';
import { listPageDrafts } from '@/services/api/pages';
import MenuForm, { type MenuFormValues } from './MenuForm';
import MenuTree, { type MenuMountedPage } from './MenuTree';
import { type SortUpdate } from './sortUtils';

/** MenuManagement 菜单管理页：树查看 + 新建/编辑/删除 + 拖拽排序。 */
export default function MenuManagementPage() {
  const intl = useIntl();
  const { message } = App.useApp();
  const access = useAccess?.() || {};
  const canManage = Boolean(access.canMenuManage);

  const [items, setItems] = useState<MenuItem[]>([]);
  const [pages, setPages] = useState<MenuMountedPage[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<MenuItem | null>(null);
  const [presetParentId, setPresetParentId] = useState<number | undefined>(undefined);

  const fmt = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [menus, drafts] = await Promise.all([listMenus(), listPageDrafts()]);
      setItems(menus);
      setPages(
        drafts
          .filter((d) => typeof d.menuId === 'number' && d.menuId > 0)
          .map((d) => ({
            pageKey: d.pageKey,
            type: d.type,
            menuId: d.menuId as number,
            title: d.title ?? {},
            status: d.status,
            order: (d as { order?: number }).order,
          })),
      );
    } catch {
      // 错误 toast 由全局 request 拦截器负责
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const openCreate = () => {
    setEditing(null);
    setPresetParentId(undefined);
    setModalOpen(true);
  };

  const handleEdit = (menu: MenuItem, mode: 'edit' | 'createChild') => {
    if (mode === 'edit') {
      setEditing(menu);
      setPresetParentId(undefined);
    } else {
      setEditing(null);
      setPresetParentId(menu.id);
    }
    setModalOpen(true);
  };

  const handleSubmit = async (values: MenuFormValues): Promise<boolean> => {
    const payload = {
      menuKey: values.menuKey.trim(),
      parentId: values.parentId ?? 0,
      labels: values.labels,
      icon: values.icon,
      sortOrder: values.sortOrder ?? 0,
      permission: (values.permission ?? '').trim(),
      isVisible: values.isVisible !== false,
    };
    try {
      if (editing) {
        await updateMenu(editing.id, payload);
        message.success(fmt('pages.menuManagement.updateSuccess', '菜单已更新'));
      } else {
        await createMenu(payload);
        message.success(fmt('pages.menuManagement.createSuccess', '菜单已创建'));
      }
      await load();
      return true;
    } catch {
      // 全局拦截器已提示；保持弹窗开启供用户修正
      return false;
    }
  };

  const handleDelete = async (menu: MenuItem) => {
    try {
      await deleteMenu(menu.id);
      message.success(fmt('pages.menuManagement.deleteSuccess', '菜单已删除'));
    } finally {
      await load();
    }
  };

  const handleMove = async (updates: SortUpdate[]) => {
    const originalParent = new Map<number, number | null>();
    const walk = (nodes: MenuItem[], parent: number | null) => {
      for (const node of nodes) {
        originalParent.set(node.id, parent);
        walk(node.children, node.id);
      }
    };
    walk(items, null);
    try {
      for (const update of updates) {
        const before = originalParent.get(update.id);
        if (before !== undefined && before !== update.parentId) {
          // 跨父级移动：parentId=0 表示顶级（后端契约）
          await updateMenu(update.id, { parentId: update.parentId ?? 0 });
        }
        await updateMenuSort(update.id, update.sortOrder);
      }
      message.success(fmt('pages.menuManagement.sortUpdated', '菜单排序已更新'));
    } catch {
      message.error(fmt('pages.menuManagement.sortFailed', '菜单排序更新失败'));
    } finally {
      await load();
    }
  };

  return (
    <PageContainer
      header={{
        title: fmt('pages.menuManagement.title', '菜单管理'),
        breadcrumb: {},
      }}
      content={fmt(
        'pages.menuManagement.description',
        '维护看板导航菜单：层级、名称、图标与访问权限',
      )}
      extra={
        canManage
          ? [
              <Button key="create" type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                {fmt('pages.menuManagement.action.create', '新建菜单')}
              </Button>,
            ]
          : []
      }
    >
      <Card>
        <Spin spinning={loading}>
          {items.length === 0 && !loading ? (
            <Empty
              description={fmt(
                'pages.menuManagement.empty',
                '暂无菜单，点击「新建菜单」创建第一个菜单',
              )}
            />
          ) : (
            <MenuTree
              items={items}
              pages={pages}
              canManage={canManage}
              onEditPage={(pageKey) =>
                history.push(`/functions/pages?focus=${encodeURIComponent(pageKey)}`)
              }
              onEdit={handleEdit}
              onDelete={handleDelete}
              onMove={handleMove}
            />
          )}
        </Spin>
      </Card>
      <MenuForm
        open={modalOpen}
        editing={editing}
        items={items}
        presetParentId={presetParentId}
        onOpenChange={setModalOpen}
        onSubmit={handleSubmit}
      />
    </PageContainer>
  );
}
