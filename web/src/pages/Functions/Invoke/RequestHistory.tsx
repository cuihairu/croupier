import { Button, Card, Empty, Space, Tag, Typography } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';
import { formatDuration, type RequestHistoryItem } from './types';

const { Text } = Typography;

interface RequestHistoryProps {
  items: RequestHistoryItem[];
  onClear: () => void;
  onSelect: (item: RequestHistoryItem) => void;
}

export default function RequestHistory({ items, onClear, onSelect }: RequestHistoryProps) {
  return (
    <Card
      size="small"
      title="请求历史"
      extra={
        <Button size="small" icon={<DeleteOutlined />} onClick={onClear}>
          清空
        </Button>
      }
    >
      {items.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无本地历史记录" />
      ) : (
        <Space orientation="vertical" size={8} style={{ width: '100%' }}>
          {items.map((item) => (
            <Card
              key={item.id}
              size="small"
              hoverable
              onClick={() => onSelect(item)}
              style={{ cursor: 'pointer' }}
            >
              <Space wrap>
                <Tag color={item.status === 'success' ? 'green' : 'red'}>
                  {item.status === 'success' ? '成功' : '失败'}
                </Tag>
                <Text code ellipsis style={{ maxWidth: 180 }}>
                  {item.functionId}
                </Text>
                <Text type="secondary">{formatDuration(item.duration)}</Text>
              </Space>
              <Text type="secondary" style={{ display: 'block', fontSize: 12, marginTop: 4 }}>
                {new Date(item.timestamp).toLocaleString()}
              </Text>
            </Card>
          ))}
        </Space>
      )}
    </Card>
  );
}
