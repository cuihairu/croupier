import React, { Suspense } from 'react';

const OpsJobsPage = React.lazy(() => import('@/pages/Ops/Jobs'));

export default function ExecutionMonitor() {
  return (
    <Suspense fallback={<div style={{ padding: 24 }}>加载执行监控...</div>}>
      <OpsJobsPage />
    </Suspense>
  );
}
