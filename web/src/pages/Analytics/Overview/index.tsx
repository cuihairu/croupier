import React, { useEffect, useState, useCallback } from 'react';
import { Button, Card, Space, Row, Col, Divider } from 'antd';
import { PageContainer, StatisticCard } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { exportToXLSX } from '@/utils/export';
import { fetchAnalyticsOverview } from '@/services/api/analytics';

interface OverviewData {
  dau?: number;
  wau?: number | null;
  mau?: number;
  newUsers?: number;
  registeredTotal?: number | null;
  peakOnline?: number;
  revenue?: number;
  d1?: number | null;
  d7?: number | null;
  d30?: number | null;
  payRate?: number | null;
  arpu?: number;
  arppu?: number;
  series?: {
    newUsers?: [string | number, number][];
    peakOnline?: [string | number, number][];
    revenue?: [string | number, number][];
  };
}

export default function AnalyticsOverviewPage() {
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<OverviewData>({});

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await fetchAnalyticsOverview({});
      setData(r || {});
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    load();
  }, [load]);

  const exportExcel = async () => {
    // Sheet 1: summary; Sheet 2: series (new_users/peak_online/revenue)
    const summary = [
      ['metric', 'value'],
      ['dau', data?.dau || 0],
      ['wau', data?.wau ?? ''],
      ['mau', data?.mau || 0],
      ['new_users', data?.newUsers || 0],
      ['registered_total', data?.registeredTotal ?? ''],
      ['retention_d1', data?.d1 ?? ''],
      ['retention_d7', data?.d7 ?? ''],
      ['retention_d30', data?.d30 ?? ''],
      ['pay_rate', data?.payRate ?? ''],
      ['arpu', data?.arpu || 0],
      ['arppu', data?.arppu || 0],
      ['revenue', data?.revenue || 0],
    ];
    const ser = data?.series || {};
    const seriesRows = [['time', 'new_users', 'peak_online', 'revenue_cents']];
    const len = Math.max(
      ser?.newUsers?.length || 0,
      ser?.peakOnline?.length || 0,
      ser?.revenue?.length || 0,
    );
    for (let i = 0; i < len; i++) {
      const t =
        ser?.newUsers?.[i]?.[0] || ser?.peakOnline?.[i]?.[0] || ser?.revenue?.[i]?.[0] || '';
      const nu = ser?.newUsers?.[i]?.[1] ?? '';
      const po = ser?.peakOnline?.[i]?.[1] ?? '';
      const rv = ser?.revenue?.[i]?.[1] ?? '';
      seriesRows.push([String(t), String(nu), String(po), String(rv)]);
    }
    await exportToXLSX('overview.csv', [
      { sheet: 'summary', rows: summary },
      { sheet: 'series', rows: seriesRows },
    ]);
  };

  const Spark: React.FC<{ data?: [string | number, number][] }> = ({ data }) => {
    const w = 300,
      h = 60,
      p = 4;
    const pts = (data || []).map((d) => [Number(d[0]), Number(d[1])]);
    if (pts.length === 0) return <div style={{ height: h }} />;
    const xs = pts.map((p) => p[0]);
    const ys = pts.map((p) => p[1]);
    const x0 = Math.min(...xs),
      x1 = Math.max(...xs),
      y0 = Math.min(...ys),
      y1 = Math.max(...ys);
    const sx = (x: number) => (x1 === x0 ? p : p + ((w - 2 * p) * (x - x0)) / (x1 - x0));
    const sy = (y: number) => (y1 === y0 ? h - p : h - (p + ((h - 2 * p) * (y - y0)) / (y1 - y0)));
    const d = pts.map((pt, i) => `${i ? 'L' : 'M'}${sx(pt[0])},${sy(pt[1])}`).join(' ');
    return (
      <svg width={w} height={h} style={{ display: 'block' }}>
        <path d={d} fill="none" stroke="#1677ff" strokeWidth={2} />
      </svg>
    );
  };

  return (
    <PageContainer
      extra={[
        <Button key="export" onClick={() => void exportExcel()}>
          <FormattedMessage id="pages.analyticsOverview.button.export" defaultMessage="导出" />
        </Button>,
      ]}
    >
      <Card
        title={intl.formatMessage({
          id: 'pages.analyticsOverview.title',
          defaultMessage: '概览 KPI',
        })}
      >
        <Space orientation="vertical" size={16} style={{ width: '100%' }}>
          <Row gutter={[16, 16]}>
            <Col span={4}>
              <StatisticCard
                loading={loading}
                statistic={{ title: 'DAU', value: data?.dau || 0 }}
              />
            </Col>
            <Col span={4}>
              <StatisticCard
                loading={loading}
                statistic={{ title: 'WAU', value: data?.wau ?? '-' }}
              />
            </Col>
            <Col span={4}>
              <StatisticCard
                loading={loading}
                statistic={{ title: 'MAU', value: data?.mau || 0 }}
              />
            </Col>
            <Col span={4}>
              <StatisticCard
                loading={loading}
                statistic={{
                  title: intl.formatMessage({
                    id: 'pages.analyticsOverview.stat.newUsers',
                    defaultMessage: '新增',
                  }),
                  value: data?.newUsers || 0,
                }}
              />
            </Col>
            <Col span={4}>
              <StatisticCard
                loading={loading}
                statistic={{
                  title: intl.formatMessage({
                    id: 'pages.analyticsOverview.stat.registeredTotal',
                    defaultMessage: '注册用户总数',
                  }),
                  value: data?.registeredTotal ?? '-',
                }}
              />
            </Col>
            <Col span={4}>
              <StatisticCard
                loading={loading}
                statistic={{
                  title: intl.formatMessage({
                    id: 'pages.analyticsOverview.stat.revenue',
                    defaultMessage: '收入',
                  }),
                  value: data?.revenue || 0,
                }}
              />
            </Col>
          </Row>
          <Row gutter={[16, 16]}>
            <Col span={8}>
              <StatisticCard
                loading={loading}
                statistic={{
                  title: intl.formatMessage({
                    id: 'pages.analyticsOverview.stat.payRate',
                    defaultMessage: '付费率',
                  }),
                  suffix: '%',
                  value: data?.payRate ?? '-',
                }}
              />
            </Col>
            <Col span={8}>
              <StatisticCard
                loading={loading}
                statistic={{ title: 'ARPU', value: data?.arpu || 0 }}
              />
            </Col>
            <Col span={8}>
              <StatisticCard
                loading={loading}
                statistic={{ title: 'ARPPU', value: data?.arppu || 0 }}
              />
            </Col>
          </Row>
          <Divider />
          <Row gutter={[16, 16]}>
            <Col span={8}>
              <StatisticCard
                loading={loading}
                statistic={{
                  title: intl.formatMessage({
                    id: 'pages.analyticsOverview.stat.retentionD1',
                    defaultMessage: 'D1 留存',
                  }),
                  value: data?.d1 ?? '-',
                  suffix: data?.d1 == null ? '' : '%',
                }}
              />
            </Col>
            <Col span={8}>
              <StatisticCard
                loading={loading}
                statistic={{
                  title: intl.formatMessage({
                    id: 'pages.analyticsOverview.stat.retentionD7',
                    defaultMessage: 'D7 留存',
                  }),
                  value: data?.d7 ?? '-',
                  suffix: data?.d7 == null ? '' : '%',
                }}
              />
            </Col>
            <Col span={8}>
              <StatisticCard
                loading={loading}
                statistic={{
                  title: intl.formatMessage({
                    id: 'pages.analyticsOverview.stat.retentionD30',
                    defaultMessage: 'D30 留存',
                  }),
                  value: data?.d30 ?? '-',
                  suffix: data?.d30 == null ? '' : '%',
                }}
              />
            </Col>
          </Row>
          <Divider />
          <Row gutter={[16, 16]}>
            <Col span={8}>
              <Card
                size="small"
                title={intl.formatMessage({
                  id: 'pages.analyticsOverview.spark.dailyNewUsers',
                  defaultMessage: '每日新增（曲线）',
                })}
              >
                <Spark data={data?.series?.newUsers || []} />
              </Card>
            </Col>
            <Col span={8}>
              <Card
                size="small"
                title={intl.formatMessage({
                  id: 'pages.analyticsOverview.spark.dailyPeakOnline',
                  defaultMessage: '每日峰值在线（曲线）',
                })}
              >
                <Spark data={data?.series?.peakOnline || []} />
              </Card>
            </Col>
            <Col span={8}>
              <Card
                size="small"
                title={intl.formatMessage({
                  id: 'pages.analyticsOverview.spark.dailyRevenue',
                  defaultMessage: '每日收入（曲线）',
                })}
              >
                <Spark data={data?.series?.revenue || []} />
              </Card>
            </Col>
          </Row>
        </Space>
      </Card>
    </PageContainer>
  );
}
