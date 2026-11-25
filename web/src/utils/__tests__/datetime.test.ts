/**
 * SUI-022 纯函数验收：日期/时间序列化契约与时区种子归一。
 *
 * 测试以进程时区自洽断言（jest 进程级 TZ pin：TZ=UTC 与
 * TZ=America/New_York 各跑一轮，见交付说明）；serializeDateValue 用本地
 * 日历日、serializeDateTimeValue 用 dayjs `Z` token 的显式偏移，
 * serializeTimeValue 保留真实分秒。仅用 dayjs 核心 API（utc 插件未加载）。
 */
import dayjs from 'dayjs';
import {
  hasTimezoneDesignator,
  normalizeDateTimeSeed,
  parseDateTimeSeed,
  parseTimeSeed,
  serializeDateValue,
  serializeDateTimeValue,
  serializeTimeValue,
} from '../datetime';

const LOCAL_TZ_OFFSET = dayjs('2026-01-31T08:00:00').format('Z');

describe('SUI-022: 日期/时间值序列化', () => {
  it('serializeDateValue：本地日历日期 YYYY-MM-DD（无时间、无偏移）', () => {
    expect(serializeDateValue(dayjs('2026-01-31T08:00:00'))).toBe('2026-01-31');
    expect(serializeDateValue(dayjs('2026-01-01T00:30:00'))).toBe('2026-01-01');
  });

  it('serializeDateTimeValue：带显式偏移的 RFC3339 YYYY-MM-DDTHH:mm:ssZ', () => {
    const value = dayjs('2026-01-31T08:00:00');
    expect(serializeDateTimeValue(value)).toBe(`2026-01-31T08:00:00${LOCAL_TZ_OFFSET}`);
    // 与内置 toISOString 的差异：本地墙钟被保留，不再平移成 UTC Z
    if (LOCAL_TZ_OFFSET !== '+00:00') {
      expect(serializeDateTimeValue(value)).not.toBe(value.toISOString());
    }
  });

  it('serializeTimeValue：HH:mm:ss（分秒真实，不拼 :00 覆盖分钟）', () => {
    expect(serializeTimeValue(dayjs('2026-01-31T08:37:19'))).toBe('08:37:19');
    expect(serializeTimeValue(dayjs('2026-01-31T00:00:00'))).toBe('00:00:00');
  });

  it('清空/非法值 → undefined（键移除语义，不产生 null）', () => {
    expect(serializeDateValue(null)).toBeUndefined();
    expect(serializeDateValue(undefined)).toBeUndefined();
    expect(serializeDateValue(dayjs('invalid'))).toBeUndefined();
    expect(serializeDateTimeValue(null)).toBeUndefined();
    expect(serializeDateTimeValue(undefined)).toBeUndefined();
    expect(serializeDateTimeValue(dayjs('invalid'))).toBeUndefined();
    expect(serializeTimeValue(null)).toBeUndefined();
    expect(serializeTimeValue(undefined)).toBeUndefined();
    expect(serializeTimeValue(dayjs('invalid'))).toBeUndefined();
  });
});

describe('SUI-022: 显示种子归一', () => {
  it('hasTimezoneDesignator：Z / ±HH:MM / ±HHMM 识别，naive 串不识别', () => {
    expect(hasTimezoneDesignator('2026-01-01T00:00:00Z')).toBe(true);
    expect(hasTimezoneDesignator('2026-01-01T00:00:00z')).toBe(true);
    expect(hasTimezoneDesignator('2026-01-01T00:00:00+08:00')).toBe(true);
    expect(hasTimezoneDesignator('2026-01-01T00:00:00-0500')).toBe(true);
    expect(hasTimezoneDesignator('2026-01-01T00:00:00')).toBe(false);
    expect(hasTimezoneDesignator('2026-01-01')).toBe(false);
  });

  it('normalizeDateTimeSeed：naive 串保持本地墙钟原样；带偏移串保持瞬时原样', () => {
    expect(normalizeDateTimeSeed('2026-01-01T08:00:00')).toBe('2026-01-01T08:00:00');
    expect(normalizeDateTimeSeed('2026-01-01T08:00:00+08:00')).toBe(
      '2026-01-01T08:00:00+08:00',
    );
    expect(normalizeDateTimeSeed('2026-01-01T00:00:00Z')).toBe('2026-01-01T00:00:00Z');
    expect(normalizeDateTimeSeed('')).toBe('');
  });

  it('parseDateTimeSeed：naive 按本地墙钟解析，带偏移按瞬时解析；非法得 undefined', () => {
    const naive = parseDateTimeSeed('2026-01-31T08:00:00');
    expect(naive?.format('YYYY-MM-DDTHH:mm:ss')).toBe('2026-01-31T08:00:00');
    expect(naive?.format('Z')).toBe(LOCAL_TZ_OFFSET);

    const offset = parseDateTimeSeed('2026-01-31T08:00:00+08:00');
    // 瞬时等价：+08:00 的 08:00 即 UTC 的 00:00（同一 valueOf，无需 utc 插件）
    expect(offset?.valueOf()).toBe(dayjs('2026-01-31T00:00:00Z').valueOf());
    // 「保持瞬时」语义：显示折算到本地时区（naive 串才是墙钟原样）
    expect(offset?.format('YYYY-MM-DDTHH:mm:ssZ')).toBe(
      dayjs('2026-01-31T00:00:00Z').format('YYYY-MM-DDTHH:mm:ssZ'),
    );

    expect(parseDateTimeSeed(undefined)).toBeUndefined();
    expect(parseDateTimeSeed('')).toBeUndefined();
    expect(parseDateTimeSeed('not-a-date')).toBeUndefined();
  });

  it('parseTimeSeed：HH:mm / HH:mm:ss → 当日同刻；越界/非法 → undefined', () => {
    const parsed = parseTimeSeed('08:37');
    expect(parsed?.format('HH:mm:ss')).toBe('08:37:00');
    expect(parseTimeSeed('08:37:19')?.format('HH:mm:ss')).toBe('08:37:19');
    expect(parseTimeSeed('25:00')).toBeUndefined();
    expect(parseTimeSeed('08:60')).toBeUndefined();
    expect(parseTimeSeed('08:00:99')).toBeUndefined();
    expect(parseTimeSeed('nope')).toBeUndefined();
    expect(parseTimeSeed(undefined)).toBeUndefined();
  });
});
