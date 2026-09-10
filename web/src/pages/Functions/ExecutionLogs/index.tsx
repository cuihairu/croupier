import React, { useMemo, useRef, useState } from 'react';
import { Card, Drawer, Input, Select, Space, Tag, DatePicker, Button, Typography } from 'antd';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { ReloadOutlined } from '@ant-design/icons';
import {
  getExecutionLog,
  listExecutionLogs,
  type ExecutionLogDetail,
  type ExecutionLogItem,
} from '@/services/api/executionLogs';
import { FormattedMessage, useIntl } from '@umijs/max';
import { formatDateTime } from '@/utils/format';

const { Text } = Typography;
const { RangePicker } = DatePicker;

const PAGE_SIZE = 20;

const preStyle: React.CSSProperties = {
  whiteSpace: 'pre-wrap',
  background: '#fafafa',
  padding: 12,
  borderRadius: 6,
  maxHeight: 320,
  overflow: 'auto',
  fontSize: 12,
};

function toLocalInput(value: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())} ${pad(
    value.getHours(),
  )}:${pad(value.getMinutes())}:${pad(value.getSeconds())}`;
}

/** 执行留痕（管理员审计视角）：全量执行记录按用户/函数/来源/状态/时间过滤。 */
export default function ExecutionLogsPage() {
  const intl = useIntl();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [loadError, setLoadError] = useState('');

  const [actor, setActor] = useState('');
  const [functionId, setFunctionId] = useState('');
  const [source, setSource] = useState('');
  const [status, setStatus] = useState('');
  const [traceId, setTraceId] = useState('');
  const [range, setRange] = useState<[Date | null, Date | null] | null>(null);

  const [detail, setDetail] = useState<ExecutionLogDetail | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const columns: ProColumns<ExecutionLogItem>[] = [
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.time',
        defaultMessage: '时间',
      }),
      dataIndex: 'createdAt',
      width: 165,
      render: (_, r) => formatDateTime(r.createdAt),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.actor',
        defaultMessage: '申请人',
      }),
      dataIndex: 'actor',
      width: 120,
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.function',
        defaultMessage: '函数',
      }),
      dataIndex: 'functionId',
      ellipsis: true,
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.gameEnv',
        defaultMessage: '游戏/环境',
      }),
      width: 150,
      render: (_, r) => `${r.gameId}/${r.env}`,
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.source',
        defaultMessage: '来源',
      }),
      dataIndex: 'source',
      width: 80,
      render: (_, r) =>
        r.source === 'page' ? (
          <Tag>
            <FormattedMessage
              id="pages.functionsExecutionLogs.sourceLabel.page"
              defaultMessage="页面"
            />
          </Tag>
        ) : (
          <Tag>
            <FormattedMessage
              id="pages.functionsExecutionLogs.sourceLabel.invoke"
              defaultMessage="调用"
            />
          </Tag>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      width: 80,
      render: (_, r) => (
        <Tag color={r.status === 'ok' ? 'green' : 'red'} style={{ marginInlineEnd: 0 }}>
          {r.status === 'ok'
            ? intl.formatMessage({
                id: 'pages.functionsExecutionLogs.statusLabel.ok',
                defaultMessage: '成功',
              })
            : intl.formatMessage({
                id: 'pages.functionsExecutionLogs.statusLabel.error',
                defaultMessage: '失败',
              })}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.duration',
        defaultMessage: '耗时(ms)',
      }),
      dataIndex: 'durationMs',
      width: 90,
    },
  ];

  const viewDetail = useMemo(
    () => async (id: number) => {
      try {
        setDetail(await getExecutionLog(id));
        setDetailOpen(true);
      } catch {
        /* 详情加载失败静默：可重开 */
      }
    },
    [],
  );

  return (
    <Card
      title={intl.formatMessage({
        id: 'pages.functionsExecutionLogs.page.title',
        defaultMessage: '执行留痕',
      })}
      extra={
        <Button icon={<ReloadOutlined />} onClick={() => actionRef.current?.reload()}>
          <FormattedMessage
            id="pages.functionsExecutionLogs.action.refresh"
            defaultMessage="刷新"
          />
        </Button>
      }
    >
      <Space style={{ marginBottom: 16 }} wrap>
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.functionsExecutionLogs.filter.actor',
            defaultMessage: '申请人',
          })}
          value={actor}
          onChange={(e) => {
            setActor(e.target.value);
            // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
            // ProTable 内部 debounce + abort 合并，不会出现错序数据
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
          style={{ width: 140 }}
          allowClear
        />
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.functionsExecutionLogs.filter.functionId',
            defaultMessage: '函数ID',
          })}
          value={functionId}
          onChange={(e) => {
            setFunctionId(e.target.value);
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
          style={{ width: 200 }}
          allowClear
        />
        <Select
          placeholder={intl.formatMessage({
            id: 'pages.functionsExecutionLogs.filter.source',
            defaultMessage: '来源',
          })}
          style={{ width: 110 }}
          value={source || undefined}
          onChange={(v) => {
            setSource(v || '');
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
          allowClear
          options={[
            {
              label: intl.formatMessage({
                id: 'pages.functionsExecutionLogs.sourceLabel.invoke',
                defaultMessage: '调用',
              }),
              value: 'invoke',
            },
            {
              label: intl.formatMessage({
                id: 'pages.functionsExecutionLogs.sourceLabel.page',
                defaultMessage: '页面',
              }),
              value: 'page',
            },
          ]}
        />
        <Select
          placeholder={intl.formatMessage({
            id: 'pages.functionsExecutionLogs.filter.status',
            defaultMessage: '状态',
          })}
          style={{ width: 110 }}
          value={status || undefined}
          onChange={(v) => {
            setStatus(v || '');
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
          allowClear
          options={[
            {
              label: intl.formatMessage({
                id: 'pages.functionsExecutionLogs.statusLabel.ok',
                defaultMessage: '成功',
              }),
              value: 'ok',
            },
            {
              label: intl.formatMessage({
                id: 'pages.functionsExecutionLogs.statusLabel.error',
                defaultMessage: '失败',
              }),
              value: 'error',
            },
          ]}
        />
        <Input
          placeholder="Trace ID"
          value={traceId}
          onChange={(e) => {
            setTraceId(e.target.value);
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
          style={{ width: 200 }}
          allowClear
        />
        <RangePicker
          showTime
          onChange={(dates) => {
            setRange(dates ? [dates[0]?.toDate() ?? null, dates[1]?.toDate() ?? null] : null);
            actionRef.current?.setPageInfo?.({ current: 1 });
          }}
        />
        <Button type="primary" onClick={() => actionRef.current?.reload()}>
          <FormattedMessage id="pages.functionsExecutionLogs.action.search" defaultMessage="查询" />
        </Button>
      </Space>

      {loadError ? (
        <div style={{ marginBottom: 16 }}>
          <AlertMessage message={loadError} onRetry={() => actionRef.current?.reload()} />
        </div>
      ) : null}
      <ProTable<ExecutionLogItem>
        actionRef={actionRef}
        rowKey="id"
        size="small"
        columns={columns}
        search={false}
        options={false}
        toolBarRender={false}
        params={{ actor, functionId, source, status, traceId, range }}
        request={async ({
          current = 1,
          pageSize = PAGE_SIZE,
          actor: actorFilter = '',
          functionId: functionIdFilter = '',
          source: sourceFilter = '',
          status: statusFilter = '',
          traceId: traceIdFilter = '',
          range: timeRange = null,
        }) => {
          setLoadError('');
          const params: Record<string, string | number> = { page: current, pageSize };
          if (actorFilter.trim()) params.actor = actorFilter.trim();
          if (functionIdFilter.trim()) params.functionId = functionIdFilter.trim();
          if (sourceFilter) params.source = sourceFilter;
          if (statusFilter) params.status = statusFilter;
          if (traceIdFilter.trim()) params.traceId = traceIdFilter.trim();
          if (timeRange?.[0]) params.from = toLocalInput(timeRange[0]);
          if (timeRange?.[1]) params.to = toLocalInput(timeRange[1]);
          try {
            const json = await listExecutionLogs(params);
            return { data: json.items || [], total: json.total || 0, success: true };
          } catch (e) {
            setLoadError(
              e instanceof Error
                ? e.message
                : intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.error.loadFailed',
                    defaultMessage: '加载失败',
                  }),
            );
            return { data: [], total: 0, success: false };
          }
        }}
        pagination={{
          pageSize: PAGE_SIZE,
          showSizeChanger: true,
          showTotal: (t) =>
            intl.formatMessage(
              { id: 'pages.functionsExecutionLogs.pagination.total', defaultMessage: `共 ${t} 条` },
              { total: t },
            ),
        }}
        onRow={(record) => ({
          onClick: () => void viewDetail(record.id),
          style: { cursor: 'pointer' },
        })}
      />

      <Drawer
        title={
          detail
            ? intl.formatMessage(
                {
                  id: 'pages.functionsExecutionLogs.detail.title',
                  defaultMessage: `执行留痕 #${detail.id}`,
                },
                { id: detail.id },
              )
            : ''
        }
        width={720}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
      >
        {detail && (
          <>
            <Space orientation="vertical" size={4} style={{ width: '100%', marginBottom: 12 }}>
              <Text>
                <Text type="secondary">
                  <FormattedMessage
                    id="pages.functionsExecutionLogs.detail.label.actor"
                    defaultMessage="申请人："
                  />
                </Text>
                {detail.actor}
              </Text>
              <Text>
                <Text type="secondary">
                  <FormattedMessage
                    id="pages.functionsExecutionLogs.detail.label.function"
                    defaultMessage="函数："
                  />
                </Text>
                {detail.functionId}
              </Text>
              <Text>
                <Text type="secondary">
                  <FormattedMessage
                    id="pages.functionsExecutionLogs.detail.label.source"
                    defaultMessage="来源："
                  />
                </Text>
                {detail.source === 'page'
                  ? intl.formatMessage(
                      {
                        id: 'pages.functionsExecutionLogs.detail.sourcePage',
                        defaultMessage: `页面（${detail.pageKey} / ${detail.bindingId}）`,
                      },
                      { pageKey: detail.pageKey, bindingId: detail.bindingId },
                    )
                  : intl.formatMessage({
                      id: 'pages.functionsExecutionLogs.sourceLabel.invoke',
                      defaultMessage: '调用',
                    })}
              </Text>
              <Text>
                <Text type="secondary">
                  <FormattedMessage
                    id="pages.functionsExecutionLogs.detail.label.status"
                    defaultMessage="状态："
                  />
                </Text>
                {detail.status === 'ok'
                  ? intl.formatMessage({
                      id: 'pages.functionsExecutionLogs.statusLabel.ok',
                      defaultMessage: '成功',
                    })
                  : intl.formatMessage({
                      id: 'pages.functionsExecutionLogs.statusLabel.error',
                      defaultMessage: '失败',
                    })}
                {detail.truncated
                  ? intl.formatMessage({
                      id: 'pages.functionsExecutionLogs.detail.truncated',
                      defaultMessage: '（载荷已截断）',
                    })
                  : ''}
              </Text>
              <Text>
                <Text type="secondary">
                  <FormattedMessage
                    id="pages.functionsExecutionLogs.detail.label.time"
                    defaultMessage="时间："
                  />
                </Text>
                {formatDateTime(detail.createdAt)} · {detail.durationMs}ms
              </Text>
              {detail.traceId && (
                <Text>
                  <Text type="secondary">Trace：</Text>
                  <Text code>{detail.traceId}</Text>
                </Text>
              )}
            </Space>
            <Text type="secondary">
              <FormattedMessage
                id="pages.functionsExecutionLogs.detail.requestTitle"
                defaultMessage="请求（已脱敏）："
              />
            </Text>
            <pre style={preStyle}>
              {detail.requestPayload
                ? JSON.stringify(detail.requestPayload, null, 2)
                : intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.detail.emptyPayload',
                    defaultMessage: '（无）',
                  })}
            </pre>
            <Text type="secondary">
              <FormattedMessage
                id="pages.functionsExecutionLogs.detail.responseTitle"
                defaultMessage="响应（已脱敏）："
              />
            </Text>
            <pre style={preStyle}>
              {detail.responseBody
                ? JSON.stringify(detail.responseBody, null, 2)
                : intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.detail.emptyPayload',
                    defaultMessage: '（无）',
                  })}
            </pre>
          </>
        )}
      </Drawer>
    </Card>
  );
}

function AlertMessage({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div role="alert">
      <Text type="danger">{message}</Text>
      <Button size="small" icon={<ReloadOutlined />} onClick={onRetry} style={{ marginLeft: 8 }}>
        <FormattedMessage id="pages.functionsExecutionLogs.action.retry" defaultMessage="重试" />
      </Button>
    </div>
  );
}
