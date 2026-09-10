import { useCallback } from 'react';
import { Alert, Button, Card, Space } from 'antd';
import { useIntl } from '@umijs/max';
import type { AuditEvent } from '@/services/api/audit';
import AuditList from './AuditList';

/** 近期活动 Tab：审计事件卡 + 手动刷新。 */
export default function ActivityTab({
  activities,
  loading,
  username,
  onRefresh,
}: {
  activities: AuditEvent[];
  loading: boolean;
  username: string;
  onRefresh: (username: string) => void;
}) {
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  return (
    <Space orientation="vertical" size={16} style={{ width: '100%' }}>
      {activities.length === 0 && !loading ? (
        <Alert
          showIcon
          type="info"
          message={formatMessage('profile.activities.unavailable')}
          style={{ marginBottom: 16 }}
        />
      ) : null}
      <Card
        loading={loading}
        extra={
          <Button type="link" onClick={() => onRefresh(username)}>
            {formatMessage('profile.activities.refresh')}
          </Button>
        }
      >
        <AuditList data={activities} emptyText={formatMessage('profile.activities.empty')} />
      </Card>
    </Space>
  );
}
