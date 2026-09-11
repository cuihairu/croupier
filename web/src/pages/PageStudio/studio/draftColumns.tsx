import { Button, Dropdown, Popconfirm, Space, Tag, Tooltip, Typography } from 'antd';
import {
  DiffOutlined,
  EditOutlined,
  EyeOutlined,
  HistoryOutlined,
  MoreOutlined,
  ReloadOutlined,
  RocketOutlined,
  StopOutlined,
} from '@ant-design/icons';
import type { ProColumns } from '@ant-design/pro-components';
import type { PageSpecDraftSummary, PageType } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';
import { formatDate, pageTypeLabel, statusColor } from './shared';

const { Text } = Typography;

/** 模块级文案助手接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
type IntlFormatter = {
  formatMessage: (
    descriptor: { id: string; defaultMessage: string },
    values?: Record<string, string | number>,
  ) => string;
};

/** 草稿列表列定义：操作列回调（发布/编辑/预览/重生成/版本/变更链/对比）
 * 由工作台主页注入；modal（regenerate 二次确认）随回调一起传入。 */
export function buildDraftColumns(
  handlers: {
    onEdit: (pageKey: string, type: PageType) => void;
    onPreview: (pageKey: string) => void;
    onPublish: (pageKey: string, draftRevision: number) => void;
    onUnpublish: (pageKey: string) => void;
    onRegenerate: (pageKey: string, draftRevision: number) => void;
    onVersions: (pageKey: string) => void;
    onChangeChain: (pageKey: string) => void;
    onDiff: (pageKey: string) => void;
  },
  modal: { confirm: (config: { title: string; content: string; onOk: () => void }) => void },
  intl: IntlFormatter,
): ProColumns<PageSpecDraftSummary>[] {
  return [
    {
      title: intl.formatMessage({
        id: 'pages.pageStudio.studio.column.pageKey',
        defaultMessage: '页面标识',
      }),
      dataIndex: 'pageKey',
      key: 'pageKey',
      width: 220,
      render: (_, record) => (
        <Space>
          <Text strong>{record.pageKey}</Text>
          <Tag color="blue">{pageTypeLabel(record.type)}</Tag>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.pageStudio.studio.column.title',
        defaultMessage: '标题',
      }),
      dataIndex: 'title',
      key: 'title',
      render: (_, record) => localizedText(record.title, record.pageKey),
    },
    {
      title: intl.formatMessage({
        id: 'pages.pageStudio.studio.column.category',
        defaultMessage: '分类',
      }),
      dataIndex: ['category', 'key'],
      key: 'category',
      width: 120,
      render: (_, record) => localizedText(record.category?.labels, record.category?.key || '-'),
    },
    {
      title: intl.formatMessage({
        id: 'pages.pageStudio.studio.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (_, record) => <Tag color={statusColor(record.status)}>{record.status}</Tag>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.pageStudio.studio.column.version',
        defaultMessage: '版本',
      }),
      dataIndex: 'publishedVersion',
      key: 'version',
      width: 80,
      render: (_, record) => record.publishedVersion || '-',
    },
    {
      title: intl.formatMessage({
        id: 'pages.pageStudio.studio.column.updatedAt',
        defaultMessage: '更新时间',
      }),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 180,
      render: (_, record) => formatDate(record.updatedAt),
    },
    {
      title: intl.formatMessage({
        id: 'pages.pageStudio.studio.column.actions',
        defaultMessage: '操作',
      }),
      key: 'actions',
      width: 160,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4}>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.pageStudio.studio.action.edit',
              defaultMessage: '编辑',
            })}
          >
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handlers.onEdit(record.pageKey, record.type)}
            />
          </Tooltip>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.pageStudio.studio.action.preview',
              defaultMessage: '预览',
            })}
          >
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handlers.onPreview(record.pageKey)}
            />
          </Tooltip>
          {record.status === 'draft' ? (
            <Popconfirm
              title={intl.formatMessage({
                id: 'pages.pageStudio.studio.action.publishConfirm',
                defaultMessage: '确认发布此页面？',
              })}
              onConfirm={() => handlers.onPublish(record.pageKey, record.draftRevision)}
            >
              <Tooltip
                title={intl.formatMessage({
                  id: 'pages.pageStudio.studio.action.publish',
                  defaultMessage: '发布',
                })}
              >
                <Button type="link" size="small" icon={<RocketOutlined />} />
              </Tooltip>
            </Popconfirm>
          ) : (
            <Popconfirm
              title={intl.formatMessage({
                id: 'pages.pageStudio.studio.action.unpublishConfirm',
                defaultMessage: '确认取消发布此页面？',
              })}
              onConfirm={() => handlers.onUnpublish(record.pageKey)}
            >
              <Tooltip
                title={intl.formatMessage({
                  id: 'pages.pageStudio.studio.action.unpublish',
                  defaultMessage: '取消发布',
                })}
              >
                <Button type="link" size="small" icon={<StopOutlined />} danger />
              </Tooltip>
            </Popconfirm>
          )}
          <Dropdown
            menu={{
              items: [
                {
                  key: 'regenerate',
                  icon: <ReloadOutlined />,
                  label: intl.formatMessage({
                    id: 'pages.pageStudio.studio.action.regenerate',
                    defaultMessage: '重新生成',
                  }),
                  onClick: () =>
                    modal.confirm({
                      title: intl.formatMessage({
                        id: 'pages.pageStudio.studio.action.regenerateConfirmTitle',
                        defaultMessage: '确认按最新 Proposal 重新生成草稿？',
                      }),
                      content: intl.formatMessage({
                        id: 'pages.pageStudio.studio.action.regenerateConfirmContent',
                        defaultMessage: '当前草稿修改将被最新 Proposal 覆盖，已发布版本不会变更。',
                      }),
                      onOk: () => handlers.onRegenerate(record.pageKey, record.draftRevision),
                    }),
                },
                {
                  key: 'versions',
                  icon: <HistoryOutlined />,
                  label: intl.formatMessage({
                    id: 'pages.pageStudio.studio.action.versions',
                    defaultMessage: '版本历史',
                  }),
                  onClick: () => handlers.onVersions(record.pageKey),
                },
                {
                  key: 'change-chain',
                  icon: <HistoryOutlined />,
                  label: intl.formatMessage({
                    id: 'pages.pageStudio.studio.action.changeChain',
                    defaultMessage: '变更链',
                  }),
                  onClick: () => handlers.onChangeChain(record.pageKey),
                },
                {
                  key: 'diff',
                  icon: <DiffOutlined />,
                  label: intl.formatMessage({
                    id: 'pages.pageStudio.studio.action.diff',
                    defaultMessage: '变更对比',
                  }),
                  onClick: () => handlers.onDiff(record.pageKey),
                },
              ],
            }}
            trigger={['click']}
          >
            <Button type="link" size="small" icon={<MoreOutlined />} />
          </Dropdown>
        </Space>
      ),
    },
  ];
}
