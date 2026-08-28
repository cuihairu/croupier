import React, { useEffect, useMemo, useState, useCallback } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Input,
  Modal,
  Progress,
  Select,
  Space,
  Spin,
  Table,
  Tag,
  Drawer,
  Divider,
  Typography,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import {
  drainOpsNode,
  listOpsNodes,
  restartOpsNode,
  undrainOpsNode,
  getAgentMetricsHistory,
  type MetricsHistoryEntry,
  type OpsNode,
} from '@/services/api/ops';
import { fetchRegistry, type RegistryAgent } from '@/services/api/registry';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
import { formatBytes } from '@/utils/format';
import { Line } from '@ant-design/charts';

const { Text } = Typography;

type NodeRow = RegistryAgent & {
  type?: string;
  ip?: string;
  version?: string;
  sdkName?: string;
  sdkLanguage?: string;
  sdkVersion?: string;
  lastSeen?: string;
  nodeStatus?: string;
  labels?: Record<string, string>;
  // System metrics (from detail)
  cpu?: {
    usagePercent: number;
    cores: number;
    perCore?: number[];
    load1m: number;
    load5m: number;
    load15m: number;
  };
  memory?: {
    totalBytes: number;
    usedBytes: number;
    availableBytes: number;
    usagePercent: number;
    swapTotal: number;
    swapUsed: number;
  };
  disks?: Array<{
    mountPoint: string;
    device: string;
    fsType: string;
    totalBytes: number;
    usedBytes: number;
    availableBytes: number;
    usagePercent: number;
    inodeTotal?: number;
    inodeUsed?: number;
  }>;
};

// 从 "host:port" 提取 host 部分作为 IP 展示。
function addrHost(addr?: string): string {
  if (!addr) return '';
  const idx = addr.lastIndexOf(':');
  return idx > 0 ? addr.slice(0, idx) : addr;
}

function normalizeOpsNode(node: OpsNode): NodeRow {
  return {
    agentId: node.id || node.addr || '',
    type: 'agent',
    gameId: node.gameId || '',
    env: node.env || '',
    addr: node.addr || '',
    ip: addrHost(node.addr),
    functions: node.functions || 0,
    healthy: ['active', 'healthy', 'online'].includes(node.status || ''),
    expiresInSec: node.expiresInSec || 0,
    sdkName: node.sdkName || '',
    sdkLanguage: node.sdkLanguage || '',
    sdkVersion: node.sdkVersion || '',
    version: node.sdkVersion || '',
    lastSeen: node.lastSeen || '',
    nodeStatus: node.status || 'active',
    labels: node.labels || {},
    // System metrics
    cpu: node.cpu,
    memory: node.memory,
    disks: node.disks,
  };
}

export default function OpsNodesPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<NodeRow[]>([]);
  const [q, setQ] = useState('');
  const [healthy, setHealthy] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [game, setGame] = useState<string>('');
  const [detailNode, setDetailNode] = useState<NodeRow | null>(null);
  const [metricsHistory, setMetricsHistory] = useState<MetricsHistoryEntry[]>([]);
  const [metricsLoading, setMetricsLoading] = useState(false);
  const [metricsMinutes, setMetricsMinutes] = useState(5);

  const load = async () => {
    setLoading(true);
    try {
      try {
        const r = await listOpsNodes();
        const nodes = r.nodes || [];
        setRows(nodes.map(normalizeOpsNode));
      } catch {
        const r = await fetchRegistry();
        setRows(
          (r.agents || []).map((agent) => ({ ...agent, type: 'agent', ip: '', version: '' })),
        );
      }
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    load();
  }, []);

  const data = useMemo(() => {
    return (rows || []).filter((a) => {
      if (game && a.gameId !== game) return false;
      if (env && a.env !== env) return false;
      if (healthy === 'healthy' && !a.healthy) return false;
      if (healthy === 'unhealthy' && a.healthy) return false;
      if (q) {
        const s = `${a.agentId} ${a.ip || ''} ${a.addr || ''} ${a.type || ''}`.toLowerCase();
        if (!s.includes(q.toLowerCase())) return false;
      }
      return true;
    });
  }, [rows, q, healthy, env, game]);

  const summary = useMemo(() => {
    const total = rows.length;
    const healthyCount = rows.filter((item) => item.healthy).length;
    const unhealthyCount = total - healthyCount;
    const gameCount = new Set(rows.map((item) => item.gameId).filter(Boolean)).size;
    const envCount = new Set(rows.map((item) => item.env).filter(Boolean)).size;
    return { total, healthyCount, unhealthyCount, gameCount, envCount };
  }, [rows]);

  // Load metrics history when detail node changes
  const loadMetricsHistory = useCallback(
    async (agentId: string, since?: string, limit?: number) => {
      if (!agentId) return;
      setMetricsLoading(true);
      try {
        const entries = await getAgentMetricsHistory(agentId, {
          since: since || new Date(Date.now() - metricsMinutes * 60 * 1000).toISOString(),
          limit: limit || 50,
        });
        setMetricsHistory(entries || []);
      } catch (error) {
        console.error('Failed to load metrics history:', error);
        setMetricsHistory([]);
      } finally {
        setMetricsLoading(false);
      }
    },
    [metricsMinutes],
  );

  // Load metrics when detail node changes
  useEffect(() => {
    if (detailNode?.agentId) {
      loadMetricsHistory(detailNode.agentId);
    } else {
      setMetricsHistory([]);
    }
  }, [detailNode?.agentId, loadMetricsHistory]);

  const drain = async (id: string) => {
    Modal.confirm({
      title: '下线节点',
      content: `确认将节点 ${id} 标记为下线吗？`,
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await drainOpsNode(id);
          message.success('已下线');
          load();
        } catch (e) {
          const msg = e instanceof Error ? e.message : '操作失败';
          message.error(msg);
        }
      },
    });
  };
  const undrain = async (id: string) => {
    Modal.confirm({
      title: '恢复节点',
      content: `确认恢复节点 ${id} 的调度吗？`,
      onOk: async () => {
        try {
          await undrainOpsNode(id);
          message.success('已取消下线');
          load();
        } catch (e) {
          const msg = e instanceof Error ? e.message : '操作失败';
          message.error(msg);
        }
      },
    });
  };
  const restart = async (id: string) => {
    Modal.confirm({
      title: '重启节点',
      content: `确认重启 ${id} ?`,
      onOk: async () => {
        try {
          await restartOpsNode(id);
          message.success('已下发重启');
        } catch (e) {
          const msg = e instanceof Error ? e.message : '操作失败';
          message.error(msg);
        }
      },
    });
  };

  // 主表格列 - 只显示关键信息
  const cols: ColumnsType<NodeRow> = [
    { title: '节点 ID', dataIndex: 'agentId', width: 200, ellipsis: true },
    { title: '游戏', dataIndex: 'gameId', width: 100 },
    { title: '环境', dataIndex: 'env', width: 80 },
    { title: 'IP', dataIndex: 'ip', width: 130, ellipsis: true },
    {
      title: '归属实例',
      dataIndex: ['labels', 'ownerInstance'],
      width: 130,
      ellipsis: true,
      render: (v: unknown) =>
        typeof v === 'string' && v ? (
          <Tag color="geekblue">{v}</Tag>
        ) : (
          <span style={{ color: '#999' }}>本实例</span>
        ),
    },
    {
      title: '健康状态',
      dataIndex: 'healthy',
      width: 90,
      render: (v, record) =>
        record.nodeStatus === 'stale' || record.nodeStatus === 'offline' ? (
          <Tag color="red">离线</Tag>
        ) : v ? (
          <Tag color="green">健康</Tag>
        ) : (
          <Tag color="default">异常</Tag>
        ),
    },
    {
      title: '运维状态',
      dataIndex: 'nodeStatus',
      width: 100,
      render: (v: string, record: NodeRow) => {
        const statusMap: Record<string, { color: string; text: string }> = {
          active: { color: 'green', text: '在线' },
          online: { color: 'green', text: '在线' },
          drained: { color: 'orange', text: '已下线' },
          stale: { color: 'red', text: '离线' },
          offline: { color: 'red', text: '离线' },
          restarting: { color: 'blue', text: '重启中' },
        };
        const s = statusMap[v] || { color: 'default', text: v || '未知' };
        // active/online 对运维语义等价（都健康在线），区别只在连接归属
        // 实例——active=本实例直连，online=集群对端持有（HA 转发可达），
        // 归属信息放 title 提示而不是状态文案。
        const owner = record.labels?.ownerInstance;
        if (v === 'online' && owner) {
          return (
            <Tag color={s.color} title={`连接由集群实例 ${owner} 持有`}>
              {s.text}
            </Tag>
          );
        }
        return <Tag color={s.color}>{s.text}</Tag>;
      },
    },
    {
      title: '操作',
      width: 220,
      fixed: 'right',
      render: (_, r) => (
        <Space>
          <Button size="small" onClick={() => setDetailNode(r)}>
            详情
          </Button>
          <Button size="small" onClick={() => drain(r.agentId)}>
            下线
          </Button>
          <Button size="small" onClick={() => restart(r.agentId)}>
            重启
          </Button>
        </Space>
      ),
    },
  ];

  const games = Array.from(new Set(rows.map((r) => r.gameId).filter(Boolean))).map((v) => ({
    label: v,
    value: v,
  }));
  const envs = Array.from(new Set(rows.map((r) => r.env).filter(Boolean))).map((v) => ({
    label: v,
    value: v,
  }));
  const hasFilters = Boolean(game || env || healthy || q.trim());
  const filterSummary = [
    game ? `游戏 ${game}` : null,
    env ? `环境 ${env}` : null,
    healthy ? `健康 ${healthy}` : null,
    q.trim() ? `搜索 ${q.trim()}` : null,
  ]
    .filter(Boolean)
    .join(' / ');

  return (
    <PageContainer title="节点维护" subTitle="查看节点健康状态，并执行下线、恢复和重启等运维动作">
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title="节点概览"
          description="这里优先完成节点排查和运维动作，建议先用筛选缩小范围，再对单个节点执行操作。"
          items={[
            { color: '#1677ff', text: `节点 ${summary.total}` },
            { color: '#52c41a', text: `健康 ${summary.healthyCount}` },
            { color: '#d9d9d9', text: `异常 ${summary.unhealthyCount}` },
            { color: '#722ed1', text: `游戏 ${summary.gameCount}` },
            { color: '#13c2c2', text: `环境 ${summary.envCount}` },
          ]}
          hint="推荐路径：先按游戏、环境和健康状态筛选，再执行下线或重启，避免误操作到无关节点。"
        />

        <StandardListSection title="节点列表">
          <StandardFilterBar
            resultText={`当前结果 ${data.length} 个节点`}
            controls={
              <>
                <Select
                  allowClear
                  placeholder="游戏"
                  value={game}
                  onChange={(val) => setGame(val)}
                  style={{ width: 140 }}
                  options={games}
                />
                <Select
                  allowClear
                  placeholder="环境"
                  value={env}
                  onChange={(val) => setEnv(val)}
                  style={{ width: 120 }}
                  options={envs}
                />
                <Select
                  allowClear
                  placeholder="健康"
                  value={healthy}
                  onChange={(val) => setHealthy(val)}
                  style={{ width: 120 }}
                  options={[
                    { label: '健康', value: 'healthy' },
                    { label: '异常', value: 'unhealthy' },
                  ]}
                />
                <Space.Compact style={{ width: 280 }}>
                  <Input
                    allowClear
                    placeholder="搜索节点 ID / IP"
                    value={q}
                    onChange={(e) => setQ(e.target.value)}
                    onPressEnter={load}
                  />
                  <Button type="primary" onClick={load}>
                    刷新
                  </Button>
                </Space.Compact>
                {hasFilters && (
                  <Button
                    onClick={() => {
                      setGame('');
                      setEnv('');
                      setHealthy('');
                      setQ('');
                    }}
                  >
                    清空筛选
                  </Button>
                )}
              </>
            }
          />
          {hasFilters ? (
            <Alert
              style={{ marginBottom: 12 }}
              type="info"
              showIcon
              message="当前正在查看筛选后的节点范围"
              description={`已生效条件：${filterSummary}`}
            />
          ) : null}
          <Table<NodeRow>
            rowKey={(r) => r.agentId}
            dataSource={data}
            loading={loading}
            columns={cols}
            size="small"
            scroll={{ x: 1000 }}
            tableLayout="fixed"
            pagination={{ pageSize: 10 }}
            locale={{
              emptyText: hasFilters
                ? '当前筛选条件下没有匹配节点，请调整筛选后重试。'
                : '暂时没有节点数据，请先确认节点注册是否正常。',
            }}
          />
        </StandardListSection>
      </Space>

      {/* 节点详情抽屉 */}
      <Drawer
        title={`节点详情 - ${detailNode?.agentId || ''}`}
        open={!!detailNode}
        onClose={() => setDetailNode(null)}
        width={600}
      >
        {detailNode && (
          <Space direction="vertical" size={24} style={{ width: '100%' }}>
            {/* 基本信息 */}
            <Descriptions title="基本信息" bordered column={2} size="small">
              <Descriptions.Item label="节点 ID">{detailNode.agentId}</Descriptions.Item>
              <Descriptions.Item label="类型">{detailNode.type || 'agent'}</Descriptions.Item>
              <Descriptions.Item label="游戏">{detailNode.gameId || '-'}</Descriptions.Item>
              <Descriptions.Item label="环境">{detailNode.env || '-'}</Descriptions.Item>
              <Descriptions.Item label="IP">{detailNode.ip || '-'}</Descriptions.Item>
              <Descriptions.Item label="RPC 地址">{detailNode.addr || '-'}</Descriptions.Item>
              <Descriptions.Item label="健康状态">
                {detailNode.healthy ? <Tag color="green">健康</Tag> : <Tag color="red">异常</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="运维状态">
                {(() => {
                  const statusMap: Record<string, { color: string; text: string }> = {
                    active: { color: 'green', text: '在线' },
                    online: { color: 'green', text: '在线' },
                    drained: { color: 'orange', text: '已下线' },
                    stale: { color: 'red', text: '离线' },
                    offline: { color: 'red', text: '离线' },
                  };
                  const s = statusMap[detailNode.nodeStatus || ''] || {
                    color: 'default',
                    text: '未知',
                  };
                  return <Tag color={s.color}>{s.text}</Tag>;
                })()}
              </Descriptions.Item>
              <Descriptions.Item label="TTL">{detailNode.expiresInSec}秒</Descriptions.Item>
              <Descriptions.Item label="最后心跳">{detailNode.lastSeen || '-'}</Descriptions.Item>
            </Descriptions>

            <Divider />

            {/* 系统指标 */}
            <div>
              <Text strong style={{ fontSize: 16, marginBottom: 16, display: 'block' }}>
                系统指标
              </Text>
              {detailNode.cpu || detailNode.memory ? (
                <Space direction="vertical" size={16} style={{ width: '100%' }}>
                  {/* CPU */}
                  {detailNode.cpu && (
                    <Card title="CPU" size="small">
                      <Space direction="vertical" size={8} style={{ width: '100%' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                          <Text>使用率</Text>
                          <Text strong>{detailNode.cpu.usagePercent.toFixed(2)}%</Text>
                        </div>
                        <Progress
                          percent={Math.round(detailNode.cpu.usagePercent * 100) / 100}
                          size="small"
                        />
                        <Descriptions column={2} size="small">
                          <Descriptions.Item label="核心数">
                            {detailNode.cpu.cores}
                          </Descriptions.Item>
                          <Descriptions.Item label="负载 (1m/5m/15m)">
                            {detailNode.cpu.load1m?.toFixed(2)} /{' '}
                            {detailNode.cpu.load5m?.toFixed(2)} /{' '}
                            {detailNode.cpu.load15m?.toFixed(2)}
                          </Descriptions.Item>
                        </Descriptions>
                      </Space>
                    </Card>
                  )}

                  {/* 内存 */}
                  {detailNode.memory && (
                    <Card title="内存" size="small">
                      <Space direction="vertical" size={8} style={{ width: '100%' }}>
                        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                          <Text>使用率</Text>
                          <Text strong>{detailNode.memory.usagePercent.toFixed(2)}%</Text>
                        </div>
                        <Progress
                          percent={Math.round(detailNode.memory.usagePercent * 100) / 100}
                          size="small"
                        />
                        <Descriptions column={2} size="small">
                          <Descriptions.Item label="总量">
                            {formatBytes(detailNode.memory.totalBytes)}
                          </Descriptions.Item>
                          <Descriptions.Item label="已用">
                            {formatBytes(detailNode.memory.usedBytes)}
                          </Descriptions.Item>
                          <Descriptions.Item label="可用">
                            {formatBytes(detailNode.memory.availableBytes)}
                          </Descriptions.Item>
                          <Descriptions.Item label="Swap 已用/总量">
                            {formatBytes(detailNode.memory.swapUsed)} /{' '}
                            {formatBytes(detailNode.memory.swapTotal)}
                          </Descriptions.Item>
                        </Descriptions>
                      </Space>
                    </Card>
                  )}

                  {/* 磁盘 */}
                  {detailNode.disks && detailNode.disks.length > 0 && (
                    <Card title="磁盘" size="small">
                      <Space direction="vertical" size={12} style={{ width: '100%' }}>
                        {detailNode.disks.map((disk, idx) => (
                          <div key={idx}>
                            <div
                              style={{
                                display: 'flex',
                                justifyContent: 'space-between',
                                marginBottom: 4,
                              }}
                            >
                              <Text>{disk.mountPoint}</Text>
                              <Text strong>{disk.usagePercent.toFixed(2)}%</Text>
                            </div>
                            <Progress
                              percent={Math.round(disk.usagePercent * 100) / 100}
                              size="small"
                            />
                            <Descriptions column={2} size="small" style={{ marginTop: 4 }}>
                              <Descriptions.Item label="设备">
                                {disk.device || '-'}
                              </Descriptions.Item>
                              <Descriptions.Item label="文件系统">
                                {disk.fsType || '-'}
                              </Descriptions.Item>
                              <Descriptions.Item label="已用">
                                {formatBytes(disk.usedBytes)}
                              </Descriptions.Item>
                              <Descriptions.Item label="可用">
                                {formatBytes(disk.availableBytes)}
                              </Descriptions.Item>
                              <Descriptions.Item label="总量">
                                {formatBytes(disk.totalBytes)}
                              </Descriptions.Item>
                            </Descriptions>
                          </div>
                        ))}
                      </Space>
                    </Card>
                  )}
                </Space>
              ) : (
                <Text type="secondary">暂无系统指标数据</Text>
              )}
            </div>

            <Divider />

            {/* 历史指标趋势图 */}
            <div>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 16,
                }}
              >
                <Text strong style={{ fontSize: 16 }}>
                  指标趋势
                </Text>
                <Space>
                  {[
                    { label: '5分钟', value: 5 },
                    { label: '1小时', value: 60 },
                    { label: '6小时', value: 360 },
                    { label: '24小时', value: 1440 },
                    { label: '3天', value: 4320 },
                    { label: '7天', value: 10080 },
                  ].map((range) => (
                    <Button
                      key={range.value}
                      size="small"
                      type={metricsMinutes === range.value ? 'primary' : 'default'}
                      onClick={() => {
                        setMetricsMinutes(range.value);
                        if (detailNode?.agentId) {
                          const since = new Date(
                            Date.now() - range.value * 60 * 1000,
                          ).toISOString();
                          loadMetricsHistory(
                            detailNode.agentId,
                            since,
                            range.value === 5 ? 50 : range.value === 60 ? 120 : 200,
                          );
                        }
                      }}
                    >
                      {range.label}
                    </Button>
                  ))}
                </Space>
              </div>
              {metricsLoading ? (
                <div style={{ textAlign: 'center', padding: '20px 0' }}>
                  <Spin size="small" />
                </div>
              ) : metricsHistory.length > 0 ? (
                <Space direction="vertical" size={16} style={{ width: '100%' }}>
                  {/* CPU 趋势图 */}
                  <Card title="CPU 使用率趋势" size="small">
                    <Line
                      data={metricsHistory.map((entry) => ({
                        time: new Date(entry.timestamp).toLocaleTimeString(),
                        value: Math.round((entry.cpu?.usagePercent ?? 0) * 100) / 100,
                      }))}
                      xField="time"
                      yField="value"
                      smooth
                      point={false}
                      height={200}
                      yAxis={{
                        min: 0,
                        max: 100,
                        label: { formatter: (v: number) => `${v}%` },
                      }}
                    />
                  </Card>

                  {/* 内存趋势图 */}
                  <Card title="内存使用率趋势" size="small">
                    <Line
                      data={metricsHistory.map((entry) => ({
                        time: new Date(entry.timestamp).toLocaleTimeString(),
                        value: Math.round((entry.memory?.usagePercent ?? 0) * 100) / 100,
                      }))}
                      xField="time"
                      yField="value"
                      smooth
                      point={false}
                      height={200}
                      yAxis={{
                        min: 0,
                        max: 100,
                        label: { formatter: (v: number) => `${v}%` },
                      }}
                    />
                  </Card>

                  {/* 磁盘趋势图 */}
                  {metricsHistory[0]?.disks && metricsHistory[0].disks.length > 0 && (
                    <Card title="磁盘使用率趋势" size="small">
                      <Line
                        data={metricsHistory.flatMap((entry) =>
                          (entry.disks || []).map(
                            (disk: NonNullable<MetricsHistoryEntry['disks']>[number]) => ({
                              time: new Date(entry.timestamp).toLocaleTimeString(),
                              value: Math.round((disk.usagePercent ?? 0) * 100) / 100,
                              series: disk.mountPoint,
                            }),
                          ),
                        )}
                        xField="time"
                        yField="value"
                        seriesField="series"
                        smooth
                        point={false}
                        height={200}
                        yAxis={{
                          min: 0,
                          max: 100,
                          label: { formatter: (v: number) => `${v}%` },
                        }}
                      />
                    </Card>
                  )}
                </Space>
              ) : (
                <Text type="secondary">暂无历史数据</Text>
              )}
            </div>

            <Divider />

            {/* 操作 */}
            <Space>
              <Button onClick={() => drain(detailNode.agentId)} danger>
                下线节点
              </Button>
              <Button onClick={() => undrain(detailNode.agentId)}>恢复节点</Button>
              <Button onClick={() => restart(detailNode.agentId)}>重启节点</Button>
            </Space>
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
}
