import { useCallback, useEffect, useRef, useState } from 'react';
import { openAnalyticsRealtimeEventSource } from '@/services/api/analytics';
import { STALE_AFTER_MS, type RealtimeData, type StreamStatus } from './types';

/** 实时大屏 SSE 流 hook：连接管理（stale 判定）、指标归一化、
 * 四条趋势序列（120 点环形裁剪 + sessionStorage 持久化恢复）、
 * 阈值 localStorage 持久化与自动刷新开关。 */
export function useRealtimeStream() {
  const [data, setData] = useState<RealtimeData>({});
  const [loading, setLoading] = useState(false);
  const [auto, setAuto] = useState(true);
  const [streamStatus, setStreamStatus] = useState<StreamStatus>('connecting');
  const [lastMessageAt, setLastMessageAt] = useState<number | null>(null);
  const [ptsOnline, setPtsOnline] = useState<[number, number][]>([]);
  const [ptsA5, setPtsA5] = useState<[number, number][]>([]);
  const [ptsA15, setPtsA15] = useState<[number, number][]>([]);
  const [ptsRev5, setPtsRev5] = useState<[number, number][]>([]);
  const [thrOnline, setThrOnline] = useState<number>(0);
  const [thrA5, setThrA5] = useState<number>(0);
  const esRef = useRef<EventSource | null>(null);
  const staleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const normalizeRealtime = (payload: RealtimeData): RealtimeData => {
    const metrics = payload?.realtimeMetrics || {};
    return {
      ...payload,
      online: payload?.online ?? metrics.onlineUsers ?? 0,
      active1M: payload?.active1M ?? metrics.activeSessions ?? 0,
      active5M: payload?.active5M ?? metrics.activeSessions ?? 0,
      active15M: payload?.active15M ?? metrics.activeSessions ?? 0,
      qps: payload?.qps ?? metrics.qps ?? 0,
      avgLatency: payload?.avgLatency ?? metrics.avgLatency ?? 0,
      errorRate: payload?.errorRate ?? metrics.errorRate ?? 0,
      topEvents: payload?.topEvents ?? metrics.topEvents ?? [],
    };
  };

  const tryPersist = useCallback(
    (
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
    },
    [ptsOnline, ptsA5, ptsA15, ptsRev5],
  );

  const pushRealtime = useCallback(
    (r: RealtimeData) => {
      const normalized = normalizeRealtime(r || {});
      setData(normalized);
      setLastMessageAt(Date.now());
      setStreamStatus('connected');
      const now = Date.now();
      const online = Number(normalized.online || 0);
      const a5 = Number(normalized.active5M || 0);
      const a15 = Number(normalized.active15M || 0);
      const rev5 = Number(normalized.rev5M || 0);
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
    },
    [tryPersist],
  );

  // Frame cadence comes from the server SSE config (sse.updateInterval,
  // default 60s). Mark the stream stale only after missing ~2 frames plus
  // the keep-alive margin, instead of a hardcoded 15s that would flag a
  // healthy 60s stream as stale.
  const resetStaleTimer = () => {
    if (staleTimerRef.current) {
      clearTimeout(staleTimerRef.current);
    }
    staleTimerRef.current = setTimeout(() => {
      setStreamStatus((prev) => (prev === 'error' ? prev : 'stale'));
    }, STALE_AFTER_MS);
  };

  const closeStream = () => {
    esRef.current?.close();
    esRef.current = null;
    if (staleTimerRef.current) {
      clearTimeout(staleTimerRef.current);
      staleTimerRef.current = null;
    }
  };

  const connect = useCallback(
    (singleShot = false) => {
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
    },
    [pushRealtime],
  );

  const refresh = async () => {
    connect(!auto);
  };

  useEffect(() => {
    connect();
    return () => {
      closeStream();
    };
  }, [connect]);
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
  }, [auto, connect]);

  const clearTrend = () => {
    setPtsOnline([]);
    setPtsA5([]);
    setPtsA15([]);
    setPtsRev5([]);
    try {
      sessionStorage.removeItem('realtime:series');
    } catch {}
  };

  return {
    data,
    loading,
    auto,
    setAuto,
    streamStatus,
    lastMessageAt,
    ptsOnline,
    ptsA5,
    ptsA15,
    ptsRev5,
    thrOnline,
    setThrOnline,
    thrA5,
    setThrA5,
    refresh,
    clearTrend,
  };
}
