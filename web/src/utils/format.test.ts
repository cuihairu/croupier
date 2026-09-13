import { getIntl } from '@umijs/max';
import { formatBytes, formatDateTime, formatDuration, formatNumber, formatPercent } from './format';

// 工厂内自建 formatMessage mock（避免 TDZ）：getIntl 每次返回同一个 mock 实例，
// 返回 defaultMessage 模拟无 locale 词条时的 intl fallback 行为。
jest.mock('@umijs/max', () => {
  const formatMessage = jest.fn(({ defaultMessage }: { defaultMessage: string }) => defaultMessage);
  return {
    getIntl: jest.fn(() => ({ formatMessage })),
  };
});

const mockedGetIntl = getIntl as unknown as jest.MockedFunction<() => { formatMessage: jest.Mock }>;

describe('utils/format formatBytes', () => {
  it('handles zero and negative inputs', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(-1)).toBe('-');
    expect(formatBytes(-1024)).toBe('-');
  });

  it('formats sub-10 values with two decimals', () => {
    expect(formatBytes(1)).toBe('1.00 B');
    expect(formatBytes(9.5)).toBe('9.50 B');
    expect(formatBytes(1024)).toBe('1.00 KB');
    expect(formatBytes(1536)).toBe('1.50 KB');
  });

  it('formats 10-100 values with one decimal', () => {
    expect(formatBytes(15360)).toBe('15.0 KB');
    expect(formatBytes(51712)).toBe('50.5 KB'); // 50.5 * 1024
  });

  it('formats >=100 values with integer rounding', () => {
    expect(formatBytes(500)).toBe('500 B');
    expect(formatBytes(102400)).toBe('100 KB'); // 恰好 100，走 Math.round 路
    expect(formatBytes(153600)).toBe('150 KB'); // 150 * 1024
  });

  it('scales up to petabytes', () => {
    expect(formatBytes(1073741824)).toBe('1.00 GB');
    expect(formatBytes(1099511627776)).toBe('1.00 TB');
    expect(formatBytes(1125899906842624)).toBe('1.00 PB');
  });
});

describe('utils/format formatPercent', () => {
  it('clamps at both ends', () => {
    expect(formatPercent(100)).toBe('100%');
    expect(formatPercent(150)).toBe('100%');
    expect(formatPercent(0)).toBe('0%');
    expect(formatPercent(-1)).toBe('0%');
  });

  it('formats in-between values with one decimal', () => {
    expect(formatPercent(75.5)).toBe('75.5%');
    expect(formatPercent(99.5)).toBe('99.5%');
    expect(formatPercent(0.1)).toBe('0.1%');
  });
});

describe('utils/format formatDuration', () => {
  let formatMessageMock: jest.Mock;

  beforeEach(() => {
    formatMessageMock = mockedGetIntl().formatMessage;
    formatMessageMock.mockClear();
    mockedGetIntl.mockClear();
  });

  it('renders seconds below one minute', () => {
    expect(formatDuration(30)).toBe('30秒');
    expect(formatDuration(0)).toBe('0秒');
    expect(formatDuration(59)).toBe('59秒');
    expect(formatMessageMock).toHaveBeenCalledWith(
      { id: 'utils.format.duration.second', defaultMessage: '30秒' },
      { value: 30 },
    );
  });

  it('renders minutes below one hour', () => {
    expect(formatDuration(60)).toBe('1分钟');
    expect(formatDuration(90)).toBe('1分钟');
    expect(formatDuration(3599)).toBe('59分钟');
    expect(formatMessageMock).toHaveBeenCalledWith(
      { id: 'utils.format.duration.minute', defaultMessage: '59分钟' },
      { value: 59 },
    );
  });

  it('renders hours below one day', () => {
    expect(formatDuration(3600)).toBe('1小时');
    expect(formatDuration(86399)).toBe('23小时');
    expect(formatMessageMock).toHaveBeenCalledWith(
      { id: 'utils.format.duration.hour', defaultMessage: '23小时' },
      { value: 23 },
    );
  });

  it('renders days for a day and beyond', () => {
    expect(formatDuration(86400)).toBe('1天');
    expect(formatDuration(172800)).toBe('2天');
    expect(formatMessageMock).toHaveBeenCalledWith(
      { id: 'utils.format.duration.day', defaultMessage: '2天' },
      { value: 2 },
    );
  });
});

describe('utils/format formatNumber', () => {
  it('groups thousands with zh-CN locale', () => {
    expect(formatNumber(1234567)).toBe('1,234,567');
    expect(formatNumber(0)).toBe('0');
    expect(formatNumber(123456.78)).toBe('123,456.78');
  });
});

describe('utils/format formatDateTime', () => {
  const dateTimeOptions = {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  } as const;

  it('returns a dash for empty input', () => {
    expect(formatDateTime('')).toBe('-');
  });

  it('formats an ISO timestamp with the zh-CN calendar fields', () => {
    const dateStr = '2024-01-01T12:00:00Z';
    const expected = new Date(dateStr).toLocaleString('zh-CN', dateTimeOptions);
    expect(formatDateTime(dateStr)).toBe(expected);
    expect(formatDateTime(dateStr)).toMatch(/2024/);
  });

  it('returns the Invalid Date marker instead of throwing for unparseable input', () => {
    // new Date(<非法串>) 不抛异常，而是得到 Invalid Date；toLocaleString 输出 "Invalid Date"
    const expected = new Date('not-a-date').toLocaleString('zh-CN', dateTimeOptions);
    expect(formatDateTime('not-a-date')).toBe(expected);
  });
});
