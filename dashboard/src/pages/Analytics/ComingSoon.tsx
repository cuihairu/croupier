import React from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Alert, Button, Card, List, Space, Typography } from 'antd';
import { LinkOutlined } from '@ant-design/icons';

type ComingSoonProps = {
  pageName: string;
  docHref?: string;
  docLabel?: string;
  description?: React.ReactNode;
  checklist?: { title: string; detail?: string }[];
  children?: React.ReactNode;
};

const defaultChecklist: { title: string; detail?: string }[] = [
  { title: '启动 ingest / analytics-worker', detail: '确保 Redis/ClickHouse 就绪，metrics 才能实时产出。' },
  { title: '在 configs/analytics/ 中配置游戏事件', detail: 'events.yaml / metrics.yaml 决定数据口径与显示。' },
  { title: '确认账号具有 canAnalyticsRead 权限', detail: 'RBAC 权限缺失会导致接口 403 或空白页面。' },
];

const AnalyticsComingSoon: React.FC<ComingSoonProps> = ({
  pageName,
  docHref = 'https://github.com/cuihairu/croupier/tree/main/docs/analytics',
  docLabel = '查看 Analytics 文档',
  description,
  checklist,
  children,
}) => {
  const list = checklist && checklist.length > 0 ? checklist : defaultChecklist;

  return (
    <PageContainer title={`${pageName}（建设中）`}>
      <Card>
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          <Typography.Title level={3}>{pageName} 即将上线</Typography.Title>
          <Typography.Paragraph type="secondary">
            {description || '后端 API 已就绪，等待产品与设计交付最终大屏。当前页面作为占位确保路由构建成功。'}
          </Typography.Paragraph>
          <Alert
            type="info"
            showIcon
            message="如何准备 Analytics 数据？"
            description="参考 docs/analytics/* 的 ClickHouse schema、事件规范与 quick-start，完成埋点与 ETL 部署后即可渲染可视化组件。"
          />
          <div>
            <Typography.Title level={5}>上线前检查清单</Typography.Title>
            <List
              dataSource={list}
              renderItem={(item) => (
                <List.Item>
                  <List.Item.Meta
                    title={<Typography.Text strong>{item.title}</Typography.Text>}
                    description={item.detail}
                  />
                </List.Item>
              )}
            />
          </div>
          <Button icon={<LinkOutlined />} type="primary" href={docHref} target="_blank" rel="noreferrer">
            {docLabel}
          </Button>
          {children && (
            <>
              <Divider />
              {children}
            </>
          )}
        </Space>
      </Card>
    </PageContainer>
  );
};

export default AnalyticsComingSoon;
