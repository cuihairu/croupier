import React, { useEffect, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Alert, Badge, Button, Card, Form, Space, Tabs, Tag, Typography, Row, Col } from 'antd';
import {
  AppstoreOutlined,
  ArrowLeftOutlined,
  CopyOutlined,
  DeleteOutlined,
  EditOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  SaveOutlined,
} from '@ant-design/icons';
import { history, useLocation, useParams } from '@umijs/max';
import { App } from 'antd';
import { DASHBOARD_PAGE_TOKENS, PageStatePanel } from '@/components';
import { BasicInfoTab, PermissionsTab } from './DetailSections';
import { AnalyticsTab, HistoryTab, WarningsTab } from './DetailTabs';
import DetailConfigTab from './DetailConfigTab';
import useFunctionDetailPage from './useFunctionDetailPage';
import { FUNCTION_DETAIL_SCHEMA, type DetailActionKey, type DetailTabKey } from './detailSchema';

export default function FunctionDetailPage() {
  const { message } = App.useApp();
  const location = useLocation();
  const params = useParams<{ id: string }>();
  const searchParams = new URLSearchParams(location.search);
  const [activeTab, setActiveTab] = useState(searchParams.get('tab') || 'basic');
  const [activeSubTab, setActiveSubTab] = useState(searchParams.get('subTab') || 'json');

  const {
    loading,
    functionDetail,
    editing,
    setEditing,
    form,
    permLoading,
    permSaving,
    permError,
    permForm,
    routeConfigSaving,
    routeConfigForm,
    routePreview,
    parsedInputSchema,
    effectiveCategory,
    jsonViewData,
    uiDescriptor,
    loadDetail,
    handleSave,
    handleStatusToggle,
    handleCopy,
    handleDelete,
    handleSavePermissions,
    handleSaveRoute,
    handleResetRoute,
    onSaveUi,
  } = useFunctionDetailPage(params.id);
  const workspaceObjectKey = String(uiDescriptor?.entity || params.id?.split('.')[0] || '').trim();
  const workspaceSearch = new URLSearchParams();
  if (workspaceObjectKey) workspaceSearch.set('objectKey', workspaceObjectKey);
  if (params.id) workspaceSearch.set('functionId', params.id);
  workspaceSearch.set('from', 'function_detail');
  const workspaceEditorPath = `/system/functions/workspaces?${workspaceSearch.toString()}`;
  const invokePath = params.id ? `/system/functions/invoke?fid=${encodeURIComponent(params.id)}` : '';

  const buildSearch = (tab: string, subTab?: string) => {
    const search = new URLSearchParams(location.search);
    search.set('tab', tab);
    if (tab === 'config') search.set('subTab', subTab || activeSubTab || 'json');
    else search.delete('subTab');
    const query = search.toString();
    return query ? `?${query}` : '';
  };

  useEffect(() => {
    const next = new URLSearchParams(location.search);
    setActiveTab(next.get('tab') || 'basic');
    setActiveSubTab(next.get('subTab') || 'json');
  }, [location.search]);

  if (!functionDetail && !loading) {
    return (
      <PageContainer>
        <PageStatePanel
          tone="warning"
          badgeText="未找到函数"
          title="当前函数不存在"
          description="请检查函数 ID 是否正确，或从函数目录重新进入。"
          extra={params.id}
          actions={
            <Button type="primary" onClick={() => history.push('/system/functions/catalog')}>
              返回函数列表
            </Button>
          }
        />
      </PageContainer>
    );
  }

  const tabContent: Record<DetailTabKey, React.ReactNode> = {
    basic: (
      <BasicInfoTab
        functionDetail={functionDetail}
        effectiveCategory={effectiveCategory}
        editing={editing}
        onStatusToggle={handleStatusToggle}
      />
    ),
    config: (
      <DetailConfigTab
        functionId={params.id || ''}
        activeSubTab={activeSubTab}
        onSubTabChange={(key) => {
          setActiveSubTab(key);
          history.replace(`${location.pathname}${buildSearch('config', key)}`);
        }}
        jsonViewData={jsonViewData}
        onJsonCopySuccess={() => message.success('JSON 已复制')}
        onJsonCopyError={() => message.error('复制失败')}
        uiDescriptor={uiDescriptor}
        parsedInputSchema={parsedInputSchema}
        onSaveUi={onSaveUi}
        routePreview={routePreview || {}}
        routeConfigForm={routeConfigForm}
        routeConfigSaving={routeConfigSaving}
        onSaveRoute={handleSaveRoute}
        onResetRoute={handleResetRoute}
        onOpenAssignments={() => history.push('/system/functions/assignments')}
      />
    ),
    permissions: (
      <PermissionsTab
        functionId={params.id}
        permError={permError}
        permLoading={permLoading}
        permSaving={permSaving}
        permForm={permForm}
        onSave={handleSavePermissions}
      />
    ),
    history: <HistoryTab functionId={params.id || ''} />,
    analytics: <AnalyticsTab functionId={params.id || ''} />,
    warnings: <WarningsTab functionId={params.id || ''} />,
  };

  const mainTabItems = FUNCTION_DETAIL_SCHEMA.tabs.map((tab) => ({
    key: tab.key,
    label: tab.label,
    children: tabContent[tab.key],
  }));

  const actionFlags = {
    loading,
    noFunction: !functionDetail,
  } as const;

  const descriptorEntity = String(uiDescriptor?.entity || '').trim();
  const descriptorOperation = String(uiDescriptor?.operation || '').trim();
  const functionStatusText = functionDetail?.enabled ? '已启用' : '未启用';
  const functionStatusTone = functionDetail?.enabled ? 'success' : 'default';

  const runAction = (key: DetailActionKey) => {
    if (key === 'reload') return loadDetail();
    if (key === 'copy') return handleCopy();
    if (key === 'delete') return handleDelete();
    if (editing) form.submit();
    else setEditing(true);
    return undefined;
  };

  return (
    <PageContainer
      title={
        <Space>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => history.push('/system/functions/catalog')}
          >
            返回
          </Button>
          <span>{functionDetail?.name || functionDetail?.id}</span>
          <Badge status={functionDetail?.enabled ? 'success' : 'default'} />
        </Space>
      }
      subTitle="这里处理单个函数的描述、表单与挂载信息。函数是原子能力，对象工作台负责把它装配成业务页面。"
      extra={[
        <Space key="actions">
          <Button onClick={() => history.push(workspaceEditorPath)}>
            {workspaceObjectKey ? `去 ${workspaceObjectKey} 对象工作台` : '去对象工作台'}
          </Button>
          {FUNCTION_DETAIL_SCHEMA.actions.map((action) => (
            <Button
              key={action.key}
              type={action.primary ? 'primary' : 'default'}
              danger={!!action.danger}
              loading={action.loadingWhen ? !!actionFlags[action.loadingWhen] : false}
              disabled={!!action.disabledWhen?.some((flag) => !!actionFlags[flag])}
              icon={
                action.key === 'reload' ? (
                  <ReloadOutlined />
                ) : action.key === 'copy' ? (
                  <CopyOutlined />
                ) : action.key === 'delete' ? (
                  <DeleteOutlined />
                ) : editing ? (
                  <SaveOutlined />
                ) : (
                  <EditOutlined />
                )
              }
              onClick={() => runAction(action.key)}
            >
              {action.key === 'edit' && editing ? '保存' : action.label}
            </Button>
          ))}
        </Space>,
      ]}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card
          loading={loading}
          styles={{
            body: {
              padding: DASHBOARD_PAGE_TOKENS.cardPadding,
              background:
                'linear-gradient(135deg, rgba(22,119,255,0.1) 0%, rgba(82,196,26,0.05) 55%, rgba(250,173,20,0.04) 100%)',
            },
          }}
        >
          <Space direction="vertical" size={18} style={{ width: '100%' }}>
            <Space wrap size={[8, 8]}>
              <Tag color="blue">函数能力详情</Tag>
              <Badge status={functionStatusTone} text={functionStatusText} />
              {functionDetail?.version ? <Tag>{`v${functionDetail.version}`}</Tag> : null}
              {effectiveCategory ? <Tag color="purple">{effectiveCategory}</Tag> : null}
              {descriptorEntity ? <Tag>{`对象 ${descriptorEntity}`}</Tag> : null}
              {descriptorOperation ? <Tag>{`操作 ${descriptorOperation}`}</Tag> : null}
            </Space>
            <Space direction="vertical" size={6} style={{ width: '100%' }}>
              <Typography.Title level={4} style={{ margin: 0 }}>
                {functionDetail?.name || functionDetail?.id}
              </Typography.Title>
              <Typography.Text type="secondary">
                {functionDetail?.description ||
                  '这里用于确认单个函数的能力定义、挂载信息和单函数配置。最终业务页面应在对象工作台中完成装配。'}
              </Typography.Text>
            </Space>
            <Row gutter={[12, 12]}>
              <Col xs={24} lg={10}>
                <Card
                  size="small"
                  style={{ height: '100%', background: 'rgba(255,255,255,0.78)' }}
                  styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.cardPadding } }}
                >
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Typography.Text strong>当前建议动作</Typography.Text>
                    <Typography.Text type="secondary">
                      函数能力确认无误后，下一步应该去对象工作台生成页面骨架，而不是把这里当成最终业务界面。
                    </Typography.Text>
                    <Space wrap size={[8, 8]}>
                      <Button
                        type="primary"
                        icon={<AppstoreOutlined />}
                        onClick={() => history.push(workspaceEditorPath)}
                      >
                        {workspaceObjectKey ? `去 ${workspaceObjectKey} 对象工作台` : '去对象工作台'}
                      </Button>
                      <Button
                        icon={<PlayCircleOutlined />}
                        disabled={!params.id}
                        onClick={() => history.push(invokePath)}
                      >
                        测试调用
                      </Button>
                    </Space>
                  </Space>
                </Card>
              </Col>
              <Col xs={24} lg={14}>
                <Card
                  size="small"
                  style={{ height: '100%', background: 'rgba(255,255,255,0.78)' }}
                  styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.cardPadding } }}
                >
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Typography.Text strong>这里适合确认什么</Typography.Text>
                    <Typography.Text type="secondary">
                      重点检查函数摘要、入参 schema、挂载路由、权限、调用历史和告警配置，确认它是否足够稳定地支撑后续页面装配与发布验证。
                    </Typography.Text>
                    <Space wrap size={[8, 8]}>
                      <Badge status="processing" text="函数定义与 schema" />
                      <Badge status="success" text="路由与调用入口" />
                      <Badge status="default" text="权限与告警配置" />
                    </Space>
                  </Space>
                </Card>
              </Col>
            </Row>
          </Space>
        </Card>

        <Card loading={loading}>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="函数层负责能力定义，工作台层负责页面装配"
            description="这里适合校验函数定义、权限、告警和单函数表单配置；如果目标是做运营可用的实际界面，下一步应进入对象工作台完成页面骨架、预览和发布。"
            action={
              <Button type="primary" onClick={() => history.push(workspaceEditorPath)}>
                去对象工作台
              </Button>
            }
          />
          <Form form={form} layout="vertical" onFinish={handleSave} component={false}>
            <Tabs
              activeKey={activeTab}
              onChange={(key) => {
                setActiveTab(key);
                history.replace(
                  `${location.pathname}${buildSearch(
                    key,
                    key === 'config' ? activeSubTab : undefined,
                  )}`,
                );
              }}
              items={mainTabItems}
            />
          </Form>
        </Card>
      </Space>
    </PageContainer>
  );
}
