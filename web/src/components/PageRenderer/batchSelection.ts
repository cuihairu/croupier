/**
 * batchSelection - 批量目标选择的可核对逻辑（D2）
 *
 * 渲染器只负责交互，选择核对的判断集中在这里以便直接验证：
 * - 行标识与表格 rowKey 同口径，确认框展示的目标与提交的目标逐条对应
 * - 筛选指纹剔除分页入参，翻页不算筛选变更
 * - 目标摘要在确认框内展示标识值，超出部分只报数
 */
import type { FormValues } from '@/types/dashboard';

/** 分页/游标类入参：翻页不算筛选变更（跨页保留选择本身是要求的能力）。 */
const PAGINATION_PARAM_KEYS = new Set(['current', 'page', 'pageSize', 'size', 'offset', 'total']);

/** 行标识：identity 字段 > id > key，缺失兜底数据本身（多条空串 key 在
 * React diff 下会产生幻影残留行，数据串保证行内唯一）。 */
export function rowIdentity(record: FormValues, identityKey?: string): string {
  const identity = record[identityKey ?? 'id'] ?? record.id ?? record.key;
  return identity === undefined || identity === null ? JSON.stringify(record) : String(identity);
}

/** 筛选指纹：剔除分页入参与空值后按 key 排序，保证同条件不同页序得到同一指纹。 */
export function selectionFilterFingerprint(params: Record<string, unknown>): string {
  return Object.entries(params)
    .filter(
      ([key, value]) =>
        !PAGINATION_PARAM_KEYS.has(key) && value !== undefined && value !== null && value !== '',
    )
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => `${key}=${String(value)}`)
    .join('&');
}

/** 筛选变更后仍保留旧选择。保留的跨页键无法区分「仍在其它页」与「已被新筛选
 * 排除」，一律提示算子核对或清除。previousFingerprint 为 null 表示尚未完成过
 * 查询（首查不计）；空串是合法指纹——「无筛选条件」本身也是一种条件，从无条
 * 件切到有条件同样要核对。 */
export function isStaleSelection(
  previousFingerprint: string | null,
  nextFingerprint: string,
  selectedCount: number,
): boolean {
  return (
    selectedCount > 0 && previousFingerprint !== null && previousFingerprint !== nextFingerprint
  );
}

export interface TargetSummary {
  identities: string[];
  omitted: number;
}

/** 目标摘要：确认框内展示前 limit 个标识值，其余只报数（完整清单见选择栏）。 */
export function summarizeTargets(
  rows: FormValues[],
  identityKey?: string,
  limit = 5,
): TargetSummary {
  const identities = rows.map((row) => rowIdentity(row, identityKey));
  return {
    identities: identities.slice(0, limit),
    omitted: Math.max(0, identities.length - limit),
  };
}

/** 批量操作的每人数量 × 目标数 = 总量（D2 跨目标总量）。
 * 每人数量必须是有限非负整数且目标数为正整数，否则返回 undefined——
 * 不确定的量不能硬猜成 0，也不能静默给出小数总量。 */
export function summarizeQuantityTotal(
  perQuantity: number | string | undefined | null,
  targetCount: number,
): number | undefined {
  const per = typeof perQuantity === 'string' ? Number(perQuantity) : perQuantity;
  // 不确定输入不猜 0：空串/全空白串 Number('')=0 会伪装成合法 0，必须拒绝
  //（REVIEW-002：空/空白金额不得确认并提交，与「不确定输入不猜 0、不执行」注释一致）。
  if (typeof perQuantity === 'string' && perQuantity.trim() === '') {
    return undefined;
  }
  if (typeof per !== 'number' || !Number.isFinite(per) || per < 0 || !Number.isInteger(per)) {
    return undefined;
  }
  if (!Number.isInteger(targetCount) || targetCount <= 0) return undefined;
  return per * targetCount;
}
