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
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// F：Provider 侧入站 payload 校验（ValidateInputPayloads）。
/// </summary>
public class InboundValidationTests
{
    private static ClientConfig NewConfig(bool validate) => new ClientConfig
    {
        ValidateInputPayloads = validate,
    };

    private static FunctionDescriptor NewDescriptor() => new FunctionDescriptor
    {
        Id = "player.ban",
        Version = "1.0.0",
        InputSchema = "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\"}},\"required\":[\"id\"]}",
    };

    [Fact]
    public void Config_Defaults_To_Off()
    {
        Assert.False(new ClientConfig().ValidateInputPayloads);
    }

    [Fact]
    public void Validator_Accepts_Valid_Payload()
    {
        using var schemaDocument = System.Text.Json.JsonDocument.Parse(
            NewDescriptor().InputSchema!);
        using var payloadDocument = System.Text.Json.JsonDocument.Parse("{\"id\":\"p1\"}");
        var errors = Validation.JsonSchemaValidator.Validate(
            schemaDocument.RootElement, payloadDocument.RootElement);
        Assert.Empty(errors);
    }

    [Fact]
    public void Validator_Rejects_Missing_Required()
    {
        using var schemaDocument = System.Text.Json.JsonDocument.Parse(
            NewDescriptor().InputSchema!);
        using var payloadDocument = System.Text.Json.JsonDocument.Parse("{}");
        var errors = Validation.JsonSchemaValidator.Validate(
            schemaDocument.RootElement, payloadDocument.RootElement);
        Assert.NotEmpty(errors);
    }

    [Fact]
    public void Validator_Rejects_Type_Mismatch()
    {
        using var schemaDocument = System.Text.Json.JsonDocument.Parse(
            NewDescriptor().InputSchema!);
        using var payloadDocument = System.Text.Json.JsonDocument.Parse("{\"id\":123}");
        var errors = Validation.JsonSchemaValidator.Validate(
            schemaDocument.RootElement, payloadDocument.RootElement);
        Assert.NotEmpty(errors);
    }
}
