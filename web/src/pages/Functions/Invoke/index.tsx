/**
 * 函数调试页面 - 类似 Postman 的专业调试界面
 */

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Collapse,
  Divider,
  Empty,
  Input,
  Row,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import {
  ClockCircleOutlined,
  CopyOutlined,
  DeleteOutlined,
  HistoryOutlined,
  InfoCircleOutlined,
  PlayCircleOutlined,
  SendOutlined,
} from '@ant-design/icons';
import { history, useLocation, getLocale, useModel } from '@umijs/max';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import { invokeFunction, listDescriptors, type FunctionDescriptor } from '@/services/api';
import { extractErrorMessage } from '@/utils/errors';
import { parseInputSchema, type JSONSchemaType } from '@/utils/json';
import type { FormPresentationSpec, FormValues, JSONSchema, JSONValue } from '@/types/dashboard';

const { Text, Title } = Typography;
const { TextArea } = Input;

// Types
interface RequestHistory {
  id: string;
  functionId: string;
  timestamp: string;
  duration: number;
  status: 'success' | 'error';
  request: JSONValue;
  response?: JSONValue;
  error?: string;
}

type FormSchemaState =
  | { status: 'idle' | 'loading'; schema?: undefined; error?: undefined }
  | { status: 'ready'; spec: FormPresentationSpec; error?: undefined }
  | { status: 'error'; schema?: undefined; error: string };

const EMPTY_FORM_STATE: FormSchemaState = { status: 'idle' };

// Helpers
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

function buildFormPresentationSpec(schema: JSONSchemaType): FormPresentationSpec {
  return {
    jsonSchema: schema as JSONSchema,
    layout: 'vertical',
  };
}

function toJSONValue(values: FormValues): JSONValue {
  return JSON.parse(JSON.stringify(values)) as JSONValue;
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

// Main component
export default function FunctionInvokePage() {
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
  const [rawJson, setRawJson] = useState<string>('{}');
  const [inputMode, setInputMode] = useState<'form' | 'json'>('form');
  const [result, setResult] = useState<JSONValue | undefined>(undefined);
  const [resultRaw, setResultRaw] = useState<string>('');
  const [error, setError] = useState<string>('');
  const [duration, setDuration] = useState<number>(0);
  const [statusCode, setStatusCode] = useState<number>(0);
  const [requestHistory, setRequestHistory] = useState<RequestHistory[]>([]);
  const [showHistory, setShowHistory] = useState(false);

  const selected = useMemo(
    () => descriptors.find((d) => d.id === fid) || descriptors[0],
    [descriptors, fid],
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
    if (!selected?.id) {
      setFormState(EMPTY_FORM_STATE);
      return;
    }
    const inputSchema = resolveInputSchema(selected);
    if (!inputSchema) {
      setFormState({ status: 'error', error: '当前函数没有 inputSchema' });
      return;
    }
    setFormState({
      status: 'ready',
      spec: buildFormPresentationSpec(inputSchema),
    });
    setFormValues({});
    setRawJson('{}');
    setResult(undefined);
    setError('');
  }, [selected]);

  const executeRequest = useCallback(async () => {
    if (!selected?.id) return;
    setExecuting(true);
    setError('');
    setResult(undefined);
    setDuration(0);
    setStatusCode(0);

    const startTime = Date.now();
    let payload: JSONValue;

    try {
      if (inputMode === 'form') {
        if (!formRef.current?.validate()) {
          throw new Error('表单校验失败');
        }
        const values = formRef.current?.getValues() || formValues;
        payload = toJSONValue(values);
      } else {
        payload = JSON.parse(rawJson);
      }
    } catch (err) {
      setError(`参数解析失败: ${err instanceof Error ? err.message : String(err)}`);
      setExecuting(false);
      return;
    }

    try {
      const response = await invokeFunction(selected.id, payload);
      const elapsed = Date.now() - startTime;
      setDuration(elapsed);
      setStatusCode(200);
      setResult(response);
      setResultRaw(JSON.stringify(response, null, 2));
      message.success('执行成功');

      // Add to history
      setRequestHistory((prev) => [
        {
          id: `${Date.now()}`,
          functionId: selected.id,
          timestamp: new Date().toISOString(),
          duration: elapsed,
          status: 'success',
          request: payload,
          response,
        },
        ...prev.slice(0, 49),
      ]);
    } catch (err: unknown) {
      const elapsed = Date.now() - startTime;
      setDuration(elapsed);
      setStatusCode(500);
      const msg = extractErrorMessage(err, '执行失败');
      setError(msg);
      message.error(msg);

      // Add to history
      setRequestHistory((prev) => [
        {
          id: `${Date.now()}`,
          functionId: selected.id,
          timestamp: new Date().toISOString(),
          duration: elapsed,
          status: 'error',
          request: payload,
          error: msg,
        },
        ...prev.slice(0, 49),
      ]);
    } finally {
      setExecuting(false);
    }
  }, [selected?.id, inputMode, formValues, rawJson, message]);

  const handleFunctionChange = (functionId: string) => {
    history.push(`/system/functions/invoke?fid=${functionId}`);
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    message.success('已复制到剪贴板');
  };

  return (
    <PageContainer
      title="函数调试"
      subTitle="类似 Postman 的函数调试工具"
      extra={[
        <Button
          key="history"
          icon={<HistoryOutlined />}
          onClick={() => setShowHistory(!showHistory)}
        >
          历史记录
        </Button>,
        <Button key="catalog" onClick={() => history.push('/system/functions/catalog')}>
          函数目录
        </Button>,
      ]}
    >
      <Row gutter={16}>
        {/* Left panel - Request */}
        <Col span={showHistory ? 16 : 24}>
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            {/* Function selector */}
            <Card size="small">
              <Space.Compact style={{ width: '100%' }}>
                <Select
                  showSearch
                  placeholder="选择函数"
                  value={selected?.id}
                  onChange={handleFunctionChange}
                  style={{ width: '100%' }}
                  options={descriptors.map((d) => ({
                    label: `${d.id} - ${resolveName(d, locale)}`,
                    value: d.id,
                  }))}
                  loading={loading}
                />
              </Space.Compact>
              {selected && (
                <div style={{ marginTop: 8 }}>
                  <Space wrap>
                    {selected.resource && <Tag color="blue">{selected.resource}</Tag>}
                    {selected.operation && <Tag color="purple">{selected.operation}</Tag>}
                    {selected.tags?.map((tag) => (
                      <Tag key={tag}>{tag}</Tag>
                    ))}
                  </Space>
                  {resolveSummary(selected, locale) && (
                    <Text type="secondary" style={{ display: 'block', marginTop: 4 }}>
                      {resolveSummary(selected, locale)}
                    </Text>
                  )}
                </div>
              )}
            </Card>

            {/* Input tabs */}
            <Card
              size="small"
              title="请求参数"
              extra={
                <Space>
                  <Button
                    type="primary"
                    icon={<SendOutlined />}
                    loading={executing}
                    disabled={formState.status !== 'ready'}
                    onClick={executeRequest}
                  >
                    发送
                  </Button>
                </Space>
              }
            >
              <Tabs
                activeKey={inputMode}
                onChange={(key) => setInputMode(key as 'form' | 'json')}
                items={[
                  {
                    key: 'form',
                    label: '表单',
                    children:
                      formState.status === 'ready' ? (
                        <SchemaFormRenderer
                          ref={formRef}
                          spec={formState.spec}
                          initialValues={formValues}
                          onValuesChange={(_, allValues) => {
                            setFormValues(allValues);
                            setRawJson(JSON.stringify(allValues, null, 2));
                          }}
                          hideSubmit
                        />
                      ) : formState.status === 'error' ? (
                        <Alert type="error" message={formState.error} />
                      ) : (
                        <Empty description="请选择函数" />
                      ),
                  },
                  {
                    key: 'json',
                    label: 'JSON',
                    children: (
                      <TextArea
                        value={rawJson}
                        onChange={(e) => {
                          setRawJson(e.target.value);
                          try {
                            const parsed = JSON.parse(e.target.value);
                            setFormValues(parsed);
                          } catch {
                            // Ignore parse errors
                          }
                        }}
                        rows={15}
                        style={{ fontFamily: 'monospace' }}
                        placeholder='{"key": "value"}'
                      />
                    ),
                  },
                ]}
              />
            </Card>

            {/* Response */}
            {(result !== undefined || error) && (
              <Card
                size="small"
                title="响应"
                extra={
                  <Space>
                    {statusCode > 0 && (
                      <Tag color={statusCode < 400 ? 'green' : 'red'}>{statusCode}</Tag>
                    )}
                    {duration > 0 && (
                      <Tag icon={<ClockCircleOutlined />}>{formatDuration(duration)}</Tag>
                    )}
                    {resultRaw && (
                      <Tooltip title="复制响应">
                        <Button
                          size="small"
                          icon={<CopyOutlined />}
                          onClick={() => copyToClipboard(resultRaw)}
                        />
                      </Tooltip>
                    )}
                  </Space>
                }
              >
                <Tabs
                  defaultActiveKey="pretty"
                  items={[
                    {
                      key: 'pretty',
                      label: '格式化',
                      children: error ? (
                        <Alert type="error" showIcon message="执行失败" description={error} />
                      ) : (
                        <pre
                          style={{
                            margin: 0,
                            padding: 16,
                            background: '#f5f5f5',
                            borderRadius: 8,
                            maxHeight: 400,
                            overflow: 'auto',
                            fontSize: 13,
                            fontFamily: 'monospace',
                          }}
                        >
                          {resultRaw}
                        </pre>
                      ),
                    },
                    {
                      key: 'raw',
                      label: '原始数据',
                      children: (
                        <TextArea
                          value={resultRaw || error || ''}
                          readOnly
                          rows={10}
                          style={{ fontFamily: 'monospace' }}
                        />
                      ),
                    },
                  ]}
                />
              </Card>
            )}
          </Space>
        </Col>

        {/* Right panel - History */}
        {showHistory && (
          <Col span={8}>
            <Card
              size="small"
              title="请求历史"
              extra={
                <Button
                  size="small"
                  icon={<DeleteOutlined />}
                  onClick={() => setRequestHistory([])}
                >
                  清空
                </Button>
              }
              style={{ maxHeight: 'calc(100vh - 200px)', overflow: 'auto' }}
            >
              {requestHistory.length === 0 ? (
                <Empty description="暂无历史记录" />
              ) : (
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  {requestHistory.map((item) => (
                    <Card
                      key={item.id}
                      size="small"
                      hoverable
                      onClick={() => {
                        setFormValues(item.request as FormValues);
                        setRawJson(JSON.stringify(item.request, null, 2));
                        if (item.response) {
                          setResult(item.response);
                          setResultRaw(JSON.stringify(item.response, null, 2));
                        }
                        if (item.error) {
                          setError(item.error);
                        }
                      }}
                      style={{ cursor: 'pointer' }}
                    >
                      <Space>
                        <Tag color={item.status === 'success' ? 'green' : 'red'}>{item.status}</Tag>
                        <Text code>{item.functionId}</Text>
                        <Text type="secondary">{formatDuration(item.duration)}</Text>
                      </Space>
                      <Text
                        type="secondary"
                        style={{ display: 'block', fontSize: 12, marginTop: 4 }}
                      >
                        {new Date(item.timestamp).toLocaleString()}
                      </Text>
                    </Card>
                  ))}
                </Space>
              )}
            </Card>
          </Col>
        )}
      </Row>
    </PageContainer>
  );
}
