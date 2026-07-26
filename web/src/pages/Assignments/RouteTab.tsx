import React from 'react';
import { Alert } from 'antd';
import { ProTable, type ProColumns } from '@ant-design/pro-components';
import type { AssignmentItem } from './types';

type Props = {
  data: AssignmentItem[];
  columns: ProColumns<AssignmentItem>[];
};

export default function RouteTab({ data, columns }: Props) {
  return (
    <ProTable<AssignmentItem>
      rowKey="id"
      columns={columns}
      dataSource={data}
      pagination={{ pageSize: 10 }}
      search={false}
      toolBarRender={() => [
        <Alert
          key="hint"
          message="函数能力归属说明"
          description="这里只展示已分配函数的 resource/operation 能力归属。菜单、分类和页面标题必须在 PageSpec/Page Studio 中确定。"
          type="info"
          showIcon
        />,
      ]}
    />
  );
}
