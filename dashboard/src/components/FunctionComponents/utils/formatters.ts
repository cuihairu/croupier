// Formatting utilities for function components

/**
 * 格式化文件大小
 */
export const formatFileSize = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

/**
 * 格式化持续时间
 */
export const formatDuration = (ms: number): string => {
  if (ms < 1000) {
    return `${ms}ms`;
  } else if (ms < 60000) {
    return `${(ms / 1000).toFixed(2)}s`;
  } else if (ms < 3600000) {
    const minutes = Math.floor(ms / 60000);
    const seconds = Math.floor((ms % 60000) / 1000);
    return `${minutes}m ${seconds}s`;
  } else {
    const hours = Math.floor(ms / 3600000);
    const minutes = Math.floor((ms % 3600000) / 60000);
    return `${hours}h ${minutes}m`;
  }
};

/**
 * 格式化时间戳
 */
export const formatTimestamp = (timestamp: string | Date): string => {
  const date = new Date(timestamp);
  return date.toLocaleString('zh-CN');
};

/**
 * 格式化相对时间
 */
export const formatRelativeTime = (timestamp: string | Date): string => {
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSeconds = Math.floor(diffMs / 1000);
  const diffMinutes = Math.floor(diffSeconds / 60);
  const diffHours = Math.floor(diffMinutes / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffSeconds < 60) {
    return '刚刚';
  } else if (diffMinutes < 60) {
    return `${diffMinutes}分钟前`;
  } else if (diffHours < 24) {
    return `${diffHours}小时前`;
  } else if (diffDays < 30) {
    return `${diffDays}天前`;
  } else {
    return formatTimestamp(timestamp);
  }
};

/**
 * 格式化百分比
 */
export const formatPercentage = (value: number, decimals: number = 1): string => {
  return `${value.toFixed(decimals)}%`;
};

/**
 * 格式化数字
 */
export const formatNumber = (num: number): string => {
  return new Intl.NumberFormat('zh-CN').format(num);
};

/**
 * 格式化函数状态
 */
export const formatFunctionStatus = (status: string): string => {
  const statusMap: Record<string, string> = {
    active: '启用',
    inactive: '禁用',
    enabled: '启用',
    disabled: '禁用',
    running: '运行中',
    stopped: '已停止',
    error: '错误',
    pending: '等待中'
  };
  return statusMap[status] || status;
};

/**
 * 格式化执行状态
 */
export const formatExecutionStatus = (status: string): string => {
  const statusMap: Record<string, string> = {
    success: '成功',
    failed: '失败',
    running: '运行中',
    cancelled: '已取消',
    timeout: '超时',
    pending: '等待中'
  };
  return statusMap[status] || status;
};

/**
 * 格式化服务状态
 */
export const formatServiceStatus = (status: string): string => {
  const statusMap: Record<string, string> = {
    healthy: '健康',
    unhealthy: '异常',
    unknown: '未知',
    online: '在线',
    offline: '离线'
  };
  return statusMap[status] || status;
};

/**
 * 格式化版本号
 */
export const formatVersion = (version: string): string => {
  if (!version) return '未指定';

  // 移除 'v' 前缀
  const cleanVersion = version.replace(/^v/i, '');

  // 确保版本号格式一致
  if (/^\d+\.\d+\.\d+$/.test(cleanVersion)) {
    return `v${cleanVersion}`;
  }

  return version;
};

/**
 * 格式化标签列表
 */
export const formatTags = (tags: string[], maxDisplay: number = 3): string[] => {
  if (!tags || tags.length === 0) return [];

  const displayTags = tags.slice(0, maxDisplay);
  const remaining = tags.length - maxDisplay;

  if (remaining > 0) {
    return [...displayTags, `+${remaining}`];
  }

  return displayTags;
};

/**
 * 格式化错误信息
 */
export const formatError = (error: any): string => {
  if (typeof error === 'string') {
    return error;
  }

  if (error?.message) {
    return error.message;
  }

  if (error?.error) {
    return formatError(error.error);
  }

  return '未知错误';
};

/**
 * 格式化JSON
 */
export const formatJSON = (obj: any, indent: number = 2): string => {
  try {
    return JSON.stringify(obj, null, indent);
  } catch {
    return String(obj);
  }
};

/**
 * 截断文本
 */
export const truncateText = (text: string, maxLength: number): string => {
  if (!text || text.length <= maxLength) {
    return text;
  }
  return text.substring(0, maxLength - 3) + '...';
};

/**
 * 格式化内存使用量
 */
export const formatMemory = (bytes: number): string => {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let size = bytes;
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }

  return `${size.toFixed(2)} ${units[unitIndex]}`;
};

/**
 * 格式化网络地址
 */
export const formatAddress = (addr: string): string => {
  if (!addr) return '-';

  // 隐藏部分IP地址（仅显示前3段）
  const ipPattern = /^(\d+\.\d+\.\d+\.)\d+$/;
  if (ipPattern.test(addr)) {
    return addr.replace(ipPattern, '$1***');
  }

  return addr;
};