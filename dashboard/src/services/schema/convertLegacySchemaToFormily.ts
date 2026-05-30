function mapWidget(widget?: string): string | undefined {
  if (!widget) return undefined;
  const w = widget.toLowerCase();
  if (w === 'input') return 'Input';
  if (w === 'textarea') return 'Input.TextArea';
  if (w === 'number' || w === 'inputnumber') return 'NumberPicker';
  if (w === 'select') return 'Select';
  if (w === 'radio') return 'Radio.Group';
  if (w === 'checkbox' || w === 'checkboxes') return 'Checkbox.Group';
  if (w === 'switch') return 'Switch';
  if (w === 'date') return 'DatePicker';
  if (w === 'datetime') return 'DatePicker';
  if (w === 'array') return 'ArrayItems';
  return undefined;
}

function normalizeNode(node: any): any {
  if (!node || typeof node !== 'object') return node;
  const next: any = Array.isArray(node) ? [] : { ...node };

  if (!next['x-component']) {
    const widget = next['ui:widget'] || next.widget;
    const mapped = mapWidget(widget);
    if (mapped) next['x-component'] = mapped;
  }

  if (!next['x-decorator'] && next['x-component']) {
    next['x-decorator'] = 'FormItem';
  }

  if (next['ui:props'] && !next['x-component-props']) {
    next['x-component-props'] = next['ui:props'];
  }

  if (next['ui:options'] && !next['x-component-props']) {
    next['x-component-props'] = next['ui:options'];
  }

  if (typeof next['ui:disabled'] === 'boolean' && next['x-disabled'] === undefined) {
    next['x-disabled'] = next['ui:disabled'];
  }

  if (typeof next['ui:readonly'] === 'boolean' && next['x-readonly'] === undefined) {
    next['x-readonly'] = next['ui:readonly'];
  }

  if (next['ui:rules'] && !next['x-validator']) {
    next['x-validator'] = next['ui:rules'];
  }

  if (next.properties) {
    const props: any = {};
    Object.keys(next.properties).forEach((key) => {
      props[key] = normalizeNode(next.properties[key]);
    });
    next.properties = props;
  }

  if (next.items) {
    if (Array.isArray(next.items)) {
      next.items = next.items.map((item: any) => normalizeNode(item));
    } else {
      next.items = normalizeNode(next.items);
    }
  }

  return next;
}

export function convertLegacySchemaToFormily(schema: any): any {
  return normalizeNode(schema);
}
