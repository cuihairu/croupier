import { Button, Card, Drawer, Empty, Space, Tag, Typography } from 'antd';
import { MergeOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { DiffResponse } from '@/services/api/versioning';

const { Paragraph, Text } = Typography;

/** 变更对比抽屉：可自动合并项 / 冲突项 / 全部变更明细；
 * 「合并变更」入口按钮由 onMerge 回调打开合并弹窗。 */
export default function DiffDrawer({
  open,
  data,
  loading,
  onClose,
  onMerge,
}: {
  open: boolean;
  data: DiffResponse | null;
  loading: boolean;
  onClose: () => void;
  onMerge: () => void;
}) {
  const intl = useIntl();
  return (
    <Drawer
      title={intl.formatMessage({
        id: 'pages.pageStudio.studio.diff.title',
        defaultMessage: '变更对比',
      })}
      width={840}
      open={open}
      onClose={onClose}
      loading={loading}
      extra={
        <Button type="primary" icon={<MergeOutlined />} onClick={onMerge}>
          <FormattedMessage
            id="pages.pageStudio.studio.diff.mergeButton"
            defaultMessage="合并变更"
          />
        </Button>
      }
    >
      {data ? (
        <Space orientation="vertical" style={{ width: '100%' }}>
          <Paragraph>
            <Text strong>{data.summary}</Text>
          </Paragraph>
          {data.autoMergeItems?.length ? (
            <Card
              size="small"
              title={intl.formatMessage(
                {
                  id: 'pages.pageStudio.studio.diff.autoMergeCard',
                  defaultMessage: '可自动合并 {count} 个展示字段',
                },
                { count: data.autoMergeItems.length },
              )}
            >
              <Space orientation="vertical" style={{ width: '100%' }}>
                {data.autoMergeItems.map((item) => (
                  <Space key={item.field}>
                    <Tag color="blue">auto</Tag>
                    <Text code>{item.field}</Text>
                    <Text type="secondary">{item.reason}</Text>
                  </Space>
                ))}
              </Space>
            </Card>
          ) : null}
          {data.conflictItems?.length ? (
            <Card
              size="small"
              title={intl.formatMessage(
                {
                  id: 'pages.pageStudio.studio.diff.conflictCard',
                  defaultMessage: '必须人工确认 {count} 个冲突字段',
                },
                { count: data.conflictItems.length },
              )}
            >
              <Space orientation="vertical" style={{ width: '100%' }}>
                {data.conflictItems.map((item) => (
                  <Space key={item.field}>
                    <Tag color="red">conflict</Tag>
                    <Text code>{item.field}</Text>
                    <Text type="secondary">{item.reason}</Text>
                  </Space>
                ))}
              </Space>
            </Card>
          ) : null}
          {data.changes.map((change) => (
            <Card key={`${change.path}:${change.changeType}`} size="small">
              <Space orientation="vertical" style={{ width: '100%' }}>
                <Space>
                  <Tag
                    color={
                      change.changeType === 'added'
                        ? 'green'
                        : change.changeType === 'removed'
                          ? 'red'
                          : 'orange'
                    }
                  >
                    {change.changeType}
                  </Tag>
                  <Text code>{change.path}</Text>
                  {change.isSemantic && (
                    <Tag color="purple">
                      <FormattedMessage
                        id="pages.pageStudio.studio.diff.semanticChangeTag"
                        defaultMessage="语义变更"
                      />
                    </Tag>
                  )}
                </Space>
                {change.oldValue && (
                  <pre style={{ margin: 0, padding: 8, background: '#f5f5f5' }}>
                    {JSON.stringify(change.oldValue, null, 2)}
                  </pre>
                )}
                {change.newValue && (
                  <pre style={{ margin: 0, padding: 8, background: '#f5f5f5' }}>
                    {JSON.stringify(change.newValue, null, 2)}
                  </pre>
                )}
              </Space>
            </Card>
          ))}
        </Space>
      ) : (
        <Empty
          description={intl.formatMessage({
            id: 'pages.pageStudio.studio.diff.empty',
            defaultMessage: '暂无变更',
          })}
        />
      )}
    </Drawer>
  );
}
