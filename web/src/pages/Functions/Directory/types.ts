export type I18N = { zh?: string; en?: string };

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
