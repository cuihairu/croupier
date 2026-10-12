/**
 * capture.gates 三段闸表单的纯数据层（web 侧镜像 agents/capture/config
 * Gates + gate.*Rule 形状；列集变更需两侧同步）。
 *
 * 定位：表单是 JSON 生产者——所有编辑最终都序列化回 capture.gates wire
 * JSON，走既有的 updateContent → 校验/发布链（校验真值与发布路径零改动）。
 * parse 容错（存量内容缺段/类型漂移时兜底空值），serialize 恒输出全四段
 * （空数组 = 只采不限，与 agent 配置文件语义一致）。
 */
export type GateSeverity = 'warn' | 'critical';

export interface GatesReconcileRule {
  name: string;
  itemTable: string;
  ledgerTable: string;
  itemKey: string;
  ledgerKey: string;
  windowSec: number;
  severity: GateSeverity | '';
  exemptKeys: string[];
  maxPending: number;
}

export interface GatesOffHoursWindow {
  startMin: number;
  endMin: number;
}

export interface GatesOffHoursRule {
  name: string;
  table: string;
  ops: string[];
  windows: GatesOffHoursWindow[];
  tzOffsetMin: number;
  severity: GateSeverity | '';
}

export interface GatesBurstRule {
  name: string;
  table: string;
  ops: string[];
  deltaField: string;
  threshold: number;
  mode: 'single' | 'window' | '';
  windowSec: number;
  maxEvents: number;
  severity: GateSeverity | '';
}

export interface GatesProbeRule {
  name: string;
  table: string;
  ops: string[];
  keyField: string;
  deltaField: string;
  smallBelow: number;
  count: number;
  windowSec: number;
  maxTracked: number;
  severity: GateSeverity | '';
}

export interface GatesLineageRule {
  name: string;
  table: string;
  ops: string[];
  allowAccounts: string[];
  severity: GateSeverity | '';
}

export interface GatesFormState {
  reconcile: GatesReconcileRule[];
  cepOffHours: GatesOffHoursRule[];
  cepBurst: GatesBurstRule[];
  cepProbing: GatesProbeRule[];
  lineage: GatesLineageRule[];
  eventOps: string[];
}

// ops 闭集（gate.checkOps / filter.NewEventFilter 同一约束）
export const GATE_OPS = ['insert', 'update', 'delete'] as const;
export const GATE_SEVERITIES = ['warn', 'critical'] as const;

/** 宽容取串：非串值兜底空（存量手写 JSON 的类型漂移不炸表单）。 */
function asString(v: unknown): string {
  return typeof v === 'string' ? v : '';
}

function asNumber(v: unknown): number {
  return typeof v === 'number' && Number.isFinite(v) ? v : 0;
}

function asSeverity(v: unknown): GateSeverity | '' {
  return v === 'warn' || v === 'critical' ? v : '';
}

function asMode(v: unknown): 'single' | 'window' | '' {
  return v === 'single' || v === 'window' ? v : '';
}

function asStringArray(v: unknown): string[] {
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : [];
}

function asRecord(v: unknown): Record<string, unknown> {
  return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
}

function asArray(v: unknown): unknown[] {
  return Array.isArray(v) ? v : [];
}

/** capture.gates JSON → 表单状态（坏 JSON 抛错由调用方拦在模式切换外）。 */
export function parseGatesForm(content: string): GatesFormState {
  const root = asRecord(JSON.parse(content || '{}'));
  const reconcile = asRecord(root.reconcile);
  const cep = asRecord(root.cep);
  const lineage = asRecord(root.lineage);
  const event = asRecord(root.event);
  return {
    reconcile: asArray(reconcile.rules).map((raw): GatesReconcileRule => {
      const r = asRecord(raw);
      return {
        name: asString(r.name),
        itemTable: asString(r.itemTable),
        ledgerTable: asString(r.ledgerTable),
        itemKey: asString(r.itemKey),
        ledgerKey: asString(r.ledgerKey),
        windowSec: asNumber(r.windowSec),
        severity: asSeverity(r.severity),
        exemptKeys: asStringArray(r.exemptKeys),
        maxPending: asNumber(r.maxPending),
      };
    }),
    cepOffHours: asArray(cep.offHours).map((raw): GatesOffHoursRule => {
      const r = asRecord(raw);
      return {
        name: asString(r.name),
        table: asString(r.table),
        ops: asStringArray(r.ops),
        windows: asArray(r.windows).map((w): GatesOffHoursWindow => {
          const w0 = asRecord(w);
          return { startMin: asNumber(w0.startMin), endMin: asNumber(w0.endMin) };
        }),
        tzOffsetMin: asNumber(r.tzOffsetMin),
        severity: asSeverity(r.severity),
      };
    }),
    cepBurst: asArray(cep.burst).map((raw): GatesBurstRule => {
      const r = asRecord(raw);
      return {
        name: asString(r.name),
        table: asString(r.table),
        ops: asStringArray(r.ops),
        deltaField: asString(r.deltaField),
        threshold: asNumber(r.threshold),
        mode: asMode(r.mode),
        windowSec: asNumber(r.windowSec),
        maxEvents: asNumber(r.maxEvents),
        severity: asSeverity(r.severity),
      };
    }),
    cepProbing: asArray(cep.probing).map((raw): GatesProbeRule => {
      const r = asRecord(raw);
      return {
        name: asString(r.name),
        table: asString(r.table),
        ops: asStringArray(r.ops),
        keyField: asString(r.keyField),
        deltaField: asString(r.deltaField),
        smallBelow: asNumber(r.smallBelow),
        count: asNumber(r.count),
        windowSec: asNumber(r.windowSec),
        maxTracked: asNumber(r.maxTracked),
        severity: asSeverity(r.severity),
      };
    }),
    lineage: asArray(lineage.rules).map((raw): GatesLineageRule => {
      const r = asRecord(raw);
      return {
        name: asString(r.name),
        table: asString(r.table),
        ops: asStringArray(r.ops),
        allowAccounts: asStringArray(r.allowAccounts),
        severity: asSeverity(r.severity),
      };
    }),
    eventOps: asStringArray(event.ops),
  };
}

/** 表单状态 → capture.gates wire JSON（恒输出全四段；两空格缩进）。 */
export function serializeGatesForm(s: GatesFormState): string {
  return JSON.stringify(
    {
      reconcile: { rules: s.reconcile },
      cep: {
        offHours: s.cepOffHours,
        burst: s.cepBurst,
        probing: s.cepProbing,
      },
      lineage: { rules: s.lineage },
      event: { ops: s.eventOps },
    },
    null,
    2,
  );
}

/** 空状态（未发布/空内容时的表单初始态）。 */
export function emptyGatesForm(): GatesFormState {
  return {
    reconcile: [],
    cepOffHours: [],
    cepBurst: [],
    cepProbing: [],
    lineage: [],
    eventOps: [],
  };
}

/** 分钟数（0-1439）→ HH:mm（表格列展示）。 */
export function minutesToHHmm(min: number): string {
  const m = Math.max(0, Math.min(1439, Math.floor(min)));
  return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`;
}
