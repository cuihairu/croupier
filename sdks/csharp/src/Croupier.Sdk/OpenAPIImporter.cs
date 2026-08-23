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

using System.IO;
using System.Text;
using System.Text.Json;
using Croupier.Sdk.Models;

namespace Croupier.Sdk;

/// <summary>
/// OpenAPI 3 导入配置（与 Go SDK 的 ImportOptions 对齐）。
/// </summary>
public sealed class OpenAPIImportOptions
{
    /// <summary>为每个导入的 resource 添加前缀（如 "game"）。</summary>
    public string ResourcePrefix { get; set; } = string.Empty;

    /// <summary>为每个导入的 tag 添加前缀。</summary>
    public string TagPrefix { get; set; } = string.Empty;

    /// <summary>单个函数失败时是否继续导入其余函数。</summary>
    public bool ContinueOnError { get; set; }
}

/// <summary>
/// OpenAPI 3 导入助手，等价于 Go SDK 的 RegisterFromOpenAPI：
/// 解析 OpenAPI 3 文档，将每个 operation 转换为 FunctionDescriptor 并连同
/// 调用方提供的 handler 注册到 <see cref="CroupierClient"/>。
/// </summary>
public static class OpenAPIImporter
{
    private static readonly string[] OperationMethods =
    {
        "get", "put", "post", "delete", "options", "head", "patch", "trace",
    };

    /// <summary>
    /// 导入 OpenAPI 3 文档，将每个 operation 注册到 <paramref name="client"/>。
    /// </summary>
    /// <param name="client">目标 SDK 客户端。</param>
    /// <param name="specJson">OpenAPI 3 JSON 文档。</param>
    /// <param name="options">导入配置；可为 null。</param>
    /// <param name="handlerResolver">为派生函数 ID 提供 handler；返回 null 表示未提供。</param>
    /// <returns>已注册的函数 ID 列表。</returns>
    /// <exception cref="ArgumentException">spec 非法。</exception>
    /// <exception cref="InvalidOperationException">缺少 handler（未启用 ContinueOnError 时）。</exception>
    public static IReadOnlyList<string> RegisterFromOpenAPI(
        CroupierClient client,
        string specJson,
        OpenAPIImportOptions? options,
        Func<string, FunctionHandlerDelegate?> handlerResolver)
    {
        ArgumentNullException.ThrowIfNull(client);
        ArgumentNullException.ThrowIfNull(specJson);
        ArgumentNullException.ThrowIfNull(handlerResolver);

        JsonElement root;
        try
        {
            using var document = JsonDocument.Parse(specJson);
            root = document.RootElement.Clone();
        }
        catch (JsonException exception)
        {
            throw new ArgumentException($"load OpenAPI spec failed: {exception.Message}", exception);
        }
        if (root.ValueKind != JsonValueKind.Object
            || !root.TryGetProperty("paths", out var paths)
            || paths.ValueKind != JsonValueKind.Object)
        {
            throw new ArgumentException("OpenAPI spec must be an object containing 'paths'");
        }

        var registered = new List<string>();
        foreach (var pathEntry in paths.EnumerateObject())
        {
            if (pathEntry.Value.ValueKind != JsonValueKind.Object)
            {
                continue;
            }
            foreach (var method in OperationMethods)
            {
                if (!pathEntry.Value.TryGetProperty(method, out var operation)
                    || operation.ValueKind != JsonValueKind.Object)
                {
                    continue;
                }

                var descriptor = OperationToDescriptor(pathEntry.Name, operation, options);
                var handler = handlerResolver(descriptor.Id);
                if (handler is null)
                {
                    if (options is { ContinueOnError: true })
                    {
                        continue;
                    }
                    throw new InvalidOperationException($"no handler provided for function: {descriptor.Id}");
                }
                try
                {
                    client.RegisterFunction(descriptor, handler);
                }
                catch (Exception exception)
                {
                    if (options is { ContinueOnError: true })
                    {
                        continue;
                    }
                    throw new InvalidOperationException(
                        $"register function {descriptor.Id} failed: {exception.Message}", exception);
                }
                registered.Add(descriptor.Id);
            }
        }
        return registered;
    }

    /// <summary>
    /// 使用显式 handler 映射导入 OpenAPI 3 文档（等价 Go 的
    /// RegisterFromOpenAPIWithHandlers）。
    /// </summary>
    public static IReadOnlyList<string> RegisterFromOpenAPI(
        CroupierClient client,
        string specJson,
        OpenAPIImportOptions? options,
        IReadOnlyDictionary<string, FunctionHandlerDelegate> handlers)
    {
        ArgumentNullException.ThrowIfNull(handlers);
        return RegisterFromOpenAPI(client, specJson, options, id => handlers.TryGetValue(id, out var handler) ? handler : null);
    }

    internal static string DeriveOperationId(JsonElement operation, string path)
    {
        if (operation.TryGetProperty("operationId", out var operationId)
            && operationId.ValueKind == JsonValueKind.String
            && !string.IsNullOrEmpty(operationId.GetString()))
        {
            return operationId.GetString()!;
        }
        if (!string.IsNullOrEmpty(path))
        {
            var segments = path.Split('/', StringSplitOptions.RemoveEmptyEntries);
            if (segments.Length > 0)
            {
                return string.Join(".", segments);
            }
        }
        return "unknown.function";
    }

    internal static string DeriveSummary(JsonElement operation, string functionId)
    {
        if (operation.TryGetProperty("summary", out var summary)
            && summary.ValueKind == JsonValueKind.String
            && !string.IsNullOrEmpty(summary.GetString()))
        {
            return summary.GetString()!;
        }
        return functionId != "unknown.function" ? ToTitleCase(functionId) : "Unnamed Function";
    }

    internal static string ToTitleCase(string value)
    {
        var words = value.Split('_');
        for (var i = 0; i < words.Length; i++)
        {
            if (words[i].Length > 0)
            {
                words[i] = char.ToUpperInvariant(words[i][0]) + words[i][1..].ToLowerInvariant();
            }
        }
        return string.Join(' ', words);
    }

    internal static string ExtractExtension(JsonElement operation, string key)
    {
        if (!operation.TryGetProperty(key, out var value))
        {
            return string.Empty;
        }
        return value.ValueKind switch
        {
            JsonValueKind.String => value.GetString() ?? string.Empty,
            JsonValueKind.True => "true",
            JsonValueKind.False => "false",
            _ => value.GetRawText(),
        };
    }

    internal static string ParseRiskLevel(string level)
    {
        var normalized = level.ToLowerInvariant();
        return normalized switch
        {
            "low" or "safe" => "low",
            "high" => "high",
            "danger" or "critical" => "danger",
            _ => "medium",
        };
    }

    private static FunctionDescriptor OperationToDescriptor(
        string path, JsonElement operation, OpenAPIImportOptions? options)
    {
        var functionId = DeriveOperationId(operation, path);
        var descriptor = new FunctionDescriptor
        {
            Id = functionId,
            Summary = DeriveSummary(operation, functionId),
            Tags = new List<string>(),
        };

        if (operation.TryGetProperty("description", out var description)
            && description.ValueKind == JsonValueKind.String)
        {
            descriptor.Description = description.GetString();
        }
        if (operation.TryGetProperty("tags", out var tags) && tags.ValueKind == JsonValueKind.Array)
        {
            foreach (var tag in tags.EnumerateArray())
            {
                if (tag.ValueKind == JsonValueKind.String)
                {
                    descriptor.Tags.Add(tag.GetString()!);
                }
            }
        }

        var resource = ExtractExtension(operation, "x-resource");
        if (resource.Length > 0)
        {
            descriptor.Resource = resource;
        }
        var operationName = ExtractExtension(operation, "x-operation");
        if (operationName.Length > 0)
        {
            descriptor.Operation = operationName;
        }
        var permission = ExtractExtension(operation, "x-permission");
        if (permission.Length > 0)
        {
            descriptor.Permission = permission;
        }

        if (operation.TryGetProperty("requestBody", out var requestBody))
        {
            descriptor.InputSchema = JsonContentSchema(requestBody);
        }
        if (operation.TryGetProperty("responses", out var responses)
            && responses.ValueKind == JsonValueKind.Object
            && responses.TryGetProperty("200", out var ok))
        {
            descriptor.OutputSchema = JsonContentSchema(ok);
        }

        var risk = ExtractExtension(operation, "x-risk");
        descriptor.Risk = risk.Length == 0 ? "medium" : ParseRiskLevel(risk);

        if (options is not null)
        {
            if (options.ResourcePrefix.Length > 0 && descriptor.Resource is { Length: > 0 } target)
            {
                descriptor.Resource = $"{options.ResourcePrefix}.{target}";
            }
            if (options.TagPrefix.Length > 0)
            {
                descriptor.Tags = descriptor.Tags!.Select(tag => options.TagPrefix + tag).ToList();
            }
        }
        return descriptor;
    }

    /// <summary>浅层 OpenAPI schema → JSON Schema 转换（与 Go 对齐）。</summary>
    private static string? JsonContentSchema(JsonElement holder)
    {
        if (holder.ValueKind != JsonValueKind.Object
            || !holder.TryGetProperty("content", out var content)
            || content.ValueKind != JsonValueKind.Object
            || !content.TryGetProperty("application/json", out var media)
            || media.ValueKind != JsonValueKind.Object
            || !media.TryGetProperty("schema", out var schema)
            || schema.ValueKind != JsonValueKind.Object
            || !schema.EnumerateObject().Any())
        {
            return null;
        }

        using var buffer = new MemoryStream();
        using (var writer = new Utf8JsonWriter(buffer))
        {
            WriteSimplifiedSchema(writer, schema);
        }
        return Encoding.UTF8.GetString(buffer.ToArray());
    }

    private static void WriteSimplifiedSchema(Utf8JsonWriter writer, JsonElement schema)
    {
        writer.WriteStartObject();
        if (schema.TryGetProperty("type", out var type) && type.ValueKind == JsonValueKind.String)
        {
            writer.WriteString("type", type.GetString());
        }
        if (schema.TryGetProperty("description", out var description)
            && description.ValueKind == JsonValueKind.String
            && !string.IsNullOrEmpty(description.GetString()))
        {
            writer.WriteString("description", description.GetString());
        }
        if (schema.TryGetProperty("properties", out var properties) && properties.ValueKind == JsonValueKind.Object)
        {
            writer.WritePropertyName("properties");
            writer.WriteStartObject();
            foreach (var property in properties.EnumerateObject())
            {
                if (property.Value.ValueKind != JsonValueKind.Object)
                {
                    continue;
                }
                writer.WritePropertyName(property.Name);
                writer.WriteStartObject();
                var propertyType = property.Value.TryGetProperty("type", out var pt) && pt.ValueKind == JsonValueKind.String
                    ? pt.GetString()
                    : "object";
                writer.WriteString("type", propertyType);
                if (property.Value.TryGetProperty("description", out var pd)
                    && pd.ValueKind == JsonValueKind.String
                    && !string.IsNullOrEmpty(pd.GetString()))
                {
                    writer.WriteString("description", pd.GetString());
                }
                writer.WriteEndObject();
            }
            writer.WriteEndObject();
        }
        if (schema.TryGetProperty("required", out var required) && required.ValueKind == JsonValueKind.Array)
        {
            var items = required.EnumerateArray().Where(item => item.ValueKind == JsonValueKind.String).ToList();
            if (items.Count > 0)
            {
                writer.WritePropertyName("required");
                writer.WriteStartArray();
                foreach (var item in items)
                {
                    writer.WriteStringValue(item.GetString());
                }
                writer.WriteEndArray();
            }
        }
        writer.WriteEndObject();
    }
}
