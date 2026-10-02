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
  fetchProviderMetaOptions,
  fetchSdkStats,
  type SdkInstanceItem,
  type SdkLanguageStats,
  type SdkStatsResponse,
} from '@/services/api/sdkStats';
import ServerOptionsSelect from '@/components/ServerOptionsSelect';
import { useScopeReload } from '@/hooks/useScopeReload';
import { FormattedMessage, useIntl } from '@umijs/max';

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
  const intl = useIntl();
  const maxCount = Math.max(...stats.versions.map((item) => item.count), 1);
  return (
    <Card
      size="small"
      title={
        <Space>
          <Tag color={languageColor(stats.language)}>{stats.language}</Tag>
          <Text type="secondary">
            {intl.formatMessage(
              {
                id: 'pages.functionsSdk.languageCard.instanceCount',
                defaultMessage: `${stats.count} 实例`,
              },
              { count: stats.count },
            )}
          </Text>
        </Space>
      }
      style={{ height: '100%' }}
    >
      <Space orientation="vertical" size={6} style={{ width: '100%' }}>
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
                  background: 'linear-gradient(90deg, #93394d88, #93394d)',
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
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [stats, setStats] = useState<SdkStatsResponse | null>(null);
  const [keyword, setKeyword] = useState('');
  // #2：元数据过滤下拉（选项来自服务端 meta-options 聚合），选中即走
  // sdk-stats 的 metaKey/metaValue 服务端过滤（子串匹配）。
  const [metaKey, setMetaKey] = useState<string | undefined>(undefined);
  const [metaValue, setMetaValue] = useState<string | undefined>(undefined);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      setStats(await fetchSdkStats({ metaKey, metaValue }));
    } catch {
      // 错误提示交给全局拦截器；保留旧数据
    } finally {
      setLoading(false);
    }
  }, [metaKey, metaValue]);

  useEffect(() => {
    refresh();
    const timer = setInterval(refresh, 30000);
    return () => clearInterval(timer);
  }, [refresh]);

  // #38：SDK 实例与游戏绑定（请求经 X-Game-ID/X-Env 由服务端过滤），
  // 顶栏切游戏后必须重拉，否则列表停留在旧游戏。
  const { scope } = useScopeReload(refresh);

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
        // 实例元数据参与搜索：k=v 整对 + 单独的键/值（如直接粘 serverId 值）
        ...Object.entries(item.metadata ?? {}).flatMap(([key, value]) => [
          `${key}=${value}`,
          key,
          value,
        ]),
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
    // #3：元数据是排查实例归属的主信息（serverId 等），从倒数第二列提到
    // 第二列并加宽——挤在行尾窄列里 tag 全靠悬停才能看全。
    {
      title: intl.formatMessage({
        id: 'pages.functionsSdk.column.metadata',
        defaultMessage: '元数据',
      }),
      dataIndex: 'metadata',
      key: 'metadata',
      width: 280,
      render: (value?: Record<string, string>) => {
        const entries = Object.entries(value ?? {});
        if (!entries.length) return '-';
        return (
          <Space size={4} wrap>
            {entries.slice(0, 3).map(([key, val]) => (
              <Tag key={key} style={{ marginInlineEnd: 0 }}>
                {key}={val}
              </Tag>
            ))}
            {entries.length > 3 && (
              <Tooltip title={entries.map(([key, val]) => `${key}=${val}`).join('\n')}>
                <Tag style={{ marginInlineEnd: 0 }}>+{entries.length - 3}</Tag>
              </Tooltip>
            )}
          </Space>
        );
      },
    },
    { title: 'Agent', dataIndex: 'agentId', key: 'agentId' },
    {
      title: intl.formatMessage({
        id: 'pages.functionsSdk.column.language',
        defaultMessage: '语言',
      }),
      dataIndex: 'sdkLanguage',
      key: 'sdkLanguage',
      render: (value: string) => <Tag color={languageColor(value)}>{value}</Tag>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsSdk.column.sdkVersion',
        defaultMessage: 'SDK 版本',
      }),
      dataIndex: 'sdkVersion',
      key: 'sdkVersion',
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsSdk.column.sdkName',
        defaultMessage: 'SDK 名称',
      }),
      dataIndex: 'sdkName',
      key: 'sdkName',
      render: (value: string) => value || '-',
    },
    { title: 'Game', dataIndex: 'gameId', key: 'gameId' },
    { title: 'Env', dataIndex: 'env', key: 'env' },
    {
      // #44：注册时间（服务端已归一，零值回退最后活跃）。进程窗口语义——
      // 在线会话是内存态，agent/provider 重启后从零计起，排查「实例活了
      // 多久 vs 只是无响应」时与最后活跃列对照看。
      title: intl.formatMessage({
        id: 'pages.functionsSdk.column.firstSeen',
        defaultMessage: '注册时间',
      }),
      dataIndex: 'firstSeenUnix',
      key: 'firstSeenUnix',
      render: (value: number) =>
        value ? (
          <Tooltip title={new Date(value * 1000).toLocaleString()}>
            <Text type="secondary">{new Date(value * 1000).toLocaleTimeString()}</Text>
          </Tooltip>
        ) : (
          '-'
        ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.functionsSdk.column.lastSeen',
        defaultMessage: '最后活跃',
      }),
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
      title={intl.formatMessage({
        id: 'pages.functionsSdk.pageTitle',
        defaultMessage: 'SDK 版本分布',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.functionsSdk.pageSubtitle',
        defaultMessage: '当前在线 provider 实例的 SDK 语言与版本聚合（30s 自动刷新）',
      })}
      extra={[
        <Button key="refresh" icon={<ReloadOutlined />} loading={loading} onClick={refresh}>
          <FormattedMessage id="pages.functionsSdk.button.refresh" defaultMessage="刷新" />
        </Button>,
      ]}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <Card size="small">
          <Space size={48} wrap>
            <Statistic
              title={intl.formatMessage({
                id: 'pages.functionsSdk.stat.onlineInstances',
                defaultMessage: '在线实例',
              })}
              value={stats?.totalInstances ?? 0}
            />
            <Statistic
              title={intl.formatMessage({
                id: 'pages.functionsSdk.stat.sdkLanguages',
                defaultMessage: 'SDK 语言',
              })}
              value={stats?.languages?.length ?? 0}
            />
            <Statistic
              title={intl.formatMessage({
                id: 'pages.functionsSdk.stat.activeAgents',
                defaultMessage: '活跃 Agent',
              })}
              value={agentCount}
            />
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
              <Empty
                description={intl.formatMessage(
                  {
                    id: 'pages.functionsSdk.empty',
                    defaultMessage:
                      '游戏 {game} / 环境 {env} 下没有在线的 provider 实例（可切换顶栏游戏或等待 SDK 接入）',
                  },
                  { game: scope.gameId || '-', env: scope.env || '-' },
                )}
                style={{ padding: '24px 0' }}
              />
            </Card>
          )
        )}

        <Card
          size="small"
          title={intl.formatMessage({
            id: 'pages.functionsSdk.instances.title',
            defaultMessage: '实例明细',
          })}
          extra={
            <Space size={8} wrap>
              {/* #2：下拉选项来自服务端实例元数据聚合（不可从过滤后列表
                  推导——同 #14 塌缩病灶）；count = 该键的实例数 */}
              <ServerOptionsSelect
                allowClear
                placeholder={intl.formatMessage({
                  id: 'pages.functionsSdk.filter.metaKey',
                  defaultMessage: '元数据键',
                })}
                style={{ minWidth: 160 }}
                fetchOptions={async () =>
                  (await fetchProviderMetaOptions()).map((k) => ({
                    value: k.key,
                    label: k.key,
                    count: k.values.reduce((sum, v) => sum + v.count, 0),
                  }))
                }
                value={metaKey}
                onChange={(next: unknown) => {
                  setMetaKey(typeof next === 'string' ? next : undefined);
                  setMetaValue(undefined);
                }}
              />
              {/* 值选项跟随所选键（epoch 触发重拉）；未选键时禁用 */}
              <ServerOptionsSelect
                allowClear
                disabled={!metaKey}
                placeholder={intl.formatMessage({
                  id: 'pages.functionsSdk.filter.metaValue',
                  defaultMessage: '元数据值',
                })}
                style={{ minWidth: 160 }}
                fetchOptions={async () =>
                  (await fetchProviderMetaOptions())
                    .find((k) => k.key === metaKey)
                    ?.values.map((v) => ({ value: v.value, label: v.value, count: v.count })) ?? []
                }
                epoch={metaKey}
                value={metaValue}
                onChange={(next: unknown) => {
                  setMetaValue(typeof next === 'string' ? next : undefined);
                }}
              />
              <Input.Search
                allowClear
                placeholder={intl.formatMessage({
                  id: 'pages.functionsSdk.instances.searchPlaceholder',
                  defaultMessage: '搜索 provider / agent / 版本 / 元数据…',
                })}
                style={{ width: 260 }}
                onSearch={setKeyword}
                onChange={(event) => {
                  if (!event.target.value) setKeyword('');
                }}
              />
            </Space>
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
