/**
 * Dashboard Resource/Page 模型强类型定义
 *
 * 本文件是 Dashboard 模型的唯一 TypeScript 类型入口。
 * 所有 API DTO、页面组件、服务层必须引用此处的类型，禁止自行定义重复结构。
 *
 * 与 Go spec 包 (internal/dashboard/spec/types.go) 的 JSON 字段名保持一致。
 *
 * @module types/dashboard
 */

// ---------------------------------------------------------------------------
// Primitive type aliases
// ---------------------------------------------------------------------------

/** 多语言文本，locale code -> display text */
export type LocalizedText = Record<string, string>;

/** JSON 基础值，避免核心 DTO 使用非约束动态类型 */
export type JSONValue =
  | null
  | boolean
  | number
  | string
  | JSONValue[]
  | { [key: string]: JSONValue };

/** JSON Schema 对象 (draft-07 / 2020-12) */
export type JSONSchema = { [key: string]: JSONValue };

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

/** 风险等级 */
export type RiskLevel = 'safe' | 'warning' | 'high' | 'danger';

/** 函数在资源生命周期中的能力语义 */
export type CapabilityKind =
  | 'collection_query'
  | 'item_query'
  | 'create'
  | 'update'
  | 'delete'
  | 'action'
  | 'task'
  | 'report';

/** 函数执行方式 */
export type FunctionExecution = 'sync' | 'task';

/** 页面类型 */
export type PageType = 'resource' | 'operation' | 'task' | 'report';

/** 页面 binding 在运行期的用途 */
export type PageBindingUsage = 'query' | 'detail' | 'action' | 'task' | 'report';

/** 页面 binding 执行模式 */
export type PageExecutionMode = 'sync' | 'task';

/** 页面执行返回类型 */
export type PageExecutionKind = 'sync' | 'task' | 'approval';

/** 页面执行函数签名 - 所有渲染器统一使用 */
export type PageExecuteFn = (bindingId: string, payload: FormValues) => Promise<PageExecutionResult>;

/** 任务状态查询结果 */
export interface TaskStatusResult {
  taskId: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  progress?: number;
  message?: string;
  result?: JSONValue;
  error?: string;
  events?: TaskEvent[];
}

/** 任务事件 */
export interface TaskEvent {
  timestamp: string;
  type: 'info' | 'warning' | 'error' | 'progress';
  message: string;
  data?: JSONValue;
}

/** 表单值类型 - 所有表单统一使用 */
export type FormValues = Record<string, JSONValue>;

/** 诊断严重级别 */
export type DiagnosticSeverity = 'error' | 'warning' | 'info';

/** 已发布 binding 与最新函数契约的匹配状态 */
export type BindingFreshnessStatus =
  | 'fresh'
  | 'contract_missing'
  | 'function_missing'
  | 'function_version_stale'
  | 'input_schema_stale'
  | 'output_schema_stale'
  | 'governance_stale'
  | 'execution_mode_stale';

// ---------------------------------------------------------------------------
// Diagnostic
// ---------------------------------------------------------------------------

/** 结构化校验/就绪诊断消息 */
export interface Diagnostic {
  code: string;
  severity: DiagnosticSeverity;
  message: string;
  functionId?: string;
  field?: string;
}

/** 已发布 binding 的函数契约变化诊断 */
export interface BindingFreshnessDiagnostic {
  bindingId: string;
  functionId?: string;
  status: BindingFreshnessStatus;
  diagnostic: Diagnostic;
}

/** 函数审批策略，独立于 sync/task 执行模式 */
export interface ApprovalPolicy {
  required: boolean;
  policyKey?: string;
}

// ---------------------------------------------------------------------------
// FunctionSpec
// ---------------------------------------------------------------------------

/** 归一化后的单个函数能力规格 */
export interface FunctionSpec {
  id: string;
  version: string;
  enabled: boolean;
  deprecated?: boolean;
  inputSchema?: JSONSchema;
  outputSchema?: JSONSchema;

  // Catalog/search text. These fields are not runtime menu or page labels.
  summary?: LocalizedText;
  description?: LocalizedText;

  // Executable capability contract.
  resource?: string;
  operation?: string;
  capability?: CapabilityKind;
  execution?: FunctionExecution;
  approval: ApprovalPolicy;

  // 治理
  risk?: RiskLevel;
  permission?: string;
  tags?: string[];

  // 归一化诊断
  diagnostics?: Diagnostic[];
}

// ---------------------------------------------------------------------------
// ResourceSpec
// ---------------------------------------------------------------------------

/** 资源分类规格 */
export interface ResourceCategorySpec {
  key: string;
  labels: LocalizedText;
  order?: number;
}

/** 稳定业务资源或能力域规格 */
export interface ResourceSpec {
  key: string;
  labels: LocalizedText;
  description?: LocalizedText;
  category: ResourceCategorySpec;
  order?: number;
  tags?: string[];
  operations?: OperationSpec[];
  diagnostics?: Diagnostic[];
}

// ---------------------------------------------------------------------------
// OperationSpec
// ---------------------------------------------------------------------------

/** 函数在资源/页面中的操作语义 */
export interface OperationSpec {
  functionId: string;
  resourceKey?: string;
  operation: string;
  capability?: CapabilityKind;
  execution?: FunctionExecution;
  approval: ApprovalPolicy;
  risk?: RiskLevel;
  permission?: string;
  enabled: boolean;
  diagnostics?: Diagnostic[];
}

// ---------------------------------------------------------------------------
// PageSpec
// ---------------------------------------------------------------------------

/** 页面分类规格 */
export interface PageCategorySpec {
  key: string;
  labels: LocalizedText;
  order?: number;
}

/** 页面函数绑定 */
export interface PageFunctionBinding {
  id: string;
  functionId: string;
  usage: PageBindingUsage;
  selectors?: BindingSelectors;
  execution: PageBindingExecution;
}

/** 页面 binding selector 集合 */
export interface BindingSelectors {
  input: SelectorAST;
  output?: SelectorAST;
}

/** 选择器 AST */
export interface SelectorAST {
  assignments: Assignment[];
}

/** 选择器赋值 */
export interface Assignment {
  target: string;
  source: SelectorSource;
}

/** 选择器源 */
export interface SelectorSource {
  type: SelectorSourceType;
  path?: string;
  value?: JSONValue;
  transform?: TransformSpec;
}

/** 选择器源类型 */
export type SelectorSourceType = 'form' | 'row' | 'selection' | 'detail' | 'page_state' | 'literal';

/** 选择器转换 */
export interface TransformSpec {
  type: 'default' | 'format' | 'convert' | 'pick' | 'map';
  params?: Record<string, JSONValue>;
}

/** 页面 binding 执行策略 */
export interface PageBindingExecution {
  mode: PageExecutionMode;
  requireConfirm?: boolean;
}

/** 完整页面编排规格 */
export interface PageSpec {
  pageKey: string;
  type: PageType;
  resourceKey?: string;
  title: LocalizedText;
  description?: LocalizedText;
  category: PageCategorySpec;
  order?: number;
  icon?: string;
  navigation?: NavigationSpec;
  resource?: ResourcePageSpec;
  operation?: OperationPageSpec;
  task?: TaskPageSpec;
  report?: ReportPageSpec;
  bindings: PageFunctionBinding[];
  metadata?: Record<string, JSONValue>;
}

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

/** 操作页面规格 */
export interface OperationPageSpec {
  form: FormPresentationSpec;
  confirm?: ConfirmActionSpec;
  resultView?: ResultViewSpec;
}

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

/** 表单展示规格 */
export interface FormPresentationSpec {
  jsonSchema: JSONSchema;
  layout?: FormLayout;
  groups?: FormGroupSpec[];
  fields?: FormFieldSpec[];
  submitButton?: FormButtonSpec;
  cancelButton?: FormButtonSpec;
}

/** 表单布局 */
export type FormLayout = 'vertical' | 'horizontal' | 'inline' | 'grid';

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
  defaultValue?: JSONValue;
  enumOptions?: EnumOption[];
  widgetProps?: Record<string, JSONValue>;
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
  value?: JSONValue;
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
// PublishedPageSpec
// ---------------------------------------------------------------------------

/** 已发布的不可变页面快照 */
export interface PublishedPageSpec extends PageSpec {
  gameId?: string;
  env?: string;
  version: number;
  publishedAt: string;
  publishedBy?: string;
  rendererSchemaVersion: string;
  bindingContracts: BindingContractSnapshot[];
  bindingFreshness?: BindingFreshnessDiagnostic[];
}

/** 已发布页面 binding 的函数契约快照 */
export interface BindingContractSnapshot {
  bindingId: string;
  functionId: string;
  functionVersion?: string;
  inputSchemaDigest?: string;
  outputSchemaDigest?: string;
  risk?: RiskLevel;
  permission?: string;
  executionMode: PageExecutionMode;
  rendererSchemaVersion: string;
}

/** Page renderer 只消费该执行返回结构，不直接消费函数原始响应 */
export interface PageExecutionResult {
  kind: PageExecutionKind;
  requestId: string;
  traceId?: string;
  data?: JSONValue;
  taskId?: string;
  approvalId?: string;
  diagnostics?: Diagnostic[];
}

// ---------------------------------------------------------------------------
// ConsoleMenuSpec
// ---------------------------------------------------------------------------

/** 运行控制台菜单项 */
export interface ConsoleMenuItem {
  key: string;
  path: string;
  title: LocalizedText;
  locale: boolean; // 动态菜单项固定为 false
  icon?: string;
  order?: number;
  children?: ConsoleMenuItem[];
}

/** 运行控制台菜单规格 */
export interface ConsoleMenuSpec {
  items: ConsoleMenuItem[];
}

// ---------------------------------------------------------------------------
// Generated page suggestion
// ---------------------------------------------------------------------------

/** 默认页面建议质量 */
export type GeneratedPageQuality = 'ready' | 'basic' | 'needs_review' | 'blocked';

/** Server 生成的默认页面建议（发布前） */
export interface GeneratedPageSpec extends PageSpec {
  quality: GeneratedPageQuality;
  diagnostics?: Diagnostic[];
}

// ---------------------------------------------------------------------------
// Page draft
// ---------------------------------------------------------------------------

/** 页面草稿状态 */
export type PageDraftStatus = 'draft' | 'published' | 'archived';

/** 页面草稿摘要（列表用） */
export interface PageSpecDraftSummary {
  gameId?: string;
  env?: string;
  pageKey: string;
  type: PageType;
  resourceKey?: string;
  title: LocalizedText;
  category: PageCategorySpec;
  status: PageDraftStatus;
  draftRevision: number;
  publishedVersion?: number;
  updatedAt: string;
  updatedBy?: string;
}

/** 页面草稿详情 */
export interface PageSpecDraft extends PageSpec {
  gameId?: string;
  env?: string;
  status: PageDraftStatus;
  draftRevision: number;
  publishedVersion?: number;
  diagnostics?: Diagnostic[];
  bindingFreshness?: BindingFreshnessDiagnostic[];
  updatedAt: string;
  updatedBy?: string;
}

/** 页面版本记录 */
export interface PageVersionItem {
  version: number;
  status: string;
  message?: string;
  isCurrentDraft: boolean;
  isCurrentPublished: boolean;
  createdAt: string;
  createdBy?: string;
}

// ---------------------------------------------------------------------------
// Resource catalog
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
  severity: DiagnosticSeverity;
  message: string;
}

// ---------------------------------------------------------------------------
// Page proposal
// ---------------------------------------------------------------------------

/** 提案状态 */
export type ProposalStatus = 'pending' | 'accepted' | 'rejected' | 'expired';

/** 提案质量 */
export type ProposalQuality = GeneratedPageQuality;

/** 页面提案 */
export interface PageProposal {
  id: number;
  gameId?: string;
  env?: string;
  proposalKey: string;
  pageKey: string;
  pageType: PageType;
  resourceKey?: string;
  quality: ProposalQuality;
  generatorVersion: string;
  title: LocalizedText;
  description?: LocalizedText;
  categoryKey?: string;
  pageSpec: PageSpec;
  diagnostics?: DiagnosticInfo[];
  status: ProposalStatus;
  updatedAt: string;
  updatedBy?: string;
}

// ---------------------------------------------------------------------------
// Versioning
// ---------------------------------------------------------------------------

/** 变更类型 */
export type ChangeType = 'function_update' | 'semantic_update' | 'proposal_update' | 'draft_update' | 'publish';

/** 变更项 */
export interface ChangeItem {
  type: ChangeType;
  timestamp: string;
  version?: number;
  summary: string;
  details?: JSONValue;
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
  value?: JSONValue;
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
