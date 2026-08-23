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

namespace Croupier.Sdk.Validation;

/// <summary>
/// 本地 JSON Schema（Draft-07 子集）校验器，与 Go SDK 的 Draft7 行为对齐，
/// 覆盖常用关键字：
/// type、enum、const、properties、required、additionalProperties、
/// items、minItems、maxItems、uniqueItems、minimum、maximum、
/// exclusiveMinimum、exclusiveMaximum、multipleOf、minLength、maxLength、
/// pattern 以及本地 <c>$ref</c>（<c>#/...</c>）。
/// 子集之外的关键字被忽略；Server 仍是权威校验方。
/// </summary>
public static class JsonSchemaValidator
{
    /// <summary>
    /// 校验 JSON 文本 <paramref name="payload"/> 是否符合 schema 文本
    /// <paramref name="schemaJson"/>。
    /// </summary>
    /// <returns>错误消息列表；为空表示通过。</returns>
    public static IReadOnlyList<string> Validate(string payload, string schemaJson)
    {
        using var schemaDocument = JsonDocument.Parse(schemaJson);
        using var payloadDocument = JsonDocument.Parse(payload);
        return Validate(schemaDocument.RootElement, payloadDocument.RootElement);
    }

    /// <summary>
    /// 校验 <paramref name="value"/> 是否符合 <paramref name="schema"/>。
    /// </summary>
    /// <returns>错误消息列表；为空表示通过。</returns>
    public static IReadOnlyList<string> Validate(JsonElement schema, JsonElement value)
    {
        var errors = new List<string>();
        ValidateCore(schema, schema, value, "$", errors, new HashSet<string>());
        return errors;
    }

    /// <summary>是否通过校验。</summary>
    public static bool IsValid(JsonElement schema, JsonElement value) => Validate(schema, value).Count == 0;

    private static void ValidateCore(
        JsonElement root,
        JsonElement schema,
        JsonElement value,
        string path,
        List<string> errors,
        HashSet<string> activeRefs)
    {
        if (schema.ValueKind != JsonValueKind.Object)
        {
            return; // 布尔 schema：true 放行；false 对 payload 场景不常见，按放行处理
        }

        if (schema.TryGetProperty("$ref", out var refElement)
            && refElement.ValueKind == JsonValueKind.String
            && refElement.GetString() is { } refText
            && refText.StartsWith("#/", StringComparison.Ordinal)
            && activeRefs.Add(refText))
        {
            if (TryResolvePointer(root, refText, out var resolved))
            {
                ValidateCore(root, resolved, value, path, errors, activeRefs);
            }
            else
            {
                errors.Add($"{path}: unresolved $ref '{refText}'");
            }
            return;
        }

        CheckType(schema, value, path, errors);
        CheckEnumAndConst(schema, value, path, errors);
        CheckNumeric(schema, value, path, errors);
        CheckString(schema, value, path, errors);
        CheckArray(root, schema, value, path, errors);
        CheckObject(root, schema, value, path, errors);
    }

    private static void CheckType(JsonElement schema, JsonElement value, string path, List<string> errors)
    {
        if (!TryGetTypeList(schema, out var allowed))
        {
            return;
        }
        foreach (var candidate in allowed!)
        {
            if (MatchesType(candidate, value))
            {
                return;
            }
        }
        errors.Add($"{path}: expected type [{string.Join(", ", allowed)}] but found {TypeName(value.ValueKind)}");
    }

    private static bool TryGetTypeList(JsonElement schema, out List<string>? allowed)
    {
        allowed = null;
        if (!schema.TryGetProperty("type", out var typeElement))
        {
            return false;
        }
        var list = new List<string>();
        if (typeElement.ValueKind == JsonValueKind.String)
        {
            list.Add(typeElement.GetString()!);
        }
        else if (typeElement.ValueKind == JsonValueKind.Array)
        {
            foreach (var item in typeElement.EnumerateArray())
            {
                if (item.ValueKind == JsonValueKind.String)
                {
                    list.Add(item.GetString()!);
                }
            }
        }
        if (list.Count == 0)
        {
            return false;
        }
        allowed = list;
        return true;
    }

    private static bool MatchesType(string type, JsonElement value) => type switch
    {
        "object" => value.ValueKind == JsonValueKind.Object,
        "array" => value.ValueKind == JsonValueKind.Array,
        "string" => value.ValueKind == JsonValueKind.String,
        "boolean" => value.ValueKind is JsonValueKind.True or JsonValueKind.False,
        "null" => value.ValueKind == JsonValueKind.Null,
        "number" => value.ValueKind == JsonValueKind.Number,
        "integer" => value.ValueKind == JsonValueKind.Number && IsIntegral(value),
        _ => true, // 未知类型名忽略（Server 权威校验）
    };

    private static bool IsIntegral(JsonElement value)
    {
        var raw = value.GetRawText();
        return !raw.Contains('.') && !raw.Contains('e') && !raw.Contains('E');
    }

    private static string TypeName(JsonValueKind kind) => kind switch
    {
        JsonValueKind.Object => "object",
        JsonValueKind.Array => "array",
        JsonValueKind.String => "string",
        JsonValueKind.True or JsonValueKind.False => "boolean",
        JsonValueKind.Null => "null",
        JsonValueKind.Number => "number",
        _ => kind.ToString().ToLowerInvariant(),
    };

    private static void CheckEnumAndConst(JsonElement schema, JsonElement value, string path, List<string> errors)
    {
        if (schema.TryGetProperty("enum", out var enumElement) && enumElement.ValueKind == JsonValueKind.Array)
        {
            var matched = false;
            foreach (var option in enumElement.EnumerateArray())
            {
                if (JsonEquals(option, value))
                {
                    matched = true;
                    break;
                }
            }
            if (!matched)
            {
                errors.Add($"{path}: value is not one of the allowed enum values");
            }
        }
        if (schema.TryGetProperty("const", out var constElement) && !JsonEquals(constElement, value))
        {
            errors.Add($"{path}: value does not match the const value");
        }
    }

    private static bool JsonEquals(JsonElement left, JsonElement right) =>
        left.ValueKind == JsonValueKind.Number && right.ValueKind == JsonValueKind.Number
            ? left.GetDouble() == right.GetDouble()
            : left.GetRawText() == right.GetRawText();

    private static void CheckNumeric(JsonElement schema, JsonElement value, string path, List<string> errors)
    {
        if (value.ValueKind != JsonValueKind.Number)
        {
            return;
        }
        var actual = value.GetDouble();
        if (TryGetNumber(schema, "minimum", out var minimum) && actual < minimum)
        {
            errors.Add($"{path}: value {actual} is less than minimum {minimum}");
        }
        if (TryGetNumber(schema, "maximum", out var maximum) && actual > maximum)
        {
            errors.Add($"{path}: value {actual} is greater than maximum {maximum}");
        }
        if (TryGetNumber(schema, "exclusiveMinimum", out var exclusiveMinimum) && actual <= exclusiveMinimum)
        {
            errors.Add($"{path}: value {actual} must be greater than {exclusiveMinimum}");
        }
        if (TryGetNumber(schema, "exclusiveMaximum", out var exclusiveMaximum) && actual >= exclusiveMaximum)
        {
            errors.Add($"{path}: value {actual} must be less than {exclusiveMaximum}");
        }
        if (TryGetNumber(schema, "multipleOf", out var divisor) && divisor != 0)
        {
            var quotient = actual / divisor;
            if (Math.Abs(quotient - Math.Round(quotient)) > 1e-9)
            {
                errors.Add($"{path}: value {actual} is not a multiple of {divisor}");
            }
        }
    }

    private static bool TryGetNumber(JsonElement schema, string key, out double number)
    {
        number = 0;
        return schema.TryGetProperty(key, out var element)
            && element.ValueKind == JsonValueKind.Number
            && element.TryGetDouble(out number);
    }

    private static void CheckString(JsonElement schema, JsonElement value, string path, List<string> errors)
    {
        if (value.ValueKind != JsonValueKind.String)
        {
            return;
        }
        var text = value.GetString()!;
        var length = text.Length;
        if (TryGetNumber(schema, "minLength", out var minLength) && length < minLength)
        {
            errors.Add($"{path}: string length {length} is less than minLength {minLength}");
        }
        if (TryGetNumber(schema, "maxLength", out var maxLength) && length > maxLength)
        {
            errors.Add($"{path}: string length {length} is greater than maxLength {maxLength}");
        }
        if (schema.TryGetProperty("pattern", out var patternElement)
            && patternElement.ValueKind == JsonValueKind.String
            && patternElement.GetString() is { } pattern)
        {
            try
            {
                if (!System.Text.RegularExpressions.Regex.IsMatch(text, pattern))
                {
                    errors.Add($"{path}: string does not match pattern '{pattern}'");
                }
            }
            catch (System.Text.RegularExpressions.RegexParseException)
            {
                // 非法 pattern 忽略；Server 权威校验。
            }
        }
    }

    private static void CheckArray(JsonElement root, JsonElement schema, JsonElement value, string path, List<string> errors)
    {
        if (value.ValueKind != JsonValueKind.Array)
        {
            return;
        }
        var count = value.GetArrayLength();
        if (TryGetNumber(schema, "minItems", out var minItems) && count < minItems)
        {
            errors.Add($"{path}: array has fewer than minItems {minItems}");
        }
        if (TryGetNumber(schema, "maxItems", out var maxItems) && count > maxItems)
        {
            errors.Add($"{path}: array has more than maxItems {maxItems}");
        }
        if (schema.TryGetProperty("uniqueItems", out var uniqueElement)
            && uniqueElement.ValueKind == JsonValueKind.True)
        {
            var seen = new HashSet<string>();
            foreach (var item in value.EnumerateArray())
            {
                if (!seen.Add(item.GetRawText()))
                {
                    errors.Add($"{path}: array items are not unique");
                    break;
                }
            }
        }
        if (schema.TryGetProperty("items", out var itemsElement) && itemsElement.ValueKind == JsonValueKind.Object)
        {
            var index = 0;
            foreach (var item in value.EnumerateArray())
            {
                ValidateCore(root, itemsElement, item, $"{path}/{index}", errors, new HashSet<string>());
                index++;
            }
        }
    }

    private static void CheckObject(JsonElement root, JsonElement schema, JsonElement value, string path, List<string> errors)
    {
        if (value.ValueKind != JsonValueKind.Object)
        {
            return;
        }

        if (schema.TryGetProperty("required", out var requiredElement) && requiredElement.ValueKind == JsonValueKind.Array)
        {
            foreach (var nameElement in requiredElement.EnumerateArray())
            {
                if (nameElement.ValueKind == JsonValueKind.String
                    && nameElement.GetString() is { } fieldName
                    && !value.TryGetProperty(fieldName, out _))
                {
                    errors.Add($"{path}: missing required property '{fieldName}'");
                }
            }
        }

        var properties = schema.TryGetProperty("properties", out var propertiesElement)
            && propertiesElement.ValueKind == JsonValueKind.Object
                ? propertiesElement
                : (JsonElement?)null;

        if (properties is { } propertyMap)
        {
            foreach (var property in propertyMap.EnumerateObject())
            {
                if (value.TryGetProperty(property.Name, out var propertyValue))
                {
                    ValidateCore(root, property.Value, propertyValue, $"{path}/{property.Name}", errors, new HashSet<string>());
                }
            }
        }

        if (schema.TryGetProperty("additionalProperties", out var additionalElement) && properties is { } declared)
        {
            foreach (var property in value.EnumerateObject())
            {
                if (declared.TryGetProperty(property.Name, out _))
                {
                    continue;
                }
                if (additionalElement.ValueKind == JsonValueKind.False)
                {
                    errors.Add($"{path}: additional property '{property.Name}' is not allowed");
                }
                else if (additionalElement.ValueKind == JsonValueKind.Object)
                {
                    ValidateCore(root, additionalElement, property.Value, $"{path}/{property.Name}", errors, new HashSet<string>());
                }
            }
        }
    }

    private static bool TryResolvePointer(JsonElement root, string pointer, out JsonElement resolved)
    {
        resolved = default;
        var current = root;
        foreach (var rawSegment in pointer.Substring(2).Split('/'))
        {
            if (rawSegment.Length == 0)
            {
                continue;
            }
            var segment = rawSegment.Replace("~1", "/").Replace("~0", "~");
            if (current.ValueKind != JsonValueKind.Object || !current.TryGetProperty(segment, out current))
            {
                return false;
            }
        }
        resolved = current;
        return true;
    }
}
