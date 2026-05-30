import { history, useParams } from '@umijs/max';
import { Button, Space, Typography, Badge, Card, Row, Col, Tag, Divider } from 'antd';
import { useEffect } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { PageStatePanel } from '@/components';
import WorkspaceRenderer, { useWorkspaceConfig } from '@/components/WorkspaceRenderer';
import { trackWorkspaceEvent } from '@/services/workspace/telemetry';
import { buildWorkspaceQualityReport } from '@/services/workspace/quality';

function safeDecodeObjectKey(value: string): { objectKey: string; invalid: boolean } {
  try {
    const decoded = decodeURIComponent(String(value || '')).trim();
    return {
      objectKey: decoded,
      invalid: !decoded,
    };
  } catch {
    return {
      objectKey: '',
      invalid: true,
    };
  }
}

export default function ConsoleDetailPage() {
  const params = useParams<{ objectKey: string }>();
  const { objectKey, invalid } = safeDecodeObjectKey(String(params?.objectKey || ''));

  useEffect(() => {
    trackWorkspaceEvent('workspace_page_open', {
      page: 'console_detail',
      objectKey,
    });
  }, [objectKey]);

  const { config, loading, error, errorCode, reload } = useWorkspaceConfig(objectKey);
  const quality = config ? buildWorkspaceQualityReport(config) : null;
  const publishedAt = config?.publishedAt
    ? new Date(config.publishedAt).toLocaleString('zh-CN')
    : '';
  const updatedAt = config?.meta?.updatedAt
    ? new Date(config.meta.updatedAt).toLocaleString('zh-CN')
    : '';
  const statusTone = quality?.blockingCount
    ? 'error'
    : quality?.warningCount
    ? 'warning'
    : 'success';
  const heroBackground =
    statusTone === 'error'
      ? 'linear-gradient(135deg, rgba(255,77,79,0.12) 0%, rgba(22,119,255,0.03) 100%)'
      : statusTone === 'warning'
      ? 'linear-gradient(135deg, rgba(250,173,20,0.12) 0%, rgba(22,119,255,0.03) 100%)'
      : 'linear-gradient(135deg, rgba(82,196,26,0.12) 0%, rgba(22,119,255,0.03) 100%)';

  const metricCards = quality
    ? [
        {
          label: '发布评分',
          value: `${quality.score}/100`,
          hint: quality.headline,
        },
        {
          label: '可运行页面',
          value: `${quality.readyTabCount}/${quality.tabs.length || 0}`,
          hint: quality.tabs.length > 0 ? '已通过运行检查的标签页' : '还没有可访问页面',
        },
        {
          label: '风险提示',
          value: String(quality.warningCount),
          hint: quality.warningCount > 0 ? '发布后仍需重点核对' : '当前未发现明显风险',
        },
        {
          label: '阻塞项',
          value: String(quality.blockingCount),
          hint: quality.blockingCount > 0 ? '应先回工作台处理' : '当前没有硬阻塞',
        },
      ]
    : [];

  if (invalid) {
    return (
      <PageContainer title="运行控制台">
        <PageStatePanel
          tone="error"
          badgeText="对象标识错误"
          title="无法打开运行页"
          description="当前地址里的对象标识无法识别，请从控制台列表重新进入。"
          actions={
            <Button type="primary" onClick={() => history.push('/system/functions/console')}>
              返回控制台列表
            </Button>
          }
        />
      </PageContainer>
    );
  }

  if (!loading && errorCode === 'workspace_not_found') {
    return (
      <PageContainer title={objectKey}>
        <PageStatePanel
          tone="warning"
          badgeText="未找到已发布配置"
          title="当前对象还没有可运行版本"
          description="控制台只展示已发布工作台。请先回对象工作台完成发布，再来验证运行结果。"
          extra={<Typography.Text code>{objectKey}</Typography.Text>}
          actions={
            <>
              <Button
                type="primary"
                onClick={() =>
                  history.push(
                    `/system/functions/workspace-editor/${encodeURIComponent(objectKey)}`,
                  )
                }
              >
                去发布工作台
              </Button>
              <Button onClick={() => history.push('/system/functions/workspaces')}>
                查看对象工作台
              </Button>
            </>
          }
        />
      </PageContainer>
    );
  }

  if (!loading && errorCode === 'forbidden') {
    return (
      <PageContainer title={objectKey}>
        <PageStatePanel
          tone="error"
          badgeText="权限受限"
          title="当前账号无法访问该运行页"
          description="你没有查看该工作台已发布配置的权限，需要具备控制台读取权限后才能继续。"
          extra={<Typography.Text code>{objectKey}</Typography.Text>}
          actions={<Button onClick={() => history.push('/system/functions/console')}>返回控制台列表</Button>}
        />
      </PageContainer>
    );
  }

  if (!loading && errorCode && errorCode !== 'unknown') {
    return (
      <PageContainer title={objectKey}>
        <PageStatePanel
          tone="warning"
          badgeText="加载异常"
          title="加载失败"
          description={error || '工作台加载失败，请稍后重试。'}
          extra={<Typography.Text code>{objectKey}</Typography.Text>}
          actions={
            <>
              <Button type="primary" onClick={reload}>
                重试
              </Button>
              <Button onClick={() => history.push('/system/functions/console')}>返回控制台列表</Button>
            </>
          }
        />
      </PageContainer>
    );
  }

  return (
    <PageContainer title={config?.title || objectKey}>
      {config && quality ? (
        <Space direction="vertical" size={16} style={{ width: '100%', marginBottom: 16 }}>
        <Card
          styles={{
            body: {
              padding: 22,
              background: heroBackground,
            },
          }}
        >
          <Space direction="vertical" size={18} style={{ width: '100%' }}>
            <Space wrap size={[8, 8]}>
              <Tag color="processing">正式运行态</Tag>
              <Typography.Text code>{objectKey}</Typography.Text>
              <Badge
                status={statusTone === 'error' ? 'error' : statusTone === 'warning' ? 'warning' : 'success'}
                text={quality.headline}
              />
              {typeof config.version === 'number' ? (
                <Tag>{`v${config.version}`}</Tag>
              ) : null}
            </Space>
            <Space direction="vertical" size={6} style={{ width: '100%' }}>
              <Typography.Title level={4} style={{ margin: 0 }}>
                {config.title || objectKey}
              </Typography.Title>
              <Typography.Text type="secondary">
                {config.description || '这里展示的是已经发布的真实运行页面，重点核对入口是否清晰、数据是否可信、结构是否达到交付标准。'}
              </Typography.Text>
            </Space>
            <Row gutter={[12, 12]}>
              {metricCards.map((item) => (
                <Col xs={24} sm={12} xl={6} key={item.label}>
                  <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.72)' }}>
                    <Space direction="vertical" size={4}>
                      <Typography.Text type="secondary">{item.label}</Typography.Text>
                      <Typography.Title level={3} style={{ margin: 0 }}>
                        {item.value}
                      </Typography.Title>
                      <Typography.Text type="secondary">{item.hint}</Typography.Text>
                    </Space>
                  </Card>
                </Col>
              ))}
            </Row>
            <Space wrap size={[8, 8]}>
              {publishedAt ? <Tag color="success">{`发布时间 ${publishedAt}`}</Tag> : null}
              {updatedAt ? <Tag>{`最近更新 ${updatedAt}`}</Tag> : null}
              {config.meta?.updatedBy ? <Tag>{`更新人 ${config.meta.updatedBy}`}</Tag> : null}
            </Space>
            <Typography.Text type="secondary">{quality.summary}</Typography.Text>
            <Space wrap size={[8, 8]}>
              <Button type="primary" onClick={reload}>
                刷新运行结果
              </Button>
              <Button
                onClick={() =>
                  history.push(`/system/functions/workspace-editor/${encodeURIComponent(objectKey)}`)
                }
              >
                回对象工作台修改
              </Button>
              <Button onClick={() => history.push('/system/functions/console')}>返回控制台列表</Button>
            </Space>
          </Space>
        </Card>

        {quality.tabs.length > 0 ? (
          <Card
            title="运行页检查摘要"
            extra={<Typography.Text type="secondary">{`共 ${quality.tabs.length} 个标签页`}</Typography.Text>}
          >
            <Row gutter={[12, 12]}>
              {quality.tabs.map((tab) => {
                const levelTone =
                  tab.level === 'blocking' ? 'error' : tab.level === 'warning' ? 'warning' : 'success';
                const topItems = tab.items.filter((item) => item.level !== 'ready').slice(0, 2);
                return (
                  <Col xs={24} lg={12} key={tab.key}>
                    <Card size="small" style={{ height: '100%' }}>
                      <Space direction="vertical" size={10} style={{ width: '100%' }}>
                        <Space wrap size={[8, 8]}>
                          <Typography.Text strong>{tab.title}</Typography.Text>
                          <Tag>{tab.layoutType}</Tag>
                          <Badge
                            status={
                              levelTone === 'error'
                                ? 'error'
                                : levelTone === 'warning'
                                ? 'warning'
                                : 'success'
                            }
                            text={tab.summary}
                          />
                        </Space>
                        {topItems.length > 0 ? (
                          <Space direction="vertical" size={6} style={{ width: '100%' }}>
                            {topItems.map((item) => (
                              <Typography.Text key={`${tab.key}-${item.title}-${item.detail}`} type="secondary">
                                {`${item.title}：${item.detail}`}
                              </Typography.Text>
                            ))}
                          </Space>
                        ) : (
                          <Typography.Text type="secondary">
                            当前页面在主函数、核心函数和页面骨架层面已通过检查。
                          </Typography.Text>
                        )}
                      </Space>
                    </Card>
                  </Col>
                );
              })}
            </Row>
          </Card>
        ) : null}
        <Divider style={{ margin: 0 }} />
        </Space>
      ) : null}
      <WorkspaceRenderer
        config={config}
        loading={loading}
        error={error}
        context={{ runtimeMode: 'console' }}
      />
    </PageContainer>
  );
}
