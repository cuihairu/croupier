/**
 * Function UI Generator
 *
 * Automatically generates UI configurations from FunctionMetadata.
 * Includes menu path derivation, icon mapping, operation type inference,
 * and Formily Schema conversion.
 */

import type { FormilySchema, UISchema, UIConfig } from './types';

/**
 * FunctionMetadata type matching the backend API response
 */
export interface FunctionMetadata {
  id: string;
  version?: string;
  category: string;
  tags: string[];
  name: string;
  description?: string;
  input_schema?: string;
  output_schema?: string;
  behavior?: {
    mode: 'query' | 'command';
    idempotent: boolean;
    timeout_ms: number;
    route_strategy: 'lb' | 'broadcast' | 'targeted' | 'hash';
    cacheable: boolean;
    cache_ttl_seconds?: number;
  };
  security?: {
    risk_level: 'low' | 'medium' | 'high' | 'danger';
    permission?: string;
    requires_approval: boolean;
    approval_type?: 'none' | 'single' | 'two_person';
    allowed_roles?: string[];
    audit_log: boolean;
    mask_sensitive_data: boolean;
  };
  extensions?: Record<string, string>;
  created_at?: string;
  updated_at?: string;
}

/**
 * Generated UI configuration for a function
 */
export interface FunctionUIConfig {
  // Menu configuration
  menuPath: string[];
  menuIcon: string;
  menuOrder: number;

  // Formily form schema
  formSchema: FormilySchema;

  // UI schema for form rendering
  uiSchema: UISchema;

  // Operation configuration
  operationType: 'query' | 'create' | 'update' | 'delete' | 'custom';
  operationStyle: 'modal' | 'drawer' | 'page' | 'inline';
  confirmRequired: boolean;
  confirmMessage?: string;

  // Display configuration
  displayInMenu: boolean;
  displayInSearch: boolean;
  groupByCategory: boolean;

  // Security configuration
  requirePermission: boolean;
  permission?: string;
  requireApproval: boolean;
  approvalType?: 'single' | 'two_person';

  // Color coding by risk level
  riskColor: string;
}

/**
 * Category to icon mapping
 */
const CATEGORY_ICONS: Record<string, string> = {
  // Player management
  player: 'UserOutlined',
  player_info: 'IdcardOutlined',
  player_ban: 'StopOutlined',
  player_kick: 'LogoutOutlined',
  player_mute: 'MutedOutlined',

  // Game management
  game: 'GameOutlined',
  game_config: 'SettingOutlined',
  game_match: 'TrophyOutlined',
  game_rank: 'RiseOutlined',

  // Item management
  item: 'GiftOutlined',
  item_add: 'PlusSquareOutlined',
  item_remove: 'MinusSquareOutlined',
  item_use: 'ToolOutlined',

  // System
  system: 'ControlOutlined',
  system_log: 'FileTextOutlined',
  system_monitor: 'MonitorOutlined',
  system_config: 'ControlOutlined',

  // Moderation
  moderation: 'SafetyOutlined',
  moderation_chat: 'MessageOutlined',
  moderation_behavior: 'AlertOutlined',

  // Economy
  economy: 'DollarOutlined',
  economy_currency: 'MoneyCollectOutlined',
  economy_shop: 'ShoppingOutlined',
  economy_trade: 'SwapOutlined',

  // Guild/Clan
  guild: 'TeamOutlined',
  guild_member: 'UsergroupAddOutlined',
  guild_war: 'ThunderboltOutlined',

  // Analytics
  analytics: 'LineChartOutlined',
  analytics_report: 'BarChartOutlined',
  analytics_stats: 'DotChartOutlined',

  // Communication
  chat: 'MessageOutlined',
  chat_global: 'GlobalOutlined',
  chat_private: 'LockOutlined',
  mail: 'MailOutlined',

  // Events
  event: 'CalendarOutlined',
  event_timed: 'ClockCircleOutlined',
  event_limited: 'HourglassOutlined',

  // Default fallback
  default: 'AppstoreOutlined',
};

/**
 * Category to color mapping (for risk level visualization)
 */
const RISK_COLORS: Record<string, string> = {
  low: '#52c41a',    // green
  medium: '#faad14', // orange
  high: '#ff4d4f',   // red
  danger: '#cf1322', // dark red
};

/**
 * Parse function ID to extract domain, entity, and action
 */
function parseFunctionID(id: string): { domain: string; entity: string; action?: string } {
  const parts = id.split('.');
  if (parts.length >= 2) {
    return {
      domain: parts[0],
      entity: parts[1],
      action: parts[2],
    };
  }
  return { domain: 'unknown', entity: id };
}

/**
 * Derive menu path from category and function ID
 */
export function deriveMenuPath(metadata: FunctionMetadata): string[] {
  const { category, id } = metadata;
  const { domain, entity } = parseFunctionID(id);

  // Build menu path: [Domain, Category, Entity]
  const path: string[] = [];

  // Add domain as top level
  if (domain !== 'unknown') {
    path.push(domain.charAt(0).toUpperCase() + domain.slice(1));
  }

  // Add category as second level
  if (category && category !== domain) {
    path.push(category.charAt(0).toUpperCase() + category.slice(1));
  }

  // Add entity as third level if different from category
  if (entity && entity !== category) {
    path.push(entity.charAt(0).toUpperCase() + entity.slice(1).replace(/_/g, ' '));
  }

  return path.length > 0 ? path : ['Functions'];
}

/**
 * Map category to icon
 */
export function mapCategoryToIcon(category: string): string {
  // Direct match
  if (CATEGORY_ICONS[category]) {
    return CATEGORY_ICONS[category];
  }

  // Partial match (e.g., "player_ban" -> "player")
  for (const [key, icon] of Object.entries(CATEGORY_ICONS)) {
    if (category.includes(key) || key.includes(category)) {
      return icon;
    }
  }

  return CATEGORY_ICONS.default;
}

/**
 * Infer operation type from function metadata
 */
export function inferOperationType(metadata: FunctionMetadata): 'query' | 'create' | 'update' | 'delete' | 'custom' {
  const { id, behavior } = metadata;
  const { entity, action } = parseFunctionID(id);

  // Check behavior mode first
  if (behavior?.mode === 'query') {
    return 'query';
  }

  // Infer from action name
  const actionLower = (action || '').toLowerCase();
  if (actionLower.includes('create') || actionLower.includes('add') || actionLower.includes('new')) {
    return 'create';
  }
  if (actionLower.includes('update') || actionLower.includes('modify') || actionLower.includes('edit') || actionLower.includes('change')) {
    return 'update';
  }
  if (actionLower.includes('delete') || actionLower.includes('remove') || actionLower.includes('ban') || actionLower.includes('kick')) {
    return 'delete';
  }

  return 'custom';
}

/**
 * Determine operation style based on operation type and risk level
 */
export function determineOperationStyle(
  operationType: string,
  riskLevel: string,
): 'modal' | 'drawer' | 'page' | 'inline' {
  // High-risk operations use modal with confirmation
  if (riskLevel === 'high' || riskLevel === 'danger') {
    return 'modal';
  }

  // Create operations often use drawer or modal
  if (operationType === 'create') {
    return 'drawer';
  }

  // Update operations use drawer
  if (operationType === 'update') {
    return 'drawer';
  }

  // Delete operations use modal
  if (operationType === 'delete') {
    return 'modal';
  }

  // Query operations use inline or page
  if (operationType === 'query') {
    return 'inline';
  }

  return 'modal';
}

/**
 * Generate confirmation message based on operation type
 */
export function generateConfirmMessage(operationType: string, entityName: string): string | undefined {
  const messages: Record<string, string> = {
    delete: `Are you sure you want to delete this ${entityName}? This action cannot be undone.`,
    create: undefined,
    update: undefined,
    query: undefined,
    custom: `Are you sure you want to perform this action?`,
  };

  return messages[operationType];
}

/**
 * Convert JSON Schema to Formily Schema
 */
export function convertToFormilySchema(jsonSchema: string): FormilySchema {
  try {
    const schema = JSON.parse(jsonSchema || '{}');
    return convertSchemaToFormily(schema);
  } catch {
    // Return default schema if parsing fails
    return {
      type: 'object',
      properties: {},
    };
  }
}

/**
 * Recursively convert JSON Schema to Formily Schema
 */
function convertSchemaToFormily(schema: any): FormilySchema {
  if (!schema || typeof schema !== 'object') {
    return { type: 'string' };
  }

  const formily: FormilySchema = {
    type: schema.type || 'string',
    title: schema.title || '',
    description: schema.description,
    default: schema.default,
    required: schema.required || false,
  };

  // Handle array types
  if (schema.type === 'array' && schema.items) {
    formily.items = convertSchemaToFormily(schema.items);
  }

  // Handle object types with properties
  if (schema.type === 'object' && schema.properties) {
    formily.properties = {};
    formily.required = schema.required || [];

    for (const [key, value] of Object.entries(schema.properties)) {
      formily.properties[key] = convertSchemaToFormily(value);
    }
  }

  // Handle enum values
  if (schema.enum) {
    formily.enum = schema.enum;
    formily['x-component'] = 'Select';
  }

  // Handle string format
  if (schema.format) {
    switch (schema.format) {
      case 'date':
        formily['x-component'] = 'DatePicker';
        break;
      case 'date-time':
        formily['x-component'] = 'DateTimePicker';
        break;
      case 'time':
        formily['x-component'] = 'TimePicker';
        break;
      case 'email':
        formily['x-component'] = 'Input';
        formily['x-component-props'] = { type: 'email' };
        break;
      case 'uri':
      case 'url':
        formily['x-component'] = 'Input';
        formily['x-component-props'] = { type: 'url' };
        break;
      case 'textarea':
        formily['x-component'] = 'Input.TextArea';
        break;
    }
  }

  // Handle number constraints
  if (schema.minimum !== undefined) {
    formily.minimum = schema.minimum;
  }
  if (schema.maximum !== undefined) {
    formily.maximum = schema.maximum;
  }
  if (schema.minLength !== undefined) {
    formily.minLength = schema.minLength;
  }
  if (schema.maxLength !== undefined) {
    formily.maxLength = schema.maxLength;
  }
  if (schema.pattern) {
    formily.pattern = schema.pattern;
  }

  // Auto-detect component from type
  if (!formily['x-component']) {
    switch (schema.type) {
      case 'boolean':
        formily['x-component'] = 'Checkbox';
        break;
      case 'number':
      case 'integer':
        formily['x-component'] = 'InputNumber';
        break;
      case 'array':
        formily['x-component'] = 'ArrayTable';
        break;
      case 'object':
        formily['x-component'] = 'Card';
        break;
      default:
        formily['x-component'] = 'Input';
    }
  }

  return formily;
}

/**
 * Generate UI schema from JSON Schema
 */
export function generateUISchema(jsonSchema: string): UISchema {
  try {
    const schema = JSON.parse(jsonSchema || '{}');
    return buildUISchema(schema);
  } catch {
    return {};
  }
}

/**
 * Build UI schema from JSON Schema
 */
function buildUISchema(schema: any, parentKey = ''): UISchema {
  const uiSchema: UISchema = {};

  if (!schema || typeof schema !== 'object') {
    return uiSchema;
  }

  // Handle object properties
  if (schema.properties) {
    for (const [key, value] of Object.entries(schema.properties)) {
      const fieldConfig = value as any;
      const fieldUi: UISchema = {};

      // Set widget based on type and format
      if (fieldConfig.enum) {
        fieldUi.widget = 'select';
      } else if (fieldConfig.type === 'boolean') {
        fieldUi.widget = 'checkbox';
      } else if (fieldConfig.type === 'number' || fieldConfig.type === 'integer') {
        fieldUi.widget = 'number';
      } else if (fieldConfig.type === 'array') {
        fieldUi.widget = 'array';
      } else if (fieldConfig.format === 'textarea') {
        fieldUi.widget = 'textarea';
      } else if (fieldConfig.format?.includes('date')) {
        fieldUi.widget = fieldConfig.format;
      }

      // Set placeholder
      if (fieldConfig.description) {
        fieldUi.placeholder = fieldConfig.description;
      }

      // Set disabled/readonly
      if (fieldConfig.readOnly) {
        fieldUi.disabled = true;
      }

      // Set validation
      if (fieldConfig.minimum !== undefined || fieldConfig.maximum !== undefined) {
        fieldUi.minimum = fieldConfig.minimum;
        fieldUi.maximum = fieldConfig.maximum;
      }
      if (fieldConfig.minLength !== undefined || fieldConfig.maxLength !== undefined) {
        fieldUi.minLength = fieldConfig.minLength;
        fieldUi.maxLength = fieldConfig.maxLength;
      }

      // Recursively handle nested objects
      if (fieldConfig.type === 'object' && fieldConfig.properties) {
        fieldUi.properties = buildUISchema(fieldConfig.properties, key);
      }

      uiSchema[key] = fieldUi;
    }
  }

  return uiSchema;
}

/**
 * Calculate menu order based on category and function importance
 */
export function calculateMenuOrder(metadata: FunctionMetadata): number {
  const { category, security } = metadata;

  // Higher priority for critical functions
  if (security?.risk_level === 'danger') {
    return 100;
  }

  // Category-based ordering
  const categoryOrder: Record<string, number> = {
    system: 1000,
    player: 2000,
    game: 3000,
    moderation: 4000,
    economy: 5000,
    guild: 6000,
    analytics: 7000,
    event: 8000,
    chat: 9000,
  };

  return categoryOrder[category] || 5000;
}

/**
 * Generate complete UI configuration from function metadata
 */
export function generateUIConfig(metadata: FunctionMetadata): FunctionUIConfig {
  const operationType = inferOperationType(metadata);
  const riskLevel = metadata.security?.risk_level || 'medium';

  return {
    menuPath: deriveMenuPath(metadata),
    menuIcon: mapCategoryToIcon(metadata.category),
    menuOrder: calculateMenuOrder(metadata),

    formSchema: convertToFormilySchema(metadata.input_schema || '{}'),
    uiSchema: generateUISchema(metadata.input_schema || '{}'),

    operationType,
    operationStyle: determineOperationStyle(operationType, riskLevel),
    confirmRequired: riskLevel === 'high' || riskLevel === 'danger' || operationType === 'delete',
    confirmMessage: generateConfirmMessage(operationType, metadata.name),

    displayInMenu: true,
    displayInSearch: metadata.behavior?.mode === 'query',
    groupByCategory: true,

    requirePermission: !!metadata.security?.permission,
    permission: metadata.security?.permission,
    requireApproval: metadata.security?.requires_approval || false,
    approvalType: metadata.security?.approval_type === 'two_person' ? 'two_person' : 'single',

    riskColor: RISK_COLORS[riskLevel] || RISK_COLORS.medium,
  };
}

/**
 * Batch generate UI configs for multiple functions
 */
export function batchGenerateUIConfigs(metadatas: FunctionMetadata[]): Record<string, FunctionUIConfig> {
  const configs: Record<string, FunctionUIConfig> = {};

  for (const metadata of metadatas) {
    configs[metadata.id] = generateUIConfig(metadata);
  }

  return configs;
}

/**
 * Export types for external use
 */
export type {
  FormilySchema,
  UISchema,
  UIConfig,
  FunctionMetadata,
  FunctionUIConfig,
};
