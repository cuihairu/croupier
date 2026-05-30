import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import {
  Alert,
  App,
  Button,
  Card,
  Drawer,
  Empty,
  Input,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { InfoCircleOutlined } from '@ant-design/icons';
import { history, useLocation, getLocale } from '@umijs/max';
import {
  FunctionFormRenderer,
  type FormUISchema,
  type JSONSchema,
} from '@/components/FunctionFormRenderer';
import {
  fetchFunctionUiSchema,
  getFunctionOpenAPI,
  invokeFunction,
  listDescriptors,
  saveFunctionUiSchema,
  startJob,
  type FunctionDescriptor,
} from '@/services/api';
import { buildUISchemaFromJSONSchema, parseInputSchema } from '@/utils/json';

const { Text } = Typography;

type OpenAPIOperation = {
  requestBody?: any;
  parameters?: any[];
  extensions?: Record<string, any>;
  ['x-ui']?: any;
};

const isObject = (v: any): v is Record<string, any> =>
  !!v && typeof v === 'object' && !Array.isArray(v);

const isFormUISchema = (v: any): v is FormUISchema => isObject(v);
const isLegacyUISchema = (v: any): v is FormUISchema =>
  isObject(v) &&
  (Object.prototype.hasOwnProperty.call(v, 'fields') ||
    Object.prototype.hasOwnProperty.call(v, 'ui:layout') ||
    Object.prototype.hasOwnProperty.call(v, 'ui:groups') ||
    Object.prototype.hasOwnProperty.call(v, 'ui:order'));
const isJSONSchemaLike = (v: any): v is JSONSchema =>
  isObject(v) &&
  (v.type === 'object' || Object.prototype.hasOwnProperty.call(v, 'properties')) &&
  !Object.prototype.hasOwnProperty.call(v, 'x-component') &&
  !Object.prototype.hasOwnProperty.call(v, 'x-decorator') &&
  !Object.prototype.hasOwnProperty.call(v, 'fields');
const isFormilySchema = (v: any): boolean =>
  isObject(v) &&
  (Object.prototype.hasOwnProperty.call(v, 'properties') ||
    Object.prototype.hasOwnProperty.call(v, 'x-component') ||
    Object.prototype.hasOwnProperty.call(v, 'x-decorator'));
const hasFormilyComponent = (v: any): boolean => {
  if (!isObject(v)) return false;
  if (typeof v['x-component'] === 'string' && v['x-component']) return true;
  if (isObject(v.properties)) {
    return Object.values(v.properties).some((child) => hasFormilyComponent(child));
  }
  if (isObject(v.items)) return hasFormilyComponent(v.items);
  return false;
};
const mapFormilyComponentToType = (component?: string): JSONSchema['type'] => {
  const key = String(component || '').toLowerCase();
  if (key.includes('number') || key.includes('slider') || key.includes('rate')) return 'number';
  if (key.includes('switch') || key.includes('checkbox')) return 'boolean';
  if (key.includes('array')) return 'array';
  if (key.includes('select')) return 'string';
  if (key.includes('date') || key.includes('time')) return 'string';
  if (key.includes('input') || key.includes('textarea')) return 'string';
  return 'string';
};
const buildJSONSchemaFromFormily = (raw: any): JSONSchema | undefined => {
  if (!isObject(raw)) return undefined;
  const propertiesNode = isObject(raw.properties) ? raw.properties : undefined;
  if (!propertiesNode) return undefined;
  const properties: Record<string, any> = {};
  const required: string[] = Array.isArray(raw.required) ? raw.required.filter(Boolean) : [];
  Object.entries(propertiesNode).forEach(([key, node]) => {
    if (!isObject(node)) return;
    const propType =
      typeof node.type === 'string' && node.type
        ? node.type
        : mapFormilyComponentToType(String(node['x-component'] || ''));
    properties[key] = {
      type: propType,
      title: node.title || key,
      description: node.description,
      enum: Array.isArray(node.enum) ? node.enum : undefined,
      items: isObject(node.items) ? node.items : undefined,
      properties: isObject(node.properties) ? node.properties : undefined,
    };
  });
  return { type: 'object', properties, required };
};

const getRequestSchemaFromOpenAPI = (operation?: OpenAPIOperation): JSONSchema | undefined => {
  const bodySchema = operation?.requestBody?.content?.['application/json']?.schema;
  if (isObject(bodySchema)) return bodySchema as JSONSchema;
  const parameters = Array.isArray(operation?.parameters) ? operation?.parameters : [];
  if (parameters.length === 0) return undefined;
  const properties: Record<string, any> = {};
  const required: string[] = [];
  parameters.forEach((p: any) => {
    if (!isObject(p)) return;
    const name = String(p.name || '').trim();
    if (!name) return;
    const schema = isObject(p.schema) ? p.schema : {};
    properties[name] = {
      type: schema.type || 'string',
      title: p.title || p.description || name,
      description: p.description,
      enum: Array.isArray(schema.enum) ? schema.enum : undefined,
      default: schema.default,
    };
    if (p.required || p.in === 'path') required.push(name);
  });
  if (Object.keys(properties).length === 0) return undefined;
  return { type: 'object', properties, required };
};

const getUISchemaFromOpenAPI = (operation?: OpenAPIOperation): FormUISchema | undefined => {
  const ui = operation?.['x-ui'] || operation?.extensions?.['x-ui'];
  return isFormUISchema(ui) ? ui : undefined;
};

const buildSchemaFromUISchema = (ui?: FormUISchema): JSONSchema | undefined => {
  const fields = ui?.fields || {};
  const keys = Object.keys(fields);
  if (!keys.length) return undefined;
  const properties: Record<string, any> = {};
  keys.forEach((key) => {
    const f: any = fields[key] || {};
    const widget = String(f.widget || '').toLowerCase();
    let type: any = 'string';
    if (widget === 'switch' || widget === 'checkbox') type = 'boolean';
    if (widget === 'number' || widget === 'slider' || widget === 'rate') type = 'number';
    if (widget === 'multiselect' || widget === 'checkboxgroup') type = 'array';
    if (widget === 'json' || widget === 'object') type = 'object';
    properties[key] = {
      type,
      title: f.label || key,
      description: f.description,
    };
  });
  return { type: 'object', properties };
};

const normalizeCustomUISchema = (
  raw: any,
): {
  kind: 'none' | 'legacy' | 'json-schema' | 'formily' | 'invalid';
  legacy?: FormUISchema;
  jsonSchema?: JSONSchema;
  formily?: any;
} => {
  if (raw == null) return { kind: 'none' };
  let parsed: any = raw;
  if (typeof raw === 'string') {
    const text = raw.trim();
    if (!text) return { kind: 'none' };
    try {
      parsed = JSON.parse(text);
    } catch {
      return { kind: 'invalid' };
    }
  }
  if (!isObject(parsed)) return { kind: 'invalid' };
  const keys = Object.keys(parsed);
  if (keys.length === 0) return { kind: 'none' };
  if (isLegacyUISchema(parsed)) return { kind: 'legacy', legacy: parsed as FormUISchema };
  if (isJSONSchemaLike(parsed)) return { kind: 'json-schema', jsonSchema: parsed as JSONSchema };
  if (isFormilySchema(parsed) && hasFormilyComponent(parsed))
    return { kind: 'formily', formily: parsed };
  if (isFormilySchema(parsed) && !hasFormilyComponent(parsed)) {
    return { kind: 'json-schema', jsonSchema: parsed as JSONSchema };
  }
  return { kind: 'legacy', legacy: parsed as FormUISchema };
};

const resolveName = (d: FunctionDescriptor, locale: string) => {
  const zh = d.displayName?.zh || d.summary?.zh;
  const en = d.displayName?.en || d.summary?.en;
  if (locale.toLowerCase().startsWith('zh')) return zh || en || d.id;
  return en || zh || d.id;
};

const inferTokenFromID = (id: string) =>
  String(id || '')
    .split(/[./_-]+/)
    .map((x) => x.trim().toLowerCase())
    .filter(Boolean);

const buildGeneratedSchema = (d?: FunctionDescriptor): JSONSchema => {
  const tokens = inferTokenFromID(d?.id || '');
  const knownOps = new Set([
    'create',
    'add',
    'new',
    'read',
    'get',
    'list',
    'query',
    'search',
    'detail',
    'update',
    'edit',
    'patch',
    'modify',
    'delete',
    'remove',
  ]);
  let operation = String((d as any)?.operation || '')
    .trim()
    .toLowerCase();
  if (!operation) {
    for (let i = tokens.length - 1; i >= 0; i -= 1) {
      if (knownOps.has(tokens[i])) {
        operation = tokens[i];
        break;
      }
    }
  }
  const entity =
    String((d as any)?.entity || '')
      .trim()
      .toLowerCase() ||
    tokens.find((t) => !knownOps.has(t)) ||
    'object';
  const idKey = `${entity}_id`;
  const objectLabel = (d?.displayName?.zh || d?.displayName?.en || entity || '对象').toString();

  const schema: JSONSchema = { type: 'object', properties: {}, required: [] };
  const props: Record<string, any> = schema.properties || {};
  const required = new Set<string>();
  if (operation === 'create' || operation === 'add' || operation === 'new') {
    props[idKey] = { type: 'string', title: `${objectLabel}ID（可选）` };
    props.data = { type: 'object', title: `${objectLabel}数据` };
    required.add('data');
  } else if (
    operation === 'read' ||
    operation === 'get' ||
    operation === 'query' ||
    operation === 'search' ||
    operation === 'detail'
  ) {
    props[idKey] = { type: 'string', title: `${objectLabel}ID` };
    required.add(idKey);
  } else if (operation === 'update' || operation === 'edit' || operation === 'patch') {
    props[idKey] = { type: 'string', title: `${objectLabel}ID` };
    props.patch = { type: 'object', title: '更新内容' };
    required.add(idKey);
    required.add('patch');
  } else if (operation === 'delete' || operation === 'remove') {
    props[idKey] = { type: 'string', title: `${objectLabel}ID` };
    required.add(idKey);
  } else {
    props.payload = { type: 'object', title: '请求参数' };
  }
  schema.required = Array.from(required);
  return schema;
};
type ManualField = {
  name: string;
  type: 'string' | 'number' | 'integer' | 'boolean' | 'array' | 'object';
  required: boolean;
  description?: string;
};
const suggestFirstParamName = (fid?: string) => {
  const text = String(fid || '').toLowerCase();
  if (text.includes('player')) return 'player_id';
  if (text.includes('user')) return 'user_id';
  return 'id';
};
const widgetByType: Record<ManualField['type'], string> = {
  string: 'input',
  number: 'number',
  integer: 'number',
  boolean: 'switch',
  array: 'multiselect',
  object: 'json',
};

const extractErrorMessage = (e: any, fallback: string) => {
  const candidates = [
    e?.data?.message,
    e?.info?.data?.message,
    e?.response?.data?.message,
    e?.response?.data?.error,
    e?.message,
  ].filter((x) => typeof x === 'string' && x.trim());
  return (candidates[0] as string) || fallback;
};

export default function FunctionRuntimeUIPage() {
  const { message } = App.useApp();
  const location = useLocation();
  const locale = getLocale();
  const searchParams = new URLSearchParams(location.search);
  const fid = searchParams.get('fid') || searchParams.get('id') || '';

  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [descriptors, setDescriptors] = useState<FunctionDescriptor[]>([]);
  const [openapiOperation, setOpenapiOperation] = useState<OpenAPIOperation | undefined>(undefined);
  const [uiSchema, setUISchema] = useState<FormUISchema | undefined>(undefined);
  const [customUISchema, setCustomUISchema] = useState<FormUISchema | undefined>(undefined);
  const [customJSONSchema, setCustomJSONSchema] = useState<JSONSchema | undefined>(undefined);
  const [uiSource, setUISource] = useState<'custom' | 'openapi' | 'generated'>('generated');
  const [uiResolving, setUIResolving] = useState(false);
  const [invalidCustomUI, setInvalidCustomUI] = useState(false);
  const [infoOpen, setInfoOpen] = useState(false);
  const [reloadToken, setReloadToken] = useState(0);
  const [manualFields, setManualFields] = useState<ManualField[]>([]);
  const [newFieldName, setNewFieldName] = useState('');
  const [newFieldType, setNewFieldType] = useState<ManualField['type']>('string');
  const [newFieldRequired, setNewFieldRequired] = useState(true);
  const [newFieldDescription, setNewFieldDescription] = useState('');
  const [result, setResult] = useState<any>(undefined);
  const [error, setError] = useState<string>('');

  const selected = useMemo(
    () => descriptors.find((d) => d.id === fid) || descriptors[0],
    [descriptors, fid],
  );

  const schemaMeta = useMemo<{
    schema: JSONSchema;
    source: 'input' | 'openapi' | 'custom-ui' | 'generated';
  }>(() => {
    if (!selected) return { schema: { type: 'object', properties: {} }, source: 'generated' };
    const parsed = parseInputSchema(selected.inputSchema, selected.params);
    if (parsed && parsed.type === 'object' && Object.keys(parsed.properties || {}).length > 0) {
      return { schema: parsed as JSONSchema, source: 'input' };
    }
    const fromOpenAPI = getRequestSchemaFromOpenAPI(openapiOperation);
    if (fromOpenAPI && Object.keys(fromOpenAPI.properties || {}).length > 0) {
      return { schema: fromOpenAPI, source: 'openapi' };
    }
    const fromCustomUI = buildSchemaFromUISchema(customUISchema);
    if (fromCustomUI && Object.keys(fromCustomUI.properties || {}).length > 0) {
      return { schema: fromCustomUI, source: 'custom-ui' };
    }
    if (customJSONSchema && Object.keys(customJSONSchema.properties || {}).length > 0) {
      return { schema: customJSONSchema, source: 'custom-ui' };
    }
    return { schema: buildGeneratedSchema(selected), source: 'generated' };
  }, [selected, openapiOperation, customUISchema, customJSONSchema]);
  const schema = schemaMeta.schema;
  const hasStructuredSchema = useMemo(() => {
    const props = schema?.properties || {};
    return Object.keys(props).length > 0;
  }, [schema]);
  const blockInvokeForIncompleteSchema = useMemo(
    () => !uiResolving && schemaMeta.source === 'generated',
    [uiResolving, schemaMeta.source],
  );
  const sourceNotice = useMemo(() => {
    if (uiResolving) return null;
    if (invalidCustomUI) {
      return {
        type: 'warning' as const,
        message: '检测到无效 UI 配置，已自动回退',
        description: '当前函数的 UI 配置为空或格式错误，系统使用可用 Schema 自动渲染。',
      };
    }
    if (uiSource === 'custom') {
      return {
        type: 'success' as const,
        message: '已加载你保存的 UI 配置',
        description: '当前页面优先使用自定义 UI，并据此渲染表单。',
      };
    }
    if (!hasStructuredSchema || schemaMeta.source === 'generated') {
      return {
        type: 'warning' as const,
        message: '使用默认参数模板',
        description: '该函数未提供可用 Schema，已按对象/操作自动生成默认参数。',
      };
    }
    return null;
  }, [uiResolving, invalidCustomUI, uiSource, hasStructuredSchema, schemaMeta.source]);

  useEffect(() => {
    const loadDescriptors = async () => {
      setLoading(true);
      try {
        const res = await listDescriptors();
        setDescriptors(Array.isArray(res) ? res : []);
      } catch (e: any) {
        message.error(e?.message || '加载函数列表失败');
      } finally {
        setLoading(false);
      }
    };
    loadDescriptors();
  }, [message]);

  useEffect(() => {
    let active = true;
    const loadUI = async () => {
      if (!selected?.id) return;
      setUIResolving(true);
      setInvalidCustomUI(false);
      setUISchema(undefined);
      setCustomUISchema(undefined);
      setCustomJSONSchema(undefined);
      setOpenapiOperation(undefined);
      setUISource('generated');
      setManualFields([]);
      setNewFieldName(suggestFirstParamName(selected.id));
      setNewFieldType('string');
      setNewFieldRequired(true);
      setNewFieldDescription('');
      try {
        const baseSchema = (() => {
          const parsed = parseInputSchema(selected.inputSchema, selected.params);
          if (parsed && parsed.type === 'object') return parsed as JSONSchema;
          return buildGeneratedSchema(selected);
        })();
        // 1) Prefer user-saved custom UI schema
        try {
          const customUI = await fetchFunctionUiSchema(selected.id);
          const normalized = normalizeCustomUISchema(customUI?.schema);
          if (normalized.kind === 'invalid' && active) {
            setInvalidCustomUI(true);
          }
          if (active && normalized.kind === 'legacy' && normalized.legacy) {
            const parsedCustom = normalized.legacy;
            setUISchema(parsedCustom);
            setCustomUISchema(parsedCustom);
            setUISource('custom');
            return;
          }
          if (active && normalized.kind === 'json-schema' && normalized.jsonSchema) {
            setCustomJSONSchema(normalized.jsonSchema);
            setUISchema(buildUISchemaFromJSONSchema(normalized.jsonSchema) as FormUISchema);
            setUISource('custom');
            return;
          }
          if (active && normalized.kind === 'formily' && normalized.formily) {
            const converted = buildJSONSchemaFromFormily(normalized.formily);
            if (converted && Object.keys(converted.properties || {}).length > 0) {
              setCustomJSONSchema(converted);
              setUISchema(buildUISchemaFromJSONSchema(converted) as FormUISchema);
              setUISource('custom');
              return;
            }
            setInvalidCustomUI(true);
          }
        } catch {
          // ignore and fallback
        }

        // 2) Fallback to OpenAPI x-ui / request schema generated UI
        const operation = (await getFunctionOpenAPI(selected.id)) as OpenAPIOperation;
        if (!active) return;
        setOpenapiOperation(operation);
        const openapiSchema = getRequestSchemaFromOpenAPI(operation);
        const openapiUI = getUISchemaFromOpenAPI(operation);
        if (openapiUI) {
          setUISchema(openapiUI);
          setUISource('openapi');
          return;
        }
        setUISchema(buildUISchemaFromJSONSchema(openapiSchema || baseSchema) as FormUISchema);
        setUISource(openapiSchema ? 'openapi' : 'generated');
      } catch {
        if (!active) return;
        setOpenapiOperation(undefined);
        const parsed = parseInputSchema(selected.inputSchema, selected.params);
        const fallbackSchema =
          parsed && parsed.type === 'object'
            ? (parsed as JSONSchema)
            : buildGeneratedSchema(selected);
        setUISchema(buildUISchemaFromJSONSchema(fallbackSchema) as FormUISchema);
        setUISource('generated');
      } finally {
        if (active) setUIResolving(false);
      }
    };
    loadUI();
    return () => {
      active = false;
    };
  }, [selected?.id, reloadToken]);
  useEffect(() => {
    if (!selected?.id) return;
    setNewFieldName(suggestFirstParamName(selected.id));
  }, [selected?.id]);
  const buildManualUISchema = (fields: ManualField[]): FormUISchema => {
    const outFields: Record<string, any> = {};
    const order: string[] = [];
    fields.forEach((f) => {
      outFields[f.name] = {
        label: f.name,
        description: f.description,
        widget: widgetByType[f.type] || 'input',
      };
      order.push(f.name);
    });
    return {
      fields: outFields,
      'ui:order': order,
      'ui:layout': { type: 'grid', cols: 2 },
    };
  };

  const addManualField = () => {
    const name = String(newFieldName || '').trim();
    if (!name) {
      message.warning('请先填写参数名');
      return;
    }
    if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(name)) {
      message.warning('参数名需为字母/数字/下划线，且不能数字开头');
      return;
    }
    if (manualFields.some((f) => f.name === name)) {
      message.warning('参数名重复');
      return;
    }
    setManualFields((prev) => [
      ...prev,
      {
        name,
        type: newFieldType,
        required: newFieldRequired,
        description: newFieldDescription.trim() || undefined,
      },
    ]);
    setNewFieldName('');
    setNewFieldType('string');
    setNewFieldRequired(true);
    setNewFieldDescription('');
  };

  const saveManualSchema = async () => {
    if (!selected?.id) return;
    if (manualFields.length === 0) {
      message.warning('请至少添加一个参数');
      return;
    }
    const ui = buildManualUISchema(manualFields);
    try {
      await saveFunctionUiSchema(selected.id, { schema: ui });
      message.success('已保存参数模型，现可调用');
      setReloadToken((x) => x + 1);
      window.dispatchEvent(new Event('function-route:changed'));
    } catch (e: any) {
      message.error(extractErrorMessage(e, '保存参数模型失败'));
    }
  };

  const handleInvoke = async (values: any) => {
    if (!selected?.id) return;
    setExecuting(true);
    setError('');
    setResult(undefined);
    try {
      const res = await invokeFunction(selected.id, values);
      setResult(res);
      message.success('执行成功');
    } catch (e: any) {
      const msg = extractErrorMessage(e, '执行失败');
      setError(msg);
      message.error(msg);
    } finally {
      setExecuting(false);
    }
  };

  const handleStartJob = async (values: any) => {
    if (!selected?.id) return;
    setExecuting(true);
    setError('');
    setResult(undefined);
    try {
      const res = await startJob(selected.id, values);
      setResult(res);
      message.success('任务已创建');
    } catch (e: any) {
      const msg = extractErrorMessage(e, '创建任务失败');
      setError(msg);
      message.error(msg);
    } finally {
      setExecuting(false);
    }
  };

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
      subTitle={selected.summary?.zh || selected.summary?.en || selected.id}
      extra={[
        <Button key="info" icon={<InfoCircleOutlined />} onClick={() => setInfoOpen(true)}>
          函数信息
        </Button>,
        <Button key="catalog" onClick={() => history.push('/system/functions/catalog')}>
          函数目录
        </Button>,
      ]}
    >
      <Card title="参数配置">
        {sourceNotice && (
          <Alert
            style={{ marginBottom: 12 }}
            type={sourceNotice.type}
            showIcon
            message={sourceNotice.message}
            description={sourceNotice.description}
          />
        )}
        {blockInvokeForIncompleteSchema ? (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Alert
              type="warning"
              showIcon
              message="该函数缺少可用 Schema，暂不允许调用"
              description="请先补全参数定义（第一个参数名/类型），保存后再调用。"
            />
            <Card size="small" title="参数补全向导">
              <Space direction="vertical" style={{ width: '100%' }}>
                <Space wrap>
                  <Input
                    style={{ width: 220 }}
                    placeholder="第一个参数名（如 player_id）"
                    value={newFieldName}
                    onChange={(e) => setNewFieldName(e.target.value)}
                  />
                  <Select
                    style={{ width: 160 }}
                    value={newFieldType}
                    onChange={(v) => setNewFieldType(v)}
                    options={[
                      { label: 'string', value: 'string' },
                      { label: 'number', value: 'number' },
                      { label: 'integer', value: 'integer' },
                      { label: 'boolean', value: 'boolean' },
                      { label: 'array', value: 'array' },
                      { label: 'object', value: 'object' },
                    ]}
                  />
                  <Space>
                    <span>必填</span>
                    <Switch checked={newFieldRequired} onChange={setNewFieldRequired} />
                  </Space>
                </Space>
                <Input
                  placeholder="参数说明（可选）"
                  value={newFieldDescription}
                  onChange={(e) => setNewFieldDescription(e.target.value)}
                />
                <Space>
                  <Button onClick={addManualField}>添加参数</Button>
                  <Button type="primary" onClick={saveManualSchema}>
                    保存并启用调用
                  </Button>
                </Space>
                {manualFields.length > 0 && (
                  <Space direction="vertical" size="small">
                    {manualFields.map((f) => (
                      <Tag
                        key={f.name}
                        closable
                        onClose={(e) => {
                          e.preventDefault();
                          setManualFields((prev) => prev.filter((x) => x.name !== f.name));
                        }}
                      >
                        {`${f.name}: ${f.type}${f.required ? ' (required)' : ''}`}
                      </Tag>
                    ))}
                  </Space>
                )}
              </Space>
            </Card>
          </Space>
        ) : (
          <FunctionFormRenderer
            schema={schema}
            uiSchema={uiSchema}
            onSubmit={handleInvoke}
            onSecondarySubmit={handleStartJob}
            loading={executing}
            secondaryLoading={executing}
            submitText="执行"
            secondarySubmitText="创建任务"
            resetText="重置"
            enableRawJson
          />
        )}
      </Card>
      {(error || result) && (
        <Card title="执行结果" style={{ marginTop: 16 }}>
          {error && <Alert type="error" showIcon message="执行失败" description={error} />}
          {result && (
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
          {selected.category && <Tag color="blue">{selected.category}</Tag>}
          {selected.version && <Tag>v{selected.version}</Tag>}
          {(selected.summary?.zh || selected.summary?.en) && (
            <Alert
              type="info"
              showIcon
              message="说明"
              description={selected.summary?.zh || selected.summary?.en}
            />
          )}
        </Space>
      </Drawer>
    </PageContainer>
  );
}
