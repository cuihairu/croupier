import React, { useEffect, useMemo, useRef } from 'react';
import type { Form as FormilyForm } from '@formily/core';
import { createForm, onFormValuesChange } from '@formily/core';
import { createSchemaField } from '@formily/react';
import {
  Form as FormilyFormLayout,
  FormItem,
  Input,
  NumberPicker,
  Select,
  Switch,
  DatePicker,
  ArrayTable,
  ArrayItems,
  FormGrid,
  Space,
  Checkbox,
  Radio,
  PreviewText,
} from '@formily/antd-v5';
import { Card, Empty } from 'antd';
import FormilyProvider from './FormilyProvider';
import { FormilyContextProvider, type FormilyRuntimeContext } from './context';
import type { FormilySchema } from './schema/types';

interface SchemaRendererProps {
  schema?: FormilySchema;
  value?: Record<string, any>;
  readOnly?: boolean;
  onChange?: (values: Record<string, any>) => void;
  scope?: Record<string, any>;
  context?: FormilyRuntimeContext;
  effects?: (form: FormilyForm) => void;
}

function withDefaultDecorator(schema: any): any {
  if (!schema || typeof schema !== 'object') return schema;
  const walk = (node: any, fieldName = ''): any => {
    if (!node || typeof node !== 'object') return node;
    const next: Record<string, any> = { ...node };

    if (!next.title && fieldName) {
      next.title = fieldName.replace(/_/g, ' ');
    }
    if (next['x-component'] && !next['x-decorator'] && next.type !== 'void') {
      next['x-decorator'] = 'FormItem';
    }

    if (next.properties && typeof next.properties === 'object') {
      const mapped: Record<string, any> = {};
      Object.keys(next.properties).forEach((key) => {
        mapped[key] = walk(next.properties[key], key);
      });
      next.properties = mapped;
    }
    if (next.items && typeof next.items === 'object') {
      next.items = walk(next.items, fieldName ? `${fieldName}Item` : 'item');
    }
    return next;
  };
  return walk(schema);
}

const SchemaField = createSchemaField({
  components: {
    FormItem,
    Input,
    NumberPicker,
    Select,
    Switch,
    DatePicker,
    ArrayTable,
    ArrayItems,
    FormGrid,
    Space,
    Card,
    Checkbox,
    Radio,
  },
});

export default function SchemaRenderer({
  schema,
  value,
  readOnly,
  onChange,
  scope,
  context,
  effects,
}: SchemaRendererProps) {
  const formRef = useRef<FormilyForm | null>(null);
  const form = useMemo(() => {
    if (formRef.current) return formRef.current;
    const created = createForm({
      readPretty: !!readOnly,
      values: value || {},
      effects: (formInstance) => {
        if (effects) effects(formInstance);
      },
    });
    formRef.current = created;
    return created;
  }, [readOnly, value]);

  useEffect(() => {
    form.setValues(value || {}, 'overwrite');
  }, [form, value]);

  useEffect(() => {
    form.setState((state) => {
      state.readPretty = !!readOnly;
    });
  }, [form, readOnly]);

  useEffect(() => {
    if (!onChange) return undefined;
    const effectId = `schema-renderer:${Date.now()}`;
    form.addEffects(effectId, () => {
      onFormValuesChange((next) => {
        onChange(next.values as Record<string, any>);
      });
    });
    return () => form.removeEffects(effectId);
  }, [form, onChange]);

  const normalizedSchema = useMemo(() => withDefaultDecorator(schema), [schema]);

  useEffect(() => {
    if (!normalizedSchema || !scope?.fetchOptions) return;
    const tasks: Array<{ path: string; source: any }> = [];
    const walk = (node: any, path: string) => {
      if (!node || typeof node !== 'object') return;
      if (node['x-data-source']) {
        tasks.push({ path, source: node['x-data-source'] });
      }
      if (node.properties) {
        Object.keys(node.properties).forEach((key) => {
          walk(node.properties[key], path ? `${path}.${key}` : key);
        });
      }
    };
    walk(normalizedSchema, '');
    if (tasks.length === 0) return;
    tasks.forEach(async ({ path, source }) => {
      try {
        const url = typeof source === 'string' ? source : source?.url;
        if (!url) return;
        const params = typeof source === 'object' ? source?.params : undefined;
        const options = await scope.fetchOptions(url, params);
        form.setFieldState(path, (state) => {
          state.componentProps = { ...(state.componentProps || {}), options };
        });
      } catch {
        // ignore async option errors
      }
    });
  }, [form, normalizedSchema, scope]);

  if (!normalizedSchema || typeof normalizedSchema !== 'object') {
    return <Empty description="暂无可渲染的 Schema" />;
  }

  return (
    <FormilyContextProvider value={context || {}}>
      <FormilyProvider form={form}>
        <FormilyFormLayout layout="vertical" form={form}>
          <SchemaField schema={normalizedSchema} scope={scope} />
          {readOnly && <PreviewText />}
        </FormilyFormLayout>
      </FormilyProvider>
    </FormilyContextProvider>
  );
}
