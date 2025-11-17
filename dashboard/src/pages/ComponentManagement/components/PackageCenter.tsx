import React, { Suspense } from 'react';

const PacksPage = React.lazy(() => import('@/pages/Packs'));

export default function PackageCenter() {
  return (
    <Suspense fallback={<div style={{ padding: 24 }}>加载组件包...</div>}>
      <PacksPage />
    </Suspense>
  );
}
