import type {
  SchemaAction,
  SchemaStat,
  SchemaTab,
} from '@/components/page-schema/PageSchemaRenderer';

export type AssignmentPageSchema = {
  listColumns: Array<{
    key: 'id' | 'name' | 'version' | 'status' | 'route' | 'assignedAt' | 'actions';
    title: string;
    width?: number;
    copyable?: boolean;
  }>;
  categoryColumns: Array<{
    key: 'category' | 'count' | 'activeCount' | 'activeRate' | 'actions';
    title: string;
    width?: number;
  }>;
  routeColumns: Array<{
    key: 'id' | 'name' | 'route' | 'actions';
    title: string;
    width?: number;
    copyable?: boolean;
  }>;
  listToolbar: Array<{
    key: 'selectAll' | 'clearAll' | 'save' | 'reload';
    label: string;
    icon: 'plus' | 'delete' | 'save' | 'reload';
    type?: 'primary';
    permission?: 'read' | 'write';
    disabledWhen?: Array<'noScope' | 'noSelection' | 'loading'>;
    loadingWhen?: 'loading';
  }>;
  categoryActions: Array<{
    key: 'enable' | 'disable';
    label: string;
  }>;
  rowActions: Array<{
    key: 'enable' | 'disable' | 'canary' | 'detail' | 'route';
    tooltip: string;
    icon: 'check' | 'delete' | 'experiment' | 'setting' | 'edit';
    danger?: boolean;
    permission?: 'read' | 'write';
    visibleWhen?: 'isActive' | 'notActive';
  }>;
  stats: Array<
    SchemaStat & {
      key: 'total' | 'active' | 'inactive' | 'categories';
      icon: 'setting' | 'check' | 'warning' | 'experiment';
    }
  >;
  actions: Array<
    SchemaAction & {
      key: 'history' | 'clone';
      icon: 'history' | 'copy';
      disabledWhen?: Array<'noScope' | 'noSelection' | 'loading'>;
    }
  >;
  tabs: Array<
    SchemaTab & {
      key: 'list' | 'category' | 'route';
      visibleWhen?: 'hasGroups';
      component: 'ListTab' | 'CategoryTab' | 'RouteTab';
    }
  >;
};

export const ASSIGNMENTS_PAGE_SCHEMA: AssignmentPageSchema = {
  listColumns: [
    { key: 'id', title: '函数ID', width: 200, copyable: true },
    { key: 'name', title: '名称', width: 180 },
    { key: 'version', title: '版本', width: 100 },
    { key: 'status', title: '状态', width: 100 },
    { key: 'route', title: '路由展示', width: 320 },
    { key: 'assignedAt', title: '分配时间', width: 180 },
    { key: 'actions', title: '操作', width: 180 },
  ],
  categoryColumns: [
    { key: 'category', title: '分类', width: 200 },
    { key: 'count', title: '函数数量', width: 120 },
    { key: 'activeCount', title: '已启用', width: 120 },
    { key: 'activeRate', title: '启用率', width: 150 },
    { key: 'actions', title: '操作', width: 200 },
  ],
  routeColumns: [
    { key: 'id', title: '函数ID', width: 240, copyable: true },
    { key: 'name', title: '名称', width: 180 },
    { key: 'route', title: '路由展示', width: 420 },
    { key: 'actions', title: '操作', width: 150 },
  ],
  listToolbar: [
    { key: 'selectAll', label: '全选', icon: 'plus', permission: 'read' },
    { key: 'clearAll', label: '清空', icon: 'delete', permission: 'read' },
    {
      key: 'save',
      label: '保存分配',
      icon: 'save',
      type: 'primary',
      permission: 'write',
      disabledWhen: ['noScope', 'loading'],
      loadingWhen: 'loading',
    },
    {
      key: 'reload',
      label: '刷新',
      icon: 'reload',
      permission: 'read',
      disabledWhen: ['loading'],
      loadingWhen: 'loading',
    },
  ],
  categoryActions: [
    { key: 'enable', label: '启用' },
    { key: 'disable', label: '禁用' },
  ],
  rowActions: [
    {
      key: 'enable',
      tooltip: '启用',
      icon: 'check',
      permission: 'write',
      visibleWhen: 'notActive',
    },
    {
      key: 'disable',
      tooltip: '禁用',
      icon: 'delete',
      permission: 'write',
      danger: true,
      visibleWhen: 'isActive',
    },
    { key: 'canary', tooltip: '灰度配置', icon: 'experiment', permission: 'write' },
    { key: 'detail', tooltip: '查看详情', icon: 'setting', permission: 'read' },
    { key: 'route', tooltip: '路由配置', icon: 'edit', permission: 'write' },
  ],
  stats: [
    { key: 'total', title: '总函数数', icon: 'setting' },
    { key: 'active', title: '已分配', icon: 'check', color: '#3f8600' },
    { key: 'inactive', title: '未分配', icon: 'warning', color: '#cf1322' },
    { key: 'categories', title: '分类数', icon: 'experiment' },
  ],
  actions: [
    {
      key: 'history',
      label: '变更历史',
      icon: 'history',
      permission: 'read',
      disabledWhen: ['noScope', 'loading'],
      loadingWhen: 'loading',
    },
    {
      key: 'clone',
      label: '克隆到环境',
      icon: 'copy',
      permission: 'write',
      disabledWhen: ['noScope', 'noSelection', 'loading'],
      loadingWhen: 'loading',
    },
  ],
  tabs: [
    {
      key: 'list',
      labelTemplate: '列表视图 ({active}/{total})',
      permission: 'read',
      component: 'ListTab',
    },
    {
      key: 'category',
      labelTemplate: '分类管理',
      permission: 'read',
      visibleWhen: 'hasGroups',
      component: 'CategoryTab',
    },
    {
      key: 'route',
      labelTemplate: '路由展示 ({selectedCount})',
      permission: 'read',
      component: 'RouteTab',
    },
  ],
};
