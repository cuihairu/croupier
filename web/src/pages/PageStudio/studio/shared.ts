import { getIntl } from '@umijs/max';
import type { PageSpecDraftSummary, PageType } from '@/types/dashboard';

/** 页面工作台共享工具（列表列 / 版本历史 / 变更链共用）。 */

// 展示 label 经 getIntl 求值（工具函数无组件上下文；SelectLang 切语言整页
// 刷新后重新求值，先例 services/api/bugs.ts）；PageType 枚举值是逻辑契约不动
const intl = getIntl();

export function statusColor(status: PageSpecDraftSummary['status']) {
  if (status === 'published') return 'green';
  if (status === 'archived') return 'default';
  return 'blue';
}

export function formatDate(value?: string): string {
  if (!value) return '-';
  const time = new Date(value);
  return Number.isNaN(time.getTime()) ? value : time.toLocaleString();
}

export function pageTypeLabel(type: PageType): string {
  switch (type) {
    case 'resource':
      return intl.formatMessage({
        id: 'pages.pageStudio.studio.pageType.resource',
        defaultMessage: '资源页面',
      });
    case 'operation':
      return intl.formatMessage({
        id: 'pages.pageStudio.studio.pageType.operation',
        defaultMessage: '操作页面',
      });
    case 'task':
      return intl.formatMessage({
        id: 'pages.pageStudio.studio.pageType.task',
        defaultMessage: '任务页面',
      });
    case 'report':
      return intl.formatMessage({
        id: 'pages.pageStudio.studio.pageType.report',
        defaultMessage: '报表页面',
      });
    default:
      return type;
  }
}

/** URL ?focus= 参数：进入工作台时聚焦指定页面（ProposalInbox 联动）。 */
export function currentFocusPageKey(): string {
  if (typeof window === 'undefined') {
    return '';
  }
  return new URLSearchParams(window.location.search).get('focus') || '';
}
