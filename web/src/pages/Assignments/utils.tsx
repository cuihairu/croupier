import React from 'react';
import { Space, Tag } from 'antd';
import { getIntl } from '@umijs/max';

export const formatDateTime = (value?: string | number | Date) => {
  if (value === null || value === undefined || value === '') return '-';
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? '-' : value.toLocaleString('zh-CN');
  }
  if (typeof value === 'number') {
    const ts = value < 1e12 ? value * 1000 : value;
    const d = new Date(ts);
    return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-CN');
  }

  const raw = String(value).trim();
  if (!raw) return '-';
  if (/^\d+$/.test(raw)) {
    const n = Number(raw);
    const ts = n < 1e12 ? n * 1000 : n;
    const d = new Date(ts);
    return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-CN');
  }

  const d = new Date(raw);
  return Number.isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-CN');
};

export const renderHistoryDetail = (key: string, value: unknown) => {
  if (Array.isArray(value)) {
    if (value.length === 0) return <Tag>0</Tag>;
    if (value.length <= 8) {
      return (
        <Space wrap>
          {value.map((x, idx) => (
            <Tag key={`${key}-${idx}`}>{String(x)}</Tag>
          ))}
        </Space>
      );
    }
    // 展示文案经 getIntl() 在调用时解析（非组件模块无 intl 上下文，先例 Ops/Terms）
    return (
      <Tag>
        {getIntl().formatMessage(
          { id: 'pages.assignments.history.itemCount', defaultMessage: `${value.length} 项` },
          { count: value.length },
        )}
      </Tag>
    );
  }
  if (value && typeof value === 'object') {
    return <pre style={{ margin: 0 }}>{JSON.stringify(value, null, 2)}</pre>;
  }
  return String(value);
};
