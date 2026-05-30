import { history, useIntl } from '@umijs/max';
import { Button } from 'antd';
import React from 'react';
import { PageStatePanel } from '@/components';

const ForbiddenPage: React.FC = () => (
  <div style={{ padding: 24 }}>
    <PageStatePanel
      tone="error"
      badgeText="403"
      title="当前页面无访问权限"
      description="抱歉，您没有权限访问此页面。请返回首页或切换到具备权限的入口。"
      actions={
      <Button type="primary" onClick={() => history.push('/')}>
        返回首页
      </Button>
      }
    />
  </div>
);

export default ForbiddenPage;
