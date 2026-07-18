import React, { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Col, Divider, Row, Space, Tag, Typography } from 'antd';
import { CodeEditor } from '@/components/MonacoDynamic';
import SchemaRenderer from '@/components/formily/SchemaRenderer';
import type { FormilySchema } from '@/components/formily/schema/types';
import { validateFormilySchema } from '@/services/schema/validateSchema';

const { Text } = Typography;

interface UISchemaEditorProps {
  value?: FormilySchema;
  onChange?: (value: FormilySchema) => void;
  jsonSchema?: unknown;
}

type ParseResult =
  | { ok: true; schema: FormilySchema }
  | { ok: false; error: string; line?: number; column?: number };

const EMPTY_FORMILY_SCHEMA: FormilySchema = {
  type: 'object',
  properties: {},
};

function stringifySchema(schema?: FormilySchema) {
  return JSON.stringify(schema || EMPTY_FORMILY_SCHEMA, null, 2);
}

function parseJsonErrorLocation(error: Error, source: string) {
  const match = error.message.match(/position\s+(\d+)/i);
  if (!match) return {};
  const offset = Number(match[1]);
  if (!Number.isFinite(offset)) return {};
  const prefix = source.slice(0, Math.max(0, Math.min(offset, source.length)));
  const lines = prefix.split('\n');
  return {
    line: lines.length,
    column: lines[lines.length - 1].length + 1,
  };
}

function parseFormilySchema(raw: string, allowEmpty = true): ParseResult {
  try {
    const parsed = JSON.parse(raw) as unknown;
    const validation = validateFormilySchema(parsed, { allowEmpty });
    if (!validation.ok) {
      return { ok: false, error: validation.error || 'Formily Schema 校验失败' };
    }
    return { ok: true, schema: parsed as FormilySchema };
  } catch (error) {
    const err = error as Error;
    return {
      ok: false,
      error: err.message || 'JSON 解析失败',
      ...parseJsonErrorLocation(err, raw),
    };
  }
}

function countFields(schema?: FormilySchema): number {
  const properties = schema?.properties;
  if (!properties || typeof properties !== 'object' || Array.isArray(properties)) {
    return 0;
  }
  return Object.keys(properties).length;
}

export default function UISchemaEditor({ value, onChange, jsonSchema }: UISchemaEditorProps) {
  const [draft, setDraft] = useState(() => stringifySchema(value));
  const [previewValue, setPreviewValue] = useState<Record<string, unknown>>({});
  const [parseError, setParseError] = useState<ParseResult & { ok: false }>();
  const [previewSchema, setPreviewSchema] = useState<FormilySchema>(value || EMPTY_FORMILY_SCHEMA);

  useEffect(() => {
    const next = value || EMPTY_FORMILY_SCHEMA;
    setDraft(stringifySchema(next));
    setPreviewSchema(next);
    setParseError(undefined);
    setPreviewValue({});
  }, [value]);

  const sourceFieldCount = useMemo(() => {
    const source = jsonSchema as { properties?: unknown } | undefined;
    const properties = source?.properties;
    if (!properties || typeof properties !== 'object' || Array.isArray(properties)) {
      return 0;
    }
    return Object.keys(properties).length;
  }, [jsonSchema]);

  const applyDraft = () => {
    const result = parseFormilySchema(draft);
    if (!result.ok) {
      setParseError(result);
      return;
    }
    setParseError(undefined);
    setPreviewSchema(result.schema);
    onChange?.(result.schema);
  };

  const formatDraft = () => {
    const result = parseFormilySchema(draft);
    if (!result.ok) {
      setParseError(result);
      return;
    }
    const formatted = stringifySchema(result.schema);
    setDraft(formatted);
    setPreviewSchema(result.schema);
    setParseError(undefined);
    onChange?.(result.schema);
  };

  const resetDraft = () => {
    const next = value || EMPTY_FORMILY_SCHEMA;
    setDraft(stringifySchema(next));
    setPreviewSchema(next);
    setPreviewValue({});
    setParseError(undefined);
  };

  return (
    <Card
      title="Formily Schema 编辑器"
      extra={
        <Space>
          <Button size="small" onClick={resetDraft}>
            重置
          </Button>
          <Button size="small" onClick={formatDraft}>
            格式化
          </Button>
          <Button size="small" type="primary" onClick={applyDraft}>
            应用
          </Button>
        </Space>
      }
    >
      <Alert
        type="info"
        showIcon
        message="函数 UI 只接受 Formily Schema"
        description="请直接编辑 x-component、x-decorator、x-component-props、x-reactions 等 Formily 字段。旧 fields/widget 格式会被拒绝。"
        style={{ marginBottom: 16 }}
      />

      <Row gutter={16}>
        <Col xs={24} lg={13}>
          <CodeEditor
            value={draft}
            language="json"
            height={520}
            onChange={(next) => setDraft(next || '')}
            options={{
              automaticLayout: true,
              fontSize: 13,
              tabSize: 2,
              formatOnPaste: true,
              formatOnType: true,
              scrollBeyondLastLine: false,
            }}
          />
          {parseError && (
            <Alert
              type="error"
              showIcon
              message="Schema 校验失败"
              description={
                parseError.line
                  ? `${parseError.error}（第 ${parseError.line} 行）`
                  : parseError.error
              }
              style={{ marginTop: 12 }}
            />
          )}
          <Divider style={{ margin: '12px 0' }} />
          <Space wrap>
            <Tag>Formily 字段: {countFields(previewSchema)}</Tag>
            {sourceFieldCount > 0 && <Tag color="blue">注册契约字段: {sourceFieldCount}</Tag>}
          </Space>
        </Col>

        <Col xs={24} lg={11}>
          <Card
            size="small"
            title="实时预览"
            extra={
              <Button size="small" onClick={() => setPreviewValue({})}>
                清空数据
              </Button>
            }
          >
            {countFields(previewSchema) === 0 ? (
              <Alert
                type="warning"
                showIcon
                message="Schema 为空"
                description="空 Formily Schema 只能作为草稿，发布或执行前必须至少包含一个 Formily 字段。"
              />
            ) : (
              <SchemaRenderer
                schema={previewSchema}
                value={previewValue}
                onChange={(next) => setPreviewValue(next as Record<string, unknown>)}
              />
            )}
          </Card>
          <Card size="small" title="当前预览数据" style={{ marginTop: 12 }}>
            <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {JSON.stringify(previewValue, null, 2)}
            </pre>
          </Card>
          <Text type="secondary">
            JSON Schema 只作为注册契约参考，不在编辑器内转换为第二套 UI 格式。
          </Text>
        </Col>
      </Row>
    </Card>
  );
}
