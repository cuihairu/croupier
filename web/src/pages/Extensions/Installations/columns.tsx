import { Button, Dropdown, Space, Tag, Typography } from 'antd';
import { MoreOutlined } from '@ant-design/icons';
import { FormattedMessage } from '@umijs/max';
import type { ProColumns } from '@ant-design/pro-components';
import type { ExtensionInstallationItem } from '@/services/api/extensions';
import { formatUnix } from './shared';

const { Text } = Typography;

/** 模块级文案助手接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
type IntlFormatter = {
  formatMessage: (
    descriptor: { id: string; defaultMessage: string },
    values?: Record<string, string | number>,
  ) => string;
};

/** 安装列表列定义：操作列回调（详情/事件/升级/启停/重建/卸载）由页面注入。 */
export function buildInstallationsColumns({
  intl,
  canManage,
  onDetail,
  onEvents,
  onUpgrade,
  onToggleEnabled,
  onReconcile,
  onUninstall,
}: {
  intl: IntlFormatter;
  canManage: boolean;
  onDetail: (row: ExtensionInstallationItem) => void;
  onEvents: (row: ExtensionInstallationItem) => void;
  onUpgrade: (row: ExtensionInstallationItem) => void;
  onToggleEnabled: (row: ExtensionInstallationItem) => void;
  onReconcile: (row: ExtensionInstallationItem) => void;
  onUninstall: (row: ExtensionInstallationItem) => void;
}): ProColumns<ExtensionInstallationItem>[] {
  return [
    {
      title: intl.formatMessage({
        id: 'pages.extensionsInstallations.column.displayName',
        defaultMessage: '安装实例',
      }),
      dataIndex: 'displayName',
      key: 'displayName',
      render: (_, row) => (
        <Space orientation="vertical" size={0}>
          <Text strong>{row.displayName || row.extensionId}</Text>
          <Text type="secondary">
            #{row.id} / {row.installationKey}
          </Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsInstallations.column.releaseVersion',
        defaultMessage: '版本',
      }),
      dataIndex: 'releaseVersion',
      key: 'releaseVersion',
      width: 120,
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsInstallations.column.status',
        defaultMessage: '状态',
      }),
      key: 'status',
      width: 150,
      render: (_, row) => (
        <Space>
          <Tag color={row.enabled ? 'green' : 'default'}>
            {row.enabled
              ? intl.formatMessage({
                  id: 'pages.extensionsInstallations.column.enabledTag',
                  defaultMessage: '启用',
                })
              : intl.formatMessage({
                  id: 'pages.extensionsInstallations.column.disabledTag',
                  defaultMessage: '禁用',
                })}
          </Tag>
          <Tag>{row.status || '-'}</Tag>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsInstallations.column.health',
        defaultMessage: '健康',
      }),
      dataIndex: 'healthStatus',
      key: 'healthStatus',
      width: 120,
      render: (_, row) => (
        <Tag
          color={
            row.healthStatus === 'healthy'
              ? 'green'
              : row.healthStatus === 'error'
                ? 'red'
                : 'default'
          }
        >
          {row.healthStatus || '-'}
        </Tag>
      ),
    },
    {
      title: 'Scope/Target',
      key: 'scope_target',
      render: (_, row) => (
        <Space orientation="vertical" size={0}>
          <Text>
            {row.scopeType}:{row.scopeId}
          </Text>
          <Text type="secondary">
            {row.targetType}:{row.targetId || '-'}
          </Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsInstallations.column.updatedAt',
        defaultMessage: '更新时间',
      }),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 170,
      render: (_, row) => formatUnix(row.updatedAt),
    },
    {
      title: intl.formatMessage({
        id: 'pages.extensionsInstallations.column.actions',
        defaultMessage: '操作',
      }),
      key: 'actions',
      width: 220,
      render: (_, row) => (
        <Space wrap>
          <Button size="small" type="primary" ghost onClick={() => onDetail(row)}>
            <FormattedMessage
              id="pages.extensionsInstallations.action.detail"
              defaultMessage="查看详情"
            />
          </Button>
          <Button size="small" onClick={() => onEvents(row)}>
            <FormattedMessage
              id="pages.extensionsInstallations.action.events"
              defaultMessage="查看事件"
            />
          </Button>
          <Dropdown
            trigger={['click']}
            menu={{
              items: [
                {
                  key: row.enabled ? 'disable' : 'enable',
                  label: row.enabled
                    ? intl.formatMessage({
                        id: 'pages.extensionsInstallations.action.disableCurrent',
                        defaultMessage: '禁用当前安装',
                      })
                    : intl.formatMessage({
                        id: 'pages.extensionsInstallations.action.enableCurrent',
                        defaultMessage: '启用当前安装',
                      }),
                  disabled: !canManage,
                },
                {
                  key: 'upgrade',
                  label: intl.formatMessage({
                    id: 'pages.extensionsInstallations.action.upgradeCurrent',
                    defaultMessage: '升级当前安装',
                  }),
                  disabled: !canManage,
                },
                {
                  key: 'reconcile',
                  label: intl.formatMessage({
                    id: 'pages.extensionsInstallations.action.reconcileCurrent',
                    defaultMessage: '重建当前绑定',
                  }),
                  disabled: !canManage,
                },
                {
                  type: 'divider',
                },
                {
                  key: 'uninstall',
                  label: intl.formatMessage({
                    id: 'pages.extensionsInstallations.action.uninstallCurrent',
                    defaultMessage: '卸载当前安装',
                  }),
                  danger: true,
                  disabled: !canManage,
                },
              ],
              onClick: ({ key }) => {
                if (key === 'enable' || key === 'disable') {
                  onToggleEnabled(row);
                  return;
                }
                if (key === 'upgrade') {
                  onUpgrade(row);
                  return;
                }
                if (key === 'reconcile') {
                  onReconcile(row);
                  return;
                }
                if (key === 'uninstall') {
                  onUninstall(row);
                }
              },
            }}
          >
            <Button size="small" icon={<MoreOutlined />}>
              <FormattedMessage
                id="pages.extensionsInstallations.action.more"
                defaultMessage="更多"
              />
            </Button>
          </Dropdown>
        </Space>
      ),
    },
  ];
}
