type AnyRecord = Record<string, any>;

export type FunctionUIResponse = {
  ui?: any;
  active?: boolean;
  schema?: any;
  layout?: any;
  components?: any;
  custom?: boolean;
  hasDefault?: boolean;
  uiSource?: string;
  uiSourceDetail?: string;
};

export type FunctionUIUpdatePayload = {
  schema?: any;
  layout?: any;
  components?: any;
  clearCustom?: boolean;
};

function isRecord(value: any): value is AnyRecord {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function cloneValue<T>(value: T): T {
  if (value === undefined) {
    return value;
  }
  return JSON.parse(JSON.stringify(value));
}

function inferTypeFromWidget(widget?: string): string | undefined {
  switch (widget) {
    case 'number':
    case 'slider':
    case 'rate':
      return 'number';
    case 'switch':
    case 'checkbox':
    case 'radio':
      return 'boolean';
    case 'multiselect':
    case 'cascader':
    case 'treeSelect':
      return 'array';
    default:
      return 'string';
  }
}

export function toEditorUISchema(rawSchema: any): AnyRecord {
  if (!isRecord(rawSchema)) {
    return { type: 'object', properties: {} };
  }

  if (isRecord(rawSchema.properties)) {
    return cloneValue(rawSchema);
  }

  const fields = isRecord(rawSchema.fields) ? rawSchema.fields : {};
  const properties = Object.entries(fields).reduce<AnyRecord>((acc, [fieldName, rawField]) => {
    const field = isRecord(rawField) ? rawField : {};
    acc[fieldName] = {
      type: field.type || inferTypeFromWidget(field.widget),
      title: field.label || field.title || fieldName,
      description: field.description,
      placeholder: field.placeholder,
      widget: field.widget,
      enum: field.enum,
      enumNames: field.enumNames,
      format: field.format,
      minimum: field.min,
      maximum: field.max,
      minLength: field.minLength,
      maxLength: field.maxLength,
      pattern: field.pattern,
      readOnly: field.readOnly,
      disabled: field.disabled,
      hidden: field.hidden,
      span: field.span,
      rows: field.rows,
      default: field.default,
      options: field.options,
      show_if: field.show_if,
      required_if: field.required_if,
      disabled_if: field.disabled_if,
      errorMessages: field.errorMessages,
    };
    return acc;
  }, {});

  return {
    type: 'object',
    properties,
  };
}

export function toRenderableUISchema(editorSchema: any, previousSchema?: any): AnyRecord {
  if (
    isRecord(editorSchema) &&
    (isRecord(editorSchema.fields) ||
      editorSchema['ui:layout'] ||
      editorSchema['ui:groups'] ||
      editorSchema['ui:order'])
  ) {
    return cloneValue(editorSchema);
  }

  const source = isRecord(editorSchema) ? editorSchema : {};
  const previous = isRecord(previousSchema) ? previousSchema : {};
  const sourceProperties = isRecord(source.properties) ? source.properties : {};
  const previousFields = isRecord(previous.fields) ? previous.fields : {};

  const fields = Object.entries(sourceProperties).reduce<AnyRecord>(
    (acc, [fieldName, rawField]) => {
      const field = isRecord(rawField) ? rawField : {};
      const previousField = isRecord(previousFields[fieldName]) ? previousFields[fieldName] : {};

      acc[fieldName] = {
        ...cloneValue(previousField),
        type: field.type || previousField.type,
        widget: field.widget || previousField.widget,
        label: field.title || field.label || previousField.label || fieldName,
        title: field.title || previousField.title,
        description: field.description ?? previousField.description,
        placeholder: field.placeholder ?? previousField.placeholder,
        enum: field.enum ?? previousField.enum,
        enumNames: field.enumNames ?? previousField.enumNames,
        format: field.format ?? previousField.format,
        min: field.minimum ?? previousField.min,
        max: field.maximum ?? previousField.max,
        minLength: field.minLength ?? previousField.minLength,
        maxLength: field.maxLength ?? previousField.maxLength,
        pattern: field.pattern ?? previousField.pattern,
        readOnly: field.readOnly ?? previousField.readOnly,
        disabled: field.disabled ?? previousField.disabled,
        hidden: field.hidden ?? previousField.hidden,
        span: field.span ?? previousField.span,
        rows: field.rows ?? previousField.rows,
        default: field.default ?? previousField.default,
        options: field.options ?? previousField.options,
        show_if: field.show_if ?? previousField.show_if,
        required_if: field.required_if ?? previousField.required_if,
        disabled_if: field.disabled_if ?? previousField.disabled_if,
        errorMessages: field.errorMessages ?? previousField.errorMessages,
      };
      return acc;
    },
    {},
  );

  return {
    fields,
    'ui:layout': previous['ui:layout'] || { type: 'grid', cols: 2 },
    'ui:groups': previous['ui:groups'],
    'ui:order': previous['ui:order'] || Object.keys(fields),
  };
}

export function buildFunctionUISavePayload(input: FunctionUIUpdatePayload): AnyRecord {
  if (input.clearCustom) {
    const clearSchema = { __clear_custom_ui: true };
    return {
      ui: clearSchema,
      schema: clearSchema,
      layout: input.layout,
      components: input.components,
    };
  }

  return {
    ui: input.schema,
    schema: input.schema,
    layout: input.layout,
    components: input.components,
  };
}
