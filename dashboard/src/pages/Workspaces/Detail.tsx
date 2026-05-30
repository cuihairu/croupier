import { history, useAccess, useParams } from '@umijs/max';
import {
  Alert,
  Button,
  Modal,
  Tooltip,
  message,
  Space,
  Typography,
  Badge,
  Card,
  Row,
  Col,
  Tag,
  Divider,
} from 'antd';
import { DeleteOutlined, EditOutlined, PlayCircleOutlined } from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import { PageStatePanel } from '@/components';
import WorkspaceRenderer, { useWorkspaceConfig } from '@/components/WorkspaceRenderer';
import { deleteWorkspaceConfig } from '@/services/workspaceConfig';
import { trackWorkspaceEvent } from '@/services/workspace/telemetry';
import { getWorkspaceErrorMessage } from '@/services/workspace/errors';
import { buildWorkspaceQualityReport } from '@/services/workspace/quality';

export default function WorkspaceDetailPage() {
  const access = useAccess() as any;
  const params = useParams<{ objectKey: string }>();
  const objectKey = decodeURIComponent(String(params?.objectKey || ''));

  const { config, loading, error, errorCode, reload } = useWorkspaceConfig(objectKey);
  const canEdit = Boolean(access?.canWorkspaceEdit);
  const canDelete = Boolean(access?.canWorkspaceDelete);
  const statusText = config?.status || (config?.published ? 'published' : 'draft');
  const quality = config ? buildWorkspaceQualityReport(config) : null;
  const statusBadge =
    statusText === 'published'
      ? { color: 'green', text: '已发布' }
      : statusText === 'archived'
      ? { color: 'orange', text: '已归档' }
      : { color: 'default', text: '草稿' };
  const heroBackground =
    quality?.blockingCount && statusText !== 'published'
      ? 'linear-gradient(135deg, rgba(255,77,79,0.12) 0%, rgba(22,119,255,0.03) 100%)'
      : quality?.warningCount
      ? 'linear-gradient(135deg, rgba(250,173,20,0.12) 0%, rgba(22,119,255,0.03) 100%)'
      : 'linear-gradient(135deg, rgba(22,119,255,0.08) 0%, rgba(82,196,26,0.04) 100%)';
  const updatedAt = config?.meta?.updatedAt
    ? new Date(config.meta.updatedAt).toLocaleString('zh-CN')
    : '';

  const handleDelete = () => {
    if (!canDelete) {
      message.error('无删除权限');
      return;
    }
    Modal.confirm({
      title: '确认删除',
      content: (
        <div>
          <div>{`对象: ${objectKey}`}</div>
          <div>{`标题: ${config?.title || '-'}`}</div>
          <div style={{ marginTop: 8, color: '#cf1322' }}>删除后不可恢复，请谨慎操作。</div>
        </div>
      ),
      okButtonProps: { danger: true },
      okText: '确认删除',
      cancelText: '取消',
      onOk: async () => {
        try {
          await deleteWorkspaceConfig(objectKey);
          trackWorkspaceEvent('workspace_delete', { objectKey, scope: 'workspaces_detail' });
          message.success('删除成功');
          history.push('/system/functions/workspaces');
        } catch (err: any) {
          trackWorkspaceEvent('workspace_delete_error', {
            objectKey,
            scope: 'workspaces_detail',
            error: err?.message || String(err),
          });
          message.error(getWorkspaceErrorMessage(err, '删除失败'));
        }
      },
    });
  };

  if (!objectKey) {
    return (
      <PageContainer title="对象工作台">
        <PageStatePanel
          tone="error"
          badgeText="对象标识错误"
          title="无法打开对象工作台"
          description="当前地址里的对象标识无法识别，请从对象工作台列表重新进入。"
          actions={
            <Button type="primary" onClick={() => history.push('/system/functions/workspaces')}>
              返回对象工作台
            </Button>
          }
        />
      </PageContainer>
    );
  }

  if (!canEdit && !canDelete && !access?.canWorkspaceRead) {
    return (
      <PageContainer title={objectKey}>
        <PageStatePanel
          tone="error"
          badgeText="权限受限"
          title="当前账号无法查看对象工作台"
          description="你没有查看该对象工作台的权限，需要先具备读取权限后才能访问装配结果。"
          extra={<Typography.Text code>{objectKey}</Typography.Text>}
          actions={<Button onClick={() => history.push('/system/functions/workspaces')}>返回对象工作台</Button>}
        />
      </PageContainer>
    );
  }

  if (!loading && errorCode === 'workspace_not_found') {
    return (
      <PageContainer title={objectKey}>
        <PageStatePanel
          tone="warning"
          badgeText="未找到配置"
          title="当前对象还没有工作台配置"
          description="你还没有为这个对象建立页面装配。先进入编辑器创建骨架，再补齐字段、布局与发布配置。"
          extra={<Typography.Text code>{objectKey}</Typography.Text>}
          actions={
            <>
              <Button
                type="primary"
                disabled={!canEdit}
                onClick={() =>
                  history.push(`/system/functions/workspace-editor/${encodeURIComponent(objectKey)}`)
                }
              >
                创建工作台
              </Button>
              <Button onClick={() => history.push('/system/functions/workspaces')}>
                返回对象工作台
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
          title="当前账号无法访问该配置"
          description="你没有查看这份工作台配置的权限，需要具备对象工作台读取权限后才能继续。"
          extra={<Typography.Text code>{objectKey}</Typography.Text>}
          actions={<Button onClick={() => history.push('/system/functions/workspaces')}>返回对象工作台</Button>}
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
              <Button onClick={() => history.push('/system/functions/workspaces')}>返回对象工作台</Button>
            </>
          }
        />
      </PageContainer>
    );
  }

  return (
    <PageContainer
      title={
        <Space wrap size={[10, 8]}>
          <span>{config?.title || objectKey}</span>
          <Badge color={statusBadge.color} text={statusBadge.text} />
        </Space>
      }
      subTitle="这里查看对象工作台当前装配结果。结构调整进入页面编排器，发布后验证进入运行控制台。"
      extra={[
        <Tooltip key="edit" title={canEdit ? '' : '无编辑权限'}>
          <span>
            <Button
              icon={<EditOutlined />}
              disabled={!canEdit}
              onClick={() =>
                history.push(`/system/functions/workspace-editor/${encodeURIComponent(objectKey)}`)
              }
            >
              编辑配置
            </Button>
          </span>
        </Tooltip>,
        <Tooltip key="delete" title={canDelete ? '' : '无删除权限'}>
          <span>
            <Button danger icon={<DeleteOutlined />} disabled={!canDelete} onClick={handleDelete}>
              删除配置
            </Button>
          </span>
        </Tooltip>,
      ]}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
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
              <Tag color="blue">对象工作台详情</Tag>
              <Badge color={statusBadge.color} text={statusBadge.text} />
              <Typography.Text code>{objectKey}</Typography.Text>
              {typeof config?.version === 'number' ? <Tag>{`v${config.version}`}</Tag> : null}
              {config?.layout?.tabs?.length ? <Tag>{`${config.layout.tabs.length} 个标签页`}</Tag> : null}
            </Space>
            <Space direction="vertical" size={6} style={{ width: '100%' }}>
              <Typography.Title level={4} style={{ margin: 0 }}>
                {config?.title || objectKey}
              </Typography.Title>
              <Typography.Text type="secondary">
                {config?.description ||
                  '这里查看对象页面的当前装配结果。先确认信息结构和页面层级，再决定回编辑器继续调整，或去运行控制台验证最终访问效果。'}
              </Typography.Text>
            </Space>
            <Row gutter={[12, 12]}>
              <Col xs={24} sm={12} xl={6}>
                <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.78)' }}>
                  <Space direction="vertical" size={4}>
                    <Typography.Text type="secondary">装配评分</Typography.Text>
                    <Typography.Title level={3} style={{ margin: 0 }}>
                      {quality ? `${quality.score}/100` : '-'}
                    </Typography.Title>
                    <Typography.Text type="secondary">
                      {quality?.headline || '等待工作台配置加载'}
                    </Typography.Text>
                  </Space>
                </Card>
              </Col>
              <Col xs={24} sm={12} xl={6}>
                <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.78)' }}>
                  <Space direction="vertical" size={4}>
                    <Typography.Text type="secondary">已准备页面</Typography.Text>
                    <Typography.Title level={3} style={{ margin: 0 }}>
                      {quality ? `${quality.readyTabCount}/${quality.tabs.length || 0}` : '-'}
                    </Typography.Title>
                    <Typography.Text type="secondary">通过基础装配检查的标签页数量</Typography.Text>
                  </Space>
                </Card>
              </Col>
              <Col xs={24} sm={12} xl={6}>
                <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.78)' }}>
                  <Space direction="vertical" size={4}>
                    <Typography.Text type="secondary">风险提示</Typography.Text>
                    <Typography.Title level={3} style={{ margin: 0 }}>
                      {quality?.warningCount ?? 0}
                    </Typography.Title>
                    <Typography.Text type="secondary">需要重点核对的页面或配置提醒</Typography.Text>
                  </Space>
                </Card>
              </Col>
              <Col xs={24} sm={12} xl={6}>
                <Card size="small" style={{ height: '100%', background: 'rgba(255,255,255,0.78)' }}>
                  <Space direction="vertical" size={4}>
                    <Typography.Text type="secondary">最近更新</Typography.Text>
                    <Typography.Title level={5} style={{ margin: 0 }}>
                      {updatedAt || '-'}
                    </Typography.Title>
                    <Typography.Text type="secondary">确认当前看到的是最新装配草稿</Typography.Text>
                  </Space>
                </Card>
              </Col>
            </Row>
            <Space wrap size={[8, 8]}>
              <Button
                type="primary"
                icon={<EditOutlined />}
                disabled={!canEdit}
                onClick={() =>
                  history.push(`/system/functions/workspace-editor/${encodeURIComponent(objectKey)}`)
                }
              >
                进入编辑器
              </Button>
              <Button
                icon={<PlayCircleOutlined />}
                disabled={!config?.published}
                onClick={() => history.push(`/console/${encodeURIComponent(objectKey)}`)}
              >
                去运行控制台
              </Button>
            </Space>
          </Space>
        </Card>
        <Alert
          type="info"
          showIcon
          message="对象工作台是页面装配层"
          description="函数目录负责能力供给，页面编排器负责结构调整，这里负责查看当前对象已经装配出的页面和运行结构。"
        />
        {quality?.tabs?.length ? (
          <Card
            title="装配检查摘要"
            extra={<Typography.Text type="secondary">{quality.summary}</Typography.Text>}
          >
            <Row gutter={[12, 12]}>
              {quality.tabs.map((tab) => (
                <Col xs={24} lg={12} key={tab.key}>
                  <Card size="small" style={{ height: '100%' }}>
                    <Space direction="vertical" size={10} style={{ width: '100%' }}>
                      <Space wrap size={[8, 8]}>
                        <Typography.Text strong>{tab.title}</Typography.Text>
                        <Tag>{tab.layoutType}</Tag>
                        <Badge
                          status={
                            tab.level === 'blocking'
                              ? 'error'
                              : tab.level === 'warning'
                              ? 'warning'
                              : 'success'
                          }
                          text={tab.summary}
                        />
                      </Space>
                      <Typography.Text type="secondary">
                        {tab.items[0]?.detail || '当前页面已具备基础装配能力。'}
                      </Typography.Text>
                    </Space>
                  </Card>
                </Col>
              ))}
            </Row>
          </Card>
        ) : null}
        <Divider style={{ margin: 0 }} />
      </Space>
      <WorkspaceRenderer
        config={config}
        loading={loading}
        error={error}
        context={{ runtimeMode: 'workspace' }}
      />
    </PageContainer>
  );
}
