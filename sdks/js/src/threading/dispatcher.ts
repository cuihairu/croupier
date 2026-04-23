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

/**
 * Main thread dispatcher - ensures callbacks execute on the main thread.
 *
 * In JavaScript's single-threaded event loop, this dispatcher is primarily useful for:
 * - Game engine integrations where callbacks need to be processed in a controlled manner
 * - Batching callbacks to prevent blocking during heavy processing
 * - Web Worker scenarios where messages need to be queued for main thread processing
 *
 * Usage:
 *   1. Call initialize() once at startup (typically in main entry)
 *   2. Call processQueue() in your main loop or animation frame
 *   3. Use enqueue() from event handlers or async callbacks
 *
 * Example:
 * ```typescript
 * import { MainThreadDispatcher, getDispatcher } from 'croupier/threading';
 *
 * // In initialization
 * getDispatcher().initialize();
 *
 * // From async callback or event handler
 * getDispatcher().enqueue(() => {
 *   // This runs during processQueue
 *   updateUI();
 * });
 *
 * // In game loop or requestAnimationFrame
 * function gameLoop() {
 *   getDispatcher().processQueue();
 *   // ... game logic
 *   requestAnimationFrame(gameLoop);
 * }
 * ```
 */

export type Callback = () => void;
export type CallbackWithData<T> = (data: T) => void;

export class MainThreadDispatcher {
  private static instance: MainThreadDispatcher | null = null;

  private queue: Callback[] = [];
  private initialized = false;
  private maxProcessPerFrame = 1000;

  private constructor() {
    // Private constructor for singleton
  }

  /**
   * Gets the singleton instance of MainThreadDispatcher.
   */
  public static getInstance(): MainThreadDispatcher {
    if (!MainThreadDispatcher.instance) {
      MainThreadDispatcher.instance = new MainThreadDispatcher();
    }
    return MainThreadDispatcher.instance;
  }

  /**
   * Resets the singleton instance. Primarily for testing.
   */
  public static resetInstance(): void {
    if (MainThreadDispatcher.instance) {
      MainThreadDispatcher.instance.clear();
      MainThreadDispatcher.instance.initialized = false;
    }
    MainThreadDispatcher.instance = null;
  }

  /**
   * Initialize the dispatcher. Should be called once at startup.
   */
  public initialize(): void {
    this.initialized = true;
  }

  /**
   * Check if the dispatcher has been initialized.
   */
  public isInitialized(): boolean {
    return this.initialized;
  }

  /**
   * Enqueue a callback to be executed during processQueue.
   * If initialized and called synchronously in the main event loop,
   * you can choose to execute immediately.
   *
   * @param callback - The callback to execute
   * @param executeImmediatelyIfInitialized - If true and initialized, execute immediately (default: true)
   */
  public enqueue(
    callback: Callback | null,
    executeImmediatelyIfInitialized = true,
  ): void {
    if (!callback) {
      return;
    }

    // In JavaScript's single-threaded model, if we're initialized
    // and want immediate execution, we can execute directly
    if (this.initialized && executeImmediatelyIfInitialized) {
      try {
        callback();
      } catch (error) {
        console.error(
          "[MainThreadDispatcher] Callback error (immediate):",
          error,
        );
      }
      return;
    }

    this.queue.push(callback);
  }

  /**
   * Enqueue a callback with data to be executed during processQueue.
   *
   * @param callback - The callback to execute
   * @param data - The data to pass to the callback
   */
  public enqueueWithData<T>(
    callback: CallbackWithData<T> | null,
    data: T,
  ): void {
    if (!callback) {
      return;
    }
    this.enqueue(() => callback(data), false);
  }

  /**
   * Enqueue a callback that will always be queued, never executed immediately.
   * Useful when you explicitly want to defer execution.
   *
   * @param callback - The callback to execute
   */
  public enqueueDeferred(callback: Callback | null): void {
    if (!callback) {
      return;
    }
    this.queue.push(callback);
  }

  /**
   * Process queued callbacks. Call this from your main loop.
   *
   * @returns The number of callbacks processed
   */
  public processQueue(): number {
    return this.processQueueWithLimit(this.maxProcessPerFrame);
  }

  /**
   * Process queued callbacks up to a maximum count.
   *
   * @param maxCount - Maximum number of callbacks to process
   * @returns The number of callbacks processed
   */
  public processQueueWithLimit(maxCount: number): number {
    if (maxCount <= 0) {
      maxCount = this.maxProcessPerFrame;
    }

    const count = Math.min(this.queue.length, maxCount);
    if (count === 0) {
      return 0;
    }

    // Extract callbacks to process
    const toProcess = this.queue.splice(0, count);

    // Process callbacks
    let processed = 0;
    for (const callback of toProcess) {
      try {
        callback();
      } catch (error) {
        console.error("[MainThreadDispatcher] Callback error:", error);
      }
      processed++;
    }

    return processed;
  }

  /**
   * Gets the number of pending callbacks in the queue.
   */
  public getPendingCount(): number {
    return this.queue.length;
  }

  /**
   * In JavaScript, we're always on the "main thread" (event loop).
   * This method is provided for API consistency with other SDK implementations.
   *
   * @returns true if initialized
   */
  public isMainThread(): boolean {
    return this.initialized;
  }

  /**
   * Sets the maximum number of callbacks to process per frame/tick.
   *
   * @param max - Maximum callbacks per frame. Use 0 for default (1000).
   */
  public setMaxProcessPerFrame(max: number): void {
    this.maxProcessPerFrame = max > 0 ? max : 1000;
  }

  /**
   * Clears all pending callbacks from the queue.
   */
  public clear(): void {
    this.queue.length = 0;
  }
}

/**
 * Gets the singleton dispatcher instance.
 */
export function getDispatcher(): MainThreadDispatcher {
  return MainThreadDispatcher.getInstance();
}

/**
 * Convenience function to enqueue a callback.
 */
export function enqueue(callback: Callback): void {
  getDispatcher().enqueue(callback);
}

/**
 * Convenience function to process the queue.
 */
export function processQueue(): number {
  return getDispatcher().processQueue();
}

export default MainThreadDispatcher;
