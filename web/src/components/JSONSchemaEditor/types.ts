import type { JSONValue } from '@/types/dashboard';

export interface SchemaObject {
  type?: string;
  properties?: Record<string, PropertyConfig>;
  required?: string[];
}

export type SchemaTemplate = {
  type: 'object';
  properties: Record<string, PropertyConfig>;
  required?: string[];
};

export interface PropertyConfig {
  type: 'string' | 'number' | 'integer' | 'boolean' | 'array' | 'object';
  title?: string;
  description?: string;
  default?: JSONValue;
  enum?: JSONValue[];
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  format?: string;
  required?: boolean;
  readOnly?: boolean;
  const?: JSONValue;
  $ref?: string;
  // Array specific
  items?: PropertyConfig;
  minItems?: number;
  maxItems?: number;
  uniqueItems?: boolean;
  // Object specific
  properties?: Record<string, PropertyConfig>;
  additionalProperties?: boolean | PropertyConfig;
  requiredProperties?: string[];
  // UI specific
  widget?: string;
  placeholder?: string;
  help?: string;
}

export interface JSONSchemaEditorProps {
  value?: SchemaObject;
  onChange?: (value: SchemaObject) => void;
  schemaFormSchema?: SchemaObject;
}
