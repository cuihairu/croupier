export type DetailTabKey =
  | 'basic'
  | 'config'
  | 'permissions'
  | 'history'
  | 'analytics'
  | 'warnings';

export type DetailActionKey = 'reload' | 'copy' | 'delete' | 'edit';

export type DetailActionSchema = {
  key: DetailActionKey;
  label: string;
  danger?: boolean;
  primary?: boolean;
  loadingWhen?: 'loading';
  disabledWhen?: Array<'noFunction'>;
};

export const FUNCTION_DETAIL_SCHEMA = {
  tabs: [
    { key: 'basic', label: '基本信息' },
    { key: 'config', label: '函数配置' },
    { key: 'permissions', label: '权限' },
    { key: 'history', label: '调用历史' },
    { key: 'analytics', label: '统计分析' },
    { key: 'warnings', label: '注册告警' },
  ] as Array<{ key: DetailTabKey; label: string }>,
  actions: [
    { key: 'reload', label: '刷新', loadingWhen: 'loading' },
    { key: 'copy', label: '复制', disabledWhen: ['noFunction'] },
    { key: 'delete', label: '删除', danger: true, disabledWhen: ['noFunction'] },
    { key: 'edit', label: '编辑', primary: true, disabledWhen: ['noFunction'] },
  ] as DetailActionSchema[],
};
