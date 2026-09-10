import { getIntl } from '@umijs/max';
import type {
  SchemaAction,
  SchemaStat,
  SchemaTab,
} from '@/components/page-schema/PageSchemaRenderer';

export type AssignmentPageSchema = {
  listColumns: Array<{
    key: 'id' | 'name' | 'version' | 'status' | 'capability' | 'assignedAt' | 'actions';
    title: string;
    width?: number;
    copyable?: boolean;
  }>;
  resourceColumns: Array<{
    key: 'resource' | 'count' | 'activeCount' | 'activeRate' | 'actions';
    title: string;
    width?: number;
  }>;
  capabilityColumns: Array<{
    key: 'id' | 'name' | 'capability' | 'actions';
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
  resourceActions: Array<{
    key: 'enable' | 'disable';
    label: string;
  }>;
  rowActions: Array<{
    key: 'enable' | 'disable' | 'canary' | 'detail';
    tooltip: string;
    icon: 'check' | 'delete' | 'experiment' | 'setting';
    danger?: boolean;
    permission?: 'read' | 'write';
    visibleWhen?: 'isActive' | 'notActive';
  }>;
  stats: Array<
    SchemaStat & {
      key: 'total' | 'active' | 'inactive' | 'resources';
      icon: 'setting' | 'check' | 'warning' | 'experiment';
    }
  >;
  actions: Array<
    SchemaAction & {
      key: 'history' | 'clone';
      icon: 'history' | 'copy';
      disabledWhen?: Array<'noScope' | 'noSelection' | 'loading'>;
      loadingWhen?: 'loading';
    }
  >;
  tabs: Array<
    SchemaTab & {
      key: 'list' | 'resource' | 'capability';
      visibleWhen?: 'hasGroups';
      component: 'ListTab' | 'CategoryTab' | 'RouteTab';
    }
  >;
};

// 纯 schema 常量（本文件无 LocalizedText spec 内容数据，全部为 UI 文案）：
// 经 getIntl 在模块加载时解析（SelectLang 切换语言会整页刷新重新求值）。
// labelTemplate 中的 {active}/{total}/{selectedCount} 是 PageSchemaRenderer 的
// 模板占位符，按 ICU 字面量转义（'{'）保留。
const intl = getIntl();

export const ASSIGNMENTS_PAGE_SCHEMA: AssignmentPageSchema = {
  listColumns: [
    {
      key: 'id',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.functionId',
        defaultMessage: '函数ID',
      }),
      width: 200,
      copyable: true,
    },
    {
      key: 'name',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.name',
        defaultMessage: '名称',
      }),
      width: 180,
    },
    {
      key: 'version',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.version',
        defaultMessage: '版本',
      }),
      width: 100,
    },
    {
      key: 'status',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.status',
        defaultMessage: '状态',
      }),
      width: 100,
    },
    {
      key: 'capability',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.capability',
        defaultMessage: '能力归属',
      }),
      width: 320,
    },
    {
      key: 'assignedAt',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.assignedAt',
        defaultMessage: '分配时间',
      }),
      width: 180,
    },
    {
      key: 'actions',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.actions',
        defaultMessage: '操作',
      }),
      width: 180,
    },
  ],
  resourceColumns: [
    {
      key: 'resource',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.resource',
        defaultMessage: '资源',
      }),
      width: 200,
    },
    {
      key: 'count',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.functionCount',
        defaultMessage: '函数数量',
      }),
      width: 120,
    },
    {
      key: 'activeCount',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.enabledCount',
        defaultMessage: '已启用',
      }),
      width: 120,
    },
    {
      key: 'activeRate',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.enableRate',
        defaultMessage: '启用率',
      }),
      width: 150,
    },
    {
      key: 'actions',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.actions',
        defaultMessage: '操作',
      }),
      width: 200,
    },
  ],
  capabilityColumns: [
    {
      key: 'id',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.functionId',
        defaultMessage: '函数ID',
      }),
      width: 240,
      copyable: true,
    },
    {
      key: 'name',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.name',
        defaultMessage: '名称',
      }),
      width: 180,
    },
    {
      key: 'capability',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.capability',
        defaultMessage: '能力归属',
      }),
      width: 420,
    },
    {
      key: 'actions',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.column.actions',
        defaultMessage: '操作',
      }),
      width: 150,
    },
  ],
  listToolbar: [
    {
      key: 'selectAll',
      label: intl.formatMessage({
        id: 'pages.assignments.schema.toolbar.selectAll',
        defaultMessage: '全选',
      }),
      icon: 'plus',
      permission: 'read',
    },
    {
      key: 'clearAll',
      label: intl.formatMessage({
        id: 'pages.assignments.schema.toolbar.clearAll',
        defaultMessage: '清空',
      }),
      icon: 'delete',
      permission: 'read',
    },
    {
      key: 'save',
      label: intl.formatMessage({
        id: 'pages.assignments.schema.toolbar.saveAssignment',
        defaultMessage: '保存分配',
      }),
      icon: 'save',
      type: 'primary',
      permission: 'write',
      disabledWhen: ['noScope', 'loading'],
      loadingWhen: 'loading',
    },
    {
      key: 'reload',
      label: intl.formatMessage({
        id: 'pages.assignments.schema.toolbar.reload',
        defaultMessage: '刷新',
      }),
      icon: 'reload',
      permission: 'read',
      disabledWhen: ['loading'],
      loadingWhen: 'loading',
    },
  ],
  resourceActions: [
    {
      key: 'enable',
      label: intl.formatMessage({
        id: 'pages.assignments.schema.action.enable',
        defaultMessage: '启用',
      }),
    },
    {
      key: 'disable',
      label: intl.formatMessage({
        id: 'pages.assignments.schema.action.disable',
        defaultMessage: '禁用',
      }),
    },
  ],
  rowActions: [
    {
      key: 'enable',
      tooltip: intl.formatMessage({
        id: 'pages.assignments.schema.action.enable',
        defaultMessage: '启用',
      }),
      icon: 'check',
      permission: 'write',
      visibleWhen: 'notActive',
    },
    {
      key: 'disable',
      tooltip: intl.formatMessage({
        id: 'pages.assignments.schema.action.disable',
        defaultMessage: '禁用',
      }),
      icon: 'delete',
      permission: 'write',
      danger: true,
      visibleWhen: 'isActive',
    },
    {
      key: 'canary',
      tooltip: intl.formatMessage({
        id: 'pages.assignments.schema.action.canaryConfig',
        defaultMessage: '灰度配置',
      }),
      icon: 'experiment',
      permission: 'write',
    },
    {
      key: 'detail',
      tooltip: intl.formatMessage({
        id: 'pages.assignments.schema.action.detail',
        defaultMessage: '查看详情',
      }),
      icon: 'setting',
      permission: 'read',
    },
  ],
  stats: [
    {
      key: 'total',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.stat.totalFunctions',
        defaultMessage: '总函数数',
      }),
      icon: 'setting',
    },
    {
      key: 'active',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.stat.assigned',
        defaultMessage: '已分配',
      }),
      icon: 'check',
      color: '#3f8600',
    },
    {
      key: 'inactive',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.stat.unassigned',
        defaultMessage: '未分配',
      }),
      icon: 'warning',
      color: '#cf1322',
    },
    {
      key: 'resources',
      title: intl.formatMessage({
        id: 'pages.assignments.schema.stat.resourceCount',
        defaultMessage: '资源数',
      }),
      icon: 'experiment',
    },
  ],
  actions: [
    {
      key: 'history',
      label: intl.formatMessage({
        id: 'pages.assignments.schema.action.history',
        defaultMessage: '变更历史',
      }),
      icon: 'history',
      permission: 'read',
      disabledWhen: ['noScope', 'loading'],
      loadingWhen: 'loading',
    },
    {
      key: 'clone',
      label: intl.formatMessage({
        id: 'pages.assignments.schema.action.cloneToEnv',
        defaultMessage: '克隆到环境',
      }),
      icon: 'copy',
      permission: 'write',
      disabledWhen: ['noScope', 'noSelection', 'loading'],
      loadingWhen: 'loading',
    },
  ],
  tabs: [
    {
      key: 'list',
      labelTemplate: intl.formatMessage({
        id: 'pages.assignments.schema.tab.listTemplate',
        defaultMessage: "列表视图 ('{active}'/'{total}')",
      }),
      permission: 'read',
      component: 'ListTab',
    },
    {
      key: 'resource',
      labelTemplate: intl.formatMessage({
        id: 'pages.assignments.schema.tab.resourceGroup',
        defaultMessage: '资源分组',
      }),
      permission: 'read',
      visibleWhen: 'hasGroups',
      component: 'CategoryTab',
    },
    {
      key: 'capability',
      labelTemplate: intl.formatMessage({
        id: 'pages.assignments.schema.tab.capabilityTemplate',
        defaultMessage: "能力归属 ('{selectedCount}')",
      }),
      permission: 'read',
      component: 'RouteTab',
    },
  ],
};
