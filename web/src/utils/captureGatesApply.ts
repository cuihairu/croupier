import type { OpsNode } from '@/services/api/ops';

// 生效对账汇总（appliedRevision 上报链）：capture agent 注册 labels 里
// 自报的生效版本（captureGatesRevision）与 desired 版本比对。
// Source: agents/capture/runner buildPayload 写 labels captureGatesRevision
// / role=capture。滞后正常（30s 轮询 + 重注册），不作强一致读。
export type CaptureGatesApplySummary = {
  /** 面板 desired 版本（config_versions 最新版） */
  desired: number;
  /** scope 内在线 capture-agent 数 */
  total: number;
  /** 已自报 desired 版本 */
  applied: OpsNode[];
  /** 未自报或自报旧版本（含从未上报） */
  stale: OpsNode[];
};

export function summarizeGatesApply(
  nodes: OpsNode[],
  desired: number,
): CaptureGatesApplySummary | null {
  if (desired <= 0) return null;
  const captureNodes = nodes.filter((n) => n.labels?.role === 'capture');
  const applied = captureNodes.filter((n) => n.labels?.captureGatesRevision === String(desired));
  const stale = captureNodes.filter((n) => n.labels?.captureGatesRevision !== String(desired));
  return { desired, total: captureNodes.length, applied, stale };
}
