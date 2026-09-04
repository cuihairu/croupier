import React, { useCallback, useEffect, useState } from 'react';
import { Card, Col, Input, Radio, Row, Select, Space, Statistic, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PageContainer } from '@ant-design/pro-components';
import { Column } from '@ant-design/charts';
import { useIntl } from '@umijs/max';
import {
  fetchInvocationsList,
  fetchInvocationsSummary,
  fetchInvocationsTrend,
  type InvocationFunctionStats,
  type InvocationItem,
  type InvocationsSummary,
} from '@/services/api/analytics';

const DEFAULT_SUMMARY: InvocationsSummary = {
  total: 0,
  failed: 0,
  successRate: 0,
  avgDurationMs: 0,
  p95DurationMs: 0,
  topFunctions: [],
};

type WindowKey = '24h' | '30d';
const WINDOW_CONFIG: Record<WindowKey, { hours: number; interval: string; label: string }> = {
  '24h': { hours: 24, interval: 'hour', label: '近 24 小时' },
  '30d': { hours: 24 * 30, interval: 'day', label: '近 30 天' },
};

export default function AnalyticsInvocationsPage() {
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [summary, setSummary] = useState<InvocationsSummary>(DEFAULT_SUMMARY);
  const [trend, setTrend] = useState<Array<{ bucket: string; value: number; type: string }>>([]);
  const [items, setItems] = useState<InvocationItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
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

  const loadList = useCallback(async () => {
    setLoading(true);
    try {
      const params: Record<string, string | number> = { page, pageSize };
      if (outcome) params.outcome = outcome;
      if (functionId) params.functionId = functionId;
      const r = await fetchInvocationsList(params);
      setItems(r?.items || []);
      setTotal(Number(r?.total || 0));
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, outcome, functionId]);

  useEffect(() => {
    loadSummary();
  }, [loadSummary]);
  useEffect(() => {
    loadList();
  }, [loadList]);

  const functionColumns: ColumnsType<InvocationFunctionStats> = [
    { title: '函数', dataIndex: 'functionId', key: 'functionId' },
    { title: '调用次数', dataIndex: 'total', key: 'total', width: 120 },
    { title: '失败次数', dataIndex: 'failed', key: 'failed', width: 120 },
    {
      title: '平均耗时 (ms)',
      dataIndex: 'avgDurationMs',
      key: 'avgDurationMs',
      width: 140,
      render: (v: number) => (v ? v.toFixed(1) : '-'),
    },
  ];

  const listColumns: ColumnsType<InvocationItem> = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 200,
      render: (v: string) => (v ? new Date(v).toLocaleString() : '-'),
    },
    { title: '函数', dataIndex: 'functionId', key: 'functionId' },
    { title: '操作者', dataIndex: 'actor', key: 'actor', width: 140 },
    {
      title: '结果',
      dataIndex: 'outcome',
      key: 'outcome',
      width: 100,
      render: (v: string) =>
        v === 'success' ? <Tag color="success">成功</Tag> : <Tag color="error">失败</Tag>,
    },
    {
      title: '耗时 (ms)',
      dataIndex: 'durationMs',
      key: 'durationMs',
      width: 110,
      render: (v?: number) => (v == null ? '-' : v),
    },
    {
      title: 'Trace',
      dataIndex: 'traceId',
      key: 'traceId',
      width: 160,
      render: (v?: string) => (v ? <code>{v.slice(0, 16)}</code> : '-'),
    },
    { title: '错误', dataIndex: 'error', key: 'error', ellipsis: true },
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
                { value: '24h', label: WINDOW_CONFIG['24h'].label },
                { value: '30d', label: WINDOW_CONFIG['30d'].label },
              ]}
            />
          }
        >
          <Row gutter={[16, 16]}>
            <Col span={4}>
              <Statistic title="总调用" value={summary.total} />
            </Col>
            <Col span={4}>
              <Statistic title="失败" value={summary.failed} valueStyle={{ color: '#cf1322' }} />
            </Col>
            <Col span={4}>
              <Statistic title="成功率" value={(summary.successRate * 100).toFixed(1)} suffix="%" />
            </Col>
            <Col span={4}>
              <Statistic title="平均耗时 (ms)" value={summary.avgDurationMs.toFixed(1)} />
            </Col>
            <Col span={4}>
              <Statistic title="P95 耗时 (ms)" value={summary.p95DurationMs.toFixed(1)} />
            </Col>
          </Row>
        </Card>

        <Card title={`调用趋势（${WINDOW_CONFIG[window].label}）`} size="small">
          <Column
            data={trend}
            xField="bucket"
            yField="value"
            seriesField="type"
            stack
            height={260}
          />
        </Card>

        <Card title="Top 函数" size="small">
          <Table<InvocationFunctionStats>
            rowKey="functionId"
            columns={functionColumns}
            dataSource={summary.topFunctions}
            pagination={false}
            size="small"
          />
        </Card>

        <Card title="调用明细" size="small">
          <Space style={{ marginBottom: 16 }} wrap>
            <Input.Search
              placeholder="按函数 ID 过滤"
              allowClear
              style={{ width: 260 }}
              onSearch={(v) => {
                setPage(1);
                setFunctionId(v.trim());
              }}
            />
            <Select
              placeholder="结果"
              allowClear
              style={{ width: 140 }}
              value={outcome || undefined}
              onChange={(v) => {
                setPage(1);
                setOutcome(v || '');
              }}
              options={[
                { value: 'success', label: '成功' },
                { value: 'failure', label: '失败' },
              ]}
            />
          </Space>
          <Table<InvocationItem>
            rowKey={(r) => `${r.timestamp}-${r.functionId}-${r.actor}`}
            columns={listColumns}
            dataSource={items}
            loading={loading}
            size="small"
            pagination={{
              current: page,
              pageSize,
              total,
              showSizeChanger: true,
              onChange: (p, ps) => {
                setPage(p);
                setPageSize(ps);
              },
            }}
          />
        </Card>
      </Space>
    </PageContainer>
  );
}
