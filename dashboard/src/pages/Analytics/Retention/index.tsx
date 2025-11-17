import React from 'react';
import AnalyticsComingSoon from '../ComingSoon';

export default function RetentionPage() {
  return (
    <AnalyticsComingSoon
      pageName="Retention Analytics"
      description="用户留存 Cohort 视图建设中。待 ClickHouse cohort 计算逻辑验收后，将展示 DAU/WAU 与分 cohort 曲线。"
      checklist={[
        { title: 'analytics.daily_users 有 cohort 字段', detail: 'worker 定时任务填充' },
        { title: 'configs/analytics/metrics.yaml 中定义 retention_* 指标', detail: '确保 API 可返回所需字段' },
      ]}
    />
  );
}
