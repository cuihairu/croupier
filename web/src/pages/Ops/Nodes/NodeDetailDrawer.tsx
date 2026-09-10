import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Card,
  Descriptions,
  Divider,
  Drawer,
  Progress,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd';
import { Line } from '@ant-design/charts';
import { getAgentMetricsHistory, type MetricsHistoryEntry } from '@/services/api/ops';
import { formatBytes } from '@/utils/format';
import type { NodeRow } from './shared';

const { Text } = Typography;

/** 节点详情抽屉：基本信息 + 当前系统指标 + 历史指标趋势（时间窗切换）。
 * 指标历史拉取小闭环内聚在抽屉内；下线/恢复/重启动作经回调上抛给页面。 */
export default function NodeDetailDrawer({
  node,
  onClose,
  onDrain,
  onUndrain,
  onRestart,
}: {
  node: NodeRow | null;
  onClose: () => void;
  onDrain: (agentId: string) => void;
  onUndrain: (agentId: string) => void;
  onRestart: (agentId: string) => void;
}) {
  const [metricsHistory, setMetricsHistory] = useState<MetricsHistoryEntry[]>([]);
  const [metricsLoading, setMetricsLoading] = useState(false);
  const [metricsMinutes, setMetricsMinutes] = useState(5);

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
    if (node?.agentId) {
      loadMetricsHistory(node.agentId);
    } else {
      setMetricsHistory([]);
    }
  }, [node?.agentId, loadMetricsHistory]);

  return (
    <Drawer title={`节点详情 - ${node?.agentId || ''}`} open={!!node} onClose={onClose} width={600}>
      {node && (
        <Space orientation="vertical" size={24} style={{ width: '100%' }}>
          {/* 基本信息 */}
          <Descriptions title="基本信息" bordered column={2} size="small">
            <Descriptions.Item label="节点 ID">{node.agentId}</Descriptions.Item>
            <Descriptions.Item label="类型">{node.type || 'agent'}</Descriptions.Item>
            <Descriptions.Item label="游戏">{node.gameId || '-'}</Descriptions.Item>
            <Descriptions.Item label="环境">{node.env || '-'}</Descriptions.Item>
            <Descriptions.Item label="IP">{node.ip || '-'}</Descriptions.Item>
            <Descriptions.Item label="RPC 地址">{node.addr || '-'}</Descriptions.Item>
            <Descriptions.Item label="健康状态">
              {node.healthy ? <Tag color="green">健康</Tag> : <Tag color="red">异常</Tag>}
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
                const s = statusMap[node.nodeStatus || ''] || {
                  color: 'default',
                  text: '未知',
                };
                return <Tag color={s.color}>{s.text}</Tag>;
              })()}
            </Descriptions.Item>
            <Descriptions.Item label="TTL">{node.expiresInSec}秒</Descriptions.Item>
            <Descriptions.Item label="最后心跳">{node.lastSeen || '-'}</Descriptions.Item>
          </Descriptions>

          <Divider />

          {/* 系统指标 */}
          <div>
            <Text strong style={{ fontSize: 16, marginBottom: 16, display: 'block' }}>
              系统指标
            </Text>
            {node.cpu || node.memory ? (
              <Space orientation="vertical" size={16} style={{ width: '100%' }}>
                {/* CPU */}
                {node.cpu && (
                  <Card title="CPU" size="small">
                    <Space orientation="vertical" size={8} style={{ width: '100%' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Text>使用率</Text>
                        <Text strong>{node.cpu.usagePercent.toFixed(2)}%</Text>
                      </div>
                      <Progress
                        percent={Math.round(node.cpu.usagePercent * 100) / 100}
                        size="small"
                      />
                      <Descriptions column={2} size="small">
                        <Descriptions.Item label="核心数">{node.cpu.cores}</Descriptions.Item>
                        <Descriptions.Item label="负载 (1m/5m/15m)">
                          {node.cpu.load1m?.toFixed(2)} / {node.cpu.load5m?.toFixed(2)} /{' '}
                          {node.cpu.load15m?.toFixed(2)}
                        </Descriptions.Item>
                      </Descriptions>
                    </Space>
                  </Card>
                )}

                {/* 内存 */}
                {node.memory && (
                  <Card title="内存" size="small">
                    <Space orientation="vertical" size={8} style={{ width: '100%' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Text>使用率</Text>
                        <Text strong>{node.memory.usagePercent.toFixed(2)}%</Text>
                      </div>
                      <Progress
                        percent={Math.round(node.memory.usagePercent * 100) / 100}
                        size="small"
                      />
                      <Descriptions column={2} size="small">
                        <Descriptions.Item label="总量">
                          {formatBytes(node.memory.totalBytes)}
                        </Descriptions.Item>
                        <Descriptions.Item label="已用">
                          {formatBytes(node.memory.usedBytes)}
                        </Descriptions.Item>
                        <Descriptions.Item label="可用">
                          {formatBytes(node.memory.availableBytes)}
                        </Descriptions.Item>
                        <Descriptions.Item label="Swap 已用/总量">
                          {formatBytes(node.memory.swapUsed)} / {formatBytes(node.memory.swapTotal)}
                        </Descriptions.Item>
                      </Descriptions>
                    </Space>
                  </Card>
                )}

                {/* 磁盘 */}
                {node.disks && node.disks.length > 0 && (
                  <Card title="磁盘" size="small">
                    <Space orientation="vertical" size={12} style={{ width: '100%' }}>
                      {node.disks.map((disk, idx) => (
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
                            <Descriptions.Item label="设备">{disk.device || '-'}</Descriptions.Item>
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
                      if (node?.agentId) {
                        const since = new Date(Date.now() - range.value * 60 * 1000).toISOString();
                        loadMetricsHistory(
                          node.agentId,
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
              <Space orientation="vertical" size={16} style={{ width: '100%' }}>
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
            <Button onClick={() => onDrain(node.agentId)} danger>
              下线节点
            </Button>
            <Button onClick={() => onUndrain(node.agentId)}>恢复节点</Button>
            <Button onClick={() => onRestart(node.agentId)}>重启节点</Button>
          </Space>
        </Space>
      )}
    </Drawer>
  );
}
