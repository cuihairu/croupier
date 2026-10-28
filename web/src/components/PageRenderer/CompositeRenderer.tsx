import React, { useEffect, useMemo, useRef, useState } from 'react';
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
  Typography,
} from 'antd';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { formatDateTime } from '@/utils/format';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import ExecutorUnboundAlert, {
  EXECUTOR_UNBOUND_CODE,
  executeErrorToastText,
} from './ExecutorUnboundAlert';
import { localizedText } from '@/utils/localizedText';
import { confirmFieldTitle, humanizeParamKey } from './confirmContent';
import { isStaleSelection, rowIdentity, selectionFilterFingerprint } from './batchSelection';
import type { ExecFailure } from './execFeedback';
import OperationStageBar from './OperationStageBar';
import { useCompositeRuntime } from './compositeRuntime';
import { sectionVisible } from './sectionCondition';
import type {
  PageExecuteFn,
  CompositeSection,
  PageFunctionBinding,
  FormValues,
  ColumnSpec,
} from '@/types/dashboard';

// 分页数据适配层已归位到共享运行时模块（B1 ④：状态/联动/动作链唯一实现）；
// 保留此处的再导出以维持既有导入路径。
export { paginationRequestParams, estimateTotal } from './compositeRuntime';

/** D3 列排序比较函数（分页未启用时的客户端排序）：number 列按数值，
 * 其余按字符串码点序（ISO 日期串的字典序即时间序）；缺失值排末尾。
 * 服务端分页启用时列用 sorter=true（仅表头 UI），排序状态下推请求。 */
function columnSorter(
  c: ColumnSpec,
): (a: Record<string, unknown>, b: Record<string, unknown>) => number {
  const numeric = c.dataType === 'number';
  return (a, b) => {
    const av = a[c.key];
    const bv = b[c.key];
    if (av == null && bv == null) return 0;
    if (av == null) return 1;
    if (bv == null) return -1;
    if (numeric) {
      const an = typeof av === 'number' ? av : Number(av);
      const bn = typeof bv === 'number' ? bv : Number(bv);
      return (Number.isNaN(an) ? 0 : an) - (Number.isNaN(bn) ? 0 : bn);
    }
    const as = String(av);
    const bs = String(bv);
    return as < bs ? -1 : as > bs ? 1 : 0;
  };
}

/** D5 列单元格语义渲染：enum→Tag 标签、render copy→可复制、datetime→带
 * 浏览器时区标签格式化、unit 后缀（number 列）。无匹配返回 undefined（走
 * 表格默认文本渲染）；空值统一 '- '。 */
function compositeColumnRender(
  c: ColumnSpec,
): ((_: unknown, row: Record<string, unknown>) => React.ReactNode) | undefined {
  const hasEnumTag = c.render === 'tag' && !!c.enum?.length;
  const unitText = c.unit ? localizedText(c.unit, 'zh-CN', '') : '';
  const needsRender =
    hasEnumTag ||
    c.render === 'copy' ||
    c.dataType === 'datetime' ||
    (unitText && c.dataType === 'number');
  if (!needsRender) return undefined;
  return (_: unknown, row: Record<string, unknown>) => {
    const value = row[c.key];
    if (value === undefined || value === null || value === '') return '-';
    if (hasEnumTag) {
      const opt = c.enum!.find((e) => e.value === String(value));
      if (opt) {
        return (
          <Tag color={opt.color || 'default'}>
            {localizedText(opt.label, 'zh-CN', String(value))}
          </Tag>
        );
      }
      return String(value);
    }
    if (c.render === 'copy') {
      return <Typography.Text copyable>{String(value)}</Typography.Text>;
    }
    if (c.dataType === 'datetime') {
      return formatDateTime(String(value), true);
    }
    if (unitText) {
      return `${String(value)} ${unitText}`;
    }
    return String(value);
  };
}

/** 组合页渲染器：区块栅格布局；autoRun 加载即执行；refreshOn 联动
 * 自动重跑；dialog 形态区块由行操作/工具栏按钮触发弹窗打开（行字段
 * 映射进表单参数）；操作成功后按 onSuccessRefresh 刷新目标区块。
 *
 * 状态机/级联/动作链不在本文件实现——统一走 useCompositeRuntime，与编辑器
 * 预览共用同一份运行时（B1：三个执行面只差异注入的 execute 适配器）。 */
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
  const { message } = App.useApp();
  const intl = useIntl();
  const {
    results,
    running,
    sectionInputs,
    execFailures,
    execPending,
    dialogKey,
    setDialogKey,
    run: runSection,
    tablePaginationProps,
    handleTableSortChange,
    tableSortStateOf,
    confirmDanger,
    openDialog,
    runChain,
    fireEvent,
    setSelectionState,
    mergeFormValues,
    mergeStaticValues,
    mapRowParams,
    clearExecFeedbackOf,
    submitForm: submitFormSection,
  } = useCompositeRuntime<CompositeSection>({
    sections,
    preview,
    onPageStateMerge,
    // 执行适配器（B1 注入点）：发布页 = 已发布快照绑定执行。
    execute: (sec, inputs) => onExecute(sec.bindingId, { form: inputs as never }),
  });

  /** D4：区块执行反馈条——失败给稳定错误码 + 字段级定位，待处理给审批 /
   * 任务入口。字段名按表单展示层 label 渲染（与 D2 确认框同源），不显示
   * 原始 key。 */
  const renderExecFeedback = (sec: CompositeSection): React.ReactNode => {
    const failure = execFailures[sec.key];
    const pending = execPending[sec.key];
    // D4：已注入控件的字段错误不再重复列在反馈条里，只留表单外的未知字段
    const residualFieldErrors = failure
      ? failure.fieldErrors.filter(
          (fe) => !fe.field || !formFieldErrors(sec.form, failure)[fe.field],
        )
      : [];
    const fieldTitle = (key: string) =>
      confirmFieldTitle(
        sec.form?.fields?.find((f) => f.key === key),
        key,
      );
    return (
      <>
        {/* D1：多阶段状态反馈阶段条，与下方 Alert/Result 同源派生。复合编排
         * 不查询审批状态（无续跑查询通道），审批中按 process 呈现；失败终态
         * 停在提交阶段 error，与操作页一致。 */}
        <OperationStageBar
          input={{
            loading: running[sec.key] || false,
            // D4：结果未知（超时/网络中断）不是「已失败」，阶段条不落 submit error
            hasError: !!failure && failure.outcome !== 'unknown',
            resultKind: pending ? (pending.outcome === 'approval' ? 'approval' : 'task') : null,
            approvalStatus: null,
            continuationKind: null,
          }}
        />
        {failure ? (
          failure.outcome === 'unknown' ? (
            // D4：超时/网络中断——请求可能已在服务端执行，结果未知；不得按
            // 「已失败」提示盲目重复提交，只能经幂等键安全重试或查询执行状态。
            <Alert
              type="warning"
              showIcon
              title={intl.formatMessage({
                id: 'component.pageRenderer.composite.execUnknownOutcome',
                defaultMessage: '执行结果未知',
              })}
              description={
                <>
                  {failure.message ? <div>{failure.message}</div> : null}
                  <div>
                    {intl.formatMessage({
                      id: 'component.pageRenderer.composite.execUnknownOutcomeHint',
                      defaultMessage:
                        '请求可能已在服务端执行，请勿盲目重复提交；可在服务端幂等保证下安全重试，或稍后查询执行状态。',
                    })}
                  </div>
                </>
              }
            />
          ) : (
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
                  {failure.reason ? <div style={{ marginTop: 4 }}>{failure.reason}</div> : null}
                  {residualFieldErrors.length > 0 ? (
                    <ul style={{ margin: '4px 0 0', paddingLeft: 20 }}>
                      {residualFieldErrors.map((fe, i) => (
                        <li key={i}>
                          {fe.field ? `${fieldTitle(fe.field)}：${fe.message}` : fe.message}
                        </li>
                      ))}
                    </ul>
                  ) : null}
                </>
              }
            />
          )
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
                    onClick={() =>
                      history.push(
                        pending.id
                          ? `/approvals?approvalId=${encodeURIComponent(pending.id)}`
                          : '/approvals',
                      )
                    }
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
                  <Button
                    type="link"
                    size="small"
                    style={{ padding: 0 }}
                    onClick={() => history.push('/ops/jobs')}
                  >
                    {intl.formatMessage({
                      id: 'component.pageRenderer.composite.goToTaskCenter',
                      defaultMessage: '前往任务中心',
                    })}
                  </Button>
                </Space>
              }
            />
          )
        ) : null}
      </>
    );
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
              // D2 跨目标总量：本表格区块已选目标数（多选时确认框对
              // x-field-per-target 字段追加总量条目）。
              targetCount: selRows[sec.key]?.length,
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

  /** D2 批量目标核对：行选择配置（multiple=true 才启用 checkbox 多选与
   * 选择栏；缺省保持既有单选 radio 语义）。 */
  const tableSelectionOf = (sec: CompositeSection) =>
    sec.table?.selection?.multiple ? sec.table.selection : undefined;

  /** D2 行标识口径：selection.identityKey → 表格身份列，与目标清单、选择
   * 计数取同一口径（多选缺身份列时 batchSelection.rowIdentity 按
   * id/key/数据串兜底，跨页保留可能错位——编辑器提示配置身份列）。 */
  const rowIdKeyOf = (sec: CompositeSection) =>
    sec.table?.selection?.identityKey || sec.table?.identityKey;

  /** D2 目标清单列：身份列置前，随后按展示列序最多补两列（缺失时人性化
   * key 而非原始字段名——与资源页目标清单同一口径）。 */
  const targetListColumnsOf = (sec: CompositeSection, idKey: string | undefined) => {
    const cols = (sec.table?.columns ?? []).slice();
    cols.sort((a, b) => {
      if (a.key === idKey) return -1;
      if (b.key === idKey) return 1;
      return 0;
    });
    return cols.slice(0, 3).map((c) => ({
      title: localizedText(c.title, 'zh-CN', humanizeParamKey(c.key)),
      dataIndex: c.key,
      width: 180,
      ellipsis: true,
    }));
  };

  // D2：选择状态按区块 key 记账（跨页保留的选中行对象 + 键 + 筛选变更警示
  // + 目标清单弹窗归属）。选择行写入运行时（selectedRow/selectedRows），
  // 供 inputAssignments 服务端求值与 rowSelected 事件消费。
  const [selRows, setSelRows] = useState<Record<string, FormValues[]>>({});
  const [selKeys, setSelKeys] = useState<Record<string, React.Key[]>>({});
  const [selStale, setSelStale] = useState<Record<string, boolean>>({});
  const [targetListKey, setTargetListKey] = useState<string | null>(null);
  const selFingerprintRef = useRef<Record<string, string | null>>({});

  // D2：筛选变化后旧选择的核对提示。指纹取执行入参（sectionInputs 含上游
  // 映射后的查询条件，分页参数在 run 内并入不入账——翻页不算筛选变更）；
  // 每次入参变化重新判定，选择变更（selRows）时同条件重算回 false。
  useEffect(() => {
    const next: Record<string, boolean> = {};
    for (const sec of sections) {
      if (sec.view !== 'table' || !tableSelectionOf(sec)) continue;
      const key = sec.key;
      const fp = selectionFilterFingerprint(sectionInputs[key] || {});
      next[key] = isStaleSelection(
        selFingerprintRef.current[key] ?? null,
        fp,
        selRows[key]?.length ?? 0,
      );
      selFingerprintRef.current = { ...selFingerprintRef.current, [key]: fp };
    }
    setSelStale((prev) => {
      const changed = Object.keys(next).some((k) => (next[k] ?? false) !== (prev[k] ?? false));
      return changed ? next : prev;
    });
  }, [sectionInputs, sections, selRows]);

  const clearSelection = (sec: CompositeSection): void => {
    setSelRows((prev) => ({ ...prev, [sec.key]: [] }));
    setSelKeys((prev) => ({ ...prev, [sec.key]: [] }));
    setSelStale((prev) => ({ ...prev, [sec.key]: false }));
    setSelectionState(sec.key, []);
  };

  /** 表格区块渲染（D2 批量目标核对：选择栏 + 目标清单 + 多选/单选差异；
   * 单选 radio 路径保持既有 rowSelected 事件语义）。 */
  const renderTable = (sec: CompositeSection): React.ReactNode => {
    const sel = tableSelectionOf(sec);
    const idKey = rowIdKeyOf(sec);
    const rows = selRows[sec.key] ?? [];
    const keys = selKeys[sec.key] ?? [];
    const data = Array.isArray(resultFor(sec)?.items)
      ? (resultFor(sec)?.items as Record<string, unknown>[])
      : [];
    const visibleCount = data.filter((r) =>
      keys.includes(rowIdentity(r as FormValues, idKey)),
    ).length;
    const stale = (selStale[sec.key] ?? false) && rows.length > 0;
    return (
      <>
        {sel && rows.length > 0 ? (
          <Space size={12} wrap style={{ marginBottom: 8 }}>
            <span data-testid="selection-count">
              {intl.formatMessage(
                {
                  id: 'component.pageRenderer.composite.selection.count',
                  defaultMessage: '已选择 {count} 项',
                },
                { count: rows.length },
              )}
            </span>
            {visibleCount !== rows.length ? (
              <span data-testid="selection-visible">
                {intl.formatMessage(
                  {
                    id: 'component.pageRenderer.composite.selection.visible',
                    defaultMessage: '本页可见 {count} 项',
                  },
                  { count: visibleCount },
                )}
              </span>
            ) : null}
            {stale ? (
              <Alert
                type="warning"
                showIcon
                data-testid="selection-stale"
                title={intl.formatMessage({
                  id: 'component.pageRenderer.composite.selection.stale',
                  defaultMessage: '筛选条件已变更，已选对象不在当前结果集，请核对后清除或重新勾选',
                })}
              />
            ) : null}
            <Button
              type="link"
              size="small"
              data-testid="selection-target-list"
              onClick={() => setTargetListKey(sec.key)}
            >
              <FormattedMessage
                id="component.pageRenderer.composite.selection.targetList"
                defaultMessage="查看目标清单"
              />
            </Button>
            <Button type="link" size="small" onClick={() => clearSelection(sec)}>
              <FormattedMessage
                id="component.pageRenderer.composite.selection.clear"
                defaultMessage="取消选择"
              />
            </Button>
          </Space>
        ) : null}
        <Table
          size="small"
          rowKey={
            sel
              ? (record, i) => rowIdentity(record as FormValues, idKey) || String(i)
              : (_record, i) => String(i)
          }
          onRow={(record) => ({
            onClick: () => fireEvent(sec, 'rowClick', record as Record<string, unknown>),
          })}
          rowSelection={
            sel
              ? {
                  type: 'checkbox',
                  preserveSelectedRowKeys: true,
                  selectedRowKeys: keys,
                  onChange: (nextKeys, nextRows) => {
                    setSelRows((prev) => ({ ...prev, [sec.key]: nextRows as FormValues[] }));
                    setSelKeys((prev) => ({ ...prev, [sec.key]: nextKeys }));
                    // V5：选中行写入运行时状态（selectedRow/selectedRows
                    // 数据来源，{{var.selectedRows.x}} 服务端求值）。
                    setSelectionState(sec.key, nextRows as Record<string, unknown>[]);
                    if (nextRows[0] && (sec.events ?? []).some((e) => e.event === 'rowSelected')) {
                      fireEvent(sec, 'rowSelected', nextRows[0] as Record<string, unknown>);
                    }
                  },
                }
              : {
                  type: 'radio',
                  onChange: (_keys, nextRows) => {
                    // V5：选中行写入运行时状态（{{var.selectedRow.x}} 数据来源），
                    // 仅在绑定 rowSelected 事件时同步触发事件。
                    setSelectionState(sec.key, nextRows as Record<string, unknown>[]);
                    if (nextRows[0] && (sec.events ?? []).some((e) => e.event === 'rowSelected')) {
                      fireEvent(sec, 'rowSelected', nextRows[0] as Record<string, unknown>);
                    }
                  },
                }
          }
          columns={[
            ...(sec.table?.columns || []).map((c) => {
              const serverPage = sec.table?.pagination?.enabled === true;
              const sortState = tableSortStateOf(sec);
              // 受控 sortOrder 仅在有值时下键：antd 以 `'sortOrder' in column`
              // 判定受控排序，显式 undefined 会让内部排序状态失效（点击
              // 表头不排序）。
              const sortOrder: 'ascend' | 'descend' | undefined =
                c.sortable && serverPage && sortState?.field === c.key
                  ? sortState.order === 'desc'
                    ? 'descend'
                    : 'ascend'
                  : undefined;
              // D5：单位/枚举标签/复制/时区标注的列语义渲染
              const cellRender = compositeColumnRender(c);
              const unitText = c.unit ? localizedText(c.unit, 'zh-CN', '') : '';
              return {
                title: localizedText(c.title, 'zh-CN', c.key) + (unitText ? ` (${unitText})` : ''),
                dataIndex: c.key,
                ellipsis: true,
                ...(cellRender ? { render: cellRender } : {}),
                // D3：可排序列给表头排序 UI。分页未启用时 antd 本地排序
                //（compare 函数）；启用时 sorter=true（仅 UI），排序状态
                // 经受控 sortOrder 下推请求（sortBy/sortOrder）。
                ...(c.sortable
                  ? {
                      sorter: serverPage ? true : columnSorter(c),
                      ...(sortOrder ? { sortOrder } : {}),
                    }
                  : {}),
              };
            }),
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
          dataSource={data}
          onChange={(_pagination, _filters, sorter) => {
            // D3：排序变化走受控状态（服务端分页时下推 sortBy/sortOrder 并
            // 回首页重跑；客户端排序由 antd 本地完成，运行时按分页状态
            // 兜底 no-op）。翻页不经过此分支（pagination.onChange 已接手）。
            const s = (Array.isArray(sorter) ? sorter[0] : sorter) as
              | {
                  order?: 'ascend' | 'descend' | null;
                  field?: string;
                  sortColumn?: { dataIndex?: unknown; key?: unknown };
                }
              | undefined;
            if (!s) return;
            const field = (s.sortColumn?.dataIndex ?? s.sortColumn?.key ?? s.field) as
              string | undefined;
            handleTableSortChange(
              sec,
              field,
              s.order === 'ascend' ? 'asc' : s.order === 'descend' ? 'desc' : undefined,
            );
          }}
          pagination={tablePaginationProps(sec)}
        />
        <Modal
          title={intl.formatMessage(
            {
              id: 'component.pageRenderer.composite.selection.targetListTitle',
              defaultMessage: '已选目标清单（{count} 项）',
            },
            { count: rows.length },
          )}
          open={targetListKey === sec.key}
          onCancel={() => setTargetListKey(null)}
          footer={null}
          width={720}
        >
          <Table
            size="small"
            rowKey={(record, i) => rowIdentity(record as FormValues, idKey) || String(i)}
            pagination={false}
            columns={targetListColumnsOf(sec, idKey)}
            dataSource={rows as unknown as Record<string, unknown>[]}
          />
        </Modal>
      </>
    );
  };

  const bindingById = useMemo(() => {
    const map: Record<string, PageFunctionBinding> = {};
    for (const binding of bindings) map[binding.id] = binding;
    return map;
  }, [bindings]);
  const renderSection = (sec: CompositeSection): React.ReactNode => (
    <Card
      size="small"
      title={localizedText(sec.title, 'zh-CN', sec.key)}
      // D4：表单区块提交期间不进整卡骨架——骨架会卸载表单，失败后已填值
      // 随之丢失；在途状态由表单 disabled 表达。非表单区块保留骨架。
      loading={(running[sec.key] || false) && !sectionHasForm(sec)}
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
        renderTable(sec)
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
            mergeStaticValues(sec.key, values as Record<string, unknown>);
          }}
        />
      ) : sectionHasForm(sec) ? (
        <>
          {renderExecFeedback(sec)}
          <SchemaFormRenderer
            spec={sec.form!}
            initialValues={(sectionInputs[sec.key] || {}) as FormValues}
            disabled={running[sec.key] || false}
            externalErrors={formFieldErrors(sec.form, execFailures[sec.key])}
            onValuesChange={(_, values) => {
              // V5：fnForm 当前值防抖写入 results[key].values（{{var.values.x}}）
              mergeFormValues(sec.key, values as Record<string, unknown>);
              // D4：输入变化后旧失败反馈失效
              clearExecFeedbackOf(sec.key);
            }}
            onFinish={(values) => {
              // 失败已在运行时落失败反馈，此处仅防未捕获 rejection
              submitFormSection(sec, values).catch(() => undefined);
            }}
          />
        </>
      ) : (
        <>
          {renderExecFeedback(sec)}
          <Button
            type="primary"
            loading={running[sec.key] || false}
            onClick={() => {
              // 失败已在运行时落失败反馈，此处仅防未捕获 rejection
              submitFormSection(sec, (sectionInputs[sec.key] || {}) as FormValues).catch(
                () => undefined,
              );
            }}
          >
            {localizedText(sec.title, 'zh-CN', sec.key)}
          </Button>
        </>
      )}
    </Card>
  );

  const resultFor = (sec: CompositeSection): Record<string, unknown> | undefined =>
    (results[sec.key] as { data?: Record<string, unknown> } | null | undefined)?.data;

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
                  fieldErrors={formFieldErrors(sec.form, execFailures[sec.key])}
                  onValuesChange={(values) => {
                    mergeFormValues(sec.key, values);
                    // D4：输入变化后旧失败反馈失效（弹窗保留待处理提示直至提交）
                    clearExecFeedbackOf(sec.key);
                  }}
                  onSubmit={async (values) => {
                    try {
                      // D4：审批 / 任务提交 ≠ 业务执行成功——弹窗保持开启，
                      // 待处理反馈（含审批入口）与字段级错误都留在弹窗内。
                      const outcome = await submitFormSection(sec, values as FormValues);
                      if (!outcome.ok) return;
                      setDialogKey(null);
                      const title = localizedText(sec.title, 'zh-CN', sec.key);
                      // D4 幂等重放：结果取自服务端已落地记录，明确告知未重复执行。
                      message.success(
                        intl.formatMessage(
                          outcome.replayed
                            ? {
                                id: 'component.pageRenderer.composite.executeReplayed',
                                defaultMessage: `${title} 已在先前提交过，本次返回服务端记录的结果（未重复执行）`,
                              }
                            : {
                                id: 'component.pageRenderer.composite.executeSuccess',
                                defaultMessage: `${title} 执行成功`,
                              },
                          { title },
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
                  columns={(sec.table?.columns ?? []).map((c) => {
                    // D5：弹窗内表格列保留语义单元格渲染（unit/枚举标签/时区），
                    // 但不加表头排序 UI（D3 边界：弹窗表格不分页不排序）。
                    const cellRender = compositeColumnRender(c);
                    const unitText = c.unit ? localizedText(c.unit, 'zh-CN', '') : '';
                    return {
                      title:
                        localizedText(c.title, 'zh-CN', c.key) + (unitText ? ` (${unitText})` : ''),
                      dataIndex: c.key,
                      ellipsis: true,
                      ...(cellRender ? { render: cellRender } : {}),
                    };
                  })}
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
  /** D4：字段级错误按字段名注入控件下方（只留表单外字段在反馈条）。 */
  fieldErrors?: Record<string, string[]>;
  /** V5：当前值变化（防抖写入 values 运行时状态由调用方处理）。 */
  onValuesChange?: (values: Record<string, unknown>) => void;
  onSubmit: (values: Record<string, unknown>) => Promise<void>;
}> = ({ section, running, initialParams, feedback, fieldErrors, onValuesChange, onSubmit }) => {
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
        externalErrors={fieldErrors}
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

/** D4：把服务端字段级错误映射成表单可注入的 externalErrors——只映射 schema
 * 顶层存在且带文案的字段；其余字段留在反馈条汇总（不回退服务端返回的未知
 * 字段错误）。键为 schema 顶层属性名，与 SchemaFormRenderer 的注入口径一致。 */
function formFieldErrors(
  form: CompositeSection['form'],
  failure: ExecFailure | null | undefined,
): Record<string, string[]> {
  if (!failure || failure.fieldErrors.length === 0) return {};
  const properties = form?.jsonSchema?.properties;
  const known =
    properties && typeof properties === 'object' ? new Set(Object.keys(properties)) : undefined;
  const mapped: Record<string, string[]> = {};
  for (const fe of failure.fieldErrors) {
    if (!fe.field || !fe.message || !known || !known.has(fe.field)) continue;
    (mapped[fe.field] ??= []).push(fe.message);
  }
  return mapped;
}

/** T8：区块失败标记是否为 executor_unbound（unbound 阻断 → 绑定引导空态）。 */
function isExecutorUnboundResult(r: unknown): boolean {
  return (
    !!r &&
    typeof r === 'object' &&
    (r as { errorCode?: unknown }).errorCode === EXECUTOR_UNBOUND_CODE
  );
}
