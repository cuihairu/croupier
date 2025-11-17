import React from 'react';
import AnalyticsComingSoon from '../ComingSoon';

export default function LevelsPage() {
  return (
    <AnalyticsComingSoon
      pageName="Levels Analytics"
      description="关卡分析页面将展示关卡通关率、重试次数与热力图。等待游戏端埋点稳定后开放。"
      checklist={[
        { title: 'analytics.levels_* 相关聚合在 worker 中实现', detail: '需要 episodes/maps 维度' },
        { title: 'configs/analytics/metrics.yaml 对关卡做单位说明', detail: '供 UI 展示' },
      ]}
    />
  );
}
