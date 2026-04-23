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
using System.Collections.Concurrent;
using System.Threading;

namespace Croupier.Sdk.Threading;

/// <summary>
/// Main thread dispatcher - ensures callbacks execute on the main thread.
///
/// Usage:
/// 1. Call Initialize() once on the main thread at startup
/// 2. Call ProcessQueue() in your main loop (e.g., Unity's Update method)
/// 3. Use Enqueue() from any thread to schedule callbacks
///
/// For Unity:
/// - Add CroupierUnityBehaviour to a GameObject, or
/// - Call MainThreadDispatcher.Instance.ProcessQueue() in your own Update method
/// </summary>
public sealed class MainThreadDispatcher : IMainThreadDispatcher
{
    private static MainThreadDispatcher? _instance;
    private static readonly object _instanceLock = new();

    private readonly ConcurrentQueue<Action> _queue = new();
    private int _mainThreadId = -1;
    private int _maxProcessPerFrame = int.MaxValue;
    private volatile bool _initialized;

    private MainThreadDispatcher()
    {
    }

    /// <summary>
    /// Gets the singleton instance of the dispatcher.
    /// The instance is created on first access, but Initialize() must be called
    /// on the main thread before using Enqueue() from other threads.
    /// </summary>
    public static MainThreadDispatcher Instance
    {
        get
        {
            if (_instance == null)
            {
                lock (_instanceLock)
                {
                    _instance ??= new MainThreadDispatcher();
                }
            }
            return _instance;
        }
    }

    /// <summary>
    /// Initialize the dispatcher. Must be called once on the main thread.
    /// </summary>
    public static void Initialize()
    {
        var instance = Instance;
        instance._mainThreadId = Thread.CurrentThread.ManagedThreadId;
        instance._initialized = true;
    }

    /// <summary>
    /// Gets whether the dispatcher has been initialized.
    /// </summary>
    public bool IsInitialized => _initialized;

    /// <inheritdoc/>
    public void Enqueue(Action action)
    {
        if (action == null) return;

        // If already on main thread and initialized, execute immediately
        if (_initialized && IsMainThread)
        {
            try
            {
                action();
            }
            catch (Exception ex)
            {
                LogError($"Callback error (immediate): {ex.Message}");
            }
            return;
        }

        _queue.Enqueue(action);
    }

    /// <inheritdoc/>
    public void Enqueue<T>(Action<T> action, T data)
    {
        if (action == null) return;
        Enqueue(() => action(data));
    }

    /// <inheritdoc/>
    public int ProcessQueue()
    {
        return ProcessQueue(_maxProcessPerFrame);
    }

    /// <inheritdoc/>
    public int ProcessQueue(int maxCount)
    {
        int processed = 0;

        while (processed < maxCount && _queue.TryDequeue(out var action))
        {
            try
            {
                action?.Invoke();
            }
            catch (Exception ex)
            {
                // Log but don't interrupt processing
                LogError($"Callback error: {ex.Message}");
            }
            processed++;
        }

        return processed;
    }

    /// <inheritdoc/>
    public int PendingCount => _queue.Count;

    /// <inheritdoc/>
    public bool IsMainThread => _initialized && Thread.CurrentThread.ManagedThreadId == _mainThreadId;

    /// <inheritdoc/>
    public void SetMaxProcessPerFrame(int max)
    {
        _maxProcessPerFrame = max > 0 ? max : int.MaxValue;
    }

    /// <inheritdoc/>
    public void Clear()
    {
        while (_queue.TryDequeue(out _)) { }
    }

    /// <summary>
    /// Resets the singleton instance. Primarily for testing purposes.
    /// </summary>
    internal static void Reset()
    {
        lock (_instanceLock)
        {
            _instance?.Clear();
            _instance = null;
        }
    }

    private static void LogError(string message)
    {
        // Use Console.Error for non-Unity environments
        // Unity will override this with Debug.LogError via conditional compilation
#if UNITY_5_3_OR_NEWER || UNITY_2017_1_OR_NEWER
        UnityEngine.Debug.LogError($"[MainThreadDispatcher] {message}");
#else
        Console.Error.WriteLine($"[MainThreadDispatcher] {message}");
#endif
    }
}
