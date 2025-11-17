import React from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Card, Typography } from 'antd';

type PlaceholderPageProps = {
  title: string;
  description?: string;
};

const PlaceholderPage: React.FC<PlaceholderPageProps> = ({ title, description }) => (
  <PageContainer>
    <Card>
      <Typography.Title level={3}>{title}</Typography.Title>
      <Typography.Paragraph type="secondary">
        {description || '功能开发中，敬请期待。'}
      </Typography.Paragraph>
    </Card>
  </PageContainer>
);

export default PlaceholderPage;
