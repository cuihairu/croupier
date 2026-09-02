import { useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Empty,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  Input,
} from 'antd';
import {
  ClockCircleOutlined,
  CopyOutlined,
  LinkOutlined,
  NodeIndexOutlined,
} from '@ant-design/icons';
import { CodeEditor } from '@/components/MonacoDynamic';
import { fetchOpsConfig, type OpsConfig } from '@/services/api/ops';
import { history } from '@umijs/max';
import { formatDuration } from './types';
import { deriveResultSpec, isArrayOfObjects } from '@/utils/resultSpec';
import { renderJSONValueSummary } from '@/components/PageRenderer/ResultViewRenderer';
import type { ApiErrorDetail } from '@/utils/errors';
import { localizedText } from '@/utils/localizedText';
import type { JSONSchema, JSONValue } from '@/types/dashboard';

const { Text } = Typography;
const { TextArea } = Input;

interface InvocationResponseProps {
  responseRaw: string;
  error: string;
  /** F10：结构化错误明细（后端 error 契约 details） */
  errorDetails?: ApiErrorDetail[];
  duration: number;
  /** OTel trace id returned by the invoke API, empty when unavailable. */
  traceId?: string;
  /** F10：函数 outputSchema，可推导结构化结果视图 */
  outputSchema?: JSONSchema | null;
  /** F10：解析后的调用结果（未提供时仅展示 JSON 兜底） */
  response?: JSONValue;
  onCopy: (value: string) => void;
}

export default function InvocationResponse({
  responseRaw,
  error,
  errorDetails,
  duration,
  traceId,
  outputSchema,
  response,
  onCopy,
}: InvocationResponseProps) {
  const [opsConfig, setOpsConfig] = useState<OpsConfig>({});

  useEffect(() => {
    // 仅在有 traceId 时加载一次，用于生成 Jaeger 跳转链接
    if (!traceId) return;
    fetchOpsConfig()
      .then((config) => setOpsConfig(config || {}))
      .catch(() => {});
  }, [traceId]);

  const hasResponse = responseRaw || error;
  const jaegerUrl =
    traceId && opsConfig.jaegerUrl
      ? `${opsConfig.jaegerUrl.replace(/\/+$/, '')}/trace/${encodeURIComponent(traceId)}`
      : '';

  const traceTag = traceId ? (
    <Tooltip
      title={
        jaegerUrl ? (
          <span>
            OTel Trace ID，点击在 Jaeger 中查看完整链路
            <br />
            <Text code style={{ color: '#fff' }}>
              {traceId}
            </Text>
          </span>
        ) : (
          <span>
            OTel Trace ID（未配置 Jaeger 地址，复制后可在 链路追踪 页面查询）
            <br />
            <Text code style={{ color: '#fff' }}>
              {traceId}
            </Text>
          </span>
        )
      }
    >
      <Tag
        color="geekblue"
        icon={<NodeIndexOutlined />}
        style={{ cursor: jaegerUrl ? 'pointer' : 'copy', maxWidth: 260 }}
        onClick={() =>
          jaegerUrl ? window.open(jaegerUrl, '_blank', 'noopener,noreferrer') : onCopy(traceId)
        }
      >
        <span style={{ verticalAlign: 'middle' }}>{traceId.slice(0, 16)}…</span>
      </Tag>
    </Tooltip>
  ) : null;

  // F10：结构化视图（outputSchema 可推导时）
  const derived = useMemo(() => deriveResultSpec(outputSchema), [outputSchema]);
  const structuredBody = useMemo(() => {
    if (!derived || response === undefined || response === null || error) return null;
    if (
      derived.shape === 'object' &&
      typeof response === 'object' &&
      !Array.isArray(response) &&
      response !== null
    ) {
      return (
        <Descriptions column={1} bordered size="small">
          {derived.spec.fields!.map((field) => (
            <Descriptions.Item
              key={field.key}
              label={localizedText(field.title, 'zh-CN', field.key)}
            >
              {renderJSONValueSummary((response as Record<string, JSONValue>)[field.key])}
            </Descriptions.Item>
          ))}
        </Descriptions>
      );
    }
    if (derived.shape === 'arrayOfObjects' && isArrayOfObjects(response)) {
      return (
        <Table
          size="small"
          rowKey={(_, index) => String(index)}
          pagination={{ pageSize: 10, hideOnSinglePage: true }}
          columns={derived.spec.fields!.map((field) => ({
            title: localizedText(field.title, 'zh-CN', field.key),
            dataIndex: field.key,
            key: field.key,
            render: (value: JSONValue) => renderJSONValueSummary(value),
          }))}
          dataSource={response as Record<string, JSONValue>[]}
        />
      );
    }
    // 数据形态与 schema 不匹配时回退 JSON
    return null;
  }, [derived, response, error]);

  const errorDetailList = errorDetails?.length ? (
    <ul style={{ margin: '8px 0 0', paddingLeft: 18 }}>
      {errorDetails.map((detail, index) => (
        <li key={`${detail.field}-${index}`}>
          {detail.field ? <Text code>{detail.field}</Text> : null}
          {detail.field ? '：' : ''}
          {detail.message}
        </li>
      ))}
    </ul>
  ) : null;

  const jsonTab = {
    key: 'pretty',
    label: '格式化',
    children: (
      <CodeEditor
        value={responseRaw || 'null'}
        language="json"
        theme="vs-dark"
        readOnly
        height={320}
        options={{ lineNumbers: 'on', folding: true, scrollBeyondLastLine: false }}
      />
    ),
  };
  const rawTab = {
    key: 'raw',
    label: '原始数据',
    children: (
      <TextArea
        readOnly
        value={responseRaw}
        rows={12}
        style={{ fontFamily: 'var(--ant-font-family-code, monospace)' }}
      />
    ),
  };

  return (
    <Card
      size="small"
      title="响应"
      extra={
        hasResponse ? (
          <Space>
            {error ? <Tag color="red">失败</Tag> : <Tag color="green">成功</Tag>}
            {duration ? <Tag icon={<ClockCircleOutlined />}>{formatDuration(duration)}</Tag> : null}
            {traceTag}
            <Tooltip title="复制响应">
              <Button
                size="small"
                icon={<CopyOutlined />}
                onClick={() => onCopy(responseRaw || error)}
              />
            </Tooltip>
          </Space>
        ) : null
      }
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          message="调用失败"
          description={
            <>
              {error}
              {errorDetailList}
            </>
          }
        />
      ) : hasResponse ? (
        <Tabs
          items={[
            // F10：结构化优先展示；不可结构化时仅 JSON 两档
            ...(structuredBody
              ? [
                  {
                    key: 'structured',
                    label: '结构化',
                    children: structuredBody,
                  },
                ]
              : []),
            jsonTab,
            rawTab,
          ]}
        />
      ) : (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="发送请求后，响应结果将显示在这里"
          style={{ padding: '40px 0' }}
        />
      )}
      {traceId ? (
        <div style={{ marginTop: 8 }}>
          <Space size={4}>
            <Text type="secondary">链路追踪：</Text>
            <Text code copyable={{ text: traceId }}>
              {traceId}
            </Text>
            <Button
              size="small"
              type="link"
              icon={<LinkOutlined />}
              onClick={() =>
                history.push(`/ops/telemetry/traces?traceId=${encodeURIComponent(traceId)}`)
              }
            >
              链路追踪页
            </Button>
          </Space>
        </div>
      ) : null}
    </Card>
  );
}
