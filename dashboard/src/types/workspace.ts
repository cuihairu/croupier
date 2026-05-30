/**
 * Workspace 配置类型定义
 *
 * 这些类型定义了 Workspace 的完整配置结构，用于配置驱动的 UI 渲染。
 *
 * @module types/workspace
 */

/**
 * Workspace 配置
 *
 * 描述一个完整的 Workspace，包括标题、布局、权限等信息。
 */
export interface WorkspaceConfig {
  /** 对象标识，如 "player"、"order" */
  objectKey: string;

  /** 显示标题 */
  title: string;

  /** 描述信息 */
  description?: string;

  /** 图标名称（Ant Design Icons） */
  icon?: string;

  /** 布局配置 */
  layout: WorkspaceLayout;

  /** 权限列表 */
  permissions?: string[];

  /** 是否已发布（发布后出现在控制台菜单） */
  published?: boolean;

  /** 配置状态 */
  status?: WorkspaceStatus;

  /** 发布时间 */
  publishedAt?: string;

  /** 发布人 */
  publishedBy?: string;

  /** 菜单顺序（控制台菜单排序） */
  menuOrder?: number;

  /** 配置版本号 */
  version?: number;

  /** 元数据 */
  meta?: WorkspaceMeta;
}

export type WorkspaceStatus = 'draft' | 'published' | 'archived';

export interface WorkspaceVersionRecord {
  id: string;
  objectKey: string;
  version: number;
  config: WorkspaceConfig;
  isCurrentDraft?: boolean;
  isCurrentPublished?: boolean;
  createdAt?: string;
  createdBy?: string;
  comment?: string;
}

/**
 * Workspace 布局
 *
 * 定义 Workspace 的整体布局结构。
 */
export interface WorkspaceLayout {
  /** 布局类型 */
  type: 'tabs' | 'sections' | 'wizard' | 'dashboard';

  /** Tab 配置列表（当 type 为 'tabs' 时使用） */
  tabs?: TabConfig[];

  /** Section 配置列表（当 type 为 'sections' 时使用） */
  sections?: SectionConfig[];
}

/**
 * Tab 配置
 *
 * 定义一个 Tab 页签的配置。
 */
export interface TabConfig {
  /** Tab 唯一标识 */
  key: string;

  /** Tab 标题 */
  title: string;

  /** Tab 图标（Ant Design Icons） */
  icon?: string;

  /** 使用的函数 ID 列表 */
  functions: string[];

  /** Tab 内的布局配置 */
  layout: TabLayout;

  /** 是否默认激活 */
  defaultActive?: boolean;

  /** 权限要求 */
  permissions?: string[];

  /** 配置注释（不影响运行时，用于团队协作说明） */
  comment?: string;

  /** 配置锁定（锁定后不可编辑，防止误操作） */
  locked?: boolean;

  /** 锁定者信息（记录谁锁定的配置） */
  lockedBy?: string;

  /** 锁定时间 */
  lockedAt?: string;
}

/**
 * Section 配置
 *
 * 定义一个区块的配置（用于 sections 布局）。
 */
export interface SectionConfig {
  /** Section 唯一标识 */
  key: string;

  /** Section 标题 */
  title: string;

  /** 使用的函数 ID 列表 */
  functions: string[];

  /** Section 内的布局配置 */
  layout: TabLayout;

  /** 是否可折叠 */
  collapsible?: boolean;

  /** 默认是否展开 */
  defaultExpanded?: boolean;
}

/**
 * Tab 布局类型
 *
 * 定义 Tab 内的具体布局方式。
 */
export type TabLayout =
  | FormDetailLayout
  | ListLayout
  | FormLayout
  | DetailLayout
  | GridLayout
  | KanbanLayout
  | TimelineLayout
  | SplitLayout
  | CustomLayout;

/**
 * 表单-详情布局
 *
 * 先通过表单查询，然后显示详情和操作按钮。
 * 适用场景：玩家信息查询、订单详情查询等。
 */
export interface FormDetailLayout {
  type: 'form-detail';

  /** 查询函数 ID */
  queryFunction: string;

  /** 查询表单字段配置 */
  queryFields: FieldConfig[];

  /** 详情分区配置 */
  detailSections: DetailSection[];

  /** 操作按钮配置 */
  actions?: ActionConfig[];

  /** 是否自动查询（页面加载时） */
  autoQuery?: boolean;
}

/**
 * 列表布局
 *
 * 显示数据列表，支持分页、搜索、行操作等。
 * 适用场景：玩家列表、订单列表等。
 */
export interface ListLayout {
  type: 'list';

  /** 列表数据函数 ID */
  listFunction: string;

  /** 列配置 */
  columns: ColumnConfig[];

  /** 搜索字段配置 */
  searchFields?: FieldConfig[];

  /** 行操作配置 */
  rowActions?: ActionConfig[];

  /** 工具栏操作配置 */
  toolbarActions?: ActionConfig[];

  /** 是否支持分页 */
  pagination?: boolean | PaginationConfig;

  /** 是否支持选择 */
  rowSelection?: boolean | RowSelectionConfig;
}

/**
 * 表单布局
 *
 * 显示表单，用于数据提交。
 * 适用场景：创建玩家、发送邮件等。
 */
export interface FormLayout {
  type: 'form';

  /** 提交函数 ID */
  submitFunction: string;

  /** 表单字段配置 */
  fields: FieldConfig[];

  /** 表单布局方式 */
  formLayout?: 'horizontal' | 'vertical' | 'inline';

  /** 提交按钮文本 */
  submitText?: string;

  /** 是否显示重置按钮 */
  showReset?: boolean;
}

/**
 * 详情布局
 *
 * 显示详情信息，只读。
 * 适用场景：系统信息、配置详情等。
 */
export interface DetailLayout {
  type: 'detail';

  /** 数据加载函数 ID */
  detailFunction?: string;

  /** 详情分区配置 */
  sections: DetailSection[];

  /** 操作按钮配置 */
  actions?: ActionConfig[];
}

/**
 * 自定义布局
 *
 * 使用自定义组件渲染。
 */
export interface CustomLayout {
  type: 'custom';

  /** 自定义组件名称 */
  component: string;

  /** 传递给组件的 props */
  props?: Record<string, any>;
}

/**
 * 网格布局
 *
 * 使用网格展示内容，支持响应式布局。
 * 适用场景：仪表盘、数据卡片展示等。
 */
export interface GridLayout {
  type: 'grid';

  /** 列数 */
  columns?: number;

  /** 间距 */
  gutter?: number | [number, number];

  /** 网格项配置 */
  items: GridItem[];

  /** 是否响应式 */
  responsive?: boolean;
}

/**
 * 网格项配置
 */
export interface GridItem {
  /** 唯一标识 */
  key: string;

  /** 占据列数 */
  colSpan?: number;

  /** 占据行数 */
  rowSpan?: number;

  /** 最小宽度 */
  minWidth?: number;

  /** 最大宽度 */
  maxWidth?: number;

  /** 是否可见 */
  visible?: boolean;

  /** 组件配置 */
  component?: {
    type: 'list' | 'form' | 'detail' | 'form-detail' | 'chart' | 'stat' | 'custom';
    config: Record<string, any>;
  };
}

/**
 * 看板布局
 *
 * 支持看板视图，可拖拽卡片。
 * 适用场景：任务管理、项目跟踪等。
 */
export interface KanbanLayout {
  type: 'kanban';

  /** 看板列配置 */
  columns?: KanbanColumn[];

  /** 数据函数 ID */
  dataFunction?: string;
}

/**
 * 看板列配置
 */
export interface KanbanColumn {
  /** 列唯一标识 */
  id: string;

  /** 列标题 */
  title: string;

  /** 列颜色 */
  color?: string;

  /** 卡片数量限制 */
  limit?: number;
}

/**
 * 时间线布局
 *
 * 支持时间线视图，展示事件流。
 * 适用场景：操作日志、审批记录等。
 */
export interface TimelineLayout {
  type: 'timeline';

  /** 数据函数 ID */
  dataFunction?: string;

  /** 事件类型过滤 */
  filterTypes?: string[];

  /** 是否显示筛选 */
  showFilter?: boolean;

  /** 是否逆序显示 */
  reverse?: boolean;
}

/**
 * 分栏布局
 *
 * 支持水平或垂直分栏。
 * 适用场景：主从结构、对比视图等。
 */
export interface SplitLayout {
  type: 'split';

  /** 分栏方向 */
  direction?: 'horizontal' | 'vertical';

  /** 面板配置 */
  panels: SplitPanel[];

  /** 各面板尺寸（百分比或像素） */
  sizes?: string[];
}

/**
 * 分栏面板配置
 */
export interface SplitPanel {
  /** 面板唯一标识 */
  key: string;

  /** 面板标题 */
  title?: string;

  /** 面板内容 */
  content?: React.ReactNode;

  /** 最小尺寸 */
  minSize?: string;

  /** 是否可折叠 */
  collapsible?: boolean;
}

/**
 * 字段配置
 *
 * 定义表单字段或查询字段的配置。
 */
export interface FieldConfig {
  /** 字段名 */
  key: string;

  /** 字段标签 */
  label: string;

  /** 字段类型 */
  type:
    | 'input'
    | 'number'
    | 'select'
    | 'date'
    | 'datetime'
    | 'textarea'
    | 'switch'
    | 'radio'
    | 'checkbox';

  /** 是否必填 */
  required?: boolean;

  /** 占位符 */
  placeholder?: string;

  /** 默认值 */
  defaultValue?: any;

  /** 默认值表达式（如 "$now()", "$user.id", "$query.key"）*/
  defaultValueExpression?: string;

  /** 选项列表（用于 select、radio、checkbox） */
  options?: Array<{ label: string; value: any }>;

  /** 验证规则 */
  rules?: FieldRule[];

  /** 是否禁用 */
  disabled?: boolean;

  /** 提示信息 */
  tooltip?: string;

  /** 显隐联动条件（如 "status === 'active'"，为 false 时隐藏该字段） */
  visibleWhen?: string;

  /** 禁用联动条件（如 "type === 'readonly'"，为 true 时禁用该字段） */
  disabledWhen?: string;
}

/**
 * 字段验证规则
 */
export interface FieldRule {
  /** 规则类型 */
  type?: 'string' | 'number' | 'email' | 'url' | 'pattern';

  /** 正则表达式（当 type 为 'pattern' 时） */
  pattern?: string;

  /** 最小值/最小长度 */
  min?: number;

  /** 最大值/最大长度 */
  max?: number;

  /** 错误提示信息 */
  message?: string;
}

/**
 * 列配置
 *
 * 定义列表的列配置。
 */
export interface ColumnConfig {
  /** 列字段名 */
  key: string;

  /** 列标题 */
  title: string;

  /** 列宽度 */
  width?: number;

  /** 渲染方式 */
  render?: 'text' | 'status' | 'datetime' | 'date' | 'tag' | 'money' | 'link' | 'image' | 'custom';

  /** 自定义渲染函数名称（当 render 为 'custom' 时） */
  customRender?: string;

  /** 是否可排序 */
  sortable?: boolean;

  /** 是否固定列 */
  fixed?: 'left' | 'right';

  /** 对齐方式 */
  align?: 'left' | 'center' | 'right';

  /** 是否启用超长省略 */
  ellipsis?: boolean;

  /** 渲染选项（如状态映射、日期格式等） */
  renderOptions?: Record<string, any>;

  /** 条件显隐 - 权限控制（需要的权限标识，无权限时隐藏该列） */
  visiblePermission?: string;

  /** 条件显隐 - 数据状态表达式（如 "status !== 'archived'"，为 false 时隐藏该列） */
  visibleCondition?: string;
}

/**
 * 详情分区
 *
 * 将详情信息分组显示。
 */
export interface DetailSection {
  /** 分区标题 */
  title: string;

  /** 分区字段配置 */
  fields: DetailField[];

  /** 列数 */
  column?: number;

  /** 是否可折叠 */
  collapsible?: boolean;

  /** 默认是否展开 */
  defaultExpanded?: boolean;
}

/**
 * 详情字段
 *
 * 定义详情字段的显示配置。
 */
export interface DetailField {
  /** 字段名 */
  key: string;

  /** 字段标签 */
  label: string;

  /** 渲染方式 */
  render?: 'text' | 'status' | 'datetime' | 'date' | 'tag' | 'money' | 'link' | 'image' | 'custom';

  /** 自定义渲染函数名称 */
  customRender?: string;

  /** 渲染选项 */
  renderOptions?: Record<string, any>;

  /** 占据的列数 */
  span?: number;
}

/**
 * 操作配置
 *
 * 定义操作按钮的配置。
 */
export interface ActionConfig {
  /** 操作唯一标识 */
  key: string;

  /** 操作标签 */
  label: string;

  /** 操作图标 */
  icon?: string;

  /** 操作函数 ID */
  function: string;

  /** 操作类型 */
  type?: 'modal' | 'drawer' | 'popconfirm' | 'direct';

  /** 按钮类型 */
  buttonType?: 'primary' | 'default' | 'dashed' | 'link' | 'text';

  /** 是否危险操作 */
  danger?: boolean;

  /** 操作字段配置（用于 modal 和 drawer） */
  fields?: FieldConfig[];

  /** 确认提示信息（用于 popconfirm） */
  confirmMessage?: string;

  /** 权限要求 */
  permissions?: string[];

  /** 是否禁用 */
  disabled?: boolean;
}

/**
 * 分页配置
 */
export interface PaginationConfig {
  /** 每页条数 */
  pageSize?: number;

  /** 每页条数选项 */
  pageSizeOptions?: number[];

  /** 是否显示快速跳转 */
  showQuickJumper?: boolean;

  /** 是否显示总数 */
  showTotal?: boolean;
}

/**
 * 行选择配置
 */
export interface RowSelectionConfig {
  /** 选择类型 */
  type?: 'checkbox' | 'radio';

  /** 是否固定选择列 */
  fixed?: boolean;

  /** 选择列宽度 */
  columnWidth?: number;
}

/**
 * Workspace 元数据
 */
export interface WorkspaceMeta {
  /** 创建时间 */
  createdAt?: string;

  /** 更新时间 */
  updatedAt?: string;

  /** 创建者 */
  createdBy?: string;

  /** 更新者 */
  updatedBy?: string;

  /** 版本号 */
  version?: number;

  /** 标签 */
  tags?: string[];

  /** 自定义元数据 */
  custom?: Record<string, any>;
}
