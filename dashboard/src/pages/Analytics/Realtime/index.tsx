import { Card, Col, Divider, List, Row, Statistic, Typography } from 'antd';
import { useRequest } from '@umijs/max';
import React from 'react';
import { fetchAnalyticsRealtime, fetchRealtimeSeries } from '@/services/croupier/analytics';
import AnalyticsComingSoon from '../ComingSoon';

const PlaceholderRealtime: React.FC = () => {
  const realtime = useRequest(fetchAnalyticsRealtime);
  const series = useRequest(
    async () => fetchRealtimeSeries({ window: '30m' }),
    { pollingInterval: 60_000 },
  );

  const stats = realtime.data || {};
  const timeseries = series.data || { online: [], revenue_cents: [] };

  return (
    <Card>
      <Typography.Title level={4}>Realtime snapshot</Typography.Title>
      <Row gutter={16}>
        <Col span={6}><Statistic title="在线人数" value={stats.online ?? '-'} suffix="人" /></Col>
        <Col span={6}><Statistic title="付费(当日)" value={(stats.revenue_yuan ?? 0)} suffix="元" precision={2} /></Col>
        <Col span={6}><Statistic title="DAU (预估)" value={stats.dau_estimate ?? '-'} /></Col>
        <Col span={6}><Statistic title="峰值在线(今日)" value={stats.peak_online ?? '-'} /></Col>
      </Row>

      <Divider />
      <Typography.Title level={5}>最近 30 分钟（每分钟）</Typography.Title>
      <Row gutter={16}>
        <Col span={12}>
          <Card title="Online">
            <List
              dataSource={timeseries.online?.slice(-20).reverse() || []}
              renderItem={(item: any) => (
                <List.Item>
                  <Typography.Text>{item?.ts || '-'}</Typography.Text>
                  <Typography.Text strong style={{ marginLeft: 'auto' }}>{item?.value ?? '-'}</Typography.Text>
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col span={12}>
          <Card title="Revenue (元)">
            <List
              dataSource={timeseries.revenue_cents?.slice(-20).reverse() || []}
              renderItem={(item: any) => (
                <List.Item>
                  <Typography.Text>{item?.ts || '-'}</Typography.Text>
                  <Typography.Text strong style={{ marginLeft: 'auto' }}>
                    {item && typeof item.value === 'number' ? (item.value / 100).toFixed(2) : '-'}
                  </Typography.Text>
                </List.Item>
              )}
            />
          </Card>
        </Col>
      </Row>
    </Card>
  );
};

const RealtimePage: React.FC = () => (
  <AnalyticsComingSoon
    pageName="Realtime"
    description="实时看板将在完善设计后替换此占位。当前展示简单的统计卡片，便于验证 API 与权限。"
    checklist={[
      { title: 'analytics.minute_online / .daily_online_peak 表有数据', detail: 'cmd/analytics-worker 写入 ClickHouse' },
      { title: 'configs/analytics/metrics.yaml 已定义 realtime_* 指标', detail: '供 API 聚合' },
      { title: 'Grafana / DataV 设计完成', detail: '替换占位组件' },
    ]}
  >
    <PlaceholderRealtime />
  </AnalyticsComingSoon>
);

export default RealtimePage;
