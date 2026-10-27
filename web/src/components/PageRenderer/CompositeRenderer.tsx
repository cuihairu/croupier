import React, { useMemo } from 'react';
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
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import ExecutorUnboundAlert, {
  EXECUTOR_UNBOUND_CODE,
  executeErrorToastText,
} from './ExecutorUnboundAlert';
import { localizedText } from '@/utils/localizedText';
import { confirmFieldTitle } from './confirmContent';
import type { ExecFailure } from './execFeedback';
import { useCompositeRuntime } from './compositeRuntime';
import { sectionVisible } from './sectionCondition';
import type {
  PageExecuteFn,
  CompositeSection,
  PageFunctionBinding,
  FormValues,
} from '@/types/dashboard';

// 分页数据适配层已归位到共享运行时模块（B1 ④：状态/联动/动作链唯一实现）；
// 保留此处的再导出以维持既有导入路径。
export { paginationRequestParams, estimateTotal } from './compositeRuntime';

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
