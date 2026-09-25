import React, { useMemo, useRef, useState } from 'react';
import { Card, Space, Input, Button, DatePicker, Tag, Row, Col, Typography } from 'antd';
import type { Dayjs } from 'dayjs';
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { auditRowKey, listAudit, type AuditEvent } from '@/services/api';
import { exportToCSV } from '@/utils/export';
import { formatDateTime } from '@/utils/format';

/** 过滤区小标签：控件分组可视化，全部走 i18n（禁止硬编码中文）。 */
function FilterLabel({ id, defaultMessage }: { id: string; defaultMessage: string }) {
  return (
    <Typography.Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
      <FormattedMessage id={id} defaultMessage={defaultMessage} />
    </Typography.Text>
  );
}

export default function OperationLogsPage() {
  const intl = useIntl();
  const actionRef = useRef<ActionType | undefined>(undefined);
  // 仅为「导出 CSV」保留当前页数据，由 request 成功时更新
  const [rows, setRows] = useState<AuditEvent[]>([]);
  const [actor, setActor] = useState<string>(
    () => new URLSearchParams(location.search).get('actor') || '',
  );
  const [ip, setIP] = useState<string>('');
  // Approvals 详情抽屉等入口经 URL 预置 kind（如 ?kind=approval_approve）：
  // 带参时聚焦单事件类型，否则回默认全集
  const presetKind = useMemo(() => new URLSearchParams(location.search).get('kind') || '', []);
  // 默认展示常见操作类事件，不含登录
  const defaultKinds = useMemo(
    () => [
      'invoke',
      'start_job',
      'cancel_job',
      'assignments.update',
      'user_create',
      'user_update',
      'user_delete',
      'user_set_password',
      'user_set_games',
      'message_send',
      'message_broadcast',
      'approval_approve',
      'approval_reject',
      // support ops
      'support.ticketCreate',
      'support.ticketUpdate',
      'support.ticketDelete',
      'support.ticketComment',
      'support.ticketTransition',
    ],
    [],
  );
  const [kinds, setKinds] = useState<string[]>(() => (presetKind ? [presetKind] : defaultKinds));
  const [timeRange, setTimeRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');

  const exportCSV = () => {
    const arr = (rows || []).map((e: AuditEvent) => [
      new Date(e.time).toISOString(),
      e.kind,
      e.actor,
      e.target,
      String(e.meta?.ip || ''),
      String(e.meta?.ipRegion || ''),
      String(e.meta?.gameId || ''),
      String(e.meta?.env || ''),
      String(e.meta?.traceId || ''),
    ]);
    arr.unshift(['time', 'kind', 'actor', 'target', 'ip', 'region', 'game_id', 'env', 'trace_id']);
    exportToCSV('operation_logs.csv', arr);
  };

  const kindTags = useMemo(() => {
    const list = defaultKinds;
    return (
      <Space size={4} wrap>
        {list.map((k) => (
          <Tag
            key={k}
            color={kinds.includes(k) ? 'blue' : 'default'}
            onClick={() =>
              setKinds((prev) => (prev.includes(k) ? prev.filter((x) => x !== k) : [...prev, k]))
            }
            style={{ cursor: 'pointer' }}
          >
            {k}
          </Tag>
        ))}
      </Space>
    );
  }, [kinds, defaultKinds]);

  const columns: ProColumns<AuditEvent>[] = [
    {
      title: intl.formatMessage({
        id: 'pages.adminLogs.operationLog.column.time',
        defaultMessage: '时间',
      }),
      dataIndex: 'time',
      render: (_, row) => formatDateTime(row.time ?? ''),
    },
    {
      title: intl.formatMessage({
        id: 'pages.adminLogs.operationLog.column.kind',
        defaultMessage: '类型',
      }),
      dataIndex: 'kind',
    },
    {
      title: intl.formatMessage({
        id: 'pages.adminLogs.operationLog.column.actor',
        defaultMessage: '操作者',
      }),
      dataIndex: 'actor',
    },
    {
      title: intl.formatMessage({
        id: 'pages.adminLogs.operationLog.column.target',
        defaultMessage: '目标',
      }),
      dataIndex: 'target',
    },
    { title: 'IP', dataIndex: ['meta', 'ip'] },
    {
      title: intl.formatMessage({
        id: 'pages.adminLogs.operationLog.column.region',
        defaultMessage: '属地',
      }),
      render: (_: unknown, r: AuditEvent) => {
        const v = String(r?.meta?.ipRegion || '');
        if (!v) return '-';
        if (v === '本地')
          return (
            <Tag color="blue">
              <FormattedMessage
                id="pages.adminLogs.operationLog.region.local"
                defaultMessage="本地"
              />
            </Tag>
          );
        if (v === '局域网')
          return (
            <Tag color="geekblue">
              <FormattedMessage
                id="pages.adminLogs.operationLog.region.lan"
                defaultMessage="局域网"
              />
            </Tag>
          );
        return v;
      },
    },
    {
      title: intl.formatMessage({
        id: 'pages.adminLogs.operationLog.column.game',
        defaultMessage: '游戏',
      }),
      dataIndex: ['meta', 'game_id'],
    },
    {
      title: intl.formatMessage({
        id: 'pages.adminLogs.operationLog.column.env',
        defaultMessage: '环境',
      }),
      dataIndex: ['meta', 'env'],
    },
    { title: 'Trace', dataIndex: ['meta', 'trace_id'] },
  ];

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({
          id: 'pages.adminLogs.operationLog.title',
          defaultMessage: '操作日志',
        })}
      >
        {/* 过滤区分两行：输入/时间/操作 → 类型 Tag（窄屏 Col 自动换行） */}
        <div data-testid="operation-log-filters">
          <Row gutter={[12, 12]} align="bottom" style={{ marginBottom: 16 }}>
            <Col xs={24} sm={12} md={4}>
              <FilterLabel id="pages.adminLogs.operationLog.filter.actor" defaultMessage="操作者" />
              <Input
                placeholder={intl.formatMessage({
                  id: 'pages.adminLogs.operationLog.search.actor',
                  defaultMessage: '操作者',
                })}
                value={actor}
                onChange={(e) => setActor(e.target.value)}
                style={{ width: '100%' }}
              />
            </Col>
            <Col xs={24} sm={12} md={4}>
              <FilterLabel id="pages.adminLogs.operationLog.filter.ip" defaultMessage="IP" />
              <Input
                placeholder="IP"
                value={ip}
                onChange={(e) => setIP(e.target.value)}
                style={{ width: '100%' }}
              />
            </Col>
            <Col xs={24} sm={12} md={4}>
              <FilterLabel id="pages.adminLogs.operationLog.filter.game" defaultMessage="游戏" />
              <Input
                placeholder={intl.formatMessage({
                  id: 'pages.adminLogs.operationLog.search.game',
                  defaultMessage: '游戏',
                })}
                value={gameId}
                onChange={(e) => setGameId(e.target.value)}
                style={{ width: '100%' }}
              />
            </Col>
            <Col xs={24} sm={12} md={3}>
              <FilterLabel id="pages.adminLogs.operationLog.filter.env" defaultMessage="环境" />
              <Input
                placeholder={intl.formatMessage({
                  id: 'pages.adminLogs.operationLog.search.env',
                  defaultMessage: '环境',
                })}
                value={env}
                onChange={(e) => setEnv(e.target.value)}
                style={{ width: '100%' }}
              />
            </Col>
            <Col xs={24} sm={24} md={6}>
              <FilterLabel id="pages.adminLogs.operationLog.filter.time" defaultMessage="时间" />
              <DatePicker.RangePicker
                showTime
                value={timeRange as [Dayjs, Dayjs]}
                onChange={(dates) => setTimeRange(dates as [Dayjs | null, Dayjs | null] | null)}
                style={{ width: '100%' }}
              />
            </Col>
            <Col xs={12} sm={12} md={3}>
              <Button
                type="primary"
                block
                onClick={() => {
                  // 回第 1 页并重查：已在第 1 页时 setPageInfo 不触发请求，
                  // 由 reload 兜底；非第 1 页时双触发经 debounce + abort 合并
                  actionRef.current?.setPageInfo?.({ current: 1 });
                  actionRef.current?.reload();
                }}
              >
                <FormattedMessage
                  id="pages.adminLogs.operationLog.action.query"
                  defaultMessage="查询"
                />
              </Button>
            </Col>
            <Col xs={12} sm={12} md={3}>
              <Button onClick={exportCSV} block>
                <FormattedMessage
                  id="pages.adminLogs.operationLog.action.exportCsv"
                  defaultMessage="导出 CSV"
                />
              </Button>
            </Col>
          </Row>
          <Row gutter={[8, 8]} style={{ marginBottom: 16 }}>
            <Col xs={24}>
              <Space size={4} wrap>
                <span>
                  <FormattedMessage
                    id="pages.adminLogs.operationLog.filter.kind"
                    defaultMessage="类型:"
                  />
                </span>
                {kindTags}
              </Space>
            </Col>
          </Row>
        </div>
        <ProTable<AuditEvent>
          actionRef={actionRef}
          rowKey="__rowKey"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ actor, ip, kinds, timeRange, gameId, env }}
          request={async ({
            current = 1,
            pageSize = 20,
            actor: actorFilter,
            ip: ipFilter,
            kinds: kindsFilter,
            timeRange: timeRangeFilter,
            gameId: gameIdFilter,
            env: envFilter,
          }) => {
            const params: Record<string, string | number> = { page: current, size: pageSize };
            if (actorFilter) params.actor = actorFilter;
            if (ipFilter) params.ip = ipFilter;
            if (gameIdFilter) params.gameId = gameIdFilter;
            if (envFilter) params.env = envFilter;
            const want = kindsFilter && kindsFilter.length > 0 ? kindsFilter : defaultKinds;
            params.kinds = want.join(',');
            const range = timeRangeFilter as [Dayjs | null, Dayjs | null] | null | undefined;
            if (range && range[0]) params.start = range[0].toISOString();
            if (range && range[1]) params.end = range[1].toISOString();
            try {
              const r = await listAudit(params);
              // 行 key 在此处落定，不在 rowKey 回调里用 index 兜底——
              // antd 6 已废弃该参数（docs/BUGS.md BUG-011）。
              const withKeys = (r.events || []).map((e, i) => ({
                ...e,
                __rowKey: auditRowKey(e, i),
              }));
              setRows(withKeys);
              return { data: withKeys, total: r.total || 0, success: true };
            } catch {
              // 原实现无本地弹错（依赖全局请求拦截器 toast），保持静默
              return { data: [], total: 0, success: false };
            }
          }}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />
      </Card>
    </PageContainer>
  );
}
