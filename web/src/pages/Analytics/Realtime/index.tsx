import React from 'react';
import { Alert, Card, Row, Col } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { useRealtimeStream } from './useRealtimeStream';
import Toolbar from './Toolbar';
import StatCard from './StatCard';

export default function AnalyticsRealtimePage() {
  const intl = useIntl();
  const {
    data,
    loading,
    auto,
    setAuto,
    streamStatus,
    lastMessageAt,
    ptsOnline,
    ptsA5,
    ptsA15,
    ptsRev5,
    thrOnline,
    setThrOnline,
    thrA5,
    setThrA5,
    refresh,
    clearTrend,
  } = useRealtimeStream();

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({ id: 'pages.analytics.realtime.title' }) || '实时大屏'}
        extra={
          <Toolbar
            streamStatus={streamStatus}
            lastMessageAt={lastMessageAt}
            loading={loading}
            auto={auto}
            onToggleAuto={() => setAuto((x) => !x)}
            onRefresh={refresh}
            onClearTrend={clearTrend}
            thrOnline={thrOnline}
            onThrOnlineChange={(value) => setThrOnline(value)}
            thrA5={thrA5}
            onThrA5Change={(value) => setThrA5(value)}
            ptsOnline={ptsOnline}
            ptsA5={ptsA5}
            ptsA15={ptsA15}
            ptsRev5={ptsRev5}
          />
        }
      >
        <Alert
          type={streamStatus === 'error' ? 'error' : streamStatus === 'stale' ? 'warning' : 'info'}
          showIcon
          style={{ marginBottom: 16 }}
          message={
            streamStatus === 'error'
              ? '实时流连接异常'
              : streamStatus === 'stale'
                ? '实时流已连接，但当前暂未收到新的数据帧'
                : '实时流已连接；当前没有业务事件时，指标显示为 0 属于正常情况'
          }
        />
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <StatCard
              loading={loading}
              title="实时在线"
              value={data?.online || 0}
              contentStyle={
                thrOnline > 0 && Number(data?.online || 0) < thrOnline
                  ? { color: '#cf1322' }
                  : undefined
              }
              spark={ptsOnline}
            />
          </Col>
          <Col span={6}>
            <StatCard loading={loading} title="1分钟活跃" value={data?.active1M || 0} />
          </Col>
          <Col span={6}>
            <StatCard
              loading={loading}
              title="5分钟活跃"
              value={data?.active5M || 0}
              contentStyle={
                thrA5 > 0 && Number(data?.active5M || 0) < thrA5 ? { color: '#cf1322' } : undefined
              }
              spark={ptsA5}
            />
          </Col>
          <Col span={6}>
            <StatCard
              loading={loading}
              title="15分钟活跃"
              value={data?.active15M || 0}
              spark={ptsA15}
            />
          </Col>
        </Row>
        <Row gutter={[16, 16]} style={{ marginTop: 12 }}>
          <Col span={6}>
            <StatCard loading={loading} title="今日峰值在线" value={data?.onlinePeakToday || 0} />
          </Col>
          <Col span={6}>
            <StatCard loading={loading} title="历史峰值在线" value={data?.onlinePeakAllTime || 0} />
          </Col>
          <Col span={6}>
            <StatCard loading={loading} title="今日DAU" value={data?.dauToday || 0} />
          </Col>
          <Col span={6}>
            <StatCard loading={loading} title="今日新增" value={data?.newToday || 0} />
          </Col>
        </Row>
        <Row gutter={[16, 16]} style={{ marginTop: 12 }}>
          <Col span={6}>
            <StatCard loading={loading} title="注册用户总数" value={data?.registeredTotal || 0} />
          </Col>
          <Col span={6}>
            <StatCard
              loading={loading}
              title="5分钟订单额(元)"
              value={Number(data?.rev5M || 0) / 100}
              precision={2}
              prefix="¥"
              spark={ptsRev5}
            />
          </Col>
          <Col span={6}>
            <StatCard
              loading={loading}
              title="支付成功率"
              value={data?.paySuccRate || 0}
              precision={2}
              suffix="%"
            />
          </Col>
          <Col span={6}>
            <StatCard
              loading={loading}
              title="今日充值(元)"
              value={Number(data?.revToday || 0) / 100}
              precision={2}
              prefix="¥"
            />
          </Col>
        </Row>
      </Card>
    </PageContainer>
  );
}
