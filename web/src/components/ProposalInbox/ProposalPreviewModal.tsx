import React from 'react';
import { Empty, Modal } from 'antd';
import type { PageProposal } from '@/types/dashboard';
import PageRenderer from '@/components/PageRenderer';

/** 默认页面预览弹窗：只读渲染 PageSpec，函数执行被显式拒绝。 */
export default function ProposalPreviewModal({
  open,
  proposal,
  onClose,
}: {
  open: boolean;
  proposal: PageProposal | null;
  onClose: () => void;
}) {
  return (
    <Modal title="默认页面预览" open={open} onCancel={onClose} footer={null} width={1100}>
      {proposal?.pageSpec ? (
        <PageRenderer
          pageSpec={proposal.pageSpec}
          preview
          onExecute={async () => {
            throw new Error('Proposal 预览不执行函数；发布后请在运行控制台执行。');
          }}
        />
      ) : (
        <Empty description="暂无可预览页面" />
      )}
    </Modal>
  );
}
