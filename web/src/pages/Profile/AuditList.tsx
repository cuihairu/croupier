import { List, Space, Tag, Typography } from 'antd';
import type { AuditEvent } from '@/services/api/audit';

const { Text } = Typography;

/** 审计事件列表（活动/会话两个 Tab 共用的展示组件）。 */
export default function AuditList({ data, emptyText }: { data: AuditEvent[]; emptyText: string }) {
  return (
    <List
      dataSource={data}
      locale={{ emptyText }}
      renderItem={(item) => (
        <List.Item>
          <List.Item.Meta
            title={
              <Space>
                <Tag color="blue">{item.kind}</Tag>
                <Text>{item.target || '-'}</Text>
              </Space>
            }
            description={
              <Space orientation="vertical" size={0}>
                <Text type="secondary">
                  {item.time ? new Date(item.time).toLocaleString() : '-'}
                </Text>
                {item.meta && (
                  <Text type="secondary">
                    {Object.entries(item.meta)
                      .slice(0, 2)
                      .map(([key, value]) => `${key}: ${value}`)
                      .join(' · ')}
                  </Text>
                )}
              </Space>
            }
          />
        </List.Item>
      )}
    />
  );
}
