import React, { useMemo, useRef, useState } from 'react';
import { Card, Space, Input, Button, DatePicker, Tag } from 'antd';
import type { Dayjs } from 'dayjs';
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { listAudit, type AuditEvent } from '@/services/api';
import { exportToCSV } from '@/utils/export';
import { formatDateTime } from '@/utils/format';

export default function OperationLogsPage() {
  const actionRef = useRef<ActionType | undefined>(undefined);
  // 仅为「导出 CSV」保留当前页数据，由 request 成功时更新
  const [rows, setRows] = useState<AuditEvent[]>([]);
  const [actor, setActor] = useState<string>(
    () => new URLSearchParams(location.search).get('actor') || '',
  );
  const [ip, setIP] = useState<string>('');
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
  const [kinds, setKinds] = useState<string[]>(defaultKinds);
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
    { title: '时间', dataIndex: 'time', render: (_, row) => formatDateTime(row.time ?? '') },
    { title: '类型', dataIndex: 'kind' },
    { title: '操作者', dataIndex: 'actor' },
    { title: '目标', dataIndex: 'target' },
    { title: 'IP', dataIndex: ['meta', 'ip'] },
    {
      title: '属地',
      render: (_: unknown, r: AuditEvent) => {
        const v = String(r?.meta?.ipRegion || '');
        if (!v) return '-';
        if (v === '本地') return <Tag color="blue">本地</Tag>;
        if (v === '局域网') return <Tag color="geekblue">局域网</Tag>;
        return v;
      },
    },
    { title: '游戏', dataIndex: ['meta', 'game_id'] },
    { title: '环境', dataIndex: ['meta', 'env'] },
    { title: 'Trace', dataIndex: ['meta', 'trace_id'] },
  ];

  return (
    <PageContainer>
      <Card title="操作日志">
        <Space style={{ marginBottom: 12 }} wrap>
          <Input
            placeholder="操作者"
            value={actor}
            onChange={(e) => setActor(e.target.value)}
            style={{ width: 160 }}
          />
          <Input
            placeholder="IP"
            value={ip}
            onChange={(e) => setIP(e.target.value)}
            style={{ width: 160 }}
          />
          <Input
            placeholder="游戏"
            value={gameId}
            onChange={(e) => setGameId(e.target.value)}
            style={{ width: 140 }}
          />
          <Input
            placeholder="环境"
            value={env}
            onChange={(e) => setEnv(e.target.value)}
            style={{ width: 120 }}
          />
          {kindTags}
          <DatePicker.RangePicker
            showTime
            value={timeRange as [Dayjs, Dayjs]}
            onChange={(dates) => setTimeRange(dates as [Dayjs | null, Dayjs | null] | null)}
          />
          <Button
            type="primary"
            onClick={() => {
              // 回第 1 页并重查：已在第 1 页时 setPageInfo 不触发请求，
              // 由 reload 兜底；非第 1 页时双触发经 debounce + abort 合并
              actionRef.current?.setPageInfo?.({ current: 1 });
              actionRef.current?.reload();
            }}
          >
            查询
          </Button>
          <Button onClick={exportCSV}>导出 CSV</Button>
        </Space>
        <ProTable<AuditEvent>
          actionRef={actionRef}
          rowKey={(r) => r.hash}
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
              setRows(r.events || []);
              return { data: r.events || [], total: r.total || 0, success: true };
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
