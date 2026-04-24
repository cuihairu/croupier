export type DirectoryPageSchema = {
  headerActions: Array<{
    key: 'refresh';
    label: string;
    icon: 'reload';
    loadingWhen?: 'loading';
    disabledWhen?: Array<'loading'>;
  }>;
  drawerActions: Array<{
    key: 'invoke' | 'detailPage';
    label: string;
    icon: 'play' | 'info';
    disabledWhen?: Array<'noSelection' | 'loading'>;
    loadingWhen?: 'loading';
  }>;
  rowActions: Array<{
    key: 'detail' | 'ui' | 'invoke';
    tooltip: string;
    icon: 'info' | 'setting' | 'play';
  }>;
  columns: Array<{
    key: 'id' | 'displayName' | 'summary' | 'category' | 'tags' | 'enabled' | 'actions';
    title: string;
    width?: number;
    copyable?: boolean;
  }>;
};

export const DIRECTORY_PAGE_SCHEMA: DirectoryPageSchema = {
  headerActions: [
    {
      key: 'refresh',
      label: '刷新',
      icon: 'reload',
      loadingWhen: 'loading',
      disabledWhen: ['loading'],
    },
  ],
  drawerActions: [
    {
      key: 'detailPage',
      label: '详情页',
      icon: 'info',
      disabledWhen: ['noSelection', 'loading'],
      loadingWhen: 'loading',
    },
    {
      key: 'invoke',
      label: '调用函数',
      icon: 'play',
      disabledWhen: ['noSelection', 'loading'],
      loadingWhen: 'loading',
    },
  ],
  rowActions: [
    { key: 'detail', tooltip: '查看详情', icon: 'info' },
    { key: 'ui', tooltip: '编辑UI', icon: 'setting' },
    { key: 'invoke', tooltip: '调用函数', icon: 'play' },
  ],
  columns: [
    { key: 'id', title: '函数ID', width: 250, copyable: true },
    { key: 'displayName', title: '函数名称', width: 200 },
    { key: 'summary', title: '函数摘要', width: 300 },
    { key: 'category', title: '分类', width: 120 },
    { key: 'tags', title: '标签', width: 200 },
    { key: 'enabled', title: '状态', width: 80 },
    { key: 'actions', title: '操作', width: 200 },
  ],
};
