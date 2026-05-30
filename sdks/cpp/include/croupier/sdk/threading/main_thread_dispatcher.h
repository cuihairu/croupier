// Copyright 2025 Croupier Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#pragma once

#include <atomic>
#include <functional>
#include <mutex>
#include <queue>
#include <thread>
#include <climits>

namespace croupier {
namespace sdk {
namespace threading {

/**
 * @brief Main thread dispatcher - ensures callbacks execute on the main thread.
 *
 * Usage:
 * 1. Call Initialize() once on the main thread at startup
 * 2. Call ProcessQueue() in your main loop (e.g., Unreal's Tick method)
 * 3. Use Enqueue() from any thread to schedule callbacks
 *
 * For Unreal Engine:
 * - Use UCroupierSubsystem which automatically processes the queue, or
 * - Call MainThreadDispatcher::GetInstance().ProcessQueue() in your own Tick
 *
 * Example:
 * @code
 * // In BeginPlay or similar
 * MainThreadDispatcher::GetInstance().Initialize();
 *
 * // From any thread
 * MainThreadDispatcher::GetInstance().Enqueue([]() {
 *     // This runs on the main thread
 *     UE_LOG(LogTemp, Log, TEXT("Running on main thread!"));
 * });
 *
 * // In Tick
 * MainThreadDispatcher::GetInstance().ProcessQueue();
 * @endcode
 */
class MainThreadDispatcher {
public:
    using Callback = std::function<void()>;

    /**
     * @brief Get the singleton instance
     * @return Reference to the singleton instance
     */
    static MainThreadDispatcher& GetInstance() {
        static MainThreadDispatcher instance;
        return instance;
    }

    /**
     * @brief Initialize the dispatcher. Must be called once on the main thread.
     */
    void Initialize() {
        main_thread_id_ = std::this_thread::get_id();
        initialized_.store(true, std::memory_order_release);
    }

    /**
     * @brief Check if the dispatcher has been initialized
     * @return true if initialized
     */
    bool IsInitialized() const {
        return initialized_.load(std::memory_order_acquire);
    }

    /**
     * @brief Enqueue a callback to be executed on the main thread.
     * If called from the main thread and initialized, executes immediately.
     * @param callback The callback to execute
     */
    void Enqueue(Callback callback) {
        if (!callback) return;

        // If already on main thread and initialized, execute immediately
        if (IsInitialized() && IsMainThread()) {
            try {
                callback();
            } catch (...) {
                // Log but don't propagate
            }
            return;
        }

        std::lock_guard<std::mutex> lock(mutex_);
        queue_.push(std::move(callback));
    }

    /**
     * @brief Enqueue a callback with data to be executed on the main thread.
     * @tparam T Type of the data
     * @param callback The callback to execute
     * @param data The data to pass to the callback
     */
    template<typename T>
    void Enqueue(std::function<void(T)> callback, T data) {
        if (!callback) return;
        Enqueue([callback = std::move(callback), data = std::move(data)]() {
            callback(data);
        });
    }

    /**
     * @brief Process queued callbacks on the main thread.
     * Call this from your main loop.
     * @return Number of callbacks processed
     */
    int ProcessQueue() {
        return ProcessQueue(max_process_per_frame_.load(std::memory_order_relaxed));
    }

    /**
     * @brief Process queued callbacks, up to a maximum count.
     * @param max_count Maximum number of callbacks to process
     * @return Number of callbacks processed
     */
    int ProcessQueue(int max_count) {
        int processed = 0;

        while (processed < max_count) {
            Callback callback;
            {
                std::lock_guard<std::mutex> lock(mutex_);
                if (queue_.empty()) break;
                callback = std::move(queue_.front());
                queue_.pop();
            }

            try {
                if (callback) {
                    callback();
                }
            } catch (...) {
                // Log but don't interrupt processing
            }
            processed++;
        }

        return processed;
    }

    /**
     * @brief Get the number of pending callbacks in the queue.
     * @return Number of pending callbacks
     */
    size_t GetPendingCount() const {
        std::lock_guard<std::mutex> lock(mutex_);
        return queue_.size();
    }

    /**
     * @brief Check if the current thread is the main thread.
     * @return true if on main thread
     */
    bool IsMainThread() const {
        return IsInitialized() && std::this_thread::get_id() == main_thread_id_;
    }

    /**
     * @brief Set the maximum number of callbacks to process per frame.
     * @param max Maximum callbacks per frame. Use INT_MAX for unlimited.
     */
    void SetMaxProcessPerFrame(int max) {
        max_process_per_frame_.store(max > 0 ? max : INT_MAX, std::memory_order_relaxed);
    }

    /**
     * @brief Clear all pending callbacks from the queue.
     */
    void Clear() {
        std::lock_guard<std::mutex> lock(mutex_);
        std::queue<Callback> empty;
        std::swap(queue_, empty);
    }

    /**
     * @brief Reset the dispatcher state. Primarily for testing.
     */
    void Reset() {
        Clear();
        initialized_.store(false, std::memory_order_release);
        main_thread_id_ = std::thread::id();
    }

private:
    MainThreadDispatcher() = default;
    ~MainThreadDispatcher() = default;

    // Disable copy and move
    MainThreadDispatcher(const MainThreadDispatcher&) = delete;
    MainThreadDispatcher& operator=(const MainThreadDispatcher&) = delete;
    MainThreadDispatcher(MainThreadDispatcher&&) = delete;
    MainThreadDispatcher& operator=(MainThreadDispatcher&&) = delete;

    mutable std::mutex mutex_;
    std::queue<Callback> queue_;
    std::thread::id main_thread_id_;
    std::atomic<bool> initialized_{false};
    std::atomic<int> max_process_per_frame_{INT_MAX};
};

}  // namespace threading
}  // namespace sdk
}  // namespace croupier
