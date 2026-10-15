import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Alert, Button, Space, Table, Tag, Tooltip, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { StandardListSection } from '@/components';
import { getOpsProbeSummary, type ProbeSummaryRow } from '@/services/api/opsProbe';
import { useScopeReload } from '@/hooks/useScopeReload';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

// 探针在线汇总卡（agent-health-recovery-review §13.2）：
// 一行 = 目标 × 执行位置，呈现窗口开合（两态事实）+ 窗口聚合计数 + 最近
// 失败原因 + 数据新鲜度；**不出单一合成分**——权重无真实数据支撑。
// 在线趋势列在 SDK metric 入口接入前显式灰态（未接入即一等状态）。
// 协议目标另带类型/适配器/失败阶段标注（§5.5.3 四态）：不可达（dial 失败）
// 与协议失败（handshake/status 半途失败）在探活列即可区分，无需读原因文本。
export default function ProbeSummarySection() {
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建（Ops/Status 同款陷阱）；经 ref 转发后 load 依赖稳定。
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string>('');
  const [rows, setRows] = useState<ProbeSummaryRow[]>([]);
  const [onlineDataIntegrated, setOnlineDataIntegrated] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const r = await getOpsProbeSummary();
      setRows(r?.rows ?? []);
      setOnlineDataIntegrated(Boolean(r?.onlineDataIntegrated));
    } catch (error) {
      setRows([]);
      setLoadError(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.opsNodes.probe.loadFailed',
            defaultMessage: '探针汇总加载失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // scope（X-Game-ID/X-Env）由全局拦截器注入服务端过滤：切游戏/环境后必须重拉，
  // 否则面板显示的是上一个 scope 的行。
  useScopeReload(load);

  // 异常行前置：先看有问题的，再看正常的；组内按执行位置+目标稳定排序。
  const sorted = useMemo(
    () =>
      [...rows].sort((a, b) => {
        if (a.available !== b.available) return a.available ? 1 : -1;
        const an = `${a.agentId}/${a.target}`;
        const bn = `${b.agentId}/${b.target}`;
        return an.localeCompare(bn);
      }),
    [rows],
  );

  const unavailableCount = useMemo(() => rows.filter((r) => !r.available).length, [rows]);

  // 探活标签（§5.5.3 协议四态标注面）：协议目标不可用时按失败阶段区分
  // 不可达（dial 失败，未进协议交互）与协议失败（handshake/status 半途失败）；
  // 无阶段标记 = 该目标未定义阶段或旧版 agent 未上报，退回内核两态文案。
  const statusLabel = useCallback((available: boolean, r: ProbeSummaryRow): string => {
    if (available) {
      return intlRef.current.formatMessage({
        id: 'pages.opsNodes.probe.available',
        defaultMessage: '可用',
      });
    }
    if (r.probeType !== 'protocol') {
      return intlRef.current.formatMessage({
        id: 'pages.opsNodes.probe.unavailable',
        defaultMessage: '不可用',
      });
    }
    if (r.phase === 'dial') {
      return intlRef.current.formatMessage({
        id: 'pages.opsNodes.probe.unreachable',
        defaultMessage: '不可达',
      });
    }
    if (r.phase) {
      return intlRef.current.formatMessage(
        { id: 'pages.opsNodes.probe.protocolFailed', defaultMessage: '协议失败·{phase}' },
        { phase: r.phase },
      );
    }
    return intlRef.current.formatMessage({
      id: 'pages.opsNodes.probe.unavailable',
      defaultMessage: '不可用',
    });
  }, []);

  const columns = [
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.location',
        defaultMessage: '执行位置',
      }),
      dataIndex: 'agentId',
      width: 190,
      render: (_: unknown, r: ProbeSummaryRow) => (
        <Space orientation="vertical" size={0}>
          <Text>{r.agentId}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {r.gameId && r.env
              ? `${r.gameId} / ${r.env}`
              : intl.formatMessage({
                  id: 'pages.opsNodes.probe.platformScope',
                  defaultMessage: '平台级',
                })}
          </Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.target',
        defaultMessage: '目标',
      }),
      dataIndex: 'target',
      width: 160,
      ellipsis: true,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.semantics',
        defaultMessage: '视角',
      }),
      dataIndex: 'semantics',
      width: 110,
      render: (v?: string) => <Tag color={v === 'readiness' ? 'blue' : 'default'}>{v || '-'}</Tag>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.probeType',
        defaultMessage: '探测类型',
      }),
      dataIndex: 'probeType',
      width: 120,
      render: (_: unknown, r: ProbeSummaryRow) =>
        r.probeType ? (
          <Tag>
            {r.probeType}
            {r.protocol ? `:${r.protocol}` : ''}
          </Tag>
        ) : (
          // 旧版 agent 未上报类型（wire 新增字段）：缺失就显示缺失，不猜类型。
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.opsNodes.probe.probeTypeMissing',
              defaultMessage: '该行数据由旧版 agent 上报，未携带探测类型',
            })}
          >
            <Text type="secondary">-</Text>
          </Tooltip>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.status',
        defaultMessage: '探活',
      }),
      dataIndex: 'available',
      width: 120,
      render: (available: boolean, r: ProbeSummaryRow) => (
        <Tag color={available ? 'green' : 'red'}>{statusLabel(available, r)}</Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.window',
        defaultMessage: '窗口时长',
      }),
      dataIndex: 'windowDurationMs',
      width: 110,
      render: (ms: number | undefined, r: ProbeSummaryRow) =>
        r.windowOpen && typeof ms === 'number' && ms > 0 ? (
          `${(ms / 1000).toFixed(1)}s`
        ) : (
          <Text type="secondary">-</Text>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.checkedFailed',
        defaultMessage: '检查 / 失败',
      }),
      key: 'checkedFailed',
      width: 130,
      render: (_: unknown, r: ProbeSummaryRow) => `${r.checked} / ${r.failed}`,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.successRate',
        defaultMessage: '成功率',
      }),
      dataIndex: 'successRate',
      width: 100,
      render: (rate: number, r: ProbeSummaryRow) =>
        r.checked > 0 ? (
          `${(rate * 100).toFixed(1)}%`
        ) : (
          // checked=0 = 没测过，不是「全成功」——缺失就显示缺失。
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.opsNodes.probe.noSamples',
              defaultMessage: '窗口内没有采样，指标缺失',
            })}
          >
            <Text type="secondary">-</Text>
          </Tooltip>
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.lastReason',
        defaultMessage: '最近失败原因',
      }),
      dataIndex: 'lastReason',
      ellipsis: true,
      render: (v?: string) =>
        v ? <Text style={{ fontSize: 12 }}>{v}</Text> : <Text type="secondary">-</Text>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNodes.probe.column.reportAt',
        defaultMessage: '数据时间',
      }),
      dataIndex: 'lastReportAt',
      width: 170,
      render: (v?: string) => <Text type="secondary">{v || '-'}</Text>,
    },
  ];

  return (
    <StandardListSection
      title={intl.formatMessage({
        id: 'pages.opsNodes.probe.title',
        defaultMessage: '探针汇总',
      })}
      extra={
        <Space>
          {rows.length > 0 ? (
            <Text type="secondary">
              {intl.formatMessage(
                {
                  id: 'pages.opsNodes.probe.resultCount',
                  defaultMessage: `目标 ${rows.length} / 异常 ${unavailableCount}`,
                },
                { total: rows.length, bad: unavailableCount },
              )}
            </Text>
          ) : null}
          <Button
            icon={<ReloadOutlined />}
            onClick={load}
            loading={loading}
            size="small"
            aria-label={intl.formatMessage({
              id: 'pages.opsNodes.probe.refresh',
              defaultMessage: '刷新探针汇总',
            })}
          />
        </Space>
      }
    >
      {loadError ? (
        <Alert type="error" showIcon title={loadError} style={{ marginBottom: 12 }} />
      ) : null}
      {!onlineDataIntegrated ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          title={intl.formatMessage({
            id: 'pages.opsNodes.probe.notIntegrated',
            defaultMessage: '在线趋势：未接入在线数据',
          })}
          description={intl.formatMessage({
            id: 'pages.opsNodes.probe.notIntegratedDesc',
            defaultMessage:
              '在线人数/峰值来源是游戏服务器自身上报，需 SDK 提供 metric 上报入口；未接入前本页只呈现探活事实，不做推导。',
          })}
        />
      ) : null}
      <Table<ProbeSummaryRow>
        rowKey={(r) => `${r.agentId}/${r.target}`}
        size="small"
        dataSource={sorted}
        loading={loading}
        columns={columns}
        tableLayout="fixed"
        scroll={{ x: 1340 }}
        pagination={false}
        locale={{
          emptyText: intl.formatMessage({
            id: 'pages.opsNodes.probe.empty',
            defaultMessage: '暂无探针数据：agent 未配置 probe.targets，或尚未上报采样。',
          }),
        }}
      />
      <Text type="secondary" style={{ fontSize: 12 }}>
        {intl.formatMessage({
          id: 'pages.opsNodes.probe.scopeHint',
          defaultMessage: '列表按顶栏游戏/环境过滤（同环境行 + 平台级行）。',
        })}
      </Text>
    </StandardListSection>
  );
}
