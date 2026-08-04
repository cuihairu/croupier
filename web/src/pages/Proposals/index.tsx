import React from 'react';
import { PageContainer } from '@ant-design/pro-components';
import ProposalInbox from '@/components/ProposalInbox';

export default function ProposalsPage() {
  return (
    <PageContainer
      title="默认页面提案"
      subTitle="查看、预览并发布平台生成的默认页面"
    >
      <ProposalInbox />
    </PageContainer>
  );
}
