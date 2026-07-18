export type FormilyJSONValue =
  | string
  | number
  | boolean
  | null
  | FormilyJSONValue[]
  | { [key: string]: FormilyJSONValue | undefined };

export type FormilySchemaObject = {
  type?: string;
  title?: FormilyJSONValue;
  description?: FormilyJSONValue;
  properties?: Record<string, FormilySchemaObject>;
  items?: FormilySchemaObject | FormilySchemaObject[];
  required?: string[];
  enum?: FormilyJSONValue[];
  default?: FormilyJSONValue;
  format?: string;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  ['x-component']?: string;
  ['x-decorator']?: string;
  ['x-component-props']?: Record<string, FormilyJSONValue | undefined>;
  ['x-decorator-props']?: Record<string, FormilyJSONValue | undefined>;
  ['x-validator']?: FormilyJSONValue;
  ['x-reactions']?: FormilyJSONValue;
  ['x-data-source']?: string | { url?: string; params?: Record<string, FormilyJSONValue | undefined> };
  [key: `x-${string}`]: FormilyJSONValue | undefined;
};

export type FormilySchema = FormilySchemaObject;

export type FormilyValues = Record<string, unknown>;

export type FormilyScope = Record<string, unknown>;

export type FormilySchemaVersion = 'formily:1';

export interface FormilySchemaDoc {
  functionId: string;
  version: FormilySchemaVersion;
  schema: FormilySchema;
  updatedAt?: string;
  updatedBy?: string;
  status?: 'draft' | 'published';
}
