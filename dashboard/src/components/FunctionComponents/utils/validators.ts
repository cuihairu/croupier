// Validation utilities for function components

/**
 * 验证函数ID格式
 */
export const validateFunctionId = (functionId: string): boolean => {
  // 函数ID规则：只能包含字母、数字、下划线、中划线和点
  const pattern = /^[a-zA-Z0-9._-]+$/;
  return pattern.test(functionId) && functionId.length > 0;
};

/**
 * 验证游戏ID格式
 */
export const validateGameId = (gameId: string): boolean => {
  const pattern = /^[a-zA-Z0-9_-]+$/;
  return pattern.test(gameId) && gameId.length > 0;
};

/**
 * 验证环境名称格式
 */
export const validateEnv = (env: string): boolean => {
  const pattern = /^[a-z0-9_-]+$/;
  return pattern.test(env) && env.length > 0;
};

/**
 * 验证JSON Schema
 */
export const validateJSONSchema = (schema: any): { valid: boolean; errors: string[] } => {
  const errors: string[] = [];

  if (!schema || typeof schema !== 'object') {
    errors.push('Schema must be a valid object');
    return { valid: false, errors };
  }

  if (schema.type && schema.type !== 'object') {
    errors.push('Root schema type must be "object"');
  }

  if (schema.properties && typeof schema.properties !== 'object') {
    errors.push('Properties must be a valid object');
  }

  if (schema.required && Array.isArray(schema.required)) {
    schema.required.forEach((prop: string) => {
      if (!schema.properties || !schema.properties[prop]) {
        errors.push(`Required property "${prop}" not found in properties`);
      }
    });
  }

  return { valid: errors.length === 0, errors };
};

/**
 * 验证函数参数
 */
export const validateFunctionParams = (params: any, schema: any): { valid: boolean; errors: string[] } => {
  const errors: string[] = [];

  if (!schema || !schema.properties) {
    return { valid: true, errors: [] };
  }

  const required = schema.required || [];

  // Check required fields
  required.forEach((field: string) => {
    if (params[field] === undefined || params[field] === null) {
      errors.push(`Required field "${field}" is missing`);
    }
  });

  // Check field types
  Object.keys(params).forEach(field => {
    const value = params[field];
    const property = schema.properties[field];

    if (!property) {
      return; // Skip unknown fields
    }

    const type = property.type;

    if (type === 'string' && typeof value !== 'string') {
      errors.push(`Field "${field}" must be a string`);
    }

    if (type === 'number' && typeof value !== 'number') {
      errors.push(`Field "${field}" must be a number`);
    }

    if (type === 'integer' && (!Number.isInteger(value) || typeof value !== 'number')) {
      errors.push(`Field "${field}" must be an integer`);
    }

    if (type === 'boolean' && typeof value !== 'boolean') {
      errors.push(`Field "${field}" must be a boolean`);
    }

    if (type === 'array' && !Array.isArray(value)) {
      errors.push(`Field "${field}" must be an array`);
    }

    if (type === 'object' && (typeof value !== 'object' || Array.isArray(value) || value === null)) {
      errors.push(`Field "${field}" must be an object`);
    }

    // String validations
    if (type === 'string' && typeof value === 'string') {
      if (property.minLength !== undefined && value.length < property.minLength) {
        errors.push(`Field "${field}" minimum length is ${property.minLength}`);
      }
      if (property.maxLength !== undefined && value.length > property.maxLength) {
        errors.push(`Field "${field}" maximum length is ${property.maxLength}`);
      }
      if (property.pattern && !new RegExp(property.pattern).test(value)) {
        errors.push(`Field "${field}" does not match required pattern`);
      }
    }

    // Number validations
    if ((type === 'number' || type === 'integer') && typeof value === 'number') {
      if (property.minimum !== undefined && value < property.minimum) {
        errors.push(`Field "${field}" minimum value is ${property.minimum}`);
      }
      if (property.maximum !== undefined && value > property.maximum) {
        errors.push(`Field "${field}" maximum value is ${property.maximum}`);
      }
    }
  });

  return { valid: errors.length === 0, errors };
};