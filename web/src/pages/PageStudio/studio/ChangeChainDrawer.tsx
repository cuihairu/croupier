import { Drawer, Empty, Space, Tag, Timeline, Typography } from 'antd';
import type { ChangeChain } from '@/services/api/versioning';
import { formatDate } from './shared';

const { Paragraph, Text } = Typography;

/** 变更链抽屉：函数/语义/提案/草稿/发布版本时间线。 */
export default function ChangeChainDrawer({
  open,
  chain,
  loading,
  onClose,
}: {
  open: boolean;
  chain: ChangeChain | null;
  loading: boolean;
  onClose: () => void;
}) {
  return (
    <Drawer title="变更链" width={640} open={open} onClose={onClose} loading={loading}>
      {chain ? (
        <Space orientation="vertical" style={{ width: '100%' }}>
          <Paragraph>
            <Text strong>页面：</Text> {chain.pageKey}
          </Paragraph>
          <Paragraph>
            <Text strong>资源：</Text> {chain.resourceKey}
          </Paragraph>
          <Paragraph>
            <Text strong>当前状态：</Text>
            函数版本: {chain.current.functionVersion || '-'}, 语义版本:{' '}
            {chain.current.semanticVersion || '-'}, 提案版本: {chain.current.proposalVersion || '-'}
            , 草稿版本: {chain.current.draftRevision || '-'}, 发布版本:{' '}
            {chain.current.publishedVersion || '-'}
          </Paragraph>
          <Timeline
            items={chain.items.map((item) => ({
              children: (
                <Space orientation="vertical" size={0}>
                  <Space>
                    <Tag color="blue">{item.type}</Tag>
                    <Text>{item.summary}</Text>
                  </Space>
                  <Text type="secondary">
                    {formatDate(item.timestamp)}
                    {item.actor ? ` - ${item.actor}` : ''}
                  </Text>
                </Space>
              ),
            }))}
          />
        </Space>
      ) : (
        <Empty description="暂无变更记录" />
      )}
    </Drawer>
  );
}
