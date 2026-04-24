/**
 * Layout Engine 相关类型定义
 *
 * 这些类型用于 Layout Engine 的渲染逻辑。
 *
 * @module types/layout
 */

/**
 * 渲染器 Props 基础接口
 */
export interface BaseRendererProps {
  /** 布局配置 */
  layout: any;

  /** 可用函数列表 */
  functions?: string[];

  /** 额外的上下文数据 */
  context?: Record<string, any>;
}

/**
 * 渲染上下文
 *
 * 在渲染过程中传递的上下文信息。
 */
export interface RenderContext {
  /** Workspace 对象标识 */
  objectKey: string;

  /** 当前用户信息 */
  user?: UserInfo;

  /** 权限列表 */
  permissions?: string[];

  /** 额外的上下文数据 */
  extra?: Record<string, any>;
}

/**
 * 用户信息
 */
export interface UserInfo {
  /** 用户 ID */
  id: string;

  /** 用户名 */
  username: string;

  /** 显示名称 */
  displayName?: string;

  /** 角色列表 */
  roles?: string[];

  /** 权限列表 */
  permissions?: string[];
}

/**
 * 函数调用参数
 */
export interface FunctionInvokeParams {
  /** 函数 ID */
  functionId: string;

  /** 函数参数 */
  params: Record<string, any>;

  /** 调用选项 */
  options?: FunctionInvokeOptions;
}

/**
 * 函数调用选项
 */
export interface FunctionInvokeOptions {
  /** 是否显示加载状态 */
  showLoading?: boolean;

  /** 是否显示成功提示 */
  showSuccess?: boolean;

  /** 成功提示信息 */
  successMessage?: string;

  /** 是否显示错误提示 */
  showError?: boolean;

  /** 超时时间（毫秒） */
  timeout?: number;
}

/**
 * 函数调用结果
 */
export interface FunctionInvokeResult<T = any> {
  /** 是否成功 */
  success: boolean;

  /** 返回数据 */
  data?: T;

  /** 错误信息 */
  error?: string;

  /** 错误代码 */
  errorCode?: string;

  /** 额外信息 */
  meta?: Record<string, any>;
}

/**
 * 渲染器注册表
 *
 * 用于注册和获取自定义渲染器。
 */
export interface RendererRegistry {
  /** 注册渲染器 */
  register(type: string, renderer: React.ComponentType<any>): void;

  /** 获取渲染器 */
  get(type: string): React.ComponentType<any> | undefined;

  /** 是否已注册 */
  has(type: string): boolean;

  /** 注销渲染器 */
  unregister(type: string): void;
}

/**
 * 字段渲染器 Props
 */
export interface FieldRendererProps {
  /** 字段配置 */
  field: any;

  /** 字段值 */
  value: any;

  /** 值变化回调 */
  onChange?: (value: any) => void;

  /** 是否禁用 */
  disabled?: boolean;

  /** 是否只读 */
  readonly?: boolean;
}

/**
 * 列渲染器 Props
 */
export interface ColumnRendererProps {
  /** 列配置 */
  column: any;

  /** 单元格值 */
  value: any;

  /** 行数据 */
  record: any;

  /** 行索引 */
  index: number;
}

/**
 * 操作渲染器 Props
 */
export interface ActionRendererProps {
  /** 操作配置 */
  action: any;

  /** 当前数据 */
  record?: any;

  /** 点击回调 */
  onClick?: (action: any, record?: any) => void;

  /** 是否禁用 */
  disabled?: boolean;
}

/**
 * 表单值
 */
export type FormValues = Record<string, any>;

/**
 * 表单实例
 */
export interface FormInstance {
  /** 获取字段值 */
  getFieldValue(name: string): any;

  /** 获取所有字段值 */
  getFieldsValue(): FormValues;

  /** 设置字段值 */
  setFieldValue(name: string, value: any): void;

  /** 设置多个字段值 */
  setFieldsValue(values: FormValues): void;

  /** 重置表单 */
  resetFields(): void;

  /** 验证表单 */
  validateFields(): Promise<FormValues>;

  /** 提交表单 */
  submit(): void;
}

/**
 * 列表数据源
 */
export interface ListDataSource<T = any> {
  /** 数据列表 */
  data: T[];

  /** 总数 */
  total: number;

  /** 当前页码 */
  current: number;

  /** 每页条数 */
  pageSize: number;

  /** 是否加载中 */
  loading: boolean;
}

/**
 * 列表操作
 */
export interface ListActions {
  /** 刷新列表 */
  refresh(): void;

  /** 重新加载（重置到第一页） */
  reload(): void;

  /** 跳转到指定页 */
  goToPage(page: number): void;

  /** 改变每页条数 */
  changePageSize(pageSize: number): void;

  /** 搜索 */
  search(values: FormValues): void;

  /** 重置搜索 */
  resetSearch(): void;
}

/**
 * 详情数据
 */
export interface DetailData {
  /** 数据 */
  data: Record<string, any>;

  /** 是否加载中 */
  loading: boolean;

  /** 错误信息 */
  error?: string;
}

/**
 * 详情操作
 */
export interface DetailActions {
  /** 刷新详情 */
  refresh(): void;

  /** 重新加载 */
  reload(): void;
}

/**
 * 模态框配置
 */
export interface ModalConfig {
  /** 标题 */
  title: string;

  /** 宽度 */
  width?: number | string;

  /** 是否可拖拽 */
  draggable?: boolean;

  /** 是否全屏 */
  fullscreen?: boolean;

  /** 确认按钮文本 */
  okText?: string;

  /** 取消按钮文本 */
  cancelText?: string;

  /** 是否显示取消按钮 */
  showCancel?: boolean;
}

/**
 * 抽屉配置
 */
export interface DrawerConfig {
  /** 标题 */
  title: string;

  /** 宽度 */
  width?: number | string;

  /** 位置 */
  placement?: 'left' | 'right' | 'top' | 'bottom';

  /** 确认按钮文本 */
  okText?: string;

  /** 取消按钮文本 */
  cancelText?: string;

  /** 是否显示取消按钮 */
  showCancel?: boolean;
}

/**
 * 渲染选项
 */
export interface RenderOptions {
  /** 状态映射（用于 status 渲染） */
  statusMap?: Record<
    string,
    { text: string; status: 'success' | 'error' | 'warning' | 'default' | 'processing' }
  >;

  /** 日期格式（用于 date/datetime 渲染） */
  dateFormat?: string;

  /** 货币符号（用于 money 渲染） */
  currencySymbol?: string;

  /** 货币精度（用于 money 渲染） */
  currencyPrecision?: number;

  /** 标签颜色（用于 tag 渲染） */
  tagColor?: string | ((value: any) => string);

  /** 链接目标（用于 link 渲染） */
  linkTarget?: '_blank' | '_self' | '_parent' | '_top';

  /** 图片预览（用于 image 渲染） */
  imagePreview?: boolean;

  /** 自定义渲染函数 */
  customRender?: (value: any, record?: any) => React.ReactNode;
}

/**
 * 验证结果
 */
export interface ValidationResult {
  /** 是否通过验证 */
  valid: boolean;

  /** 错误信息列表 */
  errors?: Array<{
    field: string;
    message: string;
  }>;
}

/**
 * 加载状态
 */
export interface LoadingState {
  /** 是否加载中 */
  loading: boolean;

  /** 加载提示信息 */
  tip?: string;

  /** 延迟显示加载状态的时间（毫秒） */
  delay?: number;
}

/**
 * 错误状态
 */
export interface ErrorState {
  /** 是否有错误 */
  hasError: boolean;

  /** 错误信息 */
  message?: string;

  /** 错误代码 */
  code?: string;

  /** 错误详情 */
  details?: any;
}

/**
 * 组件状态
 */
export interface ComponentState {
  /** 加载状态 */
  loading: LoadingState;

  /** 错误状态 */
  error: ErrorState;

  /** 数据 */
  data?: any;

  /** 额外状态 */
  extra?: Record<string, any>;
}
