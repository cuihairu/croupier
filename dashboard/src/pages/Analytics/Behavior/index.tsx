import React from 'react';
import AnalyticsComingSoon from '../ComingSoon';

export default function BehaviorPage() {
  return (
    <AnalyticsComingSoon
      pageName="Behavior Analytics"
      description="事件分析与漏斗页面暂未完成，后续计划接入路径图、Top Events 等组件。"
      checklist={[
        { title: 'events.yaml 中定义事件分组与属性', detail: '供 UI 快速筛选' },
        { title: 'analytics.events 表分区与 TTL OK', detail: '保证查询性能' },
      ]}
    />
  );
}
