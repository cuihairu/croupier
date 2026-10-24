import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Modal,
  Row,
  Space,
  Table,
  Tabs,
  Tag,
} from 'antd';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { ExclamationCircleOutlined } from '@ant-design/icons';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import ExecutorUnboundAlert, {
  EXECUTOR_UNBOUND_CODE,
  executeErrorToastText,
} from './ExecutorUnboundAlert';
import { localizedText } from '@/utils/localizedText';
import { extractApiErrorCode } from '@/utils/apiError';
import { resolveStepParams } from './runtime';
import { sectionVisible } from './sectionCondition';
import { confirmFieldTitle, describeConfirmEntries } from './confirmContent';
import type { ConfirmEntry } from './confirmContent';
import { describeExecFailure, describeExecOutcome, isErrorExecResult } from './execFeedback';
import type { ExecFailure, ExecOutcome } from './execFeedback';
import type {
  PageExecuteFn,
  PageExecutionResult,
  CompositeSection,
  PageFunctionBinding,
  FormValues,
  FormPresentationSpec,
  PaginationSpec,
} from '@/types/dashboard';

/** 危险动作确认框内容：提交参数条目 + 待执行的动作链步骤。 */
interface ConfirmContentModel {
  entries?: ConfirmEntry[];
  steps?: Array<{ kind: string; target: string }>;
}

/** 确认框内容：参数条目按字段标题渲染（Descriptions），链步骤逐行列目标。 */
function renderConfirmContent(model: ConfirmContentModel): React.ReactNode {
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
interface TablePageState {
  page: number;
  pageSize: number;
  inputFingerprint: string;
  total: number;
}

/** Table pagination 配置（服务端分页受控形态 + 客户端分页缺省形态）。 */
interface TablePaginationProps {
  current?: number;
  pageSize?: number;
  total?: number;
  showSizeChanger?: boolean;
  showQuickJumper?: boolean;
  pageSizeOptions?: string[];
  showTotal?: (total: number) => React.ReactNode;
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
function tablePaginationOf(sec: CompositeSection): PaginationSpec {
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

/** 组合页渲染器：区块栅格布局；autoRun 加载即执行；refreshOn 联动
 * 自动重跑；dialog 形态区块由行操作/工具栏按钮触发弹窗打开（行字段
 * 映射进表单参数）；操作成功后按 onSuccessRefresh 刷新目标区块。 */
export const CompositeRenderer: React.FC<{
  sections: CompositeSection[];
  bindings: PageFunctionBinding[];
  onExecute: PageExecuteFn;
  preview: boolean;
  /** 常量表单/表单提交值并入 page_state（显式参数映射的数据来源）。
   * V5 mode='merge'：按区块键浅合并（selectedRow/data 分支共存），默认 replace。 */
  onPageStateMerge?: (
    key: string,
    values: Record<string, unknown>,
    mode?: 'replace' | 'merge',
  ) => void;
}> = ({ sections, bindings, onExecute, preview, onPageStateMerge }) => {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  // V5 §7.1：每区块运行时状态 = data（函数输出）+ selectedRow/selectedRows（表格
  // 选中）+ values（表单当前值）。键为区块 key（= 变量名），表达式求值直接消费。
  const [results, setResults] = useState<
    Record<
      string,
      | PageExecutionResult
      | {
          data?: Record<string, unknown>;
          selectedRow?: Record<string, unknown>;
          selectedRows?: Record<string, unknown>[];
          values?: Record<string, unknown>;
        }
      // U9 失败标记：runSection catch 写入，下游 cascadePolicy 判定依据；
      // errorCode 为 T8 稳定码（executor_unbound → 区块级结构化空态）
      | { error: string; errorCode?: string }
      | null
    >
  >({});
  const [running, setRunning] = useState<Record<string, boolean>>({});
  const [sectionInputs, setSectionInputs] = useState<Record<string, Record<string, unknown>>>({});
  // B2：结果修订号——每个区块每次产出新结果（成功或失败）+1。仅由 runSection
  // 写入，选区/常量等辅助写入不进修订号。级联改由修订号驱动：上游同态新数据
  // 也改变修订号，下游随之重跑（旧 key 集合信号看不到「成功→成功·新数据」）。
  const [revisions, setRevisions] = useState<Record<string, number>>({});
  // B2：请求序号（ref，同步占用）——并发同区块重跑时仅最新一次可落账（旧
  // 响应拒收）。与结果修订号分离：修订号在结果真正落地时才前进，信号因此
  // 只反映已落账的结果，不会被在飞请求预先污染。
  const execSeqRef = useRef<Record<string, number>>({});
  // B2：级联联动账本——每区块已消费的上游修订号 + 上次失败态（同态连续失败
  // 不重复提示）。首次看到的上游结果照常联动（refreshOn 语义保持）。
  const cascadeSeenRef = useRef<Record<string, { rev: number; failed: boolean }>>({});
  // D3：表格服务端分页状态（ref 为请求真值——runSection 在同一事件帧内读取，
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
    valuesTimerRef.current = setTimeout(() => {
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
    }, 300);
  }, []);
  /** V5 表格选中行状态写入（selectedRow/selectedRows；不触发 refreshOn 自动重跑）。
   * 同步刷新 resultsRef——同一事件帧内的动作链求值（runChain）立即可见；
   * 并以 merge 模式并入 page_state（服务端 inputAssignment /selectedRow/* 求值）。 */
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
    staticTimerRef.current = setTimeout(() => {
      staticTimerRef.current = null;
      setResults((prev) => ({ ...prev, ...staticMergeRef.current }));
      for (const [key, v] of Object.entries(staticMergeRef.current)) {
        // 双形态快照：扁平值兼容遗留 /字段 路径 + values 包装供 {{var.values.x}}
        onPageStateMerge?.(key, { ...v.data, values: v.data });
      }
    }, 400);
  }, [onPageStateMerge]);
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

  const runSection = useCallback(
    async (sec: CompositeSection, overrides?: Record<string, unknown>) => {
      if (preview) return null;
      const baseInputs = { ...(inputsRef.current[sec.key] || {}), ...(overrides || {}) };
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
        const result = await onExecute(sec.bindingId, { form: merged as never });
        if (!isCurrent()) return null; // 旧响应拒收
        setRevisions((prev) => ({ ...prev, [sec.key]: (prev[sec.key] ?? 0) + 1 }));
        setResults((prev) => ({ ...prev, [sec.key]: result || null }));
        // V5：函数输出并入 page_state.data（{{var.data.x}} 服务端求值来源）
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
            if (t) runSection(t).catch(() => undefined);
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
    [onExecute, onPageStateMerge, preview],
  );

  const runSectionRef = useRef(runSection);
  runSectionRef.current = runSection;

  // autoRun：加载即执行（inline 区块）
  useEffect(() => {
    for (const sec of sections) {
      if (sec.autoRun && sec.display !== 'dialog') {
        runSection(sec).catch(() => undefined);
      }
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
    (sec: CompositeSection, failedDep: string) => {
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
      runSection(sec).catch(() => undefined);
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

  const resultFor = useCallback(
    (sec: CompositeSection): Record<string, unknown> | undefined => {
      const r = results[sec.key];
      return (r as { data?: Record<string, unknown> } | null | undefined)?.data;
    },
    [results],
  );

  /** D3 翻页/改每页条数：先同步写入分页状态（runSection 同帧读取），
   * 再按新 page/pageSize 重跑区块——筛选与列设置不变，仅分页参数变化。 */
  const handleTablePageChange = useCallback((sec: CompositeSection, page: number, size: number) => {
    const cur = tablePagesRef.current[sec.key];
    if (!cur) return;
    const next = { ...cur, page: Math.max(1, page), pageSize: Math.max(1, size) };
    tablePagesRef.current = { ...tablePagesRef.current, [sec.key]: next };
    setTablePages(tablePagesRef.current);
    runSectionRef.current(sec).catch(() => undefined);
  }, []);

  /** 表格分页 UI：enabled 时服务端分页（current/pageSize 受控、total 已落账），
   * 否则保持客户端分页（不分页请求，避免全量拉取被误判为只有一页）。 */
  const tablePaginationProps = useCallback(
    (sec: CompositeSection): TablePaginationProps => {
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

  /** 目标（区块 key / 弹窗分组名）的表单展示规格：确认框据此按字段标题、
   * 枚举文案渲染待提交的参数。 */
  const formSpecOfTarget = useCallback(
    (target: string): FormPresentationSpec | undefined => {
      const hits = sectionsRef.current.filter(
        (s) => s.key === target || s.group === target || s.bindingId === target,
      );
      return (hits.find((s) => s.form) ?? hits[0])?.form as FormPresentationSpec | undefined;
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

  /** 动作链执行：run/refresh（params 来源解析）/openModal/closeModal/navigate/showMessage。 */
  const runChain = useCallback(
    async (
      chain: Array<{ kind: string; target: string; params?: Record<string, string> }> | undefined,
      ctx?: Record<string, unknown>,
    ): Promise<void> => {
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
            params: (resolveStepParams(step.params, resultsRef.current, ctx) ?? {}) as Record<
              string,
              unknown
            >,
          });
          continue;
        }
        if (step.kind === 'runBinding' || step.kind === 'refreshNode') {
          const target = sectionsRef.current.find(
            (x) => x.key === step.target || x.group === step.target,
          );
          if (!target) continue;
          // B2：链步骤顺序执行且失败中止——执行类步骤失败即终止整条链，不再
          // 吞错继续跑后续步骤（避免「操作失败却跳转成功页」这类假成功）。
          try {
            await runSectionRef.current(
              target,
              resolveStepParams(step.params, resultsRef.current, ctx) as never,
            );
          } catch {
            return;
          }
        }
      }
    },
    [message, openDialog],
  );

  /** 区块事件执行：events 里找事件名 → 主动作 + 链。 */
  const fireEvent = useCallback(
    (sec: CompositeSection, eventName: string, ctx?: Record<string, unknown>) => {
      const binding = (sec.events ?? []).find((e) => e.event === eventName);
      if (!binding) return;
      runChain([{ ...binding.action }, ...(binding.chain ?? [])], ctx);
    },
    [runChain],
  );

  /** 清空区块执行反馈（改输入或关弹窗时旧反馈失效）。 */
  const clearExecFeedbackOf = (key: string) => {
    setExecFailures((prev) => (prev[key] ? { ...prev, [key]: null } : prev));
    setExecPending((prev) => (prev[key] ? { ...prev, [key]: undefined } : prev));
  };

  /** D4：表单区块提交统一出口。返回 true 仅表示业务执行成功（同步结果），
   * 调用方据此关弹窗；审批待处理 / 任务已提交 / 执行失败均保持弹窗开启，
   * 把待处理入口与字段级错误留在用户面前。 */
  const submitFormSection = useCallback(
    async (sec: CompositeSection, values: FormValues): Promise<boolean> => {
      const plain = values as Record<string, unknown>;
      clearExecFeedbackOf(sec.key);
      // 无表单区块（降级执行按钮）不落页面状态：与既有提交路径行为对齐
      if (sec.form) onPageStateMerge?.(sec.key, { ...plain, values: plain });
      try {
        const r = await runSectionRef.current(sec, plain);
        // 无结果（preview / 旧响应拒收）与结果体自带 error 标记都不算成功：
        // 后者若不门控，执行器返回错误体（未抛异常）会误触发 success 级联。
        if (!r || isErrorExecResult(r)) return false;
        const outcome = describeExecOutcome(r);
        if (outcome === 'approval') {
          setExecPending((prev) => ({ ...prev, [sec.key]: { outcome, id: r?.approvalId } }));
          message.info(
            intl.formatMessage({
              id: 'component.pageRenderer.composite.submittedForApproval',
              defaultMessage: '操作已提交审批，等待审批通过后生效',
            }),
          );
          return false;
        }
        if (outcome === 'task') {
          setExecPending((prev) => ({ ...prev, [sec.key]: { outcome, id: r?.taskId } }));
          message.success(
            intl.formatMessage({
              id: 'component.pageRenderer.composite.taskSubmitted',
              defaultMessage: '任务已提交，正在后台执行',
            }),
          );
          return false;
        }
        setExecPending((prev) => ({ ...prev, [sec.key]: undefined }));
        fireEvent(sec, 'success');
        return true;
      } catch (e) {
        setExecFailures((prev) => ({ ...prev, [sec.key]: describeExecFailure(e) }));
        // 上抛：弹窗提交路径沿用既有失败 toast 分支（含 T8 unbound 指引文案）
        throw e;
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [message, intl, onPageStateMerge, fireEvent],
  );

  /** D4：区块执行反馈条——失败给稳定错误码 + 字段级定位，待处理给审批 /
   * 任务入口。字段名按表单展示层 label 渲染（与 D2 确认框同源），不显示
   * 原始 key。 */
  const renderExecFeedback = (sec: CompositeSection): React.ReactNode => {
    const failure = execFailures[sec.key];
    const pending = execPending[sec.key];
    const fieldTitle = (key: string) =>
      confirmFieldTitle(
        sec.form?.fields?.find((f) => f.key === key),
        key,
      );
    return (
      <>
        {failure ? (
          <Alert
            type="error"
            showIcon
            title={intl.formatMessage(
              {
                id: 'component.pageRenderer.composite.execFailed',
                defaultMessage: '执行失败{codeSuffix}',
              },
              { codeSuffix: failure.code ? `（${failure.code}）` : '' },
            )}
            description={
              <>
                {failure.message ? <div>{failure.message}</div> : null}
                {failure.fieldErrors.length > 0 ? (
                  <ul style={{ margin: '4px 0 0', paddingLeft: 20 }}>
                    {failure.fieldErrors.map((fe, i) => (
                      <li key={i}>
                        {fe.field ? `${fieldTitle(fe.field)}：${fe.message}` : fe.message}
                      </li>
                    ))}
                  </ul>
                ) : null}
              </>
            }
          />
        ) : null}
        {pending ? (
          pending.outcome === 'approval' ? (
            <Alert
              type="info"
              showIcon
              title={intl.formatMessage({
                id: 'component.pageRenderer.composite.pendingApproval',
                defaultMessage: '已提交审批，等待审批通过后生效',
              })}
              description={
                <Space size={8} wrap>
                  {pending.id ? <Tag color="blue">审批单 {pending.id}</Tag> : null}
                  <Button
                    type="link"
                    size="small"
                    style={{ padding: 0 }}
                    onClick={() => history.push('/approvals')}
                  >
                    {intl.formatMessage({
                      id: 'component.pageRenderer.composite.goToApprovals',
                      defaultMessage: '前往审批中心',
                    })}
                  </Button>
                </Space>
              }
            />
          ) : (
            <Alert
              type="warning"
              showIcon
              title={intl.formatMessage({
                id: 'component.pageRenderer.composite.pendingTask',
                defaultMessage: '任务已提交，正在后台执行',
              })}
              description={
                <Space size={8} wrap>
                  {pending.id ? <Tag color="processing">任务 {pending.id}</Tag> : null}
                  <span>
                    {intl.formatMessage({
                      id: 'component.pageRenderer.composite.pendingTaskHint',
                      defaultMessage: '任务执行完成后业务数据才会变更，请避免重复提交',
                    })}
                  </span>
                </Space>
              }
            />
          )
        ) : null}
      </>
    );
  };

  /** 行字段映射：params 目标参数名 → 本行字段名。
   * V5 兼容两种形态：纯字段名（V3 编辑器产出）与 `row.字段`（{{row.x}} 编译产物）。 */
  const mapRowParams = (
    mapping: Record<string, string> | undefined,
    row: Record<string, unknown>,
  ): Record<string, unknown> => {
    const out: Record<string, unknown> = {};
    for (const [param, rowField] of Object.entries(mapping || {})) {
      const field = rowField.startsWith('row.') ? rowField.slice('row.'.length) : rowField;
      out[param] = row[field];
    }
    return out;
  };

  const inline = sections.filter(
    (s) => s.display !== 'dialog' && s.display !== 'tab' && s.display !== 'card',
  );
  const tabbed = sections.filter((s) => s.display === 'tab');
  // #94 卡片分组：display='card' 区块按 group 聚合渲染进同一 Card（整行）
  const carded = sections.filter((s) => s.display === 'card');
  const dialogs = sections.filter((s) => s.display === 'dialog');
  /** dialogKey 命中的弹窗分组（target 可为 group 名或区块 key）。 */
  const groupOf = (sec: CompositeSection): string => sec.group ?? sec.key ?? sec.bindingId;
  const activeDialogs = dialogKey
    ? dialogs.filter(
        (d) => groupOf(d) === dialogKey || d.key === dialogKey || d.bindingId === dialogKey,
      )
    : [];

  /** 表格区块的工具栏按钮：编辑器把按钮编译到 table section 的 toolbar
   * （compiler compileButton），渲染端必须在表格卡片头部消费——否则按钮
   * 发布后消失。行为与 view=toolbar 独立区块分支一致。 */
  const toolbarButtonsOf = (sec: CompositeSection): React.ReactNode =>
    (sec.toolbar?.actions || []).map((act, i) => (
      <Button
        key={i}
        size="small"
        danger={act.danger}
        onClick={() => {
          if (act.targetSection) {
            openDialog({
              target: act.targetSection,
              params: { ...(act.params || {}) },
              danger: act.danger,
              label: localizedText(act.label, 'zh-CN'),
              steps: act.chain,
              afterOpen: () => runChain(act.chain),
            });
          } else if (act.danger) {
            // D2：无目标弹窗的危险操作（纯动作链）同样先确认，确认内容列链步骤
            confirmDanger({
              label: localizedText(act.label, 'zh-CN'),
              steps: act.chain,
              onOk: () => runChain(act.chain),
            });
          } else {
            runChain(act.chain);
          }
        }}
      >
        {localizedText(act.label, 'zh-CN')}
      </Button>
    ));

  /** 单区块渲染（V2 从 inline.map 抽出复用：栅格区块与页签页内整行堆叠
   * 共用同一卡片渲染——span 由调用方决定）。 */
  const bindingById = useMemo(() => {
    const map: Record<string, PageFunctionBinding> = {};
    for (const binding of bindings) map[binding.id] = binding;
    return map;
  }, [bindings]);
  const renderSection = (sec: CompositeSection): React.ReactNode => (
    <Card
      size="small"
      title={localizedText(sec.title, 'zh-CN', sec.key)}
      loading={running[sec.key] || false}
      extra={
        sec.view === 'table' && (sec.toolbar?.actions?.length ?? 0) > 0 ? (
          <Space size={4}>
            {toolbarButtonsOf(sec)}
            {!sec.autoRun ? (
              <Button size="small" onClick={() => runSection(sec).catch(() => undefined)}>
                <FormattedMessage
                  id="component.pageRenderer.composite.executeButton"
                  defaultMessage="执行"
                />
              </Button>
            ) : null}
          </Space>
        ) : sec.view !== 'actions' && sec.view !== 'toolbar' && !sec.autoRun ? (
          <Button size="small" onClick={() => runSection(sec).catch(() => undefined)}>
            <FormattedMessage
              id="component.pageRenderer.composite.executeButton"
              defaultMessage="执行"
            />
          </Button>
        ) : null
      }
    >
      {/* T8：unbound 阻断（409 executor_unbound）渲染区块级空态 + 去绑定
          入口；表格/字段区块保持空数据，无伪数据兜底 */}
      {isExecutorUnboundResult(results[sec.key]) ? (
        <ExecutorUnboundAlert
          functionId={sec.bindingId ? bindingById[sec.bindingId]?.functionId : undefined}
          style={{ marginBottom: 12 }}
        />
      ) : null}
      {sec.view === 'table' ? (
        <Table
          size="small"
          rowKey={(_, i) => String(i)}
          onRow={(record) => ({
            onClick: () => fireEvent(sec, 'rowClick', record as Record<string, unknown>),
          })}
          rowSelection={{
            type: 'radio',
            onChange: (_keys, rows) => {
              // V5：选中行写入运行时状态（{{var.selectedRow.x}} 数据来源），
              // 仅在绑定 rowSelected 事件时同步触发事件。
              setSelectionState(sec.key, rows as Record<string, unknown>[]);
              if (rows[0] && (sec.events ?? []).some((e) => e.event === 'rowSelected')) {
                fireEvent(sec, 'rowSelected', rows[0] as Record<string, unknown>);
              }
            },
          }}
          columns={[
            ...(sec.table?.columns || []).map((c) => ({
              title: localizedText(c.title, 'zh-CN', c.key),
              dataIndex: c.key,
              ellipsis: true,
            })),
            ...(sec.table?.rowActions?.length
              ? [
                  {
                    title: intl.formatMessage({
                      id: 'component.pageRenderer.composite.rowActionsColumn',
                      defaultMessage: '操作',
                    }),
                    key: '__row_actions',
                    render: (_: unknown, row: Record<string, unknown>) => (
                      <Space size={4}>
                        {sec.table!.rowActions!.map((ra, i) => (
                          <Button
                            key={i}
                            size="small"
                            type="link"
                            danger={ra.danger}
                            onClick={() => {
                              if (ra.targetSection) {
                                openDialog({
                                  target: ra.targetSection,
                                  params: mapRowParams(ra.params, row),
                                  danger: ra.danger,
                                  label: localizedText(ra.label, 'zh-CN'),
                                  steps: ra.chain,
                                  afterOpen: () => runChain(ra.chain, row),
                                });
                              } else if (ra.danger) {
                                // D2：行级危险操作无目标弹窗时先确认（ctx 为当前行）
                                confirmDanger({
                                  label: localizedText(ra.label, 'zh-CN'),
                                  steps: ra.chain,
                                  onOk: () => runChain(ra.chain, row),
                                });
                              } else {
                                runChain(ra.chain, row);
                              }
                            }}
                          >
                            {localizedText(ra.label, 'zh-CN')}
                          </Button>
                        ))}
                      </Space>
                    ),
                  },
                ]
              : []),
          ]}
          dataSource={
            Array.isArray(resultFor(sec)?.items)
              ? (resultFor(sec)?.items as Record<string, unknown>[])
              : []
          }
          pagination={tablePaginationProps(sec)}
        />
      ) : sec.view === 'fields' ? (
        <div onClick={() => fireEvent(sec, 'click')}>
          <Descriptions size="small" column={2}>
            {(resultFor(sec) ? Object.entries(resultFor(sec) as Record<string, unknown>) : []).map(
              ([k, v]) => (
                <Descriptions.Item key={k} label={k}>
                  {String(v ?? '-')}
                </Descriptions.Item>
              ),
            )}
          </Descriptions>
        </div>
      ) : sec.view === 'toolbar' ? (
        <Space wrap>
          {(sec.toolbar?.actions || []).map((act, i) => (
            <Button
              key={i}
              size="small"
              danger={act.danger}
              onClick={() => {
                if (act.targetSection) {
                  openDialog({
                    target: act.targetSection,
                    params: { ...(act.params || {}) },
                    danger: act.danger,
                    label: localizedText(act.label, 'zh-CN'),
                    steps: act.chain,
                    afterOpen: () => runChain(act.chain),
                  });
                } else if (act.danger) {
                  // D2：纯链危险操作先确认（与表格工具栏分支同构）
                  confirmDanger({
                    label: localizedText(act.label, 'zh-CN'),
                    steps: act.chain,
                    onOk: () => runChain(act.chain),
                  });
                } else {
                  runChain(act.chain);
                }
              }}
            >
              {localizedText(act.label, 'zh-CN')}
            </Button>
          ))}
        </Space>
      ) : sec.static && sectionHasForm(sec) ? (
        // 常量表单：不执行绑定，值并入页面状态驱动 refreshOn 联动
        //（防抖合并，避免文本输入逐键触发下游重跑）
        <SchemaFormRenderer
          spec={sec.form!}
          initialValues={(sectionInputs[sec.key] || {}) as FormValues}
          hideSubmit
          onValuesChange={(_, values) => {
            const v = values as Record<string, unknown>;
            staticMergeRef.current[sec.key] = { data: v, values: v };
            scheduleStaticFlush();
          }}
        />
      ) : sectionHasForm(sec) ? (
        <>
          {renderExecFeedback(sec)}
          <SchemaFormRenderer
            spec={sec.form!}
            initialValues={(sectionInputs[sec.key] || {}) as FormValues}
            disabled={running[sec.key] || false}
            onValuesChange={(_, values) => {
              // V5：fnForm 当前值防抖写入 results[key].values（{{var.values.x}}）
              valuesMergeRef.current[sec.key] = values as Record<string, unknown>;
              scheduleValuesFlush();
              // D4：输入变化后旧失败反馈失效
              clearExecFeedbackOf(sec.key);
            }}
            onFinish={(values) => submitFormSection(sec, values).catch(() => undefined)}
          />
        </>
      ) : (
        <>
          {renderExecFeedback(sec)}
          <Button
            type="primary"
            loading={running[sec.key] || false}
            onClick={() =>
              submitFormSection(sec, (sectionInputs[sec.key] || {}) as FormValues).catch(
                () => undefined,
              )
            }
          >
            {localizedText(sec.title, 'zh-CN', sec.key)}
          </Button>
        </>
      )}
    </Card>
  );

  /** 页签聚合（V2）：同 group → 一个 Tabs（整行）；组内按 tab 标签 → 页，
   * 页内区块整行堆叠（编辑器页签页的发布形态）。 */
  const tabGroups: Array<{
    group: string;
    pages: Array<{ label: string; sections: CompositeSection[] }>;
  }> = [];
  for (const sec of tabbed) {
    const group = sec.group ?? sec.key ?? sec.bindingId;
    let g = tabGroups.find((x) => x.group === group);
    if (!g) {
      g = { group, pages: [] };
      tabGroups.push(g);
    }
    const label =
      localizedText(sec.tab, 'zh-CN', '') ||
      intl.formatMessage(
        { id: 'component.pageRenderer.composite.tabFallback', defaultMessage: '页签 {n}' },
        { n: g.pages.length + 1 },
      );
    let page = g.pages.find((p) => p.label === label);
    if (!page) {
      page = { label, sections: [] };
      g.pages.push(page);
    }
    page.sections.push(sec);
  }

  /** U10 区块级条件显示：visibleWhen 按页面状态（results）求值，false
   * 的 inline/tab 区块不渲染（执行不变——autoRun/refreshOn 照常跑）。
   * dialog 区块不参与条件显隐（弹窗由动作显式触发）。 */
  const isVisible = (sec: CompositeSection): boolean => sectionVisible(sec.visibleWhen, results);

  /** #94 卡片分组聚合：同 group 的 card 区块 → 一个 Card（整行），
   * 组内区块垂直堆叠（标题缺省回退组名）。 */
  const cardGroups: Array<{ group: string; title: string; sections: CompositeSection[] }> = [];
  for (const sec of carded) {
    const group = sec.group ?? sec.key ?? sec.bindingId;
    let g = cardGroups.find((x) => x.group === group);
    if (!g) {
      g = {
        group,
        title: localizedText(sec.cardTitle, 'zh-CN', group),
        sections: [],
      };
      cardGroups.push(g);
    }
    g.sections.push(sec);
  }

  return (
    <>
      <Row gutter={[12, 12]}>
        {inline.filter(isVisible).map((sec) => (
          <Col key={sec.key} span={sec.span && sec.span > 0 && sec.span <= 24 ? sec.span : 24}>
            {renderSection(sec)}
          </Col>
        ))}
        {tabGroups.map((g) => (
          <Col key={`tabs-${g.group}`} span={24}>
            <Tabs
              size="small"
              items={g.pages.map((page) => ({
                key: page.label,
                label: page.label,
                children: (
                  <Space orientation="vertical" size={12} style={{ width: '100%' }}>
                    {page.sections.filter(isVisible).map((sec) => (
                      <React.Fragment key={sec.key}>{renderSection(sec)}</React.Fragment>
                    ))}
                  </Space>
                ),
              }))}
            />
          </Col>
        ))}
        {cardGroups.map((g) => (
          <Col key={`card-${g.group}`} span={24}>
            <Card size="small" title={g.title}>
              <Space orientation="vertical" size={12} style={{ width: '100%' }}>
                {g.sections.filter(isVisible).map((sec) => (
                  <React.Fragment key={sec.key}>{renderSection(sec)}</React.Fragment>
                ))}
              </Space>
            </Card>
          </Col>
        ))}
      </Row>

      {/* 弹窗形态区块：按分组聚合渲染（一个弹窗多区块：表单提交 + 展示组件） */}
      {dialogKey && activeDialogs.length > 0 && (
        <Modal
          title={localizedText(activeDialogs[0].title, 'zh-CN', dialogKey)}
          open
          onCancel={() => {
            activeDialogs.forEach((s) => clearExecFeedbackOf(s.key));
            setDialogKey(null);
          }}
          footer={null}
          destroyOnHidden
        >
          <Space orientation="vertical" size={16} style={{ width: '100%' }}>
            {activeDialogs.map((sec) =>
              sec.view === 'form' ? (
                <DialogForm
                  key={sec.key}
                  section={sec}
                  running={running[sec.key] || false}
                  initialParams={sectionInputs[dialogKey] || {}}
                  feedback={renderExecFeedback(sec)}
                  onValuesChange={(values) => {
                    valuesMergeRef.current[sec.key] = values;
                    scheduleValuesFlush();
                    // D4：输入变化后旧失败反馈失效（弹窗保留待处理提示直至提交）
                    clearExecFeedbackOf(sec.key);
                  }}
                  onSubmit={async (values) => {
                    try {
                      // D4：审批 / 任务提交 ≠ 业务执行成功——弹窗保持开启，
                      // 待处理反馈（含审批入口）与字段级错误都留在弹窗内。
                      const succeeded = await submitFormSection(sec, values as FormValues);
                      if (!succeeded) return;
                      setDialogKey(null);
                      message.success(
                        intl.formatMessage(
                          {
                            id: 'component.pageRenderer.composite.executeSuccess',
                            defaultMessage: `${localizedText(sec.title, 'zh-CN', sec.key)} 执行成功`,
                          },
                          { title: localizedText(sec.title, 'zh-CN', sec.key) },
                        ),
                      );
                    } catch (e) {
                      // 失败保持弹窗开启（保留已填参数）；App 实例 toast 给出
                      // 明确错误反馈（全局拦截器只对 request 层错误兜底）；
                      // T8：unbound 阻断 toast 换绑定指引文案
                      const errMsg = e instanceof Error ? e.message : '';
                      message.error(
                        executeErrorToastText(
                          e,
                          errMsg ||
                            intl.formatMessage({
                              id: 'component.pageRenderer.composite.executeFailed',
                              defaultMessage: '执行失败',
                            }),
                        ),
                      );
                    }
                  }}
                />
              ) : sec.view === 'fields' ? (
                <Descriptions key={sec.key} size="small" column={1} bordered>
                  {Object.entries(resultFor(sec) ?? {})
                    .filter(([k]) => k !== 'items' && k !== 'total')
                    .slice(0, 10)
                    .map(([k, v]) => (
                      <Descriptions.Item key={k} label={k}>
                        {typeof v === 'object' ? JSON.stringify(v) : String(v ?? '-')}
                      </Descriptions.Item>
                    ))}
                </Descriptions>
              ) : sec.view === 'table' ? (
                <Table
                  key={sec.key}
                  size="small"
                  rowKey={(_, i) => String(i)}
                  pagination={false}
                  columns={(sec.table?.columns ?? []).map((c) => ({
                    title: localizedText(c.title, 'zh-CN', c.key),
                    dataIndex: c.key,
                    ellipsis: true,
                  }))}
                  dataSource={
                    Array.isArray(resultFor(sec)?.items)
                      ? (resultFor(sec)?.items as Record<string, unknown>[])
                      : []
                  }
                />
              ) : null,
            )}
          </Space>
        </Modal>
      )}
    </>
  );
};

/** 弹窗表单：复用 SchemaFormRenderer（与 Operation/Task 页同一 RJSF 运行时），
 * 保证 enum/日期/校验等控件行为与独立操作页一致。 */
const DialogForm: React.FC<{
  section: CompositeSection;
  running: boolean;
  initialParams: Record<string, unknown>;
  /** D4：执行反馈条（失败定位 + 审批/任务待处理），由调用方按区块状态渲染。 */
  feedback?: React.ReactNode;
  /** V5：当前值变化（防抖写入 values 运行时状态由调用方处理）。 */
  onValuesChange?: (values: Record<string, unknown>) => void;
  onSubmit: (values: Record<string, unknown>) => Promise<void>;
}> = ({ section, running, initialParams, feedback, onValuesChange, onSubmit }) => {
  const spec = section.form;
  const properties = spec?.jsonSchema?.properties;
  const hasFields = !!properties && typeof properties === 'object';
  if (!spec || !hasFields) {
    return (
      <>
        {feedback}
        <Button type="primary" block loading={running} onClick={() => void onSubmit({})}>
          <FormattedMessage
            id="component.pageRenderer.composite.confirmExecute"
            defaultMessage="确认执行"
          />
        </Button>
      </>
    );
  }
  return (
    <>
      {feedback}
      <SchemaFormRenderer
        spec={spec}
        initialValues={initialParams as FormValues}
        disabled={running}
        onValuesChange={(_, values) => onValuesChange?.(values as Record<string, unknown>)}
        onFinish={async (values) => {
          await onSubmit(values);
        }}
      />
    </>
  );
};

/** 区块是否带有可渲染的表单 schema（无字段时降级为执行按钮）。 */
function sectionHasForm(sec: CompositeSection): boolean {
  const properties = sec.form?.jsonSchema?.properties;
  return !!properties && typeof properties === 'object' && Object.keys(properties).length > 0;
}

/** U9：失败结果判定——runSection catch 写入的 {error} 标记，或执行结果
 * 自带顶层 error 字段的防御形态（kind/data 之外不含 error 的正常结果、
 * null 占位、undefined 均不算失败）。 */
function isFailedResult(r: unknown): boolean {
  return !!r && typeof r === 'object' && !!(r as { error?: unknown }).error;
}

/** T8：区块失败标记是否为 executor_unbound（unbound 阻断 → 绑定引导空态）。 */
function isExecutorUnboundResult(r: unknown): boolean {
  return (
    !!r &&
    typeof r === 'object' &&
    (r as { errorCode?: unknown }).errorCode === EXECUTOR_UNBOUND_CODE
  );
}
