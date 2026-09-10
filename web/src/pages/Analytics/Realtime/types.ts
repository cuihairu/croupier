export type SeriesPoint = [number | string, number];

export type RealtimeSeriesResponse = {
  online?: SeriesPoint[];
  active5MSum?: SeriesPoint[];
  active15MSum?: SeriesPoint[];
  revenueCents?: SeriesPoint[];
};

export type ExportRow = Array<string | number>;

export interface RealtimeData {
  online?: number;
  active1M?: number;
  active5M?: number;
  active15M?: number;
  qps?: number;
  avgLatency?: number;
  errorRate?: number;
  topEvents?: { event: string; count: number }[];
  rev5M?: number;
  onlinePeakToday?: number;
  onlinePeakAllTime?: number;
  dauToday?: number;
  newToday?: number;
  registeredTotal?: number;
  paySuccRate?: number;
  revToday?: number;
  realtimeMetrics?: {
    onlineUsers?: number;
    activeSessions?: number;
    qps?: number;
    avgLatency?: number;
    errorRate?: number;
    topEvents?: { event: string; count: number }[];
  };
}

export type StreamStatus = 'connecting' | 'connected' | 'stale' | 'error';

// Server pushes one frame per sse.updateInterval (default 60s); allow two
// missed frames plus the 30s keep-alive margin before showing "stale".
export const STALE_AFTER_MS = 150_000;
