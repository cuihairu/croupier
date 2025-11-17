import React from 'react';
import AnalyticsComingSoon from '../ComingSoon';

export default function SegmentsPage() {
  return (
    <AnalyticsComingSoon
      pageName="Segment Explorer"
      description="用户分群管理界面建设中，将允许运营根据属性/行为创建 saved segments。"
      checklist={[
        { title: 'configs/analytics/segments.yaml 准备完毕', detail: '定义默认分组' },
        { title: 'segment materialization job 运行成功', detail: 'ClickHouse 中需要有 segment_members 表' },
      ]}
    />
  );
}
