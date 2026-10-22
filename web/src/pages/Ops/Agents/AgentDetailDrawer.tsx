import React from 'react';
import { Button, Descriptions, Drawer, Space, Table, Tag, Typography } from 'antd';
import { useIntl } from '@umijs/max';
import type { NodeRow } from '../Nodes/shared';
import type { AgentType } from '@/services/api/ops';

type AgentDetailDrawerProps = {
  node: NodeRow | null;
  agentType: AgentType;
  onViewAudit: (agentId: string) => void;
};

export default function AgentDetailDrawer({
  node,
  agentType,
  onViewAudit,
}: AgentDetailDrawerProps) {
  const intl = useIntl();
  const missingScope = Boolean(node && (!node.gameId || !node.env));

  return (
    <Drawer
      title={
        node
          ? `${node.agentId} · gameId=${node.gameId || '-'} env=${node.env || '-'}`
          : 'Agent 详情'
      }
      open={Boolean(node)}
      onClose={() => onViewAudit('')}
      destroyOnHidden
    >
      {node ? (
        <Space orientation="vertical" size={16} style={{ width: '100%' }}>
          <Descriptions
            bordered
            size="small"
            column={1}
            title={`身份 scope · gameId=${node.gameId || '-'} env=${node.env || '-'}`}
          >
            <Descriptions.Item label="Agent ID">{node.agentId}</Descriptions.Item>
            <Descriptions.Item label="类型">{agentType}</Descriptions.Item>
            <Descriptions.Item label="gameId">
              {missingScope ? <Tag color="red">未上报</Tag> : node.gameId}
            </Descriptions.Item>
            <Descriptions.Item label="env">
              {missingScope ? <Tag color="red">未上报</Tag> : node.env}
            </Descriptions.Item>
            <Descriptions.Item label="主机">
              {node.hostname || node.reportedIp || node.ip || node.addr || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="版本">{node.version || '未上报'}</Descriptions.Item>
            <Descriptions.Item label="指纹">
              {node.labels?.binarySha256 || '未上报'}
            </Descriptions.Item>
            <Descriptions.Item label="连接状态">
              <Tag
                color={node.healthy ? 'green' : node.nodeStatus === 'offline' ? 'default' : 'gold'}
              >
                {node.nodeStatus}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="最后采集时间">{node.lastSeen || '-'}</Descriptions.Item>
          </Descriptions>

          <Table
            rowKey="name"
            size="small"
            pagination={false}
            dataSource={[
              {
                name: 'Supervisor 快照',
                state: node.supervisor
                  ? `${node.supervisor.running}/${node.supervisor.total} running`
                  : '未上报',
              },
            ]}
            columns={[
              { title: '职责数据', dataIndex: 'name' },
              { title: '状态', dataIndex: 'state' },
            ]}
          />

          <Space wrap>
            <Button type="primary" onClick={() => onViewAudit(node.agentId)}>
              查看执行记录
            </Button>
            {agentType === 'sidecar' ? (
              <Typography.Text type="secondary">
                进程监管入口在实例列表行；此处不重复提供控制动作。
              </Typography.Text>
            ) : null}
          </Space>
          <Typography.Text type="secondary">
            {intl.formatMessage({
              id: 'pages.opsAgents.detail.source',
              defaultMessage: '数据来源：当前 scope 注册表与 Agent 最近上报。',
            })}
          </Typography.Text>
        </Space>
      ) : null}
    </Drawer>
  );
}
