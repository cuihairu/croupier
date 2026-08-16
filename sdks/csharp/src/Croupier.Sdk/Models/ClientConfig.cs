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

namespace Croupier.Sdk.Models;

/// <summary>
/// 客户端配置
/// </summary>
public class ClientConfig
{
    /// <summary>
    /// Agent 本地 SDK gateway 地址
    /// </summary>
    public string AgentAddr { get; set; } = "127.0.0.1:19091";

    /// <summary>
    /// 服务标识符
    /// </summary>
    public string ServiceId { get; set; } = "csharp-service";

    /// <summary>
    /// Agent 唯一标识符（为空时由服务端或上层生成）
    /// </summary>
    public string? AgentId { get; set; }

    /// <summary>
    /// 服务版本
    /// </summary>
    public string ServiceVersion { get; set; } = "1.0.0";

    /// <summary>
    /// 游戏 ID
    /// </summary>
    public string GameId { get; set; } = "default-game";

    /// <summary>
    /// 环境
    /// </summary>
    public string Env { get; set; } = "development";

    /// <summary>
    /// 可选控制面地址
    /// </summary>
    public string? ControlAddr { get; set; }

    /// <summary>
    /// 是否使用不安全连接（跳过 TLS 验证）
    /// </summary>
    public bool Insecure { get; set; } = true;

    /// <summary>
    /// TLS 证书文件路径
    /// </summary>
    public string? CertFile { get; set; }

    /// <summary>
    /// TLS 私钥文件路径
    /// </summary>
    public string? KeyFile { get; set; }

    /// <summary>
    /// TLS CA 证书文件路径
    /// </summary>
    public string? CaFile { get; set; }

    /// <summary>
    /// TLS 服务器名称（用于 SNI）
    /// </summary>
    public string? ServerName { get; set; }

    /// <summary>
    /// Provider 语言标识
    /// </summary>
    public string ProviderLang { get; set; } = "csharp";

    /// <summary>
    /// Provider SDK 标识
    /// </summary>
    public string ProviderSdk { get; set; } = "croupier-csharp-sdk";

    /// <summary>
    /// 认证 Bearer Token
    /// </summary>
    public string? AuthToken { get; set; }

    /// <summary>
    /// 附加请求头
    /// </summary>
    public Dictionary<string, string> Headers { get; set; } = new();

    /// <summary>
    /// 连接超时（秒）
    /// </summary>
    public int TimeoutSeconds { get; set; } = 30;

    /// <summary>
    /// 连接尝试超时（秒）- 用于初始连接，默认 5 秒
    /// </summary>
    public int ConnectTimeoutSeconds { get; set; } = 5;

    /// <summary>
    /// 心跳间隔（秒）
    /// </summary>
    public int HeartbeatIntervalSeconds { get; set; } = 60;

    /// <summary>
    /// 是否自动重连
    /// </summary>
    public bool AutoReconnect { get; set; } = true;

    /// <summary>
    /// 重连间隔（秒）
    /// </summary>
    public int ReconnectIntervalSeconds { get; set; } = 5;

    /// <summary>
    /// 重连最大尝试次数（0 表示无限重试）
    /// </summary>
    public int ReconnectMaxAttempts { get; set; } = 0;

    /// <summary>
    /// 最大并发消息数
    /// </summary>
    public int MaxConcurrentMessages { get; set; } = 100;

    /// <summary>
    /// 消息最大大小（字节）
    /// </summary>
    public int MaxMessageSize { get; set; } = 4 * 1024 * 1024; // 4MB

    /// <summary>
    /// 禁用所有日志输出
    /// </summary>
    public bool DisableLogging { get; set; }

    /// <summary>
    /// 启用调试日志
    /// </summary>
    public bool DebugLogging { get; set; }

    /// <summary>
    /// 日志级别
    /// </summary>
    public string LogLevel { get; set; } = "INFO";

    /// <summary>
    /// 启用文件传输能力
    /// </summary>
    public bool EnableFileTransfer { get; set; }

    /// <summary>
    /// 最大文件大小（字节）
    /// </summary>
    public int MaxFileSize { get; set; } = 10 * 1024 * 1024;
}
