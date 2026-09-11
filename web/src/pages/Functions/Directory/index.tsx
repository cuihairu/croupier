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
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { DASHBOARD_PAGE_TOKENS, StandardListSection, SummaryOverview } from '@/components';
import type { SummaryRow } from './types';
import useDirectoryPage from './useDirectoryPage';
import { localizedText } from '@/utils/localizedText';

const { Text } = Typography;

export default function DirectoryPage() {
  const intl = useIntl();
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
    // 哨兵兜底值经 intl 求值：47/58 行的分组 key 会作为 topResourceLabel 直接
    // 展示（tag.topResource 的 {label} 插值），43 行仅用于 distinct 计数
    const undeclaredLabel = intl.formatMessage({
      id: 'pages.functionsDirectory.desc.undeclared',
      defaultMessage: '未声明',
    });
    const total = processedData.length;
    const enabledCount = processedData.filter((item) => item.enabled).length;
    const disabledCount = total - enabledCount;
    const resourceCount = new Set(processedData.map((item) => item.resource || undeclaredLabel))
      .size;
    const operationCount = processedData.filter((item) => Boolean(item.operation)).length;
    const topResource = Object.entries(
      processedData.reduce<Record<string, number>>((acc, item) => {
        const key = item.resource || undeclaredLabel;
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
      topResourceLabel: topResource?.[0] || undeclaredLabel,
      topResourceCount: topResource?.[1] || 0,
    };
  }, [intl, processedData]);

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.functionsDirectory.title',
        defaultMessage: '函数目录',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.functionsDirectory.subTitle',
        defaultMessage: '函数目录只管理原子能力契约；页面、菜单和分类在 Page Studio 中确定',
      })}
      extra={headerActions}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <Card
          styles={{
            body: {
              padding: DASHBOARD_PAGE_TOKENS.cardPadding,
              background:
                'linear-gradient(135deg, rgba(22,119,255,0.1) 0%, rgba(82,196,26,0.05) 55%, rgba(250,173,20,0.04) 100%)',
            },
          }}
        >
          <Space orientation="vertical" size={18} style={{ width: '100%' }}>
            <Space wrap size={[8, 8]}>
              <Tag color="blue">
                <FormattedMessage
                  id="pages.functionsDirectory.tag.capabilityLayer"
                  defaultMessage="能力供给层"
                />
              </Tag>
              <Tag color="green">
                {intl.formatMessage(
                  {
                    id: 'pages.functionsDirectory.tag.assemblableFunctions',
                    defaultMessage: '可装配函数 {count}',
                  },
                  { count: summary.enabledCount },
                )}
              </Tag>
              <Tag>
                {intl.formatMessage(
                  {
                    id: 'pages.functionsDirectory.tag.declaredOperations',
                    defaultMessage: '已声明操作 {count}',
                  },
                  { count: summary.operationCount },
                )}
              </Tag>
              {summary.topResourceCount > 0 ? (
                <Tag color="purple">
                  {intl.formatMessage(
                    {
                      id: 'pages.functionsDirectory.tag.topResource',
                      defaultMessage: '当前最大资源 {label} · {count}',
                    },
                    { label: summary.topResourceLabel, count: summary.topResourceCount },
                  )}
                </Tag>
              ) : null}
            </Space>
            <Space orientation="vertical" size={6} style={{ width: '100%' }}>
              <Typography.Title level={4} style={{ margin: 0 }}>
                <FormattedMessage
                  id="pages.functionsDirectory.intro.title"
                  defaultMessage="先确认函数能力，再进入 Page Studio 编排页面"
                />
              </Typography.Title>
              <Typography.Text type="secondary">
                <FormattedMessage
                  id="pages.functionsDirectory.intro.description"
                  defaultMessage="函数目录负责 descriptor、入参表单、实例覆盖和调用校验，不决定菜单、页面分类、表格、分页或多函数组合。页面发布后的菜单只来自 PublishedPageSpec。"
                />
              </Typography.Text>
            </Space>
            <Row gutter={[12, 12]}>
              <Col xs={24} lg={10}>
                <Card
                  size="small"
                  style={{ height: '100%', background: 'rgba(255,255,255,0.78)' }}
                  styles={{ body: { padding: DASHBOARD_PAGE_TOKENS.cardPadding } }}
                >
                  <Space orientation="vertical" size={8} style={{ width: '100%' }}>
                    <Space wrap size={[8, 8]}>
                      <ProfileOutlined />
                      <Typography.Text strong>
                        <FormattedMessage
                          id="pages.functionsDirectory.suggestedAction.title"
                          defaultMessage="当前建议动作"
                        />
                      </Typography.Text>
                    </Space>
                    <Typography.Text type="secondary">
                      <FormattedMessage
                        id="pages.functionsDirectory.suggestedAction.description"
                        defaultMessage="如果函数能力已经可用，下一步应到资源/页面候选中检查 PageSpec 生成质量，再进入 Page Studio 修改并发布。"
                      />
                    </Typography.Text>
                    <Space wrap size={[8, 8]}>
                      <Button
                        type="primary"
                        icon={<ApartmentOutlined />}
                        onClick={() => history.push('/functions/resource-catalog')}
                      >
                        <FormattedMessage
                          id="pages.functionsDirectory.button.viewResourceCandidates"
                          defaultMessage="查看资源/页面候选"
                        />
                      </Button>
                      <Button
                        onClick={() =>
                          history.push(
                            selectedFunction
                              ? buildInvokePath(selectedFunction.id)
                              : '/functions/invoke',
                          )
                        }
                      >
                        <FormattedMessage
                          id="pages.functionsDirectory.suggestedAction.testInvoke"
                          defaultMessage="测试函数调用"
                        />
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
                  <Space orientation="vertical" size={8} style={{ width: '100%' }}>
                    <Typography.Text strong>
                      <FormattedMessage
                        id="pages.functionsDirectory.checklist.title"
                        defaultMessage="这里适合确认什么"
                      />
                    </Typography.Text>
                    <Typography.Text type="secondary">
                      <FormattedMessage
                        id="pages.functionsDirectory.checklist.description"
                        defaultMessage="重点检查函数是否启用、是否有可调用实例、资源和操作声明是否清晰，以及是否有足够 schema 支撑后续 PageSpec 编排。"
                      />
                    </Typography.Text>
                    <Space wrap size={[8, 8]}>
                      <Badge
                        status="success"
                        text={intl.formatMessage({
                          id: 'pages.functionsDirectory.checklist.item.definition',
                          defaultMessage: '函数定义与摘要',
                        })}
                      />
                      <Badge
                        status="processing"
                        text={intl.formatMessage({
                          id: 'pages.functionsDirectory.checklist.item.instances',
                          defaultMessage: '实例与调用入口',
                        })}
                      />
                      <Badge
                        status="default"
                        text={intl.formatMessage({
                          id: 'pages.functionsDirectory.checklist.item.contract',
                          defaultMessage: '资源与操作契约',
                        })}
                      />
                    </Space>
                  </Space>
                </Card>
              </Col>
            </Row>
          </Space>
        </Card>
        <SummaryOverview
          title={intl.formatMessage({
            id: 'pages.functionsDirectory.summary.title',
            defaultMessage: '函数概览',
          })}
          description={intl.formatMessage({
            id: 'pages.functionsDirectory.summary.description',
            defaultMessage:
              '这里是能力供给层。函数注册不负责页面显示，页面编排只在 PageSpec/Page Studio 中完成。',
          })}
          items={[
            {
              color: '#1677ff',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsDirectory.summary.item.total',
                  defaultMessage: '总数 {count}',
                },
                { count: summary.total },
              ),
            },
            {
              color: '#52c41a',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsDirectory.summary.item.enabled',
                  defaultMessage: '启用 {count}',
                },
                { count: summary.enabledCount },
              ),
            },
            {
              color: '#d9d9d9',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsDirectory.summary.item.disabled',
                  defaultMessage: '禁用 {count}',
                },
                { count: summary.disabledCount },
              ),
            },
            {
              color: '#722ed1',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsDirectory.summary.item.resources',
                  defaultMessage: '资源 {count}',
                },
                { count: summary.resourceCount },
              ),
            },
          ]}
          hint={intl.formatMessage({
            id: 'pages.functionsDirectory.summary.hint',
            defaultMessage:
              '函数层负责供给，Page Studio 负责装配，运行控制台只展示已发布 PageSpec。',
          })}
        />

        <Alert
          type="info"
          showIcon
          message={intl.formatMessage({
            id: 'pages.functionsDirectory.alert.title',
            defaultMessage: '函数目录只展示能力供给，不承载页面 UI',
          })}
          description={intl.formatMessage({
            id: 'pages.functionsDirectory.alert.description',
            defaultMessage:
              '如果目标是做运营人员真正访问的页面，不要在函数层配置菜单或页面布局；请到资源/页面候选中进入 Page Studio。',
          })}
          action={
            <Button type="primary" onClick={() => history.push('/functions/resource-catalog')}>
              <FormattedMessage
                id="pages.functionsDirectory.button.viewResources"
                defaultMessage="查看资源"
              />
            </Button>
          }
        />

        <StandardListSection
          title={intl.formatMessage({
            id: 'pages.functionsDirectory.list.title',
            defaultMessage: '函数列表',
          })}
          resultText={intl.formatMessage(
            {
              id: 'pages.functionsDirectory.list.resultText',
              defaultMessage: '当前结果 {count} 个函数',
            },
            { count: processedData.length },
          )}
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
              showTotal: (total) =>
                intl.formatMessage(
                  {
                    id: 'pages.functionsDirectory.list.total',
                    defaultMessage: '共 {total} 个函数',
                  },
                  { total },
                ),
            }}
            search={{ filterType: 'light', labelWidth: 'auto' }}
            scroll={{ x: 1390 }}
            dateFormatter="string"
            headerTitle={false}
            options={false}
            toolBarRender={false}
            sticky={false}
          />
        </StandardListSection>
      </Space>

      <Drawer
        title={intl.formatMessage({
          id: 'pages.functionsDirectory.drawer.title',
          defaultMessage: '函数详情',
        })}
        width={600}
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
        extra={drawerActions}
      >
        {selectedFunction && (
          <Card
            size="small"
            title={intl.formatMessage({
              id: 'pages.functionsDirectory.drawer.basicInfo',
              defaultMessage: '基本信息',
            })}
          >
            <Descriptions column={1} size="small">
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.functionsDirectory.desc.functionId',
                  defaultMessage: '函数ID',
                })}
              >
                <Text code copyable>
                  {selectedFunction.id}
                </Text>
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.functionsDirectory.desc.version',
                  defaultMessage: '版本',
                })}
              >
                {selectedFunction.version || (
                  <Text type="secondary">
                    <FormattedMessage
                      id="pages.functionsDirectory.desc.unspecified"
                      defaultMessage="未指定"
                    />
                  </Text>
                )}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.functionsDirectory.desc.resource',
                  defaultMessage: '资源',
                })}
              >
                <Tag color={selectedFunction.resource ? 'geekblue' : 'default'}>
                  {selectedFunction.resource ||
                    intl.formatMessage({
                      id: 'pages.functionsDirectory.desc.undeclared',
                      defaultMessage: '未声明',
                    })}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.functionsDirectory.desc.operation',
                  defaultMessage: '操作',
                })}
              >
                {selectedFunction.operation ? (
                  <Text code copyable>
                    {selectedFunction.operation}
                  </Text>
                ) : (
                  <Text type="secondary">
                    <FormattedMessage
                      id="pages.functionsDirectory.desc.undeclared"
                      defaultMessage="未声明"
                    />
                  </Text>
                )}
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.functionsDirectory.desc.state',
                  defaultMessage: '状态',
                })}
              >
                <Badge
                  status={selectedFunction.enabled ? 'success' : 'default'}
                  text={intl.formatMessage({
                    id: selectedFunction.enabled
                      ? 'pages.functionsDirectory.state.enabled'
                      : 'pages.functionsDirectory.state.disabled',
                    defaultMessage: selectedFunction.enabled ? '启用' : '禁用',
                  })}
                />
              </Descriptions.Item>
              <Descriptions.Item
                label={intl.formatMessage({
                  id: 'pages.functionsDirectory.desc.instances',
                  defaultMessage: '覆盖实例',
                })}
              >
                {selectedFunction.instances !== undefined ? (
                  intl.formatMessage(
                    {
                      id: 'pages.functionsDirectory.desc.instancesCount',
                      defaultMessage: '{count} 个实例',
                    },
                    { count: selectedFunction.instances },
                  )
                ) : (
                  <Text type="secondary">
                    <FormattedMessage
                      id="pages.functionsDirectory.desc.unknown"
                      defaultMessage="未知"
                    />
                  </Text>
                )}
              </Descriptions.Item>
            </Descriptions>

            {localizedText(selectedFunction.displayName, 'zh-CN', '') && (
              <Card
                size="small"
                title={intl.formatMessage({
                  id: 'pages.functionsDirectory.drawer.displayName',
                  defaultMessage: '显示名称',
                })}
                style={{ marginTop: 16 }}
              >
                {localizedText(selectedFunction.displayName, 'zh-CN', '')}
              </Card>
            )}

            {localizedText(selectedFunction.summary, 'zh-CN', '') && (
              <Card
                size="small"
                title={intl.formatMessage({
                  id: 'pages.functionsDirectory.drawer.description',
                  defaultMessage: '函数描述',
                })}
                style={{ marginTop: 16 }}
              >
                {localizedText(selectedFunction.summary, 'zh-CN', '')}
              </Card>
            )}

            {selectedFunction.tags && selectedFunction.tags.length > 0 && (
              <Card
                size="small"
                title={intl.formatMessage({
                  id: 'pages.functionsDirectory.drawer.tags',
                  defaultMessage: '标签',
                })}
                style={{ marginTop: 16 }}
              >
                <Space wrap>
                  {selectedFunction.tags.map((tag) => (
                    <Tag key={tag}>{tag}</Tag>
                  ))}
                </Space>
              </Card>
            )}

            <Card size="small" style={{ marginTop: 16 }}>
              <Space wrap>
                <Button onClick={() => history.push('/functions/resource-catalog')}>
                  <FormattedMessage
                    id="pages.functionsDirectory.button.viewResourceCandidates"
                    defaultMessage="查看资源/页面候选"
                  />
                </Button>
                <Button
                  type="primary"
                  icon={<PlayCircleOutlined />}
                  onClick={() => history.push(buildInvokePath(selectedFunction.id))}
                >
                  <FormattedMessage
                    id="pages.functionsDirectory.button.testInvoke"
                    defaultMessage="测试调用"
                  />
                </Button>
              </Space>
            </Card>
          </Card>
        )}
      </Drawer>
    </PageContainer>
  );
}
