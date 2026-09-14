import React, { useEffect, useState } from 'react';
import { Alert, Button, Col, Descriptions, Drawer, Row, Table, Tag, Typography } from 'antd';
import { StatisticCard } from '@ant-design/pro-components';
import { BarChartOutlined } from '@ant-design/icons';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { getFunctionAnalytics, listFunctionWarnings } from '@/services/api/functions';
import {
  getExecutionLog,
  listExecutionLogs,
  type ExecutionLogDetail,
  type ExecutionLogItem,
} from '@/services/api/executionLogs';

type AnalyticsData = {
  totalCalls: number;
  successRate: number;
  avgLatency: number;
  callsToday: number;
};

const formatDateTime = (value?: string) => {
  if (!value) return '-';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString();
};

export function HistoryTab({ functionId }: { functionId: string }) {
  const intl = useIntl();
  const [historyData, setHistoryData] = useState<ExecutionLogItem[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [total, setTotal] = useState(0);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<ExecutionLogDetail | null>(null);

  useEffect(() => {
    const loadHistory = async () => {
      setHistoryLoading(true);
      try {
        // 真实调用记录（执行留痕）：操作人为发起调用的登录账号；
        // 游戏侧 SDK 直连调用没有控制台身份，actor 为空展示 '-'
        const resp = await listExecutionLogs({ functionId, page, pageSize });
        setHistoryData(resp.items ?? []);
        setTotal(resp.total ?? 0);
      } catch {
        setHistoryData([]);
        setTotal(0);
      } finally {
        setHistoryLoading(false);
      }
    };
    loadHistory();
  }, [functionId, page, pageSize]);

  const openDetail = async (id: number) => {
    setDetailOpen(true);
    setDetailLoading(true);
    try {
      setDetail(await getExecutionLog(id));
    } catch {
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const renderPayload = (value: unknown) => {
    if (value === undefined || value === null) {
      return '-';
    }
    let text: string;
    try {
      text = JSON.stringify(value, null, 2);
    } catch {
      text = String(value);
    }
    return (
      <pre
        style={{
          margin: 0,
          maxHeight: 320,
          overflow: 'auto',
          fontSize: 12,
          background: 'rgba(128, 128, 128, 0.08)',
          padding: 8,
          borderRadius: 4,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        {text}
      </pre>
    );
  };

  return (
    <>
      <Table
        loading={historyLoading}
        dataSource={historyData}
        rowKey="id"
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.time',
              defaultMessage: '时间',
            }),
            dataIndex: 'createdAt',
            width: 170,
            render: (text: string) => formatDateTime(text),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.operator',
              defaultMessage: '操作人',
            }),
            dataIndex: 'actor',
            width: 120,
            render: (text: string) => text || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.status',
              defaultMessage: '状态',
            }),
            dataIndex: 'status',
            width: 90,
            render: (text: string) => (
              <Tag color={text === 'ok' ? 'success' : 'error'}>
                {intl.formatMessage({
                  id:
                    text === 'ok'
                      ? 'pages.functionsDetail.history.status.ok'
                      : 'pages.functionsDetail.history.status.fail',
                  defaultMessage: text === 'ok' ? '成功' : '失败',
                })}
              </Tag>
            ),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.duration',
              defaultMessage: '耗时',
            }),
            dataIndex: 'durationMs',
            width: 100,
            render: (text: number) => (text ? `${text} ms` : '-'),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.source',
              defaultMessage: '来源',
            }),
            dataIndex: 'source',
            width: 100,
            render: (text: string) =>
              intl.formatMessage({
                id:
                  text === 'page'
                    ? 'pages.functionsDetail.history.source.page'
                    : 'pages.functionsDetail.history.source.invoke',
                defaultMessage: text === 'page' ? '页面执行' : '直接调用',
              }),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.history.column.detailAction',
              defaultMessage: '操作',
            }),
            key: 'detailAction',
            width: 80,
            render: (_, record) => (
              <Button type="link" size="small" onClick={() => openDetail(record.id)}>
                <FormattedMessage
                  id="pages.functionsDetail.history.action.viewDetail"
                  defaultMessage="详情"
                />
              </Button>
            ),
          },
        ]}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          pageSizeOptions: [10, 20, 50],
          showTotal: (t) =>
            intl.formatMessage(
              {
                id: 'pages.functionsDetail.history.paginationTotal',
                defaultMessage: '共 {total} 条',
              },
              { total: t },
            ),
          onChange: (nextPage, nextSize) => {
            setPage(nextPage);
            setPageSize(nextSize);
          },
        }}
      />
      <Drawer
        open={detailOpen}
        width={560}
        loading={detailLoading}
        onClose={() => setDetailOpen(false)}
        title={
          detail
            ? intl.formatMessage(
                {
                  id: 'pages.functionsDetail.history.detail.title',
                  defaultMessage: '调用详情 · {functionId}',
                },
                { functionId: detail.functionId },
              )
            : intl.formatMessage({
                id: 'pages.functionsDetail.history.detail.titlePlain',
                defaultMessage: '调用详情',
              })
        }
      >
        {detail && (
          <>
            <Descriptions
              column={2}
              size="small"
              style={{ marginBottom: 16 }}
              items={[
                {
                  key: 'actor',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.operator',
                    defaultMessage: '操作人',
                  }),
                  children: detail.actor || '-',
                },
                {
                  key: 'status',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.status',
                    defaultMessage: '状态',
                  }),
                  children:
                    detail.status === 'ok'
                      ? intl.formatMessage({
                          id: 'pages.functionsDetail.history.status.ok',
                          defaultMessage: '成功',
                        })
                      : intl.formatMessage({
                          id: 'pages.functionsDetail.history.status.fail',
                          defaultMessage: '失败',
                        }),
                },
                {
                  key: 'time',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.time',
                    defaultMessage: '时间',
                  }),
                  children: formatDateTime(detail.createdAt),
                },
                {
                  key: 'duration',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.duration',
                    defaultMessage: '耗时',
                  }),
                  children: detail.durationMs ? `${detail.durationMs} ms` : '-',
                },
                {
                  key: 'source',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.column.source',
                    defaultMessage: '来源',
                  }),
                  children:
                    detail.source === 'page'
                      ? intl.formatMessage({
                          id: 'pages.functionsDetail.history.source.page',
                          defaultMessage: '页面执行',
                        })
                      : intl.formatMessage({
                          id: 'pages.functionsDetail.history.source.invoke',
                          defaultMessage: '直接调用',
                        }),
                },
                {
                  key: 'pageKey',
                  label: intl.formatMessage({
                    id: 'pages.functionsDetail.history.detail.pageKey',
                    defaultMessage: '页面 Key',
                  }),
                  children: detail.pageKey || '-',
                },
              ]}
            />
            <Typography.Title level={5}>
              <FormattedMessage
                id="pages.functionsDetail.history.detail.request"
                defaultMessage="请求参数"
              />
            </Typography.Title>
            {renderPayload(detail.requestPayload)}
            <Typography.Title level={5} style={{ marginTop: 16 }}>
              <FormattedMessage
                id="pages.functionsDetail.history.detail.response"
                defaultMessage="执行结果"
              />
            </Typography.Title>
            {renderPayload(detail.responseBody)}
          </>
        )}
      </Drawer>
    </>
  );
}

export function AnalyticsTab({ functionId }: { functionId: string }) {
  const intl = useIntl();
  const [analyticsData, setAnalyticsData] = useState<AnalyticsData | null>(null);
  const [analyticsLoading, setAnalyticsLoading] = useState(false);

  useEffect(() => {
    const loadAnalytics = async () => {
      setAnalyticsLoading(true);
      try {
        const data = await getFunctionAnalytics(functionId);
        setAnalyticsData(data);
      } catch {
        setAnalyticsData(null);
      } finally {
        setAnalyticsLoading(false);
      }
    };
    loadAnalytics();
  }, [functionId]);

  return (
    <Row gutter={16}>
      <Col span={6}>
        <StatisticCard
          loading={analyticsLoading}
          statistic={{
            title: intl.formatMessage({
              id: 'pages.functionsDetail.analytics.totalCalls',
              defaultMessage: '总调用次数',
            }),
            value: analyticsData?.totalCalls || 0,
            prefix: <BarChartOutlined />,
          }}
        />
      </Col>
      <Col span={6}>
        <StatisticCard
          loading={analyticsLoading}
          statistic={{
            title: intl.formatMessage({
              id: 'pages.functionsDetail.analytics.successRate',
              defaultMessage: '成功率',
            }),
            value: analyticsData?.successRate || 0,
            suffix: '%',
            precision: 2,
            styles: {
              content: {
                color: (analyticsData?.successRate || 0) >= 95 ? '#3f8600' : '#cf1322',
              },
            },
          }}
        />
      </Col>
      <Col span={6}>
        <StatisticCard
          loading={analyticsLoading}
          statistic={{
            title: intl.formatMessage({
              id: 'pages.functionsDetail.analytics.avgLatency',
              defaultMessage: '平均延迟',
            }),
            value: analyticsData?.avgLatency || 0,
            suffix: 'ms',
            precision: 0,
          }}
        />
      </Col>
      <Col span={6}>
        <StatisticCard
          loading={analyticsLoading}
          statistic={{
            title: intl.formatMessage({
              id: 'pages.functionsDetail.analytics.callsToday',
              defaultMessage: '今日调用',
            }),
            value: analyticsData?.callsToday || 0,
          }}
        />
      </Col>
    </Row>
  );
}

export function WarningsTab({ functionId }: { functionId: string }) {
  const intl = useIntl();
  const [warningsData, setWarningsData] = useState<
    Array<{
      key: string;
      agentId?: string;
      functionId?: string;
      version?: string;
      code: string;
      message: string;
      count: number;
      firstSeen?: string;
      lastSeen?: string;
    }>
  >([]);
  const [warningsLoading, setWarningsLoading] = useState(false);

  useEffect(() => {
    const loadWarnings = async () => {
      setWarningsLoading(true);
      try {
        const res = await listFunctionWarnings({ functionId, limit: 200 });
        setWarningsData(Array.isArray(res?.items) ? res.items : []);
      } catch {
        setWarningsData([]);
      } finally {
        setWarningsLoading(false);
      }
    };
    loadWarnings();
  }, [functionId]);

  return (
    <>
      <Alert
        message={intl.formatMessage({
          id: 'pages.functionsDetail.warnings.alertMessage',
          defaultMessage: '注册告警',
        })}
        description={intl.formatMessage({
          id: 'pages.functionsDetail.warnings.alertDescription',
          defaultMessage:
            '这里显示函数注册校验告警（例如 function_id 格式错误、版本号不合法、重复注册去重）。',
        })}
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        action={
          <Button
            size="small"
            onClick={() =>
              history.push(`/functions/warnings?function_id=${encodeURIComponent(functionId)}`)
            }
          >
            <FormattedMessage
              id="pages.functionsDetail.warnings.viewAll"
              defaultMessage="查看全部"
            />
          </Button>
        }
      />
      <Table
        scroll={{ x: 850 }}
        loading={warningsLoading}
        dataSource={warningsData}
        rowKey="key"
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.code',
              defaultMessage: '代码',
            }),
            dataIndex: 'code',
            width: 180,
            render: (code: string) => <Tag color="orange">{code || '-'}</Tag>,
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.version',
              defaultMessage: '版本',
            }),
            dataIndex: 'version',
            width: 120,
            render: (v: string) => v || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.count',
              defaultMessage: '次数',
            }),
            dataIndex: 'count',
            width: 90,
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.lastSeen',
              defaultMessage: '最近时间',
            }),
            dataIndex: 'lastSeen',
            width: 180,
            render: (text: string) => formatDateTime(text),
          },
          { title: 'Agent', dataIndex: 'agentId', width: 220, ellipsis: true },
          {
            title: intl.formatMessage({
              id: 'pages.functionsDetail.warnings.column.details',
              defaultMessage: '详情',
            }),
            dataIndex: 'message',
            ellipsis: true,
          },
        ]}
        pagination={{ pageSize: 10 }}
      />
    </>
  );
}
