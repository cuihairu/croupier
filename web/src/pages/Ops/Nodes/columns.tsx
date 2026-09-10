import React from 'react';
import type { ColumnsType } from 'antd/es/table';
import { Button, Space, Tag } from 'antd';
import type { NodeRow } from './shared';

type BuildNodeColumnsOptions = {
  onDetail: (record: NodeRow) => void;
  onDrain: (agentId: string) => void;
  onRestart: (agentId: string) => void;
  onCron: (record: NodeRow) => void;
};

// 主表格列 - 只显示关键信息
export function buildNodeColumns({
  onDetail,
  onDrain,
  onRestart,
  onCron,
}: BuildNodeColumnsOptions): ColumnsType<NodeRow> {
  return [
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
      // 三方对账（agent 视角）：agent 自报"我连着的实例"（注册响应带回，
      // 心跳续报）。与归属列不一致 = 路由漂移/半开连接信号。
      title: 'agent 自报',
      dataIndex: ['labels', 'reportedOwner'],
      width: 150,
      ellipsis: true,
      render: (v: unknown, record) => {
        const reported = typeof v === 'string' && v ? v : '';
        if (!reported) return <span style={{ color: '#999' }}>—</span>;
        const claimed = record.labels?.ownerInstance || '';
        if (claimed && reported !== claimed) {
          return (
            <Tag color="red" title={`agent 自报 ${reported}，归属表 ${claimed}——疑似漂移/半开`}>
              {reported} ≠ {claimed}
            </Tag>
          );
        }
        return <Tag color="green">{reported}</Tag>;
      },
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
          <Button size="small" onClick={() => onDetail(r)}>
            详情
          </Button>
          <Button size="small" onClick={() => onDrain(r.agentId)}>
            下线
          </Button>
          <Button size="small" onClick={() => onRestart(r.agentId)}>
            重启
          </Button>
          <Button size="small" onClick={() => onCron(r)}>
            定时任务
          </Button>
        </Space>
      ),
    },
  ];
}
