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

namespace Croupier.Sdk.Configuration;

/// <summary>
/// 客户端配置提供者接口
/// </summary>
public interface ICroupierConfigProvider
{
    /// <summary>
    /// 获取客户端配置
    /// </summary>
    ClientConfig GetConfig();
}

/// <summary>
/// 环境变量配置提供者
/// </summary>
public class EnvironmentConfigProvider : ICroupierConfigProvider
{
    private readonly string? _prefix;

    /// <summary>
    /// 创建环境变量配置提供者
    /// </summary>
    /// <param name="prefix">环境变量前缀（默认: CROUPIER_）</param>
    public EnvironmentConfigProvider(string? prefix = null)
    {
        _prefix = prefix ?? "CROUPIER_";
    }

    /// <summary>
    /// 获取配置
    /// </summary>
    public ClientConfig GetConfig()
    {
        return new ClientConfig
        {
            AgentAddr = GetEnv("AGENT_ADDR", "127.0.0.1:19090") ?? "127.0.0.1:19090",
            ServiceId = GetEnv("SERVICE_ID", "csharp-service") ?? "csharp-service",
            AgentId = GetEnv("AGENT_ID"),
            ServiceVersion = GetEnv("SERVICE_VERSION", "1.0.0") ?? "1.0.0",
            GameId = GetEnv("GAME_ID", "default-game") ?? "default-game",
            Env = GetEnv("ENV", "development") ?? "development",
            ControlAddr = GetEnv("CONTROL_ADDR"),
            Insecure = GetEnvBool("INSECURE", true),
            CertFile = GetEnv("CERT_FILE"),
            KeyFile = GetEnv("KEY_FILE"),
            CaFile = GetEnv("CA_FILE"),
            ServerName = GetEnv("SERVER_NAME"),
            ProviderLang = GetEnv("PROVIDER_LANG", "csharp") ?? "csharp",
            ProviderSdk = GetEnv("PROVIDER_SDK", "croupier-csharp-sdk") ?? "croupier-csharp-sdk",
            AuthToken = GetEnv("AUTH_TOKEN"),
            TimeoutSeconds = GetEnvInt("TIMEOUT_SECONDS", 30),
            HeartbeatIntervalSeconds = GetEnvInt("HEARTBEAT_INTERVAL_SECONDS", 60),
            AutoReconnect = GetEnvBool("AUTO_RECONNECT", true),
            ReconnectIntervalSeconds = GetEnvInt("RECONNECT_INTERVAL_SECONDS", 5),
            ReconnectMaxAttempts = GetEnvInt("RECONNECT_MAX_ATTEMPTS", 0),
            MaxConcurrentMessages = GetEnvInt("MAX_CONCURRENT_MESSAGES", 100),
            MaxMessageSize = GetEnvInt("MAX_MESSAGE_SIZE", 4 * 1024 * 1024),
            DisableLogging = GetEnvBool("DISABLE_LOGGING", false),
            DebugLogging = GetEnvBool("DEBUG_LOGGING", false),
            LogLevel = GetEnv("LOG_LEVEL", "INFO") ?? "INFO",
            EnableFileTransfer = GetEnvBool("ENABLE_FILE_TRANSFER", false),
            MaxFileSize = GetEnvInt("MAX_FILE_SIZE", 10 * 1024 * 1024)
        };
    }

    private string? GetEnv(string name, string? defaultValue = null)
    {
        var value = Environment.GetEnvironmentVariable(_prefix + name);
        return value ?? defaultValue;
    }

    private bool GetEnvBool(string name, bool defaultValue)
    {
        var value = GetEnv(name);
        if (string.IsNullOrEmpty(value))
            return defaultValue;

        return value.Equals("true", StringComparison.OrdinalIgnoreCase) ||
               value.Equals("1", StringComparison.OrdinalIgnoreCase);
    }

    private int GetEnvInt(string name, int defaultValue)
    {
        var value = GetEnv(name);
        return int.TryParse(value, out var result) ? result : defaultValue;
    }
}

/// <summary>
/// JSON 文件配置提供者
/// </summary>
public class JsonFileConfigProvider : ICroupierConfigProvider
{
    private readonly string _filePath;

    /// <summary>
    /// 创建 JSON 文件配置提供者
    /// </summary>
    /// <param name="filePath">JSON 配置文件路径</param>
    public JsonFileConfigProvider(string filePath)
    {
        _filePath = filePath ?? throw new ArgumentNullException(nameof(filePath));
    }

    /// <summary>
    /// 获取配置
    /// </summary>
    public ClientConfig GetConfig()
    {
        if (!File.Exists(_filePath))
            throw new FileNotFoundException($"Configuration file not found: {_filePath}");

        var json = File.ReadAllText(_filePath);

        // 简单 JSON 解析（生产环境应使用 System.Text.Json 或 Newtonsoft.Json）
        var config = new ClientConfig();

        // 解析 JSON 字符串
        var properties = typeof(ClientConfig).GetProperties();
        foreach (var prop in properties)
        {
            var pattern = $"\"{prop.Name}\"\\s*:\\s*\"?([^,}}\n\"]+)\"?";
            var match = System.Text.RegularExpressions.Regex.Match(json, pattern);
            if (match.Success)
            {
                var value = match.Groups[1].Value.Trim('"');
                prop.SetValue(config, ConvertValue(value, prop.PropertyType));
            }
        }

        return config;
    }

    private static object? ConvertValue(string value, Type targetType)
    {
        if (targetType == typeof(string))
            return value;

        if (targetType == typeof(int))
            return int.TryParse(value, out var i) ? i : 0;

        if (targetType == typeof(bool))
            return bool.TryParse(value, out var b) && b;

        return value;
    }
}

/// <summary>
/// 内存配置提供者
/// </summary>
public class MemoryConfigProvider : ICroupierConfigProvider
{
    private readonly ClientConfig _config;

    /// <summary>
    /// 创建内存配置提供者
    /// </summary>
    /// <param name="config">配置对象</param>
    public MemoryConfigProvider(ClientConfig config)
    {
        _config = config ?? throw new ArgumentNullException(nameof(config));
    }

    /// <summary>
    /// 获取配置
    /// </summary>
    public ClientConfig GetConfig() => _config;
}
