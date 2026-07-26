import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { Form as FormilyForm } from '@formily/core';
import { PageContainer } from '@ant-design/pro-components';
import { Alert, App, Button, Card, Drawer, Empty, Space, Tag, Typography } from 'antd';
import { InfoCircleOutlined } from '@ant-design/icons';
import { history, useLocation, getLocale, useModel } from '@umijs/max';
import SchemaRenderer from '@/components/formily/SchemaRenderer';
import type { FormilySchema } from '@/components/formily/schema/types';
import {
  fetchFunctionUiSchema,
  invokeFunction,
  listDescriptors,
  startTask,
  type FunctionDescriptor,
} from '@/services/api';
import { fetchOptions } from '@/services/schema/async';
import { validateFormilySchema } from '@/services/schema/validateSchema';
import { extractErrorMessage } from '@/utils/errors';
import type { FormilyValues } from '@/components/formily/schema/types';
import type { JSONValue } from '@/types/dashboard';

const { Text } = Typography;

type SchemaSource =
  | 'custom_metadata'
  | 'config_file_override'
  | 'generated_default'
  | 'none'
  | string;

type UISchemaState =
  | {
      status: 'idle' | 'loading';
      schema?: undefined;
      source?: undefined;
      detail?: undefined;
      error?: undefined;
    }
  | {
      status: 'ready';
      schema: FormilySchema;
      source: SchemaSource;
      detail?: string;
      error?: undefined;
    }
  | {
      status: 'error';
      schema?: undefined;
      source?: SchemaSource;
      detail?: string;
      error: string;
    };

const EMPTY_UI_STATE: UISchemaState = { status: 'idle' };

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

function validateRuntimeUISchema(schema: unknown): FormilySchema {
  const validation = validateFormilySchema(schema);
  if (!validation.ok) {
    throw new Error(validation.error || '函数 UI Schema 不是有效 Formily Schema');
  }
  return schema as FormilySchema;
}

function getRuntimeContext(functionId: string, access?: string) {
  const permissions = String(access || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
  return {
    gameId:
      typeof window !== 'undefined' ? localStorage.getItem('game_id') || undefined : undefined,
    env: typeof window !== 'undefined' ? localStorage.getItem('env') || undefined : undefined,
    functionId,
    permissions,
  };
}

function toJSONValue(values: FormilyValues): JSONValue {
  return JSON.parse(JSON.stringify(values)) as JSONValue;
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
  const formRef = useRef<FormilyForm | null>(null);
  const searchParams = new URLSearchParams(location.search);
  const fid = searchParams.get('fid') || searchParams.get('id') || '';

  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [descriptors, setDescriptors] = useState<FunctionDescriptor[]>([]);
  const [uiState, setUIState] = useState<UISchemaState>(EMPTY_UI_STATE);
  const [formValues, setFormValues] = useState<FormilyValues>({});
  const [infoOpen, setInfoOpen] = useState(false);
  const [result, setResult] = useState<unknown>(undefined);
  const [error, setError] = useState<string>('');

  const selected = useMemo(
    () => descriptors.find((descriptor) => descriptor.id === fid) || descriptors[0],
    [descriptors, fid],
  );

  const runtimeContext = useMemo(
    () =>
      getRuntimeContext(
        selected?.id || '',
        (initialState as InitialStateWithAccess | undefined)?.currentUser?.access,
      ),
    [initialState, selected?.id],
  );

  const rendererScope = useMemo(
    () => ({
      hasPerm: (perm: string) => runtimeContext.permissions.includes(perm),
      fetchOptions,
    }),
    [runtimeContext.permissions],
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

    const loadUISchema = async () => {
      if (!selected?.id) {
        setUIState(EMPTY_UI_STATE);
        return;
      }

      formRef.current = null;
      setFormValues({});
      setResult(undefined);
      setError('');
      setUIState({ status: 'loading' });

      try {
        const response = await fetchFunctionUiSchema(selected.id);
        if (!active) return;
        const schema = validateRuntimeUISchema(response?.schema);
        setUIState({
          status: 'ready',
          schema,
          source: response?.uiSource || 'none',
          detail: response?.uiSourceDetail,
        });
      } catch (err: unknown) {
        if (!active) return;
        setUIState({
          status: 'error',
          error: extractErrorMessage(err, '函数 UI Schema 加载失败'),
        });
      }
    };

    loadUISchema();
    return () => {
      active = false;
    };
  }, [selected?.id]);

  const submitWithValues = useCallback(
    async (mode: 'invoke' | 'task') => {
      if (!selected?.id || uiState.status !== 'ready') return;
      setExecuting(true);
      setError('');
      setResult(undefined);
      try {
        await formRef.current?.validate();
        const values = (formRef.current?.values || formValues) as FormilyValues;
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
    [formValues, message, selected?.id, uiState],
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
        loading={uiState.status === 'loading'}
        extra={
          <Space>
            <Button
              type="primary"
              loading={executing}
              disabled={uiState.status !== 'ready'}
              onClick={() => submitWithValues('invoke')}
            >
              执行
            </Button>
            <Button
              loading={executing}
              disabled={uiState.status !== 'ready'}
              onClick={() => submitWithValues('task')}
            >
              创建任务
            </Button>
          </Space>
        }
      >
        {uiState.status === 'ready' && (
          <Alert
            style={{ marginBottom: 12 }}
            type="success"
            showIcon
            message="已加载 Formily UI Schema"
            description={uiState.detail || `来源：${uiState.source}`}
          />
        )}
        {uiState.status === 'error' && (
          <Alert
            type="error"
            showIcon
            message="函数 UI Schema 无法渲染"
            description={`${uiState.error}。当前函数调用页只接受 Formily Schema，请在注册描述或函数表单设计器中修正 schema。`}
          />
        )}
        {uiState.status === 'ready' ? (
          <SchemaRenderer
            schema={uiState.schema}
            value={formValues}
            onChange={setFormValues}
            onFormReady={(form) => {
              formRef.current = form;
            }}
            context={runtimeContext}
            scope={rendererScope}
          />
        ) : uiState.status === 'idle' ? (
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
          {uiState.status !== 'idle' && (
            <Tag color={uiState.status === 'ready' ? 'green' : 'red'}>
              UI: {uiState.status === 'ready' ? uiState.source : 'invalid'}
            </Tag>
          )}
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
