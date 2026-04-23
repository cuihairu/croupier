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

import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicLong;
import java.util.function.Consumer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Main thread dispatcher - ensures callbacks execute on the main thread.
 *
 * <p>Usage:
 * <ol>
 *   <li>Call {@link #initialize()} once on the main thread at startup</li>
 *   <li>Call {@link #processQueue()} in your main loop</li>
 *   <li>Use {@link #enqueue(Runnable)} from any thread to schedule callbacks</li>
 * </ol>
 *
 * <p>For Android:
 * <ul>
 *   <li>Use with Handler(Looper.getMainLooper()) for automatic processing, or</li>
 *   <li>Call processQueue() in your Activity's onResume/game loop</li>
 * </ul>
 *
 * <p>Example:
 * <pre>{@code
 * // In onCreate or similar
 * MainThreadDispatcher.getInstance().initialize();
 *
 * // From any thread
 * MainThreadDispatcher.getInstance().enqueue(() -> {
 *     // This runs on the main thread
 *     updateUI();
 * });
 *
 * // In your game loop or periodic handler
 * MainThreadDispatcher.getInstance().processQueue();
 * }</pre>
 */
public final class MainThreadDispatcher implements IMainThreadDispatcher {

    private static final Logger logger = LoggerFactory.getLogger(MainThreadDispatcher.class);

    private static volatile MainThreadDispatcher instance;
    private static final Object INSTANCE_LOCK = new Object();

    private final ConcurrentLinkedQueue<Runnable> queue = new ConcurrentLinkedQueue<>();
    private final AtomicLong mainThreadId = new AtomicLong(-1);
    private final AtomicInteger maxProcessPerFrame = new AtomicInteger(Integer.MAX_VALUE);
    private volatile boolean initialized = false;

    private MainThreadDispatcher() {
    }

    /**
     * Gets the singleton instance of the dispatcher.
     *
     * @return The singleton instance
     */
    public static MainThreadDispatcher getInstance() {
        if (instance == null) {
            synchronized (INSTANCE_LOCK) {
                if (instance == null) {
                    instance = new MainThreadDispatcher();
                }
            }
        }
        return instance;
    }

    /**
     * Initialize the dispatcher. Must be called once on the main thread.
     */
    public void initialize() {
        mainThreadId.set(Thread.currentThread().getId());
        initialized = true;
        logger.info("MainThreadDispatcher initialized on thread {}", mainThreadId.get());
    }

    /**
     * Check if the dispatcher has been initialized.
     *
     * @return true if initialized
     */
    public boolean isInitialized() {
        return initialized;
    }

    @Override
    public void enqueue(Runnable callback) {
        if (callback == null) {
            return;
        }

        // If already on main thread and initialized, execute immediately
        if (initialized && isMainThread()) {
            try {
                callback.run();
            } catch (Exception e) {
                logger.error("Callback error (immediate): {}", e.getMessage(), e);
            }
            return;
        }

        queue.offer(callback);
    }

    @Override
    public <T> void enqueue(Consumer<T> callback, T data) {
        if (callback == null) {
            return;
        }
        enqueue(() -> callback.accept(data));
    }

    @Override
    public int processQueue() {
        return processQueue(maxProcessPerFrame.get());
    }

    @Override
    public int processQueue(int maxCount) {
        int processed = 0;

        while (processed < maxCount) {
            Runnable callback = queue.poll();
            if (callback == null) {
                break;
            }

            try {
                callback.run();
            } catch (Exception e) {
                // Log but don't interrupt processing
                logger.error("Callback error: {}", e.getMessage(), e);
            }
            processed++;
        }

        return processed;
    }

    @Override
    public int getPendingCount() {
        return queue.size();
    }

    @Override
    public boolean isMainThread() {
        long mainId = mainThreadId.get();
        return mainId >= 0 && Thread.currentThread().getId() == mainId;
    }

    @Override
    public void setMaxProcessPerFrame(int max) {
        maxProcessPerFrame.set(max > 0 ? max : Integer.MAX_VALUE);
    }

    @Override
    public void clear() {
        queue.clear();
    }

    /**
     * Reset the dispatcher state. Primarily for testing purposes.
     */
    void reset() {
        clear();
        initialized = false;
        mainThreadId.set(-1);
    }

    /**
     * Reset the singleton instance. Primarily for testing purposes.
     */
    static void resetInstance() {
        synchronized (INSTANCE_LOCK) {
            if (instance != null) {
                instance.reset();
            }
            instance = null;
        }
    }
}
