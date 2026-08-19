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
};

export type DetailRow = SummaryRow & {
  description?: I18N;
  author?: string;
  createdAt?: string;
  updatedAt?: string;
  instances?: number;
};
