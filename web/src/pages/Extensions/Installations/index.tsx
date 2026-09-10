import { useCallback, useEffect, useState } from 'react';
import { Alert, App, Button, Input, Modal, Select, Space, Table, Tag, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
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
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<ExtensionInstallationItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [status, setStatus] = useState<string | undefined>(undefined);
  const [extensionID, setExtensionID] = useState('');

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
    extensionID.trim() ? `扩展 ${extensionID.trim()}` : null,
    status ? `状态 ${status}` : null,
  ]
    .filter(Boolean)
    .join(' / ');

  const loadInstallations = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await listExtensionInstallations({
        extensionId: extensionID.trim() || undefined,
        status,
        page,
        pageSize,
      });
      const vm = adaptInstallationListResponse(resp);
      setItems(vm.items);
      setTotal(vm.total);
    } finally {
      setLoading(false);
    }
  }, [extensionID, status, page, pageSize]);

  useEffect(() => {
    loadInstallations();
  }, [loadInstallations]);

  const withReload = async (fn: () => Promise<unknown>, successText: string) => {
    await fn();
    message.success(successText);
    await loadInstallations();
  };

  const handleUninstall = (row: ExtensionInstallationItem) => {
    Modal.confirm({
      title: '确认卸载扩展',
      content: `安装实例 #${row.id} 将被卸载，是否继续？`,
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await uninstallExtension(row.id);
          message.success('已卸载扩展');
          await loadInstallations();
        } catch (err) {
          const uiErr = mapExtensionError(err as Error);
          const details = uiErr.details || {};
          const blockers = Array.isArray(details.blockers) ? details.blockers : [];
          if (uiErr.code === EXTENSION_ERROR_CODES.DEPENDENCY_BLOCKED && blockers.length > 0) {
            Modal.warning({
              title: '无法卸载：存在依赖',
              content: (
                <Space orientation="vertical">
                  <Text type="secondary">以下扩展仍依赖当前扩展，请先处理它们：</Text>
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
        row.enabled ? '已禁用扩展' : '已启用扩展',
      );
    },
    onReconcile: (row) => {
      void withReload(() => reconcileExtension(row.id), '已触发重建绑定');
    },
    onUninstall: handleUninstall,
  });

  return (
    <PageContainer title="扩展安装" subTitle="查看和管理已安装扩展实例">
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title="扩展概览"
          description="这个页面优先承担安装实例排查和运维动作，建议先按扩展或状态筛选，再进入详情、事件或升级流程。"
          items={[
            { color: '#1677ff', text: `总数 ${summary.total}` },
            { color: '#52c41a', text: `启用 ${summary.enabledCount}` },
            { color: '#d9d9d9', text: `禁用 ${summary.disabledCount}` },
            { color: '#13c2c2', text: `健康 ${summary.healthyCount}` },
            { color: '#722ed1', text: `作用域 ${summary.scopeCount}` },
          ]}
          hint="推荐路径：先在列表确认实例状态，再进入详情修改配置；事件和升级属于次级动作，不应抢主流程注意力。"
        />

        <StandardListSection
          title="安装列表"
          extra={<Button onClick={loadInstallations}>刷新</Button>}
        >
          <StandardFilterBar
            resultText={`当前结果 ${items.length} 个安装实例`}
            controls={
              <>
                <Input
                  style={{ width: 240 }}
                  allowClear
                  placeholder="扩展 ID"
                  value={extensionID}
                  onChange={(e) => {
                    setPage(1);
                    setExtensionID(e.target.value);
                  }}
                />
                <Select
                  style={{ width: 160 }}
                  allowClear
                  placeholder="状态"
                  value={status}
                  onChange={(v) => {
                    setPage(1);
                    setStatus(v);
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
                      setPage(1);
                      setExtensionID('');
                      setStatus(undefined);
                    }}
                  >
                    清空筛选
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
              message="当前正在查看筛选后的安装实例"
              description={`已生效条件：${listFilterSummary}`}
            />
          ) : null}

          <Table<ExtensionInstallationItem>
            scroll={{ x: 900 }}
            rowKey="id"
            loading={loading}
            dataSource={items}
            columns={columns}
            pagination={{
              current: page,
              pageSize,
              total,
              showSizeChanger: true,
              onChange: (nextPage, nextPageSize) => {
                setPage(nextPage);
                setPageSize(nextPageSize);
              },
            }}
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
        onUpgraded={loadInstallations}
      />

      <InstallationDetailDrawer
        open={detailOpen}
        row={detailRow}
        onClose={() => setDetailOpen(false)}
        onSaved={loadInstallations}
      />
    </PageContainer>
  );
}
