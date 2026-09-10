import { Button, Dropdown, Space, Tag, Typography } from 'antd';
import { MoreOutlined } from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { ExtensionInstallationItem } from '@/services/api/extensions';
import { formatUnix } from './shared';

const { Text } = Typography;

/** 安装列表列定义：操作列回调（详情/事件/升级/启停/重建/卸载）由页面注入。 */
export function buildInstallationsColumns({
  canManage,
  onDetail,
  onEvents,
  onUpgrade,
  onToggleEnabled,
  onReconcile,
  onUninstall,
}: {
  canManage: boolean;
  onDetail: (row: ExtensionInstallationItem) => void;
  onEvents: (row: ExtensionInstallationItem) => void;
  onUpgrade: (row: ExtensionInstallationItem) => void;
  onToggleEnabled: (row: ExtensionInstallationItem) => void;
  onReconcile: (row: ExtensionInstallationItem) => void;
  onUninstall: (row: ExtensionInstallationItem) => void;
}): ColumnsType<ExtensionInstallationItem> {
  return [
    {
      title: '安装实例',
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
      title: '版本',
      dataIndex: 'releaseVersion',
      key: 'releaseVersion',
      width: 120,
    },
    {
      title: '状态',
      key: 'status',
      width: 150,
      render: (_, row) => (
        <Space>
          <Tag color={row.enabled ? 'green' : 'default'}>{row.enabled ? '启用' : '禁用'}</Tag>
          <Tag>{row.status || '-'}</Tag>
        </Space>
      ),
    },
    {
      title: '健康',
      dataIndex: 'healthStatus',
      key: 'healthStatus',
      width: 120,
      render: (value) => (
        <Tag color={value === 'healthy' ? 'green' : value === 'error' ? 'red' : 'default'}>
          {value || '-'}
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
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 170,
      render: (v) => formatUnix(v),
    },
    {
      title: '操作',
      key: 'actions',
      width: 220,
      render: (_, row) => (
        <Space wrap>
          <Button size="small" type="primary" ghost onClick={() => onDetail(row)}>
            查看详情
          </Button>
          <Button size="small" onClick={() => onEvents(row)}>
            查看事件
          </Button>
          <Dropdown
            trigger={['click']}
            menu={{
              items: [
                {
                  key: row.enabled ? 'disable' : 'enable',
                  label: row.enabled ? '禁用当前安装' : '启用当前安装',
                  disabled: !canManage,
                },
                {
                  key: 'upgrade',
                  label: '升级当前安装',
                  disabled: !canManage,
                },
                {
                  key: 'reconcile',
                  label: '重建当前绑定',
                  disabled: !canManage,
                },
                {
                  type: 'divider',
                },
                {
                  key: 'uninstall',
                  label: '卸载当前安装',
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
              更多
            </Button>
          </Dropdown>
        </Space>
      ),
    },
  ];
}
