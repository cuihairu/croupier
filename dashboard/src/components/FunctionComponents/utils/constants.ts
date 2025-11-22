// Constants for function components

/**
 * 函数执行状态枚举
 */
export const FUNCTION_EXECUTION_STATUS = {
  SUCCESS: 'success',
  FAILED: 'failed',
  RUNNING: 'running',
  CANCELLED: 'cancelled',
  TIMEOUT: 'timeout',
  PENDING: 'pending'
} as const;

/**
 * 函数状态枚举
 */
export const FUNCTION_STATUS = {
  ACTIVE: 'active',
  INACTIVE: 'inactive',
  ENABLED: 'enabled',
  DISABLED: 'disabled'
} as const;

/**
 * 服务健康状态枚举
 */
export const SERVICE_HEALTH_STATUS = {
  HEALTHY: 'healthy',
  UNHEALTHY: 'unhealthy',
  UNKNOWN: 'unknown',
  ONLINE: 'online',
  OFFLINE: 'offline'
} as const;

/**
 * 数据类型枚举
 */
export const DATA_TYPES = {
  STRING: 'string',
  NUMBER: 'number',
  INTEGER: 'integer',
  BOOLEAN: 'boolean',
  ARRAY: 'array',
  OBJECT: 'object',
  NULL: 'null'
} as const;

/**
 * HTTP方法枚举
 */
export const HTTP_METHODS = {
  GET: 'GET',
  POST: 'POST',
  PUT: 'PUT',
  DELETE: 'DELETE',
  PATCH: 'PATCH'
} as const;

/**
 * 默认配置
 */
export const DEFAULT_CONFIG = {
  // 分页配置
  PAGINATION: {
    DEFAULT_PAGE_SIZE: 10,
    PAGE_SIZE_OPTIONS: [10, 20, 50, 100],
    SHOW_SIZE_CHANGER: true,
    SHOW_QUICK_JUMPER: true,
    SHOW_TOTAL: true
  },

  // 表格配置
  TABLE: {
    DEFAULT_SCROLL_X: 1000,
    DEFAULT_SCROLL_Y: 400,
    ROW_HEIGHT: 54
  },

  // 表单配置
  FORM: {
    VALIDATE_TRIGGER: 'onChange' as const,
    LABEL_COL: { span: 6 },
    WRAPPER_COL: { span: 18 },
    AUTO_COMPLETE: 'off'
  },

  // 刷新配置
  REFRESH: {
    AUTO_REFRESH_INTERVAL: 30000, // 30秒
    MANUAL_REFRESH_DELAY: 1000, // 1秒
    RETRY_ATTEMPTS: 3,
    RETRY_DELAY: 2000 // 2秒
  },

  // 超时配置
  TIMEOUT: {
    API_REQUEST: 30000, // 30秒
    FUNCTION_EXECUTION: 300000, // 5分钟
    WEBSOCKET_CONNECTION: 60000 // 1分钟
  },

  // 缓存配置
  CACHE: {
    FUNCTION_CACHE_TTL: 300000, // 5分钟
    REGISTRY_CACHE_TTL: 60000, // 1分钟
    HISTORY_CACHE_TTL: 120000 // 2分钟
  }
} as const;

/**
 * 颜色主题
 */
export const COLOR_THEME = {
  STATUS: {
    SUCCESS: '#52c41a',
    WARNING: '#faad14',
    ERROR: '#ff4d4f',
    INFO: '#1890ff',
    PROCESSING: '#1890ff',
    DEFAULT: '#d9d9d9'
  },

  COVERAGE: {
    EXCELLENT: '#52c41a', // 90-100%
    GOOD: '#73d13d',     // 80-89%
    FAIR: '#faad14',     // 70-79%
    POOR: '#ff4d4f'      // <70%
  },

  PRIORITY: {
    HIGH: '#ff4d4f',
    MEDIUM: '#faad14',
    LOW: '#52c41a'
  }
} as const;

/**
 * 状态映射
 */
export const STATUS_MAPPING = {
  EXECUTION: {
    [FUNCTION_EXECUTION_STATUS.SUCCESS]: { text: '成功', color: COLOR_THEME.STATUS.SUCCESS, icon: '✓' },
    [FUNCTION_EXECUTION_STATUS.FAILED]: { text: '失败', color: COLOR_THEME.STATUS.ERROR, icon: '✗' },
    [FUNCTION_EXECUTION_STATUS.RUNNING]: { text: '运行中', color: COLOR_THEME.STATUS.PROCESSING, icon: '⟳' },
    [FUNCTION_EXECUTION_STATUS.CANCELLED]: { text: '已取消', color: COLOR_THEME.STATUS.WARNING, icon: '⊘' },
    [FUNCTION_EXECUTION_STATUS.TIMEOUT]: { text: '超时', color: COLOR_THEME.STATUS.WARNING, icon: '⏱' },
    [FUNCTION_EXECUTION_STATUS.PENDING]: { text: '等待中', color: COLOR_THEME.STATUS.INFO, icon: '⏸' }
  },

  FUNCTION: {
    [FUNCTION_STATUS.ACTIVE]: { text: '启用', color: COLOR_THEME.STATUS.SUCCESS, icon: '●' },
    [FUNCTION_STATUS.INACTIVE]: { text: '禁用', color: COLOR_THEME.STATUS.DEFAULT, icon: '○' },
    [FUNCTION_STATUS.ENABLED]: { text: '启用', color: COLOR_THEME.STATUS.SUCCESS, icon: '●' },
    [FUNCTION_STATUS.DISABLED]: { text: '禁用', color: COLOR_THEME.STATUS.DEFAULT, icon: '○' }
  },

  SERVICE: {
    [SERVICE_HEALTH_STATUS.HEALTHY]: { text: '健康', color: COLOR_THEME.STATUS.SUCCESS, icon: '●' },
    [SERVICE_HEALTH_STATUS.UNHEALTHY]: { text: '异常', color: COLOR_THEME.STATUS.ERROR, icon: '●' },
    [SERVICE_HEALTH_STATUS.UNKNOWN]: { text: '未知', color: COLOR_THEME.STATUS.DEFAULT, icon: '○' },
    [SERVICE_HEALTH_STATUS.ONLINE]: { text: '在线', color: COLOR_THEME.STATUS.SUCCESS, icon: '●' },
    [SERVICE_HEALTH_STATUS.OFFLINE]: { text: '离线', color: COLOR_THEME.STATUS.ERROR, icon: '●' }
  }
} as const;

/**
 * 错误消息
 */
export const ERROR_MESSAGES = {
  NETWORK_ERROR: '网络连接错误，请检查网络设置',
  TIMEOUT_ERROR: '请求超时，请稍后重试',
  UNAUTHORIZED: '未授权访问，请检查权限设置',
  NOT_FOUND: '资源未找到',
  SERVER_ERROR: '服务器内部错误',
  VALIDATION_ERROR: '数据验证失败',
  UNKNOWN_ERROR: '未知错误',
  FUNCTION_NOT_FOUND: '函数不存在',
  INVALID_PARAMS: '函数参数无效',
  EXECUTION_FAILED: '函数执行失败',
  SERVICE_UNAVAILABLE: '服务不可用'
} as const;

/**
 * 成功消息
 */
export const SUCCESS_MESSAGES = {
  FUNCTION_EXECUTED: '函数执行成功',
  FUNCTION_SAVED: '函数保存成功',
  FUNCTION_DELETED: '函数删除成功',
  FUNCTION_UPDATED: '函数更新成功',
  SERVICE_STARTED: '服务启动成功',
  SERVICE_STOPPED: '服务停止成功',
  CONFIG_SAVED: '配置保存成功',
  CACHE_CLEARED: '缓存清除成功'
} as const;

/**
 * 确认消息
 */
export const CONFIRM_MESSAGES = {
  DELETE_FUNCTION: '确定要删除此函数吗？此操作不可恢复！',
  DISABLE_FUNCTION: '确定要禁用此函数吗？',
  ENABLE_FUNCTION: '确定要启用此函数吗？',
  STOP_SERVICE: '确定要停止此服务吗？',
  RESTART_SERVICE: '确定要重启此服务吗？',
  CLEAR_CACHE: '确定要清除缓存吗？',
  RESET_CONFIG: '确定要重置配置吗？'
} as const;

/**
 * 正则表达式模式
 */
export const REGEX_PATTERNS = {
  FUNCTION_ID: /^[a-zA-Z0-9._-]+$/,
  GAME_ID: /^[a-zA-Z0-9_-]+$/,
  ENV_NAME: /^[a-z0-9_-]+$/,
  SEMVER: /^v?\d+\.\d+\.\d+.*$/,
  EMAIL: /^[^\s@]+@[^\s@]+\.[^\s@]+$/,
  IP_ADDRESS: /^(\d{1,3}\.){3}\d{1,3}$/,
  URL: /^https?:\/\/.+/
} as const;

/**
 * 键盘快捷键
 */
export const KEYBOARD_SHORTCUTS = {
  REFRESH: 'F5',
  SEARCH: 'Ctrl+F',
  NEW_ITEM: 'Ctrl+N',
  SAVE: 'Ctrl+S',
  DELETE: 'Delete',
  COPY: 'Ctrl+C',
  PASTE: 'Ctrl+V',
  UNDO: 'Ctrl+Z',
  REDO: 'Ctrl+Y'
} as const;

/**
 * 本地存储键名
 */
export const STORAGE_KEYS = {
  USER_PREFERENCES: 'croupier_user_preferences',
  FUNCTION_SEARCH_HISTORY: 'croupier_function_search_history',
  RECENT_FUNCTIONS: 'croupier_recent_functions',
  TABLE_SETTINGS: 'croupier_table_settings',
  FILTER_STATE: 'croupier_filter_state'
} as const;