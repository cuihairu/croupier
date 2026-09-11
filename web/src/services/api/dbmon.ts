import { getIntl, request } from '@umijs/max';

// Source: internal/api/dbmon/dto.go
export type DBSource = {
  id: number;
  name: string;
  driver: string; // mysql | postgres
  kind: string; // self | aliyun | huawei
  dsnMask?: string;
  gameId?: string;
  env?: string;
  enabled: boolean;
  sort: number;
  lockWaitWarn?: number;
  connWarnRatio?: number;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export async function listDBSources(): Promise<{ items: DBSource[] }> {
  return request('/api/v1/dbmon/sources');
}

export async function createDBSource(payload: {
  name: string;
  driver: string;
  kind?: string;
  dsn: string;
  gameId?: string;
  env?: string;
  lockWaitWarn?: number;
  connWarnRatio?: number;
}): Promise<DBSource> {
  return request('/api/v1/dbmon/sources', { method: 'POST', data: payload });
}

export async function updateDBSource(
  id: number,
  payload: {
    name?: string;
    driver?: string;
    kind?: string;
    dsn?: string;
    gameId?: string;
    env?: string;
    enabled?: boolean;
    lockWaitWarn?: number;
    connWarnRatio?: number;
  },
): Promise<DBSource> {
  return request(`/api/v1/dbmon/sources/${id}`, { method: 'PUT', data: payload });
}

export async function deleteDBSource(id: number): Promise<void> {
  return request(`/api/v1/dbmon/sources/${id}`, { method: 'DELETE' });
}

// Source: internal/api/dbmon/probe.go ProbeResult
export type LockWait = {
  waitId: string;
  blockedBy: string;
  table?: string;
  waitSecs: number;
  query?: string;
};

export type ProbeResult = {
  sourceId: number;
  name: string;
  driver: string;
  kind: string;
  ok: boolean;
  error?: string;
  latencyMs?: number;
  connections?: { current: number; max: number; active: number };
  lockWaits?: LockWait[];
  deadlockCount?: number;
  deadlockNote?: string;
  queryCount?: number;
  txnCount?: number;
  probedAt: string;
};

export function probeAll(): Promise<{ results: ProbeResult[] }> {
  return request('/api/v1/dbmon/probe', { method: 'POST' });
}

export type DBSourceKind =
  | 'self'
  | 'aliyun'
  | 'tencent'
  | 'huawei'
  | 'baidu'
  | 'volc'
  | 'jdcloud'
  | 'ucloud'
  | 'qingcloud'
  | 'tidb'
  | 'oceanbase'
  | 'other';

// 展示 label 经 getIntl 解析（SelectLang 切换语言会整页刷新重新求值）；
// Map key / options value 为后端枚举契约，保持不动
const intl = getIntl();

export const dbKindLabels: Record<DBSourceKind, string> = {
  self: intl.formatMessage({ id: 'services.dbmon.kindLabel.self', defaultMessage: '自建' }),
  aliyun: intl.formatMessage({
    id: 'services.dbmon.kindLabel.aliyun',
    defaultMessage: '阿里云',
  }),
  tencent: intl.formatMessage({
    id: 'services.dbmon.kindLabel.tencent',
    defaultMessage: '腾讯云',
  }),
  huawei: intl.formatMessage({
    id: 'services.dbmon.kindLabel.huawei',
    defaultMessage: '华为云',
  }),
  baidu: intl.formatMessage({
    id: 'services.dbmon.kindLabel.baidu',
    defaultMessage: '百度智能云',
  }),
  volc: intl.formatMessage({
    id: 'services.dbmon.kindLabel.volc',
    defaultMessage: '火山引擎',
  }),
  jdcloud: intl.formatMessage({
    id: 'services.dbmon.kindLabel.jdcloud',
    defaultMessage: '京东云',
  }),
  ucloud: 'UCloud',
  qingcloud: intl.formatMessage({
    id: 'services.dbmon.kindLabel.qingcloud',
    defaultMessage: '青云',
  }),
  tidb: 'TiDB',
  oceanbase: 'OceanBase',
  other: intl.formatMessage({ id: 'services.dbmon.kindLabel.other', defaultMessage: '其他' }),
};
