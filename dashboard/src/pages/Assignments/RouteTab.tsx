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
          message="分配后路由展示说明"
          description="这里显示的是函数描述符中的 menu 路由信息（nodes/path）。如需调整请点击“编辑路由”。"
          type="info"
          showIcon
        />,
      ]}
    />
  );
}
