import React from 'react';
import { Descriptions, Modal, Space, Table, Tag, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { DiagnosticInfo, PageProposal } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';
import {
  pageTypeColors,
  pageTypeLabels,
  qualityColors,
  qualityLabels,
  statusColors,
  statusLabels,
} from './shared';

const { Paragraph } = Typography;

/** 提案详情弹窗：元信息 Descriptions + 诊断表（数据由主页拉取注入）。 */
export default function ProposalDetailModal({
  open,
  proposal,
  onClose,
}: {
  open: boolean;
  proposal: PageProposal | null;
  onClose: () => void;
}) {
  const intl = useIntl();
  return (
    <Modal
      title={intl.formatMessage({
        id: 'component.proposalInbox.detail.title',
        defaultMessage: '提案详情',
      })}
      open={open}
      onCancel={onClose}
      footer={null}
      width={920}
    >
      {proposal && (
        <Space orientation="vertical" style={{ width: '100%' }} size={16}>
          <Descriptions column={2} bordered>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.proposalInbox.detail.proposal',
                defaultMessage: '提案',
              })}
            >
              {proposal.proposalKey}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.proposalInbox.detail.page',
                defaultMessage: '页面',
              })}
            >
              {proposal.pageKey}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.proposalInbox.detail.pageTitle',
                defaultMessage: '标题',
              })}
            >
              {localizedText(proposal.title, proposal.pageKey)}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.proposalInbox.detail.pageType',
                defaultMessage: '类型',
              })}
            >
              <Tag color={pageTypeColors[proposal.pageType]}>
                {intl.formatMessage(pageTypeLabels[proposal.pageType])}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.proposalInbox.detail.quality',
                defaultMessage: '质量',
              })}
            >
              <Tag color={qualityColors[proposal.quality]}>
                {intl.formatMessage(qualityLabels[proposal.quality])}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.proposalInbox.detail.status',
                defaultMessage: '状态',
              })}
            >
              <Tag color={statusColors[proposal.status]}>
                {intl.formatMessage(statusLabels[proposal.status])}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.proposalInbox.detail.functionDigest',
                defaultMessage: '函数摘要',
              })}
            >
              {proposal.functionDigest || '-'}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.proposalInbox.detail.semanticsDigest',
                defaultMessage: '语义摘要',
              })}
            >
              {proposal.semanticsDigest || '-'}
            </Descriptions.Item>
          </Descriptions>
          {proposal.diagnostics && proposal.diagnostics.length > 0 && (
            <Table
              size="small"
              dataSource={proposal.diagnostics}
              rowKey={(record) => `${record.code}:${record.field || ''}:${record.functionId || ''}`}
              pagination={false}
              columns={[
                {
                  title: intl.formatMessage({
                    id: 'component.proposalInbox.detail.diagnosticCode',
                    defaultMessage: '代码',
                  }),
                  dataIndex: 'code',
                  key: 'code',
                },
                {
                  title: intl.formatMessage({
                    id: 'component.proposalInbox.detail.diagnosticLevel',
                    defaultMessage: '级别',
                  }),
                  dataIndex: 'severity',
                  key: 'severity',
                  render: (severity: DiagnosticInfo['severity']) => (
                    <Tag
                      color={
                        severity === 'error' ? 'error' : severity === 'warning' ? 'warning' : 'blue'
                      }
                    >
                      {severity}
                    </Tag>
                  ),
                },
                {
                  title: intl.formatMessage({
                    id: 'component.proposalInbox.detail.diagnosticField',
                    defaultMessage: '字段',
                  }),
                  dataIndex: 'field',
                  key: 'field',
                  render: (value) => value || '-',
                },
                {
                  title: intl.formatMessage({
                    id: 'component.proposalInbox.detail.diagnosticMessage',
                    defaultMessage: '说明',
                  }),
                  dataIndex: 'message',
                  key: 'message',
                },
              ]}
            />
          )}
          <Paragraph type="secondary">
            <FormattedMessage
              id="component.proposalInbox.detail.pagespecNote"
              defaultMessage="PageSpec 为发布快照输入，不在正常路径手工编辑 JSON；如需调整请进入 Page Studio 编辑器。"
            />
          </Paragraph>
        </Space>
      )}
    </Modal>
  );
}
