import React from 'react';
import { ProTable, type ProColumns } from '@ant-design/pro-components';
import type { AssignmentGroup } from './types';

type Props = {
  data: AssignmentGroup[];
  columns: ProColumns<AssignmentGroup>[];
};

export default function CategoryTab({ data, columns }: Props) {
  return (
    <ProTable<AssignmentGroup>
      rowKey="category"
      columns={columns}
      dataSource={data}
      pagination={false}
      search={false}
      toolBarRender={false}
      options={false}
    />
  );
}
