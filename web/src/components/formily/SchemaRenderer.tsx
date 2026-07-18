import React, { useEffect, useMemo, useRef } from 'react';
import type { Form as FormilyForm } from '@formily/core';
import { createForm, onFormValuesChange } from '@formily/core';
import { createSchemaField } from '@formily/react';
import type { ISchema } from '@formily/react';
import {
  Form as FormilyFormLayout,
  FormItem,
  Input,
  Password,
  NumberPicker,
  Select,
  Switch,
  DatePicker,
  TimePicker,
  ArrayTable,
  ArrayItems,
  ArrayCards,
  ArrayCollapse,
  ArrayTabs,
  FormGrid,
  FormCollapse,
  FormTab,
  FormStep,
  Space,
  Checkbox,
  Radio,
  Cascader,
  TreeSelect,
  Transfer,
  Upload,
  PreviewText,
} from '@formily/antd-v5';
import { Card, Empty } from 'antd';
import FormilyProvider from './FormilyProvider';
import { FormilyContextProvider, type FormilyRuntimeContext } from './context';
import type { FormilyScope, FormilySchema, FormilyValues } from './schema/types';

interface SchemaRendererProps {
  schema?: FormilySchema;
  value?: FormilyValues;
  readOnly?: boolean;
  onChange?: (values: FormilyValues) => void;
  scope?: FormilyScope;
  context?: FormilyRuntimeContext;
  effects?: (form: FormilyForm) => void;
  onFormReady?: (form: FormilyForm) => void;
}

function stableStringify(value: unknown): string {
  try {
    return JSON.stringify(value) || '';
  } catch {
    return '';
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function withDefaultDecorator(schema?: FormilySchema): FormilySchema | undefined {
  if (!isRecord(schema)) return schema;
  const walk = (node: unknown, fieldName = ''): unknown => {
    if (!isRecord(node)) return node;
    const next: Record<string, unknown> = { ...node };

    if (next.title === undefined && fieldName) {
      next.title = fieldName.replace(/_/g, ' ');
    }
    if (typeof next['x-component'] === 'string' && !next['x-decorator'] && next.type !== 'void') {
      next['x-decorator'] = 'FormItem';
    }

    if (isRecord(next.properties)) {
      const mapped: Record<string, unknown> = {};
      Object.keys(next.properties).forEach((key) => {
        mapped[key] = walk((next.properties as Record<string, unknown>)[key], key);
      });
      next.properties = mapped;
    }
    if (Array.isArray(next.items)) {
      next.items = next.items.map((item) => walk(item, fieldName ? `${fieldName}Item` : 'item'));
    } else if (isRecord(next.items)) {
      next.items = walk(next.items, fieldName ? `${fieldName}Item` : 'item');
    }
    return next;
  };
  return walk(schema) as FormilySchema;
}

const SchemaField = createSchemaField({
  components: {
    FormItem,
    Input,
    Password,
    NumberPicker,
    Select,
    Switch,
    DatePicker,
    TimePicker,
    ArrayTable,
    ArrayItems,
    ArrayCards,
    ArrayCollapse,
    ArrayTabs,
    FormGrid,
    FormCollapse,
    FormTab,
    FormStep,
    Space,
    Card,
    Checkbox,
    Radio,
    Cascader,
    TreeSelect,
    Transfer,
    Upload,
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
  onFormReady,
}: SchemaRendererProps) {
  const formRef = useRef<FormilyForm | null>(null);
  const schemaKey = useMemo(() => stableStringify(schema), [schema]);
  const form = useMemo(() => {
    const created = createForm({
      readPretty: !!readOnly,
      values: value || {},
      effects: (formInstance) => {
        if (effects) effects(formInstance);
      },
    });
    formRef.current = created;
    return created;
  }, [readOnly, schemaKey]);

  useEffect(() => {
    form.setValues(value || {}, 'overwrite');
  }, [form, value]);

  useEffect(() => {
    form.setState((state) => {
      state.readPretty = !!readOnly;
    });
  }, [form, readOnly]);

  useEffect(() => {
    onFormReady?.(form);
  }, [form, onFormReady]);

  useEffect(() => {
    if (!onChange) return undefined;
    const effectId = `schema-renderer:${Date.now()}`;
    form.addEffects(effectId, () => {
      onFormValuesChange((next) => {
        onChange(next.values as FormilyValues);
      });
    });
    return () => form.removeEffects(effectId);
  }, [form, onChange]);

  const normalizedSchema = useMemo(() => withDefaultDecorator(schema), [schema]);

  useEffect(() => {
    if (!normalizedSchema || !scope?.fetchOptions) return;
    const fetchOptions = scope.fetchOptions;
    if (typeof fetchOptions !== 'function') return;

    const tasks: Array<{ path: string; source: unknown }> = [];
    const walk = (node: unknown, path: string) => {
      if (!isRecord(node)) return;
      if (node['x-data-source']) {
        tasks.push({ path, source: node['x-data-source'] });
      }
      if (isRecord(node.properties)) {
        Object.keys(node.properties).forEach((key) => {
          walk((node.properties as Record<string, unknown>)[key], path ? `${path}.${key}` : key);
        });
      }
    };
    walk(normalizedSchema, '');
    if (tasks.length === 0) return;
    tasks.forEach(async ({ path, source }) => {
      try {
        const sourceObj = isRecord(source) ? source : undefined;
        const url = typeof source === 'string' ? source : sourceObj?.url;
        if (typeof url !== 'string') return;
        if (!url) return;
        const params = isRecord(sourceObj?.params) ? sourceObj.params : undefined;
        const options = await fetchOptions(url, params);
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
          <SchemaField schema={normalizedSchema as ISchema} scope={scope} />
          {readOnly && <PreviewText />}
        </FormilyFormLayout>
      </FormilyProvider>
    </FormilyContextProvider>
  );
}
