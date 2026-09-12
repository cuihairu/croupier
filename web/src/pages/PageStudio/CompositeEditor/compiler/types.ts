/** 编译器共享类型与常量（编译/反编译两侧公用）。 */
/** 编译产物：与后端 CompositeSectionRequest 对齐（POST /versioning/pages/composite）。 */
export type CompiledSection = {
  /** 区块唯一 key（同函数多实例：fid、fid-2、fid-3…）；引用一律用 key。 */
  key: string;
  /** 分组名：dialog 区块按 group 聚合渲染弹窗；tab 区块按 group 聚合渲染 Tabs。 */
  group?: string;
  /** 页签标签（display='tab'）：同 group 内按标签聚合到 Tabs 对应页。 */
  tab?: string;
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
    /** 缺省兜底（U8）：page_state 源值缺失/null 时使用的字面量。 */
    transform?: { type: 'default'; params?: { value?: unknown } };
  }>;
  display?: 'inline' | 'dialog' | 'tab' | 'card';
  /** 卡片分组标题（display='card'）：同 group 区块渲染进同一卡片。 */
  cardTitle?: string;
  /** 区块级条件显示（U10）：按页面状态求值，false 时不渲染（执行不变）。
   * 编辑态 props.visibleWhen {expr,op,value} 编译期拆 key/path。 */
  visibleWhen?: CompiledCondition;
  rowActions?: CompiledAction[];
  toolbarActions?: CompiledAction[];
  onSuccessRefresh?: string[];
};

/** 编译产物的条件形态（wire ConditionSpec 叶子）：key=来源区块 key。 */
export type CompiledCondition = {
  kind: 'equals' | 'notEquals' | 'exists';
  key: string;
  path: string;
  value?: unknown;
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
  /** 页签标签（display=tab；LocalizedText 或遗留 string）。 */
  tab?: unknown;
  /** 卡片分组标题（display=card；LocalizedText 或遗留 string）。 */
  cardTitle?: unknown;
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
    /** 缺省兜底（U8）：page_state 源值缺失/null 时使用的字面量。 */
    transform?: { type: 'default'; params?: { value?: unknown } };
  }>;
  autoRun?: boolean;
  refreshOn?: string[];
  display?: string;
  /** 区块级条件显示（宽松形态：kind/key/path/value；嵌套组合条件编辑器
   * 不产出，decompile 遇到时丢弃并警告）。 */
  visibleWhen?: { kind?: string; key?: string; path?: string; value?: unknown };
  onSuccessRefresh?: string[];
  table?: { columns?: Array<{ key?: string }>; rowActions?: Array<Record<string, unknown>> };
  toolbar?: { actions?: Array<Record<string, unknown>> };
}
