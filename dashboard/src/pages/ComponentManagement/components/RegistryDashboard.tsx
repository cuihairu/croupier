import React, { Suspense } from 'react';

const Registry = React.lazy(() => import('@/pages/Registry'));

export default function RegistryDashboard() {
  return (
    <Suspense fallback={<div style={{ padding: 24 }}>加载注册表...</div>}>
      <Registry />
    </Suspense>
  );
}
