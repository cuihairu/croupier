import { useCallback } from 'react';
import { Button, Card, Divider, Table, Tag, Typography } from 'antd';
import { useIntl } from '@umijs/max';
import { formatDateTime } from '@/utils/format';

const { Text } = Typography;

/** 登录会话 Tab：登录记录表（IP/地域/UA/成败）。 */
export default function SessionsTab({
  rows,
  loading,
  username,
  onRefresh,
}: {
  rows: {
    key: string;
    time?: string;
    kind?: string;
    ip: string;
    region: string;
    userAgent: string;
    success: boolean;
    target?: string;
  }[];
  loading: boolean;
  username: string;
  onRefresh: (username: string) => void;
}) {
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  return (
    <Card
      loading={loading}
      extra={
        <Button type="link" onClick={() => onRefresh(username)}>
          {formatMessage('profile.activities.refresh')}
        </Button>
      }
    >
      <Table
        rowKey="key"
        size="small"
        pagination={{ pageSize: 8, showSizeChanger: false }}
        dataSource={rows}
        locale={{ emptyText: formatMessage('profile.sessions.empty') }}
        scroll={{ x: 900 }}
        columns={[
          {
            title: formatMessage('profile.sessions.col.time'),
            dataIndex: 'time',
            width: 180,
            render: (value: string) => formatDateTime(value ?? ''),
          },
          {
            title: formatMessage('profile.sessions.col.result'),
            dataIndex: 'success',
            width: 100,
            render: (success: boolean) =>
              success ? (
                <Tag color="green">{formatMessage('profile.sessions.result.success')}</Tag>
              ) : (
                <Tag color="red">{formatMessage('profile.sessions.result.failed')}</Tag>
              ),
          },
          {
            title: 'IP',
            dataIndex: 'ip',
            width: 160,
            render: (value: string) => value || '-',
          },
          {
            title: formatMessage('profile.sessions.col.region'),
            dataIndex: 'region',
            width: 160,
            render: (value: string) => value || '-',
          },
          {
            title: formatMessage('profile.sessions.col.kind'),
            dataIndex: 'kind',
            width: 140,
            render: (value: string) => <Tag>{value || '-'}</Tag>,
          },
          {
            title: formatMessage('profile.sessions.col.ua'),
            dataIndex: 'userAgent',
            ellipsis: true,
            render: (value: string) => value || '-',
          },
        ]}
      />
      <Divider style={{ margin: '12px 0' }} />
      <Text type="secondary">{formatMessage('profile.sessions.hint')}</Text>
    </Card>
  );
}
