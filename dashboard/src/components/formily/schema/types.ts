export type FormilySchema = Record<string, any>;

export type FormilySchemaVersion = 'formily:1';

export interface FormilySchemaDoc {
  functionId: string;
  version: FormilySchemaVersion;
  schema: FormilySchema;
  updatedAt?: string;
  updatedBy?: string;
  status?: 'draft' | 'published';
}
