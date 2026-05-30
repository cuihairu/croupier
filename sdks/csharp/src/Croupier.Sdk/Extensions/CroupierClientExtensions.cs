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
using System.Threading;
using System.Threading.Tasks;
using Croupier.Sdk.Models;
using Croupier.Sdk.Threading;

namespace Croupier.Sdk.Extensions;

/// <summary>
/// Extension methods for CroupierClient that integrate with MainThreadDispatcher.
/// </summary>
public static class CroupierClientExtensions
{
    /// <summary>
    /// Invoke a remote function asynchronously, with callbacks executed on the main thread.
    /// </summary>
    /// <param name="client">The Croupier client.</param>
    /// <param name="functionId">The function ID to invoke.</param>
    /// <param name="payload">The request payload (JSON).</param>
    /// <param name="onSuccess">Callback invoked on the main thread with the result.</param>
    /// <param name="onError">Optional callback invoked on the main thread on error.</param>
    /// <param name="options">Optional invoke options.</param>
    /// <param name="cancellationToken">Optional cancellation token.</param>
    public static void InvokeOnMainThread(
        this CroupierClient client,
        string functionId,
        string payload,
        Action<string> onSuccess,
        Action<Exception>? onError = null,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        _ = InvokeOnMainThreadAsync(client, functionId, payload, onSuccess, onError, options, cancellationToken);
    }

    /// <summary>
    /// Invoke a remote function asynchronously, with callbacks executed on the main thread.
    /// Returns a Task that completes when the callback has been enqueued.
    /// </summary>
    public static async Task InvokeOnMainThreadAsync(
        this CroupierClient client,
        string functionId,
        string payload,
        Action<string> onSuccess,
        Action<Exception>? onError = null,
        InvokeOptions? options = null,
        CancellationToken cancellationToken = default)
    {
        try
        {
            var result = await client.InvokeAsync(functionId, payload, options, cancellationToken);
            MainThreadDispatcher.Instance.Enqueue(() => onSuccess(result));
        }
        catch (Exception ex)
        {
            if (onError != null)
            {
                MainThreadDispatcher.Instance.Enqueue(() => onError(ex));
            }
        }
    }

    /// <summary>
    /// Connect to the agent asynchronously, with callback executed on the main thread.
    /// </summary>
    /// <param name="client">The Croupier client.</param>
    /// <param name="onSuccess">Callback invoked on the main thread on successful connection.</param>
    /// <param name="onError">Optional callback invoked on the main thread on error.</param>
    /// <param name="cancellationToken">Optional cancellation token.</param>
    public static void ConnectOnMainThread(
        this CroupierClient client,
        Action onSuccess,
        Action<Exception>? onError = null,
        CancellationToken cancellationToken = default)
    {
        _ = ConnectOnMainThreadAsync(client, onSuccess, onError, cancellationToken);
    }

    /// <summary>
    /// Connect to the agent asynchronously, with callback executed on the main thread.
    /// Returns a Task that completes when the callback has been enqueued.
    /// </summary>
    public static async Task ConnectOnMainThreadAsync(
        this CroupierClient client,
        Action onSuccess,
        Action<Exception>? onError = null,
        CancellationToken cancellationToken = default)
    {
        try
        {
            await client.ConnectAsync(cancellationToken);
            MainThreadDispatcher.Instance.Enqueue(onSuccess);
        }
        catch (Exception ex)
        {
            if (onError != null)
            {
                MainThreadDispatcher.Instance.Enqueue(() => onError(ex));
            }
        }
    }

    /// <summary>
    /// Execute an action on the main thread.
    /// If already on the main thread, executes immediately.
    /// </summary>
    /// <param name="client">The Croupier client (used for method chaining).</param>
    /// <param name="action">The action to execute.</param>
    /// <returns>The client for method chaining.</returns>
    public static CroupierClient RunOnMainThread(this CroupierClient client, Action action)
    {
        MainThreadDispatcher.Instance.Enqueue(action);
        return client;
    }

    /// <summary>
    /// Execute an action with data on the main thread.
    /// </summary>
    /// <typeparam name="T">The type of data.</typeparam>
    /// <param name="client">The Croupier client (used for method chaining).</param>
    /// <param name="action">The action to execute.</param>
    /// <param name="data">The data to pass to the action.</param>
    /// <returns>The client for method chaining.</returns>
    public static CroupierClient RunOnMainThread<T>(this CroupierClient client, Action<T> action, T data)
    {
        MainThreadDispatcher.Instance.Enqueue(action, data);
        return client;
    }
}
