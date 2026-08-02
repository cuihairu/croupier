import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Alert, App, Button, Card, Drawer, Empty, Space, Tag, Typography } from 'antd';
import { InfoCircleOutlined } from '@ant-design/icons';
import { history, useLocation, getLocale, useModel } from '@umijs/max';
import SchemaFormRenderer, {
  type SchemaFormRendererHandle,
} from '@/components/SchemaFormRenderer';
import {
  invokeFunction,
  listDescriptors,
  startTask,
  type FunctionDescriptor,
} from '@/services/api';
import { extractErrorMessage } from '@/utils/errors';
import { parseInputSchema, type JSONSchemaType } from '@/utils/json';
import type { FormPresentationSpec, FormValues, JSONSchema, JSONValue } from '@/types/dashboard';

const { Text } = Typography;

type FormSchemaState =
  | {
      status: 'idle' | 'loading';
      schema?: undefined;
      detail?: undefined;
      error?: undefined;
    }
  | {
      status: 'ready';
      spec: FormPresentationSpec;
      detail?: string;
      error?: undefined;
    }
  | {
      status: 'error';
      schema?: undefined;
      detail?: string;
      error: string;
    };

const EMPTY_FORM_STATE: FormSchemaState = { status: 'idle' };

const resolveName = (descriptor: FunctionDescriptor, locale: string) => {
  const zh = descriptor.displayName?.zh || descriptor.summary?.zh;
  const en = descriptor.displayName?.en || descriptor.summary?.en;
  if (locale.toLowerCase().startsWith('zh')) return zh || en || descriptor.id;
  return en || zh || descriptor.id;
};

const resolveSummary = (descriptor: FunctionDescriptor, locale: string) => {
  const zh = descriptor.summary?.zh || descriptor.displayName?.zh;
  const en = descriptor.summary?.en || descriptor.displayName?.en;
  if (locale.toLowerCase().startsWith('zh')) {
    return zh || en || descriptor.description || descriptor.id;
  }
  return en || zh || descriptor.description || descriptor.id;
};

function parseDescriptorSchema(value: unknown): JSONSchemaType | null {
  if (!value) return null;
  if (typeof value === 'string') return parseInputSchema(value);
  if (typeof value === 'object' && !Array.isArray(value)) return value as JSONSchemaType;
  return null;
}

function resolveInputSchema(descriptor: FunctionDescriptor): JSONSchemaType | null {
  return (
    parseDescriptorSchema(descriptor.inputSchema) ||
    parseDescriptorSchema(descriptor.schema) ||
    parseDescriptorSchema(descriptor.params)
  );
}

function toJSONValue(values: FormValues): JSONValue {
  return JSON.parse(JSON.stringify(values)) as JSONValue;
}

function buildFormPresentationSpec(schema: JSONSchemaType): FormPresentationSpec {
  return {
    jsonSchema: schema as JSONSchema,
    layout: 'vertical',
    submitButton: {
      text: { 'zh-CN': '执行', en: 'Invoke' },
    },
  };
}

type InitialStateWithAccess = {
  currentUser?: {
    access?: string;
  };
};

export default function FunctionRuntimeUIPage() {
  const { message } = App.useApp();
  const location = useLocation();
  const locale = getLocale();
  const { initialState } = useModel('@@initialState');
  const formRef = useRef<SchemaFormRendererHandle | null>(null);
  const searchParams = new URLSearchParams(location.search);
  const fid = searchParams.get('fid') || searchParams.get('id') || '';

  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [descriptors, setDescriptors] = useState<FunctionDescriptor[]>([]);
  const [formState, setFormState] = useState<FormSchemaState>(EMPTY_FORM_STATE);
  const [formValues, setFormValues] = useState<FormValues>({});
  const [infoOpen, setInfoOpen] = useState(false);
  const [result, setResult] = useState<unknown>(undefined);
  const [error, setError] = useState<string>('');

  const selected = useMemo(
    () => descriptors.find((descriptor) => descriptor.id === fid) || descriptors[0],
    [descriptors, fid],
  );

  const permissions = useMemo(
    () =>
      String((initialState as InitialStateWithAccess | undefined)?.currentUser?.access || '')
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean),
    [initialState],
  );

  useEffect(() => {
    const loadDescriptors = async () => {
      setLoading(true);
      try {
        const response = await listDescriptors();
        setDescriptors(Array.isArray(response) ? response : []);
      } catch (err: unknown) {
        message.error(extractErrorMessage(err, '加载函数列表失败'));
      } finally {
        setLoading(false);
      }
    };
    loadDescriptors();
  }, [message]);

  useEffect(() => {
    let active = true;

    const buildRuntimeFormSchema = () => {
      if (!selected?.id) {
        setFormState(EMPTY_FORM_STATE);
        return;
      }

      formRef.current = null;
      setFormValues({});
      setResult(undefined);
      setError('');
      setFormState({ status: 'loading' });

      try {
        const inputSchema = resolveInputSchema(selected);
        if (!inputSchema) {
          throw new Error('当前函数没有 inputSchema，无法生成调用测试表单');
        }
        if (!active) return;
        setFormState({
          status: 'ready',
          spec: buildFormPresentationSpec(inputSchema),
          detail: '由函数 inputSchema 临时生成，仅用于调用测试，不会保存为页面 UI。',
        });
      } catch (err: unknown) {
        if (!active) return;
        setFormState({
          status: 'error',
          error: extractErrorMessage(err, '调用测试表单生成失败'),
        });
      }
    };

    buildRuntimeFormSchema();
    return () => {
      active = false;
    };
  }, [selected]);

  const submitWithValues = useCallback(
    async (mode: 'invoke' | 'task') => {
      if (!selected?.id || formState.status !== 'ready') return;
      setExecuting(true);
      setError('');
      setResult(undefined);
      try {
        if (!formRef.current?.validate()) {
          throw new Error('表单校验失败，请修正参数后再执行');
        }
        const values = formRef.current?.getValues() || formValues;
        const payload = toJSONValue(values);
        const response =
          mode === 'invoke'
            ? await invokeFunction(selected.id, payload)
            : await startTask(selected.id, payload);
        setResult(response);
        message.success(mode === 'invoke' ? '执行成功' : '任务已创建');
      } catch (err: unknown) {
        const msg = extractErrorMessage(err, mode === 'invoke' ? '执行失败' : '创建任务失败');
        setError(msg);
        message.error(msg);
      } finally {
        setExecuting(false);
      }
    },
    [formValues, message, selected?.id, formState],
  );

  if (!loading && descriptors.length === 0) {
    return (
      <PageContainer title="游戏管理">
        <Empty description="暂无可用函数" />
      </PageContainer>
    );
  }

  if (!selected) {
    return (
      <PageContainer title="游戏管理">
        <Empty description="未找到函数，请从函数目录重新进入" />
      </PageContainer>
    );
  }

  return (
    <PageContainer
      title={resolveName(selected, locale)}
      subTitle={resolveSummary(selected, locale)}
      extra={[
        <Button key="info" icon={<InfoCircleOutlined />} onClick={() => setInfoOpen(true)}>
          函数信息
        </Button>,
        <Button key="catalog" onClick={() => history.push('/system/functions/catalog')}>
          函数目录
        </Button>,
      ]}
    >
      <Card
        title="参数配置"
        loading={formState.status === 'loading'}
        extra={
          <Space>
            <Button
              type="primary"
              loading={executing}
              disabled={formState.status !== 'ready'}
              onClick={() => submitWithValues('invoke')}
            >
              执行
            </Button>
            <Button
              loading={executing}
              disabled={formState.status !== 'ready'}
              onClick={() => submitWithValues('task')}
            >
              创建任务
            </Button>
          </Space>
        }
      >
        {formState.status === 'ready' && (
          <Alert
            style={{ marginBottom: 12 }}
            type="success"
            showIcon
            message="已根据函数契约生成调用测试表单"
            description={formState.detail}
          />
        )}
        {formState.status === 'error' && (
          <Alert
            type="error"
            showIcon
            message="调用测试表单无法生成"
            description={`${formState.error}。请先修正函数注册中的 inputSchema；业务页面展示请在 Page Studio 中处理。`}
          />
        )}
        {formState.status === 'ready' ? (
          <SchemaFormRenderer
            ref={formRef}
            spec={formState.spec}
            initialValues={formValues}
            onValuesChange={(_, allValues) => setFormValues(allValues)}
            hideSubmit
          />
        ) : formState.status === 'idle' ? (
          <Empty description="请选择函数" />
        ) : null}
      </Card>

      {(error || result !== undefined) && (
        <Card title="执行结果" style={{ marginTop: 16 }}>
          {error && <Alert type="error" showIcon message="执行失败" description={error} />}
          {result !== undefined && (
            <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {JSON.stringify(result, null, 2)}
            </pre>
          )}
        </Card>
      )}

      <Drawer
        title="函数信息"
        placement="right"
        width={420}
        open={infoOpen}
        onClose={() => setInfoOpen(false)}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Text code>{selected.id}</Text>
          {selected.resource && <Tag color="blue">Resource: {selected.resource}</Tag>}
          {selected.operation && <Tag color="purple">Operation: {selected.operation}</Tag>}
          {selected.version && <Tag>v{selected.version}</Tag>}
          {formState.status !== 'idle' && (
            <Tag color={formState.status === 'ready' ? 'green' : 'red'}>
              调用表单: {formState.status === 'ready' ? 'json-schema' : 'invalid'}
            </Tag>
          )}
          {permissions.length > 0 && <Tag>权限: {permissions.length}</Tag>}
          {resolveSummary(selected, locale) && (
            <Alert
              type="info"
              showIcon
              message="说明"
              description={resolveSummary(selected, locale)}
            />
          )}
        </Space>
      </Drawer>
    </PageContainer>
  );
}
