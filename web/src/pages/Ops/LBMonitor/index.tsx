import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { App, Card, Empty, Select, Space, Spin, Statistic, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { Line, Gauge } from '@ant-design/charts';
import {
  fetchClusterInfo,
  listOpsNodes,
  type ClusterLbStatsInfo,
  type OpsNode,
} from '@/services/api/ops';
import { queryLbStats } from '@/services/api/lbStats';
import { extractErrorMessage } from '@/utils/errors';

const { Text, Paragraph } = Typography;

// LB 监控核心指标（PromQL 现成，docs/operations/load-balancing.md「LB 监控」）
const QUERIES = {
  // haproxy 内置 metrics（:8404）的维度标签是 proxy（非 backend）
  backendSessions: 'sum by (proxy) (haproxy_backend_current_sessions)',
  serverStatus: 'haproxy_server_status',
  errors: 'sum by (proxy) (rate(haproxy_backend_errors_total[5m]))',
} as const;

type SeriesPoint = { time: string; value: number; backend: string };

function toSeries(
  rows: { metric: Record<string, string>; value: [number, string] }[],
  labelKey: string,
): SeriesPoint[] {
  const t = new Date();
  return rows.map((r) => ({
    time: t.toLocaleTimeString(),
    value: Number(r.value[1]) || 0,
    // fallback 链覆盖 exporter 标签变体；都缺失时才归为 unknown
    backend:
      r.metric[labelKey] || r.metric.proxy || r.metric.backend || r.metric.instance || 'unknown',
  }));
}

export default function LBMonitor() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [lbStats, setLbStats] = useState<ClusterLbStatsInfo | null>(null);
  // 未配置 Prometheus 时置位：轮询完全停止（后台零请求），只留空态说明
  const [disabled, setDisabled] = useState(false);
  const [nodes, setNodes] = useState<OpsNode[]>([]);
  const [sessionsData, setSessionsData] = useState<SeriesPoint[]>([]);
  const [unhealthy, setUnhealthy] = useState<string[]>([]);
  const [backend, setBackend] = useState<string>('all');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const info = await fetchClusterInfo();
      setLbStats(info.lbStats ?? null);
      if (!info.lbStats?.enabled) {
        // 未配置 Prometheus：不再发起 LB 查询；轮询循环见 disabled 标记
        // 停止（只保留这一次开关探测）
        setLoading(false);
        setDisabled(true);
        return;
      }
      const [sessions, status] = await Promise.all([
        queryLbStats({ query: QUERIES.backendSessions }),
        queryLbStats({ query: QUERIES.serverStatus }),
      ]);
      setSessionsData(toSeries(sessions.data?.result || [], 'proxy'));
      // haproxy_server_status 是 per-state 指标族（server×UP/DOWN/MAINT/
      // DRAIN/NOLB 各一行）——只看 state="UP" 行的值：1=健康，其余=不健康
      const down = (status.data?.result || [])
        .filter((r) => r.metric.state === 'UP' && Number(r.value[1]) !== 1)
        .map((r) => (r.metric.server || r.metric.instance || 'unknown').replace(/^.*\//, ''));
      setUnhealthy(down);
    } catch (error) {
      message.error(extractErrorMessage(error, '加载 LB 监控失败'));
    } finally {
      setLoading(false);
    }
  }, [message]);

  const loadNodes = useCallback(async () => {
    try {
      const r = await listOpsNodes();
      setNodes(r.nodes || []);
    } catch {
      /* nodes 单独失败不打断 LB 图表 */
    }
  }, []);

  useEffect(() => {
    // 首次进入立即拉取；归属数（nodes）不随 LB 状态变化，只拉一次
    void load();
    void loadNodes();
    // LB 状态弱实时：30s 轮询足够；页签不可见时跳过该轮（后台零请求）
    const t = setInterval(() => {
      if (!disabled && !document.hidden) {
        void load();
      }
    }, 30_000);
    const onVisible = () => {
      if (!disabled && !document.hidden) void load();
    };
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      clearInterval(t);
      document.removeEventListener('visibilitychange', onVisible);
    };
  }, [load]);

  const filteredSeries = useMemo(
    () => (backend === 'all' ? sessionsData : sessionsData.filter((p) => p.backend === backend)),
    [backend, sessionsData],
  );

  const backends = useMemo(
    () => Array.from(new Set(sessionsData.map((p) => p.backend))),
    [sessionsData],
  );

  if (!loading && !lbStats?.enabled) {
    return (
      <PageContainer>
        <Card>
          <Empty description="未配置 Prometheus（ops.lbPrometheusUrl），LB 监控不可用" />
        </Card>
      </PageContainer>
    );
  }

  return (
    <PageContainer
      extra={
        <Text type="secondary">
          LB 监控（Prometheus 管道：haproxy exporter → prometheus → 平台代理） · 30s 自动刷新
        </Text>
      }
    >
      <Spin spinning={loading}>
        <Space direction="vertical" style={{ width: '100%' }} size={12}>
          <Card size="small">
            <Space size={32} wrap>
              <Statistic title="后端总数" value={backends.length} />
              <Statistic
                title="不健康后端"
                value={unhealthy.length}
                valueStyle={{ color: unhealthy.length ? '#cf1322' : '#3f8600' }}
              />
              <Statistic title="agent 节点（归属表）" value={nodes.length} />
              <Statistic
                title="LB 会话总数"
                value={sessionsData.reduce((s, p) => s + p.value, 0)}
              />
            </Space>
          </Card>

          {unhealthy.length > 0 && (
            <Card size="small" style={{ borderColor: '#ffa39e' }}>
              <Paragraph type="danger" style={{ margin: 0 }}>
                不健康后端：{unhealthy.join('、')}——TCP 会话可能仍在（半开），注意与 /ops/nodes
                的归属状态对账。
              </Paragraph>
            </Card>
          )}

          <Card
            size="small"
            title="各后端会话分布（current_sessions）"
            extra={
              <Select
                size="small"
                style={{ width: 200 }}
                value={backend}
                onChange={setBackend}
                options={[
                  { label: '全部后端', value: 'all' },
                  ...backends.map((b) => ({ label: b, value: b })),
                ]}
              />
            }
          >
            {filteredSeries.length === 0 ? (
              <Empty description="暂无数据（确认 prometheus 已抓取 haproxy /metrics）" />
            ) : (
              <Line
                height={280}
                data={filteredSeries}
                xField="time"
                yField="value"
                colorField="backend"
                xAxis={{ label: { autoRotate: true } }}
              />
            )}
          </Card>

          <Card size="small" title="归属 vs LB 对账（僵尸探测）">
            <Text type="secondary">
              归属表 agent 数与 LB 会话数长期不一致（连接在、心跳停）= 半开连接信号， 结合
              /ops/nodes 的「agent 自报」列定位。
            </Text>
            <div style={{ marginTop: 12 }}>
              <Gauge
                height={160}
                percent={
                  nodes.length > 0 ? Math.min(nodes.length / Math.max(backends.length, 1), 1) : 0
                }
                innerRadius={0.7}
                annotations={{
                  0.5: {
                    content: { content: `归属 ${nodes.length} / LB 后端 ${backends.length}` },
                  },
                }}
              />
            </div>
          </Card>
        </Space>
      </Spin>
    </PageContainer>
  );
}
