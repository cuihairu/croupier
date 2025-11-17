import React from 'react';
import AnalyticsComingSoon from '../ComingSoon';

export default function PaymentsPage() {
  return (
    <AnalyticsComingSoon
      pageName="Payments Analytics"
      description="支付看板将展示交易流水、渠道分布等信息，当前暂用占位。"
      checklist={[
        { title: 'analytics.payments 表已有数据', detail: 'worker 定时落库' },
        { title: '税率配置在 configs/analytics/metrics.yaml', detail: '确保收入指标准确' },
      ]}
    />
  );
}
