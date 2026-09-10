import { Button, Drawer, Empty, Popconfirm, Space, Tag, Typography } from 'antd';
import { ProTable } from '@ant-design/pro-components';
import type { PageVersionItem } from '@/types/dashboard';
import { formatDate } from './shared';

const { Paragraph, Text } = Typography;

/** 版本历史抽屉：草稿/发布版本列表 + 回滚（Popconfirm 二次确认）。
 * 数据加载与分页由工作台主页回调（保持 loadVersionHistory 单一来源）。 */
export default function VersionsDrawer({
  open,
  pageKey,
  items,
  loading,
  page,
  pageSize,
  total,
  currentDraftVersion,
  currentPublishedVersion,
  onClose,
  onPageChange,
  onRollbackDraft,
  onRollbackPublished,
}: {
  open: boolean;
  pageKey: string;
  items: PageVersionItem[];
  loading: boolean;
  page: number;
  pageSize: number;
  total: number;
  currentDraftVersion: number;
  currentPublishedVersion: number;
  onClose: () => void;
  onPageChange: (page: number, pageSize: number) => void;
  onRollbackDraft: (version: number) => void;
  onRollbackPublished: (version: number) => void;
}) {
  return (
    <Drawer title="版本历史" width={760} open={open} onClose={onClose}>
      <Space orientation="vertical" style={{ width: '100%' }} size="middle">
        <Paragraph>
          <Text strong>页面：</Text> {pageKey || '-'}
        </Paragraph>
        <Paragraph>
          <Text strong>当前草稿：</Text> {currentDraftVersion || '-'},<Text strong>当前发布：</Text>{' '}
          {currentPublishedVersion || '-'}
        </Paragraph>
        <ProTable<PageVersionItem>
          columns={[
            {
              title: '版本',
              dataIndex: 'version',
              key: 'version',
              width: 90,
              render: (_, record) => <Text strong>v{record.version}</Text>,
            },
            {
              title: '状态',
              dataIndex: 'status',
              key: 'status',
              width: 100,
              render: (_, record) => (
                <Tag color={record.status === 'published' ? 'green' : 'blue'}>{record.status}</Tag>
              ),
            },
            {
              title: '当前位置',
              key: 'current',
              width: 160,
              render: (_, record) => (
                <Space>
                  {record.isCurrentDraft ? <Tag color="blue">当前草稿</Tag> : null}
                  {record.isCurrentPublished ? <Tag color="green">当前发布</Tag> : null}
                </Space>
              ),
            },
            {
              title: '说明',
              dataIndex: 'message',
              key: 'message',
              render: (_, record) => record.message || '-',
            },
            {
              title: '创建时间',
              dataIndex: 'createdAt',
              key: 'createdAt',
              width: 180,
              render: (_, record) => formatDate(record.createdAt),
            },
            {
              title: '操作',
              key: 'actions',
              width: 220,
              render: (_, record) => (
                <Space>
                  {!record.isCurrentDraft ? (
                    <Popconfirm
                      title={`确认回滚草稿到版本 ${record.version}？`}
                      onConfirm={() => onRollbackDraft(record.version)}
                    >
                      <Button type="link" size="small">
                        回滚草稿
                      </Button>
                    </Popconfirm>
                  ) : null}
                  {record.status === 'published' && !record.isCurrentPublished ? (
                    <Popconfirm
                      title={`确认回滚发布到版本 ${record.version}？`}
                      onConfirm={() => onRollbackPublished(record.version)}
                    >
                      <Button type="link" size="small">
                        回滚发布
                      </Button>
                    </Popconfirm>
                  ) : null}
                </Space>
              ),
            },
          ]}
          dataSource={items}
          loading={loading}
          rowKey="version"
          search={false}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            pageSizeOptions: [5, 10, 20, 50],
            showTotal: (t) => `共 ${t} 条`,
            onChange: (p, ps) => onPageChange(p, ps),
          }}
          options={false}
          locale={{ emptyText: <Empty description="暂无版本历史" /> }}
        />
      </Space>
    </Drawer>
  );
}
