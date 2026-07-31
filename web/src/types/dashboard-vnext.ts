/**
 * Dashboard vNext 强类型定义
 *
 * 本文件定义 vNext 版本的强类型 DTO，对应后端 internal/dashboard/spec/ 下的新类型。
 * 与旧版 FormilySchema 组件树协议不同，vNext 使用语义化、可校验的结构。
 *
 * @module types/dashboard-vnext
 */

import type { LocalizedText, JSONSchema, RiskLevel, CapabilityKind, FunctionExecution, Diagnostic } from './dashboard';

// ---------------------------------------------------------------------------
// PageSpecV2 - 强类型页面规格
// ---------------------------------------------------------------------------

/** 页面类型 */
export type PageTypeV2 = 'resource' | 'operation' | 'task' | 'report';

/** 页面导航规格 */
export interface NavigationSpec {
  title: LocalizedText;
  breadcrumb?: BreadcrumbItem[];
  showBack?: boolean;
  backPath?: string;
}

/** 面包屑项 */
export interface BreadcrumbItem {
  title: LocalizedText;
  path?: string;
}

/** 强类型页面规格 */
export interface PageSpecV2 {
  pageKey: string;
  type: PageTypeV2;
  resourceKey?: string;
  title: LocalizedText;
  description?: LocalizedText;
  category: PageCategorySpecV2;
  order?: number;
  icon?: string;
  navigation?: NavigationSpec;
  resource?: ResourcePageSpec;
  operation?: OperationPageSpec;
  task?: TaskPageSpec;
  report?: ReportPageSpec;
  bindings: PageFunctionBindingV2[];
  metadata?: Record<string, unknown>;
}

/** 页面分类规格 */
export interface PageCategorySpecV2 {
  key: string;
  labels: LocalizedText;
  order?: number;
}

// ---------------------------------------------------------------------------
// ResourcePageSpec - 资源 CRUD 页面
// ---------------------------------------------------------------------------

/** 资源页面规格 */
export interface ResourcePageSpec {
  listView?: ListViewSpec;
  detailView?: DetailViewSpec;
  actions?: ActionSpec[];
  createForm?: FormPresentationSpec;
  updateForm?: FormPresentationSpec;
  deleteAction?: ConfirmActionSpec;
}

/** 列表视图规格 */
export interface ListViewSpec {
  columns: ColumnSpec[];
  defaultSort?: SortSpec;
  filters?: FilterSpec[];
  pagination?: PaginationSpec;
  rowActions?: ActionSpec[];
  batchActions?: ActionSpec[];
  toolbarActions?: ActionSpec[];
}

/** 列规格 */
export interface ColumnSpec {
  key: string;
  title: LocalizedText;
  dataType: 'string' | 'number' | 'boolean' | 'date' | 'datetime' | 'enum';
  width?: number;
  fixed?: 'left' | 'right';
  sortable?: boolean;
  filterable?: boolean;
  visible?: boolean;
  enum?: EnumOption[];
  format?: string;
  render?: 'tag' | 'link' | 'copy' | 'status';
}

/** 枚举选项 */
export interface EnumOption {
  value: string;
  label: LocalizedText;
  color?: string;
}

/** 排序规格 */
export interface SortSpec {
  field: string;
  order: 'asc' | 'desc';
}

/** 筛选规格 */
export interface FilterSpec {
  key: string;
  title: LocalizedText;
  type: 'text' | 'select' | 'date' | 'daterange' | 'number';
  options?: EnumOption[];
}

/** 分页规格 */
export interface PaginationSpec {
  enabled: boolean;
  defaultSize?: number;
  pageSizes?: number[];
}

/** 详情视图规格 */
export interface DetailViewSpec {
  fields: DetailFieldSpec[];
  actions?: ActionSpec[];
  layout?: 'vertical' | 'horizontal' | 'grid';
}

/** 详情字段规格 */
export interface DetailFieldSpec {
  key: string;
  title: LocalizedText;
  dataType: string;
  span?: number;
  render?: string;
  visible?: boolean;
}

// ---------------------------------------------------------------------------
// ActionSpec - 操作规格
// ---------------------------------------------------------------------------

/** 操作规格 */
export interface ActionSpec {
  key: string;
  title: LocalizedText;
  icon?: string;
  type?: 'primary' | 'default' | 'danger' | 'link';
  confirm?: boolean;
  confirmTitle?: LocalizedText;
  confirmDescription?: LocalizedText;
  bindingId?: string;
  permission?: string;
  risk?: RiskLevel;
}

/** 确认操作规格 */
export interface ConfirmActionSpec {
  title: LocalizedText;
  description?: LocalizedText;
  confirmText: LocalizedText;
  cancelText?: LocalizedText;
  bindingId: string;
  permission?: string;
  risk?: RiskLevel;
}

// ---------------------------------------------------------------------------
// OperationPageSpec - 独立操作页面
// ---------------------------------------------------------------------------

/** 操作页面规格 */
export interface OperationPageSpec {
  form: FormPresentationSpec;
  confirm?: ConfirmActionSpec;
  resultView?: ResultViewSpec;
}

// ---------------------------------------------------------------------------
// TaskPageSpec - 异步任务页面
// ---------------------------------------------------------------------------

/** 任务页面规格 */
export interface TaskPageSpec {
  form: FormPresentationSpec;
  taskView: TaskViewSpec;
  resultView?: ResultViewSpec;
}

/** 任务视图规格 */
export interface TaskViewSpec {
  showTimeline: boolean;
  showProgress: boolean;
  showEvents: boolean;
  cancelable: boolean;
  retryable: boolean;
}

// ---------------------------------------------------------------------------
// ReportPageSpec - 报表页面
// ---------------------------------------------------------------------------

/** 报表页面规格 */
export interface ReportPageSpec {
  queryForm: FormPresentationSpec;
  dataset: DatasetSpec;
  charts?: ChartSpec[];
  table?: ListViewSpec;
  exportable?: boolean;
}

/** 数据集规格 */
export interface DatasetSpec {
  dimensions: DimensionSpec[];
  metrics: MetricSpec[];
}

/** 维度规格 */
export interface DimensionSpec {
  key: string;
  title: LocalizedText;
  dataType: 'string' | 'number' | 'date';
}

/** 指标规格 */
export interface MetricSpec {
  key: string;
  title: LocalizedText;
  dataType: 'number';
  aggType?: 'sum' | 'avg' | 'count' | 'min' | 'max';
  format?: 'number' | 'percent' | 'currency';
}

/** 图表规格 */
export interface ChartSpec {
  type: 'line' | 'bar' | 'pie' | 'area' | 'scatter';
  title: LocalizedText;
  xField?: string;
  yField?: string;
  seriesField?: string;
  groupField?: string;
}

// ---------------------------------------------------------------------------
// ResultViewSpec - 结果视图
// ---------------------------------------------------------------------------

/** 结果视图规格 */
export interface ResultViewSpec {
  fields?: ResultFieldSpec[];
  successMessage?: LocalizedText;
  errorMessage?: LocalizedText;
}

/** 结果字段规格 */
export interface ResultFieldSpec {
  key: string;
  title: LocalizedText;
  dataType: string;
  render?: string;
}

// ---------------------------------------------------------------------------
// FormPresentationSpec - 表单展示规格
// ---------------------------------------------------------------------------

/** 表单布局 */
export type FormLayout = 'vertical' | 'horizontal' | 'inline' | 'grid';

/** 表单展示规格 */
export interface FormPresentationSpec {
  jsonSchema: JSONSchema;
  layout?: FormLayout;
  groups?: FormGroupSpec[];
  fields?: FormFieldSpec[];
  submitButton?: FormButtonSpec;
  cancelButton?: FormButtonSpec;
}

/** 表单分组规格 */
export interface FormGroupSpec {
  key: string;
  title?: LocalizedText;
  fields: string[];
  collapsible?: boolean;
  collapsed?: boolean;
}

/** 表单字段规格 */
export interface FormFieldSpec {
  key: string;
  widget?: FormWidget;
  label?: LocalizedText;
  placeholder?: LocalizedText;
  description?: LocalizedText;
  width?: number;
  order?: number;
  visible?: boolean;
  disabled?: boolean;
  required?: boolean;
  defaultValue?: unknown;
  enumOptions?: EnumOption[];
  widgetProps?: Record<string, unknown>;
  validationRules?: ValidationRule[];
}

/** 表单组件类型 */
export type FormWidget =
  | 'Input'
  | 'TextArea'
  | 'InputNumber'
  | 'Password'
  | 'Select'
  | 'MultiSelect'
  | 'Radio'
  | 'Checkbox'
  | 'Switch'
  | 'DatePicker'
  | 'TimePicker'
  | 'DateRange'
  | 'Upload'
  | 'ImageUpload'
  | 'FileUpload'
  | 'RichText'
  | 'Code'
  | 'Cascader'
  | 'TreeSelect'
  | 'Color'
  | 'Slider'
  | 'Rate'
  | 'JSON'
  | 'KeyValue'
  | 'Array'
  | 'Object';

/** 验证规则 */
export interface ValidationRule {
  type: 'required' | 'min' | 'max' | 'pattern' | 'custom';
  value?: unknown;
  message: LocalizedText;
}

/** 表单按钮规格 */
export interface FormButtonSpec {
  text: LocalizedText;
  type?: 'primary' | 'default' | 'danger' | 'link';
  icon?: string;
  loading?: boolean;
}

// ---------------------------------------------------------------------------
// PageFunctionBindingV2 - 页面函数绑定
// ---------------------------------------------------------------------------

/** 选择器源类型 */
export type SelectorSourceType = 'form' | 'row' | 'selection' | 'detail' | 'page_state' | 'literal';

/** 选择器源 */
export interface SelectorSource {
  type: SelectorSourceType;
  path?: string;
  value?: unknown;
  transform?: TransformSpec;
}

/** 转换规格 */
export interface TransformSpec {
  type: 'default' | 'format' | 'convert' | 'pick' | 'map';
  params?: Record<string, unknown>;
}

/** 选择器赋值 */
export interface Assignment {
  target: string;
  source: SelectorSource;
}

/** 选择器 AST */
export interface SelectorAST {
  assignments: Assignment[];
}

/** 页面函数绑定 v2 */
export interface PageFunctionBindingV2 {
  id: string;
  functionId: string;
  usage: 'query' | 'detail' | 'action' | 'task' | 'report';
  inputSelector?: SelectorAST;
  outputSelector?: SelectorAST;
  execution: {
    mode: 'sync' | 'task';
    requireConfirm?: boolean;
  };
}

// ---------------------------------------------------------------------------
// ResourceCatalog - 资源目录
// ---------------------------------------------------------------------------

/** 资源目录项 */
export interface ResourceCatalogItem {
  resourceKey: string;
  labels: LocalizedText;
  description?: LocalizedText;
  categoryKey?: string;
  status: 'identified' | 'pending' | 'conflict' | 'not_executable';
  functions: FunctionInfo[];
  semantics?: SemanticsInfo;
  diagnostics?: DiagnosticInfo[];
}

/** 函数信息 */
export interface FunctionInfo {
  functionId: string;
  version: string;
  capability: CapabilityKind;
  execution: FunctionExecution;
  risk: RiskLevel;
  enabled: boolean;
  source: string;
}

/** 语义信息 */
export interface SemanticsInfo {
  version: number;
  hasIdentity: boolean;
  hasCollection: boolean;
  hasCreate: boolean;
  hasUpdate: boolean;
  hasDelete: boolean;
  hasActions: boolean;
  hasTasks: boolean;
  hasReports: boolean;
  source: string;
}

/** 诊断信息 */
export interface DiagnosticInfo {
  code: string;
  severity: 'error' | 'warning' | 'info';
  message: string;
}

// ---------------------------------------------------------------------------
// PageProposal - 页面提案
// ---------------------------------------------------------------------------

/** 提案状态 */
export type ProposalStatus = 'pending' | 'accepted' | 'rejected' | 'expired';

/** 提案质量 */
export type ProposalQuality = 'ready' | 'basic' | 'needs_review' | 'blocked';

/** 页面提案 */
export interface PageProposal {
  id: number;
  gameId?: string;
  env?: string;
  proposalKey: string;
  pageKey: string;
  pageType: PageTypeV2;
  resourceKey?: string;
  quality: ProposalQuality;
  generatorVersion: string;
  title: LocalizedText;
  description?: LocalizedText;
  categoryKey?: string;
  pageSpec: PageSpecV2;
  diagnostics?: DiagnosticInfo[];
  status: ProposalStatus;
  updatedAt: string;
  updatedBy?: string;
}

// ---------------------------------------------------------------------------
// Versioning - 版本管理
// ---------------------------------------------------------------------------

/** 变更类型 */
export type ChangeType = 'function_update' | 'semantic_update' | 'proposal_update' | 'draft_update' | 'publish';

/** 变更项 */
export interface ChangeItem {
  type: ChangeType;
  timestamp: string;
  version?: number;
  summary: string;
  details?: unknown;
  actor?: string;
}

/** 当前状态 */
export interface CurrentState {
  functionVersion?: string;
  semanticVersion?: number;
  proposalVersion?: number;
  draftRevision?: number;
  publishedVersion?: number;
}

/** 变更链 */
export interface ChangeChain {
  resourceKey: string;
  items: ChangeItem[];
  current: CurrentState;
}

/** 合并策略 */
export type MergeStrategy = 'auto' | 'accept' | 'reject' | 'manual';

/** 冲突解决 */
export interface ConflictResolution {
  path: string;
  acceptNew: boolean;
  value?: unknown;
}

/** 合并请求 */
export interface MergeRequest {
  strategy: MergeStrategy;
  conflicts?: ConflictResolution[];
  reason?: string;
}

/** 合并响应 */
export interface MergeResponse {
  merged: number;
  conflicts: number;
  message: string;
}
