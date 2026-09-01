// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Text.Json;
using Croupier.Sdk.Validation;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// JsonSchemaValidator 字符串入口与各约束分支的缺口补测
/// （numeric/string/array/object 约束、$ref 解析失败、错误消息文案）。
/// </summary>
public class JsonSchemaValidatorBranchTests
{
    private static JsonElement Parse(string json)
    {
        using var doc = JsonDocument.Parse(json);
        return doc.RootElement.Clone();
    }

    [Fact]
    public void Validate_StringEntry_MissingRequired_ReportsPath()
    {
        var schema = """{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}""";
        var errors = JsonSchemaValidator.Validate("{}", schema);
        errors.Should().NotBeEmpty();
        errors[0].Should().Contain("name");
    }

    [Fact]
    public void Numeric_Bounds_AllBranches()
    {
        var schema = """
            {
                "type": "number",
                "minimum": 1,
                "maximum": 10,
                "exclusiveMinimum": 0.5,
                "exclusiveMaximum": 9.5,
                "multipleOf": 3
            }
            """;
        JsonSchemaValidator.Validate("0", schema)[0].Should().Contain("less than minimum");
        JsonSchemaValidator.Validate("11", schema)[0].Should().Contain("greater than maximum");
        JsonSchemaValidator.Validate("0.4", schema).Should().Contain(e => e.Contains("greater than 0.5") || e.Contains("exclusiveMinimum"));
        JsonSchemaValidator.Validate("9.5", schema).Should().Contain(e => e.Contains("less than 9.5"));
        JsonSchemaValidator.Validate("4", schema).Should().Contain(e => e.Contains("multiple of"));
        JsonSchemaValidator.Validate("6", schema).Should().BeEmpty();
    }

    [Fact]
    public void String_Constraints_AllBranches()
    {
        var schema = """
            { "type": "string", "minLength": 2, "maxLength": 5, "pattern": "^[a-z]+$" }
            """;
        JsonSchemaValidator.Validate("\"a\"", schema)[0].Should().Contain("minLength");
        JsonSchemaValidator.Validate("\"abcdef\"", schema)[0].Should().Contain("maxLength");
        JsonSchemaValidator.Validate("\"AB\"", schema).Should().Contain(e => e.Contains("pattern"));
        JsonSchemaValidator.Validate("\"abc\"", schema).Should().BeEmpty();
    }

    [Fact]
    public void String_InvalidPattern_IsIgnored()
    {
        // 非法正则（未闭合括号）——校验器必须忽略而不是抛出。
        var schema = """{"type":"string","pattern":"[unclosed"}""";
        var act = () => JsonSchemaValidator.Validate("\"anything\"", schema);
        act.Should().NotThrow();
    }

    [Fact]
    public void Array_Constraints_AllBranches()
    {
        var schema = """
            { "type": "array", "minItems": 2, "maxItems": 3, "uniqueItems": true }
            """;
        JsonSchemaValidator.Validate("[1]", schema)[0].Should().Contain("minItems");
        JsonSchemaValidator.Validate("[1,2,3,4]", schema)[0].Should().Contain("maxItems");
        JsonSchemaValidator.Validate("[1,1]", schema).Should().Contain(e => e.Contains("not unique"));
        JsonSchemaValidator.Validate("[1,2]", schema).Should().BeEmpty();
    }

    [Fact]
    public void Array_ItemsSchema_AppliedToElements()
    {
        var schema = """{"type":"array","items":{"type":"integer"}}""";
        JsonSchemaValidator.Validate("[1, \"x\"]", schema).Should().Contain(e => e.Contains("integer"));
        JsonSchemaValidator.Validate("[1, 2]", schema).Should().BeEmpty();
    }

    [Fact]
    public void Object_AdditionalProperty_Branches()
    {
        var noAdditional = """{"type":"object","properties":{"a":{"type":"string"}},"additionalProperties":false}""";
        JsonSchemaValidator.Validate("""{"a":"x","b":1}""", noAdditional)
            .Should().Contain(e => e.Contains("not allowed"));

        var typedAdditional = """{"type":"object","properties":{"a":{"type":"string"}},"additionalProperties":{"type":"number"}}""";
        JsonSchemaValidator.Validate("""{"a":"x","b":"not-a-number"}""", typedAdditional)
            .Should().Contain(e => e.Contains("number"));

        JsonSchemaValidator.Validate("""{"a":"x","b":7}""", typedAdditional).Should().BeEmpty();
    }

    [Fact]
    public void Ref_UnknownPointer_ReportsError()
    {
        var schema = """{"$ref":"#/definitions/missing","definitions":{}}""";
        var errors = JsonSchemaValidator.Validate("42", schema);
        errors.Should().Contain(e => e.Contains("$ref") || e.Contains("missing"));
    }

    [Fact]
    public void Ref_EscapedPointerSegments_Resolve()
    {
        // ~1 = /，~0 = ~ 的 JSON Pointer 转义。
        var schema = """
            {
                "$ref": "#/definitions/a~1b",
                "definitions": { "a/b": { "type": "string" } }
            }
            """;
        JsonSchemaValidator.Validate("\"ok\"", schema).Should().BeEmpty();
        JsonSchemaValidator.Validate("42", schema).Should().NotBeEmpty();
    }

    [Fact]
    public void Enum_MismatchReports_NonStringValues()
    {
        var schema = """{"enum":[1,2,3]}""";
        JsonSchemaValidator.Validate("4", schema).Should().Contain(e => e.Contains("enum"));
        JsonSchemaValidator.Validate("2", schema).Should().BeEmpty();
    }

    [Fact]
    public void Const_MismatchReports()
    {
        var schema = """{"const":"v1"}""";
        JsonSchemaValidator.Validate("\"v2\"", schema).Should().NotBeEmpty();
        JsonSchemaValidator.Validate("\"v1\"", schema).Should().BeEmpty();
    }

    [Fact]
    public void Integer_Type_ChecksIntegralForm()
    {
        var schema = """{"type":"integer"}""";
        JsonSchemaValidator.Validate("1.5", schema).Should().NotBeEmpty();
        JsonSchemaValidator.Validate("1e3", schema).Should().NotBeEmpty();
        JsonSchemaValidator.Validate("7", schema).Should().BeEmpty();
    }

    [Fact]
    public void TypeNames_InErrorMessages()
    {
        var schema = """{"type":"string"}""";
        foreach (var payload in new[] { "42", "true", "null", "[1]", """{"k":1}""" })
        {
            var errors = JsonSchemaValidator.Validate(payload, schema);
            errors.Should().NotBeEmpty($"payload {payload} is not a string");
        }
    }
}
