/**
 * 详情页生成器
 *
 * 根据配置动态生成详情页面
 * 使用 ProDescriptions 组件
 */

import React from 'react';
import { PageContainer, ProDescriptions } from '@ant-design/pro-components';
import { Card, Space, Button, Tag, Badge } from 'antd';
import type { ProDescriptionsItemProps } from '@ant-design/pro-components';
import * as Icons from '@ant-design/icons';
import { useAccess } from '@umijs/max';
import type { PageConfig, DetailFieldConfig } from './types';
import { useDynamicData } from './hooks';

interface DetailPageProps {
  config: PageConfig;
}

const DetailPage: React.FC<DetailPageProps> = ({ config }) => {
  const access = useAccess();
  const { data, loading } = useDynamicData(config.dataSource);
  const record = data[0] || {};

  const { sections, column, actions } = config.ui.detail || {};

  // 渲染字段内容
  const renderField = (field: DetailFieldConfig, value: any) => {
    if (!value && value !== 0) return '-';

    switch (field.render) {
      case 'status':
        const statusConfig = field.renderConfig?.statusMap?.[value];
        if (statusConfig) {
          return <Badge status={statusConfig.status} text={statusConfig.text} />;
        }
        return <Badge status={value ? 'success' : 'default'} text={value ? '启用' : '禁用'} />;

      case 'datetime':
        const format = field.renderConfig?.format || 'YYYY-MM-DD HH:mm:ss';
        return value ? new Date(value).toLocaleString() : '-';

      case 'date':
        return value ? new Date(value).toLocaleDateString() : '-';

      case 'tag':
        const tagColor = field.renderConfig?.tagColor;
        const color = typeof tagColor === 'string' ? tagColor : tagColor?.[value];
        return <Tag color={color}>{value}</Tag>;

      case 'link':
        const href = field.renderConfig?.linkHref?.replace(/\{(\w+)\}/g, (_, key) => record[key]);
        return <a href={href}>{value}</a>;

      case 'money':
        const currency = field.renderConfig?.currency || '¥';
        return `${currency}${Number(value).toFixed(2)}`;

      default:
        return value;
    }
  };

  // 渲染操作按钮
  const renderActions = () => {
    if (!actions || actions.length === 0) return null;

    return (
      <Space>
        {actions.map((action) => {
          // 权限检查
          if (action.permission && !access[action.permission]) {
            return null;
          }

          const IconComponent = action.icon ? (Icons as any)[action.icon] : null;

          return (
            <Button
              key={action.key}
              type={action.type || 'default'}
              danger={action.danger}
              icon={IconComponent ? <IconComponent /> : null}
              onClick={() => handleAction(action)}
            >
              {action.label}
            </Button>
          );
        })}
      </Space>
    );
  };

  // 处理操作点击
  const handleAction = (action: any) => {
    // TODO: 实现操作处理逻辑
    console.log('Action clicked:', action);
  };

  return (
    <PageContainer title={config.title} extra={renderActions()} loading={loading}>
      {sections?.map((section, index) => (
        <Card key={index} title={section.title} style={{ marginBottom: 16 }} bordered={false}>
          <ProDescriptions column={column || 2} dataSource={record}>
            {section.fields.map((field) => (
              <ProDescriptions.Item
                key={field.key}
                label={field.label}
                dataIndex={field.key}
                span={field.span}
                copyable={field.copyable}
                render={(text) => renderField(field, text)}
              />
            ))}
          </ProDescriptions>
        </Card>
      ))}
    </PageContainer>
  );
};

export default DetailPage;
