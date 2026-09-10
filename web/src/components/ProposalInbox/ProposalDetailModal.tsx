import React from 'react';
import { Descriptions, Modal, Space, Table, Tag, Typography } from 'antd';
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
  return (
    <Modal title="提案详情" open={open} onCancel={onClose} footer={null} width={920}>
      {proposal && (
        <Space orientation="vertical" style={{ width: '100%' }} size={16}>
          <Descriptions column={2} bordered>
            <Descriptions.Item label="提案">{proposal.proposalKey}</Descriptions.Item>
            <Descriptions.Item label="页面">{proposal.pageKey}</Descriptions.Item>
            <Descriptions.Item label="标题">
              {localizedText(proposal.title, proposal.pageKey)}
            </Descriptions.Item>
            <Descriptions.Item label="类型">
              <Tag color={pageTypeColors[proposal.pageType]}>
                {pageTypeLabels[proposal.pageType]}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="质量">
              <Tag color={qualityColors[proposal.quality]}>{qualityLabels[proposal.quality]}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={statusColors[proposal.status]}>{statusLabels[proposal.status]}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="函数摘要">{proposal.functionDigest || '-'}</Descriptions.Item>
            <Descriptions.Item label="语义摘要">
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
                { title: '代码', dataIndex: 'code', key: 'code' },
                {
                  title: '级别',
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
                  title: '字段',
                  dataIndex: 'field',
                  key: 'field',
                  render: (value) => value || '-',
                },
                { title: '说明', dataIndex: 'message', key: 'message' },
              ]}
            />
          )}
          <Paragraph type="secondary">
            PageSpec 为发布快照输入，不在正常路径手工编辑 JSON；如需调整请进入 Page Studio 编辑器。
          </Paragraph>
        </Space>
      )}
    </Modal>
  );
}
