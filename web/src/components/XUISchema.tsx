import type React from 'react';

export interface XUISchemaField {
  neverUseLegacyXUISchema?: never;
}

export interface XUISchemaProps {
  neverUseLegacyXUISchema?: never;
}

export const X_UI_SCHEMA_REMOVED_ERROR =
  'XUISchema 已废弃：函数 UI 只允许使用 Formily SchemaRenderer。';

export function evaluateCondition(): boolean {
  throw new Error(X_UI_SCHEMA_REMOVED_ERROR);
}

export function renderXUIField(): React.ReactNode {
  throw new Error(X_UI_SCHEMA_REMOVED_ERROR);
}
