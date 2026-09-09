import { localizedText } from '@/utils/localizedText';
/**
 * PageRenderer - 页面渲染器统一入口
 *
 * 根据 PageSpec 的类型自动选择合适的渲染器：
 * - ResourcePageRenderer: 资源 CRUD 页面
 * - OperationPageRenderer: 独立操作页面
 * - TaskPageRenderer: 异步任务页面
 * - ReportPageRenderer: 报表页面
 *
 * @module components/PageRenderer
 */

import React, { useCallback, useEffect, useRef, useState } from 'react';
import { App, Button, Card, Col, Descriptions, Modal, Result, Row, Space, Table } from 'antd';
import { ExclamationCircleOutlined, WarningOutlined } from '@ant-design/icons';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import ResourcePageRenderer from './ResourcePageRenderer';
import OperationPageRenderer from './OperationPageRenderer';
import TaskPageRenderer from './TaskPageRenderer';
import ReportPageRenderer from './ReportPageRenderer';
import {
  contextWithPageState,
  mergePageState,
  outputPatchFromResult,
  projectBindingContext,
} from './runtime';
import type { PageState, PageStatePatch } from './runtime';
import { parseExpression, resolveExpression, resolveRef } from './expression';
import type { JSONValue } from '@/types/dashboard';
import type {
  PageSpec,
  PageExecuteFn,
  TaskStatusResult,
  BindingExecutionContext,
  PageExecutionResult,
  ApprovalStatusResult,
  CompositeSection,
  PageFunctionBinding,
  ColumnSpec,
  FormValues,
} from '@/types/dashboard';

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface PageRendererProps {
  /** 页面规格 */
  pageSpec: PageSpec;
  /** 执行绑定函数 */
  onExecute: PageExecuteFn;
  /** 预览模式只展示页面结构，禁止触发真实函数执行 */
  preview?: boolean;
  /** 查询任务状态（仅 TaskPage 需要） */
  onQueryStatus?: (taskId: string) => Promise<TaskStatusResult>;
  /** 取消任务（仅 TaskPage 需要） */
  onCancelTask?: (taskId: string) => Promise<void>;
  /** 查询审批状态（Operation/Task 等待审批需要） */
  onQueryApprovalStatus?: (approvalId: string) => Promise<ApprovalStatusResult>;
  /** 导出数据（仅 ReportPage 需要） */
  onExport?: (format: 'csv' | 'excel') => Promise<void>;
}

// ---------------------------------------------------------------------------
// PageRenderer 组件
// ---------------------------------------------------------------------------

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
      | null
    >
  >({});
  const [running, setRunning] = useState<Record<string, boolean>>({});
  const [sectionInputs, setSectionInputs] = useState<Record<string, Record<string, unknown>>>({});
  const [dialogKey, setDialogKey] = useState<string | null>(null);
  // 运行时状态快照 ref（事件帧内求值/执行读取，避免 setState 异步导致同帧读旧值）
  const resultsRef = useRef(results);
  // 常量表单（static）值缓冲：防抖后并入 results 驱动 refreshOn 联动
  const staticMergeRef = useRef<Record<string, { data: Record<string, unknown> }>>({});
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
  const setSelectionState = useCallback((key: string, rows: Record<string, unknown>[]) => {
    const cur = (resultsRef.current[key] ?? {}) as Record<string, unknown>;
    const entry = { ...cur, selectedRow: rows[0], selectedRows: rows };
    resultsRef.current = { ...resultsRef.current, [key]: entry };
    setResults(resultsRef.current);
    onPageStateMerge?.(key, { selectedRow: rows[0], selectedRows: rows }, 'merge');
  }, []);
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
      const merged = { ...(inputsRef.current[sec.key] || {}), ...(overrides || {}) };
      setRunning((prev) => ({ ...prev, [sec.key]: true }));
      try {
        const result = await onExecute(sec.bindingId, { form: merged as never });
        setResults((prev) => ({ ...prev, [sec.key]: result || null }));
        // V5：函数输出并入 page_state.data（{{var.data.x}} 服务端求值来源）
        onPageStateMerge?.(
          sec.key,
          { data: (result as { data?: unknown })?.data ?? null },
          'merge',
        );
        // 操作类区块成功后刷新目标（发邮件成功 → 刷新玩家表格）
        if ((sec.view === 'form' || sec.view === 'actions') && sec.onSuccessRefresh?.length) {
          for (const target of sec.onSuccessRefresh) {
            const t = sectionsRef.current.find((x) => x.key === target);
            if (t) void runSection(t);
          }
        }
        return result;
      } finally {
        setRunning((prev) => ({ ...prev, [sec.key]: false }));
      }
    },
    [onExecute, preview],
  );

  const runSectionRef = useRef(runSection);
  runSectionRef.current = runSection;

  // autoRun：加载即执行（inline 区块）
  useEffect(() => {
    for (const sec of sections) {
      if (sec.autoRun && sec.display !== 'dialog') void runSection(sec);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // refreshOn 联动：上游产出新结果时自动重跑本区块（跨函数联动）。
  useEffect(() => {
    resultsRef.current = results;
  }, [results]);
  useEffect(() => {
    for (const sec of sections) {
      if (!sec.refreshOn?.length || sec.display === 'dialog') continue;
      const depChanged = sec.refreshOn.some((dep) => dep in results);
      if (depChanged && !running[sec.key]) void runSection(sec);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [Object.keys(results).join(',')]);

  // 上游输出同名字段合并进下游输入
  useEffect(() => {
    setSectionInputs((prev) => {
      const next = { ...prev };
      let changed = false;
      for (const sec of sections) {
        if (!sec.refreshOn?.length) continue;
        const merged: Record<string, unknown> = { ...(next[sec.key] || {}) };
        for (const dep of sec.refreshOn) {
          const depData = (results[dep] as { data?: Record<string, unknown> } | null | undefined)
            ?.data;
          if (depData) {
            Object.assign(merged, depData);
            changed = true;
          }
        }
        next[sec.key] = merged;
      }
      return changed ? next : prev;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [Object.keys(results).join(',')]);

  const resultFor = (sec: CompositeSection): Record<string, unknown> | undefined => {
    const r = results[sec.key];
    return (r as { data?: Record<string, unknown> } | null | undefined)?.data;
  };

  /** 打开弹窗：目标可以是区块 key 或弹窗分组名（Group）；危险操作先确认。 */
  const openDialog = useCallback(
    (targetSection: string, params: Record<string, unknown>, danger?: boolean, label?: string) => {
      setSectionInputs((prev) => ({ ...prev, [targetSection]: params }));
      if (danger && label) {
        modal.confirm({
          title: `确认执行「${label}」`,
          icon: <ExclamationCircleOutlined />,
          onOk: () => setDialogKey(targetSection),
        });
        return;
      }
      setDialogKey(targetSection);
    },
    [modal],
  );

  /** 动作链执行：runBinding/refreshNode 按序触发。 */
  /** 动作链执行：run/refresh（params 来源解析）/closeModal/navigate/showMessage。 */
  const runChain = useCallback(
    (
      chain: Array<{ kind: string; target: string; params?: Record<string, string> }> | undefined,
      ctx?: Record<string, unknown>,
    ) => {
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
        if (step.kind === 'runBinding' || step.kind === 'refreshNode') {
          const target = sectionsRef.current.find(
            (x) => x.key === step.target || x.group === step.target,
          );
          if (target) {
            void runSectionRef.current(
              target,
              resolveStepParams(step.params, resultsRef.current, ctx) as never,
            );
          }
        }
      }
    },
    [message],
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

  const inline = sections.filter((s) => s.display !== 'dialog');
  const dialogs = sections.filter((s) => s.display === 'dialog');
  /** dialogKey 命中的弹窗分组（target 可为 group 名或区块 key）。 */
  const groupOf = (sec: CompositeSection): string => sec.group ?? sec.key ?? sec.bindingId;
  const activeDialogs = dialogKey
    ? dialogs.filter(
        (d) => groupOf(d) === dialogKey || d.key === dialogKey || d.bindingId === dialogKey,
      )
    : [];

  return (
    <>
      <Row gutter={[12, 12]}>
        {inline.map((sec) => (
          <Col key={sec.key} span={sec.span && sec.span > 0 && sec.span <= 24 ? sec.span : 24}>
            <Card
              size="small"
              title={localizedText(sec.title, 'zh-CN', sec.key)}
              loading={running[sec.key] || false}
              extra={
                sec.view !== 'actions' && sec.view !== 'toolbar' && !sec.autoRun ? (
                  <Button size="small" onClick={() => void runSection(sec)}>
                    执行
                  </Button>
                ) : null
              }
            >
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
                            title: '操作',
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
                                        openDialog(
                                          ra.targetSection,
                                          mapRowParams(ra.params, row),
                                          ra.danger,
                                          localizedText(ra.label, 'zh-CN'),
                                        );
                                      }
                                      runChain(ra.chain);
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
                  pagination={{ pageSize: 10, showSizeChanger: false }}
                />
              ) : sec.view === 'fields' ? (
                <div onClick={() => fireEvent(sec, 'click')}>
                  <Descriptions size="small" column={2}>
                    {(resultFor(sec)
                      ? Object.entries(resultFor(sec) as Record<string, unknown>)
                      : []
                    ).map(([k, v]) => (
                      <Descriptions.Item key={k} label={k}>
                        {String(v ?? '-')}
                      </Descriptions.Item>
                    ))}
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
                          openDialog(
                            act.targetSection,
                            { ...(act.params || {}) },
                            act.danger,
                            localizedText(act.label, 'zh-CN'),
                          );
                        }
                        runChain(act.chain);
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
                    staticMergeRef.current[sec.key] = { data: values as Record<string, unknown> };
                    scheduleStaticFlush();
                  }}
                />
              ) : sectionHasForm(sec) ? (
                <SchemaFormRenderer
                  spec={sec.form!}
                  initialValues={(sectionInputs[sec.key] || {}) as FormValues}
                  disabled={running[sec.key] || false}
                  onValuesChange={(_, values) => {
                    // V5：fnForm 当前值防抖写入 results[key].values（{{var.values.x}}）
                    valuesMergeRef.current[sec.key] = values as Record<string, unknown>;
                    scheduleValuesFlush();
                  }}
                  onFinish={async (values) => {
                    // 双形态快照（扁平兼容遗留 + values 包装），并驱动提交
                    onPageStateMerge?.(sec.key, { ...(values as Record<string, unknown>), values });
                    const r = await runSection(sec, values);
                    if (r && !(r as { error?: string }).error) fireEvent(sec, 'success');
                  }}
                />
              ) : (
                <Button
                  type="primary"
                  onClick={() =>
                    void runSection(sec).then((r) => {
                      if (r && !(r as { error?: string }).error) fireEvent(sec, 'success');
                    })
                  }
                >
                  {localizedText(sec.title, 'zh-CN', sec.key)}
                </Button>
              )}
            </Card>
          </Col>
        ))}
      </Row>

      {/* 弹窗形态区块：按分组聚合渲染（一个弹窗多区块：表单提交 + 展示组件） */}
      {dialogKey && activeDialogs.length > 0 && (
        <Modal
          title={localizedText(activeDialogs[0].title, 'zh-CN', dialogKey)}
          open
          onCancel={() => setDialogKey(null)}
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
                  onValuesChange={(values) => {
                    valuesMergeRef.current[sec.key] = values;
                    scheduleValuesFlush();
                  }}
                  onSubmit={async (values) => {
                    const r = await runSectionRef.current(sec, values);
                    setDialogKey(null);
                    message.success(`${localizedText(sec.title, 'zh-CN', sec.key)} 执行成功`);
                    fireEvent(sec, 'success');
                    void r;
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
  /** V5：当前值变化（防抖写入 values 运行时状态由调用方处理）。 */
  onValuesChange?: (values: Record<string, unknown>) => void;
  onSubmit: (values: Record<string, unknown>) => Promise<void>;
}> = ({ section, running, initialParams, onValuesChange, onSubmit }) => {
  const spec = section.form;
  const properties = spec?.jsonSchema?.properties;
  const hasFields = !!properties && typeof properties === 'object';
  if (!spec || !hasFields) {
    return (
      <Button type="primary" block loading={running} onClick={() => void onSubmit({})}>
        确认执行
      </Button>
    );
  }
  return (
    <SchemaFormRenderer
      spec={spec}
      initialValues={initialParams as FormValues}
      disabled={running}
      onValuesChange={(_, values) => onValuesChange?.(values as Record<string, unknown>)}
      onFinish={async (values) => {
        await onSubmit(values);
      }}
    />
  );
};

const PageRenderer: React.FC<PageRendererProps> = ({
  pageSpec,
  onExecute,
  preview = false,
  onQueryStatus,
  onCancelTask,
  onQueryApprovalStatus,
  onExport,
}) => {
  const { type, bindings } = pageSpec;
  const [pageState, setPageState] = useState<PageState>({});
  const pageStateRef = useRef<PageState>({});

  useEffect(() => {
    pageStateRef.current = {};
    setPageState({});
  }, [pageSpec.pageKey]);

  useEffect(() => {
    pageStateRef.current = pageState;
  }, [pageState]);

  const executeWithPageState = useCallback<PageExecuteFn>(
    async (bindingId: string, context: BindingExecutionContext): Promise<PageExecutionResult> => {
      const binding = bindings.find((item) => item.id === bindingId);
      const result = await onExecute(
        bindingId,
        projectBindingContext(binding, contextWithPageState(context, pageStateRef.current)),
      );
      const patch = outputPatchFromResult(binding, result);
      setPageState((current) => {
        const next = mergePageState(current, patch);
        pageStateRef.current = next;
        return next;
      });
      return result;
    },
    [bindings, onExecute],
  );

  // 根据页面类型选择渲染器
  switch (type) {
    case 'composite': {
      if (!pageSpec.composite || pageSpec.composite.sections.length === 0) {
        return (
          <Result
            status="warning"
            title="配置错误"
            subTitle="组合页面缺少 composite 配置"
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <CompositeRenderer
          sections={pageSpec.composite.sections}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          onPageStateMerge={(key, values, mode) => {
            setPageState((current) => {
              const prev = current[key];
              const nextValue =
                mode === 'merge' && prev && typeof prev === 'object' && !Array.isArray(prev)
                  ? ({ ...(prev as Record<string, JSONValue>), ...values } as never)
                  : (values as never);
              const patch: PageStatePatch = { [key]: nextValue };
              const next = mergePageState(current, patch);
              pageStateRef.current = next;
              return next;
            });
          }}
        />
      );
    }

    case 'resource':
      if (!pageSpec.resource) {
        return (
          <Result
            status="warning"
            title="配置错误"
            subTitle="资源页面缺少 resource 配置"
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <ResourcePageRenderer
          spec={pageSpec.resource}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          title={localizedText(pageSpec.title, 'zh-CN', '')}
        />
      );

    case 'operation':
      if (!pageSpec.operation) {
        return (
          <Result
            status="warning"
            title="配置错误"
            subTitle="操作页面缺少 operation 配置"
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <OperationPageRenderer
          spec={pageSpec.operation}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          onQueryApprovalStatus={onQueryApprovalStatus}
          title={localizedText(pageSpec.title, 'zh-CN', '')}
        />
      );

    case 'task':
      if (!pageSpec.task) {
        return (
          <Result
            status="warning"
            title="配置错误"
            subTitle="任务页面缺少 task 配置"
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <TaskPageRenderer
          spec={pageSpec.task}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          onQueryStatus={onQueryStatus}
          onCancelTask={onCancelTask}
          onQueryApprovalStatus={onQueryApprovalStatus}
          title={localizedText(pageSpec.title, 'zh-CN', '')}
        />
      );

    case 'report':
      if (!pageSpec.report) {
        return (
          <Result
            status="warning"
            title="配置错误"
            subTitle="报表页面缺少 report 配置"
            icon={<WarningOutlined />}
          />
        );
      }
      return (
        <ReportPageRenderer
          spec={pageSpec.report}
          bindings={bindings}
          onExecute={executeWithPageState}
          preview={preview}
          onExport={onExport}
          title={localizedText(pageSpec.title, 'zh-CN', '')}
        />
      );

    default:
      return (
        <Result
          status="error"
          title="未知页面类型"
          subTitle={`不支持的页面类型: ${type}`}
          icon={<WarningOutlined />}
        />
      );
  }
};

export default PageRenderer;

/** 区块是否带有可渲染的表单 schema（无字段时降级为执行按钮）。 */
function sectionHasForm(sec: CompositeSection): boolean {
  const properties = sec.form?.jsonSchema?.properties;
  return !!properties && typeof properties === 'object' && Object.keys(properties).length > 0;
}

/** V5 运行时状态的顶层键（data 之外），用于区分 {{var.x}} 的遗留 data 形态。 */
export const RUNTIME_STATE_KEYS = new Set(['data', 'selectedRow', 'selectedRows', 'values']);

/** 动作步骤参数解析（V5 §7.2：resolveStepParams = 表达式求值器的薄封装）。
 * - `{{表达式}}`：按受限路径文法求值（变量名最长前缀匹配）；
 * - 遗留裸形态 `区块key.字段` / `row.字段`：按同语义解析——单段路径落在
 *   区块 data 上（V3 行为兼容），`row.`/`ctx.` 取事件上下文；
 * - 其余原样字面量。 */
export function resolveStepParams(
  params: Record<string, string> | undefined,
  results: Record<string, unknown>,
  ctx?: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  const variables = new Set(Object.keys(results));
  for (const [k, src] of Object.entries(params ?? {})) {
    if (src.startsWith('{{')) {
      out[k] = resolveExpression(src, results, ctx, variables);
      continue;
    }
    const parsed = parseExpression(`{{${src}}}`, variables);
    if (parsed.ok && parsed.ref.variable !== 'row') {
      const ref = parsed.ref;
      const state = results[ref.variable] as Record<string, unknown> | undefined;
      // 遗留形态：单段路径且非运行时状态键 → 落在 data 上（V3 兼容）
      const legacyDataRef =
        ref.path.length === 1 &&
        typeof ref.path[0] === 'string' &&
        !RUNTIME_STATE_KEYS.has(ref.path[0]) &&
        state &&
        typeof state === 'object' &&
        'data' in state
          ? { variable: ref.variable, path: ['data', ...ref.path] as Array<string | number> }
          : ref;
      const value = resolveRef(legacyDataRef, results, ctx);
      if (value !== undefined) {
        out[k] = value;
        continue;
      }
    }
    if (parsed.ok && parsed.ref.variable === 'row') {
      const value = resolveRef(parsed.ref, results, ctx);
      if (value !== undefined) {
        out[k] = value;
        continue;
      }
    }
    out[k] = src;
  }
  return out;
}
