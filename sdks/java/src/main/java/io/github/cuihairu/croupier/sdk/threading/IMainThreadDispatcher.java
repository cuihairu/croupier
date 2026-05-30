/*
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

package io.github.cuihairu.croupier.sdk.threading;

/**
 * Interface for main thread dispatcher - ensures callbacks execute on the main thread.
 */
public interface IMainThreadDispatcher {

    /**
     * Enqueue an action to be executed on the main thread.
     * If called from the main thread, the action may be executed immediately.
     *
     * @param callback The callback to execute
     */
    void enqueue(Runnable callback);

    /**
     * Enqueue an action with data to be executed on the main thread.
     *
     * @param <T> The type of data
     * @param callback The callback to execute
     * @param data The data to pass to the callback
     */
    <T> void enqueue(java.util.function.Consumer<T> callback, T data);

    /**
     * Process queued callbacks on the main thread.
     * Call this from your main loop.
     *
     * @return The number of callbacks processed
     */
    int processQueue();

    /**
     * Process queued callbacks on the main thread, up to a maximum count.
     *
     * @param maxCount Maximum number of callbacks to process
     * @return The number of callbacks processed
     */
    int processQueue(int maxCount);

    /**
     * Gets the number of pending callbacks in the queue.
     *
     * @return Number of pending callbacks
     */
    int getPendingCount();

    /**
     * Gets whether the current thread is the main thread.
     *
     * @return true if on main thread
     */
    boolean isMainThread();

    /**
     * Sets the maximum number of callbacks to process per frame.
     *
     * @param max Maximum callbacks per frame. Use Integer.MAX_VALUE for unlimited.
     */
    void setMaxProcessPerFrame(int max);

    /**
     * Clears all pending callbacks from the queue.
     */
    void clear();
}
