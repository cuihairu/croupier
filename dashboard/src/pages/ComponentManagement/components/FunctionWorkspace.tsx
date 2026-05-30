import React, { Suspense } from 'react';

const GmFunctions = React.lazy(() => import('@/pages/GmFunctions'));

export default function FunctionWorkspace() {
  return (
    <Suspense fallback={<div style={{ padding: 24 }}>加载函数工作台...</div>}>
      <GmFunctions />
    </Suspense>
  );
}
