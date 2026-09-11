import { Button, Card, Empty, Space, Tag, Typography } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import { formatDateTime } from '@/utils/format';
import { formatDuration, type RequestHistoryItem } from './types';

const { Text } = Typography;

interface RequestHistoryProps {
  items: RequestHistoryItem[];
  onClear: () => void;
  onSelect: (item: RequestHistoryItem) => void;
}

export default function RequestHistory({ items, onClear, onSelect }: RequestHistoryProps) {
  const intl = useIntl();
  return (
    <Card
      size="small"
      title={intl.formatMessage({
        id: 'pages.functionsInvoke.requestHistory.title',
        defaultMessage: '请求历史',
      })}
      extra={
        <Button size="small" icon={<DeleteOutlined />} onClick={onClear}>
          <FormattedMessage id="pages.functionsInvoke.requestHistory.clear" defaultMessage="清空" />
        </Button>
      }
    >
      {items.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={intl.formatMessage({
            id: 'pages.functionsInvoke.requestHistory.empty',
            defaultMessage: '暂无本地历史记录',
          })}
        />
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
                  {item.status === 'success' ? (
                    <FormattedMessage
                      id="pages.functionsInvoke.requestHistory.status.success"
                      defaultMessage="成功"
                    />
                  ) : (
                    <FormattedMessage
                      id="pages.functionsInvoke.requestHistory.status.error"
                      defaultMessage="失败"
                    />
                  )}
                </Tag>
                <Text code ellipsis style={{ maxWidth: 180 }}>
                  {item.functionId}
                </Text>
                <Text type="secondary">{formatDuration(item.duration)}</Text>
              </Space>
              <Text type="secondary" style={{ display: 'block', fontSize: 12, marginTop: 4 }}>
                {formatDateTime(item.timestamp)}
              </Text>
            </Card>
          ))}
        </Space>
      )}
    </Card>
  );
}
