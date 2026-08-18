import { useEffect, useState } from 'react';
import { Alert, Button, Card, Empty, Space, Tabs, Tag, Tooltip, Typography, Input } from 'antd';
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

const { Text } = Typography;
const { TextArea } = Input;

interface InvocationResponseProps {
  responseRaw: string;
  error: string;
  duration: number;
  /** OTel trace id returned by the invoke API, empty when unavailable. */
  traceId?: string;
  onCopy: (value: string) => void;
}

export default function InvocationResponse({
  responseRaw,
  error,
  duration,
  traceId,
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
        <Alert type="error" showIcon message="调用失败" description={error} />
      ) : hasResponse ? (
        <Tabs
          items={[
            {
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
            },
            {
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
            },
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
