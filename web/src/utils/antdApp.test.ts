import { getMessage, getModal, getNotification, setAppApi, type AppApi } from './antdApp';

describe('antdApp', () => {
  it('未注入前 getters 返回 undefined', () => {
    expect(getMessage()).toBeUndefined();
    expect(getNotification()).toBeUndefined();
    expect(getModal()).toBeUndefined();
  });

  it('setAppApi 注入后 getters 返回同一实例', () => {
    const message = { success: jest.fn(), error: jest.fn() };
    const notification = { open: jest.fn(), close: jest.fn() };
    const modal = { confirm: jest.fn(), info: jest.fn() };
    setAppApi({ message, notification, modal } as unknown as AppApi);

    expect(getMessage()).toBe(message);
    expect(getNotification()).toBe(notification);
    expect(getModal()).toBe(modal);
  });

  it('重复注入以后一次为准（缺省槽位回落 undefined）', () => {
    const replacement = { success: jest.fn(), error: jest.fn() };
    setAppApi({
      message: replacement,
      notification: undefined,
      modal: undefined,
    } as unknown as AppApi);

    expect(getMessage()).toBe(replacement);
    expect(getNotification()).toBeUndefined();
    expect(getModal()).toBeUndefined();
  });
});
