import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Button, Input, Space, Table } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage } from '@umijs/max';
import { listOpsAgentsByType, type AgentType } from '@/services/api/ops';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
import { normalizeOpsNode, type NodeRow } from '../Nodes/shared';

export type AgentPageProps = {
  agentType: AgentType;
};

type AgentMeta = {
  title: string;
  description: string;
  empty: string;
};

export const AGENT_META: Record<AgentType, AgentMeta> = {
  sidecar: {
    title: '游戏代理',
    description: '查看 sidecar 实例与托管进程状态，再进入进程监管执行授权动作。',
    empty:
      '当前游戏环境暂无 sidecar Agent。请先部署 sidecar 并在其配置中设置 agent.gameId 与 agent.env。',
  },
  devops: {
    title: '运维代理',
    description: '查看 devops 实例与运行状态。主机与会话数据面未交付，本页不提供入口。',
    empty:
      '当前游戏环境暂无 devops Agent。请先部署 devops 并在其配置中设置 agent.gameId 与 agent.env。',
  },
  capture: {
    title: '数据采集代理',
    description: '查看 capture 实例与运行状态。采集明细与策略版本复用既有风险控制页。',
    empty:
      '当前游戏环境暂无 capture Agent。请先部署 capture 并在其配置中设置 agent.gameId 与 agent.env。',
  },
  synthetics: {
    title: '外部探测代理',
    description: '查看 synthetics 实例预留视图。独立运行时未交付，本页不提供目标管理入口。',
    empty: '当前游戏环境暂无 synthetics Agent。',
  },
};

export default function OpsAgentsPage({ agentType }: AgentPageProps) {
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<NodeRow[]>([]);
  const [query, setQuery] = useState('');
  const meta = AGENT_META[agentType];

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await listOpsAgentsByType(agentType);
      setRows((response.nodes || []).map(normalizeOpsNode));
    } finally {
      setLoading(false);
    }
  }, [agentType]);

  useEffect(() => {
    load();
  }, [load]);

  const data = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) return rows;
    return rows.filter((row) =>
      `${row.agentId} ${row.hostname || ''} ${row.addr || ''}`.toLowerCase().includes(keyword),
    );
  }, [query, rows]);

  const summary = useMemo(() => {
    const online = rows.filter(
      (row) => row.nodeStatus === 'active' || row.nodeStatus === 'online',
    ).length;
    return {
      total: rows.length,
      online,
      offline: rows.length - online,
      abnormal: rows.filter((row) => !row.healthy).length,
    };
  }, [rows]);

  return (
    <PageContainer
      title={meta.title}
      subTitle={`${summary.total ? '当前 scope' : '当前 scope'} · ${agentType}`}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title="实例概览"
          description={meta.description}
          items={[
            { color: 'var(--brand-2)', text: `实例 ${summary.total}` },
            { color: '#52c41a', text: `在线 ${summary.online}` },
            { color: '#d9d9d9', text: `离线 ${summary.offline}` },
            { color: '#faad14', text: `职责异常 ${summary.abnormal}` },
          ]}
        />

        <StandardListSection title="实例列表">
          <StandardFilterBar
            resultText={`当前结果 ${data.length} 个实例`}
            controls={
              <Space.Compact style={{ width: 320 }}>
                <Input
                  allowClear
                  placeholder="搜索 Agent ID / 主机"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                />
                <Button type="primary" onClick={load}>
                  <FormattedMessage id="pages.opsNodes.list.refresh" defaultMessage="刷新" />
                </Button>
              </Space.Compact>
            }
          />
          <Table<NodeRow>
            rowKey="agentId"
            dataSource={data}
            loading={loading}
            size="small"
            scroll={{ x: 900 }}
            tableLayout="fixed"
            pagination={{ pageSize: 10 }}
            locale={{ emptyText: rows.length ? '当前筛选条件下没有匹配实例。' : meta.empty }}
            columns={[
              { title: 'Agent ID', dataIndex: 'agentId', width: 200, ellipsis: true },
              { title: 'gameId', dataIndex: 'gameId', width: 120 },
              { title: 'env', dataIndex: 'env', width: 100 },
              {
                title: '类型',
                dataIndex: ['labels', 'role'],
                width: 120,
                render: (value: string) => value || agentType,
              },
              {
                title: '主机',
                dataIndex: 'hostname',
                width: 160,
                ellipsis: true,
                render: (v) => v || '-',
              },
              { title: '状态', dataIndex: 'nodeStatus', width: 110 },
              { title: '版本', dataIndex: 'version', width: 120, render: (v) => v || '-' },
              { title: '最近上报', dataIndex: 'lastSeen', width: 190, render: (v) => v || '-' },
            ]}
          />
        </StandardListSection>
      </Space>
    </PageContainer>
  );
}
