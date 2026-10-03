import type { LocalizedText } from '@/types/dashboard';

export type I18N = LocalizedText;

export type SummaryRow = {
  id: string;
  enabled?: boolean;
  displayName?: I18N;
  summary?: I18N;
  resource?: string;
  operation?: string;
  tags?: string[];
  version?: string;
  /** 函数级最低 SDK 版本门槛（未配置为 undefined） */
  minVersion?: string;
  /**
   * 开放范围摘要：open=开放该函数的环境数、total=已保存白名单的环境数。
   * undefined=白名单拉取失败（未知）；total=0=该游戏默认开放（未配置白名单）。
   */
  assignmentScope?: { open: number; total: number };
};

export type DetailRow = SummaryRow & {
  description?: I18N;
  author?: string;
  createdAt?: string;
  updatedAt?: string;
  instances?: number;
};
