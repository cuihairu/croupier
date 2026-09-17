import React from 'react';
import { useIntl } from '@umijs/max';
import { Form, Input, InputNumber, Select, Switch, TreeSelect } from 'antd';
import { ModalForm } from '@ant-design/pro-components';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import type { LocalizedText } from '@/types/dashboard';
import type { MenuItem } from '@/services/api/menu';
import { MENU_ICON_NAMES, resolveMenuIcon } from '@/utils/menuIcon';
import { localizedText } from '@/utils/localizedText';

/** TreeSelect treeData 节点（value/title/children 结构即满足 antd 契约）。 */
interface ParentOption {
  value: number;
  title: string;
  children?: ParentOption[];
}

export interface MenuFormValues {
  menuKey: string;
  /** 0 = 顶级菜单。 */
  parentId: number;
  labels: LocalizedText;
  icon?: string;
  sortOrder?: number;
  permission?: string;
  isVisible: boolean;
}

export interface MenuFormProps {
  open: boolean;
  /** null = 新建；非 null = 编辑该菜单。 */
  editing: MenuItem | null;
  /** 当前全部菜单（父菜单候选）。 */
  items: MenuItem[];
  /** 新建子菜单时的预选父级。 */
  presetParentId?: number;
  onOpenChange: (open: boolean) => void;
  onSubmit: (values: MenuFormValues) => Promise<boolean>;
}

const MENU_KEY_PATTERN = /^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/;

/** 收集 id 及其全部子孙 id（编辑时这些节点不能作为父菜单候选）。 */
function collectSubtreeIds(nodes: MenuItem[]): Set<number> {
  const out = new Set<number>();
  const walk = (list: MenuItem[]) => {
    for (const node of list) {
      out.add(node.id);
      walk(node.children);
    }
  };
  walk(nodes);
  return out;
}

function buildParentTreeData(
  items: MenuItem[],
  disabledIds: Set<number>,
  locale: string,
): ParentOption[] {
  return items
    .filter((node) => !disabledIds.has(node.id))
    .map((node) => ({
      value: node.id,
      title: `${localizedText(node.labels, locale, node.menuKey)} (${node.menuKey})`,
      children: buildParentTreeData(node.children, disabledIds, locale),
    }));
}

/** MenuForm 菜单新建/编辑弹窗（ModalForm + LocalizedTextEditor）。 */
export default function MenuForm({
  open,
  editing,
  items,
  presetParentId,
  onOpenChange,
  onSubmit,
}: MenuFormProps) {
  const intl = useIntl();
  const fmt = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const disabledIds = editing ? collectSubtreeIds([editing]) : new Set<number>();
  const parentTreeData: ParentOption[] = [
    {
      value: 0,
      title: fmt('pages.menuManagement.field.parentRoot', '顶级菜单'),
      children: buildParentTreeData(items, disabledIds, intl.locale),
    },
  ];

  const initialValues: Partial<MenuFormValues> = editing
    ? {
        menuKey: editing.menuKey,
        parentId: editing.parentId ?? 0,
        labels: editing.labels,
        icon: editing.icon,
        sortOrder: editing.sortOrder,
        permission: editing.permission,
        isVisible: editing.isVisible,
      }
    : {
        menuKey: '',
        parentId: presetParentId ?? 0,
        labels: {},
        sortOrder: 0,
        permission: '',
        isVisible: true,
      };

  return (
    <ModalForm<MenuFormValues>
      title={
        editing
          ? fmt('pages.menuManagement.action.edit', '编辑')
          : fmt('pages.menuManagement.action.create', '新建菜单')
      }
      width={520}
      open={open}
      onOpenChange={onOpenChange}
      initialValues={initialValues}
      modalProps={{ destroyOnHidden: true }}
      onFinish={onSubmit}
    >
      <Form.Item
        name="menuKey"
        label={fmt('pages.menuManagement.field.menuKey', '菜单标识')}
        rules={[
          { required: true, message: fmt('pages.menuManagement.field.menuKey', '菜单标识') },
          {
            pattern: MENU_KEY_PATTERN,
            message: fmt(
              'pages.menuManagement.field.menuKeyInvalid',
              '仅允许字母开头的字母/数字/中划线/下划线，长度 1-64',
            ),
          },
        ]}
      >
        <Input placeholder={fmt('pages.menuManagement.field.menuKeyPlaceholder', '如 resource')} />
      </Form.Item>
      <Form.Item name="parentId" label={fmt('pages.menuManagement.field.parent', '父菜单')}>
        <TreeSelect treeData={parentTreeData} treeDefaultExpandAll allowClear={false} />
      </Form.Item>
      <Form.Item
        name="labels"
        label={fmt('pages.menuManagement.field.labels', '名称（多语言）')}
        rules={[
          {
            validator: (_rule, value: LocalizedText | undefined) => {
              const hasValue = Object.values(value ?? {}).some((v) => (v ?? '').trim() !== '');
              return hasValue
                ? Promise.resolve()
                : Promise.reject(
                    new Error(
                      fmt('pages.menuManagement.field.labelsRequired', '至少填写一个语言的名称'),
                    ),
                  );
            },
          },
        ]}
      >
        <LocalizedTextEditor
          placeholder={fmt('pages.menuManagement.field.labelsPlaceholder', '请输入菜单名称')}
        />
      </Form.Item>
      <Form.Item name="icon" label={fmt('pages.menuManagement.field.icon', '图标')}>
        <Select
          allowClear
          placeholder={fmt('pages.menuManagement.field.iconPlaceholder', '选择图标（可选）')}
          options={MENU_ICON_NAMES.map((name) => ({
            value: name,
            label: (
              <span>
                {resolveMenuIcon(name)} <span style={{ marginInlineStart: 4 }}>{name}</span>
              </span>
            ),
          }))}
        />
      </Form.Item>
      <Form.Item name="sortOrder" label={fmt('pages.menuManagement.field.sortOrder', '排序')}>
        <InputNumber min={0} style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item name="permission" label={fmt('pages.menuManagement.field.permission', '权限标识')}>
        <Input
          placeholder={fmt(
            'pages.menuManagement.field.permissionPlaceholder',
            '如 resource:read（留空对所有用户可见）',
          )}
        />
      </Form.Item>
      <Form.Item
        name="isVisible"
        label={fmt('pages.menuManagement.field.isVisible', '可见')}
        valuePropName="checked"
        extra={fmt('pages.menuManagement.field.isVisibleHelp', '关闭后该菜单分支对所有用户隐藏')}
      >
        <Switch />
      </Form.Item>
    </ModalForm>
  );
}
