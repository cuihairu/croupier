import { history, useAccess } from '@umijs/max';
import {
  DASHBOARD_PAGE_TOKENS,
  PageStatePanel,
  StandardFilterBar,
  StandardListSection,
  SummaryOverview,
} from '@/components';
import {
  Alert,
  Card,
  List,
  Space,
  Typography,
  Input,
  Select,
  Button,
  Empty,
  Badge,
  Tag,
  Row,
  Col,
} from 'antd';
import { useEffect, useMemo, useState } from 'react';
import {
  AppstoreOutlined,
  CheckCircleOutlined,
  ReloadOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { listPublishedWorkspaceConfigs } from '@/services/workspaceConfig';
import type { WorkspaceConfig } from '@/types/workspace';
import { trackWorkspaceEvent } from '@/services/workspace/telemetry';
import { buildWorkspaceQualityReport } from '@/services/workspace/quality';
import {
  getWorkspaceErrorMessage,
  parseWorkspaceError,
  type WorkspaceErrorCode,
} from '@/services/workspace/errors';

export default function ConsolePage() {
  const access = useAccess() as any;
  const [loading, setLoading] = useState(false);
  const [configs, setConfigs] = useState<WorkspaceConfig[]>([]);
  const [error, setError] = useState('');
  const [errorCode, setErrorCode] = useState<WorkspaceErrorCode | undefined>();
  const [keyword, setKeyword] = useState('');
  const [sortBy, setSortBy] = useState<'updated_desc' | 'title_asc'>('updated_desc');

  useEffect(() => {
    let mounted = true;
    trackWorkspaceEvent('workspace_page_open', {
      page: 'console_index',
    });
    const load = async () => {
      setLoading(true);
      setError('');
      setErrorCode(undefined);
      try {
        const rows = await listPublishedWorkspaceConfigs();
        trackWorkspaceEvent('workspace_load', {
          scope: 'console_index',
          count: Array.isArray(rows) ? rows.length : 0,
        });
        if (!mounted) return;
        setConfigs(Array.isArray(rows) ? rows : []);
      } catch (err: any) {
        trackWorkspaceEvent('workspace_load_error', {
          scope: 'console_index',
          error: err?.message || String(err),
        });
        if (!mounted) return;
        const parsedError = parseWorkspaceError(err);
        setErrorCode(parsedError.code);
        setError(getWorkspaceErrorMessage(err, '加载控制台失败'));
      } finally {
        if (mounted) setLoading(false);
      }
    };
    load().catch(() => {});
    return () => {
      mounted = false;
    };
  }, []);

  const visibleConfigs = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    const filtered = configs.filter((config) => {
      if (!normalizedKeyword) return true;
      return (
        (config.title || '').toLowerCase().includes(normalizedKeyword) ||
        (config.objectKey || '').toLowerCase().includes(normalizedKeyword) ||
        (config.description || '').toLowerCase().includes(normalizedKeyword)
      );
    });

    const sortable = [...filtered];
    if (sortBy === 'title_asc') {
      sortable.sort((a, b) => (a.title || '').localeCompare(b.title || ''));
      return sortable;
    }

    sortable.sort((a, b) => {
      const aTime = new Date(a.meta?.updatedAt || 0).getTime();
      const bTime = new Date(b.meta?.updatedAt || 0).getTime();
      return bTime - aTime;
    });
    return sortable;
  }, [configs, keyword, sortBy]);

  const latestUpdatedAt = useMemo(() => {
    const timestamps = configs
      .map((item) => new Date(item.meta?.updatedAt || 0).getTime())
      .filter((value) => Number.isFinite(value) && value > 0);
    if (!timestamps.length) return '';
    return new Date(Math.max(...timestamps)).toLocaleString('zh-CN');
  }, [configs]);

  const qualitySummary = useMemo(() => {
    const reports = configs.map((config) => buildWorkspaceQualityReport(config));
    return {
      riskyCount: reports.filter((report) => report.warningCount > 0).length,
    };
  }, [configs]);

  const highlightedConfig = visibleConfigs[0];
  const visibleConfigGroups = useMemo(() => {
    const entries = visibleConfigs.map((config) => ({
      config,
      report: buildWorkspaceQualityReport(config),
    }));
    return {
      needsAttention: entries.filter((entry) => entry.report.warningCount > 0),
      stable: entries.filter((entry) => entry.report.warningCount === 0),
    };
  }, [visibleConfigs]);

  if (!access?.canWorkspaceRead) {
    return (
      <PageStatePanel
        tone="error"
        badgeText="权限受限"
        title="无法进入运行控制台"
        description="你没有查看控制台工作台的权限，需要先具备读取权限后才能查看已发布页面。"
      />
    );
  }
  if (loading) return <Card loading />;
  if (errorCode === 'forbidden') {
    return (
      <PageStatePanel
        tone="error"
        badgeText="权限受限"
        title="无法进入运行控制台"
        description="当前账号没有查看控制台工作台的权限，已发布页面暂时不可见。"
      />
    );
  }
  if (error) {
    return (
      <PageStatePanel
        tone="warning"
        badgeText="加载异常"
        title="控制台列表加载失败"
        description={error}
      />
    );
  }

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <SummaryOverview
        title="运行控制台"
        description="这里只展示已经发布的对象工作台。函数绑定与页面装配在对象工作台完成，控制台负责发布后的访问与验证。"
        items={[
          { color: '#1677ff', text: `已发布 ${configs.length}` },
          {
            color: visibleConfigs.length === configs.length ? '#52c41a' : '#faad14',
            text: `当前结果 ${visibleConfigs.length}`,
          },
          ...(qualitySummary.riskyCount > 0
            ? [{ color: '#faad14', text: `风险提示 ${qualitySummary.riskyCount}` }]
            : []),
          ...(latestUpdatedAt
            ? [{ color: '#722ed1', text: `最近发布视图 ${latestUpdatedAt}` }]
            : []),
        ]}
        hint="产品主路径是：函数目录确认能力，对象工作台完成装配，控制台验证发布结果。"
      />

      {configs.length > 0 ? (
        <Card
          styles={{
            body: {
              padding: DASHBOARD_PAGE_TOKENS.cardPadding,
              background:
                'linear-gradient(135deg, rgba(82,196,26,0.08) 0%, rgba(22,119,255,0.03) 100%)',
            },
          }}
        >
          <Space direction="vertical" size={14} style={{ width: '100%' }}>
            <Space wrap size={[8, 8]}>
              <Tag color="success">已进入运行态</Tag>
              <Tag color="blue">这里只看发布结果，不做页面装配</Tag>
            </Space>
            <Typography.Title level={5} style={{ margin: 0 }}>
              当前运行台焦点
            </Typography.Title>
            <Typography.Text type="secondary">
              优先确认入口是否清晰、页面是否可读、发布版本是否已经达到可交付标准；如果结果不理想，回对象工作台继续调整后再发布。
            </Typography.Text>
            <Row gutter={[16, 16]}>
              <Col xs={24} lg={14}>
                <Card size="small" style={{ height: '100%' }}>
                  <Space direction="vertical" size={10} style={{ width: '100%' }}>
                    <Typography.Text strong>推荐先检查最新入口</Typography.Text>
                    {highlightedConfig ? (
                      <>
                        {(() => {
                          const report = buildWorkspaceQualityReport(highlightedConfig);
                          return (
                            <Space wrap size={[8, 8]}>
                              <Badge
                                status={report.warningCount > 0 ? 'warning' : 'success'}
                                text={`评分 ${report.score}/100`}
                              />
                              {report.warningCount > 0 ? (
                                <Badge status="warning" text={`风险提示 ${report.warningCount}`} />
                              ) : (
                                <Badge status="success" text="已通过质量检查" />
                              )}
                            </Space>
                          );
                        })()}
                        <Space wrap size={[8, 8]}>
                          <Tag color="processing">{highlightedConfig.title}</Tag>
                          <Typography.Text code>{highlightedConfig.objectKey}</Typography.Text>
                          {typeof highlightedConfig.version === 'number' ? (
                            <Tag>{`v${highlightedConfig.version}`}</Tag>
                          ) : null}
                        </Space>
                        <Typography.Text type="secondary">
                          先检查最近更新的页面，通常能最快发现发布命名、入口可见性和页面表达上的问题。
                        </Typography.Text>
                        <Space wrap size={[8, 8]}>
                          <Button
                            type="primary"
                            onClick={() =>
                              history.push(
                                `/console/${encodeURIComponent(highlightedConfig.objectKey)}`,
                              )
                            }
                          >
                            进入最新运行页
                          </Button>
                          <Button
                            onClick={() =>
                              history.push(
                                `/system/functions/workspace-editor/${encodeURIComponent(
                                  highlightedConfig.objectKey,
                                )}`,
                              )
                            }
                          >
                            回对象工作台修改
                          </Button>
                        </Space>
                      </>
                    ) : null}
                  </Space>
                </Card>
              </Col>
              <Col xs={24} lg={10}>
                <Card size="small" style={{ height: '100%' }}>
                  <Space direction="vertical" size={10} style={{ width: '100%' }}>
                    <Typography.Text strong>运行检查清单</Typography.Text>
                    <Typography.Text type="secondary">1. 入口名是否清楚</Typography.Text>
                    <Typography.Text type="secondary">2. 页面首屏是否能读懂</Typography.Text>
                    <Typography.Text type="secondary">3. 发布版本是否与预期一致</Typography.Text>
                    <Typography.Text type="secondary">
                      4. 不通过则回对象工作台继续装配
                    </Typography.Text>
                  </Space>
                </Card>
              </Col>
            </Row>
          </Space>
        </Card>
      ) : null}

      <StandardListSection
        title="已发布工作台"
        extra={
          <Space wrap>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => history.push('/system/functions/workspaces')}
            >
              去对象工作台
            </Button>
            <Button icon={<ReloadOutlined />} onClick={() => window.location.reload()}>
              刷新
            </Button>
          </Space>
        }
      >
        <StandardFilterBar
          resultText={`当前结果 ${visibleConfigs.length} 个`}
          controls={
            <>
              <Space.Compact style={{ width: 320 }}>
                <Input
                  allowClear
                  placeholder="搜索标题 / objectKey / 描述"
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                  onPressEnter={() => {}}
                />
                <Button icon={<ReloadOutlined />} title="搜索" />
              </Space.Compact>
              <Select
                value={sortBy}
                onChange={(val) => setSortBy(val)}
                style={{ width: 180 }}
                options={[
                  { label: '按更新时间降序', value: 'updated_desc' },
                  { label: '按标题升序', value: 'title_asc' },
                ]}
              />
            </>
          }
        />

        {configs.length === 0 ? (
          <Empty description="还没有已发布的工作台">
            <Space wrap>
              <Button
                type="primary"
                icon={<SettingOutlined />}
                onClick={() => history.push('/system/functions/workspaces')}
              >
                去发布第一个工作台
              </Button>
              <Typography.Text type="secondary">
                先在对象工作台完成页面骨架和预览，再发布到这里。
              </Typography.Text>
            </Space>
          </Empty>
        ) : (
          <>
            {visibleConfigs.length === 0 && (
              <Alert
                type="info"
                showIcon
                message="没有匹配的已发布工作台"
                description="请调整搜索关键字，或回对象工作台继续发布更多页面。"
              />
            )}

            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              {visibleConfigGroups.needsAttention.length > 0 ? (
                <Card
                  size="small"
                  title="优先核查"
                  extra={<Typography.Text type="secondary">这些入口已发布，但仍建议先走一轮真实验证</Typography.Text>}
                  styles={{
                    body: {
                      padding: DASHBOARD_PAGE_TOKENS.cardPadding,
                      background: 'rgba(250,173,20,0.06)',
                    },
                  }}
                >
                  <List
                    grid={{ gutter: 16, xs: 1, sm: 2, md: 2, lg: 3, xl: 3, xxl: 4 }}
                    dataSource={visibleConfigGroups.needsAttention}
                    renderItem={({ config, report }) => (
                      <WorkspaceEntryCard config={config} report={report} />
                    )}
                  />
                </Card>
              ) : null}

              {visibleConfigGroups.stable.length > 0 ? (
                <Card
                  size="small"
                  title="稳定入口"
                  extra={<Typography.Text type="secondary">这些入口当前质量更稳定，适合作为默认演示和回归起点</Typography.Text>}
                  styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.cardPadding } }}
                >
                  <List
                    grid={{ gutter: 16, xs: 1, sm: 2, md: 2, lg: 3, xl: 3, xxl: 4 }}
                    dataSource={visibleConfigGroups.stable}
                    renderItem={({ config, report }) => (
                      <WorkspaceEntryCard config={config} report={report} />
                    )}
                  />
                </Card>
              ) : null}
            </Space>
          </>
        )}
      </StandardListSection>
    </Space>
  );
}

function WorkspaceEntryCard({
  config,
  report,
}: {
  config: WorkspaceConfig;
  report: ReturnType<typeof buildWorkspaceQualityReport>;
}) {
  return (
    <List.Item>
      <Card
        hoverable
        styles={{ body: { padding: 18 } }}
        onClick={() => history.push(`/console/${encodeURIComponent(config.objectKey)}`)}
      >
        <Card.Meta
          avatar={
            <div
              style={{
                width: 44,
                height: 44,
                borderRadius: 14,
                display: 'grid',
                placeItems: 'center',
                background: 'linear-gradient(135deg, #1677ff 0%, #69b1ff 100%)',
                color: '#fff',
              }}
            >
              <AppstoreOutlined style={{ fontSize: 20 }} />
            </div>
          }
          title={
            <Space direction="vertical" size={8} style={{ width: '100%' }}>
              <Space wrap size={[8, 8]}>
                <Typography.Text strong style={{ fontSize: 16 }}>
                  {config.title}
                </Typography.Text>
                <Tag color="success" icon={<CheckCircleOutlined />}>
                  已发布
                </Tag>
              </Space>
              <Space wrap size={[8, 6]}>
                <Typography.Text code>{config.objectKey}</Typography.Text>
                {typeof config.version === 'number' ? <Tag>{`v${config.version}`}</Tag> : null}
                {config.layout?.tabs?.length ? <Tag>{`${config.layout.tabs.length} 个标签页`}</Tag> : null}
              </Space>
            </Space>
          }
          description={
            <Space direction="vertical" size={12}>
              <Typography.Text type="secondary">{config.description || '暂无描述'}</Typography.Text>
              <div
                style={{
                  padding: '10px 12px',
                  borderRadius: 10,
                  background:
                    report.warningCount > 0 ? 'rgba(250,173,20,0.10)' : 'rgba(82,196,26,0.08)',
                }}
              >
                <Space direction="vertical" size={4}>
                  <Space wrap size={[8, 8]}>
                    <Badge
                      status={report.warningCount > 0 ? 'warning' : 'success'}
                      text={`评分 ${report.score}/100`}
                    />
                    {report.warningCount > 0 ? (
                      <Badge status="warning" text={`风险提示 ${report.warningCount}`} />
                    ) : (
                      <Badge status="success" text="已通过质量检查" />
                    )}
                  </Space>
                  <Typography.Text
                    style={{
                      fontSize: 12,
                      color: report.warningCount > 0 ? '#d46b08' : '#389e0d',
                    }}
                  >
                    {report.warningCount > 0
                      ? '当前版本已发布，但仍建议重点核对空态、真实数据绑定和首屏表达。'
                      : '这里已经进入运行态，可直接检查入口可见性、页面可读性和整体交付效果。'}
                  </Typography.Text>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {report.summary}
                  </Typography.Text>
                </Space>
              </div>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {config.meta?.updatedAt
                  ? `更新于 ${new Date(config.meta.updatedAt).toLocaleString('zh-CN')}`
                  : '暂无更新时间'}
              </Typography.Text>
              <Space wrap size={[8, 8]}>
                <Button
                  type="primary"
                  onClick={(e) => {
                    e.stopPropagation();
                    history.push(`/console/${encodeURIComponent(config.objectKey)}`);
                  }}
                >
                  进入运行页
                </Button>
                <Button
                  onClick={(e) => {
                    e.stopPropagation();
                    history.push(
                      `/system/functions/workspace-editor/${encodeURIComponent(config.objectKey)}`,
                    );
                  }}
                >
                  回对象工作台
                </Button>
              </Space>
            </Space>
          }
        />
      </Card>
    </List.Item>
  );
}
