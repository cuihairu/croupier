import { useEffect } from 'react';
import { history, useLocation } from '@umijs/max';

/** 兼容旧链接；页面提案只保留在“页面工作台”这一入口。 */
export default function ProposalsPage() {
  const location = useLocation();

  useEffect(() => {
    history.replace(`/functions/pages${location.search}`);
  }, [location.search]);

  return null;
}
