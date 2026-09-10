import React from 'react';
import { Alert, Space, Tag, Typography } from 'antd';
import type { Diagnostic } from '@/types/dashboard';
import { diagnosticAlertType, diagnosticColor } from './shared';

const { Text } = Typography;

/** 诊断列表（详情抽屉形态）：severity Tag + code + field，description 展示信息。 */
export default function DiagnosticsList({ items }: { items: Diagnostic[] }) {
  return (
    <Space orientation="vertical" size={8} style={{ width: '100%' }}>
      {items.map((item) => (
        <Alert
          key={`${item.code}:${item.field || ''}:${item.message}`}
          type={diagnosticAlertType(item.severity)}
          showIcon
          message={
            <Space>
              <Tag color={diagnosticColor(item.severity)}>{item.severity}</Tag>
              <Text code>{item.code}</Text>
              {item.field ? <Text>{item.field}</Text> : null}
            </Space>
          }
          description={item.message}
        />
      ))}
    </Space>
  );
}
