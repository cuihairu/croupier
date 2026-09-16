/** utils/format 全量直测：此前仅经 LoginLogs/OperationLogs 间接覆盖。 */
import {
  formatBytes,
  formatDateTime,
  formatDuration,
  formatNumber,
  formatPercent,
} from '../format';

describe('formatBytes', () => {
  it('按数量级选择单位与小数位', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(-5)).toBe('-');
    expect(formatBytes(5)).toBe('5.00 B'); // value < 10 → 两位小数
    expect(formatBytes(99)).toBe('99.0 B'); // 10 ≤ value < 100 → 一位小数
    expect(formatBytes(512)).toBe('512 B'); // value ≥ 100 → 取整
    expect(formatBytes(4096)).toBe('4.00 KB');
    expect(formatBytes(153600)).toBe('150 KB');
    expect(formatBytes(1048576)).toBe('1.00 MB');
    expect(formatBytes(1073741824)).toBe('1.00 GB');
  });
});

describe('formatPercent', () => {
  it('上下界封顶；区间值保留一位小数', () => {
    expect(formatPercent(120)).toBe('100%');
    expect(formatPercent(-3)).toBe('0%');
    expect(formatPercent(75.5)).toBe('75.5%');
  });
});

describe('formatDuration', () => {
  it('秒/分/时/天分级取整', () => {
    expect(formatDuration(30)).toBe('30秒');
    expect(formatDuration(90)).toBe('1分钟');
    expect(formatDuration(7200)).toBe('2小时');
    expect(formatDuration(90000)).toBe('1天');
  });
});

describe('formatNumber / formatDateTime', () => {
  it('千分位分隔', () => {
    expect(formatNumber(1234567)).toBe('1,234,567');
    expect(formatNumber(999)).toBe('999');
  });

  it('日期时间本地化；空串回退 -', () => {
    expect(formatDateTime('')).toBe('-');
    const formatted = formatDateTime('2024-01-01T12:00:00Z');
    expect(formatted).toMatch(/2024/);
    expect(formatted).not.toBe('2024-01-01T12:00:00Z'); // 已本地化重排
  });
});
