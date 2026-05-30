import React from 'react';
import { Card, Divider, Space, Tag } from 'antd';
import { ProTable, type ProColumns } from '@ant-design/pro-components';
import type { AssignmentGroup, AssignmentItem } from './types';

type Props = {
  groupedAssignments: AssignmentGroup[];
  selected: string[];
  columns: ProColumns<AssignmentItem>[];
  toolbarActions: React.ReactNode[];
  renderCategoryActions: (category: string, size?: 'small' | 'middle') => React.ReactNode;
  onSelectionChange: (keys: React.Key[]) => void;
};

export default function ListTab({
  groupedAssignments,
  selected,
  columns,
  toolbarActions,
  renderCategoryActions,
  onSelectionChange,
}: Props) {
  return (
    <>
      <Space style={{ marginBottom: 16, width: '100%' }} wrap>
        {toolbarActions}
        <Divider type="vertical" />
        {groupedAssignments.map((group) => (
          <Space key={group.category} style={{ marginRight: 16 }}>
            <span>{group.category}:</span>
            {renderCategoryActions(group.category, 'small')}
          </Space>
        ))}
      </Space>

      {groupedAssignments.map((group) => (
        <Card
          key={group.category}
          type="inner"
          title={
            <Space>
              <span>{group.category}</span>
              <Tag color="blue">{group.items.length} 个函数</Tag>
              <Tag color="green">{group.activeCount} 已启用</Tag>
              <Tag color="orange">{group.canaryCount} 灰度中</Tag>
            </Space>
          }
          style={{ marginBottom: 16 }}
          extra={<Space>{renderCategoryActions(group.category, 'small')}</Space>}
        >
          <ProTable<AssignmentItem>
            rowKey="id"
            columns={columns}
            dataSource={group.items}
            pagination={false}
            search={false}
            toolBarRender={false}
            options={false}
            rowSelection={{
              type: 'checkbox',
              selectedRowKeys: selected,
              onChange: onSelectionChange,
            }}
          />
        </Card>
      ))}
    </>
  );
}
