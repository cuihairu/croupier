import { history, useIntl } from '@umijs/max';
import { Button } from 'antd';
import React from 'react';
import { PageStatePanel } from '@/components';

const NoFoundPage: React.FC = () => {
  const intl = useIntl();
  return (
    <div style={{ padding: 24 }}>
      <PageStatePanel
        tone="warning"
        badgeText="404"
        title="当前页面不存在"
        description={intl.formatMessage({ id: 'pages.404.subTitle' })}
        actions={
      <Button type="primary" onClick={() => history.push('/')}>
        {intl.formatMessage({ id: 'pages.404.buttonText' })}
      </Button>
        }
      />
    </div>
  );
};

export default NoFoundPage;
