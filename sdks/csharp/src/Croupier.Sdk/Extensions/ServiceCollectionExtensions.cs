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

using Croupier.Sdk.Configuration;
using Croupier.Sdk.Models;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;

namespace Croupier.Sdk.Extensions;

/// <summary>
/// 服务集合扩展
/// </summary>
public static class ServiceCollectionExtensions
{
    /// <summary>
    /// 添加 Croupier SDK 到服务集合
    /// </summary>
    /// <param name="services">服务集合</param>
    /// <param name="configAction">配置操作</param>
    /// <returns>服务集合（链式调用）</returns>
    public static IServiceCollection AddCroupier(
        this IServiceCollection services,
        Action<ClientConfig>? configAction = null)
    {
        // 配置选项
        var config = new ClientConfig();
        configAction?.Invoke(config);

        services.AddSingleton(Options.Create(config));

        // 注册核心服务
        services.AddSingleton<CroupierClient>();
        services.AddSingleton<CroupierInvoker>();

        return services;
    }

    /// <summary>
    /// 添加 Croupier SDK 到服务集合（使用配置节）
    /// </summary>
    /// <param name="services">服务集合</param>
    /// <param name="section">配置节</param>
    /// <returns>服务集合（链式调用）</returns>
    public static IServiceCollection AddCroupier(
        this IServiceCollection services,
        IConfigurationSection section)
    {
        services.Configure<ClientConfig>(section);
        services.AddSingleton<CroupierClient>();
        services.AddSingleton<CroupierInvoker>();

        return services;
    }

    /// <summary>
    /// 添加带配置提供者的 Croupier SDK
    /// </summary>
    /// <param name="services">服务集合</param>
    /// <param name="configProvider">配置提供者</param>
    /// <returns>服务集合（链式调用）</returns>
    public static IServiceCollection AddCroupier(
        this IServiceCollection services,
        ICroupierConfigProvider configProvider)
    {
        var config = configProvider.GetConfig();
        services.AddSingleton(Options.Create(config));
        services.AddSingleton<CroupierClient>();
        services.AddSingleton<CroupierInvoker>();

        return services;
    }
}
