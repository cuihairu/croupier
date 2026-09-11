import React, { useEffect, useMemo, useState } from 'react';
import { Alert, Button, Card, Empty, Input, Space, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { LinkOutlined, ReloadOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import { fetchOpsConfig, type OpsConfig } from '@/services/api/ops';

const { Text } = Typography;

function normalizeBaseUrl(url?: string) {
  return (url || '').trim().replace(/\/+$/, '');
}

export default function TracesPage() {
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState<OpsConfig>({});
  const [traceId, setTraceId] = useState(() => {
    // 支持从调用页等入口带 traceId 直接跳转
    const initial = new URLSearchParams(window.location.search).get('traceId');
    return initial ? initial.trim() : '';
  });

  const jaegerTraceUrl = useMemo(() => {
    const base = normalizeBaseUrl(config.jaegerUrl);
    const id = traceId.trim();
    if (!base || !id) return '';
    return `${base}/trace/${encodeURIComponent(id)}`;
  }, [config.jaegerUrl, traceId]);

  const loadConfig = async () => {
    setLoading(true);
    try {
      const res = await fetchOpsConfig();
      setConfig(res || {});
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadConfig().catch(() => {});
  }, []);

  const openGrafanaExplore = () => {
    if (!config.grafanaExploreUrl) return;
    window.open(config.grafanaExploreUrl, '_blank', 'noopener,noreferrer');
  };

  const openJaegerTrace = () => {
    if (!jaegerTraceUrl) return;
    window.open(jaegerTraceUrl, '_blank', 'noopener,noreferrer');
  };

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({
          id: 'pages.telemetry.traces.title',
          defaultMessage: '链路追踪',
        })}
        extra={
          <Button icon={<ReloadOutlined />} onClick={loadConfig} loading={loading}>
            <FormattedMessage
              id="pages.telemetry.traces.action.refreshConfig"
              defaultMessage="刷新配置"
            />
          </Button>
        }
      >
        <Alert
          type="info"
          showIcon
          message={intl.formatMessage({
            id: 'pages.telemetry.traces.alert.message',
            defaultMessage: '当前版本不提供 Trace 列表/详情查询',
          })}
          description={
            <div>
              <div>
                <FormattedMessage
                  id="pages.telemetry.traces.alert.setupHint"
                  defaultMessage="请配置外部追踪系统并从这里跳转："
                />
              </div>
              <div style={{ marginTop: 4 }}>
                <Text code>CROUPIER_GRAFANA_EXPLORE_URL</Text> /{' '}
                <Text code>CROUPIER_JAEGER_URL</Text>
              </div>
            </div>
          }
          style={{ marginBottom: 16 }}
        />

        <Space orientation="vertical" style={{ width: '100%' }} size={16}>
          <Space wrap>
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.telemetry.traces.search.traceId',
                defaultMessage: '输入 Trace ID（用于跳转）',
              })}
              value={traceId}
              onChange={(e) => setTraceId(e.target.value)}
              style={{ width: 380 }}
            />
            <Button
              icon={<LinkOutlined />}
              onClick={openGrafanaExplore}
              disabled={!config.grafanaExploreUrl}
            >
              <FormattedMessage
                id="pages.telemetry.traces.action.openGrafana"
                defaultMessage="打开 Grafana Explore"
              />
            </Button>
            <Button icon={<LinkOutlined />} onClick={openJaegerTrace} disabled={!jaegerTraceUrl}>
              <FormattedMessage
                id="pages.telemetry.traces.action.openJaeger"
                defaultMessage="在 Jaeger 中打开"
              />
            </Button>
          </Space>

          <Empty
            description={intl.formatMessage({
              id: 'pages.telemetry.traces.empty.hint',
              defaultMessage: '请从 Task/调用日志中获取 trace_id，然后跳转到 Grafana/Jaeger 查看',
            })}
          />
        </Space>
      </Card>
    </PageContainer>
  );
}
