import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Alert, App, Button, Card, Col, Empty, Row, Space, Tag, Typography } from 'antd';
import { BarChartOutlined, LinkOutlined, ReloadOutlined, RocketOutlined } from '@ant-design/icons';
import { fetchOpsConfig, type OpsConfig } from '@/services/api/ops';
import GameSelector from '@/components/GameSelector';

const { Text, Paragraph } = Typography;

function normalizeBaseUrl(url?: string) {
  return (url || '').trim().replace(/\/+$/, '');
}

type EntryCard = {
  key: string;
  title: string;
  description: string;
  url?: string;
};

export default function TelemetryPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [config, setConfig] = useState<OpsConfig>({});
  const [gameId, setGameId] = useState<string | undefined>(
    localStorage.getItem('game_id') || undefined,
  );
  const [env, setEnv] = useState<string | undefined>(localStorage.getItem('env') || undefined);

  const entries = useMemo<EntryCard[]>(
    () => [
      {
        key: 'grafana',
        title: 'Grafana Explore',
        description: '查看指标趋势、日志与聚合观测面板。',
        url: normalizeBaseUrl(config.grafanaExploreUrl),
      },
      {
        key: 'jaeger',
        title: 'Jaeger',
        description: '查看链路追踪与 trace 详情。',
        url: normalizeBaseUrl(config.jaegerUrl),
      },
      {
        key: 'alerts',
        title: 'Alertmanager',
        description: '查看告警分发与静默策略。',
        url: normalizeBaseUrl(config.alertmanagerUrl),
      },
    ],
    [config],
  );

  const configuredCount = entries.filter((entry) => !!entry.url).length;

  const loadConfig = async () => {
    setLoading(true);
    try {
      const res = await fetchOpsConfig();
      setConfig(res || {});
    } catch (error: any) {
      message.error(error?.message || '加载观测配置失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadConfig().catch(() => {});
  }, []);

  useEffect(() => {
    const onStorage = () => {
      setGameId(localStorage.getItem('game_id') || undefined);
      setEnv(localStorage.getItem('env') || undefined);
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  return (
    <PageContainer
      title="观测中心"
      subTitle="统一跳转到外部遥测与追踪系统"
      extra={<GameSelector />}
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Alert
          type="warning"
          showIcon
          message="已移除前端随机监控数据"
          description="当前版本不再在页面内伪造 DAU、收入、崩溃率、Trace 列表等数据。此页只作为观测系统入口，真实指标请接入 Grafana / Jaeger / Alertmanager。"
        />

        <Card
          extra={
            <Button icon={<ReloadOutlined />} loading={loading} onClick={loadConfig}>
              刷新配置
            </Button>
          }
        >
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <div>
              <Text>
                当前范围:
                <Tag color="blue" style={{ marginInlineStart: 8 }}>
                  {gameId || '未选择游戏'}
                </Tag>
                <Tag>{env || '未选择环境'}</Tag>
              </Text>
            </div>

            <Paragraph type="secondary" style={{ marginBottom: 0 }}>
              本页读取 `/api/v1/ops/config` 中的观测平台地址。若要启用入口，请在服务端配置
              `grafana_explore_url`、`jaeger_url`、`alertmanager_url`。
            </Paragraph>

            {configuredCount === 0 ? <Empty description="尚未配置任何观测平台地址" /> : null}
          </Space>
        </Card>

        <Row gutter={[16, 16]}>
          {entries.map((entry) => (
            <Col xs={24} md={12} xl={8} key={entry.key}>
              <Card
                title={
                  <Space>
                    <BarChartOutlined />
                    <span>{entry.title}</span>
                  </Space>
                }
                extra={
                  entry.url ? (
                    <Button
                      type="link"
                      icon={<LinkOutlined />}
                      onClick={() => window.open(entry.url, '_blank', 'noopener,noreferrer')}
                    >
                      打开
                    </Button>
                  ) : null
                }
              >
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <Text>{entry.description}</Text>
                  {entry.url ? (
                    <>
                      <Tag color="success" icon={<RocketOutlined />}>
                        已配置
                      </Tag>
                      <Text code style={{ wordBreak: 'break-all' }}>
                        {entry.url}
                      </Text>
                    </>
                  ) : (
                    <Alert
                      type="info"
                      showIcon
                      message="未配置"
                      description="服务端尚未提供该观测平台地址。"
                    />
                  )}
                </Space>
              </Card>
            </Col>
          ))}
        </Row>
      </Space>
    </PageContainer>
  );
}
