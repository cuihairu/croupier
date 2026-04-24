import React from 'react';
import { Button, Descriptions, List, Modal, Select, Space, Tag } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { AssignmentHistory, HistoryAction } from './types';
import { HISTORY_ACTION_OPTIONS } from './constants';
import { formatDateTime, renderHistoryDetail } from './utils';

type Props = {
  visible: boolean;
  history: AssignmentHistory[];
  loading: boolean;
  page: number;
  pageSize: number;
  total: number;
  actionFilter: HistoryAction;
  onClose: () => void;
  onActionFilterChange: (action: HistoryAction) => void;
  onReload: () => void;
  onPageChange: (page: number, pageSize: number) => void;
};

export default function HistoryModal({
  visible,
  history,
  loading,
  page,
  pageSize,
  total,
  actionFilter,
  onClose,
  onActionFilterChange,
  onReload,
  onPageChange,
}: Props) {
  return (
    <Modal
      title="分配变更历史"
      open={visible}
      onCancel={onClose}
      width={800}
      footer={[
        <Button key="close" onClick={onClose}>
          关闭
        </Button>,
      ]}
    >
      <Space style={{ marginBottom: 12 }}>
        <span>动作筛选:</span>
        <Select
          value={actionFilter}
          style={{ width: 160 }}
          onChange={(v) => onActionFilterChange(v as HistoryAction)}
          options={HISTORY_ACTION_OPTIONS}
        />
        <Button icon={<ReloadOutlined />} onClick={onReload}>
          刷新
        </Button>
      </Space>
      <List
        loading={loading}
        dataSource={history}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: onPageChange,
        }}
        renderItem={(item) => (
          <List.Item>
            <List.Item.Meta
              title={
                <Space>
                  <Tag
                    color={
                      item.action === 'assign' ? 'green' : item.action === 'clone' ? 'blue' : 'red'
                    }
                  >
                    {item.action === 'assign' ? '分配' : item.action === 'clone' ? '克隆' : '移除'}
                  </Tag>
                  <span>{item.function_id}</span>
                  <span>({item.count} 个函数)</span>
                </Space>
              }
              description={
                <Space direction="vertical" style={{ width: '100%' }}>
                  <span>
                    操作人: {item.operated_by} | 时间: {formatDateTime(item.operated_at)}
                  </span>
                  {item.details && (
                    <Descriptions size="small" column={1} bordered>
                      {Object.entries(item.details).map(([k, v]) => (
                        <Descriptions.Item key={k} label={k}>
                          {renderHistoryDetail(k, v)}
                        </Descriptions.Item>
                      ))}
                    </Descriptions>
                  )}
                </Space>
              }
            />
          </List.Item>
        )}
      />
    </Modal>
  );
}
