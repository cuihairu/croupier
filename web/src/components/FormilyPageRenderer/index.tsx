/**
 * FormilyPageRenderer - PageSpec Formily 渲染器
 *
 * 运行控制台唯一页面渲染入口。页面结构来自 PageSpec.schema，函数执行由运行时
 * context 提供，不读取旧 layout 协议。
 */

import React, { createContext, useContext, useMemo, useState } from 'react';
import { createForm } from '@formily/core';
import { createSchemaField, FormProvider, useForm } from '@formily/react';
import { Alert, App, Button, Card, Space, Spin, Table, Timeline, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import type { PageSpec, PublishedPageSpec } from '@/types/dashboard';

type JSONRecord = Record<string, unknown>;

type RuntimeResult = {
  functionId: string;
  role: 'query' | 'action' | 'task';
  data: unknown;
};

type RuntimeContextValue = {
  page: PageSpec | PublishedPageSpec;
  lastResult?: RuntimeResult;
  setLastResult: (result: RuntimeResult) => void;
  onQuery?: (functionId: string, values: JSONRecord) => Promise<unknown> | unknown;
  onAction?: (functionId: string, payload?: unknown) => Promise<unknown> | unknown;
  onTaskStart?: (functionId: string, payload?: unknown) => Promise<unknown> | unknown;
};

type PaginationConfig = {
  pageField?: string;
  pageSizeField?: string;
  totalField?: string;
  itemsField?: string;
};

type RowAction = {
  functionId: string;
  label: string;
  risk?: string;
};

const RuntimeContext = createContext<RuntimeContextValue | null>(null);

function useRuntimeContext() {
  const value = useContext(RuntimeContext);
  if (!value) {
    throw new Error('FormilyPageRenderer runtime context is missing');
  }
  return value;
}

function isRecord(value: unknown): value is JSONRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function recordFromUnknown(value: unknown): JSONRecord {
  return isRecord(value) ? value : {};
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

function wrapFunctionResponse(response: unknown) {
  const record = recordFromUnknown(response);
  const result = record.result ?? response;
  return {
    response: result,
    result,
    raw: response,
  };
}

function toTableRows(items: unknown): JSONRecord[] {
  if (!Array.isArray(items)) return [];
  return items.map((item, index) => {
    if (isRecord(item)) {
      return { __rowIndex: index, ...item };
    }
    return { __rowIndex: index, value: item };
  });
}

function buildColumns(rows: JSONRecord[], rowActions: RowAction[] | undefined, runAction: (action: RowAction, row: JSONRecord) => void): ColumnsType<JSONRecord> {
  const keys = new Set<string>();
  rows.slice(0, 10).forEach((row) => {
    Object.keys(row).forEach((key) => {
      if (!key.startsWith('__')) keys.add(key);
    });
  });

  const columns: ColumnsType<JSONRecord> = Array.from(keys).map((key) => ({
    title: key,
    dataIndex: key,
    key,
    render: (value: unknown) => {
      if (isRecord(value) || Array.isArray(value)) {
        return <Typography.Text code>{JSON.stringify(value)}</Typography.Text>;
      }
      return value == null ? '-' : String(value);
    },
  }));

  if (rowActions && rowActions.length > 0) {
    columns.push({
      title: '操作',
      key: '__actions',
      render: (_, row) => (
        <Space>
          {rowActions.map((action) => (
            <Button key={action.functionId} size="small" onClick={() => runAction(action, row)}>
              {action.label || action.functionId}
            </Button>
          ))}
        </Space>
      ),
    });
  }

  return columns.length > 0 ? columns : [{ title: '结果', dataIndex: 'value', key: 'value' }];
}

function isDangerousRisk(risk?: string) {
  return risk === 'high' || risk === 'danger';
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
  functionId?: string;
  formSchemaRef?: string;
  children?: React.ReactNode;
}> = ({ functionId, children }) => {
  const form = useForm();
  const runtime = useRuntimeContext();
  const { message } = App.useApp();
  const [submitting, setSubmitting] = useState(false);

  const submit = async () => {
    if (!functionId) {
      message.error('QueryForm 缺少 functionId');
      return;
    }
    setSubmitting(true);
    try {
      await form.submit();
      const values = recordFromUnknown(form.values);
      const isTaskPage = runtime.page.type === 'task';
      const result = isTaskPage
        ? await runtime.onTaskStart?.(functionId, values)
        : await runtime.onQuery?.(functionId, values);
      runtime.setLastResult({
        functionId,
        role: isTaskPage ? 'task' : 'query',
        data: result,
      });
      message.success(isTaskPage ? '任务已启动' : '查询完成');
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
          {runtime.page.type === 'task' ? '启动任务' : '执行'}
        </Button>
      </Space>
    </Card>
  );
};

/** DataTable - 数据表格区域 */
const DataTable: React.FC<{
  queryFunctionId?: string;
  pagination?: PaginationConfig;
  rowActions?: RowAction[];
}> = ({ queryFunctionId, pagination, rowActions }) => {
  const runtime = useRuntimeContext();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<JSONRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [current, setCurrent] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [error, setError] = useState<string>();

  const runQuery = async (page = current, size = pageSize) => {
    if (!queryFunctionId) {
      setError('DataTable 缺少 queryFunctionId');
      return;
    }
    if (!pagination?.itemsField || !pagination?.totalField) {
      setError('DataTable 必须显式配置 pagination.itemsField 和 pagination.totalField');
      return;
    }
    setLoading(true);
    setError(undefined);
    try {
      const payload: JSONRecord = {};
      if (pagination.pageField) payload[pagination.pageField] = page;
      if (pagination.pageSizeField) payload[pagination.pageSizeField] = size;
      const result = await runtime.onQuery?.(queryFunctionId, payload);
      const wrapped = wrapFunctionResponse(result);
      const nextRows = toTableRows(readPath(wrapped, pagination.itemsField));
      const nextTotal = readPath(wrapped, pagination.totalField);
      setRows(nextRows);
      setTotal(typeof nextTotal === 'number' ? nextTotal : nextRows.length);
      setCurrent(page);
      setPageSize(size);
      runtime.setLastResult({ functionId: queryFunctionId, role: 'query', data: result });
    } catch (err) {
      setError(err instanceof Error ? err.message : '查询失败');
    } finally {
      setLoading(false);
    }
  };

  const runAction = (action: RowAction, row: JSONRecord) => {
    const execute = async () => {
      try {
        const result = await runtime.onAction?.(action.functionId, row);
        runtime.setLastResult({ functionId: action.functionId, role: 'action', data: result });
        message.success('操作完成');
        await runQuery(current, pageSize);
      } catch (err) {
        message.error(err instanceof Error ? err.message : '操作失败');
      }
    };

    if (isDangerousRisk(action.risk)) {
      modal.confirm({
        title: '确认执行高风险操作',
        content: `函数 ${action.functionId} 风险等级为 ${action.risk}`,
        okText: '确认执行',
        okButtonProps: { danger: true },
        onOk: execute,
      });
      return;
    }
    execute();
  };

  const columns = useMemo(() => buildColumns(rows, rowActions, runAction), [rows, rowActions]);

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
        columns={columns}
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
const DetailPanel: React.FC<{ functionId?: string }> = ({ functionId }) => {
  const runtime = useRuntimeContext();
  const matched = runtime.lastResult?.functionId === functionId ? runtime.lastResult.data : runtime.lastResult?.data;
  return (
    <Card size="small" title="详情" className="console-detail-panel">
      <Typography.Paragraph>
        <Typography.Text type="secondary">functionId: {functionId || '-'}</Typography.Text>
      </Typography.Paragraph>
      <Typography.Text code>{JSON.stringify(matched ?? {}, null, 2)}</Typography.Text>
    </Card>
  );
};

/** ActionButton - 操作按钮 */
const ActionButton: React.FC<{
  functionId: string;
  label?: string;
  risk?: string;
  placement?: string;
}> = ({ functionId, label, risk }) => {
  const runtime = useRuntimeContext();
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);

  const execute = async () => {
    setLoading(true);
    try {
      const result = await runtime.onAction?.(functionId);
      runtime.setLastResult({ functionId, role: 'action', data: result });
      message.success('操作完成');
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
        content: `函数 ${functionId} 风险等级为 ${risk}`,
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
      {label || functionId}
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
      <ActionButton key={action.functionId} {...action} />
    ))}
    {children}
  </Space>
);

/** ResultPanel - 结果面板 */
const ResultPanel: React.FC<{ children?: React.ReactNode }> = ({ children }) => {
  const runtime = useRuntimeContext();
  return (
    <Card size="small" title="执行结果" className="console-result-panel">
      {children}
      <Typography.Text code>{JSON.stringify(runtime.lastResult?.data ?? {}, null, 2)}</Typography.Text>
    </Card>
  );
};

/** TaskTimeline - 任务时间线 */
const TaskTimeline: React.FC<{ taskId?: string }> = ({ taskId }) => {
  const runtime = useRuntimeContext();
  const result = recordFromUnknown(runtime.lastResult?.data);
  const currentTaskId = taskId || String(result.taskId || result.taskID || '');
  return (
    <Card size="small" title="任务状态" className="console-task-timeline">
      <Timeline
        items={[
          { color: currentTaskId ? 'green' : 'gray', children: currentTaskId ? `任务已创建：${currentTaskId}` : '等待任务启动' },
        ]}
      />
    </Card>
  );
};

/** ChartPanel - 图表面板 */
const ChartPanel: React.FC<{ dataSource?: string }> = ({ dataSource }) => {
  const runtime = useRuntimeContext();
  const value = dataSource ? readPath(wrapFunctionResponse(runtime.lastResult?.data), dataSource) : runtime.lastResult?.data;
  return (
    <Card size="small" title="报表数据" className="console-chart-panel">
      <Alert
        type="info"
        showIcon
        message="ChartPanel 最小实现"
        description="当前仅展示 PageSpec 明确 dataSource 指向的数据；图表类型需要后续在 PageSpec 中显式配置。"
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
  onQuery?: (functionId: string, values: JSONRecord) => Promise<unknown> | unknown;
  onAction?: (functionId: string, payload?: unknown) => Promise<unknown> | unknown;
  onTaskStart?: (functionId: string, payload?: unknown) => Promise<unknown> | unknown;
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
  onQuery,
  onAction,
  onTaskStart,
}) => {
  const form = useMemo(() => createForm(), []);
  const [lastResult, setLastResult] = useState<RuntimeResult>();

  const schema = useMemo(() => {
    if (!page?.schema) return null;
    if (typeof page.schema === 'string') {
      try {
        return JSON.parse(page.schema) as JSONRecord;
      } catch {
        return null;
      }
    }
    return page.schema;
  }, [page?.schema]);

  const runtime = useMemo<RuntimeContextValue>(
    () => ({ page, lastResult, setLastResult, onQuery, onAction, onTaskStart }),
    [page, lastResult, onQuery, onAction, onTaskStart],
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
