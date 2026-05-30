import React from 'react';
import { Alert } from 'antd';
import type { RendererProps } from './types';

type CustomLayout = {
  type: 'custom';
  component?: string;
  props?: Record<string, any>;
};

export default function CustomRenderer({ layout }: RendererProps<CustomLayout>) {
  return (
    <Alert
      type="info"
      showIcon
      message={`自定义布局组件: ${layout?.component || '未设置'}`}
      description={
        <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {JSON.stringify(layout?.props || {}, null, 2)}
        </pre>
      }
    />
  );
}
