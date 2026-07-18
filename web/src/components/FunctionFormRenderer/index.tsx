import React from 'react';
import type { FormilySchema, FormilyValues } from '@/components/formily/schema/types';

export interface FunctionFormRendererProps {
  schema: FormilySchema;
  initialValues?: FormilyValues;
  onSubmit?: (values: FormilyValues) => void;
  onSecondarySubmit?: (values: FormilyValues) => void;
}

export const FUNCTION_FORM_RENDERER_REMOVED_ERROR =
  'FunctionFormRenderer 已废弃：函数 UI 只允许使用 Formily SchemaRenderer。';

export const FunctionFormRenderer: React.FC<FunctionFormRendererProps> = () => {
  throw new Error(FUNCTION_FORM_RENDERER_REMOVED_ERROR);
};

export default FunctionFormRenderer;
