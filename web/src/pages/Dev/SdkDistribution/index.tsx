import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import {
  Button,
  Card,
  Empty,
  Input,
  Space,
  Statistic,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import {
  fetchSdkStats,
  type SdkInstanceItem,
  type SdkLanguageStats,
  type SdkStatsResponse,
} from '@/services/api/sdkStats';

const { Text } = Typography;

const LANGUAGE_COLORS: Record<string, string> = {
  go: 'blue',
  python: 'gold',
  java: 'volcano',
  js: 'orange',
  ts: 'orange',
  cpp: 'purple',
  csharp: 'green',
  'c#': 'green',
  node: 'orange',
  custom: 'default',
  unknown: 'default',
};

function languageColor(language: string): string {
  return LANGUAGE_COLORS[language.toLowerCase()] ?? 'geekblue';
}

/** 单语言版本分布卡片 */
function LanguageCard({ stats }: { stats: SdkLanguageStats }) {
  const maxCount = Math.max(...stats.versions.map((item) => item.count), 1);
  return (
    <Card
      size="small"
      title={
        <Space>
          <Tag color={languageColor(stats.language)}>{stats.language}</Tag>
          <Text type="secondary">{`${stats.count} 实例`}</Text>
        </Space>
      }
      style={{ height: '100%' }}
    >
      <Space direction="vertical" size={6} style={{ width: '100%' }}>
        {stats.versions.map((version) => (
          <div key={version.version} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <Text code style={{ minWidth: 90 }}>
              {version.version}
            </Text>
            <div
              style={{
                flex: 1,
                height: 8,
                borderRadius: 4,
                background: 'rgba(0,0,0,0.06)',
                overflow: 'hidden',
              }}
            >
              <div
                style={{
                  width: `${Math.round((version.count / maxCount) * 100)}%`,
                  height: '100%',
                  background: 'linear-gradient(90deg, #1677ff88, #1677ff)',
                }}
              />
            </div>
            <Text type="secondary" style={{ minWidth: 24, textAlign: 'right' }}>
              {version.count}
            </Text>
          </div>
        ))}
      </Space>
    </Card>
  );
}

export default function SdkDistributionPage() {
  const [loading, setLoading] = useState(false);
  const [stats, setStats] = useState<SdkStatsResponse | null>(null);
  const [keyword, setKeyword] = useState('');

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      setStats(await fetchSdkStats());
    } catch {
      // 错误提示交给全局拦截器；保留旧数据
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const timer = setInterval(refresh, 30000);
    return () => clearInterval(timer);
  }, [refresh]);

  const filteredInstances = useMemo(() => {
    const items = stats?.instances ?? [];
    const keywordTrimmed = keyword.trim().toLowerCase();
    if (!keywordTrimmed) return items;
    return items.filter((item: SdkInstanceItem) =>
      [
        item.providerId,
        item.agentId,
        item.gameId,
        item.env,
        item.sdkLanguage,
        item.sdkVersion,
        item.sdkName,
      ]
        .filter(Boolean)
        .some((field) => String(field).toLowerCase().includes(keywordTrimmed)),
    );
  }, [stats, keyword]);

  const agentCount = useMemo(
    () => new Set((stats?.instances ?? []).map((item) => item.agentId)).size,
    [stats],
  );

  const columns = [
    { title: 'Provider', dataIndex: 'providerId', key: 'providerId', copyable: true },
    { title: 'Agent', dataIndex: 'agentId', key: 'agentId' },
    {
      title: '语言',
      dataIndex: 'sdkLanguage',
      key: 'sdkLanguage',
      render: (value: string) => <Tag color={languageColor(value)}>{value}</Tag>,
    },
    { title: 'SDK 版本', dataIndex: 'sdkVersion', key: 'sdkVersion' },
    {
      title: 'SDK 名称',
      dataIndex: 'sdkName',
      key: 'sdkName',
      render: (value: string) => value || '-',
    },
    { title: 'Game', dataIndex: 'gameId', key: 'gameId' },
    { title: 'Env', dataIndex: 'env', key: 'env' },
    {
      title: '最后活跃',
      dataIndex: 'lastSeenUnix',
      key: 'lastSeenUnix',
      render: (value: number) =>
        value ? (
          <Tooltip title={new Date(value * 1000).toLocaleString()}>
            <Text type="secondary">{new Date(value * 1000).toLocaleTimeString()}</Text>
          </Tooltip>
        ) : (
          '-'
        ),
    },
  ];

  return (
    <PageContainer
      title="SDK 版本分布"
      subTitle="当前在线 provider 实例的 SDK 语言与版本聚合（30s 自动刷新）"
      extra={[
        <Button key="refresh" icon={<ReloadOutlined />} loading={loading} onClick={refresh}>
          刷新
        </Button>,
      ]}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card size="small">
          <Space size={48} wrap>
            <Statistic title="在线实例" value={stats?.totalInstances ?? 0} />
            <Statistic title="SDK 语言" value={stats?.languages?.length ?? 0} />
            <Statistic title="活跃 Agent" value={agentCount} />
          </Space>
        </Card>

        {stats?.languages?.length ? (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
              gap: 16,
            }}
          >
            {stats.languages.map((language) => (
              <LanguageCard key={language.language} stats={language} />
            ))}
          </div>
        ) : (
          !loading && (
            <Card size="small">
              <Empty description="当前没有在线的 provider 实例" style={{ padding: '24px 0' }} />
            </Card>
          )
        )}

        <Card
          size="small"
          title="实例明细"
          extra={
            <Input.Search
              allowClear
              placeholder="搜索 provider / agent / 版本…"
              style={{ width: 260 }}
              onSearch={setKeyword}
              onChange={(event) => {
                if (!event.target.value) setKeyword('');
              }}
            />
          }
        >
          <Table
            size="small"
            rowKey={(record) => `${record.providerId}-${record.agentId}`}
            loading={loading}
            columns={columns}
            dataSource={filteredInstances}
            pagination={{ pageSize: 10, hideOnSinglePage: true }}
          />
        </Card>
      </Space>
    </PageContainer>
  );
}
