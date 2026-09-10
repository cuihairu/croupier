/** 编译器共享类型与常量（编译/反编译两侧公用）。 */
/** 编译产物：与后端 CompositeSectionRequest 对齐（POST /versioning/pages/composite）。 */
export type CompiledSection = {
  /** 区块唯一 key（同函数多实例：fid、fid-2、fid-3…）；引用一律用 key。 */
  key: string;
  /** 弹窗分组名（modal 容器派生）；dialog 区块按 group 聚合渲染。 */
  group?: string;
  /** 通用事件绑定（发布触发点）。 */
  events?: Array<{
    event: string;
    action: { kind: string; target: string; params?: Record<string, string> };
    chain?: Array<{ kind: string; target: string; params?: Record<string, string> }>;
  }>;
  functionId: string;
  view: 'table' | 'fields' | 'form';
  title: string;
  span: number;
  autoRun: boolean;
  refreshOn?: string[];
  /** 常量表单（staticForm）：不绑定函数，form.jsonSchema 由编辑器定义。 */
  static?: boolean;
  form?: { jsonSchema: Record<string, unknown> };
  /** 显式参数映射（sourceNodeId 编译期解析为 section key）。 */
  inputAssignments?: Array<{
    target: string;
    kind: 'page_state' | 'literal';
    key?: string;
    path?: string;
    value?: unknown;
  }>;
  display?: 'inline' | 'dialog';
  rowActions?: CompiledAction[];
  toolbarActions?: CompiledAction[];
  onSuccessRefresh?: string[];
};

export type CompiledAction = {
  label: string;
  targetSection: string;
  params?: Record<string, string>;
  danger?: boolean;
  chain?: Array<{ kind: string; target: string; params?: Record<string, string> }>;
};

export interface CompileResult {
  sections: CompiledSection[];
  warnings: string[];
}

export const VIEW_MAP: Record<string, 'table' | 'fields' | 'form'> = {
  fnTable: 'table',
  fnFields: 'fields',
  fnForm: 'form',
  staticForm: 'form',
};

/** 区块 key 合法字符（与后端 pageKey/section key 规则一致：字母数字开头，可含 . _ -）。 */
export const SECTION_KEY_RE = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;

/** spec section 的宽松形态（回读自提案/发布 PageSpec）。 */
export interface SpecSectionLike {
  key?: string;
  group?: string;
  events?: Array<{
    event: string;
    action: { kind: string; target: string; params?: Record<string, string> };
    chain?: Array<{ kind: string; target: string; params?: Record<string, string> }>;
  }>;
  bindingId?: string;
  functionId?: string;
  view?: string;
  title?: unknown;
  span?: number;
  /** 常量表单（staticForm）：无绑定，schema 由编辑器定义。 */
  static?: boolean;
  form?: { jsonSchema?: Record<string, unknown> };
  /** 显式参数映射。 */
  inputAssignments?: Array<{
    target: string;
    kind: 'page_state' | 'literal';
    key?: string;
    path?: string;
    value?: unknown;
  }>;
  autoRun?: boolean;
  refreshOn?: string[];
  display?: string;
  onSuccessRefresh?: string[];
  table?: { columns?: Array<{ key?: string }>; rowActions?: Array<Record<string, unknown>> };
  toolbar?: { actions?: Array<Record<string, unknown>> };
}
