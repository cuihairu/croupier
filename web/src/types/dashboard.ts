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
export type JSONSchema = Record<string, unknown>;

/** Formily 兼容的 JSON Schema，包含 x-component 等扩展 */
export type FormilySchema = Record<string, unknown>;

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
export type FunctionExecution = 'sync' | 'task' | 'approval';

/** 页面类型 */
export type PageType = 'entity' | 'operation' | 'task' | 'report';

/** 页面 binding 在运行期的用途 */
export type PageBindingUsage = 'query' | 'detail' | 'action' | 'task' | 'report';

/** 页面 binding 执行模式 */
export type PageExecutionMode = 'sync' | 'task';

/** 页面执行返回类型 */
export type PageExecutionKind = 'sync' | 'task' | 'approval';

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
  inputFormilySchema?: FormilySchema;
  outputSchema?: JSONSchema;

  // Catalog/search text. These fields are not runtime menu or page labels.
  summary?: LocalizedText;
  description?: LocalizedText;

  // Executable capability contract.
  resource?: string;
  operation?: string;
  capability?: CapabilityKind;
  execution?: FunctionExecution;

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
  inputMapping?: JSONValue;
  outputMapping?: JSONValue;
  execution: PageBindingExecution;
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
  schema: FormilySchema;
  bindings: PageFunctionBinding[];
  metadata?: Record<string, unknown>;
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
