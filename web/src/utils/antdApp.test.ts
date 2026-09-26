import { clearAppApi, getMessage, getModal, getNotification, setAppApi, type AppApi } from './antdApp';

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

  describe('clearAppApi（BUG-022）', () => {
    function makeApi(message: unknown): AppApi {
      return {
        message,
        notification: undefined,
        modal: undefined,
      } as unknown as AppApi;
    }

    it('清除当前实例后 getters 回落 undefined（卸载 <AntdApp> 不留死实例）', () => {
      const api = makeApi({ success: jest.fn() });
      setAppApi(api);
      expect(getMessage()).toBeDefined();

      clearAppApi(api);
      expect(getMessage()).toBeUndefined();
      expect(getNotification()).toBeUndefined();
      expect(getModal()).toBeUndefined();
    });

    it('仅当仍指向同一实例时才清——不误清后来者', () => {
      const first = makeApi({ success: jest.fn() });
      const second = makeApi({ success: jest.fn() });
      setAppApi(first);
      setAppApi(second);

      clearAppApi(first); // 旧实例的卸载清理不得影响新注册
      expect(getMessage()).toBe(second.message);

      clearAppApi(second);
      expect(getMessage()).toBeUndefined();
    });

    it('未注册时 clear 是 no-op', () => {
      setAppApi(null as unknown as AppApi);
      expect(() => clearAppApi(null as unknown as AppApi)).not.toThrow();
      expect(getMessage()).toBeUndefined();
    });
  });
});
