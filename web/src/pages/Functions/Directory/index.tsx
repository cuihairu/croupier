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
import { ApartmentOutlined, PlayCircleOutlined, ProfileOutlined } from '@ant-design/icons';
import { history } from '@umijs/max';
import { DASHBOARD_PAGE_TOKENS, StandardListSection, SummaryOverview } from '@/components';
import type { SummaryRow } from './types';
import useDirectoryPage from './useDirectoryPage';
import { localizedText } from '@/utils/localizedText';

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
    const resourceCount = new Set(processedData.map((item) => item.resource || '未声明')).size;
    const operationCount = processedData.filter((item) => Boolean(item.operation)).length;
    const topResource = Object.entries(
      processedData.reduce<Record<string, number>>((acc, item) => {
        const key = item.resource || '未声明';
        acc[key] = (acc[key] || 0) + 1;
        return acc;
      }, {}),
    ).sort((a, b) => b[1] - a[1])[0];
    return {
      total,
      enabledCount,
      disabledCount,
      resourceCount,
      operationCount,
      topResourceLabel: topResource?.[0] || '未声明',
      topResourceCount: topResource?.[1] || 0,
    };
  }, [processedData]);

  return (
    <PageContainer
      title="函数目录"
      subTitle="函数目录只管理原子能力契约；页面、菜单和分类在 Page Studio 中确定"
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
              <Tag>{`已声明操作 ${summary.operationCount}`}</Tag>
              {summary.topResourceCount > 0 ? (
                <Tag color="purple">{`当前最大资源 ${summary.topResourceLabel} · ${summary.topResourceCount}`}</Tag>
              ) : null}
            </Space>
            <Space direction="vertical" size={6} style={{ width: '100%' }}>
              <Typography.Title level={4} style={{ margin: 0 }}>
                先确认函数能力，再进入 Page Studio 编排页面
              </Typography.Title>
              <Typography.Text type="secondary">
                函数目录负责
                descriptor、入参表单、实例覆盖和调用校验，不决定菜单、页面分类、表格、分页或多函数组合。页面发布后的菜单只来自
                PublishedPageSpec。
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
                      如果函数能力已经可用，下一步应到资源/页面候选中检查 PageSpec 生成质量，再进入
                      Page Studio 修改并发布。
                    </Typography.Text>
                    <Space wrap size={[8, 8]}>
                      <Button
                        type="primary"
                        icon={<ApartmentOutlined />}
                        onClick={() => history.push('/system/functions/resource-catalog')}
                      >
                        查看资源/页面候选
                      </Button>
                      <Button
                        onClick={() =>
                          history.push(
                            selectedFunction
                              ? buildInvokePath(selectedFunction.id)
                              : '/system/functions/invoke',
                          )
                        }
                      >
                        测试函数调用
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
                      重点检查函数是否启用、是否有可调用实例、资源和操作声明是否清晰，以及是否有足够
                      schema 支撑后续 PageSpec 编排。
                    </Typography.Text>
                    <Space wrap size={[8, 8]}>
                      <Badge status="success" text="函数定义与摘要" />
                      <Badge status="processing" text="实例与调用入口" />
                      <Badge status="default" text="资源与操作契约" />
                    </Space>
                  </Space>
                </Card>
              </Col>
            </Row>
          </Space>
        </Card>
        <SummaryOverview
          title="函数概览"
          description="这里是能力供给层。函数注册不负责页面显示，页面编排只在 PageSpec/Page Studio 中完成。"
          items={[
            { color: '#1677ff', text: `总数 ${summary.total}` },
            { color: '#52c41a', text: `启用 ${summary.enabledCount}` },
            { color: '#d9d9d9', text: `禁用 ${summary.disabledCount}` },
            { color: '#722ed1', text: `资源 ${summary.resourceCount}` },
          ]}
          hint="函数层负责供给，Page Studio 负责装配，运行控制台只展示已发布 PageSpec。"
        />

        <Alert
          type="info"
          showIcon
          message="函数目录只展示能力供给，不承载页面 UI"
          description="如果目标是做运营人员真正访问的页面，不要在函数层配置菜单或页面布局；请到资源/页面候选中进入 Page Studio。"
          action={
            <Button
              type="primary"
              onClick={() => history.push('/system/functions/resource-catalog')}
            >
              查看资源
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
              <Descriptions.Item label="资源">
                <Tag color={selectedFunction.resource ? 'geekblue' : 'default'}>
                  {selectedFunction.resource || '未声明'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="操作">
                {selectedFunction.operation ? (
                  <Text code copyable>
                    {selectedFunction.operation}
                  </Text>
                ) : (
                  <Text type="secondary">未声明</Text>
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

            {localizedText(selectedFunction.displayName, 'zh-CN', '') && (
              <Card size="small" title="显示名称" style={{ marginTop: 16 }}>
                {localizedText(selectedFunction.displayName, 'zh-CN', '')}
              </Card>
            )}

            {localizedText(selectedFunction.summary, 'zh-CN', '') && (
              <Card size="small" title="函数描述" style={{ marginTop: 16 }}>
                {localizedText(selectedFunction.summary, 'zh-CN', '')}
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

            <Card size="small" style={{ marginTop: 16 }}>
              <Space wrap>
                <Button onClick={() => history.push('/system/functions/resource-catalog')}>
                  查看资源/页面候选
                </Button>
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  onClick={() => history.push(buildInvokePath(selectedFunction.id))}
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
