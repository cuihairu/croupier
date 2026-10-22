import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, Space, Table, Tag, Tooltip, Typography } from 'antd';
import { listOpsNodes } from '@/services/api/ops';
import { normalizeOpsNode, type NodeRow } from '../Nodes/shared';
import { getCaptureGates, type CaptureGatesState } from '@/services/api/opsCapture';
import { summarizeGatesApply } from '@/utils/captureGatesApply';
import { useScope } from '@/hooks/useScopeReload';
import { StandardListSection } from '@/components';

/** Capture 页职责面：只读展示当前 scope 已发布的采集闸策略与生效对账。
 *  发布/编辑仍归属风控策略页，这里不重复提供写动作。 */
export default function CaptureGateStatus() {
  const [latest, setLatest] = useState<CaptureGatesState | null>(null);
  const [nodes, setNodes] = useState<NodeRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const requestId = useRef(0);
  const { scope } = useScope();

  const load = useCallback(() => {
    const currentRequestId = ++requestId.current;
    setLatest(null);
    setNodes([]);
    setError('');
    if (!scope.gameId || !scope.env) {
      setLoading(false);
      return;
    }
    setLoading(true);
    Promise.all([
      getCaptureGates(),
      listOpsNodes()
        .then((response) => (response.nodes || []).map(normalizeOpsNode))
        .catch(() => []),
    ])
      .then(([gates, gateNodes]) => {
        if (currentRequestId !== requestId.current) return;
        setLatest(gates);
        setNodes(gateNodes);
      })
      .catch((caught: unknown) => {
        if (currentRequestId === requestId.current) {
          setError(caught instanceof Error ? caught.message : '加载采集策略失败');
        }
      })
      .finally(() => {
        if (currentRequestId === requestId.current) setLoading(false);
      });
  }, [scope.env, scope.gameId]);

  useEffect(() => {
    load();
  }, [load]);

  if (!scope.gameId || !scope.env) {
    return (
      <StandardListSection title="采集策略状态">
        <Alert type="info" showIcon title="请先选择游戏与环境。" />
      </StandardListSection>
    );
  }

  if (error) {
    return (
      <StandardListSection
        title="采集策略状态"
        resultText={`策略 scope · gameId=${scope.gameId} env=${scope.env}`}
      >
        <Alert type="error" showIcon title={error} />
      </StandardListSection>
    );
  }

  const desired = latest?.version ?? 0;
  const scopedNodes = nodes.filter(
    (node) => node.gameId === scope.gameId && node.env === scope.env,
  );
  const summary = summarizeGatesApply(scopedNodes, desired);
  const applyText = !summary
    ? '未发布策略'
    : summary.total === 0
      ? '当前 scope 无在线 capture Agent'
      : `${summary.applied.length}/${summary.total} 台已生效`;

  return (
    <StandardListSection
      title="采集策略状态"
      resultText={`策略 scope · gameId=${scope.gameId} env=${scope.env}`}
    >
      <Space orientation="vertical" size={12} style={{ width: '100%' }}>
        {latest && desired > 0 ? (
          <Space wrap>
            <Typography.Text strong>当前版本 v{desired}</Typography.Text>
            <Tag
              color={
                summary && summary.applied.length === summary.total && summary.total > 0
                  ? 'green'
                  : 'gold'
              }
            >
              {applyText}
            </Tag>
            {latest.createdBy ? <Typography.Text>发布人 {latest.createdBy}</Typography.Text> : null}
            {latest.updatedAt ? <Typography.Text>{latest.updatedAt}</Typography.Text> : null}
          </Space>
        ) : (
          <Alert
            type="info"
            showIcon
            title="当前 scope 尚未发布平台策略，capture Agent 使用本地文件规则。"
          />
        )}
        {summary && summary.stale.length > 0 ? (
          <Alert
            type="warning"
            showIcon
            title="部分 Agent 未生效"
            description={summary.stale
              .map((node) => `${node.agentId}: ${node.labels?.captureGatesRevision || '未上报'}`)
              .join('、')}
          />
        ) : null}
        <Table<NodeRow>
          rowKey="agentId"
          size="small"
          loading={loading}
          dataSource={scopedNodes}
          tableLayout="fixed"
          pagination={false}
          locale={{ emptyText: '当前游戏环境暂无 capture Agent 上报。' }}
          columns={[
            { title: 'Agent', dataIndex: 'agentId', width: 180, ellipsis: true },
            {
              title: '主机',
              dataIndex: 'hostname',
              width: 150,
              ellipsis: true,
              render: (v) => v || '-',
            },
            {
              title: '身份 scope',
              key: 'scope',
              width: 190,
              ellipsis: true,
              render: (_, row) => `gameId=${row.gameId || '-'} env=${row.env || '-'}`,
            },
            {
              title: '生效版本',
              key: 'revision',
              width: 120,
              render: (_, row) => {
                const revision = row.labels?.captureGatesRevision;
                if (!revision) return <Tag>未上报</Tag>;
                const applied = revision === String(desired);
                return (
                  <Tooltip
                    title={applied ? 'Agent 已自报当前 desired 版本' : '版本滞后或 Agent 未热生效'}
                  >
                    <Tag color={applied ? 'green' : 'gold'}>{revision}</Tag>
                  </Tooltip>
                );
              },
            },
            { title: '状态', dataIndex: 'nodeStatus', width: 100 },
          ]}
        />
        <Typography.Text type="secondary">
          发布、编辑与回滚在风控策略页执行；此区块只显示当前 scope 的发布真值与 Agent 自报对账。
        </Typography.Text>
      </Space>
    </StandardListSection>
  );
}
