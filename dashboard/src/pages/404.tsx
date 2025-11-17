import React from 'react';
import { Result, Button } from 'antd';
import { history } from '@umijs/max';

export default function NotFound() {
  return (
    <Result
      status="404"
      title="404"
      subTitle="页面不存在或仍在建设中。"
      extra={<Button type="primary" onClick={() => history.push('/')}>返回首页</Button>}
    />
  );
}
