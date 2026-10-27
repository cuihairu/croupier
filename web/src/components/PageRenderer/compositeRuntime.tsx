import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { Dispatch, MutableRefObject, ReactNode, SetStateAction } from 'react';
import { App, Descriptions } from 'antd';
import { ExclamationCircleOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { localizedText } from '@/utils/localizedText';
import { extractApiErrorCode } from '@/utils/apiError';
import { resolveStepParams } from './runtime';
import { describeExecFailure, describeExecOutcome, isErrorExecResult } from './execFeedback';
import type { ExecFailure, ExecOutcome } from './execFeedback';
import { confirmFieldTitle, describeConfirmEntries } from './confirmContent';
import type { ConfirmEntry } from './confirmContent';
import type {
  FormPresentationSpec,
  FormValues,
  LocalizedText,
  PageExecutionResult,
  PaginationSpec,
} from '@/types/dashboard';

/**
 * 表单区块提交出口结果：ok=true 仅表示业务执行成功（同步结果），调用方据此
 * 关弹窗；replayed=true 表示本次返回是幂等重放（服务端已落地结果回读，非新
 * 下发）——结果语义仍是成功，但反馈文案需说明未重复执行。
 */
export type SubmitFormOutcome = { ok: true; replayed: boolean } | { ok: false };

/**
 * B1 运行时核心（状态/级联/动作链唯一实现）：组合页发布渲染器（CompositeRenderer）
 * 与编辑器预览（PreviewRuntime）共用同一状态机——结果账本、修订号级联、
 * 失败策略、动作链、危险确认、表单提交出口、服务端分页。三个执行面
 * （模拟预览 / 草稿真实预览 / 发布快照执行）只差异注入的 execute 适配器
 * 与少量投影选项，交互行为不再各自维护一份。
 *
 * 区块契约是结构最小集（RuntimeSection）：发布端 CompositeSection 直接满足；
 * 编辑器预览把画布树节点投影成同形态区块（key=节点 id、refreshOn=refreshOnNode）。
 */

/** 动作链步骤（发布事件绑定与编辑器动作规格的公共形态）。 */
export interface RuntimeStep {
  kind: string;
  target: string;
  params?: Record<string, string>;
}

/** 事件绑定公共形态。 */
export interface RuntimeEventBinding {
  event: string;
  action: RuntimeStep;
  chain?: RuntimeStep[];
}

/** 运行时区块最小契约（结构兼容 CompositeSection）。 */
export interface RuntimeSection {
  key: string;
  /** 执行键（发布端=绑定 id）。 */
  bindingId?: string;
  title?: LocalizedText | string;
  view?: string;
  autoRun?: boolean;
  display?: string;
  group?: string;
  refreshOn?: string[];
  cascadePolicy?: 'clear' | 'keep' | 'pause';
  onSuccessRefresh?: string[];
  events?: RuntimeEventBinding[];
  form?: FormPresentationSpec;
  table?: { pagination?: PaginationSpec };
  static?: boolean;
}

/** 运行时状态条目：函数结果 / 辅助写入（选中、表单值）/ 失败标记的联合。 */
export type RuntimeResult =
  | PageExecutionResult
  | {
      data?: Record<string, unknown>;
      selectedRow?: Record<string, unknown>;
      selectedRows?: Record<string, unknown>[];
      values?: Record<string, unknown>;
    }
  | { error: string; errorCode?: string }
  | null;

/** 危险动作确认框内容：提交参数条目 + 待执行的动作链步骤。 */
export interface ConfirmContentModel {
  entries?: ConfirmEntry[];
  steps?: Array<{ kind: string; target: string }>;
}

/** 确认框内容：参数条目按字段标题渲染（Descriptions），链步骤逐行列目标。 */
export function renderConfirmContent(model: ConfirmContentModel): ReactNode {
  const { entries = [], steps = [] } = model;
  if (entries.length === 0 && steps.length === 0) return null;
  return (
    <div style={{ maxHeight: 320, overflowY: 'auto' }}>
      {entries.length > 0 ? (
        <Descriptions size="small" column={1} bordered>
          {entries.map((entry) => (
            <Descriptions.Item key={entry.key} label={entry.title}>
              {entry.value}
            </Descriptions.Item>
          ))}
        </Descriptions>
      ) : null}
      {steps.length > 0 ? (
        <ul style={{ margin: '12px 0 0', paddingLeft: 18 }}>
          {steps.map((step, index) => (
            <li key={index}>
              {step.kind} → {step.target}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

/** D3 组合页表格分页状态（仅 enabled 服务端分页的表格区块持有）。
 * page 为 1-based 页码；inputFingerprint 是上次请求的非分页入参指纹（= 上游
 * 筛选/查询条件）——入参变化回首页，入参不变（操作成功后刷新、手动重跑）保持
 * 当前页码与每页条数；total 由执行器提供，缺省时按「本页满则可能有下一页」
 * 估算上界（无总数边界），避免把缺 total 误判为只剩当前页。 */
export interface TablePageState {
  page: number;
  pageSize: number;
  inputFingerprint: string;
  total: number;
}

/** Table pagination 配置（服务端分页受控形态 + 客户端分页缺省形态）。 */
export interface TablePaginationProps {
  current?: number;
  pageSize?: number;
  total?: number;
  showSizeChanger?: boolean;
  showQuickJumper?: boolean;
  pageSizeOptions?: string[];
  showTotal?: (total: number) => ReactNode;
  onChange?: (page: number, pageSize: number) => void;
}

const TABLE_DEFAULT_SIZE = 20;
const TABLE_MAX_SIZE = 200;
const TABLE_DEFAULT_PAGE_SIZES: number[] = [10, 20, 50];

/** 每页条数钳制到 [1, TABLE_MAX_SIZE]：spec 里配过大条数也不下发超大请求。 */
function clampPageSize(size: number): number {
  return Math.min(Math.max(1, Math.floor(size)), TABLE_MAX_SIZE);
}

/** 读取表格分页配置（PaginationSpec 缺省值：defaultSize 20、pageSizes 10/20/50）。 */
export function tablePaginationOf(sec: RuntimeSection): PaginationSpec {
  const cfg = sec.table?.pagination;
  const pageSizes = (cfg?.pageSizes ?? TABLE_DEFAULT_PAGE_SIZES).map(clampPageSize);
  return {
    enabled: !!cfg?.enabled,
    defaultSize: cfg?.defaultSize ? clampPageSize(cfg.defaultSize) : undefined,
    pageSizes: [...new Set(pageSizes)].sort((a, b) => a - b),
  };
}

/** 数据适配层：分页语义 → 请求参数。offset 由 (page-1)*pageSize 派生并与
 * page/pageSize 一并下发——不把偏移量当页码传，也不把页码当偏移量传。 */
export function paginationRequestParams(page: number, pageSize: number): Record<string, number> {
  const p = Math.max(1, Math.floor(page));
  const size = clampPageSize(pageSize);
  return { page: p, pageSize: size, offset: (p - 1) * size };
}

/** 无总数边界：结果缺 total 时按「本页满则可能有下一页」估上界；不满页即收口。 */
export function estimateTotal(page: number, pageSize: number, itemCount: number, total: unknown) {
  if (typeof total === 'number' && Number.isFinite(total) && total >= 0) {
    return Math.floor(total);
  }
  const full = itemCount >= pageSize;
  return Math.max(0, page - 1) * pageSize + itemCount + (full ? 1 : 0);
}

/** U9：失败结果判定——run 的 catch 写入的 {error} 标记，或执行结果
 * 自带顶层 error 字段的防御形态（kind/data 之外不含 error 的正常结果、
 * null 占位、undefined 均不算失败）。 */
export function isFailedResult(r: unknown): boolean {
  return !!r && typeof r === 'object' && !!(r as { error?: unknown }).error;
}

export interface CompositeRuntimeOptions<T extends RuntimeSection> {
  sections: T[];
  /** 执行适配器（B1 的注入点）：模拟预览 / 草稿真实预览 / 发布快照执行
   * 各自实现；失败须抛错（run 据此写失败标记并走失败策略）。 */
  execute: (sec: T, inputs: Record<string, unknown>) => Promise<PageExecutionResult | null>;
  /** 纯预览模式（发布端 preview 入参既有语义）：不执行，run 直接返回 null。 */
  preview?: boolean;
  /** 常量表单/表单提交值并入页面状态（显式参数映射的数据来源）。
   * V5 mode='merge'：按区块键浅合并（selectedRow/data 分支共存），默认 replace。 */
  onPageStateMerge?: (
    key: string,
    values: Record<string, unknown>,
    mode?: 'replace' | 'merge',
  ) => void;
  /** 动作目标 → 区块解析（缺省按 key|group|bindingId 命中）。 */
  resolveSection?: (target: string) => T | undefined;
  /** 区块的显式参数映射求值（编辑器预览：inputAssignments 就地求值）。 */
  inputsForSection?: (sec: T) => Record<string, unknown>;
  /** 动作参数表达式求值空间（编辑器预览：sectionKey → 节点状态）；缺省 results。 */
  paramsSpace?: () => Record<string, unknown>;
  /** 自动执行判定（缺省 autoRun 且非弹窗区块）。 */
  shouldAutoRun?: (sec: T) => boolean;
  /** 表单当前值写入防抖毫秒（缺省 300；预览传 0 立即写入）。 */
  valuesFlushMs?: number;
  /** 常量表单值写入防抖毫秒（缺省 400；预览传 0 立即写入）。 */
  staticFlushMs?: number;
  /** 重跑时保留表格选中状态（编辑器预览语义：模拟数据重跑不打断选中）。 */
  preserveSelection?: boolean;
}

/** 运行时句柄：字段名与原 CompositeRenderer 内部局部量一一对应，渲染器
 * 迁移即重命名，行为不变。 */
export interface CompositeRuntime<T extends RuntimeSection> {
  results: Record<string, RuntimeResult>;
  resultsRef: MutableRefObject<Record<string, RuntimeResult>>;
  running: Record<string, boolean>;
  sectionInputs: Record<string, Record<string, unknown>>;
  revisions: Record<string, number>;
  /** 执行反馈账本（D4）：失败定位 + 审批/任务待处理。 */
  execFailures: Record<string, ExecFailure | null>;
  execPending: Record<string, { outcome: ExecOutcome; id?: string } | undefined>;
  tablePages: Record<string, TablePageState>;
  dialogKey: string | null;
  setDialogKey: Dispatch<SetStateAction<string | null>>;
  /** 执行区块（inputs = 区块输入 + overrides 合并），失败上抛。 */
  run: (sec: T, overrides?: Record<string, unknown>) => Promise<PageExecutionResult | null>;
  runRef: MutableRefObject<
    (sec: T, overrides?: Record<string, unknown>) => Promise<PageExecutionResult | null>
  >;
  /** 动作目标解析（key|group|bindingId / 自定义 resolver）。 */
  resolveTarget: (target: string) => T | undefined;
  resolveParams: (
    params: Record<string, string> | undefined,
    ctx?: Record<string, unknown>,
  ) => Record<string, unknown>;
  handleTablePageChange: (sec: T, page: number, size: number) => void;
  tablePaginationProps: (sec: T) => TablePaginationProps;
  confirmDanger: (opts: {
    label: string;
    entries?: ConfirmEntry[];
    steps?: ConfirmContentModel['steps'];
    onOk: () => void;
  }) => void;
  formSpecOfTarget: (target: string) => FormPresentationSpec | undefined;
  openDialog: (opts: {
    target: string;
    params: Record<string, unknown>;
    danger?: boolean;
    label?: string;
    steps?: ConfirmContentModel['steps'];
    afterOpen?: () => void;
  }) => void;
  runChain: (chain: RuntimeStep[] | undefined, ctx?: Record<string, unknown>) => Promise<void>;
  fireEvent: (sec: T, eventName: string, ctx?: Record<string, unknown>) => void;
  setSelectionState: (key: string, rows: Record<string, unknown>[]) => void;
  /** 表单当前值缓冲写入（防抖后进 results[key].values）。 */
  mergeFormValues: (key: string, values: Record<string, unknown>) => void;
  /** 常量表单值缓冲写入（防抖后进 results[key] + 页面状态）。 */
  mergeStaticValues: (key: string, values: Record<string, unknown>) => void;
  /** 行字段映射：params 目标参数名 → 本行字段名（row.x / {{row.x}} / 裸字段）。 */
  mapRowParams: (
    mapping: Record<string, string> | undefined,
    row: Record<string, unknown>,
  ) => Record<string, unknown>;
  clearExecFeedbackOf: (key: string) => void;
  /** 表单提交统一出口（D4）：返回 true 仅表示同步业务成功（调用方关弹窗）。 */
  submitForm: (sec: T, values: FormValues) => Promise<SubmitFormOutcome>;
}

/** B1 运行时核心（见模块注释）：唯一的状态/级联/动作链实现。 */
export function useCompositeRuntime<T extends RuntimeSection>(
  options: CompositeRuntimeOptions<T>,
): CompositeRuntime<T> {
  const {
    sections,
    execute,
    preview = false,
    onPageStateMerge,
    resolveSection,
    inputsForSection,
    paramsSpace,
    shouldAutoRun,
    valuesFlushMs = 300,
    staticFlushMs = 400,
    preserveSelection = false,
  } = options;
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const [results, setResults] = useState<Record<string, RuntimeResult>>({});
  const [running, setRunning] = useState<Record<string, boolean>>({});
  const [sectionInputs, setSectionInputs] = useState<Record<string, Record<string, unknown>>>({});
  // B2：结果修订号——每个区块每次产出新结果（成功或失败）+1。仅由 run/常量
  // 表单写入，选区等辅助写入不进修订号。级联由修订号驱动：上游同态新数据
  // 也改变修订号，下游随之重跑（旧 key 集合信号看不到「成功→成功·新数据」）。
  const [revisions, setRevisions] = useState<Record<string, number>>({});
  // B2：请求序号（ref，同步占用）——并发同区块重跑时仅最新一次可落账（旧
  // 响应拒收）。与结果修订号分离：修订号在结果真正落地时才前进，信号因此
  // 只反映已落账的结果，不会被在飞请求预先污染。
  const execSeqRef = useRef<Record<string, number>>({});
  // B2：级联联动账本——每区块已消费的上游修订号 + 上次失败态（同态连续失败
  // 不重复提示）。首次看到的上游结果照常联动（refreshOn 语义保持）。
  const cascadeSeenRef = useRef<Record<string, { rev: number; failed: boolean }>>({});
  // D3：表格服务端分页状态（ref 为请求真值——run 在同一事件帧内读取，
  // state 版仅驱动 Table 的 current/total 展示）
  const tablePagesRef = useRef<Record<string, TablePageState>>({});
  const [tablePages, setTablePages] = useState<Record<string, TablePageState>>({});
  const [dialogKey, setDialogKey] = useState<string | null>(null);
  // D4：执行反馈按区块留存——失败（稳定错误码 + 字段级定位）与待处理结果
  // （审批 / 任务）。approval/task 一律不触发 success 事件与刷新级联：任务
  // 提交与业务执行成功是两件事，不能共用一条提示语义。
  const [execFailures, setExecFailures] = useState<Record<string, ExecFailure | null>>({});
  const [execPending, setExecPending] = useState<
    Record<string, { outcome: ExecOutcome; id?: string } | undefined>
  >({});
  // 运行时状态快照 ref（事件帧内求值/执行读取，避免 setState 异步导致同帧读旧值）
  const resultsRef = useRef(results);
  // 常量表单（static）值缓冲：防抖后并入 results 驱动 refreshOn 联动。
  // data+values 双形态：data 供 refreshOn 联动上游合并（既有读取），
  // values 对齐表达式语言（{{sf.values.x}} / 区块级 visibleWhen 的寻址段）。
  const staticMergeRef = useRef<
    Record<string, { data: Record<string, unknown>; values: Record<string, unknown> }>
  >({});
  const staticTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // V5 fnForm 当前值缓冲：防抖写入 results[key].values（求值用，不触发 refreshOn）
  const valuesMergeRef = useRef<Record<string, Record<string, unknown>>>({});
  const valuesTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const scheduleValuesFlush = useCallback(() => {
    if (valuesTimerRef.current) clearTimeout(valuesTimerRef.current);
    valuesTimerRef.current = setTimeout(
      () => {
        valuesTimerRef.current = null;
        setResults((prev) => {
          let next = prev;
          for (const [key, values] of Object.entries(valuesMergeRef.current)) {
            const cur = (next[key] ?? {}) as Record<string, unknown>;
            next = { ...next, [key]: { ...cur, values } };
          }
          return next;
        });
        valuesMergeRef.current = {};
      },
      valuesFlushMs > 0 ? valuesFlushMs : 0,
    );
  }, [valuesFlushMs]);
  /** V5 表格选中行状态写入（selectedRow/selectedRows；不触发 refreshOn 自动重跑）。
   * 同步刷新 resultsRef——同一事件帧内的动作链求值（runChain）立即可见；
   * 并以 merge 模式并入页面状态（服务端 inputAssignment /selectedRow/* 求值）。 */
  const setSelectionState = useCallback(
    (key: string, rows: Record<string, unknown>[]) => {
      const cur = (resultsRef.current[key] ?? {}) as Record<string, unknown>;
      const entry = { ...cur, selectedRow: rows[0], selectedRows: rows };
      resultsRef.current = { ...resultsRef.current, [key]: entry };
      setResults(resultsRef.current);
      onPageStateMerge?.(key, { selectedRow: rows[0], selectedRows: rows }, 'merge');
    },
    [onPageStateMerge],
  );
  const scheduleStaticFlush = useCallback(() => {
    if (staticTimerRef.current) clearTimeout(staticTimerRef.current);
    staticTimerRef.current = setTimeout(
      () => {
        staticTimerRef.current = null;
        setResults((prev) => ({ ...prev, ...staticMergeRef.current }));
        // 常量表单产出视为一次新结果：修订号 +1 驱动 refreshOn 联动
        //（「常量表单值变化 → 下游表格重跑」在预览与发布页同一语义）。
        setRevisions((prev) => {
          let next = prev;
          for (const key of Object.keys(staticMergeRef.current)) {
            next = { ...next, [key]: (next[key] ?? 0) + 1 };
          }
          return next;
        });
        for (const [key, v] of Object.entries(staticMergeRef.current)) {
          // 双形态快照：扁平值兼容遗留 /字段 路径 + values 包装供 {{var.values.x}}
          onPageStateMerge?.(key, { ...v.data, values: v.data });
        }
      },
      staticFlushMs > 0 ? staticFlushMs : 0,
    );
  }, [staticFlushMs, onPageStateMerge]);
  useEffect(
    () => () => {
      if (staticTimerRef.current) clearTimeout(staticTimerRef.current);
      if (valuesTimerRef.current) clearTimeout(valuesTimerRef.current);
    },
    [],
  );

  const sectionsRef = useRef(sections);
  sectionsRef.current = sections;
  const inputsRef = useRef(sectionInputs);
  inputsRef.current = sectionInputs;

  const run = useCallback(
    async (sec: T, overrides?: Record<string, unknown>) => {
      if (preview) return null;
      const baseInputs = {
        ...(inputsRef.current[sec.key] || {}),
        ...(inputsForSection ? inputsForSection(sec) : {}),
        ...(overrides || {}),
      };
      // D3：表格区块入参 = 上游筛选/查询条件 + 派生分页参数（page/pageSize/
      // offset）。指纹只覆盖非分页入参——筛选变化回首页，入参不变（操作成功后
      // 刷新、手动重跑）保持当前页码与每页条数。
      let merged = baseInputs;
      const pagination = tablePaginationOf(sec);
      if (sec.view === 'table' && pagination.enabled) {
        const fingerprint = JSON.stringify(baseInputs);
        const prev = tablePagesRef.current[sec.key];
        const sameFilter = !!prev && prev.inputFingerprint === fingerprint;
        const next: TablePageState = {
          page: sameFilter ? prev.page : 1,
          pageSize: clampPageSize(prev?.pageSize ?? pagination.defaultSize ?? TABLE_DEFAULT_SIZE),
          inputFingerprint: fingerprint,
          total: prev?.total ?? 0,
        };
        tablePagesRef.current = { ...tablePagesRef.current, [sec.key]: next };
        setTablePages(tablePagesRef.current);
        merged = { ...baseInputs, ...paginationRequestParams(next.page, next.pageSize) };
      }
      setRunning((prev) => ({ ...prev, [sec.key]: true }));
      // B2：请求序号发请求前同步占用——并发同区块重跑时仅序号匹配的那次可
      // 落账（旧响应拒收）。修订号留给落账时 +1（成功/失败都算一次新结果，
      // 下游 cascadePolicy 语义保持），因此级联信号只反映已落账的结果。
      const execSeq = (execSeqRef.current[sec.key] ?? 0) + 1;
      execSeqRef.current[sec.key] = execSeq;
      const isCurrent = () => execSeqRef.current[sec.key] === execSeq;
      try {
        const result = await execute(sec, merged);
        if (!isCurrent()) return null; // 旧响应拒收
        setRevisions((prev) => ({ ...prev, [sec.key]: (prev[sec.key] ?? 0) + 1 }));
        setResults((prev) => {
          let next: RuntimeResult = result || null;
          // 编辑器预览语义：模拟数据重跑不打断表格选中
          if (preserveSelection) {
            const prevEntry = (prev[sec.key] ?? {}) as Record<string, unknown>;
            if (prevEntry.selectedRow || prevEntry.selectedRows) {
              next = {
                ...(next as unknown as Record<string, unknown>),
                selectedRow: prevEntry.selectedRow,
                selectedRows: prevEntry.selectedRows,
              } as RuntimeResult;
            }
          }
          return { ...prev, [sec.key]: next };
        });
        // V5：函数输出并入页面状态 data（{{var.data.x}} 服务端求值来源）
        onPageStateMerge?.(
          sec.key,
          { data: (result as { data?: unknown })?.data ?? null },
          'merge',
        );
        // D3：服务端分页落账后刷新分页展示——执行器给 total 则原样使用，
        // 缺 total 按「本页满则可能有下一页」估上界（不满页即收口）。
        if (sec.view === 'table' && tablePaginationOf(sec).enabled) {
          const data = ((result || {}) as { data?: Record<string, unknown> }).data ?? {};
          const items = Array.isArray(data.items) ? data.items : [];
          const cur = tablePagesRef.current[sec.key];
          if (cur) {
            tablePagesRef.current = {
              ...tablePagesRef.current,
              [sec.key]: {
                ...cur,
                total: estimateTotal(cur.page, cur.pageSize, items.length, data.total),
              },
            };
            setTablePages(tablePagesRef.current);
          }
        }
        // 操作类区块成功后刷新目标（发邮件成功 → 刷新玩家表格）。
        // D4：审批待处理 / 任务已提交≠业务执行成功——此时业务侧尚未变更，
        // 刷新只会拉回旧数据，故按结果类别门控。
        if (
          (sec.view === 'form' || sec.view === 'actions') &&
          sec.onSuccessRefresh?.length &&
          describeExecOutcome(result) === 'success'
        ) {
          for (const target of sec.onSuccessRefresh) {
            const t = sectionsRef.current.find((x) => x.key === target);
            if (t) runRef.current(t).catch(() => undefined);
          }
        }
        return result;
      } catch (e) {
        // U9：失败也写入 results（error 标记）——下游 cascadePolicy 据此判定
        // 级联行为；错误继续上抛（弹窗提交路径靠 catch 保持弹窗开启并 toast）。
        // T8：附稳定错误码（executor_unbound → 区块内渲染绑定引导空态）
        if (!isCurrent()) throw e; // 旧失败不覆盖新结果
        setRevisions((prev) => ({ ...prev, [sec.key]: (prev[sec.key] ?? 0) + 1 }));
        setResults((prev) => ({
          ...prev,
          [sec.key]: {
            error: e instanceof Error ? e.message : String(e),
            errorCode: extractApiErrorCode(e),
          },
        }));
        throw e;
      } finally {
        setRunning((prev) => ({ ...prev, [sec.key]: false }));
      }
    },
    [execute, onPageStateMerge, preview, inputsForSection, preserveSelection],
  );

  const runRef = useRef(run);
  runRef.current = run;

  // autoRun：加载即执行（inline 区块）
  useEffect(() => {
    for (const sec of sections) {
      const auto = shouldAutoRun ? shouldAutoRun(sec) : !!sec.autoRun && sec.display !== 'dialog';
      if (auto) runRef.current(sec).catch(() => undefined);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // refreshOn 联动：上游产出新结果时自动重跑本区块（跨函数联动）。
  useEffect(() => {
    resultsRef.current = results;
  }, [results]);

  /** U9 refreshOn 级联失败策略：上游依赖最新结果为失败时本区块的行为——
   * pause（默认）= 本次不重跑、数据保持并提示；keep = 不重跑、静默保留
   * 上次结果；clear = 不重跑、清空本区块数据（明确失效）。 */
  const applyCascadePolicy = useCallback(
    (sec: T, failedDep: string) => {
      const policy = sec.cascadePolicy ?? 'pause';
      if (policy === 'clear') {
        setResults((prev) => ({ ...prev, [sec.key]: null }));
        return;
      }
      if (policy === 'pause') {
        message.warning(
          intl.formatMessage(
            {
              id: 'component.pageRenderer.composite.cascadePaused',
              defaultMessage: '区块「{upstream}」执行失败，「{section}」联动已暂停',
            },
            { upstream: failedDep, section: localizedText(sec.title, 'zh-CN', sec.key) },
          ),
        );
      }
      // keep：静默保留上次结果
    },
    [message, intl],
  );

  // B2：级联信号 = 各区块「修订号:成败标记」。修订号是每次产出新结果的单调
  // 计数（成功/失败都算），因此「连续两次成功但数据不同」也会改变信号——旧
  // 信号只看 key 集合 + o/e 翻转，同态新值触不到下游。
  const cascadeSignal = Object.keys(results)
    .sort()
    .map((k) => `${k}:${revisions[k] ?? 0}:${isFailedResult(results[k]) ? 'e' : 'o'}`)
    .join(',');

  // B2：依赖循环判定——refreshOn 图回溯到自身即拒绝联动（A→B→A）。仅标记
  // 处于循环中的区块，其余级联照常。
  const cyclicSections = useMemo(() => {
    const graph = new Map<string, string[]>();
    for (const sec of sections) graph.set(sec.key, sec.refreshOn || []);
    const result = new Set<string>();
    for (const sec of sections) {
      if (!sec.refreshOn?.length) continue;
      const visiting = new Set<string>();
      const visited = new Set<string>();
      const dfs = (node: string): boolean => {
        for (const next of graph.get(node) || []) {
          if (next === sec.key) return true;
          if (visited.has(next) || visiting.has(next)) continue;
          visiting.add(next);
          if (dfs(next)) return true;
          visiting.delete(next);
          visited.add(next);
        }
        return false;
      };
      visiting.add(sec.key);
      if (dfs(sec.key)) result.add(sec.key);
    }
    return result;
  }, [sections]);

  useEffect(() => {
    const seen = cascadeSeenRef.current;
    for (const sec of sections) {
      if (!sec.refreshOn?.length || sec.display === 'dialog') continue;
      const depRev = sec.refreshOn
        .filter((dep) => dep in results)
        .reduce((max, dep) => Math.max(max, revisions[dep] ?? 0), 0);
      if (depRev === 0) continue; // 上游尚无结果
      // 在跑时不消费修订号：等 running 翻转后重入本 effect，用最新上游数据重跑
      if (running[sec.key]) continue;
      if (seen[sec.key]?.rev >= depRev) continue; // 该修订号已联动过
      const failedDep = sec.refreshOn.find((dep) => isFailedResult(results[dep]));
      const prevFailed = seen[sec.key]?.failed ?? false;
      seen[sec.key] = { rev: depRev, failed: !!failedDep };
      if (cyclicSections.has(sec.key)) continue; // 循环依赖：拒绝联动
      if (failedDep) {
        // U9：失败态首次出现才提示（同态连续失败不重复打扰）
        if (!prevFailed) applyCascadePolicy(sec, failedDep);
        continue;
      }
      // 发起前把上游最新输出同步并入 inputsRef（state 版 sectionInputs 要到
      // 下一渲染才可见，直接读会拿到旧入参——「同态新数据」验收的关键）。
      const merged: Record<string, unknown> = { ...(inputsRef.current[sec.key] || {}) };
      for (const dep of sec.refreshOn) {
        const depData = (results[dep] as { data?: Record<string, unknown> } | null | undefined)
          ?.data;
        if (depData) Object.assign(merged, depData);
      }
      inputsRef.current = { ...inputsRef.current, [sec.key]: merged };
      setSectionInputs((prev) => ({ ...prev, [sec.key]: merged }));
      runRef.current(sec).catch(() => undefined);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cascadeSignal, running]);

  // 上游输出同名字段合并进下游输入（按实际差异返回，避免无谓重渲染）
  useEffect(() => {
    setSectionInputs((prev) => {
      const next = { ...prev };
      let changed = false;
      for (const sec of sections) {
        if (!sec.refreshOn?.length) continue;
        const merged: Record<string, unknown> = { ...(prev[sec.key] || {}) };
        for (const dep of sec.refreshOn) {
          const depData = (results[dep] as { data?: Record<string, unknown> } | null | undefined)
            ?.data;
          if (depData) Object.assign(merged, depData);
        }
        if (JSON.stringify(next[sec.key]) !== JSON.stringify(merged)) {
          next[sec.key] = merged;
          changed = true;
        }
      }
      return changed ? next : prev;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cascadeSignal]);

  /** D3 翻页/改每页条数：先同步写入分页状态（run 同帧读取），
   * 再按新 page/pageSize 重跑区块——筛选与列设置不变，仅分页参数变化。 */
  const handleTablePageChange = useCallback((sec: T, page: number, size: number) => {
    const cur = tablePagesRef.current[sec.key];
    if (!cur) return;
    const next = { ...cur, page: Math.max(1, page), pageSize: Math.max(1, size) };
    tablePagesRef.current = { ...tablePagesRef.current, [sec.key]: next };
    setTablePages(tablePagesRef.current);
    runRef.current(sec).catch(() => undefined);
  }, []);

  /** 表格分页 UI：enabled 时服务端分页（current/pageSize 受控、total 已落账），
   * 否则保持客户端分页（不分页请求，避免全量拉取被误判为只有一页）。 */
  const tablePaginationProps = useCallback(
    (sec: T): TablePaginationProps => {
      const cfg = tablePaginationOf(sec);
      if (!cfg.enabled) return { pageSize: 10, showSizeChanger: false };
      const st = tablePages[sec.key];
      return {
        current: st?.page ?? 1,
        pageSize: st?.pageSize ?? cfg.defaultSize ?? TABLE_DEFAULT_SIZE,
        total: st?.total ?? 0,
        showSizeChanger: true,
        showQuickJumper: true,
        pageSizeOptions: (cfg.pageSizes ?? TABLE_DEFAULT_PAGE_SIZES).map(String),
        showTotal: (total: number) =>
          intl.formatMessage(
            {
              id: 'component.pageRenderer.composite.pagination.total',
              defaultMessage: '共 {total} 条',
            },
            { total },
          ),
        onChange: (page: number, size: number) => handleTablePageChange(sec, page, size),
      };
    },
    [handleTablePageChange, intl, tablePages],
  );

  /** 危险操作确认（D2）：确认框内展示即将提交/执行的完整内容——参数按字段
   * 标题与枚举文案渲染（不显示原始字段 key），链步骤逐行列目标；确认后执行。 */
  const confirmDanger = useCallback(
    (opts: {
      label: string;
      entries?: ConfirmEntry[];
      steps?: ConfirmContentModel['steps'];
      onOk: () => void;
    }) => {
      modal.confirm({
        title: intl.formatMessage(
          {
            id: 'component.pageRenderer.composite.confirmDangerTitle',
            defaultMessage: `确认执行「${opts.label}」`,
          },
          { label: opts.label },
        ),
        icon: <ExclamationCircleOutlined />,
        okButtonProps: { danger: true },
        content: renderConfirmContent({ entries: opts.entries, steps: opts.steps }),
        onOk: opts.onOk,
      });
    },
    [modal, intl],
  );

  /** 动作参数表达式求值（预览可注入自定义求值空间）。 */
  const resolveParams = useCallback(
    (params: Record<string, string> | undefined, ctx?: Record<string, unknown>) =>
      resolveStepParams(params, paramsSpace ? paramsSpace() : resultsRef.current, ctx) as Record<
        string,
        unknown
      >,
    [paramsSpace],
  );

  /** 目标（区块 key / 弹窗分组名）的表单展示规格：确认框据此按字段标题、
   * 枚举文案渲染待提交的参数。 */
  const formSpecOfTarget = useCallback(
    (target: string): FormPresentationSpec | undefined => {
      const hits = sectionsRef.current.filter(
        (s) => s.key === target || s.group === target || s.bindingId === target,
      );
      return (hits.find((s) => s.form) ?? hits[0])?.form;
    },
    [sectionsRef],
  );

  /** 打开弹窗：目标可以是区块 key 或弹窗分组名（Group）。
   * 危险操作先确认（确认内容 = 预填参数 + 将执行的动作链步骤），确认后开弹窗并
   * 执行动作链；非危险直接开弹窗并执行动作链。 */
  const openDialog = useCallback(
    (opts: {
      target: string;
      params: Record<string, unknown>;
      danger?: boolean;
      label?: string;
      steps?: ConfirmContentModel['steps'];
      afterOpen?: () => void;
    }) => {
      setSectionInputs((prev) => ({ ...prev, [opts.target]: opts.params }));
      const open = () => {
        setDialogKey(opts.target);
        opts.afterOpen?.();
      };
      if (opts.danger && opts.label) {
        confirmDanger({
          label: opts.label,
          entries: describeConfirmEntries(formSpecOfTarget(opts.target), opts.params),
          steps: opts.steps,
          onOk: open,
        });
        return;
      }
      open();
    },
    [confirmDanger, formSpecOfTarget],
  );

  const resolveTarget = useCallback(
    (target: string): T | undefined => {
      if (resolveSection) return resolveSection(target);
      return sectionsRef.current.find(
        (x) => x.key === target || x.group === target || x.bindingId === target,
      );
    },
    [resolveSection],
  );

  /** 动作链执行：run/refresh（params 来源解析）/openModal/closeModal/navigate/showMessage。 */
  const runChain = useCallback(
    async (chain: RuntimeStep[] | undefined, ctx?: Record<string, unknown>): Promise<void> => {
      for (const step of chain ?? []) {
        if (step.kind === 'closeModal') {
          setDialogKey(null);
          continue;
        }
        if (step.kind === 'navigate') {
          const url = step.params?.url;
          if (url) window.open(url, '_blank');
          continue;
        }
        if (step.kind === 'showMessage') {
          message.info(step.params?.message ?? '');
          continue;
        }
        if (step.kind === 'openModal' && step.target) {
          // 链内打开弹窗（如「关闭当前 → 打开下一个」多步向导）：params 按表达式
          // 求值为弹窗预填初值——缺失此分支会让 openModal 链步骤发布后静默丢弃。
          openDialog({
            target: step.target,
            params: resolveParams(step.params, ctx),
          });
          continue;
        }
        if (step.kind === 'runBinding' || step.kind === 'refreshNode') {
          const target = resolveTarget(step.target);
          if (!target) continue;
          // B2：链步骤顺序执行且失败中止——执行类步骤失败即终止整条链，不再
          // 吞错继续跑后续步骤（避免「操作失败却跳转成功页」这类假成功）。
          try {
            await runRef.current(target, resolveParams(step.params, ctx));
          } catch {
            return;
          }
        }
      }
    },
    [message, openDialog, resolveParams, resolveTarget],
  );

  /** 区块事件执行：events 里找事件名 → 主动作 + 链。 */
  const fireEvent = useCallback(
    (sec: T, eventName: string, ctx?: Record<string, unknown>) => {
      const binding = (sec.events ?? []).find((e) => e.event === eventName);
      if (!binding) return;
      void runChain([{ ...binding.action }, ...(binding.chain ?? [])], ctx);
    },
    [runChain],
  );

  /** 清空区块执行反馈（改输入或关弹窗时旧反馈失效）。 */
  const clearExecFeedbackOf = useCallback((key: string) => {
    setExecFailures((prev) => (prev[key] ? { ...prev, [key]: null } : prev));
    setExecPending((prev) => (prev[key] ? { ...prev, [key]: undefined } : prev));
  }, []);

  /** D4：表单区块提交统一出口。ok=true 仅表示业务执行成功（同步结果），
   * 调用方据此关弹窗；审批待处理 / 任务已提交 / 执行失败均保持弹窗开启，
   * 把待处理入口与字段级错误留在用户面前。 */
  const submitForm = useCallback(
    async (sec: T, values: FormValues): Promise<SubmitFormOutcome> => {
      const plain = values as Record<string, unknown>;
      clearExecFeedbackOf(sec.key);
      // 无表单区块（降级执行按钮）不落页面状态：与既有提交路径行为对齐
      if (sec.form) onPageStateMerge?.(sec.key, { ...plain, values: plain });
      try {
        const r = await runRef.current(sec, plain);
        // 无结果（preview / 旧响应拒收）与结果体自带 error 标记都不算成功：
        // 后者若不门控，执行器返回错误体（未抛异常）会误触发 success 级联。
        if (!r || isErrorExecResult(r)) return { ok: false };
        const outcome = describeExecOutcome(r);
        if (outcome === 'approval') {
          setExecPending((prev) => ({ ...prev, [sec.key]: { outcome, id: r?.approvalId } }));
          message.info(
            intl.formatMessage({
              id: 'component.pageRenderer.composite.submittedForApproval',
              defaultMessage: '操作已提交审批，等待审批通过后生效',
            }),
          );
          return { ok: false };
        }
        if (outcome === 'task') {
          setExecPending((prev) => ({ ...prev, [sec.key]: { outcome, id: r?.taskId } }));
          message.success(
            intl.formatMessage({
              id: 'component.pageRenderer.composite.taskSubmitted',
              defaultMessage: '任务已提交，正在后台执行',
            }),
          );
          return { ok: false };
        }
        setExecPending((prev) => ({ ...prev, [sec.key]: undefined }));
        fireEvent(sec, 'success');
        return { ok: true, replayed: r.replayed === true };
      } catch (e) {
        setExecFailures((prev) => ({ ...prev, [sec.key]: describeExecFailure(e) }));
        // 上抛：弹窗提交路径沿用既有失败 toast 分支（含 T8 unbound 指引文案）
        throw e;
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [message, intl, onPageStateMerge, fireEvent, clearExecFeedbackOf],
  );

  /** V5：表单当前值缓冲写入（防抖后进 results[key].values，{{var.values.x}}）。 */
  const mergeFormValues = useCallback(
    (key: string, values: Record<string, unknown>) => {
      valuesMergeRef.current[key] = values;
      scheduleValuesFlush();
    },
    [scheduleValuesFlush],
  );

  /** 常量表单值缓冲写入（防抖后进 results[key]，驱动 refreshOn 联动）。 */
  const mergeStaticValues = useCallback(
    (key: string, values: Record<string, unknown>) => {
      staticMergeRef.current[key] = { data: values, values };
      scheduleStaticFlush();
    },
    [scheduleStaticFlush],
  );

  /** 行字段映射：params 目标参数名 → 本行字段名。
   * V5 兼容两种形态：纯字段名（V3 编辑器产出）与 `row.字段`（{{row.x}} 编译产物）。 */
  const mapRowParams = useCallback(
    (mapping: Record<string, string> | undefined, row: Record<string, unknown>) => {
      const out: Record<string, unknown> = {};
      for (const [param, rowField] of Object.entries(mapping || {})) {
        const field = rowField.startsWith('row.') ? rowField.slice('row.'.length) : rowField;
        out[param] = row[field];
      }
      return out;
    },
    [],
  );

  return {
    results,
    resultsRef,
    running,
    sectionInputs,
    revisions,
    execFailures,
    execPending,
    tablePages,
    dialogKey,
    setDialogKey,
    run,
    runRef,
    resolveTarget,
    resolveParams,
    handleTablePageChange,
    tablePaginationProps,
    confirmDanger,
    formSpecOfTarget,
    openDialog,
    runChain,
    fireEvent,
    setSelectionState,
    mergeFormValues,
    mergeStaticValues,
    mapRowParams,
    clearExecFeedbackOf,
    submitForm,
  };
}
