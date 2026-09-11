import React, { useState } from 'react';
import { Button, DatePicker, Space, Tag } from 'antd';
import type { Dayjs } from 'dayjs';
import { FormattedMessage, useIntl } from '@umijs/max';
import { fetchRealtimeSeries } from '@/services/api/analytics';
import { exportToCSV } from '@/utils/export';
import type { ExportRow, RealtimeSeriesResponse, StreamStatus } from './types';

/** 实时大屏工具栏：连接状态/最后更新/刷新/自动刷新/清空趋势/
 * 阈值输入/窗口 CSV 与近 10 分钟 CSV 导出。 */
export default function Toolbar({
  streamStatus,
  lastMessageAt,
  loading,
  auto,
  onToggleAuto,
  onRefresh,
  onClearTrend,
  thrOnline,
  onThrOnlineChange,
  thrA5,
  onThrA5Change,
  ptsOnline,
  ptsA5,
  ptsA15,
  ptsRev5,
}: {
  streamStatus: StreamStatus;
  lastMessageAt: number | null;
  loading: boolean;
  auto: boolean;
  onToggleAuto: () => void;
  onRefresh: () => void;
  onClearTrend: () => void;
  thrOnline: number;
  onThrOnlineChange: (value: number) => void;
  thrA5: number;
  onThrA5Change: (value: number) => void;
  ptsOnline: [number, number][];
  ptsA5: [number, number][];
  ptsA15: [number, number][];
  ptsRev5: [number, number][];
}) {
  const intl = useIntl();
  const [expRange, setExpRange] = useState<[Dayjs | null, Dayjs | null] | null>(null);

  const statusTag =
    streamStatus === 'connected' ? (
      <Tag color="green">
        {intl.formatMessage({
          id: 'pages.analyticsRealtime.toolbar.status.connected',
          defaultMessage: '已连接',
        })}
      </Tag>
    ) : streamStatus === 'connecting' ? (
      <Tag color="blue">
        {intl.formatMessage({
          id: 'pages.analyticsRealtime.toolbar.status.connecting',
          defaultMessage: '连接中',
        })}
      </Tag>
    ) : streamStatus === 'stale' ? (
      <Tag color="gold">
        {intl.formatMessage({
          id: 'pages.analyticsRealtime.toolbar.status.stale',
          defaultMessage: '暂未收到新帧',
        })}
      </Tag>
    ) : (
      <Tag color="red">
        {intl.formatMessage({
          id: 'pages.analyticsRealtime.toolbar.status.error',
          defaultMessage: '连接异常',
        })}
      </Tag>
    );

  return (
    <Space>
      {statusTag}
      <span style={{ color: '#666' }}>
        <FormattedMessage
          id="pages.analyticsRealtime.toolbar.lastUpdated"
          defaultMessage="最后更新:"
        />
        {lastMessageAt ? ` ${new Date(lastMessageAt).toLocaleTimeString()}` : ' -'}
      </span>
      <Button onClick={onRefresh} loading={loading}>
        {intl.formatMessage({
          id: 'pages.analyticsRealtime.toolbar.refresh',
          defaultMessage: '刷新',
        })}
      </Button>
      <Button type={auto ? 'primary' : 'default'} onClick={onToggleAuto}>
        {auto
          ? intl.formatMessage({
              id: 'pages.analyticsRealtime.toolbar.autoRefreshOn',
              defaultMessage: '自动刷新:开',
            })
          : intl.formatMessage({
              id: 'pages.analyticsRealtime.toolbar.autoRefreshOff',
              defaultMessage: '自动刷新:关',
            })}
      </Button>
      <Button onClick={onClearTrend}>
        {intl.formatMessage({
          id: 'pages.analyticsRealtime.toolbar.clearTrend',
          defaultMessage: '清空趋势',
        })}
      </Button>
      <span>
        {intl.formatMessage({
          id: 'pages.analyticsRealtime.toolbar.threshold',
          defaultMessage: '阈值(在线/5m活跃):',
        })}
      </span>
      <input
        type="number"
        value={thrOnline}
        onChange={(e) => onThrOnlineChange(Number(e.target.value || 0))}
        style={{ width: 80 }}
      />
      <input
        type="number"
        value={thrA5}
        onChange={(e) => onThrA5Change(Number(e.target.value || 0))}
        style={{ width: 80 }}
      />
      <DatePicker.RangePicker
        showTime
        value={expRange as [Dayjs, Dayjs]}
        onChange={(dates) => setExpRange(dates as [Dayjs | null, Dayjs | null] | null)}
      />
      <Button
        onClick={async () => {
          try {
            const params: Record<string, string> = {};
            if (expRange && expRange[0]) params.start = expRange[0].toISOString();
            if (expRange && expRange[1]) params.end = expRange[1].toISOString();
            const s = (await fetchRealtimeSeries(params)) as RealtimeSeriesResponse;
            const rows: ExportRow[] = [
              ['ts', 'online', 'active_5m_sum', 'active_15m_sum', 'revenue_cents'],
            ];
            const idx: Record<
              string,
              {
                ts: string | number;
                online?: number;
                a5?: number;
                a15?: number;
                rev?: number;
              }
            > = {};
            (s?.online || []).forEach((x) => {
              idx[String(x[0])] = { ts: x[0], online: x[1] };
            });
            (s?.active5MSum || []).forEach((x) => {
              const k = String(x[0]);
              idx[k] = idx[k] || { ts: x[0] };
              idx[k].a5 = x[1];
            });
            (s?.active15MSum || []).forEach((x) => {
              const k = String(x[0]);
              idx[k] = idx[k] || { ts: x[0] };
              idx[k].a15 = x[1];
            });
            (s?.revenueCents || []).forEach((x) => {
              const k = String(x[0]);
              idx[k] = idx[k] || { ts: x[0] };
              idx[k].rev = x[1];
            });
            const times = Object.keys(idx).sort();
            for (const t of times) {
              const it = idx[t];
              rows.push([it.ts, it.online ?? '', it.a5 ?? '', it.a15 ?? '', it.rev ?? '']);
            }
            exportToCSV('realtime_window.csv', rows);
          } catch {}
        }}
      >
        {intl.formatMessage({
          id: 'pages.analyticsRealtime.toolbar.exportWindowCsv',
          defaultMessage: '导出窗口 CSV',
        })}
      </Button>
      <Button
        onClick={() => {
          try {
            const last10 = (arr: [number, number][]) => {
              const t = Date.now() - 10 * 60 * 1000;
              return (arr || []).filter((p) => p[0] >= t);
            };
            // revenue column exported in Yuan (to match UI/spark)
            const csvRows: string[][] = [
              ['ts', 'online', 'active_5m', 'active_15m', 'rev_5m_yuan'],
            ];
            const o = last10(ptsOnline),
              a5 = last10(ptsA5),
              a15 = last10(ptsA15),
              rv = last10(ptsRev5);
            const idx = new Set<number>([...o, ...a5, ...a15, ...rv].map((p) => p[0]));
            const times = Array.from(idx).sort((a, b) => a - b);
            const at = (arr: [number, number][], t: number): string => {
              const f = arr.find((p) => p[0] === t);
              return f ? String(f[1]) : '';
            };
            for (const t of times) {
              csvRows.push([new Date(t).toISOString(), at(o, t), at(a5, t), at(a15, t), at(rv, t)]);
            }
            exportToCSV('realtime_last10m.csv', csvRows);
          } catch {}
        }}
      >
        {intl.formatMessage({
          id: 'pages.analyticsRealtime.toolbar.exportLast10mCsv',
          defaultMessage: '导出10分钟CSV',
        })}
      </Button>
    </Space>
  );
}
