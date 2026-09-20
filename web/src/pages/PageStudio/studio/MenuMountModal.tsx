import React, { useEffect, useMemo, useState } from 'react';
import { Alert, Form, Modal, TreeSelect } from 'antd';
import { useIntl } from '@umijs/max';
import type { MenuItem } from '@/services/api/menu';
import type { PageSpecDraftSummary } from '@/types/dashboard';
import { toMenuTreeData, type MenuTreeDatum } from './menuTree';

export interface MenuMountModalProps {
  /** 打开中的页面（null=关闭）；draft 状态页面挂载后发布才上控制台。 */
  page: PageSpecDraftSummary | null;
  /** 当前 scope 的菜单树（listMenus）。 */
  menus: MenuItem[];
  saving: boolean;
  onCancel: () => void;
  /** 提交：menuId 为 null 表示解除挂载。 */
  onSubmit: (pageKey: string, menuId: number | null) => void;
}

/**
 * 页面挂载菜单弹窗：TreeSelect 单选（allowClear 解除挂载）。
 * 控制台导航由「菜单 + 挂载 + 已发布」三要素共同决定（menu_items 唯一驱动）。
 */
const MenuMountModal: React.FC<MenuMountModalProps> = ({
  page,
  menus,
  saving,
  onCancel,
  onSubmit,
}) => {
  const intl = useIntl();
  const [form] = Form.useForm<{ menuId: number | null }>();
  const [treeData, setTreeData] = useState<MenuTreeDatum[]>([]);

  useEffect(() => {
    setTreeData(toMenuTreeData(menus, intl.locale));
  }, [menus, intl.locale]);

  useEffect(() => {
    if (page) {
      // 未挂载页面默认选中第一个菜单（进入即有默认值，清空即解除挂载）
      form.setFieldsValue({ menuId: page.menuId ?? (menus.length > 0 ? menus[0].id : null) });
    }
  }, [page, form, menus]);

  const empty = useMemo(() => menus.length === 0, [menus]);

  return (
    <Modal
      open={page !== null}
      title={intl.formatMessage(
        {
          id: 'pages.pageStudio.studio.mountMenu.title',
          defaultMessage: '挂载菜单：{pageKey}',
        },
        { pageKey: page?.pageKey ?? '' },
      )}
      confirmLoading={saving}
      onOk={async () => {
        // 无菜单时 Form 未渲染，validateFields 返回空对象——直接关闭不提交
        if (empty) {
          onCancel();
          return;
        }
        const values = await form.validateFields();
        onSubmit(page?.pageKey ?? '', values.menuId ?? null);
      }}
      onCancel={onCancel}
      okButtonProps={{ style: empty ? { display: 'none' } : undefined }}
      destroyOnHidden
    >
      {empty ? (
        <Alert
          type="info"
          showIcon
          message={intl.formatMessage({
            id: 'pages.pageStudio.studio.mountMenu.emptyMenus',
            defaultMessage: '当前环境暂无菜单，请先在「菜单管理」中创建菜单。',
          })}
        />
      ) : (
        <Form form={form} layout="vertical">
          <Form.Item
            name="menuId"
            label={intl.formatMessage({
              id: 'pages.pageStudio.studio.mountMenu.field',
              defaultMessage: '所属菜单',
            })}
            extra={intl.formatMessage({
              id: 'pages.pageStudio.studio.mountMenu.extra',
              defaultMessage:
                '清空即解除挂载（页面从运行控制台导航消失，发布状态保留）。改挂载即时生效，无需重新发布。',
            })}
          >
            <TreeSelect
              treeData={treeData}
              treeDefaultExpandAll
              allowClear
              placeholder={intl.formatMessage({
                id: 'pages.pageStudio.studio.mountMenu.placeholder',
                defaultMessage: '选择挂载的菜单（可清空）',
              })}
            />
          </Form.Item>
          {page?.status === 'draft' ? (
            <Alert
              type="warning"
              showIcon
              message={intl.formatMessage({
                id: 'pages.pageStudio.studio.mountMenu.draftHint',
                defaultMessage:
                  '该页面尚未发布：挂载关系会保存，但发布后才会出现在运行控制台导航。',
              })}
            />
          ) : null}
        </Form>
      )}
    </Modal>
  );
};

export default MenuMountModal;
