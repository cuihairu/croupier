/** ProposalInbox URL focus 参数工具：resourceKey 过滤与 proposalKey 直达预览。 */

export function currentResourceKey(): string {
  if (typeof window === 'undefined') {
    return '';
  }
  return new URLSearchParams(window.location.search).get('resourceKey') || '';
}

export function currentProposalKey(): string {
  if (typeof window === 'undefined') {
    return '';
  }
  return new URLSearchParams(window.location.search).get('proposalKey') || '';
}

export function clearProposalKeyParam() {
  if (typeof window === 'undefined') {
    return;
  }
  const url = new URL(window.location.href);
  if (!url.searchParams.has('proposalKey')) {
    return;
  }
  url.searchParams.delete('proposalKey');
  window.history.replaceState(null, '', `${url.pathname}${url.search}${url.hash}`);
}

export function navigateTo(path: string) {
  if (typeof window !== 'undefined') {
    window.location.assign(path);
  }
}
