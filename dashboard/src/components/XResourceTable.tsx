import React, { useRef, ReactNode } from 'react';
import { Button, Space, Modal, App } from 'antd';
import { ProTable, ProColumns } from '@ant-design/pro-components';
import { PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons';

export interface XResourceTableProps<T = any> {
  // Data props
  dataSource: T[];
  loading?: boolean;
  rowKey: string | ((record: T) => string);

  // Column configuration
  columns: ProColumns<T>[];

  // Actions
  onAdd?: () => void;
  onEdit?: (record: T) => void;
  onDelete?: (record: T) => void;
  onPreview?: (record: T) => void;

  // Customization
  title?: string;
  addButtonText?: string;
  deleteConfirmTitle?: string;
  getDeleteConfirmContent?: (record: T) => string;

  // Table props
  search?: boolean;
  pagination?: any;
  toolBarRender?: () => ReactNode[];

  // Permissions
  canAdd?: boolean;
  canEdit?: boolean;
  canDelete?: boolean;
  canPreview?: boolean;
}

export default function XResourceTable<T = any>({
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
}: XResourceTableProps<T>) {
  const tableRef = useRef<any>();
  // Use App context message API to avoid React 18 concurrent-mode warnings
  const { message } = App.useApp();

  // Build action column if any action is enabled
  const shouldShowActions = (canEdit && onEdit) || (canDelete && onDelete) || (canPreview && onPreview);

  const handleDelete = (record: T) => {
    if (!onDelete) return;

    const content = getDeleteConfirmContent ? getDeleteConfirmContent(record) : 'Are you sure you want to delete this item?';

    Modal.confirm({
      title: deleteConfirmTitle,
      content,
      okText: 'Delete',
      okType: 'danger',
      onOk: async () => {
        try {
          await onDelete(record);
          message.success('Item deleted successfully');
        } catch (error: any) {
          message.error(error?.message || 'Failed to delete item');
        }
      },
    });
  };

  // Enhanced columns with actions
  const enhancedColumns: ProColumns<T>[] = [
    ...baseColumns,
    ...(shouldShowActions ? [{
      title: 'Actions',
      key: 'actions',
      width: 200,
      render: (_: any, record: T) => (
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
    }] : [])
  ];

  // Default toolbar render
  const defaultToolBarRender = () => {
    const actions: ReactNode[] = [];

    if (canAdd && onAdd) {
      actions.push(
        <Button
          key="add"
          type="primary"
          icon={<PlusOutlined />}
          onClick={onAdd}
        >
          {addButtonText}
        </Button>
      );
    }

    return actions;
  };

  return (
    <ProTable<T>
      actionRef={tableRef}
      columns={enhancedColumns}
      dataSource={dataSource}
      loading={loading}
      rowKey={rowKey}
      search={search}
      pagination={pagination}
      toolBarRender={toolBarRender || (canAdd && onAdd ? defaultToolBarRender : false)}
      headerTitle={title}
    />
  );
}

// Type helper for better TypeScript support
export type XResourceTableRef = React.RefObject<any>;
