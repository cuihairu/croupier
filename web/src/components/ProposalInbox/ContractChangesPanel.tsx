import React, { useCallback, useState } from 'react';
import { App, Button, Dropdown, Popconfirm, Space, Table, Tag, Tooltip, Typography } from 'antd';
import {
  DeleteOutlined,
  MoreOutlined,
  ReloadOutlined,
  RocketOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { ContractChangeInfo, PageType } from '@/types/dashboard';
import MergeConflictModal from '@/components/MergeConflictModal';
import { mergeChanges, regenerateProposal, republish } from '@/services/dashboard';
import { deleteVersioningPage } from '@/services/api/versioning';
import type { ConflictResolution, MergeResponse } from '@/services/api/versioning';
import { publishPageDraft } from '@/services/api/pages';
import { extractErrorMessage } from '@/utils/errors';
import { requestConsoleMenuRefresh } from '@/utils/consoleMenu';
import { localizedText } from '@/utils/localizedText';
import { navigateTo } from './urlFocus';
import { diagnosticSummary, formatDate, pageTypeColors, pageTypeLabels } from './shared';

const { Text } = Typography;

/** 契约变更队列整组：stale 草稿/已发布页面的重发布、重生成、合并、删除与手动冲突处理。
 * 动作 state 与 MergeConflictModal 内聚；完成后经 onChanged 通知主页刷新队列。 */
export default function ContractChangesPanel({
  records,
  loading,
  focusPageKey,
  onChanged,
}: {
  records: ContractChangeInfo[];
  loading: boolean;
  focusPageKey?: string;
  onChanged: () => Promise<void>;
}) {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const [contractActionKey, setContractActionKey] = useState('');
  const [manualMergeVisible, setManualMergeVisible] = useState(false);
  const [manualMergeLoading, setManualMergeLoading] = useState(false);
  const [manualMergePreview, setManualMergePreview] = useState<MergeResponse | null>(null);
  const [manualMergeRecord, setManualMergeRecord] = useState<ContractChangeInfo | null>(null);

  const runContractAction = useCallback(
    async (key: string, action: () => Promise<void>) => {
      setContractActionKey(key);
      try {
        await action();
        await onChanged();
      } finally {
        setContractActionKey('');
      }
    },
    [onChanged],
  );

  const handleRegenerateProposal = useCallback(
    async (record: ContractChangeInfo) => {
      await runContractAction(`regenerate:${record.pageKey}`, async () => {
        try {
          const result = await regenerateProposal(record.pageKey);
          modal.success({
            title: intl.formatMessage({
              id: 'component.proposalInbox.contractChanges.action.regenerateSuccessTitle',
              defaultMessage: '已重新生成 Proposal',
            }),
            content: result.message,
          });
        } catch (e) {
          // 生成失败（函数禁用/schema 非法等错误级诊断）必须显式反馈，
          // 静默成功会让用户误以为报错已修复。
          modal.error({
            title: intl.formatMessage({
              id: 'component.proposalInbox.contractChanges.action.regenerateFailedTitle',
              defaultMessage: '重新生成失败',
            }),
            content: extractErrorMessage(
              e,
              intl.formatMessage({
                id: 'component.proposalInbox.contractChanges.action.regenerateFailedFallback',
                defaultMessage: '生成页面时出现错误级诊断，请查看页面详情',
              }),
            ),
          });
          throw e;
        }
      });
    },
    [intl, modal, runContractAction],
  );

  const handleDeletePage = useCallback(
    async (record: ContractChangeInfo) => {
      modal.confirm({
        title: intl.formatMessage({
          id: 'component.proposalInbox.contractChanges.action.deleteConfirmTitle',
          defaultMessage: '删除页面',
        }),
        content: intl.formatMessage(
          {
            id: 'component.proposalInbox.contractChanges.action.deleteConfirmContent',
            defaultMessage: `将删除页面 ${record.pageKey} 的草稿、已发布版本与待审提案，且不可恢复。确认删除？`,
          },
          { pageKey: record.pageKey },
        ),
        okType: 'danger',
        okText: intl.formatMessage({
          id: 'component.proposalInbox.contractChanges.action.deleteOkText',
          defaultMessage: '删除',
        }),
        onOk: async () => {
          await runContractAction(`delete:${record.pageKey}`, async () => {
            await deleteVersioningPage(record.pageKey);
            modal.success({
              title: intl.formatMessage({
                id: 'component.proposalInbox.contractChanges.action.deletedTitle',
                defaultMessage: '页面已删除',
              }),
            });
          });
        },
      });
    },
    [intl, modal, runContractAction],
  );

  const handleAutoMerge = useCallback(
    async (record: ContractChangeInfo) => {
      await runContractAction(`merge:${record.pageKey}`, async () => {
        const result = await mergeChanges(record.pageKey, { strategy: 'auto' });
        modal.info({
          title: intl.formatMessage({
            id: 'component.proposalInbox.contractChanges.action.autoMergeTitle',
            defaultMessage: '自动合并结果',
          }),
          content: intl.formatMessage(
            {
              id: 'component.proposalInbox.contractChanges.action.autoMergeContent',
              defaultMessage: `${result.message}。安全合并 ${result.merged} 项，仍有 ${result.conflicts} 项需要人工处理。`,
            },
            { message: result.message, merged: result.merged, conflicts: result.conflicts },
          ),
        });
      });
    },
    [intl, modal, runContractAction],
  );

  const handleOpenManualMerge = useCallback(
    async (record: ContractChangeInfo) => {
      setManualMergeLoading(true);
      try {
        const preview = await mergeChanges(record.pageKey, { strategy: 'manual', dryRun: true });
        setManualMergeRecord(record);
        setManualMergePreview(preview);
        setManualMergeVisible(true);
      } catch {
        message.error(
          intl.formatMessage({
            id: 'component.proposalInbox.contractChanges.action.mergePreviewFailed',
            defaultMessage: '加载冲突预览失败',
          }),
        );
      } finally {
        setManualMergeLoading(false);
      }
    },
    [intl, message],
  );

  const handleManualMergeSubmit = useCallback(
    async (payload: { conflicts: ConflictResolution[]; reason?: string }) => {
      if (!manualMergeRecord) {
        return;
      }
      setManualMergeLoading(true);
      try {
        const result = await mergeChanges(manualMergeRecord.pageKey, {
          strategy: 'manual',
          conflicts: payload.conflicts,
          reason: payload.reason,
        });
        setManualMergeVisible(false);
        setManualMergePreview(null);
        setManualMergeRecord(null);
        await onChanged();
        modal.success({
          title: intl.formatMessage({
            id: 'component.proposalInbox.contractChanges.action.manualMergeSuccessTitle',
            defaultMessage: '冲突已处理',
          }),
          content: intl.formatMessage(
            {
              id: 'component.proposalInbox.contractChanges.action.manualMergeSuccessContent',
              defaultMessage: `页面 ${manualMergeRecord.pageKey} 的草稿已更新到版本 ${result.draftRevision || '-'}，请确认后重新发布。`,
            },
            { pageKey: manualMergeRecord.pageKey, draftRevision: result.draftRevision || '-' },
          ),
        });
      } catch {
        message.error(
          intl.formatMessage({
            id: 'component.proposalInbox.contractChanges.action.manualMergeFailed',
            defaultMessage: '手动合并失败',
          }),
        );
      } finally {
        setManualMergeLoading(false);
      }
    },
    [intl, manualMergeRecord, message, modal, onChanged],
  );

  const handleRepublish = useCallback(
    async (record: ContractChangeInfo) => {
      await runContractAction(`republish:${record.pageKey}`, async () => {
        if (record.draftRevision && record.draftRevision > 0) {
          const result = await publishPageDraft(record.pageKey, record.draftRevision);
          requestConsoleMenuRefresh();
          modal.success({
            title: intl.formatMessage({
              id: 'component.proposalInbox.contractChanges.action.republishSuccessTitle',
              defaultMessage: '已重新发布',
            }),
            content: intl.formatMessage(
              {
                id: 'component.proposalInbox.contractChanges.action.republishSuccessContent',
                defaultMessage: `页面 ${result.pageKey} 已发布，版本 ${result.publishedVersion}。`,
              },
              { pageKey: result.pageKey, publishedVersion: result.publishedVersion },
            ),
          });
          return;
        }
        const result = await republish(record.pageKey);
        requestConsoleMenuRefresh();
        modal.success({
          title: intl.formatMessage({
            id: 'component.proposalInbox.contractChanges.action.republishSuccessTitle',
            defaultMessage: '已重新发布',
          }),
          content: result.message,
        });
      });
    },
    [intl, modal, runContractAction],
  );

  const contractColumns: ColumnsType<ContractChangeInfo> = [
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.contractChanges.column.page',
        defaultMessage: '页面',
      }),
      dataIndex: 'pageKey',
      key: 'pageKey',
      render: (_, record) => (
        <Space orientation="vertical" size={0}>
          <Text strong>{localizedText(record.title, record.pageKey)}</Text>
          <Text type="secondary">{record.pageKey}</Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.contractChanges.column.type',
        defaultMessage: '类型',
      }),
      dataIndex: 'pageType',
      key: 'pageType',
      width: 90,
      render: (type: PageType) => (
        <Tag color={pageTypeColors[type]}>
          {pageTypeLabels[type] ? intl.formatMessage(pageTypeLabels[type]) : type}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.contractChanges.column.resource',
        defaultMessage: '对象',
      }),
      dataIndex: 'resourceKey',
      key: 'resourceKey',
      width: 140,
      render: (value) => value || '-',
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.contractChanges.column.location',
        defaultMessage: '位置',
      }),
      dataIndex: 'kind',
      key: 'kind',
      width: 100,
      render: (kind: ContractChangeInfo['kind']) => (
        <Tag color={kind === 'published' ? 'error' : 'warning'}>
          {kind === 'published'
            ? intl.formatMessage({
                id: 'component.proposalInbox.contractChanges.column.kindPublished',
                defaultMessage: '已发布',
              })
            : intl.formatMessage({
                id: 'component.proposalInbox.contractChanges.column.kindDraft',
                defaultMessage: '草稿',
              })}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.contractChanges.column.reason',
        defaultMessage: '变更原因',
      }),
      dataIndex: 'bindingFreshness',
      key: 'bindingFreshness',
      render: (_, record) => {
        const diagnostics = record.bindingFreshness?.map((item) => item.diagnostic) || [];
        return diagnosticSummary(intl, diagnostics);
      },
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.contractChanges.column.updatedAt',
        defaultMessage: '更新时间',
      }),
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 180,
      render: formatDate,
    },
    {
      title: intl.formatMessage({
        id: 'component.proposalInbox.contractChanges.column.actions',
        defaultMessage: '操作',
      }),
      key: 'action',
      width: 208,
      fixed: 'right',
      render: (_, record) => (
        <Space size={0}>
          <Popconfirm
            title={intl.formatMessage({
              id: 'component.proposalInbox.contractChanges.action.republishConfirm',
              defaultMessage: '确认重新发布当前草稿快照？',
            })}
            onConfirm={() => handleRepublish(record)}
          >
            <Button
              type="link"
              size="small"
              icon={<RocketOutlined />}
              loading={contractActionKey === `republish:${record.pageKey}`}
            >
              <FormattedMessage
                id="component.proposalInbox.contractChanges.action.republish"
                defaultMessage="重发布"
              />
            </Button>
          </Popconfirm>
          <Button
            type="link"
            size="small"
            onClick={() =>
              navigateTo(`/functions/pages?focus=${encodeURIComponent(record.pageKey)}`)
            }
          >
            <FormattedMessage
              id="component.proposalInbox.contractChanges.action.edit"
              defaultMessage="编辑"
            />
          </Button>
          <Dropdown
            trigger={['click']}
            menu={{
              items: [
                {
                  key: 'regenerate',
                  icon: <ReloadOutlined />,
                  label: intl.formatMessage({
                    id: 'component.proposalInbox.contractChanges.action.regenerate',
                    defaultMessage: '重生成',
                  }),
                  onClick: () => handleRegenerateProposal(record),
                },
                {
                  key: 'auto-merge',
                  icon: <SyncOutlined />,
                  label: intl.formatMessage({
                    id: 'component.proposalInbox.contractChanges.action.autoMerge',
                    defaultMessage: '自动合并',
                  }),
                  onClick: () => handleAutoMerge(record),
                },
                { type: 'divider' },
                {
                  key: 'delete-page',
                  icon: <DeleteOutlined />,
                  label: intl.formatMessage({
                    id: 'component.proposalInbox.contractChanges.action.deletePage',
                    defaultMessage: '删除页面',
                  }),
                  danger: true,
                  onClick: () => handleDeletePage(record),
                },
                {
                  key: 'manual-merge',
                  label: intl.formatMessage({
                    id: 'component.proposalInbox.contractChanges.action.resolveConflicts',
                    defaultMessage: '处理冲突',
                  }),
                  onClick: () => handleOpenManualMerge(record),
                },
              ],
            }}
          >
            <Tooltip
              title={intl.formatMessage({
                id: 'component.proposalInbox.contractChanges.action.more',
                defaultMessage: '更多',
              })}
            >
              <Button type="link" size="small" icon={<MoreOutlined />} />
            </Tooltip>
          </Dropdown>
        </Space>
      ),
    },
  ];

  return (
    <>
      <Table
        columns={contractColumns}
        dataSource={records}
        rowKey={(record) => `${record.kind}:${record.pageKey}`}
        rowClassName={(record) =>
          record.pageKey === focusPageKey ? 'proposal-inbox-focus-row' : ''
        }
        loading={loading}
        scroll={{ x: 'max-content' }}
      />
      <MergeConflictModal
        open={manualMergeVisible}
        loading={manualMergeLoading}
        preview={manualMergePreview}
        onCancel={() => setManualMergeVisible(false)}
        onSubmit={handleManualMergeSubmit}
      />
    </>
  );
}
