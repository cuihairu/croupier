import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Row,
  Space,
  Alert,
  Typography,
  Switch,
  Tag,
  Divider,
  Descriptions,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { history, useParams } from '@umijs/max';
import { CodeEditor } from '@/components/MonacoDynamic';
import SchemaRenderer from '@/components/formily/SchemaRenderer';
import {
  fetchUnifiedFormilySchemaState,
  saveDraft,
  clearDraft,
  saveFormilySchema,
} from '@/services/schema';
import { validateFormilySchema } from '@/services/schema/validateSchema';
import { trackSchemaEvent } from '@/services/schema/telemetry';
import { getFunctionDetail } from '@/services/api/functions';
import { generateFormilyFromJsonSchema } from '@/services/schema/generateFormilyFromJsonSchema';
import { parseInputSchema } from '@/utils/json';

const { Text } = Typography;

const DEFAULT_SCHEMA = { type: 'object', properties: {} };
const SCHEMA_TEMPLATES: Array<{ key: string; label: string; schema: any }> = [
  { key: 'empty', label: '空对象', schema: DEFAULT_SCHEMA },
  {
    key: 'basicForm',
    label: '基础表单',
    schema: {
      type: 'object',
      properties: {
        username: { type: 'string', title: '用户名', 'x-component': 'Input' },
        age: { type: 'number', title: '年龄', 'x-component': 'NumberPicker' },
        enabled: { type: 'boolean', title: '是否启用', 'x-component': 'Switch' },
      },
      required: ['username'],
    },
  },
  {
    key: 'withGroup',
    label: '分组布局',
    schema: {
      type: 'object',
      properties: {
        basic: {
          type: 'void',
          'x-component': 'FormGrid',
          properties: {
            appId: { type: 'string', title: 'App ID', 'x-component': 'Input' },
            env: {
              type: 'string',
              title: '环境',
              enum: ['dev', 'staging', 'prod'],
              'x-component': 'Select',
            },
          },
        },
      },
    },
  },
];

type ParsedResult = { parsed?: any; error?: string };

function parseAndValidate(raw: string): ParsedResult {
  try {
    const parsed = JSON.parse(raw);
    const validation = validateFormilySchema(parsed);
    if (!validation.ok) {
      return { error: validation.error };
    }
    return { parsed };
  } catch (err: any) {
    return { error: err?.message || 'JSON 解析失败' };
  }
}

function countTopLevelFields(schema: any): number {
  const props = schema?.properties;
  if (!props || typeof props !== 'object') return 0;
  return Object.keys(props).length;
}

export default function SchemaDesigner() {
  const { message } = App.useApp();
  const params = useParams<{ id: string }>();
  const functionId = params.id || '';
  const [raw, setRaw] = useState<string>('{}');
  const [schema, setSchema] = useState<any>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [parseError, setParseError] = useState<string | undefined>(undefined);
  const [dirty, setDirty] = useState(false);
  const [autoApply, setAutoApply] = useState(true);
  const [previewData, setPreviewData] = useState<Record<string, any>>({});
  const [draftUpdatedAt, setDraftUpdatedAt] = useState<string | undefined>(undefined);
  const [publishedUpdatedAt, setPublishedUpdatedAt] = useState<string | undefined>(undefined);
  const [draftConflict, setDraftConflict] = useState(false);
  const [generatedFromParams, setGeneratedFromParams] = useState(false);

  const applyRaw = useCallback(
    (nextRaw?: string) => {
      const target = typeof nextRaw === 'string' ? nextRaw : raw;
      const result = parseAndValidate(target);
      if (result.error) {
        setParseError(result.error);
        return;
      }
      if (result.parsed) {
        setSchema(result.parsed);
        setParseError(undefined);
      }
    },
    [raw],
  );

  const formatRaw = useCallback(() => {
    const result = parseAndValidate(raw);
    if (result.error) {
      setParseError(result.error);
      return;
    }
    if (result.parsed) {
      setRaw(JSON.stringify(result.parsed, null, 2));
      setSchema(result.parsed);
      setParseError(undefined);
      setDirty(true);
      message.success('格式化完成');
    }
  }, [message, raw]);

  const replaceWithTemplate = useCallback(
    (templateSchema: any, label: string) => {
      const next = JSON.stringify(templateSchema, null, 2);
      setRaw(next);
      setSchema(templateSchema);
      setParseError(undefined);
      setGeneratedFromParams(false);
      setDirty(true);
      setPreviewData({});
      message.success(`已应用模板：${label}`);
    },
    [message],
  );

  const validateOnly = useCallback(() => {
    const result = parseAndValidate(raw);
    if (result.error) {
      setParseError(result.error);
      message.error(result.error);
      return;
    }
    setParseError(undefined);
    message.success('Schema 校验通过');
  }, [message, raw]);

  useEffect(() => {
    if (!autoApply || !dirty) return;
    const timer = window.setTimeout(() => {
      applyRaw();
    }, 300);
    return () => window.clearTimeout(timer);
  }, [applyRaw, autoApply, dirty, raw]);

  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (!dirty) return;
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [dirty]);

  const load = useCallback(async () => {
    if (!functionId) return;
    setLoading(true);
    try {
      const state = await fetchUnifiedFormilySchemaState(functionId);
      let initial = state.schema as any;
      let generated = false;

      if (!initial || Object.keys(initial || {}).length === 0) {
        const detail = await getFunctionDetail(functionId);
        const inputSchema = parseInputSchema(detail?.inputSchema, detail?.params);
        initial = generateFormilyFromJsonSchema(inputSchema);
        generated = true;
      }
      if (!initial) {
        initial = DEFAULT_SCHEMA;
      }

      const text = JSON.stringify(initial, null, 2);
      setRaw(text);
      setSchema(initial);
      setPreviewData({});
      setDraftUpdatedAt(state.draftUpdatedAt);
      setPublishedUpdatedAt(state.publishedUpdatedAt);
      setDraftConflict(state.draftConflict);
      setGeneratedFromParams(generated);
      setParseError(undefined);
      setDirty(false);
      trackSchemaEvent('schema_load', {
        functionId,
        source: generated ? 'generated' : state.source,
      });
    } catch (e: any) {
      message.error(e?.message || '加载 schema 失败');
      trackSchemaEvent('schema_load_error', { functionId, error: e?.message || String(e) });
    } finally {
      setLoading(false);
    }
  }, [functionId, message]);

  useEffect(() => {
    load();
  }, [load]);

  const title = useMemo(() => `函数 UI 设计器：${functionId || '-'}`, [functionId]);
  const topLevelFields = useMemo(() => countTopLevelFields(schema), [schema]);
  const requiredCount = useMemo(
    () => (Array.isArray(schema?.required) ? schema.required.length : 0),
    [schema],
  );

  return (
    <PageContainer
      title={title}
      extra={[
        <Button
          key="back"
          onClick={() =>
            history.push(`/system/functions/${encodeURIComponent(functionId)}?tab=config&subTab=ui`)
          }
        >
          返回预览
        </Button>,
        <Button
          key="draft"
          onClick={() => {
            const result = parseAndValidate(raw);
            if (result.error) {
              message.error(result.error);
              return;
            }
            saveDraft(functionId, result.parsed, { baseUpdatedAt: publishedUpdatedAt });
            setDraftUpdatedAt(new Date().toISOString());
            message.success('草稿已保存');
            trackSchemaEvent('schema_draft_save', { functionId });
          }}
          disabled={!dirty}
        >
          保存草稿
        </Button>,
        <Button
          key="clear-draft"
          onClick={() => {
            clearDraft(functionId);
            setDraftUpdatedAt(undefined);
            setDraftConflict(false);
            message.success('草稿已清除');
            trackSchemaEvent('schema_draft_clear', { functionId });
          }}
        >
          清除草稿
        </Button>,
        <Button
          key="publish"
          type="primary"
          loading={saving}
          onClick={async () => {
            const result = parseAndValidate(raw);
            if (result.error) {
              message.error(result.error);
              setParseError(result.error);
              return;
            }
            try {
              setSaving(true);
              await saveFormilySchema(functionId, result.parsed);
              setDirty(false);
              message.success('已发布');
              trackSchemaEvent('schema_publish', { functionId });
            } catch (e: any) {
              message.error(e?.message || '发布失败');
              trackSchemaEvent('schema_publish_error', {
                functionId,
                error: e?.message || String(e),
              });
            } finally {
              setSaving(false);
            }
          }}
          disabled={!dirty}
        >
          发布
        </Button>,
      ]}
    >
      <Alert
        style={{ marginBottom: 16 }}
        type="warning"
        showIcon
        message="这里是函数表单设计器，不是业务页面编辑器"
        description="当前页面只负责单个函数的 Formily Schema。若目标是注册函数后搭完整操作页面，请返回并进入页面工作台；Schema 历史、回滚及部分来源追踪仍依赖后端占位实现。"
      />
      <Row gutter={16}>
        <Col span={13}>
          <Card
            title="Formily JSON Schema"
            loading={loading}
            extra={
              <Space size="middle">
                <Space size={4}>
                  <Text type="secondary">自动预览</Text>
                  <Switch checked={autoApply} onChange={setAutoApply} size="small" />
                </Space>
                <Button onClick={() => applyRaw()}>应用预览</Button>
                <Button onClick={validateOnly}>校验</Button>
                <Button onClick={formatRaw}>格式化 JSON</Button>
                <Button onClick={load}>重载</Button>
              </Space>
            }
          >
            <Space wrap style={{ marginBottom: 12 }}>
              {SCHEMA_TEMPLATES.map((item) => (
                <Button
                  key={item.key}
                  size="small"
                  onClick={() => replaceWithTemplate(item.schema, item.label)}
                >
                  模板：{item.label}
                </Button>
              ))}
            </Space>
            <CodeEditor
              value={raw}
              language="json"
              height={560}
              onChange={(v) => {
                setRaw(v || '');
                setDirty(true);
              }}
            />
            <Divider style={{ margin: '12px 0' }} />
            <Space wrap>
              <Tag color={dirty ? 'orange' : 'green'}>
                {dirty ? '有未发布变更' : '与已发布一致'}
              </Tag>
              <Tag>顶层字段: {topLevelFields}</Tag>
              <Tag>必填字段: {requiredCount}</Tag>
              {generatedFromParams && <Tag color="gold">自动生成初稿</Tag>}
              {draftUpdatedAt && (
                <Tag color="blue">草稿: {new Date(draftUpdatedAt).toLocaleString('zh-CN')}</Tag>
              )}
            </Space>
            {generatedFromParams && (
              <Alert
                style={{ marginTop: 12 }}
                type="info"
                showIcon
                message="已根据函数参数自动生成 UI 初稿"
                description="当前函数还没有已发布 UI Schema；这份初稿可直接编辑并发布。"
              />
            )}
            {draftConflict && (
              <Alert
                style={{ marginTop: 12 }}
                type="warning"
                showIcon
                message="草稿可能过期"
                description="检测到线上配置已更新，当前草稿基于旧版本生成。请发布前仔细校验差异。"
              />
            )}
            {parseError && (
              <Alert
                style={{ marginTop: 12 }}
                type="error"
                showIcon
                message="Schema 校验失败"
                description={parseError}
              />
            )}
            {!parseError && dirty && !autoApply && (
              <Text type="secondary">提示：修改后点击“应用预览”查看效果。</Text>
            )}
          </Card>
        </Col>
        <Col span={11}>
          <Card
            title="预览"
            loading={loading}
            extra={
              <Button size="small" onClick={() => setPreviewData({})}>
                清空预览数据
              </Button>
            }
          >
            <SchemaRenderer
              schema={schema}
              readOnly={false}
              value={previewData}
              onChange={(next) => setPreviewData((next || {}) as Record<string, any>)}
            />
            <Divider />
            <Descriptions size="small" column={1} bordered>
              <Descriptions.Item label="Schema 类型">
                {String(schema?.type || '-')}
              </Descriptions.Item>
              <Descriptions.Item label="顶层字段数">{topLevelFields}</Descriptions.Item>
              <Descriptions.Item label="必填字段数">{requiredCount}</Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
      </Row>
    </PageContainer>
  );
}
