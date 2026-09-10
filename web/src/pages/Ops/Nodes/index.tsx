import React, { useEffect, useMemo, useState } from 'react';
import { Alert, App, Button, Input, Select, Space, Table } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { listOpsNodes, drainOpsNode, restartOpsNode, undrainOpsNode } from '@/services/api/ops';
import { fetchRegistry } from '@/services/api/registry';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
import { buildNodeColumns } from './columns';
import { normalizeOpsNode, type NodeRow } from './shared';
import NodeDetailDrawer from './NodeDetailDrawer';
import CronJobsDrawer from './CronJobsDrawer';

export default function OpsNodesPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<NodeRow[]>([]);
  const [q, setQ] = useState('');
  const [healthy, setHealthy] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [game, setGame] = useState<string>('');
  const [detailNode, setDetailNode] = useState<NodeRow | null>(null);
  const [cronNode, setCronNode] = useState<NodeRow | null>(null);

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

  // 下线/恢复/重启共用一段 confirm 包装：执行动作 → 成功提示 →（需要时）刷新列表
  const confirmNodeAction = (options: {
    title: string;
    content: string;
    danger?: boolean;
    successText: string;
    action: () => Promise<void>;
    refreshAfter?: boolean;
  }) => {
    modal.confirm({
      title: options.title,
      content: options.content,
      okButtonProps: options.danger ? { danger: true } : undefined,
      onOk: async () => {
        try {
          await options.action();
          message.success(options.successText);
          if (options.refreshAfter) load();
        } catch (e) {
          const msg =
            e instanceof Error
              ? e.message
              : intl.formatMessage({
                  id: 'pages.opsNodes.error.operationFailed',
                  defaultMessage: '操作失败',
                });
          message.error(msg);
        }
      },
    });
  };

  const drain = (id: string) =>
    confirmNodeAction({
      title: intl.formatMessage({ id: 'pages.opsNodes.drain.title', defaultMessage: '下线节点' }),
      content: intl.formatMessage(
        { id: 'pages.opsNodes.drain.content', defaultMessage: `确认将节点 ${id} 标记为下线吗？` },
        { id },
      ),
      danger: true,
      successText: intl.formatMessage({
        id: 'pages.opsNodes.drain.success',
        defaultMessage: '已下线',
      }),
      action: () => drainOpsNode(id),
      refreshAfter: true,
    });
  const undrain = (id: string) =>
    confirmNodeAction({
      title: intl.formatMessage({ id: 'pages.opsNodes.undrain.title', defaultMessage: '恢复节点' }),
      content: intl.formatMessage(
        { id: 'pages.opsNodes.undrain.content', defaultMessage: `确认恢复节点 ${id} 的调度吗？` },
        { id },
      ),
      successText: intl.formatMessage({
        id: 'pages.opsNodes.undrain.success',
        defaultMessage: '已取消下线',
      }),
      action: () => undrainOpsNode(id),
      refreshAfter: true,
    });
  const restart = (id: string) =>
    confirmNodeAction({
      title: intl.formatMessage({ id: 'pages.opsNodes.restart.title', defaultMessage: '重启节点' }),
      content: intl.formatMessage(
        { id: 'pages.opsNodes.restart.content', defaultMessage: `确认重启 ${id} ?` },
        { id },
      ),
      successText: intl.formatMessage({
        id: 'pages.opsNodes.restart.success',
        defaultMessage: '已下发重启',
      }),
      action: () => restartOpsNode(id),
    });

  const cols = buildNodeColumns({
    intl,
    onDetail: setDetailNode,
    onDrain: drain,
    onRestart: restart,
    onCron: setCronNode,
  });

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
    game
      ? intl.formatMessage(
          { id: 'pages.opsNodes.filter.gameItem', defaultMessage: `游戏 ${game}` },
          { value: game },
        )
      : null,
    env
      ? intl.formatMessage(
          { id: 'pages.opsNodes.filter.envItem', defaultMessage: `环境 ${env}` },
          { value: env },
        )
      : null,
    healthy
      ? intl.formatMessage(
          { id: 'pages.opsNodes.filter.healthyItem', defaultMessage: `健康 ${healthy}` },
          { value: healthy },
        )
      : null,
    q.trim()
      ? intl.formatMessage(
          { id: 'pages.opsNodes.filter.searchItem', defaultMessage: `搜索 ${q.trim()}` },
          { value: q.trim() },
        )
      : null,
  ]
    .filter(Boolean)
    .join(' / ');

  return (
    <PageContainer
      title={intl.formatMessage({ id: 'pages.opsNodes.title.main', defaultMessage: '节点维护' })}
      subTitle={intl.formatMessage({
        id: 'pages.opsNodes.title.sub',
        defaultMessage: '查看节点健康状态，并执行下线、恢复和重启等运维动作',
      })}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title={intl.formatMessage({
            id: 'pages.opsNodes.summary.title',
            defaultMessage: '节点概览',
          })}
          description={intl.formatMessage({
            id: 'pages.opsNodes.summary.description',
            defaultMessage:
              '这里优先完成节点排查和运维动作，建议先用筛选缩小范围，再对单个节点执行操作。',
          })}
          items={[
            {
              color: '#1677ff',
              text: intl.formatMessage(
                { id: 'pages.opsNodes.summary.total', defaultMessage: `节点 ${summary.total}` },
                { total: summary.total },
              ),
            },
            {
              color: '#52c41a',
              text: intl.formatMessage(
                {
                  id: 'pages.opsNodes.summary.healthy',
                  defaultMessage: `健康 ${summary.healthyCount}`,
                },
                { count: summary.healthyCount },
              ),
            },
            {
              color: '#d9d9d9',
              text: intl.formatMessage(
                {
                  id: 'pages.opsNodes.summary.unhealthy',
                  defaultMessage: `异常 ${summary.unhealthyCount}`,
                },
                { count: summary.unhealthyCount },
              ),
            },
            {
              color: '#722ed1',
              text: intl.formatMessage(
                { id: 'pages.opsNodes.summary.game', defaultMessage: `游戏 ${summary.gameCount}` },
                { count: summary.gameCount },
              ),
            },
            {
              color: '#13c2c2',
              text: intl.formatMessage(
                { id: 'pages.opsNodes.summary.env', defaultMessage: `环境 ${summary.envCount}` },
                { count: summary.envCount },
              ),
            },
          ]}
          hint={intl.formatMessage({
            id: 'pages.opsNodes.summary.hint',
            defaultMessage:
              '推荐路径：先按游戏、环境和健康状态筛选，再执行下线或重启，避免误操作到无关节点。',
          })}
        />

        <StandardListSection
          title={intl.formatMessage({
            id: 'pages.opsNodes.list.title',
            defaultMessage: '节点列表',
          })}
        >
          <StandardFilterBar
            resultText={intl.formatMessage(
              {
                id: 'pages.opsNodes.list.resultCount',
                defaultMessage: `当前结果 ${data.length} 个节点`,
              },
              { count: data.length },
            )}
            controls={
              <>
                <Select
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.opsNodes.filter.gamePlaceholder',
                    defaultMessage: '游戏',
                  })}
                  value={game}
                  onChange={(val) => setGame(val)}
                  style={{ width: 140 }}
                  options={games}
                />
                <Select
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.opsNodes.filter.envPlaceholder',
                    defaultMessage: '环境',
                  })}
                  value={env}
                  onChange={(val) => setEnv(val)}
                  style={{ width: 120 }}
                  options={envs}
                />
                <Select
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.opsNodes.filter.healthyPlaceholder',
                    defaultMessage: '健康',
                  })}
                  value={healthy}
                  onChange={(val) => setHealthy(val)}
                  style={{ width: 120 }}
                  options={[
                    {
                      label: intl.formatMessage({
                        id: 'pages.opsNodes.filter.healthy',
                        defaultMessage: '健康',
                      }),
                      value: 'healthy',
                    },
                    {
                      label: intl.formatMessage({
                        id: 'pages.opsNodes.filter.unhealthy',
                        defaultMessage: '异常',
                      }),
                      value: 'unhealthy',
                    },
                  ]}
                />
                <Space.Compact style={{ width: 280 }}>
                  <Input
                    allowClear
                    placeholder={intl.formatMessage({
                      id: 'pages.opsNodes.filter.searchPlaceholder',
                      defaultMessage: '搜索节点 ID / IP',
                    })}
                    value={q}
                    onChange={(e) => setQ(e.target.value)}
                    onPressEnter={load}
                  />
                  <Button type="primary" onClick={load}>
                    <FormattedMessage id="pages.opsNodes.list.refresh" defaultMessage="刷新" />
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
                    <FormattedMessage id="pages.opsNodes.filter.clear" defaultMessage="清空筛选" />
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
              message={intl.formatMessage({
                id: 'pages.opsNodes.filter.activeMessage',
                defaultMessage: '当前正在查看筛选后的节点范围',
              })}
              description={intl.formatMessage(
                {
                  id: 'pages.opsNodes.filter.activeDescription',
                  defaultMessage: `已生效条件：${filterSummary}`,
                },
                { filters: filterSummary },
              )}
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
                ? intl.formatMessage({
                    id: 'pages.opsNodes.empty.filtered',
                    defaultMessage: '当前筛选条件下没有匹配节点，请调整筛选后重试。',
                  })
                : intl.formatMessage({
                    id: 'pages.opsNodes.empty.none',
                    defaultMessage: '暂时没有节点数据，请先确认节点注册是否正常。',
                  }),
            }}
          />
        </StandardListSection>
      </Space>

      <NodeDetailDrawer
        node={detailNode}
        onClose={() => setDetailNode(null)}
        onDrain={drain}
        onUndrain={undrain}
        onRestart={restart}
      />

      <CronJobsDrawer node={cronNode} onClose={() => setCronNode(null)} />
    </PageContainer>
  );
}
