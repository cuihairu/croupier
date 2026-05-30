/**
 * 列表页生成器
 *
 * 根据配置动态生成列表页面
 * 使用 ProTable 组件
 */

import React, { useState, useMemo } from 'react';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { Button, Space, Tag, Badge, message, Popconfirm } from 'antd';
import type { ProColumns } from '@ant-design/pro-components';
import * as Icons from '@ant-design/icons';
import { useAccess } from '@umijs/max';
import type { PageConfig, ColumnConfig, RowActionConfig } from './types';
import { useDynamicData } from './hooks';

interface ListPageProps {
  config: PageConfig;
}

const ListPage: React.FC<ListPageProps> = ({ config }) => {
  const access = useAccess();
  const { data, loading, refresh } = useDynamicData(config.dataSource);
  const [selectedRows, setSelectedRows] = useState<any[]>([]);

  const { columns, actions, filters, pagination, rowActions } = config.ui.list || {};

  // 渲染列内容
  const renderColumn = (column: ColumnConfig, text: any, record: any) => {
    if (!text && text !== 0) return '-';

    switch (column.render) {
      case 'status':
        const statusConfig = column.renderConfig?.statusMap?.[text];
        if (statusConfig) {
          return <Badge status={statusConfig.status} text={statusConfig.text} />;
        }
        return <Badge status={text ? 'success' : 'default'} text={text ? '启用' : '禁用'} />;

      case 'datetime':
        const format = column.renderConfig?.format || 'YYYY-MM-DD HH:mm:ss';
        return text ? new Date(text).toLocaleString() : '-';

      case 'date':
        return text ? new Date(text).toLocaleDateString() : '-';

      case 'tag':
        const tagColor = column.renderConfig?.tagColor;
        const color = typeof tagColor === 'string' ? tagColor : tagColor?.[text];
        return <Tag color={color}>{text}</Tag>;

      case 'link':
        const href = column.renderConfig?.linkHref?.replace(/\{(\w+)\}/g, (_, key) => record[key]);
        return <a href={href}>{text}</a>;

      case 'money':
        const currency = column.renderConfig?.currency || '¥';
        return `${currency}${Number(text).toFixed(2)}`;

      default:
        return text;
    }
  };

  // 生成 ProTable 列配置
  const tableColumns: ProColumns<any>[] = useMemo(() => {
    if (!columns) return [];

    const cols: ProColumns<any>[] = columns.map((col) => ({
      title: col.title,
      dataIndex: col.key,
      key: col.key,
      width: col.width,
      sorter: col.sorter,
      copyable: col.copyable,
      ellipsis: col.ellipsis,
      render: (text: any, record: any) => renderColumn(col, text, record),
    }));

    // 添加行操作列
    if (rowActions && rowActions.length > 0) {
      cols.push({
        title: '操作',
        key: 'actions',
        fixed: 'right',
        width: 150,
        render: (_, record) => renderRowActions(record),
      });
    }

    return cols;
  }, [columns, rowActions]);

  // 渲染顶部操作按钮
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
              onClick={() => handleAction(action, null)}
            >
              {action.label}
            </Button>
          );
        })}
      </Space>
    );
  };

  // 渲染行操作
  const renderRowActions = (record: any) => {
    if (!rowActions) return null;

    return (
      <Space size="small">
        {rowActions.map((action) => {
          // 权限检查
          if (action.permission && !access[action.permission]) {
            return null;
          }

          const IconComponent = action.icon ? (Icons as any)[action.icon] : null;

          const button = (
            <Button
              key={action.key}
              type="link"
              size="small"
              danger={action.danger}
              icon={IconComponent ? <IconComponent /> : null}
              onClick={() => handleAction(action, record)}
            >
              {action.label}
            </Button>
          );

          // 如果需要确认
          if (action.confirm) {
            return (
              <Popconfirm
                key={action.key}
                title={action.confirm.title}
                description={action.confirm.content}
                onConfirm={() => handleAction(action, record)}
              >
                {button}
              </Popconfirm>
            );
          }

          return button;
        })}
      </Space>
    );
  };

  // 处理操作点击
  const handleAction = async (action: any, record: any) => {
    const handler = action.onClick;
    if (!handler) return;

    try {
      switch (handler.type) {
        case 'navigate':
          // 导航到指定路径
          const path = handler.path?.replace(/\{(\w+)\}/g, (_, key) => record?.[key] || '');
          window.location.href = path;
          break;

        case 'function':
          // 调用函数
          const { invokeFunction } = await import('@/services/api');
          await invokeFunction(handler.functionId, record || {});
          if (handler.onSuccess?.message) {
            message.success(handler.onSuccess.message);
          }
          if (handler.onSuccess?.refresh) {
            refresh();
          }
          break;

        case 'api':
          // 调用API
          const response = await fetch(handler.api.endpoint, {
            method: handler.api.method || 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ...handler.api.params, ...record }),
          });
          if (!response.ok) throw new Error('API call failed');
          if (handler.onSuccess?.message) {
            message.success(handler.onSuccess.message);
          }
          if (handler.onSuccess?.refresh) {
            refresh();
          }
          break;

        case 'modal':
          // TODO: 打开模态框
          message.info('模态框功能待实现');
          break;

        default:
          console.warn('Unknown action type:', handler.type);
      }
    } catch (error: any) {
      message.error(error.message || '操作失败');
      console.error('Action error:', error);
    }
  };

  return (
    <PageContainer title={config.title} extra={renderActions()}>
      <ProTable
        columns={tableColumns}
        dataSource={data}
        loading={loading}
        rowKey="id"
        search={false}
        pagination={
          pagination !== false
            ? {
                pageSize: 10,
                showSizeChanger: true,
                showQuickJumper: true,
              }
            : false
        }
        rowSelection={
          selectedRows
            ? {
                selectedRowKeys: selectedRows.map((row) => row.id),
                onChange: (_, rows) => setSelectedRows(rows),
              }
            : undefined
        }
        toolBarRender={() => [
          <Button key="refresh" onClick={refresh}>
            刷新
          </Button>,
        ]}
      />
    </PageContainer>
  );
};

export default ListPage;
