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

using System.Text.Json;
using Croupier.Sdk.Models;

namespace Croupier.Sdk.Validation;

/// <summary>
/// F：x-ui 呈现 hints 便捷层（契约见 docs/architecture/presentation-hints.md）。
/// 向函数描述符的 input schema 合并 x-* 呈现意图，供 Dashboard 生成更友好的表单。
/// </summary>
public static class FieldHints
{
    /// <summary>
    /// 向 input_schema 的 properties[field] 合并单个 x-* hint。
    /// schema 为空时创建 object 骨架；重复设置覆盖；
    /// hint 非法（非 x-/x_ 前缀）或 field 为空时抛 <see cref="ArgumentException"/>。
    /// </summary>
    /// <returns>合并后的新描述符（不可变风格，原描述符不变）</returns>
    public static FunctionDescriptor SetFieldHint(FunctionDescriptor descriptor, string field,
        string hint, JsonElement value)
    {
        var normalized = NormalizeHintKey(hint)
            ?? throw new ArgumentException(
                $"hint \"{hint}\" must be an x- extension key (e.g. x-widget)", nameof(hint));
        if (string.IsNullOrWhiteSpace(field))
        {
            throw new ArgumentException("field key is required for SetFieldHint", nameof(field));
        }

        // 不可变风格：克隆描述符后再修改（避免污染调用方持有的原对象）
        var workingDescriptor = JsonSerializer.Deserialize<FunctionDescriptor>(
            JsonSerializer.Serialize(descriptor));
        if (workingDescriptor == null)
        {
            throw new ArgumentException("descriptor could not be cloned", nameof(descriptor));
        }
        var schema = ParseSchemaObject(workingDescriptor.InputSchema);
        var properties = schema.TryGetValue("properties", out var propertiesElement)
                && propertiesElement.ValueKind == JsonValueKind.Object
            ? JsonElementToMutable(propertiesElement)
            : new Dictionary<string, JsonElement>();

        var fieldObject = properties.TryGetValue(field, out var fieldElement)
                && fieldElement.ValueKind == JsonValueKind.Object
            ? JsonElementToMutable(fieldElement)
            : new Dictionary<string, JsonElement>();
        fieldObject[normalized] = value.Clone();
        properties[field] = JsonSerializer.SerializeToElement(fieldObject);

        schema["properties"] = JsonSerializer.SerializeToElement(properties);
        workingDescriptor.InputSchema = JsonSerializer.Serialize(schema, new JsonSerializerOptions
        {
            WriteIndented = false,
        });
        return workingDescriptor;
    }

    /// <summary>等价于 <see cref="SetFieldHint"/>(descriptor, field, "x-widget", widget)。</summary>
    public static FunctionDescriptor SetFieldWidget(FunctionDescriptor descriptor, string field,
        string widget)
    {
        if (string.IsNullOrWhiteSpace(widget))
        {
            throw new ArgumentException("widget is required for SetFieldWidget", nameof(widget));
        }
        return SetFieldHint(descriptor, field, "x-widget",
            JsonSerializer.SerializeToElement(widget));
    }

    private static string? NormalizeHintKey(string hint)
    {
        if (string.IsNullOrWhiteSpace(hint) || hint.Trim().Length < 3)
        {
            return null;
        }
        var trimmed = hint.Trim();
        var first = char.ToLowerInvariant(trimmed[0]);
        if (first != 'x' || (trimmed[1] != '-' && trimmed[1] != '_'))
        {
            return null;
        }
        return "x-" + trimmed[2..];
    }

    private static Dictionary<string, JsonElement> ParseSchemaObject(string? raw)
    {
        if (string.IsNullOrWhiteSpace(raw))
        {
            return new Dictionary<string, JsonElement>
            {
                ["type"] = JsonSerializer.SerializeToElement("object"),
            };
        }
        var parsed = JsonSerializer.Deserialize<Dictionary<string, JsonElement>>(raw);
        if (parsed == null)
        {
            throw new ArgumentException("input schema must be a JSON object");
        }
        if (!parsed.ContainsKey("type"))
        {
            parsed["type"] = JsonSerializer.SerializeToElement("object");
        }
        return parsed;
    }

    private static Dictionary<string, JsonElement> JsonElementToMutable(JsonElement element)
    {
        var mutable = new Dictionary<string, JsonElement>();
        foreach (var property in element.EnumerateObject())
        {
            mutable[property.Name] = property.Value.Clone();
        }
        return mutable;
    }
}
