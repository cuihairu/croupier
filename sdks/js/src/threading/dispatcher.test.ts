/**
 * Copyright 2025 Croupier Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {
  MainThreadDispatcher,
  getDispatcher,
  enqueue,
  processQueue,
} from "./dispatcher";

describe("MainThreadDispatcher", () => {
  beforeEach(() => {
    MainThreadDispatcher.resetInstance();
    getDispatcher().initialize();
  });

  afterEach(() => {
    MainThreadDispatcher.resetInstance();
  });

  describe("initialization", () => {
    it("should be initialized after initialize() call", () => {
      expect(getDispatcher().isInitialized()).toBe(true);
    });

    it("should be on main thread after initialization", () => {
      expect(getDispatcher().isMainThread()).toBe(true);
    });
  });

  describe("enqueue", () => {
    it("should execute immediately when initialized (default behavior)", () => {
      let executed = false;
      getDispatcher().enqueue(() => {
        executed = true;
      });
      expect(executed).toBe(true);
    });

    it("should queue for later when executeImmediatelyIfInitialized is false", () => {
      let executed = false;
      getDispatcher().enqueue(() => {
        executed = true;
      }, false);

      expect(executed).toBe(false);
      expect(getDispatcher().getPendingCount()).toBe(1);

      const processed = getDispatcher().processQueue();
      expect(processed).toBe(1);
      expect(executed).toBe(true);
    });

    it("should ignore null callbacks", () => {
      const initialCount = getDispatcher().getPendingCount();
      getDispatcher().enqueue(null);
      expect(getDispatcher().getPendingCount()).toBe(initialCount);
    });
  });

  describe("enqueueDeferred", () => {
    it("should always queue and never execute immediately", () => {
      let executed = false;
      getDispatcher().enqueueDeferred(() => {
        executed = true;
      });

      expect(executed).toBe(false);
      expect(getDispatcher().getPendingCount()).toBe(1);

      getDispatcher().processQueue();
      expect(executed).toBe(true);
    });
  });

  describe("enqueueWithData", () => {
    it("should pass data correctly to callback", () => {
      let receivedData: string | null = null;

      getDispatcher().enqueueWithData((data: string) => {
        receivedData = data;
      }, "test-data");

      getDispatcher().processQueue();
      expect(receivedData).toBe("test-data");
    });

    it("should ignore null callbacks", () => {
      const initialCount = getDispatcher().getPendingCount();
      getDispatcher().enqueueWithData(null, "test-data");
      expect(getDispatcher().getPendingCount()).toBe(initialCount);
    });
  });

  describe("processQueue", () => {
    it("should return 0 when queue is empty", () => {
      expect(getDispatcher().processQueue()).toBe(0);
    });

    it("should process all queued callbacks", () => {
      let count = 0;
      for (let i = 0; i < 5; i++) {
        getDispatcher().enqueueDeferred(() => {
          count++;
        });
      }

      const processed = getDispatcher().processQueue();
      expect(processed).toBe(5);
      expect(count).toBe(5);
      expect(getDispatcher().getPendingCount()).toBe(0);
    });

    it("should respect max count limit", () => {
      let count = 0;
      for (let i = 0; i < 10; i++) {
        getDispatcher().enqueueDeferred(() => {
          count++;
        });
      }

      expect(getDispatcher().getPendingCount()).toBe(10);

      const processed = getDispatcher().processQueueWithLimit(5);
      expect(processed).toBe(5);
      expect(count).toBe(5);
      expect(getDispatcher().getPendingCount()).toBe(5);

      // Process remaining
      const remaining = getDispatcher().processQueue();
      expect(remaining).toBe(5);
      expect(count).toBe(10);
    });

    it("should handle exceptions gracefully", () => {
      const results: number[] = [];
      const consoleSpy = jest
        .spyOn(console, "error")
        .mockImplementation(() => {});

      getDispatcher().enqueueDeferred(() => results.push(1));
      getDispatcher().enqueueDeferred(() => {
        throw new Error("Test error");
      });
      getDispatcher().enqueueDeferred(() => results.push(3));

      const processed = getDispatcher().processQueue();

      expect(processed).toBe(3);
      expect(results).toEqual([1, 3]);
      expect(consoleSpy).toHaveBeenCalled();

      consoleSpy.mockRestore();
    });
  });

  describe("clear", () => {
    it("should remove all pending callbacks", () => {
      for (let i = 0; i < 5; i++) {
        getDispatcher().enqueueDeferred(() => {});
      }
      expect(getDispatcher().getPendingCount()).toBe(5);

      getDispatcher().clear();
      expect(getDispatcher().getPendingCount()).toBe(0);
    });
  });

  describe("setMaxProcessPerFrame", () => {
    it("should limit processing", () => {
      getDispatcher().setMaxProcessPerFrame(3);

      for (let i = 0; i < 10; i++) {
        getDispatcher().enqueueDeferred(() => {});
      }

      const processed = getDispatcher().processQueue();
      expect(processed).toBe(3);

      // Reset to default
      getDispatcher().setMaxProcessPerFrame(0);
    });
  });

  describe("singleton", () => {
    it("should return the same instance", () => {
      const instance1 = MainThreadDispatcher.getInstance();
      const instance2 = MainThreadDispatcher.getInstance();
      expect(instance1).toBe(instance2);
    });

    it("should reset properly", () => {
      getDispatcher().enqueueDeferred(() => {});
      expect(getDispatcher().getPendingCount()).toBe(1);

      MainThreadDispatcher.resetInstance();
      expect(getDispatcher().getPendingCount()).toBe(0);
      expect(getDispatcher().isInitialized()).toBe(false);
    });
  });

  describe("convenience functions", () => {
    it("enqueue should work correctly", () => {
      let executed = false;
      enqueue(() => {
        executed = true;
      });
      expect(executed).toBe(true);
    });

    it("processQueue should work correctly", () => {
      getDispatcher().enqueueDeferred(() => {});
      const processed = processQueue();
      expect(processed).toBe(1);
    });
  });
});
