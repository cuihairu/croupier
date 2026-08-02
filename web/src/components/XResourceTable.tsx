import React, { ReactNode } from 'react';
import { Button, Space, Modal, App } from 'antd';
import { ProTable, ProColumns } from '@ant-design/pro-components';
import { PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons';
import type { TablePaginationConfig } from 'antd/es/table';
import type { JSONValue } from '@/types/dashboard';

type ResourceRow = Record<string, JSONValue>;

export interface XResourceTableProps {
  // Data props
  dataSource: ResourceRow[];
  loading?: boolean;
  rowKey: string | ((record: ResourceRow) => string);

  // Column configuration
  columns: ProColumns<ResourceRow>[];

  // Actions
  onAdd?: () => void;
  onEdit?: (record: ResourceRow) => void;
  onDelete?: (record: ResourceRow) => void;
  onPreview?: (record: ResourceRow) => void;

  // Customization
  title?: string;
  addButtonText?: string;
  deleteConfirmTitle?: string;
  getDeleteConfirmContent?: (record: ResourceRow) => string;

  // Table props
  search?: boolean;
  pagination?: TablePaginationConfig | false;
  toolBarRender?: () => ReactNode[];

  // Permissions
  canAdd?: boolean;
  canEdit?: boolean;
  canDelete?: boolean;
  canPreview?: boolean;
}

export default function XResourceTable({
  dataSource,
  loading = false,
  rowKey,
  columns: baseColumns,
  onAdd,
  onEdit,
  onDelete,
  onPreview,
  title,
  addButtonText = 'Add New',
  deleteConfirmTitle = 'Delete Confirmation',
  getDeleteConfirmContent,
  search = false,
  pagination = {
    showSizeChanger: true,
    showQuickJumper: true,
  },
  toolBarRender,
  canAdd = true,
  canEdit = true,
  canDelete = true,
  canPreview = true,
}: XResourceTableProps) {
  // Use App context message API to avoid React 18 concurrent-mode warnings
  const { message } = App.useApp();

  // Build action column if any action is enabled
  const shouldShowActions =
    (canEdit && onEdit) || (canDelete && onDelete) || (canPreview && onPreview);

  const handleDelete = (record: ResourceRow) => {
    if (!onDelete) return;

    const content = getDeleteConfirmContent
      ? getDeleteConfirmContent(record)
      : 'Are you sure you want to delete this item?';

    Modal.confirm({
      title: deleteConfirmTitle,
      content,
      okText: 'Delete',
      okType: 'danger',
      onOk: async () => {
        try {
          await onDelete(record);
          message.success('Item deleted successfully');
        } catch (error) {
          message.error(error instanceof Error ? error.message : 'Failed to delete item');
        }
      },
    });
  };

  // Enhanced columns with actions
  const enhancedColumns: ProColumns<ResourceRow>[] = [
    ...baseColumns,
    ...(shouldShowActions
      ? [
          {
            title: 'Actions',
            key: 'actions',
            width: 200,
            render: (_value: React.ReactNode, record: ResourceRow) => (
              <Space size="small">
                {canPreview && onPreview && (
                  <Button
                    key="preview"
                    icon={<EyeOutlined />}
                    size="small"
                    onClick={() => onPreview(record)}
                    title="Preview"
                  />
                )}
                {canEdit && onEdit && (
                  <Button
                    key="edit"
                    icon={<EditOutlined />}
                    size="small"
                    onClick={() => onEdit(record)}
                    title="Edit"
                  />
                )}
                {canDelete && onDelete && (
                  <Button
                    key="delete"
                    icon={<DeleteOutlined />}
                    size="small"
                    danger
                    onClick={() => handleDelete(record)}
                    title="Delete"
                  />
                )}
              </Space>
            ),
          },
        ]
      : []),
  ];

  // Default toolbar render
  const defaultToolBarRender = () => {
    const actions: ReactNode[] = [];

    if (canAdd && onAdd) {
      actions.push(
        <Button key="add" type="primary" icon={<PlusOutlined />} onClick={onAdd}>
          {addButtonText}
        </Button>,
      );
    }

    return actions;
  };

  return (
    <ProTable<ResourceRow>
      columns={enhancedColumns}
      dataSource={dataSource}
      loading={loading}
      rowKey={rowKey}
      search={search ? { labelWidth: 'auto' } : false}
      pagination={pagination}
      toolBarRender={toolBarRender || (canAdd && onAdd ? defaultToolBarRender : false)}
      headerTitle={title}
    />
  );
}

// Type helper for better TypeScript support
export type XResourceTableRef = React.RefObject<unknown>;
