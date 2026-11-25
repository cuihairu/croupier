/**
 * SUI-022：日期/时间表单值序列化与显示种子归一。
 *
 * 背景：@rjsf/antd 内置 DateWidget 的 onChange 用 toISOString() 序列化
 * date-time——本地时区被强制折算成 UTC `Z` 结尾，picker 所在的显式偏移
 * （+08:00）被丢弃；清空时透传 null，null 会滞留在 rjsf formData（只有
 * undefined 才会移除键），随后撞上 AJV `type: string`（schema 不允许 null
 * 时必炸）。@rjsf/core 的 TimeWidget（antd 主题未提供 time widget）用
 * `${value}:00` 字符串拼接，对 antd TimePicker 的 Dayjs 值会产出非法串。
 *
 * 本模块的序列化约定：
 * - date      → 本地日历日期 `YYYY-MM-DD`（无时间、无偏移）
 * - date-time → 带显式本地偏移的 RFC3339 `YYYY-MM-DDTHH:mm:ssZ`（dayjs 的
 *               `Z` token 输出 ±HH:MM，如 `2026-01-31T08:00:00+08:00`）。
 *               与 toISOString 的差异：保留 picker 的本地偏移，不再平移到
 *               UTC；串内瞬时值等价（偏移可无损换算），按 UTC 设计的消费方
 *               服务端解析后仍是同一时刻，精度到秒。
 * - time      → `HH:mm:ss`
 * 清空（null / undefined / 非法值）一律产出 undefined，键从提交值移除。
 */

import dayjs, { type Dayjs } from 'dayjs';

const DATE_FORMAT = 'YYYY-MM-DD';
const DATETIME_FORMAT = 'YYYY-MM-DDTHH:mm:ssZ';
const TIME_FORMAT = 'HH:mm:ss';

/** Dayjs → 本地日历日期 `YYYY-MM-DD`；空值/非法值 → undefined。 */
export function serializeDateValue(value: Dayjs | null | undefined): string | undefined {
  return value?.isValid() ? value.format(DATE_FORMAT) : undefined;
}

/** Dayjs → 带显式本地偏移的 RFC3339 `YYYY-MM-DDTHH:mm:ssZ`；空值/非法值 → undefined。 */
export function serializeDateTimeValue(value: Dayjs | null | undefined): string | undefined {
  return value?.isValid() ? value.format(DATETIME_FORMAT) : undefined;
}

/** Dayjs → `HH:mm:ss`；空值/非法值 → undefined。 */
export function serializeTimeValue(value: Dayjs | null | undefined): string | undefined {
  return value?.isValid() ? value.format(TIME_FORMAT) : undefined;
}

/** 时区指示后缀：`Z`/`z` 结尾，或 `±HH:MM` / `±HHMM` 偏移结尾。 */
const TZ_DESIGNATOR = /(?:[Zz]|[+-]\d{2}:?\d{2})$/;

/** 种子串是否携带时区信息（`Z` 或显式偏移结尾）。 */
export function hasTimezoneDesignator(value: string): boolean {
  return TZ_DESIGNATOR.test(value.trim());
}

/**
 * 显示种子归一：date/date-time 字段的回显种子（initialValues / defaultValue /
 * 上次提交值）进入 picker 前经过这里。
 *
 * - 无时区信息（DB 风格 naive 串，如 `2026-01-01T08:00:00`）：视为本地墙钟
 *   时间，保持原样交给 dayjs 本地解析——墙钟不被静默平移。
 * - 携带 `Z` / 偏移：保持瞬时值（dayjs 解析为瞬时，显示折算到浏览器本地时区，
 *   非 UTC 时区下日历日可能跨日——这是「保持瞬时」的显式边界，不是缺陷）。
 *
 * 两条路径现行处理一致（dayjs 默认解析语义恰好满足上述约定），显式分派以
 * 固化意图：本函数是唯一的可挂钩点，未来若需把 naive 种子改按 UTC 解释、
 * 或把带偏移种子折算到业务时区，只改这里，widget 不动。
 */
export function normalizeDateTimeSeed(seed: string): string {
  const text = seed.trim();
  if (!text) return text;
  return hasTimezoneDesignator(text) ? text : text;
}

/** 显示种子 → Dayjs：非字符串/空串/非法值 → undefined（picker 显示为空）。 */
export function parseDateTimeSeed(value: unknown): Dayjs | undefined {
  if (typeof value !== 'string') return undefined;
  const text = normalizeDateTimeSeed(value);
  if (!text) return undefined;
  const next = dayjs(text);
  return next.isValid() ? next : undefined;
}

/** `HH:mm` / `HH:mm:ss` 种子 → 当日同刻 Dayjs（TimePicker 只消费时刻）；非法/越界 → undefined。 */
export function parseTimeSeed(value: unknown): Dayjs | undefined {
  if (typeof value !== 'string') return undefined;
  const match = /^(\d{1,2}):(\d{2})(?::(\d{2}))?$/.exec(value.trim());
  if (!match) return undefined;
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  const seconds = Number(match[3] ?? '0');
  // Date.setHours 会静默进位（25:99 → 次日 1:59），越界直接判非法
  if (hours > 23 || minutes > 59 || seconds > 59) return undefined;
  const next = dayjs().hour(hours).minute(minutes).second(seconds).millisecond(0);
  return next.isValid() ? next : undefined;
}
