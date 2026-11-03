/**
 * OPS-011 运维大屏（舰队健康总览）。
 *
 * 数据源：OPS-010 节点资源指标（listOpsNodes 实时 cpu/mem）+ 告警事件流
 * （fetchOpsAlerts）+ 供给运行（listOpsProvisionRuns，今日扩缩容计数）+
 * synthetics 探针结果（listSyntheticsResults，探针失败→过载信号）。
 *
 * 交互/行为：
 * - 5s 轮询（≤5s 延迟），best-effort（单源失败不整体挂）；
 * - 三色健康态：normal(<60) 绿 / pressure(60-85) 橙 / overload(>85 或探针失败) 红，
 *   离线灰态置底；防抖：连续 3 个周期越界才变色；
 * - 断流 30s → 顶部「数据过期」横幅（不展示无依据的旧值）；
 * - 聚合带：全网节点数/在线率/平均负载/今日扩缩容/活动告警；
 * - 点击节点卡下钻：资源历史（hourly 曲线）+ 进程/端口摘要。
 *
 * 已知边界（docs/operations/ops-dashboard.md）：账号负载视图依赖调度编排的
 * 账号分布数据（OPS-002 边界：无账号分布数据源），本页以「待接入」占位，
 * 不伪造数据；三色阈值当前取默认常量（网络级可配待接入）。
 */
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Alert,
  App,
  Badge,
  Button,
  Card,
  Col,
  Descriptions,
  Modal,
  Row,
  Space,
  Statistic,
  Tag,
  Tooltip,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl, useLocation } from '@umijs/max';
import {
  fetchOpsAlerts,
  getAgentMetricsHistory,
  listOpsNodes,
  listOpsProvisionRuns,
  listSyntheticsResults,
  type OpsAlert,
  type OpsNode,
} from '@/services/api/ops';
import {
  buildFleetSummary,
  countCreatedToday,
  debounceTier,
  DEFAULT_HEALTH_THRESHOLDS,
  hasMetrics,
  isStale,
  sortNodesForWall,
  tierOfNode,
  type HealthTier,
} from './health';

const POLL_MS = 5_000;
const STALE_MS = 30_000;
const DEBOUNCE_PERIODS = 3;
const PROBE_WINDOW_MS = 10 * 60_000;

const TIER_COLOR: Record<HealthTier, string> = {
  normal: '#52c41a',
  pressure: '#faad14',
  overload: '#f5222d',
  offline: '#bfbfbf',
  unknown: '#d9d9d9',
};

const TIER_LABEL: Record<HealthTier, string> = {
  normal: '正常',
  pressure: '压力',
  overload: '过载',
  offline: '离线',
  unknown: '无数据',
};

type TierState = { tier: HealthTier; streak: number };

function tierBadge(tier: HealthTier) {
  return <Tag color={TIER_COLOR[tier]}>{TIER_LABEL[tier]}</Tag>;
}

function fmtPct(v?: number) {
  return v == null ? '-' : `${v.toFixed(0)}%`;
}

export default function OpsDashboardPage() {
  const intl = useIntl();
  const location = useLocation();
  const [nodes, setNodes] = useState<OpsNode[]>([]);
  const [alerts, setAlerts] = useState<OpsAlert[]>([]);
  const [todayProvision, setTodayProvision] = useState(0);
  const [loading, setLoading] = useState(true);
  const [lastUpdated, setLastUpdated] = useState<number | null>(null);
  const [detail, setDetail] = useState<OpsNode | null>(null);
  const [detailHistory, setDetailHistory] = useState<Array<{ cpu: number; at: number }>>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const tiersRef = useRef<Record<string, TierState>>({});
  const [, forceRender] = useState(0);
  const probeFailRef = useRef<Set<string>>(new Set());

  // 深链 ?agentId=：预选下钻。
  const deepLinkAgent = useMemo(
    () => new URLSearchParams(location.search).get('agentId'),
    [location.search],
  );

  const currentTier = useCallback(
    (n: OpsNode): HealthTier =>
      tiersRef.current[n.id]?.tier ?? tierOfNode(n, probeFailRef.current.has(n.id)),
    [],
  );

  const poll = useCallback(async () => {
    try {
      const [nodeRes, alertRes, provRes] = await Promise.allSettled([
        listOpsNodes(),
        fetchOpsAlerts(),
        listOpsProvisionRuns(200),
      ]);
      const nextNodes = nodeRes.status === 'fulfilled' ? nodeRes.value.nodes || [] : null;
      if (nextNodes) {
        // 探针失败信号（best-effort：每 distinct scope 拉最近结果，失败静默）。
        const scopes = new Set<string>();
        for (const n of nextNodes) if (n.gameId && n.env) scopes.add(`${n.gameId}|${n.env}`);
        const failed = new Set<string>();
        const probeResults = await Promise.allSettled(
          [...scopes].map((s) => {
            const [g, e] = s.split('|');
            return listSyntheticsResults(g!, e!, 50);
          }),
        );
        const now = Date.now();
        for (const r of probeResults) {
          if (r.status !== 'fulfilled') continue;
          for (const item of r.value.items || []) {
            if (
              item.status === 'failed' &&
              item.createdAt &&
              now - item.createdAt < PROBE_WINDOW_MS
            ) {
              failed.add(item.agentId);
            }
          }
        }
        probeFailRef.current = failed;

        // 防抖推进 + 触发重渲。
        for (const n of nextNodes) {
          const raw = tierOfNode(n, failed.has(n.id), DEFAULT_HEALTH_THRESHOLDS);
          const prev = tiersRef.current[n.id] ?? { tier: tierOfNode(n), streak: 0 };
          tiersRef.current[n.id] = debounceTier(raw, prev.tier, prev.streak, DEBOUNCE_PERIODS);
        }
        setNodes(nextNodes);
        setLastUpdated(Date.now());
      }
      if (alertRes.status === 'fulfilled') {
        setAlerts(alertRes.value.alerts || []);
      }
      if (provRes.status === 'fulfilled') {
        setTodayProvision(
          (provRes.value || []).filter((r) => countCreatedToday(r.createdAt)).length,
        );
      }
    } catch {
      // best-effort：单轮失败保留上一轮数据（过期由 isStale 兜底）。
    } finally {
      setLoading(false);
      forceRender((x) => x + 1);
    }
  }, []);

  useEffect(() => {
    void poll();
    const timer = setInterval(() => void poll(), POLL_MS);
    return () => clearInterval(timer);
  }, [poll]);

  // 深链下钻。
  useEffect(() => {
    if (deepLinkAgent && !detail) {
      const target = nodes.find((n) => n.id === deepLinkAgent || n.hostname === deepLinkAgent);
      if (target) setDetail(target);
    }
  }, [deepLinkAgent, nodes, detail]);

  const openDetail = useCallback(async (n: OpsNode) => {
    setDetail(n);
    setDetailHistory([]);
    setDetailLoading(true);
    try {
      const entries = await getAgentMetricsHistory(n.id, { window: 'hourly', limit: 48 });
      setDetailHistory(
        (entries || [])
          .map((e) => ({
            cpu: e.cpu?.usagePercent ?? 0,
            at: Date.parse(e.timestamp) / 1000 || 0,
          }))
          .filter((e) => e.at > 0),
      );
    } catch {
      setDetailHistory([]);
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const stale = isStale(lastUpdated, Date.now(), STALE_MS);
  const activeAlerts = useMemo(
    () => (alerts || []).filter((a) => a.silenced !== true).length,
    [alerts],
  );
  const sorted = useMemo(
    () => sortNodesForWall(nodes, (n) => currentTier(n)),
    // currentTier 依赖 tiersRef（ref，重渲经 forceRender）；nodes 变化即重排。
    [nodes, forceRender],
  );
  const summary = useMemo(
    () => buildFleetSummary(nodes, (n) => currentTier(n), activeAlerts, todayProvision),
    [nodes, activeAlerts, todayProvision, forceRender],
  );

  return (
    <PageContainer
      title={intl.formatMessage({ id: 'pages.opsDashboard.title', defaultMessage: '运维大屏' })}
      extra={
        <Space>
          <Tag color={stale ? 'red' : 'green'}>{stale ? '数据过期' : '实时'}</Tag>
          <Button size="small" loading={loading} onClick={() => void poll()}>
            刷新
          </Button>
        </Space>
      }
    >
      {stale && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message="数据过期：超过 30s 未收到节点指标刷新，以下为最近一次快照（不视为当前状态）。"
        />
      )}

      {/* 聚合带 */}
      <Card bordered={false} style={{ marginBottom: 16 }}>
        <Row gutter={[16, 16]}>
          <Col xs={12} sm={8} md={4}>
            <Statistic title="全网节点" value={summary.totalNodes} />
          </Col>
          <Col xs={12} sm={8} md={4}>
            <Statistic title="在线率" value={summary.onlineRate} suffix="%" />
          </Col>
          <Col xs={12} sm={8} md={4}>
            <Statistic
              title="平均 CPU"
              value={summary.avgCpu ?? '-'}
              suffix={summary.avgCpu != null ? '%' : ''}
            />
          </Col>
          <Col xs={12} sm={8} md={4}>
            <Statistic
              title="平均内存"
              value={summary.avgMem ?? '-'}
              suffix={summary.avgMem != null ? '%' : ''}
            />
          </Col>
          <Col xs={12} sm={8} md={4}>
            <Statistic title="今日扩缩容" value={summary.todayProvisionCount} />
          </Col>
          <Col xs={12} sm={8} md={4}>
            <Statistic
              title="活动告警"
              value={summary.activeAlerts}
              valueStyle={{ color: summary.activeAlerts > 0 ? '#f5222d' : undefined }}
            />
          </Col>
        </Row>
        <Space style={{ marginTop: 8 }} size="small">
          <Tag color={TIER_COLOR.normal}>
            正常 {summary.onlineCount - summary.pressureCount - summary.overloadCount}
          </Tag>
          <Tag color={TIER_COLOR.pressure}>压力 {summary.pressureCount}</Tag>
          <Tag color={TIER_COLOR.overload}>过载 {summary.overloadCount}</Tag>
          <Tag color={TIER_COLOR.offline}>离线 {summary.offlineCount}</Tag>
        </Space>
      </Card>

      {/* 告警滚动条（实时事件流） */}
      <Card
        bordered={false}
        style={{ marginBottom: 16 }}
        title={
          <Space>
            <span>活动告警</span>
            <Badge count={activeAlerts} size="small" />
          </Space>
        }
      >
        {activeAlerts === 0 ? (
          <span style={{ color: '#999' }}>暂无活动告警</span>
        ) : (
          <div style={{ maxHeight: 72, overflow: 'hidden' }}>
            {alerts
              .filter((a) => a.silenced !== true)
              .slice(0, 6)
              .map((a, i) => {
                const alertName = a.labels?.alertname;
                const alertText =
                  a.summary ||
                  a.instance ||
                  a.service ||
                  (typeof alertName === 'string' ? alertName : '');
                return (
                  <div
                    key={i}
                    style={{
                      padding: '2px 0',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                  >
                    <Tag
                      color={
                        a.severity === 'critical'
                          ? 'red'
                          : a.severity === 'warning'
                            ? 'orange'
                            : 'blue'
                      }
                    >
                      {a.severity || 'info'}
                    </Tag>
                    <span>{alertText}</span>
                    {a.instance ? (
                      <span style={{ color: '#999', marginLeft: 8 }}>{a.instance}</span>
                    ) : null}
                  </div>
                );
              })}
          </div>
        )}
      </Card>

      {/* 节点卡片墙 */}
      <Card bordered={false} title="节点健康" loading={loading && nodes.length === 0}>
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
            gap: 12,
          }}
        >
          {sorted.map((n) => {
            const tier = currentTier(n);
            const cpu = n.cpu?.usagePercent;
            const mem = n.memory?.usagePercent;
            return (
              <div
                key={n.id}
                data-testid={`node-card-${n.id}`}
                onClick={() => void openDetail(n)}
                style={{
                  border: `2px solid ${TIER_COLOR[tier]}`,
                  borderRadius: 8,
                  padding: 12,
                  cursor: 'pointer',
                  background: tier === 'offline' ? '#fafafa' : undefined,
                  opacity: tier === 'offline' ? 0.7 : 1,
                }}
              >
                <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                  <strong style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>
                    {n.hostname || n.id}
                  </strong>
                  {tierBadge(tier)}
                </Space>
                <div style={{ color: '#888', fontSize: 12, margin: '4px 0' }}>
                  {n.labels?.region || n.labels?.zone || '-'} ·{' '}
                  {n.labels?.role || n.type || 'agent'} · {(n.gameId || '') + '/' + (n.env || '')}
                </div>
                <Space size="large">
                  <Tooltip title="CPU 使用率">
                    <span>
                      CPU <strong>{fmtPct(cpu)}</strong>
                    </span>
                  </Tooltip>
                  <Tooltip title="内存使用率">
                    <span>
                      MEM <strong>{fmtPct(mem)}</strong>
                    </span>
                  </Tooltip>
                </Space>
                {!hasMetrics(n) && tier !== 'offline' ? (
                  <div style={{ color: '#d9d9d9', fontSize: 12, marginTop: 4 }}>暂无指标样本</div>
                ) : null}
              </div>
            );
          })}
          {sorted.length === 0 && !loading ? (
            <span style={{ color: '#999' }}>无节点数据</span>
          ) : null}
        </div>
      </Card>

      {/* 账号负载（OPS-002 边界：无账号分布数据源，待接入） */}
      <Card bordered={false} title="账号负载视图" style={{ marginTop: 16 }}>
        <Alert
          type="info"
          showIcon
          message="账号负载分布依赖调度编排的账号分布数据（OPS-002 边界：当前无账号分布数据源）。本视图待账号分布数据接入后展示「各节点承载账号数 vs 容量上限」。"
        />
      </Card>

      {/* 节点下钻 */}
      <Modal
        open={!!detail}
        title={`${detail?.hostname || detail?.id || ''} 详情`}
        onCancel={() => setDetail(null)}
        footer={null}
        width={720}
      >
        {detail ? (
          <div>
            <Descriptions column={2} size="small" bordered style={{ marginBottom: 12 }}>
              <Descriptions.Item label="节点 ID">{detail.id}</Descriptions.Item>
              <Descriptions.Item label="状态">{detail.status || '-'}</Descriptions.Item>
              <Descriptions.Item label="地址">{detail.addr || '-'}</Descriptions.Item>
              <Descriptions.Item label="最近活跃">{detail.lastSeen || '-'}</Descriptions.Item>
              <Descriptions.Item label="CPU">{fmtPct(detail.cpu?.usagePercent)}</Descriptions.Item>
              <Descriptions.Item label="内存">
                {fmtPct(detail.memory?.usagePercent)}
              </Descriptions.Item>
            </Descriptions>
            <Card size="small" title="资源历史（hourly，近 48 点）" loading={detailLoading}>
              {detailHistory.length === 0 ? (
                <span style={{ color: '#999' }}>暂无历史样本</span>
              ) : (
                <div style={{ display: 'flex', alignItems: 'flex-end', height: 80, gap: 1 }}>
                  {detailHistory.map((p, i) => (
                    <div
                      key={i}
                      title={`cpu ${p.cpu.toFixed(0)}% @ ${new Date(p.at * 1000).toLocaleString()}`}
                      style={{
                        flex: 1,
                        height: `${Math.max(4, Math.min(100, p.cpu))}%`,
                        background: p.cpu >= 85 ? '#f5222d' : p.cpu >= 60 ? '#faad14' : '#52c41a',
                      }}
                    />
                  ))}
                </div>
              )}
            </Card>
          </div>
        ) : null}
      </Modal>
    </PageContainer>
  );
}
