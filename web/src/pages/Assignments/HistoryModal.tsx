import SimpleList from '@/components/SimpleList';
import React from 'react';
import { Button, Descriptions, Modal, Select, Space, Tag } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { AssignmentHistory, HistoryAction } from './types';
import { HISTORY_ACTION_OPTIONS } from './constants';
import { formatDateTime, renderHistoryDetail, renderHistoryDiff } from './utils';

// #37：details 中已由 diff 摘要渲染的已知 key；其余 key（未来扩展）仍走
// 原始键值兜底，避免静默吞掉新字段。
const DIFF_DETAIL_KEYS = new Set(['before', 'after', 'added', 'removed', 'unknown']);

type Props = {
  open: boolean;
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
  open,
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
  const intl = useIntl();
  return (
    <Modal
      title={intl.formatMessage({
        id: 'pages.assignments.history.title',
        defaultMessage: '分配变更历史',
      })}
      open={open}
      onCancel={onClose}
      width={800}
      footer={[
        <Button key="close" onClick={onClose}>
          <FormattedMessage id="pages.assignments.history.close" defaultMessage="关闭" />
        </Button>,
      ]}
    >
      <Space style={{ marginBottom: 12 }}>
        <span>
          <FormattedMessage
            id="pages.assignments.history.actionFilter"
            defaultMessage="动作筛选:"
          />
        </span>
        <Select
          value={actionFilter}
          style={{ width: 160 }}
          onChange={(v) => onActionFilterChange(v as HistoryAction)}
          options={HISTORY_ACTION_OPTIONS}
        />
        <Button icon={<ReloadOutlined />} onClick={onReload}>
          <FormattedMessage id="pages.assignments.history.reload" defaultMessage="刷新" />
        </Button>
      </Space>
      <SimpleList
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
          <SimpleList.Item>
            <SimpleList.Item.Meta
              title={
                <Space>
                  <Tag
                    color={
                      item.action === 'assign' ? 'green' : item.action === 'clone' ? 'blue' : 'red'
                    }
                  >
                    {item.action === 'assign'
                      ? intl.formatMessage({
                          id: 'pages.assignments.history.action.assign',
                          defaultMessage: '分配',
                        })
                      : item.action === 'clone'
                        ? intl.formatMessage({
                            id: 'pages.assignments.history.action.clone',
                            defaultMessage: '克隆',
                          })
                        : intl.formatMessage({
                            id: 'pages.assignments.history.action.remove',
                            defaultMessage: '移除',
                          })}
                  </Tag>
                  <span>
                    {item.gameId}/{item.env}
                  </span>
                </Space>
              }
              description={
                <Space orientation="vertical" style={{ width: '100%' }}>
                  <span>
                    {intl.formatMessage(
                      {
                        id: 'pages.assignments.history.operatedMeta',
                        defaultMessage: `操作人: ${item.operatedBy} | 时间: ${formatDateTime(item.operatedAt)}`,
                      },
                      { operator: item.operatedBy, time: formatDateTime(item.operatedAt) },
                    )}
                  </span>
                  {renderHistoryDiff(item.details)}
                  {item.details &&
                    Object.entries(item.details).filter(([k]) => !DIFF_DETAIL_KEYS.has(k)).length >
                      0 && (
                      <Descriptions size="small" column={1} bordered>
                        {Object.entries(item.details)
                          .filter(([k]) => !DIFF_DETAIL_KEYS.has(k))
                          .map(([k, v]) => (
                            <Descriptions.Item key={k} label={k}>
                              {renderHistoryDetail(k, v)}
                            </Descriptions.Item>
                          ))}
                      </Descriptions>
                    )}
                </Space>
              }
            />
          </SimpleList.Item>
        )}
      />
    </Modal>
  );
}
