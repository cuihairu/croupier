/**
 * FormilyPageRenderer - PageSpec Formily 渲染器
 *
 * 运行控制台唯一页面渲染入口。组件只能通过 bindingId 执行已发布页面 binding，
 * 不能直接引用 functionId 或旧 layout 协议。
 */

import React, { createContext, useContext, useMemo, useState } from 'react';
import { createForm } from '@formily/core';
import { createSchemaField, FormProvider, useForm } from '@formily/react';
import { Alert, App, Button, Card, Space, Spin, Table, Timeline, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type {
  JSONValue,
  PageExecutionResult,
  PageFunctionBinding,
  PageSpec,
  PublishedPageSpec,
} from '@/types/dashboard';

type JSONRecord = { [key: string]: JSONValue };

type PageStateValue = PageExecutionResult | JSONValue;

type PageState = Record<string, PageStateValue>;

type RuntimeContextValue = {
  page: PageSpec | PublishedPageSpec;
  bindings: Map<string, PageFunctionBinding>;
  state: PageState;
  execute: (bindingId: string, payload: JSONValue) => Promise<PageExecutionResult>;
  setStateValue: (key: string, value: PageStateValue) => void;
};

type PageColumn = {
  title: string;
  dataIndex: string;
  key?: string;
};

type RowAction = {
  bindingId: string;
  label?: string;
  risk?: string;
  inputMapping?: JSONValue;
};

const DEFAULT_RESULT_STATE_KEY = 'lastExecution';

const RuntimeContext = createContext<RuntimeContextValue | null>(null);

function useRuntimeContext() {
  const value = useContext(RuntimeContext);
  if (!value) {
    throw new Error('FormilyPageRenderer runtime context is missing');
  }
  return value;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isJSONValue(value: unknown): value is JSONValue {
  if (value === null) return true;
  if (['boolean', 'number', 'string'].includes(typeof value)) return true;
  if (Array.isArray(value)) return value.every(isJSONValue);
  if (!isRecord(value)) return false;
  return Object.values(value).every(isJSONValue);
}

function toJSONValue(value: unknown): JSONValue {
  return isJSONValue(value) ? value : JSON.parse(JSON.stringify(value)) as JSONValue;
}

function toJSONRecord(value: unknown): JSONRecord {
  return isRecord(value) ? Object.fromEntries(
    Object.entries(value).map(([key, item]) => [key, toJSONValue(item)]),
  ) : {};
}

function normalizePath(path?: string): string[] {
  if (!path) return [];
  return path
    .replace(/^\$\./, '')
    .split('.')
    .map((part) => part.trim())
    .filter(Boolean);
}

function readPath(source: unknown, path?: string): unknown {
  const parts = normalizePath(path);
  if (parts.length === 0) return source;
  let current = source;
  for (const part of parts) {
    if (!isRecord(current)) return undefined;
    current = current[part];
  }
  return current;
}

function resolveBinding(runtime: RuntimeContextValue, bindingId?: string): PageFunctionBinding | undefined {
  if (!bindingId) return undefined;
  return runtime.bindings.get(bindingId);
}

function stateKeyForBinding(binding: PageFunctionBinding): string {
  const output = binding.outputMapping;
  if (isRecord(output) && typeof output.stateKey === 'string' && output.stateKey.trim()) {
    return output.stateKey.trim();
  }
  return binding.id;
}

function resultData(result: PageExecutionResult): unknown {
  return result.data ?? null;
}

function sourceForBinding(runtime: RuntimeContextValue, bindingId?: string): unknown {
  if (!bindingId) return undefined;
  return runtime.state[bindingId] ?? runtime.state[DEFAULT_RESULT_STATE_KEY];
}

function toTableRows(items: unknown): JSONRecord[] {
  if (!Array.isArray(items)) return [];
  return items.map((item, index) => {
    if (isRecord(item)) {
      return { __rowIndex: index, ...toJSONRecord(item) };
    }
    return { __rowIndex: index, value: toJSONValue(item) } as JSONRecord;
  });
}

function normalizeColumns(columns?: PageColumn[]): ColumnsType<JSONRecord> {
  if (!columns || columns.length === 0) return [];
  return columns.map((column) => ({
    title: column.title || column.dataIndex,
    dataIndex: column.dataIndex,
    key: column.key || column.dataIndex,
    render: (value: unknown) => {
      if (isRecord(value) || Array.isArray(value)) {
        return <Typography.Text code>{JSON.stringify(value)}</Typography.Text>;
      }
      return value == null ? '-' : String(value);
    },
  }));
}

function normalizeColumnsFromPath(value: unknown): ColumnsType<JSONRecord> {
  if (!Array.isArray(value)) return [];
  const columns: PageColumn[] = value
    .map((item): PageColumn | null => isRecord(item) ? {
      title: String(item.title || item.dataIndex || item.key || ''),
      dataIndex: String(item.dataIndex || item.key || ''),
      key: String(item.key || item.dataIndex || ''),
    } : null)
    .filter((item): item is PageColumn => item !== null && item.dataIndex !== '');
  return normalizeColumns(columns);
}

function applyInputMapping(mapping: JSONValue | undefined, sources: Record<string, unknown>): JSONValue {
  if (mapping === undefined || mapping === null) {
    return toJSONValue(sources.values ?? {});
  }
  if (!isRecord(mapping)) {
    throw new Error('inputMapping 必须是对象');
  }
  const payload: JSONRecord = {};
  for (const [targetKey, sourcePath] of Object.entries(mapping)) {
    if (typeof sourcePath !== 'string' || sourcePath.trim() === '') {
      throw new Error(`inputMapping.${targetKey} 必须是路径字符串`);
    }
    const [root, ...rest] = normalizePath(sourcePath);
    const rootValue = sources[root];
    payload[targetKey] = toJSONValue(readPath(rootValue, rest.join('.')));
  }
  return payload;
}

function isDangerousRisk(risk?: string) {
  return risk === 'high' || risk === 'danger';
}

function storeExecutionResult(
  runtime: RuntimeContextValue,
  binding: PageFunctionBinding,
  result: PageExecutionResult,
) {
  runtime.setStateValue(binding.id, result);
  runtime.setStateValue(stateKeyForBinding(binding), result);
  runtime.setStateValue(DEFAULT_RESULT_STATE_KEY, result);
}

async function executeBinding(
  runtime: RuntimeContextValue,
  bindingId: string | undefined,
  payload: JSONValue,
): Promise<PageExecutionResult> {
  const binding = resolveBinding(runtime, bindingId);
  if (!binding) {
    throw new Error(`页面 binding 不存在：${bindingId || '-'}`);
  }
  const result = await runtime.execute(binding.id, payload);
  storeExecutionResult(runtime, binding, result);
  return result;
}

/** ConsolePage - 页面根容器 */
const ConsolePage: React.FC<{
  pageKey?: string;
  resourceKey?: string;
  children?: React.ReactNode;
}> = ({ pageKey, resourceKey, children }) => (
  <Space direction="vertical" size="middle" style={{ width: '100%' }} data-page-key={pageKey} data-resource-key={resourceKey}>
    {children}
  </Space>
);

/** QueryForm - 查询/独立表单区域 */
const QueryForm: React.FC<{
  bindingId?: string;
  inputMapping?: JSONValue;
  resultStateKey?: string;
  children?: React.ReactNode;
}> = ({ bindingId, inputMapping, resultStateKey, children }) => {
  const form = useForm();
  const runtime = useRuntimeContext();
  const { message } = App.useApp();
  const [submitting, setSubmitting] = useState(false);

  const submit = async () => {
    setSubmitting(true);
    try {
      await form.submit();
      const payload = applyInputMapping(inputMapping, { values: form.values });
      const result = await executeBinding(runtime, bindingId, payload);
      if (resultStateKey) {
        runtime.setStateValue(resultStateKey, result);
      }
      message.success(result.kind === 'task' ? '任务已启动' : '执行完成');
    } catch (error) {
      message.error(error instanceof Error ? error.message : '执行失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Card size="small" className="console-query-form">
      <Space direction="vertical" style={{ width: '100%' }}>
        {children}
        <Button type="primary" loading={submitting} onClick={submit}>
          执行
        </Button>
      </Space>
    </Card>
  );
};

/** DataTable - 数据表格区域 */
const DataTable: React.FC<{
  bindingId?: string;
  itemsPath?: string;
  totalPath?: string;
  pageField?: string;
  pageSizeField?: string;
  columns?: PageColumn[];
  columnsPath?: string;
  rowActions?: RowAction[];
}> = ({ bindingId, itemsPath, totalPath, pageField, pageSizeField, columns, columnsPath, rowActions }) => {
  const runtime = useRuntimeContext();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<JSONRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [current, setCurrent] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [tableColumns, setTableColumns] = useState<ColumnsType<JSONRecord>>(normalizeColumns(columns));
  const [error, setError] = useState<string>();

  const runQuery = async (page = current, size = pageSize) => {
    if (!bindingId || !itemsPath || !totalPath || !pageField || !pageSizeField) {
      setError('DataTable 必须显式配置 bindingId/itemsPath/totalPath/pageField/pageSizeField');
      return;
    }
    if (!columns?.length && !columnsPath) {
      setError('DataTable 必须显式配置 columns 或 columnsPath');
      return;
    }
    setLoading(true);
    setError(undefined);
    try {
      const payload: JSONRecord = {
        [pageField]: page,
        [pageSizeField]: size,
      };
      const result = await executeBinding(runtime, bindingId, payload);
      const data = resultData(result);
      const nextRows = toTableRows(readPath(data, itemsPath));
      const nextTotal = readPath(data, totalPath);
      const columnsFromSpec = normalizeColumns(columns);
      const nextColumns = columnsFromSpec.length > 0
        ? columnsFromSpec
        : normalizeColumnsFromPath(readPath(data, columnsPath));
      setRows(nextRows);
      setTotal(typeof nextTotal === 'number' ? nextTotal : nextRows.length);
      setCurrent(page);
      setPageSize(size);
      setTableColumns(nextColumns);
    } catch (err) {
      setError(err instanceof Error ? err.message : '查询失败');
    } finally {
      setLoading(false);
    }
  };

  const runAction = (action: RowAction, row: JSONRecord) => {
    const execute = async () => {
      try {
        const payload = applyInputMapping(action.inputMapping, { row, selection: [] });
        const result = await executeBinding(runtime, action.bindingId, payload);
        if (result.kind === 'approval') {
          message.info(`已进入审批：${result.approvalId || result.requestId}`);
        } else {
          message.success(result.kind === 'task' ? '任务已启动' : '操作完成');
        }
        await runQuery(current, pageSize);
      } catch (err) {
        message.error(err instanceof Error ? err.message : '操作失败');
      }
    };

    if (isDangerousRisk(action.risk)) {
      modal.confirm({
        title: '确认执行高风险操作',
        content: `binding ${action.bindingId} 风险等级为 ${action.risk}`,
        okText: '确认执行',
        okButtonProps: { danger: true },
        onOk: execute,
      });
      return;
    }
    execute();
  };

  const mergedColumns = useMemo(() => {
    const actionColumn: ColumnsType<JSONRecord> = rowActions && rowActions.length > 0 ? [{
      title: '操作',
      key: '__actions',
      render: (_, row) => (
        <Space>
          {rowActions.map((action) => (
            <Button key={action.bindingId} size="small" onClick={() => runAction(action, row)}>
              {action.label || action.bindingId}
            </Button>
          ))}
        </Space>
      ),
    }] : [];
    return [...tableColumns, ...actionColumn];
  }, [rowActions, tableColumns]);

  return (
    <Card
      size="small"
      className="console-data-table"
      extra={
        <Button size="small" loading={loading} onClick={() => runQuery(1, pageSize)}>
          刷新
        </Button>
      }
    >
      {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 12 }} />}
      <Table<JSONRecord>
        rowKey={(row) => String(row.id ?? row.key ?? row.__rowIndex)}
        loading={loading}
        columns={mergedColumns}
        dataSource={rows}
        pagination={{
          current,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: runQuery,
        }}
      />
    </Card>
  );
};

/** DetailPanel - 详情面板 */
const DetailPanel: React.FC<{
  bindingId?: string;
  stateKey?: string;
  dataPath?: string;
}> = ({ bindingId, stateKey, dataPath }) => {
  const runtime = useRuntimeContext();
  const source = stateKey ? runtime.state[stateKey] : sourceForBinding(runtime, bindingId);
  const value = dataPath ? readPath(resultData(source as PageExecutionResult), dataPath) : source;
  return (
    <Card size="small" title="详情" className="console-detail-panel">
      <Typography.Text code>{JSON.stringify(value ?? {}, null, 2)}</Typography.Text>
    </Card>
  );
};

/** ActionButton - 操作按钮 */
const ActionButton: React.FC<{
  bindingId: string;
  label?: string;
  risk?: string;
  inputMapping?: JSONValue;
}> = ({ bindingId, label, risk, inputMapping }) => {
  const runtime = useRuntimeContext();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);

  const execute = async () => {
    setLoading(true);
    try {
      const payload = applyInputMapping(inputMapping, { values: {} });
      const result = await executeBinding(runtime, bindingId, payload);
      if (result.kind === 'approval') {
        message.info(`已进入审批：${result.approvalId || result.requestId}`);
      } else {
        message.success(result.kind === 'task' ? '任务已启动' : '操作完成');
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '操作失败');
    } finally {
      setLoading(false);
    }
  };

  const click = () => {
    if (isDangerousRisk(risk)) {
      modal.confirm({
        title: '确认执行高风险操作',
        content: `binding ${bindingId} 风险等级为 ${risk}`,
        okText: '确认执行',
        okButtonProps: { danger: true },
        onOk: execute,
      });
      return;
    }
    execute();
  };

  return (
    <Button danger={isDangerousRisk(risk)} loading={loading} onClick={click}>
      {label || bindingId}
    </Button>
  );
};

/** ActionGroup - 操作组 */
const ActionGroup: React.FC<{
  actions?: RowAction[];
  children?: React.ReactNode;
}> = ({ actions, children }) => (
  <Space className="console-action-group">
    {actions?.map((action) => (
      <ActionButton key={action.bindingId} {...action} />
    ))}
    {children}
  </Space>
);

/** ResultPanel - 结果面板 */
const ResultPanel: React.FC<{
  bindingId?: string;
  stateKey?: string;
  dataPath?: string;
  children?: React.ReactNode;
}> = ({ bindingId, stateKey, dataPath, children }) => {
  const runtime = useRuntimeContext();
  const source = stateKey ? runtime.state[stateKey] : sourceForBinding(runtime, bindingId);
  const value = dataPath ? readPath(resultData(source as PageExecutionResult), dataPath) : source;
  return (
    <Card size="small" title="执行结果" className="console-result-panel">
      {children}
      <Typography.Text code>{JSON.stringify(value ?? {}, null, 2)}</Typography.Text>
    </Card>
  );
};

/** TaskTimeline - 任务时间线 */
const TaskTimeline: React.FC<{
  bindingId?: string;
  stateKey?: string;
}> = ({ bindingId, stateKey }) => {
  const runtime = useRuntimeContext();
  const source = stateKey ? runtime.state[stateKey] : sourceForBinding(runtime, bindingId);
  const taskId = isRecord(source) && typeof source.taskId === 'string' ? source.taskId : '';
  const approvalId = isRecord(source) && typeof source.approvalId === 'string' ? source.approvalId : '';
  return (
    <Card size="small" title="任务状态" className="console-task-timeline">
      <Timeline
        items={[
          {
            color: taskId || approvalId ? 'green' : 'gray',
            children: taskId
              ? `任务已创建：${taskId}`
              : approvalId
                ? `等待审批：${approvalId}`
                : '等待任务启动',
          },
        ]}
      />
    </Card>
  );
};

/** ChartPanel - 报表数据面板 */
const ChartPanel: React.FC<{
  bindingId?: string;
  stateKey?: string;
  dataPath?: string;
}> = ({ bindingId, stateKey, dataPath }) => {
  const runtime = useRuntimeContext();
  const source = stateKey ? runtime.state[stateKey] : sourceForBinding(runtime, bindingId);
  const value = dataPath ? readPath(resultData(source as PageExecutionResult), dataPath) : source;
  return (
    <Card size="small" title="报表数据" className="console-chart-panel">
      <Alert
        type="info"
        showIcon
        message="ChartPanel 最小实现"
        description="当前仅展示 PageSpec 明确 dataPath 指向的数据；图表类型需要在 PageSpec 中显式配置。"
        style={{ marginBottom: 12 }}
      />
      <Typography.Text code>{JSON.stringify(value ?? {}, null, 2)}</Typography.Text>
    </Card>
  );
};

export interface FormilyPageRendererProps {
  page: PageSpec | PublishedPageSpec;
  loading?: boolean;
  error?: string;
  onExecute: (bindingId: string, payload: JSONValue) => Promise<PageExecutionResult> | PageExecutionResult;
}

const SchemaField = createSchemaField({
  components: {
    ConsolePage,
    QueryForm,
    DataTable,
    DetailPanel,
    ActionButton,
    ActionGroup,
    ResultPanel,
    TaskTimeline,
    ChartPanel,
  },
});

const FormilyPageRenderer: React.FC<FormilyPageRendererProps> = ({
  page,
  loading = false,
  error,
  onExecute,
}) => {
  const form = useMemo(() => createForm(), []);
  const [state, setState] = useState<PageState>({});

  const schema = useMemo(() => {
    if (!page?.schema) return null;
    if (typeof page.schema === 'string') {
      try {
        return JSON.parse(page.schema) as Record<string, unknown>;
      } catch {
        return null;
      }
    }
    return page.schema;
  }, [page?.schema]);

  const bindings = useMemo(() => {
    const map = new Map<string, PageFunctionBinding>();
    page.bindings?.forEach((binding) => {
      map.set(binding.id, binding);
    });
    return map;
  }, [page.bindings]);

  const runtime = useMemo<RuntimeContextValue>(
    () => ({
      page,
      bindings,
      state,
      execute: (bindingId, payload) => Promise.resolve(onExecute(bindingId, payload)),
      setStateValue: (key, value) => {
        setState((prev) => ({ ...prev, [key]: value }));
      },
    }),
    [bindings, onExecute, page, state],
  );

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '48px 0' }}>
        <Spin size="large" tip="加载页面中..." />
      </div>
    );
  }

  if (error) {
    return <Alert type="error" message="页面加载失败" description={error} showIcon />;
  }

  if (!schema) {
    return (
      <Alert
        type="warning"
        message="页面配置无效"
        description="PageSpec schema 缺失或格式错误"
        showIcon
      />
    );
  }

  return (
    <RuntimeContext.Provider value={runtime}>
      <div className="formily-page-renderer">
        <FormProvider form={form}>
          <SchemaField schema={schema} />
        </FormProvider>
      </div>
    </RuntimeContext.Provider>
  );
};

export default FormilyPageRenderer;

export {
  ConsolePage,
  QueryForm,
  DataTable,
  DetailPanel,
  ActionButton,
  ActionGroup,
  ResultPanel,
  TaskTimeline,
  ChartPanel,
};
