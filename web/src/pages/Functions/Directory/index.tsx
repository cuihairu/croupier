import React from 'react';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import {
  Alert,
  Badge,
  Button,
  Card,
  Descriptions,
  Drawer,
  Space,
  Tag,
  Typography,
  Row,
  Col,
} from 'antd';
import { AppstoreOutlined, PlayCircleOutlined, ProfileOutlined } from '@ant-design/icons';
import { history } from '@umijs/max';
import { DASHBOARD_PAGE_TOKENS, StandardListSection, SummaryOverview } from '@/components';
import type { SummaryRow } from './types';
import useDirectoryPage from './useDirectoryPage';

const { Text } = Typography;

export default function DirectoryPage() {
  const {
    loading,
    processedData,
    columns,
    headerActions,
    detailVisible,
    setDetailVisible,
    selectedFunction,
    drawerActions,
    buildInvokePath,
  } = useDirectoryPage();

  const summary = React.useMemo(() => {
    const total = processedData.length;
    const enabledCount = processedData.filter((item) => item.enabled).length;
    const disabledCount = total - enabledCount;
    const categoryCount = new Set(processedData.map((item) => item.category || '未分类')).size;
    const entityBoundCount = processedData.filter((item) => Boolean(item.entity)).length;
    const topCategory = Object.entries(
      processedData.reduce<Record<string, number>>((acc, item) => {
        const key = item.category || '未分类';
        acc[key] = (acc[key] || 0) + 1;
        return acc;
      }, {}),
    ).sort((a, b) => b[1] - a[1])[0];
    return {
      total,
      enabledCount,
      disabledCount,
      categoryCount,
      entityBoundCount,
      topCategoryLabel: topCategory?.[0] || '未分类',
      topCategoryCount: topCategory?.[1] || 0,
    };
  }, [processedData]);

  const buildWorkspacePath = React.useCallback((objectKey: string, functionId?: string) => {
    const search = new URLSearchParams();
    if (objectKey) search.set('objectKey', objectKey);
    if (functionId) search.set('functionId', functionId);
    search.set('from', 'functions_directory');
    return `/system/functions/workspaces?${search.toString()}`;
  }, []);

  return (
    <PageContainer
      title="函数目录"
      subTitle="函数目录负责管理原子能力，并为对象工作台提供可装配的函数供给"
      extra={headerActions}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <Card
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
              <Tag color="blue">能力供给层</Tag>
              <Tag color="green">{`可装配函数 ${summary.enabledCount}`}</Tag>
              <Tag>{`对象已绑定 ${summary.entityBoundCount}`}</Tag>
              {summary.topCategoryCount > 0 ? (
                <Tag color="purple">{`当前最大分类 ${summary.topCategoryLabel} · ${summary.topCategoryCount}`}</Tag>
              ) : null}
            </Space>
            <Space direction="vertical" size={6} style={{ width: '100%' }}>
              <Typography.Title level={4} style={{ margin: 0 }}>
                先确认函数能力，再进入对象工作台完成页面装配
              </Typography.Title>
              <Typography.Text type="secondary">
                函数目录负责原子能力供给和单函数校验，不负责最终业务页面编排。推荐顺序是：先确认 descriptor、入参与实例，再去对象工作台生成页面骨架，最后到运行控制台验证发布结果。
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
                    <Space wrap size={[8, 8]}>
                      <ProfileOutlined />
                      <Typography.Text strong>当前建议动作</Typography.Text>
                    </Space>
                    <Typography.Text type="secondary">
                      如果你已经确认函数可用，下一步应该进入对象工作台做页面骨架，而不是停留在单函数 UI 配置。
                    </Typography.Text>
                    <Space wrap size={[8, 8]}>
                      <Button
                        type="primary"
                        icon={<AppstoreOutlined />}
                        onClick={() => history.push(buildWorkspacePath('', undefined))}
                      >
                        去对象工作台
                      </Button>
                      <Button onClick={() => history.push('/system/functions/console')}>
                        去运行控制台
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
                      重点检查函数是否启用、是否有可调用实例、对象归属是否正确，以及它是否已经足够支撑后续页面装配。这里查的是能力本身，不是最终业务界面。
                    </Typography.Text>
                    <Space wrap size={[8, 8]}>
                      <Badge status="success" text="函数定义与摘要" />
                      <Badge status="processing" text="实例与调用入口" />
                      <Badge status="default" text="对象绑定与分类" />
                    </Space>
                  </Space>
                </Card>
              </Col>
            </Row>
          </Space>
        </Card>
        <SummaryOverview
          title="函数概览"
          description="这里是能力供给层。先确认函数本身是否可用，再把它们装配成业务页面。"
          items={[
            { color: '#1677ff', text: `总数 ${summary.total}` },
            { color: '#52c41a', text: `启用 ${summary.enabledCount}` },
            { color: '#d9d9d9', text: `禁用 ${summary.disabledCount}` },
            { color: '#722ed1', text: `分类 ${summary.categoryCount}` },
          ]}
          hint="函数层负责供给，对象工作台负责装配，运行控制台负责发布后的验证。"
        />

        <Alert
          type="info"
          showIcon
          message="函数详情里的 UI 配置只影响单函数表单"
          description="如果目标是做运营人员真正访问的页面，不要在函数层停太久。函数层确认能力后，应转到对象工作台完成页面骨架、预览和发布。"
          action={
            <Button
              type="primary"
              onClick={() => history.push(buildWorkspacePath('', undefined))}
            >
              去对象工作台
            </Button>
          }
        />

        <StandardListSection
          title="函数列表"
          resultText={`当前结果 ${processedData.length} 个函数`}
        >
          <ProTable<SummaryRow>
            rowKey="id"
            loading={loading}
            columns={columns}
            dataSource={processedData}
            pagination={{
              pageSize: 10,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) => `共 ${total} 个函数`,
            }}
            search={{ filterType: 'light', labelWidth: 'auto' }}
            dateFormatter="string"
            headerTitle={false}
            options={false}
            toolBarRender={false}
            sticky={false}
          />
        </StandardListSection>
      </Space>

      <Drawer
        title="函数详情"
        width={600}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
        extra={drawerActions}
      >
        {selectedFunction && (
          <Card size="small" title="基本信息">
            <Descriptions column={1} size="small">
              <Descriptions.Item label="函数ID">
                <Text code copyable>
                  {selectedFunction.id}
                </Text>
              </Descriptions.Item>
              <Descriptions.Item label="版本">
                {selectedFunction.version || <Text type="secondary">未指定</Text>}
              </Descriptions.Item>
              <Descriptions.Item label="分类">
                <Tag color={selectedFunction.category ? 'geekblue' : 'default'}>
                  {selectedFunction.category || '未分类'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="对象键">
                {selectedFunction.entity ? (
                  <Text code copyable>
                    {selectedFunction.entity}
                  </Text>
                ) : (
                  <Text type="secondary">未提供，当前将按函数 ID 推断</Text>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Badge
                  status={selectedFunction.enabled ? 'success' : 'default'}
                  text={selectedFunction.enabled ? '启用' : '禁用'}
                />
              </Descriptions.Item>
              <Descriptions.Item label="覆盖实例">
                {selectedFunction.instances !== undefined ? (
                  `${selectedFunction.instances} 个实例`
                ) : (
                  <Text type="secondary">未知</Text>
                )}
              </Descriptions.Item>
            </Descriptions>

            {(selectedFunction.displayName?.zh || selectedFunction.displayName?.en) && (
              <Card size="small" title="显示名称" style={{ marginTop: 16 }}>
                {selectedFunction.displayName?.zh || selectedFunction.displayName?.en}
              </Card>
            )}

            {(selectedFunction.summary?.zh || selectedFunction.summary?.en) && (
              <Card size="small" title="函数描述" style={{ marginTop: 16 }}>
                {selectedFunction.summary?.zh || selectedFunction.summary?.en}
              </Card>
            )}

            {selectedFunction.tags && selectedFunction.tags.length > 0 && (
              <Card size="small" title="标签" style={{ marginTop: 16 }}>
                <Space wrap>
                  {selectedFunction.tags.map((tag) => (
                    <Tag key={tag}>{tag}</Tag>
                  ))}
                </Space>
              </Card>
            )}

            {selectedFunction.menu && (
              <Card size="small" title="菜单信息" style={{ marginTop: 16 }}>
                <Descriptions column={1} size="small">
                  {Array.isArray(selectedFunction.menu.nodes) &&
                    selectedFunction.menu.nodes.length > 0 && (
                      <Descriptions.Item label="菜单节点">
                        <Space wrap>
                          {selectedFunction.menu.nodes.map((n) => (
                            <Tag key={n}>{n}</Tag>
                          ))}
                        </Space>
                      </Descriptions.Item>
                    )}
                  {selectedFunction.menu.path && (
                    <Descriptions.Item label="调用路径">
                      {buildInvokePath(selectedFunction.menu.path, selectedFunction.id)}
                    </Descriptions.Item>
                  )}
                </Descriptions>
              </Card>
            )}

            <Card size="small" style={{ marginTop: 16 }}>
              <Space wrap>
                <Button
                  onClick={() =>
                    history.push(
                      buildWorkspacePath(
                        String(
                          selectedFunction.entity || selectedFunction.id.split('.')[0] || '',
                        ).trim(),
                        selectedFunction.id,
                      ),
                    )
                  }
                >
                  去对象工作台装配
                </Button>
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  onClick={() =>
                    history.push(buildInvokePath(selectedFunction.menu?.path, selectedFunction.id))
                  }
                >
                  测试调用
                </Button>
              </Space>
            </Card>
          </Card>
        )}
      </Drawer>
    </PageContainer>
  );
}
