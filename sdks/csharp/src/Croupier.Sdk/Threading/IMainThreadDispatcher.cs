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

using System;

namespace Croupier.Sdk.Threading;

/// <summary>
/// Interface for main thread dispatcher - ensures callbacks execute on the main thread.
/// </summary>
public interface IMainThreadDispatcher
{
    /// <summary>
    /// Enqueue an action to be executed on the main thread.
    /// If called from the main thread, the action is executed immediately.
    /// </summary>
    /// <param name="action">The action to execute.</param>
    void Enqueue(Action action);

    /// <summary>
    /// Enqueue an action with data to be executed on the main thread.
    /// </summary>
    /// <typeparam name="T">The type of data.</typeparam>
    /// <param name="action">The action to execute.</param>
    /// <param name="data">The data to pass to the action.</param>
    void Enqueue<T>(Action<T> action, T data);

    /// <summary>
    /// Process queued callbacks on the main thread.
    /// Call this from your main loop (e.g., Unity's Update method).
    /// </summary>
    /// <returns>The number of callbacks processed.</returns>
    int ProcessQueue();

    /// <summary>
    /// Process queued callbacks on the main thread, up to a maximum count.
    /// </summary>
    /// <param name="maxCount">Maximum number of callbacks to process.</param>
    /// <returns>The number of callbacks processed.</returns>
    int ProcessQueue(int maxCount);

    /// <summary>
    /// Gets the number of pending callbacks in the queue.
    /// </summary>
    int PendingCount { get; }

    /// <summary>
    /// Gets whether the current thread is the main thread.
    /// </summary>
    bool IsMainThread { get; }

    /// <summary>
    /// Sets the maximum number of callbacks to process per frame.
    /// </summary>
    /// <param name="max">Maximum callbacks per frame. Use int.MaxValue for unlimited.</param>
    void SetMaxProcessPerFrame(int max);

    /// <summary>
    /// Clears all pending callbacks from the queue.
    /// </summary>
    void Clear();
}
