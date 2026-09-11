import { FormattedMessage, history } from '@umijs/max';
import { Button } from 'antd';
import React from 'react';
import { PageStatePanel } from '@/components';

const ForbiddenPage: React.FC = () => (
  <div style={{ padding: 24 }}>
    <PageStatePanel
      tone="error"
      badgeText="403"
      title={<FormattedMessage id="pages.403.title" defaultMessage="当前页面无访问权限" />}
      description={
        <FormattedMessage
          id="pages.403.subTitle"
          defaultMessage="抱歉，您没有权限访问此页面。请返回首页或切换到具备权限的入口。"
        />
      }
      actions={
        <Button type="primary" onClick={() => history.push('/')}>
          <FormattedMessage id="pages.403.buttonText" defaultMessage="返回首页" />
        </Button>
      }
    />
  </div>
);

export default ForbiddenPage;
