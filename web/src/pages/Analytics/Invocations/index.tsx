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
import { useIntl } from '@umijs/max';
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
const WINDOW_CONFIG: Record<WindowKey, { hours: number; interval: string; label: string }> = {
  '24h': { hours: 24, interval: 'hour', label: '近 24 小时' },
  '30d': { hours: 24 * 30, interval: 'day', label: '近 30 天' },
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

  const listColumns: ProColumns<InvocationItem>[] = [
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 200,
      render: (_, r) => formatDateTime(r.timestamp ?? ''),
    },
    { title: '函数', dataIndex: 'functionId', key: 'functionId' },
    { title: '操作者', dataIndex: 'actor', key: 'actor', width: 140 },
    {
      title: '结果',
      dataIndex: 'outcome',
      key: 'outcome',
      width: 100,
      render: (_, r) =>
        r.outcome === 'success' ? <Tag color="success">成功</Tag> : <Tag color="error">失败</Tag>,
    },
    {
      title: '耗时 (ms)',
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
                setFunctionId(v.trim());
                // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
                // ProTable 内部 debounce + abort 合并，不会出现错序数据
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
            />
            <Select
              placeholder="结果"
              allowClear
              style={{ width: 140 }}
              value={outcome || undefined}
              onChange={(v) => {
                setOutcome(v || '');
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              options={[
                { value: 'success', label: '成功' },
                { value: 'failure', label: '失败' },
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
