import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Card, Col, Input, Radio, Row, Select, Space, Statistic, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { Column } from '@ant-design/charts';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  fetchInvocationsList,
  fetchInvocationsSummary,
  fetchInvocationsTrend,
  type InvocationFunctionStats,
  type InvocationItem,
  type InvocationsSummary,
} from '@/services/api/analytics';
import { formatDateTime } from '@/utils/format';

const DEFAULT_SUMMARY: InvocationsSummary = {
  total: 0,
  failed: 0,
  successRate: 0,
  avgDurationMs: 0,
  p95DurationMs: 0,
  topFunctions: [],
};

type WindowKey = '24h' | '30d';
// key/hours/interval 是请求契约；label 是展示文案，经 textId/textDefault 由 intl 解析
const WINDOW_CONFIG: Record<
  WindowKey,
  { hours: number; interval: string; labelId: string; labelDefault: string }
> = {
  '24h': {
    hours: 24,
    interval: 'hour',
    labelId: 'pages.analyticsInvocations.window.last24h',
    labelDefault: '近 24 小时',
  },
  '30d': {
    hours: 24 * 30,
    interval: 'day',
    labelId: 'pages.analyticsInvocations.window.last30d',
    labelDefault: '近 30 天',
  },
};

export default function AnalyticsInvocationsPage() {
  const intl = useIntl();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [summary, setSummary] = useState<InvocationsSummary>(DEFAULT_SUMMARY);
  const [trend, setTrend] = useState<Array<{ bucket: string; value: number; type: string }>>([]);
  const [outcome, setOutcome] = useState<string>('');
  const [functionId, setFunctionId] = useState<string>('');
  const [window, setWindow] = useState<WindowKey>('24h');

  const loadSummary = useCallback(async () => {
    const cfg = WINDOW_CONFIG[window];
    const [s, t] = await Promise.all([
      fetchInvocationsSummary({ hours: cfg.hours }),
      fetchInvocationsTrend({ interval: cfg.interval }),
    ]);
    setSummary(s || DEFAULT_SUMMARY);
    const points = t?.points || [];
    setTrend(
      points.flatMap((p) => [
        { bucket: p.bucket, value: Number(p.total || 0), type: 'total' },
        { bucket: p.bucket, value: Number(p.failed || 0), type: 'failed' },
      ]),
    );
  }, [window]);

  useEffect(() => {
    loadSummary();
  }, [loadSummary]);

  const windowLabel = (key: WindowKey) =>
    intl.formatMessage({
      id: WINDOW_CONFIG[key].labelId,
      defaultMessage: WINDOW_CONFIG[key].labelDefault,
    });

  const functionColumns: ColumnsType<InvocationFunctionStats> = [
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.function',
        defaultMessage: '函数',
      }),
      dataIndex: 'functionId',
      key: 'functionId',
    },
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.callCount',
        defaultMessage: '调用次数',
      }),
      dataIndex: 'total',
      key: 'total',
      width: 120,
    },
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.failedCount',
        defaultMessage: '失败次数',
      }),
      dataIndex: 'failed',
      key: 'failed',
      width: 120,
    },
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.avgDuration',
        defaultMessage: '平均耗时 (ms)',
      }),
      dataIndex: 'avgDurationMs',
      key: 'avgDurationMs',
      width: 140,
      render: (v: number) => (v ? v.toFixed(1) : '-'),
    },
  ];

  const listColumns: ProColumns<InvocationItem>[] = [
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.time',
        defaultMessage: '时间',
      }),
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 200,
      render: (_, r) => formatDateTime(r.timestamp ?? ''),
    },
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.function',
        defaultMessage: '函数',
      }),
      dataIndex: 'functionId',
      key: 'functionId',
    },
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.actor',
        defaultMessage: '操作者',
      }),
      dataIndex: 'actor',
      key: 'actor',
      width: 140,
    },
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.outcome',
        defaultMessage: '结果',
      }),
      dataIndex: 'outcome',
      key: 'outcome',
      width: 100,
      render: (_, r) =>
        r.outcome === 'success' ? (
          <Tag color="success">
            <FormattedMessage
              id="pages.analyticsInvocations.outcome.success"
              defaultMessage="成功"
            />
          </Tag>
        ) : (
          <Tag color="error">
            <FormattedMessage
              id="pages.analyticsInvocations.outcome.failure"
              defaultMessage="失败"
            />
          </Tag>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.duration',
        defaultMessage: '耗时 (ms)',
      }),
      dataIndex: 'durationMs',
      key: 'durationMs',
      width: 110,
      render: (_, r) => (r.durationMs == null ? '-' : r.durationMs),
    },
    {
      title: 'Trace',
      dataIndex: 'traceId',
      key: 'traceId',
      width: 160,
      render: (_, r) => (r.traceId ? <code>{r.traceId.slice(0, 16)}</code> : '-'),
    },
    {
      title: intl.formatMessage({
        id: 'pages.analyticsInvocations.column.error',
        defaultMessage: '错误',
      }),
      dataIndex: 'error',
      key: 'error',
      ellipsis: true,
    },
  ];

  return (
    <PageContainer>
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <Card
          title={intl.formatMessage({ id: 'pages.analytics.invocations.title' })}
          extra={
            <Radio.Group
              value={window}
              onChange={(e) => setWindow(e.target.value as WindowKey)}
              optionType="button"
              buttonStyle="solid"
              size="small"
              options={[
                { value: '24h', label: windowLabel('24h') },
                { value: '30d', label: windowLabel('30d') },
              ]}
            />
          }
        >
          <Row gutter={[16, 16]}>
            <Col span={4}>
              <Statistic
                title={intl.formatMessage({
                  id: 'pages.analyticsInvocations.summary.total',
                  defaultMessage: '总调用',
                })}
                value={summary.total}
              />
            </Col>
            <Col span={4}>
              <Statistic
                title={intl.formatMessage({
                  id: 'pages.analyticsInvocations.summary.failed',
                  defaultMessage: '失败',
                })}
                value={summary.failed}
                valueStyle={{ color: '#cf1322' }}
              />
            </Col>
            <Col span={4}>
              <Statistic
                title={intl.formatMessage({
                  id: 'pages.analyticsInvocations.summary.successRate',
                  defaultMessage: '成功率',
                })}
                value={(summary.successRate * 100).toFixed(1)}
                suffix="%"
              />
            </Col>
            <Col span={4}>
              <Statistic
                title={intl.formatMessage({
                  id: 'pages.analyticsInvocations.summary.avgDuration',
                  defaultMessage: '平均耗时 (ms)',
                })}
                value={summary.avgDurationMs.toFixed(1)}
              />
            </Col>
            <Col span={4}>
              <Statistic
                title={intl.formatMessage({
                  id: 'pages.analyticsInvocations.summary.p95Duration',
                  defaultMessage: 'P95 耗时 (ms)',
                })}
                value={summary.p95DurationMs.toFixed(1)}
              />
            </Col>
          </Row>
        </Card>

        <Card
          title={intl.formatMessage(
            {
              id: 'pages.analyticsInvocations.card.trend',
              defaultMessage: `调用趋势（${windowLabel(window)}）`,
            },
            { window: windowLabel(window) },
          )}
          size="small"
        >
          <Column
            data={trend}
            xField="bucket"
            yField="value"
            seriesField="type"
            stack
            height={260}
          />
        </Card>

        <Card
          title={intl.formatMessage({
            id: 'pages.analyticsInvocations.card.topFunctions',
            defaultMessage: 'Top 函数',
          })}
          size="small"
        >
          <Table<InvocationFunctionStats>
            rowKey="functionId"
            columns={functionColumns}
            dataSource={summary.topFunctions}
            pagination={false}
            size="small"
          />
        </Card>

        <Card
          title={intl.formatMessage({
            id: 'pages.analyticsInvocations.card.invocationDetail',
            defaultMessage: '调用明细',
          })}
          size="small"
        >
          <Space style={{ marginBottom: 16 }} wrap>
            <Input.Search
              placeholder={intl.formatMessage({
                id: 'pages.analyticsInvocations.filter.placeholder.functionId',
                defaultMessage: '按函数 ID 过滤',
              })}
              allowClear
              style={{ width: 260 }}
              onSearch={(v) => {
                setFunctionId(v.trim());
                // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
                // ProTable 内部 debounce + abort 合并，不会出现错序数据
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.analyticsInvocations.filter.placeholder.outcome',
                defaultMessage: '结果',
              })}
              allowClear
              style={{ width: 140 }}
              value={outcome || undefined}
              onChange={(v) => {
                setOutcome(v || '');
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              options={[
                {
                  value: 'success',
                  label: intl.formatMessage({
                    id: 'pages.analyticsInvocations.outcome.success',
                    defaultMessage: '成功',
                  }),
                },
                {
                  value: 'failure',
                  label: intl.formatMessage({
                    id: 'pages.analyticsInvocations.outcome.failure',
                    defaultMessage: '失败',
                  }),
                },
              ]}
            />
          </Space>
          <ProTable<InvocationItem>
            actionRef={actionRef}
            rowKey={(r) => `${r.timestamp}-${r.functionId}-${r.actor}`}
            columns={listColumns}
            size="small"
            search={false}
            options={false}
            toolBarRender={false}
            params={{ outcome, functionId }}
            request={async ({
              current = 1,
              pageSize = 20,
              outcome: outcomeFilter = '',
              functionId: functionIdFilter = '',
            }) => {
              const params: Record<string, string | number> = { page: current, pageSize };
              if (outcomeFilter) params.outcome = outcomeFilter;
              if (functionIdFilter) params.functionId = functionIdFilter;
              try {
                const r = await fetchInvocationsList(params);
                return { data: r?.items || [], total: Number(r?.total || 0), success: true };
              } catch {
                // 原实现不本地弹错（try/finally 直接上抛，由全局请求拦截器 toast），保持该语义
                return { data: [], total: 0, success: false };
              }
            }}
            pagination={{ pageSize: 20, showSizeChanger: true }}
          />
        </Card>
      </Space>
    </PageContainer>
  );
}
