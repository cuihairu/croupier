/**
 * Type definitions for Function UI Generator
 */

/**
 * Formily Schema type
 */
export interface FormilySchema {
  type?: string;
  title?: string;
  description?: string;
  default?: any;
  required?: string[] | boolean;
  properties?: Record<string, FormilySchema>;
  items?: FormilySchema;
  enum?: any[];
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  readOnly?: boolean;
  writeOnly?: boolean;
  // Formily-specific extensions
  'x-component'?: string;
  'x-component-props'?: Record<string, any>;
  'x-decorator'?: string;
  'x-decorator-props'?: Record<string, any>;
  'x-reactions'?: any;
}

/**
 * UI Schema type for controlling form rendering
 */
export interface UISchema {
  widget?: string;
  placeholder?: string;
  disabled?: boolean;
  readonly?: boolean;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  properties?: Record<string, UISchema>;
  items?: UISchema;
  'ui:widget'?: string;
  'ui:placeholder'?: string;
  'ui:disabled'?: boolean;
  'ui:readonly'?: boolean;
}

/**
 * Complete UI Configuration
 */
export interface UIConfig {
  menu?: {
    path?: string[];
    icon?: string;
    order?: number;
    visible?: boolean;
  };
  form?: {
    schema?: FormilySchema;
    uiSchema?: UISchema;
    mode?: 'modal' | 'drawer' | 'page' | 'inline';
    size?: 'small' | 'medium' | 'large';
    layout?: 'vertical' | 'horizontal';
  };
  operation?: {
    type?: 'query' | 'create' | 'update' | 'delete' | 'custom';
    confirmRequired?: boolean;
    confirmMessage?: string;
    style?: 'modal' | 'drawer' | 'page' | 'inline';
  };
  display?: {
    inMenu?: boolean;
    inSearch?: boolean;
    groupByCategory?: boolean;
  };
  security?: {
    requirePermission?: boolean;
    permission?: string;
    requireApproval?: boolean;
    approvalType?: 'single' | 'two_person';
  };
  style?: {
    riskColor?: string;
    primaryColor?: string;
    dangerColor?: string;
  };
}
