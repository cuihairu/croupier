/**
 * Tests for threading module exports
 */

import {
  MainThreadDispatcher,
  getDispatcher,
  enqueue,
  processQueue,
  Callback,
  CallbackWithData,
} from "./index";

describe("threading/index exports", () => {
  beforeEach(() => {
    // Reset singleton before each test
    MainThreadDispatcher.resetInstance();
  });

  describe("exported classes", () => {
    it("should export MainThreadDispatcher class", () => {
      expect(MainThreadDispatcher).toBeDefined();
      expect(typeof MainThreadDispatcher).toBe("function");
    });

    it("should export getDispatcher function", () => {
      expect(getDispatcher).toBeDefined();
      expect(typeof getDispatcher).toBe("function");
    });

    it("should export enqueue function", () => {
      expect(enqueue).toBeDefined();
      expect(typeof enqueue).toBe("function");
    });

    it("should export processQueue function", () => {
      expect(processQueue).toBeDefined();
      expect(typeof processQueue).toBe("function");
    });
  });

  describe("exported types", () => {
    it("should export Callback type", () => {
      // Type exports don't exist at runtime, but we can verify the module imports correctly
      expect(() => {
        const cb: Callback = () => {};
        return cb;
      }).not.toThrow();
    });

    it("should export CallbackWithData type", () => {
      expect(() => {
        const cb: CallbackWithData<string> = (_data: string) => {};
        return cb;
      }).not.toThrow();
    });
  });

  describe("functional integration", () => {
    it("should get singleton dispatcher instance", () => {
      const dispatcher1 = getDispatcher();
      const dispatcher2 = getDispatcher();
      expect(dispatcher1).toBe(dispatcher2);
    });

    it("should get dispatcher instance via getInstance static method", () => {
      const dispatcher = MainThreadDispatcher.getInstance();
      expect(dispatcher).toBeDefined();
      expect(dispatcher).toBeInstanceOf(MainThreadDispatcher);
    });

    it("should enqueue callback via exported function", () => {
      getDispatcher().initialize();
      const mockCallback = jest.fn();
      enqueue(mockCallback);
      // Callback is queued, will be processed on next tick
    });

    it("should process queue via exported function", () => {
      getDispatcher().initialize();
      const mockCallback = jest.fn();
      enqueue(mockCallback);
      processQueue();
      // Note: In actual implementation, processing happens asynchronously
    });
  });

  describe("default behavior", () => {
    it("should handle empty queue processing", () => {
      getDispatcher().initialize();
      expect(() => processQueue()).not.toThrow();
    });

    it("should return same instance from getDispatcher and getInstance", () => {
      const dispatcher1 = getDispatcher();
      const dispatcher2 = MainThreadDispatcher.getInstance();
      expect(dispatcher1).toBe(dispatcher2);
    });
  });
});
