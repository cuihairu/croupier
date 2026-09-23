import React from 'react';
import { Alert, Card, Space, Typography, theme as antdTheme } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { DASHBOARD_PAGE_TOKENS } from '@/components';
import { useRealtimeStream } from './useRealtimeStream';
import Toolbar from './Toolbar';
import StatCard from './StatCard';

// 指标网格：auto-fit + minmax 自适应列数（宽屏 4 列、窄屏自动降列），
// 取代固定 span 的 Row/Col（CSS Grid 惯用法先例 SdkDistribution）。
const statGridStyle: React.CSSProperties = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
  gap: DASHBOARD_PAGE_TOKENS.sectionGap,
};

/** 指标分组：主次指标分区块展示（实时活跃 / 规模与存量 / 收入转化）。 */
function StatGroup({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Space
      orientation="vertical"
      size={DASHBOARD_PAGE_TOKENS.itemGap}
      style={{ width: '100%' }}
      data-testid="stat-group"
    >
      <Typography.Title level={5} style={{ margin: 0 }}>
        {title}
      </Typography.Title>
      <div style={statGridStyle} data-testid="stat-grid">
        {children}
      </div>
    </Space>
  );
}

export default function AnalyticsRealtimePage() {
  const intl = useIntl();
  const { token } = antdTheme.useToken();
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

  // 低于阈值时数值标红（色值取 antd token，不再硬编码）
  const belowThresholdStyle = (
    value: number,
    threshold: number,
  ): React.CSSProperties | undefined =>
    threshold > 0 && value < threshold ? { color: token.colorError } : undefined;

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({
          id: 'pages.analyticsRealtime.title',
          defaultMessage: '实时大屏',
        })}
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
          style={{ marginBottom: DASHBOARD_PAGE_TOKENS.sectionGap }}
          message={
            streamStatus === 'error'
              ? intl.formatMessage({
                  id: 'pages.analyticsRealtime.alert.streamError',
                  defaultMessage: '实时流连接异常',
                })
              : streamStatus === 'stale'
                ? intl.formatMessage({
                    id: 'pages.analyticsRealtime.alert.streamStale',
                    defaultMessage: '实时流已连接，但当前暂未收到新的数据帧',
                  })
                : intl.formatMessage({
                    id: 'pages.analyticsRealtime.alert.streamInfo',
                    defaultMessage: '实时流已连接；当前没有业务事件时，指标显示为 0 属于正常情况',
                  })
          }
        />
        <Space
          orientation="vertical"
          size={DASHBOARD_PAGE_TOKENS.sectionGap}
          style={{ width: '100%' }}
        >
          <StatGroup
            title={intl.formatMessage({
              id: 'pages.analyticsRealtime.stat.group.live',
              defaultMessage: '实时活跃',
            })}
          >
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.online',
                defaultMessage: '实时在线',
              })}
              value={data?.online || 0}
              contentStyle={belowThresholdStyle(Number(data?.online || 0), thrOnline)}
              spark={ptsOnline}
            />
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.active1M',
                defaultMessage: '1分钟活跃',
              })}
              value={data?.active1M || 0}
            />
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.active5M',
                defaultMessage: '5分钟活跃',
              })}
              value={data?.active5M || 0}
              contentStyle={belowThresholdStyle(Number(data?.active5M || 0), thrA5)}
              spark={ptsA5}
            />
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.active15M',
                defaultMessage: '15分钟活跃',
              })}
              value={data?.active15M || 0}
              spark={ptsA15}
            />
          </StatGroup>
          <StatGroup
            title={intl.formatMessage({
              id: 'pages.analyticsRealtime.stat.group.scale',
              defaultMessage: '规模与存量',
            })}
          >
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.onlinePeakToday',
                defaultMessage: '今日峰值在线',
              })}
              value={data?.onlinePeakToday || 0}
            />
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.onlinePeakAllTime',
                defaultMessage: '历史峰值在线',
              })}
              value={data?.onlinePeakAllTime || 0}
            />
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.dauToday',
                defaultMessage: '今日DAU',
              })}
              value={data?.dauToday || 0}
            />
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.newToday',
                defaultMessage: '今日新增',
              })}
              value={data?.newToday || 0}
            />
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.registeredTotal',
                defaultMessage: '注册用户总数',
              })}
              value={data?.registeredTotal || 0}
            />
          </StatGroup>
          <StatGroup
            title={intl.formatMessage({
              id: 'pages.analyticsRealtime.stat.group.revenue',
              defaultMessage: '收入转化',
            })}
          >
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.rev5M',
                defaultMessage: '5分钟订单额(元)',
              })}
              value={Number(data?.rev5M || 0) / 100}
              precision={2}
              prefix="¥"
              spark={ptsRev5}
            />
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.paySuccRate',
                defaultMessage: '支付成功率',
              })}
              value={data?.paySuccRate || 0}
              precision={2}
              suffix="%"
            />
            <StatCard
              loading={loading}
              title={intl.formatMessage({
                id: 'pages.analyticsRealtime.stat.revToday',
                defaultMessage: '今日充值(元)',
              })}
              value={Number(data?.revToday || 0) / 100}
              precision={2}
              prefix="¥"
            />
          </StatGroup>
        </Space>
      </Card>
    </PageContainer>
  );
}
