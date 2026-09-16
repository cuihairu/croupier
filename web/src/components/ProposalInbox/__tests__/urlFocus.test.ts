/**
 * ProposalInbox URL focus 工具：
 * currentResourceKey/currentProposalKey 读 query 参数（缺省空串）、
 * clearProposalKeyParam 仅在存在 proposalKey 时 replaceState 收口 URL、
 * navigateTo 走 location.assign（jsdom Location 全属性 unforgeable 不可
 * 替换/spy，以 jsdom 导航层的 Not implemented 信号证明 assign 被调用）。
 */
import {
  clearProposalKeyParam,
  currentProposalKey,
  currentResourceKey,
  navigateTo,
} from '../urlFocus';

const replaceStateSpy = jest.spyOn(window.history, 'replaceState');

beforeEach(() => {
  window.history.pushState({}, '', '/functions/proposals');
  replaceStateSpy.mockClear();
});

describe('currentResourceKey / currentProposalKey', () => {
  it('无参数时返回空串', () => {
    expect(currentResourceKey()).toBe('');
    expect(currentProposalKey()).toBe('');
  });

  it('读取对应 query 参数', () => {
    window.history.pushState({}, '', '/functions/proposals?resourceKey=players&proposalKey=p-1');
    expect(currentResourceKey()).toBe('players');
    expect(currentProposalKey()).toBe('p-1');
  });
});

describe('clearProposalKeyParam', () => {
  it('URL 无 proposalKey 时不做 rewrite', () => {
    clearProposalKeyParam();
    expect(replaceStateSpy).not.toHaveBeenCalled();
  });

  it('存在 proposalKey 时移除并保留其他参数与 hash', () => {
    window.history.pushState({}, '', '/inbox?resourceKey=players&proposalKey=p-1#anchor');
    clearProposalKeyParam();
    expect(replaceStateSpy).toHaveBeenCalledWith(null, '', '/inbox?resourceKey=players#anchor');
  });
});

describe('navigateTo', () => {
  it('调用 location.assign（到达 jsdom 导航层触发 Not implemented 信号）', () => {
    const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
    navigateTo('/functions/pages?focus=k');
    // jsdom 未实现导航：真实走到 assign 才会触发 Not implemented
    // （pushState 不产生导航错误，该信号唯一来源即 assign 调用）
    const logged = errorSpy.mock.calls.map((args) => args.join(' ')).join('\n');
    expect(logged).toContain('Not implemented: navigation');
    errorSpy.mockRestore();
  });
});
