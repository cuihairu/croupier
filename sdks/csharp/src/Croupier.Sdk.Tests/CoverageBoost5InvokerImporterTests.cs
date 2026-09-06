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

using System.Net;
using System.Text;
using System.Text.Json;
using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// CroupierInvoker 与 OpenAPIImporter 低覆盖分支补测：SetSchema 参数校验、
/// 非法 JSON payload、operationId 兜底、扩展值类型转换与 schema 精简序列化边界。
/// </summary>
public sealed class CoverageBoost5InvokerImporterTests
{
    #region CroupierInvoker

    [Fact]
    public void SetSchema_EmptyFunctionId_Throws()
    {
        using var invoker = new CroupierInvoker(new InvokerConfig());

        var action = () => invoker.SetSchema("  ", "{\"type\":\"object\"}");

        action.Should().Throw<ArgumentException>()
            .WithMessage("*Function ID cannot be empty*");
    }

    [Fact]
    public async Task InvokeAsync_WithSchema_MalformedPayloadJson_ThrowsBeforeRequest()
    {
        var handler = new RecordingHandler(_ => JsonResponse("{}"));
        using var invoker = CreateInvoker(handler);
        invoker.SetSchema("fn.schema", "{\"type\":\"object\"}");

        var action = () => invoker.InvokeAsync("fn.schema", "{oops");

        (await action.Should().ThrowAsync<ArgumentException>())
            .WithMessage("*payload must be valid JSON*");
        handler.Requests.Should().BeEmpty();
    }

    private static CroupierInvoker CreateInvoker(RecordingHandler handler)
    {
        var client = new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") };
        return new CroupierInvoker(new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            TaskPollIntervalMilliseconds = 1,
        }, client, ownsHttpClient: true);
    }

    private static HttpResponseMessage JsonResponse(string json, HttpStatusCode statusCode = HttpStatusCode.OK) => new(statusCode)
    {
        Content = new StringContent(json, Encoding.UTF8, "application/json"),
    };

    private sealed class RecordingHandler : HttpMessageHandler
    {
        private readonly Func<RecordedRequest, HttpResponseMessage> _respond;

        public RecordingHandler(Func<RecordedRequest, HttpResponseMessage> respond)
        {
            _respond = respond;
        }

        public List<RecordedRequest> Requests { get; } = new();

        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var recorded = new RecordedRequest(
                request.Method,
                request.RequestUri!,
                request.Content == null ? string.Empty : await request.Content.ReadAsStringAsync(cancellationToken));
            Requests.Add(recorded);
            return _respond(recorded);
        }
    }

    private sealed record RecordedRequest(HttpMethod Method, Uri Uri, string Body);

    #endregion

    #region OpenAPIImporter

    [Fact]
    public void OpenAPIImportOptions_DefaultTimeoutMs_RoundTrips()
    {
        var options = new OpenAPIImportOptions { DefaultTimeoutMs = 7500 };

        options.DefaultTimeoutMs.Should().Be(7500);

        var cloned = options with { ResourcePrefix = "game" };
        cloned.DefaultTimeoutMs.Should().Be(7500);
        cloned.ResourcePrefix.Should().Be("game");
    }

    [Fact]
    public void DeriveOperationId_MissingOperationIdAndEmptyPath_ReturnsUnknownFallback()
    {
        using var operation = JsonDocument.Parse("{}");

        OpenAPIImporter.DeriveOperationId(operation.RootElement, "")
            .Should().Be("unknown.function");
        OpenAPIImporter.DeriveOperationId(operation.RootElement, "/")
            .Should().Be("unknown.function");
    }

    [Theory]
    [InlineData("true", "true")]
    [InlineData("false", "false")]
    [InlineData("42", "42")]
    [InlineData("{\"a\":1}", "{\"a\":1}")]
    [InlineData("[1,2]", "[1,2]")]
    public void ExtractExtension_NonStringValues_ReturnsCanonicalText(string json, string expected)
    {
        using var operation = JsonDocument.Parse($"{{\"x-widget\":{json}}}");

        OpenAPIImporter.ExtractExtension(operation.RootElement, "x-widget")
            .Should().Be(expected);
    }

    [Fact]
    public void OperationToDescriptor_SimplifiedSchemaKeepsDescriptionsAndSkipsNonObjectProperties()
    {
        var specJson = """
        {
          "summary": "create player",
          "requestBody": {
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "description": "player creation payload",
                  "properties": {
                    "name": { "type": "string", "description": "display name" },
                    "bad": "not-an-object"
                  },
                  "required": ["name"]
                }
              }
            }
          },
          "responses": {
            "200": {
              "content": {
                "application/json": {
                  "schema": {
                    "type": "object",
                    "description": "created player",
                    "properties": {
                      "id": { "type": "string" }
                    }
                  }
                }
              }
            }
          }
        }
        """;
        using var operation = JsonDocument.Parse(specJson);

        var descriptor = OpenAPIImporter.OperationToDescriptor("/players", operation.RootElement, null);

        using var input = JsonDocument.Parse(descriptor.InputSchema!);
        input.RootElement.GetProperty("description").GetString().Should().Be("player creation payload");
        input.RootElement.GetProperty("properties").GetProperty("name").GetProperty("description").GetString()
            .Should().Be("display name");
        input.RootElement.GetProperty("properties").EnumerateObject()
            .Select(property => property.Name).Should().Equal("name");
        input.RootElement.GetProperty("required").EnumerateArray()
            .Select(item => item.GetString()).Should().Equal("name");

        using var output = JsonDocument.Parse(descriptor.OutputSchema!);
        output.RootElement.GetProperty("description").GetString().Should().Be("created player");
    }

    [Fact]
    public void OperationToDescriptor_RequestBodyWithoutContent_LeavesSchemaNull()
    {
        using var operation = JsonDocument.Parse("""{"requestBody": {"description": "no content"}}""");

        var descriptor = OpenAPIImporter.OperationToDescriptor("/x", operation.RootElement, null);

        descriptor.InputSchema.Should().BeNull();
    }

    #endregion
}
