export type I18N = { zh?: string; en?: string };
export type Menu = { nodes?: string[]; path?: string; order?: number; hidden?: boolean };

export type SummaryRow = {
  id: string;
  enabled?: boolean;
  displayName?: I18N;
  summary?: I18N;
  entity?: string;
  tags?: string[];
  menu?: Menu;
  version?: string;
  category?: string;
};

export type DetailRow = SummaryRow & {
  description?: I18N;
  author?: string;
  createdAt?: string;
  updatedAt?: string;
  instances?: number;
};
