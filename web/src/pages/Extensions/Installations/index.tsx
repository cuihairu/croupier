import { useRef, useState } from 'react';
import { Alert, App, Button, Input, Select, Space, Tag, Typography } from 'antd';
import { PageContainer, ProTable, type ActionType } from '@ant-design/pro-components';
import { FormattedMessage, useAccess, useIntl } from '@umijs/max';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
import {
  disableExtension,
  enableExtension,
  listExtensionInstallations,
  reconcileExtension,
  uninstallExtension,
  type ExtensionInstallationItem,
} from '@/services/api/extensions';
import { adaptInstallationListResponse } from '@/services/adapters/extensions';
import { EXTENSION_ERROR_CODES } from '@/services/errors/codes';
import { mapExtensionError } from '@/services/errors/mapper';
import { buildInstallationsColumns } from './columns';
import EventsDrawer from './EventsDrawer';
import InstallationDetailDrawer from './InstallationDetailDrawer';
import UpgradeModal from './UpgradeModal';

const { Text } = Typography;

/** 扩展安装页：列表 + 筛选 + 概览，详情/事件/升级三个 overlay 以受控组件组合。 */
export default function ExtensionsInstallationsPage() {
  const access = useAccess();
  const intl = useIntl();
  const { message, modal } = App.useApp();
  // 当前页数据副本：概览统计与「当前结果」计数依赖它，在 request 成功后同步
  const [items, setItems] = useState<ExtensionInstallationItem[]>([]);
  const [total, setTotal] = useState(0);
  const [status, setStatus] = useState<string | undefined>(undefined);
  const [extensionID, setExtensionID] = useState('');
  const actionRef = useRef<ActionType | undefined>(undefined);

  const [eventsOpen, setEventsOpen] = useState(false);
  const [eventsRow, setEventsRow] = useState<ExtensionInstallationItem | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailRow, setDetailRow] = useState<ExtensionInstallationItem | null>(null);
  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [upgradeRow, setUpgradeRow] = useState<ExtensionInstallationItem | null>(null);

  const summary = {
    total,
    enabledCount: items.filter((item) => item.enabled).length,
    disabledCount: items.filter((item) => !item.enabled).length,
    healthyCount: items.filter((item) => item.healthStatus === 'healthy').length,
    scopeCount: new Set(items.map((item) => `${item.scopeType}:${item.scopeId}`)).size,
  };
  const hasListFilters = Boolean(extensionID.trim() || status);
  const listFilterSummary = [
    extensionID.trim()
      ? intl.formatMessage(
          {
            id: 'pages.extensionsInstallations.filter.extensionChip',
            defaultMessage: `扩展 ${extensionID.trim()}`,
          },
          { value: extensionID.trim() },
        )
      : null,
    status
      ? intl.formatMessage(
          {
            id: 'pages.extensionsInstallations.filter.statusChip',
            defaultMessage: `状态 ${status}`,
          },
          { value: status },
        )
      : null,
  ]
    .filter(Boolean)
    .join(' / ');

  const reload = () => actionRef.current?.reload();

  const withReload = async (fn: () => Promise<unknown>, successText: string) => {
    await fn();
    message.success(successText);
    reload();
  };

  const handleUninstall = (row: ExtensionInstallationItem) => {
    modal.confirm({
      title: intl.formatMessage({
        id: 'pages.extensionsInstallations.confirmUninstall.title',
        defaultMessage: '确认卸载扩展',
      }),
      content: intl.formatMessage(
        {
          id: 'pages.extensionsInstallations.confirmUninstall.content',
          defaultMessage: `安装实例 #${row.id} 将被卸载，是否继续？`,
        },
        { id: row.id },
      ),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await uninstallExtension(row.id);
          message.success(
            intl.formatMessage({
              id: 'pages.extensionsInstallations.toast.uninstalled',
              defaultMessage: '已卸载扩展',
            }),
          );
          reload();
        } catch (err) {
          const uiErr = mapExtensionError(err as Error);
          const details = uiErr.details || {};
          const blockers = Array.isArray(details.blockers) ? details.blockers : [];
          if (uiErr.code === EXTENSION_ERROR_CODES.DEPENDENCY_BLOCKED && blockers.length > 0) {
            modal.warning({
              title: intl.formatMessage({
                id: 'pages.extensionsInstallations.uninstallBlocked.title',
                defaultMessage: '无法卸载：存在依赖',
              }),
              content: (
                <Space orientation="vertical">
                  <Text type="secondary">
                    <FormattedMessage
                      id="pages.extensionsInstallations.uninstallBlocked.blockersHint"
                      defaultMessage="以下扩展仍依赖当前扩展，请先处理它们："
                    />
                  </Text>
                  {blockers.map((item: unknown) => (
                    <Tag key={String(item)} color="orange">
                      {String(item)}
                    </Tag>
                  ))}
                </Space>
              ),
            });
            return;
          }
          message.error(uiErr.message);
        }
      },
    });
  };

  const columns = buildInstallationsColumns({
    intl,
    canManage: access.canExtensionsManage,
    onDetail: (row) => {
      setDetailRow(row);
      setDetailOpen(true);
    },
    onEvents: (row) => {
      setEventsRow(row);
      setEventsOpen(true);
    },
    onUpgrade: (row) => {
      setUpgradeRow(row);
      setUpgradeOpen(true);
    },
    onToggleEnabled: (row) => {
      void withReload(
        () => (row.enabled ? disableExtension(row.id) : enableExtension(row.id)),
        row.enabled
          ? intl.formatMessage({
              id: 'pages.extensionsInstallations.action.toggleDisabledToast',
              defaultMessage: '已禁用扩展',
            })
          : intl.formatMessage({
              id: 'pages.extensionsInstallations.action.toggleEnabledToast',
              defaultMessage: '已启用扩展',
            }),
      );
    },
    onReconcile: (row) => {
      void withReload(
        () => reconcileExtension(row.id),
        intl.formatMessage({
          id: 'pages.extensionsInstallations.action.reconcileToast',
          defaultMessage: '已触发重建绑定',
        }),
      );
    },
    onUninstall: handleUninstall,
  });

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.extensionsInstallations.page.title',
        defaultMessage: '扩展安装',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.extensionsInstallations.page.subTitle',
        defaultMessage: '查看和管理已安装扩展实例',
      })}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title={intl.formatMessage({
            id: 'pages.extensionsInstallations.overview.title',
            defaultMessage: '扩展概览',
          })}
          description={intl.formatMessage({
            id: 'pages.extensionsInstallations.overview.description',
            defaultMessage:
              '这个页面优先承担安装实例排查和运维动作，建议先按扩展或状态筛选，再进入详情、事件或升级流程。',
          })}
          items={[
            {
              color: '#1677ff',
              text: intl.formatMessage(
                {
                  id: 'pages.extensionsInstallations.overview.itemTotal',
                  defaultMessage: `总数 ${summary.total}`,
                },
                { count: summary.total },
              ),
            },
            {
              color: '#52c41a',
              text: intl.formatMessage(
                {
                  id: 'pages.extensionsInstallations.overview.itemEnabled',
                  defaultMessage: `启用 ${summary.enabledCount}`,
                },
                { count: summary.enabledCount },
              ),
            },
            {
              color: '#d9d9d9',
              text: intl.formatMessage(
                {
                  id: 'pages.extensionsInstallations.overview.itemDisabled',
                  defaultMessage: `禁用 ${summary.disabledCount}`,
                },
                { count: summary.disabledCount },
              ),
            },
            {
              color: '#13c2c2',
              text: intl.formatMessage(
                {
                  id: 'pages.extensionsInstallations.overview.itemHealthy',
                  defaultMessage: `健康 ${summary.healthyCount}`,
                },
                { count: summary.healthyCount },
              ),
            },
            {
              color: '#722ed1',
              text: intl.formatMessage(
                {
                  id: 'pages.extensionsInstallations.overview.itemScope',
                  defaultMessage: `作用域 ${summary.scopeCount}`,
                },
                { count: summary.scopeCount },
              ),
            },
          ]}
          hint={intl.formatMessage({
            id: 'pages.extensionsInstallations.overview.hint',
            defaultMessage:
              '推荐路径：先在列表确认实例状态，再进入详情修改配置；事件和升级属于次级动作，不应抢主流程注意力。',
          })}
        />

        <StandardListSection
          title={intl.formatMessage({
            id: 'pages.extensionsInstallations.list.title',
            defaultMessage: '安装列表',
          })}
          extra={
            <Button onClick={reload}>
              <FormattedMessage
                id="pages.extensionsInstallations.list.refresh"
                defaultMessage="刷新"
              />
            </Button>
          }
        >
          <StandardFilterBar
            resultText={intl.formatMessage(
              {
                id: 'pages.extensionsInstallations.list.resultText',
                defaultMessage: `当前结果 ${items.length} 个安装实例`,
              },
              { count: items.length },
            )}
            controls={
              <>
                <Input
                  style={{ width: 240 }}
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.extensionsInstallations.filter.placeholderExtensionId',
                    defaultMessage: '扩展 ID',
                  })}
                  value={extensionID}
                  onChange={(e) => {
                    // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
                    // ProTable 内部 debounce + abort 合并，不会出现错序数据
                    setExtensionID(e.target.value);
                    actionRef.current?.setPageInfo?.({ current: 1 });
                  }}
                />
                <Select
                  style={{ width: 160 }}
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.extensionsInstallations.filter.placeholderStatus',
                    defaultMessage: '状态',
                  })}
                  value={status}
                  onChange={(v) => {
                    setStatus(v);
                    actionRef.current?.setPageInfo?.({ current: 1 });
                  }}
                  options={[
                    { label: 'installing', value: 'installing' },
                    { label: 'running', value: 'running' },
                    { label: 'disabled', value: 'disabled' },
                    { label: 'error', value: 'error' },
                    { label: 'uninstalling', value: 'uninstalling' },
                  ]}
                />
                {hasListFilters ? (
                  <Button
                    onClick={() => {
                      setExtensionID('');
                      setStatus(undefined);
                      actionRef.current?.setPageInfo?.({ current: 1 });
                    }}
                  >
                    <FormattedMessage
                      id="pages.extensionsInstallations.filter.clear"
                      defaultMessage="清空筛选"
                    />
                  </Button>
                ) : null}
              </>
            }
          />
          {hasListFilters ? (
            <Alert
              style={{ marginBottom: 12 }}
              type="info"
              showIcon
              message={intl.formatMessage({
                id: 'pages.extensionsInstallations.filter.activeMessage',
                defaultMessage: '当前正在查看筛选后的安装实例',
              })}
              description={intl.formatMessage(
                {
                  id: 'pages.extensionsInstallations.filter.activeDescription',
                  defaultMessage: `已生效条件：${listFilterSummary}`,
                },
                { conditions: listFilterSummary },
              )}
            />
          ) : null}

          <ProTable<ExtensionInstallationItem>
            actionRef={actionRef}
            scroll={{ x: 900 }}
            rowKey="id"
            columns={columns}
            search={false}
            options={false}
            toolBarRender={false}
            params={{ extensionID, status }}
            request={async ({
              current = 1,
              pageSize = 10,
              extensionID: extensionIdFilter,
              status: statusFilter,
            }) => {
              try {
                const resp = await listExtensionInstallations({
                  extensionId: extensionIdFilter.trim() || undefined,
                  status: statusFilter,
                  page: current,
                  pageSize,
                });
                const vm = adaptInstallationListResponse(resp);
                setItems(vm.items);
                setTotal(vm.total);
                return { data: vm.items, total: vm.total, success: true };
              } catch {
                // 原实现不本地弹错（全局请求拦截器已 toast），保持该语义
                return { data: [], total: 0, success: false };
              }
            }}
            pagination={{ pageSize: 10, showSizeChanger: true }}
          />
        </StandardListSection>
      </Space>

      <EventsDrawer
        open={eventsOpen}
        installation={eventsRow}
        onClose={() => setEventsOpen(false)}
      />

      <UpgradeModal
        open={upgradeOpen}
        row={upgradeRow}
        onClose={() => setUpgradeOpen(false)}
        onUpgraded={async () => {
          await actionRef.current?.reload();
        }}
      />

      <InstallationDetailDrawer
        open={detailOpen}
        row={detailRow}
        onClose={() => setDetailOpen(false)}
        onSaved={async () => {
          await actionRef.current?.reload();
        }}
      />
    </PageContainer>
  );
}
