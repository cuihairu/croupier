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
import type { PageState } from './runtime';
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
}> = ({ sections, bindings, onExecute, preview }) => {
  const { message, modal } = App.useApp();
  const [results, setResults] = useState<Record<string, PageExecutionResult | null>>({});
  const [running, setRunning] = useState<Record<string, boolean>>({});
  const [sectionInputs, setSectionInputs] = useState<Record<string, Record<string, unknown>>>({});
  const [dialogKey, setDialogKey] = useState<string | null>(null);

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
  const resultsRef = useRef(results);
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

  /** 行字段映射：params 目标参数名 → 本行字段名。 */
  const mapRowParams = (
    mapping: Record<string, string> | undefined,
    row: Record<string, unknown>,
  ): Record<string, unknown> => {
    const out: Record<string, unknown> = {};
    for (const [param, rowField] of Object.entries(mapping || {})) {
      out[param] = row[rowField];
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
                  rowSelection={
                    (sec.events ?? []).some((e) => e.event === 'rowSelected')
                      ? {
                          type: 'radio',
                          onChange: (_keys, rows) => {
                            if (rows[0]) {
                              fireEvent(sec, 'rowSelected', rows[0] as Record<string, unknown>);
                            }
                          },
                        }
                      : undefined
                  }
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
              ) : sectionHasForm(sec) ? (
                <SchemaFormRenderer
                  spec={sec.form!}
                  initialValues={(sectionInputs[sec.key] || {}) as FormValues}
                  disabled={running[sec.key] || false}
                  onFinish={async (values) => {
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
  onSubmit: (values: Record<string, unknown>) => Promise<void>;
}> = ({ section, running, initialParams, onSubmit }) => {
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

/** 动作步骤参数解析："区块key.字段"取其输出、"row.字段"取事件行、其余字面量。 */
function resolveStepParams(
  params: Record<string, string> | undefined,
  results: Record<string, unknown>,
  ctx?: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, src] of Object.entries(params ?? {})) {
    const dot = src.indexOf('.');
    if (dot > 0) {
      const head = src.slice(0, dot);
      const field = src.slice(dot + 1);
      if ((head === 'row' || head === 'ctx') && ctx && ctx[field] !== undefined) {
        out[k] = ctx[field];
        continue;
      }
      const raw = results[head] as { data?: Record<string, unknown> } | undefined;
      const payload = raw?.data ?? (raw as Record<string, unknown> | undefined);
      if (payload && payload[field] !== undefined) {
        out[k] = payload[field];
        continue;
      }
    }
    out[k] = src;
  }
  return out;
}
