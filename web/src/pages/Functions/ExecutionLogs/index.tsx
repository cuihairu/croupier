import React, { useMemo, useRef, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Descriptions,
  Drawer,
  Input,
  Select,
  Space,
  Tag,
  DatePicker,
  Typography,
} from 'antd';
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { DownOutlined, ReloadOutlined, UpOutlined } from '@ant-design/icons';
import {
  getExecutionLog,
  listExecutionLogs,
  type ExecutionLogDetail,
  type ExecutionLogItem,
} from '@/services/api/executionLogs';
import { FormattedMessage, useIntl } from '@umijs/max';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
import { formatDateTime } from '@/utils/format';

const { Text } = Typography;
const { RangePicker } = DatePicker;

const PAGE_SIZE = 20;

const preStyle: React.CSSProperties = {
  whiteSpace: 'pre-wrap',
  background: 'rgba(128, 128, 128, 0.08)',
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

/** 从失败记录的 responseBody 中提取失败原因（写入端约定 responseBody={"error": ...}）。 */
function extractErrorReason(body: unknown): string {
  if (body && typeof body === 'object' && 'error' in (body as Record<string, unknown>)) {
    const err = (body as Record<string, unknown>).error;
    if (typeof err === 'string' && err.trim()) return err;
  }
  if (typeof body === 'string' && body.trim()) return body;
  return '';
}

/** 执行留痕（管理员审计视角）：全量执行记录按操作人/函数/来源/状态/时间过滤。 */
export default function ExecutionLogsPage() {
  const intl = useIntl();
  const { message } = App.useApp();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [loadError, setLoadError] = useState('');
  // 高频筛选默认展示；Trace ID / 时间范围低频，收进「更多筛选」减少常驻占位
  const [advancedOpen, setAdvancedOpen] = useState(false);
  // 服务端分页总数：供概览指标与筛选栏结果计数
  const [total, setTotal] = useState(0);

  const [actor, setActor] = useState('');
  const [functionId, setFunctionId] = useState('');
  const [source, setSource] = useState('');
  const [status, setStatus] = useState('');
  const [traceId, setTraceId] = useState('');
  const [range, setRange] = useState<[Date | null, Date | null] | null>(null);

  const [detail, setDetail] = useState<ExecutionLogDetail | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);

  const backToFirstPage = () => actionRef.current?.setPageInfo?.({ current: 1 });

  const hasFilters = Boolean(
    actor.trim() ||
    functionId.trim() ||
    source ||
    status ||
    traceId.trim() ||
    range?.[0] ||
    range?.[1],
  );

  const filterSummary = useMemo(() => {
    const parts: string[] = [];
    if (actor.trim())
      parts.push(
        intl.formatMessage(
          {
            id: 'pages.functionsExecutionLogs.filter.summaryActor',
            defaultMessage: `操作人 {value}`,
          },
          { value: actor.trim() },
        ),
      );
    if (functionId.trim())
      parts.push(
        intl.formatMessage(
          {
            id: 'pages.functionsExecutionLogs.filter.summaryFunctionId',
            defaultMessage: `函数 {value}`,
          },
          { value: functionId.trim() },
        ),
      );
    if (source)
      parts.push(
        intl.formatMessage({
          id:
            source === 'page'
              ? 'pages.functionsExecutionLogs.sourceLabel.page'
              : 'pages.functionsExecutionLogs.sourceLabel.invoke',
          defaultMessage: source,
        }),
      );
    if (status)
      parts.push(
        intl.formatMessage({
          id:
            status === 'ok'
              ? 'pages.functionsExecutionLogs.statusLabel.ok'
              : 'pages.functionsExecutionLogs.statusLabel.error',
          defaultMessage: status,
        }),
      );
    if (traceId.trim())
      parts.push(
        intl.formatMessage(
          {
            id: 'pages.functionsExecutionLogs.filter.summaryTraceId',
            defaultMessage: `Trace {value}`,
          },
          { value: traceId.trim() },
        ),
      );
    if (range?.[0] || range?.[1]) {
      const from = range[0] ? formatDateTime(range[0].toISOString()) : '…';
      const to = range[1] ? formatDateTime(range[1].toISOString()) : '…';
      parts.push(
        intl.formatMessage(
          {
            id: 'pages.functionsExecutionLogs.filter.summaryTimeRange',
            defaultMessage: `时间 {from} ~ {to}`,
          },
          { from, to },
        ),
      );
    }
    return parts.join(' / ');
  }, [intl, actor, functionId, source, status, traceId, range]);

  const clearFilters = () => {
    setActor('');
    setFunctionId('');
    setSource('');
    setStatus('');
    setTraceId('');
    setRange(null);
    backToFirstPage();
  };

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
        defaultMessage: '操作人',
      }),
      dataIndex: 'actor',
      width: 110,
      render: (_, r) => r.actor || '-',
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.function',
        defaultMessage: '函数',
      }),
      dataIndex: 'functionId',
      width: 220,
      ellipsis: true,
      render: (_, r) => (
        <Text code copyable>
          {r.functionId}
        </Text>
      ),
    },
    {
      // 页面执行的记录展示所属页面 Key，调用链路溯源到发起页面
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.pageKey',
        defaultMessage: '页面 Key',
      }),
      dataIndex: 'pageKey',
      width: 170,
      ellipsis: true,
      render: (_, r) =>
        r.pageKey ? (
          <Text code copyable>
            {r.pageKey}
          </Text>
        ) : (
          '-'
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.gameEnv',
        defaultMessage: '游戏/环境',
      }),
      width: 130,
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
    {
      title: intl.formatMessage({
        id: 'pages.functionsExecutionLogs.column.actions',
        defaultMessage: '操作',
      }),
      key: 'actions',
      width: 70,
      render: (_, r) => (
        <Button type="link" size="small" onClick={() => void viewDetail(r.id)}>
          <FormattedMessage
            id="pages.functionsExecutionLogs.action.viewDetail"
            defaultMessage="详情"
          />
        </Button>
      ),
    },
  ];

  const viewDetail = useMemo(
    () => async (id: number) => {
      try {
        setDetail(await getExecutionLog(id));
        setDetailOpen(true);
      } catch (e) {
        message.error(
          e instanceof Error
            ? e.message
            : intl.formatMessage({
                id: 'pages.functionsExecutionLogs.detail.loadFailedToast',
                defaultMessage: '详情加载失败',
              }),
        );
      }
    },
    [intl, message],
  );

  const errorReason = detail ? extractErrorReason(detail.responseBody) : '';

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.functionsExecutionLogs.title.main',
        defaultMessage: '执行留痕',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.functionsExecutionLogs.title.sub',
        defaultMessage: '按操作人、函数、来源、状态与时间范围排查函数执行记录',
      })}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title={intl.formatMessage({
            id: 'pages.functionsExecutionLogs.summary.title',
            defaultMessage: '执行留痕概览',
          })}
          description={intl.formatMessage({
            id: 'pages.functionsExecutionLogs.summary.description',
            defaultMessage:
              '这里集中查询函数执行的留痕：谁通过调用或页面触发了哪个函数、执行结果与耗时，请求/响应载荷已脱敏。',
          })}
          items={[
            {
              color: '#1677ff',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsExecutionLogs.summary.total',
                  defaultMessage: `记录总数 {count}`,
                },
                { count: total },
              ),
            },
          ]}
          hint={intl.formatMessage({
            id: 'pages.functionsExecutionLogs.summary.hint',
            defaultMessage:
              '推荐路径：先按操作人或状态筛选定位可疑执行，点击行查看已脱敏的请求/响应详情；同一调用链可按 Trace ID 串联。',
          })}
        />

        <StandardListSection
          title={intl.formatMessage({
            id: 'pages.functionsExecutionLogs.list.title',
            defaultMessage: '执行记录',
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
          <StandardFilterBar
            resultText={intl.formatMessage(
              {
                id: 'pages.functionsExecutionLogs.list.resultCount',
                defaultMessage: `当前结果 {count} 条`,
              },
              { count: total },
            )}
            controls={
              <>
                <Input
                  placeholder={intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.filter.actor',
                    defaultMessage: '操作人',
                  })}
                  value={actor}
                  onChange={(e) => {
                    setActor(e.target.value);
                    // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
                    // ProTable 内部 debounce + abort 合并，不会出现错序数据
                    backToFirstPage();
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
                    backToFirstPage();
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
                    backToFirstPage();
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
                    backToFirstPage();
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
                {advancedOpen && (
                  <>
                    <Input
                      placeholder="Trace ID"
                      value={traceId}
                      onChange={(e) => {
                        setTraceId(e.target.value);
                        backToFirstPage();
                      }}
                      style={{ width: 200 }}
                      allowClear
                    />
                    <RangePicker
                      showTime
                      onChange={(dates) => {
                        setRange(
                          dates ? [dates[0]?.toDate() ?? null, dates[1]?.toDate() ?? null] : null,
                        );
                        backToFirstPage();
                      }}
                    />
                  </>
                )}
                <Button type="text" size="small" onClick={() => setAdvancedOpen((v) => !v)}>
                  {advancedOpen ? <UpOutlined /> : <DownOutlined />}
                  <FormattedMessage
                    id={
                      advancedOpen
                        ? 'pages.functionsExecutionLogs.filter.less'
                        : 'pages.functionsExecutionLogs.filter.more'
                    }
                    defaultMessage={advancedOpen ? '收起筛选' : '更多筛选'}
                  />
                </Button>
                {hasFilters && (
                  <Button onClick={clearFilters}>
                    <FormattedMessage
                      id="pages.functionsExecutionLogs.filter.clear"
                      defaultMessage="清空筛选"
                    />
                  </Button>
                )}
                <Button type="primary" onClick={() => actionRef.current?.reload()}>
                  <FormattedMessage
                    id="pages.functionsExecutionLogs.action.search"
                    defaultMessage="查询"
                  />
                </Button>
              </>
            }
          />
          {hasFilters ? (
            <Alert
              style={{ marginBottom: 12 }}
              type="info"
              showIcon
              message={intl.formatMessage({
                id: 'pages.functionsExecutionLogs.filter.activeMessage',
                defaultMessage: '当前正在查看筛选后的执行记录',
              })}
              description={intl.formatMessage(
                {
                  id: 'pages.functionsExecutionLogs.filter.activeDescription',
                  defaultMessage: `已生效条件：{filters}`,
                },
                { filters: filterSummary },
              )}
            />
          ) : null}
          {loadError ? (
            <Alert
              style={{ marginBottom: 12 }}
              type="error"
              showIcon
              message={intl.formatMessage({
                id: 'pages.functionsExecutionLogs.error.loadFailed',
                defaultMessage: '加载失败',
              })}
              description={loadError}
              action={
                <Button size="small" danger onClick={() => actionRef.current?.reload()}>
                  <FormattedMessage
                    id="pages.functionsExecutionLogs.action.retry"
                    defaultMessage="重试"
                  />
                </Button>
              }
            />
          ) : null}
          <ProTable<ExecutionLogItem>
            actionRef={actionRef}
            rowKey="id"
            size="small"
            columns={columns}
            scroll={{ x: 1120 }}
            search={false}
            options={false}
            toolBarRender={false}
            locale={{
              emptyText: hasFilters
                ? intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.empty.filtered',
                    defaultMessage: '当前筛选条件下没有匹配的执行记录，请调整筛选条件后重试。',
                  })
                : intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.empty.none',
                    defaultMessage:
                      '暂无执行留痕。函数被调用或页面发起执行后，记录会出现在这里；可先确认对应函数是否已被调用。',
                  }),
            }}
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
                setTotal(json.total || 0);
                return { data: json.items || [], total: json.total || 0, success: true };
              } catch (e) {
                setTotal(0);
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
                  {
                    id: 'pages.functionsExecutionLogs.pagination.total',
                    defaultMessage: `共 {total} 条`,
                  },
                  { total: t },
                ),
            }}
            onRow={(record) => ({
              onClick: () => void viewDetail(record.id),
              style: { cursor: 'pointer' },
            })}
          />
        </StandardListSection>
      </Space>

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
            <Descriptions
              column={2}
              size="small"
              bordered
              style={{ marginBottom: 16 }}
              items={[
                {
                  key: 'actor',
                  label: intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.column.actor',
                    defaultMessage: '操作人',
                  }),
                  children: detail.actor || '-',
                },
                {
                  key: 'status',
                  label: intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.column.status',
                    defaultMessage: '状态',
                  }),
                  children: (
                    <>
                      <Tag
                        color={detail.status === 'ok' ? 'green' : 'red'}
                        style={{ marginInlineEnd: 0 }}
                      >
                        {detail.status === 'ok'
                          ? intl.formatMessage({
                              id: 'pages.functionsExecutionLogs.statusLabel.ok',
                              defaultMessage: '成功',
                            })
                          : intl.formatMessage({
                              id: 'pages.functionsExecutionLogs.statusLabel.error',
                              defaultMessage: '失败',
                            })}
                      </Tag>
                      {detail.truncated
                        ? intl.formatMessage({
                            id: 'pages.functionsExecutionLogs.detail.truncated',
                            defaultMessage: '（载荷已截断）',
                          })
                        : ''}
                    </>
                  ),
                },
                {
                  key: 'function',
                  label: intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.column.function',
                    defaultMessage: '函数',
                  }),
                  children: (
                    <Text code copyable>
                      {detail.functionId}
                    </Text>
                  ),
                },
                {
                  key: 'gameEnv',
                  label: intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.column.gameEnv',
                    defaultMessage: '游戏/环境',
                  }),
                  children: `${detail.gameId}/${detail.env}`,
                },
                {
                  key: 'source',
                  label: intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.column.source',
                    defaultMessage: '来源',
                  }),
                  span: 2,
                  children:
                    detail.source === 'page'
                      ? intl.formatMessage(
                          {
                            id: 'pages.functionsExecutionLogs.detail.sourcePage',
                            defaultMessage: `页面（{pageKey} / {bindingId}）`,
                          },
                          { pageKey: detail.pageKey, bindingId: detail.bindingId },
                        )
                      : intl.formatMessage({
                          id: 'pages.functionsExecutionLogs.sourceLabel.invoke',
                          defaultMessage: '调用',
                        }),
                },
                {
                  key: 'time',
                  label: intl.formatMessage({
                    id: 'pages.functionsExecutionLogs.detail.label.time',
                    defaultMessage: '时间：',
                  }),
                  children: `${formatDateTime(detail.createdAt)} · ${detail.durationMs}ms`,
                },
                {
                  key: 'traceId',
                  label: 'Trace',
                  children: detail.traceId ? (
                    <Text code copyable>
                      {detail.traceId}
                    </Text>
                  ) : (
                    '-'
                  ),
                },
              ]}
            />
            {detail.status !== 'ok' && errorReason && (
              <Alert
                type="error"
                showIcon
                style={{ marginBottom: 16 }}
                message={intl.formatMessage({
                  id: 'pages.functionsExecutionLogs.detail.errorReason',
                  defaultMessage: '失败原因',
                })}
                description={errorReason}
              />
            )}
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
    </PageContainer>
  );
}
