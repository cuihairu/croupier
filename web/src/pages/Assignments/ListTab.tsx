import React from 'react';
import { Card, Divider, Space, Tag } from 'antd';
import { ProTable, type ProColumns } from '@ant-design/pro-components';
import { FormattedMessage } from '@umijs/max';
import type { AssignmentGroup, AssignmentItem } from './types';

type Props = {
  groupedAssignments: AssignmentGroup[];
  selected: string[];
  columns: ProColumns<AssignmentItem>[];
  toolbarActions: React.ReactNode[];
  renderResourceActions: (resource: string, size?: 'small' | 'middle') => React.ReactNode;
  onSelectionChange: (keys: React.Key[]) => void;
};

export default function ListTab({
  groupedAssignments,
  selected,
  columns,
  toolbarActions,
  renderResourceActions,
  onSelectionChange,
}: Props) {
  return (
    <>
      <Space style={{ marginBottom: 16, width: '100%' }} wrap>
        {toolbarActions}
        <Divider type="vertical" />
        {groupedAssignments.map((group) => (
          <Space key={group.resource} style={{ marginRight: 16 }}>
            <span>{group.resource}:</span>
            {renderResourceActions(group.resource, 'small')}
          </Space>
        ))}
      </Space>

      {groupedAssignments.map((group) => (
        <Card
          key={group.resource}
          type="inner"
          title={
            <Space>
              <span>{group.resource}</span>
              <Tag color="blue">
                <FormattedMessage
                  id="pages.assignments.group.functionCount"
                  defaultMessage={`${group.items.length} 个函数`}
                  values={{ count: group.items.length }}
                />
              </Tag>
              <Tag color="green">
                <FormattedMessage
                  id="pages.assignments.group.activeCount"
                  defaultMessage={`${group.activeCount} 已启用`}
                  values={{ count: group.activeCount }}
                />
              </Tag>
              <Tag color="orange">
                <FormattedMessage
                  id="pages.assignments.group.canaryCount"
                  defaultMessage={`${group.canaryCount} 灰度中`}
                  values={{ count: group.canaryCount }}
                />
              </Tag>
            </Space>
          }
          style={{ marginBottom: 16 }}
          extra={<Space>{renderResourceActions(group.resource, 'small')}</Space>}
        >
          <ProTable<AssignmentItem>
            scroll={{ x: 1300 }}
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
