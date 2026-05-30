/**
 * 页面生成器类型定义
 */

// ==================== 页面配置 ====================

export interface PageConfig {
  id: string;
  type: 'list' | 'form' | 'detail';
  title: string;
  path: string;
  icon?: string;
  permissions?: string[];
  dataSource: DataSourceConfig;
  ui: UIConfig;
}

// ==================== 数据源配置 ====================

export interface DataSourceConfig {
  type: 'function' | 'api' | 'static';
  functionId?: string;
  apiEndpoint?: string;
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  params?: Record<string, any>;
  staticData?: any[];
}

// ==================== UI配置 ====================

export interface UIConfig {
  list?: ListUIConfig;
  form?: FormUIConfig;
  detail?: DetailUIConfig;
}

// ==================== 列表页配置 ====================

export interface ListUIConfig {
  columns: ColumnConfig[];
  actions?: ActionConfig[];
  filters?: FilterConfig[];
  pagination?: boolean;
  rowActions?: RowActionConfig[];
}

export interface ColumnConfig {
  key: string;
  title: string;
  width?: number;
  render?: 'text' | 'status' | 'datetime' | 'date' | 'tag' | 'link' | 'money';
  renderConfig?: {
    statusMap?: Record<
      string,
      { text: string; status: 'success' | 'error' | 'default' | 'processing' | 'warning' }
    >;
    tagColor?: string | Record<string, string>;
    linkHref?: string;
    format?: string;
    currency?: string;
  };
  sorter?: boolean;
  copyable?: boolean;
  ellipsis?: boolean;
}

export interface ActionConfig {
  key: string;
  label: string;
  type?: 'primary' | 'default' | 'dashed' | 'link' | 'text';
  icon?: string;
  onClick?: ActionHandler;
  danger?: boolean;
  permission?: string;
}

export interface RowActionConfig {
  key: string;
  label: string;
  icon?: string;
  onClick?: ActionHandler;
  danger?: boolean;
  confirm?: {
    title: string;
    content?: string;
  };
  permission?: string;
}

export interface FilterConfig {
  key: string;
  label: string;
  type: 'input' | 'select' | 'dateRange' | 'date';
  placeholder?: string;
  options?: Array<{ label: string; value: any }>;
  defaultValue?: any;
}

// ==================== 表单页配置 ====================

export interface FormUIConfig {
  fields?: FormFieldConfig[];
  formilySchema?: any; // 复用现有的 Formily Schema
  layout?: 'horizontal' | 'vertical' | 'inline';
  labelCol?: { span: number };
  wrapperCol?: { span: number };
  submitText?: string;
  resetText?: string;
  showReset?: boolean;
}

export interface FormFieldConfig {
  key: string;
  label: string;
  type: 'input' | 'textarea' | 'number' | 'select' | 'date' | 'switch' | 'radio' | 'checkbox';
  required?: boolean;
  placeholder?: string;
  options?: Array<{ label: string; value: any }>;
  defaultValue?: any;
  rules?: Array<{
    type?: 'string' | 'number' | 'email' | 'url' | 'pattern';
    pattern?: string;
    min?: number;
    max?: number;
    message?: string;
  }>;
  tooltip?: string;
  disabled?: boolean;
}

// ==================== 详情页配置 ====================

export interface DetailUIConfig {
  sections: DetailSectionConfig[];
  column?: number;
  actions?: ActionConfig[];
}

export interface DetailSectionConfig {
  title?: string;
  fields: DetailFieldConfig[];
}

export interface DetailFieldConfig {
  key: string;
  label: string;
  render?: 'text' | 'status' | 'datetime' | 'date' | 'tag' | 'link' | 'money';
  renderConfig?: ColumnConfig['renderConfig'];
  span?: number;
  copyable?: boolean;
}

// ==================== 行为处理 ====================

export interface ActionHandler {
  type: 'navigate' | 'modal' | 'function' | 'api';
  path?: string;
  modal?: {
    title: string;
    content: PageConfig;
    width?: number;
  };
  functionId?: string;
  api?: {
    endpoint: string;
    method: 'GET' | 'POST' | 'PUT' | 'DELETE';
    params?: Record<string, any>;
  };
  onSuccess?: {
    message?: string;
    refresh?: boolean;
    redirect?: string;
  };
}
