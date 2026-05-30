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

using Croupier.Sdk.Models;

namespace Croupier.Sdk;

/// <summary>
/// 函数处理器委托
/// </summary>
/// <param name="context">函数调用上下文</param>
/// <param name="payload">请求负载（JSON 字符串）</param>
/// <returns>响应负载（JSON 字符串）</returns>
public delegate Task<string> FunctionHandlerDelegate(
    FunctionContext context,
    string payload);

/// <summary>
/// 同步函数处理器委托
/// </summary>
/// <param name="context">函数调用上下文</param>
/// <param name="payload">请求负载（JSON 字符串）</param>
/// <returns>响应负载（JSON 字符串）</returns>
public delegate string SyncFunctionHandlerDelegate(
    FunctionContext context,
    string payload);

/// <summary>
/// 函数处理器接口
/// </summary>
public interface IFunctionHandler
{
    /// <summary>
    /// 处理函数调用
    /// </summary>
    /// <param name="context">函数调用上下文</param>
    /// <param name="payload">请求负载（JSON 字符串）</param>
    /// <returns>响应负载（JSON 字符串）</returns>
    Task<string> HandleAsync(FunctionContext context, string payload);
}

/// <summary>
/// 函数处理器基类
/// </summary>
public abstract class FunctionHandlerBase : IFunctionHandler
{
    /// <summary>
    /// 异步处理函数调用
    /// </summary>
    public abstract Task<string> HandleAsync(FunctionContext context, string payload);

    /// <summary>
    /// 同步处理函数调用（默认实现）
    /// </summary>
    public virtual string Handle(FunctionContext context, string payload)
    {
        // 默认同步调用异步方法并等待
        return HandleAsync(context, payload).GetAwaiter().GetResult();
    }
}

/// <summary>
/// 委托函数处理器适配器
/// </summary>
public class DelegateFunctionHandler : IFunctionHandler
{
    private readonly FunctionHandlerDelegate _handler;

    public DelegateFunctionHandler(FunctionHandlerDelegate handler)
    {
        _handler = handler ?? throw new ArgumentNullException(nameof(handler));
    }

    public Task<string> HandleAsync(FunctionContext context, string payload)
    {
        return _handler(context, payload);
    }
}

/// <summary>
/// 同步委托函数处理器适配器
/// </summary>
public class SyncDelegateFunctionHandler : IFunctionHandler
{
    private readonly SyncFunctionHandlerDelegate _handler;

    public SyncDelegateFunctionHandler(SyncFunctionHandlerDelegate handler)
    {
        _handler = handler ?? throw new ArgumentNullException(nameof(handler));
    }

    public Task<string> HandleAsync(FunctionContext context, string payload)
    {
        try
        {
            var result = _handler(context, payload);
            return Task.FromResult(result);
        }
        catch (Exception ex)
        {
            return Task.FromException<string>(ex);
        }
    }
}
