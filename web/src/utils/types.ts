import type { FormilySchema } from '@/components/formily/schema/types';

export type FunctionFormDraft = {
  functionId: string;
  schema: FormilySchema;
  updatedAt?: string;
  baseUpdatedAt?: string;
};

export type FunctionFormSource =
  | 'custom_metadata'
  | 'config_file_override'
  | 'generated_default'
  | 'none'
  | string;
