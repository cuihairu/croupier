/**
 * 格式化工具函数
 *
 * 提供对人友好的数据格式化
 */

/**
 * 格式化字节数为人类可读的字符串
 * @example formatBytes(1024) => "1 KB"
 * @example formatBytes(1048576) => "1 MB"
 * @example formatBytes(1073741824) => "1 GB"
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  if (bytes < 0) return '-';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const value = bytes / Math.pow(k, i);
  // 根据大小选择合适的小数位数
  if (value >= 100) return Math.round(value) + ' ' + sizes[i];
  if (value >= 10) return value.toFixed(1) + ' ' + sizes[i];
  return value.toFixed(2) + ' ' + sizes[i];
}

/**
 * 格式化百分比
 * @example formatPercent(75.5) => "75.5%"
 * @example formatPercent(100) => "100%"
 */
export function formatPercent(value: number): string {
  if (value >= 100) return '100%';
  if (value <= 0) return '0%';
  return value.toFixed(1) + '%';
}

/**
 * 格式化持续时间
 * @example formatDuration(30) => "30秒"
 * @example formatDuration(90) => "1分钟"
 * @example formatDuration(3600) => "1小时"
 */
export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}秒`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}分钟`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}小时`;
  return `${Math.floor(seconds / 86400)}天`;
}

/**
 * 格式化数字（添加千分位分隔符）
 * @example formatNumber(1234567) => "1,234,567"
 */
export function formatNumber(value: number): string {
  return value.toLocaleString('zh-CN');
}

/**
 * 格式化日期时间
 * @example formatDateTime("2024-01-01T12:00:00Z") => "2024-01-01 12:00:00"
 */
export function formatDateTime(dateStr: string): string {
  if (!dateStr) return '-';
  try {
    const date = new Date(dateStr);
    return date.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  } catch {
    return dateStr;
  }
}
