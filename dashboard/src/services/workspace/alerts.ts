import { subscribeWorkspaceTelemetry, type WorkspaceTelemetryEvent } from './telemetry';

export type WorkspaceAlertLevel = 'warning' | 'critical';

export interface WorkspaceAlertRecord {
  id: string;
  ts: number;
  level: WorkspaceAlertLevel;
  title: string;
  event: WorkspaceTelemetryEvent;
  payload?: Record<string, any>;
}

const ALERT_STORAGE_KEY = 'workspace_alert_records';
const MAX_ALERT_RECORDS = 100;
let initialized = false;
let memoryRecords: WorkspaceAlertRecord[] = [];

function buildAlertRecord(
  event: WorkspaceTelemetryEvent,
  payload?: Record<string, any>,
): WorkspaceAlertRecord | null {
  if (event === 'workspace_render_error') {
    return {
      id: `alert_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
      ts: Date.now(),
      level: 'critical',
      title: '关键渲染失败告警',
      event,
      payload,
    };
  }
  if (event === 'workspace_publish_error' || event === 'workspace_unpublish_error') {
    return {
      id: `alert_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
      ts: Date.now(),
      level: 'warning',
      title: '发布后异常告警',
      event,
      payload,
    };
  }
  return null;
}

function pushAlertRecord(record: WorkspaceAlertRecord) {
  if (typeof window === 'undefined') {
    memoryRecords = [record, ...memoryRecords].slice(0, MAX_ALERT_RECORDS);
    return;
  }
  let existing: WorkspaceAlertRecord[] = [];
  try {
    existing = JSON.parse(localStorage.getItem(ALERT_STORAGE_KEY) || '[]');
  } catch {
    existing = memoryRecords;
  }
  const updated = [record, ...existing].slice(0, MAX_ALERT_RECORDS);
  memoryRecords = updated;
  try {
    localStorage.setItem(ALERT_STORAGE_KEY, JSON.stringify(updated));
  } catch {
    // ignore localStorage failure
  }
  try {
    window.dispatchEvent(
      new CustomEvent('croupier:workspace:alert', {
        detail: record,
      }),
    );
  } catch {
    // ignore browser event failure
  }
}

export function recordWorkspaceAlertFromTelemetry(
  event: WorkspaceTelemetryEvent,
  payload?: Record<string, any>,
) {
  const record = buildAlertRecord(event, payload);
  if (!record) return;
  pushAlertRecord(record);
}

export function initWorkspaceAlerting() {
  if (initialized) return;
  initialized = true;
  subscribeWorkspaceTelemetry((telemetryEvent, payload) => {
    recordWorkspaceAlertFromTelemetry(telemetryEvent, payload);
  });
}

export function listWorkspaceAlertRecords(): WorkspaceAlertRecord[] {
  if (typeof window === 'undefined') return memoryRecords;
  try {
    const records = JSON.parse(localStorage.getItem(ALERT_STORAGE_KEY) || '[]');
    if (Array.isArray(records) && records.length > 0) return records;
    if (memoryRecords.length > 0) return memoryRecords;
    if (Array.isArray(records)) return records;
    return memoryRecords;
  } catch {
    return memoryRecords;
  }
}

export function clearWorkspaceAlertRecords() {
  memoryRecords = [];
  if (typeof window === 'undefined') return;
  try {
    localStorage.removeItem(ALERT_STORAGE_KEY);
  } catch {
    // ignore storage failures
  }
}
