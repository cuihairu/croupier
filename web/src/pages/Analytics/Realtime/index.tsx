import React, { useEffect, useRef, useState } from 'react';
import { Alert, Card, Space, Button, Row, Col, Statistic, DatePicker, Tag } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { fetchRealtimeSeries, openAnalyticsRealtimeEventSource } from '@/services/api/analytics';

export default function AnalyticsRealtimePage() {
  const intl = useIntl();
  const [data, setData] = useState<any>({});
  const [loading, setLoading] = useState(false);
  const [auto, setAuto] = useState(true);
  const [streamStatus, setStreamStatus] = useState<'connecting' | 'connected' | 'stale' | 'error'>(
    'connecting',
  );
  const [lastMessageAt, setLastMessageAt] = useState<number | null>(null);
  const [ptsOnline, setPtsOnline] = useState<[number, number][]>([]);
  const [ptsA5, setPtsA5] = useState<[number, number][]>([]);
  const [ptsA15, setPtsA15] = useState<[number, number][]>([]);
  const [ptsRev5, setPtsRev5] = useState<[number, number][]>([]);
  const [thrOnline, setThrOnline] = useState<number>(0);
  const [thrA5, setThrA5] = useState<number>(0);
  const [expRange, setExpRange] = useState<any>(null);
  const esRef = useRef<EventSource | null>(null);
  const staleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const normalizeRealtime = (payload: any) => {
    const metrics = payload?.realtimeMetrics || {};
    return {
      ...payload,
      online: payload?.online ?? metrics.onlineUsers ?? 0,
      active_1m: payload?.active_1m ?? metrics.activeSessions ?? 0,
      active_5m: payload?.active_5m ?? metrics.activeSessions ?? 0,
      active_15m: payload?.active_15m ?? metrics.activeSessions ?? 0,
      qps: payload?.qps ?? metrics.qps ?? 0,
      avg_latency: payload?.avg_latency ?? metrics.avgLatency ?? 0,
      error_rate: payload?.error_rate ?? metrics.errorRate ?? 0,
      top_events: payload?.top_events ?? metrics.topEvents ?? [],
    };
  };

  const pushRealtime = (r: any) => {
    const normalized = normalizeRealtime(r || {});
    setData(normalized);
    setLastMessageAt(Date.now());
    setStreamStatus('connected');
    const now = Date.now();
    const online = Number(normalized.online || 0);
    const a5 = Number(normalized.active_5m || 0);
    const a15 = Number(normalized.active_15m || 0);
    const rev5 = Number(normalized.rev_5m || 0);
    const keep = (arr: [number, number][]) =>
      arr.length > 120 ? arr.slice(arr.length - 120) : arr;
    setPtsOnline((prev) => {
      const next = keep(prev.concat([[now, online]]));
      tryPersist(next, undefined, undefined, undefined);
      return next;
    });
    setPtsA5((prev) => {
      const next = keep(prev.concat([[now, a5]]));
      tryPersist(undefined, next, undefined, undefined);
      return next;
    });
    setPtsA15((prev) => {
      const next = keep(prev.concat([[now, a15]]));
      tryPersist(undefined, undefined, next, undefined);
      return next;
    });
    setPtsRev5((prev) => {
      const next = keep(prev.concat([[now, rev5 / 100]]));
      tryPersist(undefined, undefined, undefined, next);
      return next;
    });
  };

  const tryPersist = (
    online?: [number, number][],
    a5?: [number, number][],
    a15?: [number, number][],
    rev5?: [number, number][],
  ) => {
    try {
      const current = sessionStorage.getItem('realtime:series');
      const parsed = current ? JSON.parse(current) : {};
      sessionStorage.setItem(
        'realtime:series',
        JSON.stringify({
          online: online ?? parsed.online ?? ptsOnline,
          a5: a5 ?? parsed.a5 ?? ptsA5,
          a15: a15 ?? parsed.a15 ?? ptsA15,
          rev5: rev5 ?? parsed.rev5 ?? ptsRev5,
        }),
      );
    } catch {}
  };

  const resetStaleTimer = () => {
    if (staleTimerRef.current) {
      clearTimeout(staleTimerRef.current);
    }
    staleTimerRef.current = setTimeout(() => {
      setStreamStatus((prev) => (prev === 'error' ? prev : 'stale'));
    }, 15000);
  };

  const closeStream = () => {
    esRef.current?.close();
    esRef.current = null;
    if (staleTimerRef.current) {
      clearTimeout(staleTimerRef.current);
      staleTimerRef.current = null;
    }
  };

  const connect = (singleShot = false) => {
    closeStream();
    setLoading(true);
    setStreamStatus('connecting');
    const es = openAnalyticsRealtimeEventSource();
    esRef.current = es;
    es.onopen = () => {
      setLoading(false);
      setStreamStatus('connected');
      resetStaleTimer();
    };
    es.addEventListener('connected', () => {
      setLoading(false);
      setStreamStatus('connected');
      resetStaleTimer();
    });
    es.addEventListener('message', (event: MessageEvent) => {
      try {
        const payload = JSON.parse(event.data || '{}');
        pushRealtime(payload);
        resetStaleTimer();
        if (singleShot) {
          closeStream();
          if (esRef.current === es) {
            esRef.current = null;
          }
        }
      } finally {
        setLoading(false);
      }
    });
    es.addEventListener('error', () => {
      setLoading(false);
      setStreamStatus('error');
    });
  };

  const load = async () => {
    connect(!auto);
  };

  useEffect(() => {
    connect();
    return () => {
      closeStream();
    };
  }, []);
  // load thresholds from localStorage
  useEffect(() => {
    try {
      const a = localStorage.getItem('realtime:thrOnline');
      if (a) setThrOnline(Number(a) || 0);
      const b = localStorage.getItem('realtime:thrA5');
      if (b) setThrA5(Number(b) || 0);
    } catch {}
  }, []);
  useEffect(() => {
    try {
      localStorage.setItem('realtime:thrOnline', String(thrOnline || 0));
    } catch {}
  }, [thrOnline]);
  useEffect(() => {
    try {
      localStorage.setItem('realtime:thrA5', String(thrA5 || 0));
    } catch {}
  }, [thrA5]);
  // restore series from sessionStorage once
  useEffect(() => {
    try {
      const txt = sessionStorage.getItem('realtime:series');
      if (txt) {
        const obj = JSON.parse(txt);
        if (Array.isArray(obj?.online)) setPtsOnline(obj.online);
        if (Array.isArray(obj?.a5)) setPtsA5(obj.a5);
        if (Array.isArray(obj?.a15)) setPtsA15(obj.a15);
        if (Array.isArray(obj?.rev5)) setPtsRev5(obj.rev5);
      }
    } catch {}
  }, []);
  useEffect(() => {
    if (!auto) {
      closeStream();
      return;
    }
    connect();
    return () => {
      closeStream();
    };
  }, [auto]);

  const statusTag =
    streamStatus === 'connected' ? (
      <Tag color="green">已连接</Tag>
    ) : streamStatus === 'connecting' ? (
      <Tag color="blue">连接中</Tag>
    ) : streamStatus === 'stale' ? (
      <Tag color="gold">暂未收到新帧</Tag>
    ) : (
      <Tag color="red">连接异常</Tag>
    );

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({ id: 'pages.analytics.realtime.title' }) || '实时大屏'}
        extra={
          <Space>
            {statusTag}
            <span style={{ color: '#666' }}>
              最后更新:
              {lastMessageAt ? ` ${new Date(lastMessageAt).toLocaleTimeString()}` : ' -'}
            </span>
            <Button onClick={load} loading={loading}>
              {intl.formatMessage({ id: 'pages.analytics.realtime.refresh' }) || '刷新'}
            </Button>
            <Button type={auto ? 'primary' : 'default'} onClick={() => setAuto((x) => !x)}>
              {auto
                ? intl.formatMessage({ id: 'pages.analytics.realtime.auto.refresh.on' }) ||
                  '自动刷新:开'
                : intl.formatMessage({ id: 'pages.analytics.realtime.auto.refresh.off' }) ||
                  '自动刷新:关'}
            </Button>
            <Button
              onClick={() => {
                setPtsOnline([]);
                setPtsA5([]);
                setPtsA15([]);
                setPtsRev5([]);
                try {
                  sessionStorage.removeItem('realtime:series');
                } catch {}
              }}
            >
              {intl.formatMessage({ id: 'pages.analytics.realtime.clear.trend' }) || '清空趋势'}
            </Button>
            <span>
              {intl.formatMessage({ id: 'pages.analytics.realtime.threshold' }) ||
                '阈值(在线/5m活跃):'}
            </span>
            <input
              type="number"
              value={thrOnline}
              onChange={(e) => setThrOnline(Number(e.target.value || 0))}
              style={{ width: 80 }}
            />
            <input
              type="number"
              value={thrA5}
              onChange={(e) => setThrA5(Number(e.target.value || 0))}
              style={{ width: 80 }}
            />
            <DatePicker.RangePicker
              showTime
              value={expRange as any}
              onChange={setExpRange as any}
            />
            <Button
              onClick={async () => {
                try {
                  const params: any = {};
                  if (expRange && expRange[0]) params.start = expRange[0].toISOString();
                  if (expRange && expRange[1]) params.end = expRange[1].toISOString();
                  const s = await fetchRealtimeSeries(params);
                  const rows: any[] = [
                    ['ts', 'online', 'active_5m_sum', 'active_15m_sum', 'revenue_cents'],
                  ];
                  const idx: Record<string, any> = {};
                  (s?.online || []).forEach((x: any) => {
                    idx[String(x[0])] = { ts: x[0], online: x[1] };
                  });
                  (s?.active_5m_sum || []).forEach((x: any) => {
                    const k = String(x[0]);
                    idx[k] = idx[k] || { ts: x[0] };
                    idx[k].a5 = x[1];
                  });
                  (s?.active_15m_sum || []).forEach((x: any) => {
                    const k = String(x[0]);
                    idx[k] = idx[k] || { ts: x[0] };
                    idx[k].a15 = x[1];
                  });
                  (s?.revenue_cents || []).forEach((x: any) => {
                    const k = String(x[0]);
                    idx[k] = idx[k] || { ts: x[0] };
                    idx[k].rev = x[1];
                  });
                  const times = Object.keys(idx).sort();
                  for (const t of times) {
                    const it = idx[t];
                    rows.push([it.ts, it.online ?? '', it.a5 ?? '', it.a15 ?? '', it.rev ?? '']);
                  }
                  const csv = rows.map((r) => r.map((x) => String(x ?? '')).join(',')).join('\n');
                  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = 'realtime_window.csv';
                  a.click();
                  URL.revokeObjectURL(url);
                } catch {}
              }}
            >
              {intl.formatMessage({ id: 'pages.analytics.realtime.export.window.csv' }) ||
                '导出窗口 CSV'}
            </Button>
            <Button
              onClick={() => {
                try {
                  const last10 = (arr: [number, number][]) => {
                    const t = Date.now() - 10 * 60 * 1000;
                    return (arr || []).filter((p) => p[0] >= t);
                  };
                  // revenue column exported in Yuan (to match UI/spark)
                  const csvRows = [['ts', 'online', 'active_5m', 'active_15m', 'rev_5m_yuan']];
                  const o = last10(ptsOnline),
                    a5 = last10(ptsA5),
                    a15 = last10(ptsA15),
                    rv = last10(ptsRev5);
                  const idx = new Set<number>([...o, ...a5, ...a15, ...rv].map((p) => p[0]));
                  const times = Array.from(idx).sort((a, b) => a - b);
                  const at = (arr: [number, number][], t: number) => {
                    const f = arr.find((p) => p[0] === t);
                    return f ? f[1] : '';
                  };
                  for (const t of times) {
                    csvRows.push([
                      new Date(t).toISOString(),
                      at(o, t),
                      at(a5, t),
                      at(a15, t),
                      at(rv, t),
                    ] as any);
                  }
                  const csv = csvRows
                    .map((r) => r.map((x) => String(x ?? '')).join(','))
                    .join('\n');
                  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = 'realtime_last10m.csv';
                  a.click();
                  URL.revokeObjectURL(url);
                } catch {}
              }}
            >
              {intl.formatMessage({ id: 'pages.analytics.realtime.export.10min.csv' }) ||
                '导出10分钟CSV'}
            </Button>
          </Space>
        }
      >
        <Alert
          type={streamStatus === 'error' ? 'error' : streamStatus === 'stale' ? 'warning' : 'info'}
          showIcon
          style={{ marginBottom: 16 }}
          message={
            streamStatus === 'error'
              ? '实时流连接异常'
              : streamStatus === 'stale'
              ? '实时流已连接，但当前暂未收到新的数据帧'
              : '实时流已连接；当前没有业务事件时，指标显示为 0 属于正常情况'
          }
        />
        <Row gutter={[16, 16]}>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic
                title="实时在线"
                value={data?.online || 0}
                valueStyle={
                  thrOnline > 0 && Number(data?.online || 0) < thrOnline
                    ? { color: '#cf1322' }
                    : undefined
                }
              />
              <div style={{ marginTop: 6 }}>
                <Spark data={ptsOnline} />
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic title="1分钟活跃" value={data?.active_1m || 0} />
              <div style={{ marginTop: 6 }}>
                <Spark data={[]} />
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic
                title="5分钟活跃"
                value={data?.active_5m || 0}
                valueStyle={
                  thrA5 > 0 && Number(data?.active_5m || 0) < thrA5
                    ? { color: '#cf1322' }
                    : undefined
                }
              />
              <div style={{ marginTop: 6 }}>
                <Spark data={ptsA5} />
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic title="15分钟活跃" value={data?.active_15m || 0} />
              <div style={{ marginTop: 6 }}>
                <Spark data={ptsA15} />
              </div>
            </Card>
          </Col>
        </Row>
        <Row gutter={[16, 16]} style={{ marginTop: 12 }}>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic title="今日峰值在线" value={data?.online_peak_today || 0} />
              <div style={{ marginTop: 6 }}>
                <Spark data={[]} />
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic title="历史峰值在线" value={data?.online_peak_all_time || 0} />
              <div style={{ marginTop: 6 }}>
                <Spark data={[]} />
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic title="今日DAU" value={data?.dau_today || 0} />
              <div style={{ marginTop: 6 }}>
                <Spark data={[]} />
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic title="今日新增" value={data?.new_today || 0} />
              <div style={{ marginTop: 6 }}>
                <Spark data={[]} />
              </div>
            </Card>
          </Col>
        </Row>
        <Row gutter={[16, 16]} style={{ marginTop: 12 }}>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic title="注册用户总数" value={data?.registered_total || 0} />
              <div style={{ marginTop: 6 }}>
                <Spark data={[]} />
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic
                title="5分钟订单额(元)"
                value={Number(data?.rev_5m || 0) / 100}
                precision={2}
                prefix="¥"
              />
              <div style={{ marginTop: 6 }}>
                <Spark data={ptsRev5} />
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic
                title="支付成功率"
                value={data?.pay_succ_rate || 0}
                precision={2}
                suffix="%"
              />
              <div style={{ marginTop: 6 }}>
                <Spark data={[]} />
              </div>
            </Card>
          </Col>
          <Col span={6}>
            <Card loading={loading}>
              <Statistic
                title="今日充值(元)"
                value={Number(data?.rev_today || 0) / 100}
                precision={2}
                prefix="¥"
              />
              <div style={{ marginTop: 6 }}>
                <Spark data={[]} />
              </div>
            </Card>
          </Col>
        </Row>
      </Card>
    </PageContainer>
  );
}

const Spark: React.FC<{ data: [number, number][] }> = ({ data }) => {
  const w = 240,
    h = 40,
    p = 3;
  if (!data || data.length < 2) return <div style={{ height: h }} />;
  const xs = data.map((d) => d[0]);
  const ys = data.map((d) => d[1]);
  const x0 = Math.min(...xs),
    x1 = Math.max(...xs),
    y0 = Math.min(...ys),
    y1 = Math.max(...ys);
  const sx = (x: number) => (x1 === x0 ? p : p + ((w - 2 * p) * (x - x0)) / (x1 - x0));
  const sy = (y: number) => (y1 === y0 ? h - p : h - (p + ((h - 2 * p) * (y - y0)) / (y1 - y0)));
  const dstr = data.map((pt, i) => `${i ? 'L' : 'M'}${sx(pt[0])},${sy(pt[1])}`).join(' ');
  return (
    <svg width={w} height={h} style={{ display: 'block' }}>
      <path d={dstr} fill="none" stroke="#1677ff" strokeWidth={1.8} />
    </svg>
  );
};
