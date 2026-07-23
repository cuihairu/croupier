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

/** JSON Schema 对象 (draft-07 / 2020-12) */
export type JSONSchema = Record<string, unknown>;

/** Formily 兼容的 JSON Schema，包含 x-component 等扩展 */
export type FormilySchema = Record<string, unknown>;

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

/** 操作的页面生成语义 */
export type OperationKind =
  | 'list'
  | 'get'
  | 'create'
  | 'update'
  | 'delete'
  | 'action'
  | 'task'
  | 'report';

/** 操作在页面中的放置位置 */
export type OperationPlacement =
  | 'query'
  | 'tableData'
  | 'detailData'
  | 'rowAction'
  | 'detailAction'
  | 'toolbarAction'
  | 'batchAction'
  | 'standalone';

/** 风险等级 */
export type RiskLevel = 'safe' | 'warning' | 'high' | 'danger';

/** 页面类型 */
export type PageType = 'entity' | 'operation' | 'task' | 'report';

/** 诊断严重级别 */
export type DiagnosticSeverity = 'error' | 'warning' | 'info';

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

  // 显示与搜索
  displayName?: LocalizedText;
  summary?: LocalizedText;
  description?: LocalizedText;

  // 资源/页面语义
  category?: string;
  categoryDisplay?: LocalizedText;
  entity?: string;
  entityDisplay?: LocalizedText;
  operation?: string;
  operationDisplay?: LocalizedText;
  operationKind?: OperationKind;
  placement?: OperationPlacement;
  pageHint?: string;

  // 治理
  risk?: RiskLevel;
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
  kind: OperationKind;
  placement: OperationPlacement;
  labels: LocalizedText;
  risk?: RiskLevel;
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
  functionId: string;
  role: OperationPlacement;
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
  version: number;
  publishedAt: string;
  publishedBy?: string;
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
export type GeneratedPageQuality = 'ready' | 'needs_review' | 'blocked';

/** Server 生成的默认页面建议（发布前） */
export interface GeneratedPageSpec extends PageSpec {
  quality: GeneratedPageQuality;
  diagnostics?: Diagnostic[];
}

// ---------------------------------------------------------------------------
// Page draft (workspace)
// ---------------------------------------------------------------------------

/** 页面草稿状态 */
export type PageDraftStatus = 'draft' | 'published' | 'archived';

/** 页面草稿摘要（列表用） */
export interface PageSpecDraftSummary {
  pageKey: string;
  type: PageType;
  resourceKey?: string;
  title: LocalizedText;
  category: PageCategorySpec;
  status: PageDraftStatus;
  draftVersion: number;
  publishedVersion?: number;
  updatedAt: string;
  updatedBy?: string;
}

/** 页面草稿详情 */
export interface PageSpecDraft extends PageSpec {
  status: PageDraftStatus;
  draftVersion: number;
  publishedVersion?: number;
  diagnostics?: Diagnostic[];
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
