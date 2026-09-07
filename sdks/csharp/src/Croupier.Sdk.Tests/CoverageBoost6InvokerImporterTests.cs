// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Text;
using System.Text.Json;
using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// 覆盖率补测（第六轮，Invoker 与 OpenAPIImporter）：自定义 Headers 配置、
/// 空 payload 校验、事件流 done 缺省、低于 200 的状态码分支、导入容错
/// （ContinueOnError × 注册失败）、派生命名与 schema 简化边界。
/// </summary>
public sealed class CoverageBoost6InvokerImporterTests
{
    private sealed class SequenceHandler(Func<int, HttpResponseMessage> responder) : HttpMessageHandler
    {
        private int _calls;

        protected override Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var index = Interlocked.Increment(ref _calls);
            LastRequest = request;
            return Task.FromResult(responder(index));
        }

        public HttpRequestMessage? LastRequest { get; private set; }
    }

    private static HttpResponseMessage Json(string json, HttpStatusCode status = HttpStatusCode.OK) => new(status)
    {
        Content = new StringContent(json, Encoding.UTF8, "application/json"),
    };

    private static CroupierInvoker CreateInvoker(
        HttpMessageHandler handler,
        Action<InvokerConfig>? customize = null)
    {
        var config = new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            AuthToken = "token-6",
            GameId = "game-6",
            Env = "dev-6",
            TaskPollIntervalMilliseconds = 1,
        };
        customize?.Invoke(config);
        var client = new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") };
        return new CroupierInvoker(config, client, ownsHttpClient: true);
    }

    // -----------------------------------------------------------------------
    // InvokerConfig.Headers 自定义 + 空 payload 校验
    // -----------------------------------------------------------------------

    [Fact]
    public async Task InvokeAsync_CustomConfigHeaders_AreApplied()
    {
        var handler = new SequenceHandler(_ => Json("{\"result\":{}}"));
        using var invoker = CreateInvoker(handler, config =>
        {
            config.Headers = new Dictionary<string, string> { ["X-Custom-Header"] = "custom-value" };
        });

        await invoker.InvokeAsync("fn", "{}");

        handler.LastRequest!.Headers.GetValues("X-Custom-Header").Should().Equal("custom-value");
    }

    [Fact]
    public async Task InvokeAsync_EmptyPayloadWithSchema_ValidatesAsEmptyObject()
    {
        var handler = new SequenceHandler(_ => Json("{\"result\":{}}"));
        using var invoker = CreateInvoker(handler);
        invoker.SetSchema("fn.empty", "{\"type\":\"object\",\"required\":[\"id\"]}");

        // 空 payload 按 {} 参与校验：缺 required 字段直接拒绝（不发起请求）。
        var action = () => invoker.InvokeAsync("fn.empty", "");
        await action.Should().ThrowAsync<ArgumentException>()
            .WithMessage("*payload validation failed*id*");
    }

    [Fact]
    public async Task InvokeAsync_EmptyPayloadPassingValidation_FailsAtRequestEncoding()
    {
        var handler = new SequenceHandler(_ => Json("{\"result\":{}}"));
        using var invoker = CreateInvoker(handler);
        invoker.SetSchema("fn.empty-ok", "{\"type\":\"object\"}");

        var result = await invoker.InvokeAsync("fn.empty-ok", "");

        // 校验阶段空 payload 按 {} 通过；随后的请求体编码对空串报错。
        result.Success.Should().BeFalse();
        result.Error.Should().NotBeNullOrEmpty();
    }

    // -----------------------------------------------------------------------
    // StreamTaskAsync：响应缺少 done 字段时继续轮询
    // -----------------------------------------------------------------------

    [Fact]
    public async Task StreamTaskAsync_MissingDoneKey_KeepsPolling()
    {
        var handler = new SequenceHandler(n => n switch
        {
            1 => Json("{\"items\":[{\"seq\":1,\"type\":\"log\"}]}"), // 无 done 字段
            2 => Json("{\"items\":[],\"done\":true}"),
            _ => Json("{\"items\":[],\"done\":true}"),
        });
        using var invoker = CreateInvoker(handler);

        var types = new List<string>();
        await foreach (var evt in invoker.StreamTaskAsync("t6"))
        {
            types.Add(evt.Type);
        }

        types.Should().Equal("log");
    }

    // -----------------------------------------------------------------------
    // 非 2xx 且 < 200 的状态码分支
    // -----------------------------------------------------------------------

    [Fact]
    public async Task InvokeAsync_InformationalStatusCode_FailsRequest()
    {
        var handler = new SequenceHandler(_ => Json("{\"message\":\"processing\"}", (HttpStatusCode)199));
        using var invoker = CreateInvoker(handler, config =>
        {
            config.Retry = new RetryConfig { Enabled = false };
        });

        var result = await invoker.InvokeAsync("fn", "{}");

        result.Success.Should().BeFalse();
        result.Error.Should().NotBeNullOrEmpty();
    }

    // -----------------------------------------------------------------------
    // OpenAPIImporter：注册失败（client 已释放）× ContinueOnError
    // -----------------------------------------------------------------------

    private const string Spec = """
        {
          "openapi": "3.0.3",
          "info": {"title": "Boost6", "version": "1.0.0"},
          "paths": {
            "/fn/one": {
              "post": {
                "operationId": "fn.one",
                "responses": {"200": {"description": "OK"}}
              }
            }
          }
        }
        """;

    [Fact]
    public void RegisterFromOpenAPI_DisposedClientWithContinueOnError_SkipsFunction()
    {
        var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        client.Dispose();

        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, Spec,
            new OpenAPIImportOptions { ContinueOnError = true },
            id => (ctx, payload) => Task.FromResult("{}"));

        registered.Should().BeEmpty();
    }

    [Fact]
    public void RegisterFromOpenAPI_DisposedClientWithoutContinueOnError_WrapsFailure()
    {
        var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        client.Dispose();

        var action = () => OpenAPIImporter.RegisterFromOpenAPI(
            client, Spec, null, id => (ctx, payload) => Task.FromResult("{}"));

        action.Should().Throw<InvalidOperationException>()
            .WithMessage("*register function fn.one failed*");
    }

    [Fact]
    public void RegisterFromOpenAPI_DictionaryMissingHandlerWithContinueOnError_Skips()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });

        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, Spec,
            new OpenAPIImportOptions { ContinueOnError = true },
            new Dictionary<string, FunctionHandlerDelegate>());

        registered.Should().BeEmpty();
    }

    // -----------------------------------------------------------------------
    // OpenAPIImporter：派生命名与扩展解析边界（internal 直测）
    // -----------------------------------------------------------------------

    [Fact]
    public void DeriveSummary_UnknownFunctionId_ReturnsUnnamedFunction()
    {
        var operation = JsonDocument.Parse("{\"responses\":{}}").RootElement.Clone();
        OpenAPIImporter.DeriveSummary(operation, "unknown.function").Should().Be("Unnamed Function");
    }

    [Fact]
    public void ExtractExtension_NonStringValues_AreStringified()
    {
        var operation = JsonDocument.Parse(
            "{\"x-count\": 42, \"x-flag\": true, \"x-off\": false, \"x-missing\": null}").RootElement.Clone();

        OpenAPIImporter.ExtractExtension(operation, "x-count").Should().Be("42");
        OpenAPIImporter.ExtractExtension(operation, "x-flag").Should().Be("true");
        OpenAPIImporter.ExtractExtension(operation, "x-off").Should().Be("false");
        OpenAPIImporter.ExtractExtension(operation, "x-missing").Should().Be("null");
        OpenAPIImporter.ExtractExtension(operation, "x-absent").Should().BeEmpty();
    }

    [Theory]
    [InlineData("low", "safe")]
    [InlineData("safe", "safe")]
    [InlineData("medium", "warning")]
    [InlineData("moderate", "warning")]
    [InlineData("warning", "warning")]
    [InlineData("high", "high")]
    [InlineData("danger", "danger")]
    [InlineData("critical", "danger")]
    [InlineData("weird", "warning")]
    public void ParseRiskLevel_MapsAllKnownLevels(string input, string expected)
    {
        OpenAPIImporter.ParseRiskLevel(input).Should().Be(expected);
    }

    [Fact]
    public void OperationToDescriptor_ApprovalWithoutRequiredKey_KeepsDefault()
    {
        const string spec = """
            {
              "paths": {
                "/approval/only": {
                  "post": {
                    "operationId": "approval.only",
                    "x-approval": {"policyKey": "pk.1"},
                    "responses": {"200": {"description": "OK"}}
                  }
                }
              }
            }
            """;
        using var document = JsonDocument.Parse(spec);
        var operation = document.RootElement.GetProperty("paths")
            .GetProperty("/approval/only").GetProperty("post").Clone();

        var descriptor = OpenAPIImporter.OperationToDescriptor("/approval/only", operation, null);

        descriptor.ApprovalRequired.Should().BeFalse("缺失 required 键保持默认");
        descriptor.ApprovalPolicyKey.Should().Be("pk.1");
    }

    [Fact]
    public void OperationToDescriptor_NonBoolApprovalRequired_IsIgnored()
    {
        const string spec = """
            {
              "paths": {
                "/odd/op": {
                  "post": {
                    "operationId": "odd.op",
                    "x-approval": {"required": "yes", "policyKey": 7},
                    "x-enabled": false,
                    "responses": {"200": {"description": "OK"}}
                  }
                }
              }
            }
            """;
        using var document = JsonDocument.Parse(spec);
        var operation = document.RootElement.GetProperty("paths")
            .GetProperty("/odd/op").GetProperty("post").Clone();

        var descriptor = OpenAPIImporter.OperationToDescriptor("/odd/op", operation, null);

        descriptor.Id.Should().Be("odd.op");
        descriptor.ApprovalRequired.Should().BeFalse("非布尔 required 被忽略");
        descriptor.ApprovalPolicyKey.Should().BeNull("非字符串 policyKey 被忽略");
        descriptor.Enabled.Should().BeFalse();
    }

    [Fact]
    public void OperationToDescriptor_ResourcePrefixWithoutResource_KeepsNullResource()
    {
        const string spec = """
            {
              "paths": {
                "/plain/op": {
                  "post": {
                    "operationId": "plain.op",
                    "responses": {"200": {"description": "OK"}}
                  }
                }
              }
            }
            """;
        using var document = JsonDocument.Parse(spec);
        var operation = document.RootElement.GetProperty("paths")
            .GetProperty("/plain/op").GetProperty("post").Clone();

        var descriptor = OpenAPIImporter.OperationToDescriptor("/plain/op", operation,
            new OpenAPIImportOptions { ResourcePrefix = "demo", TagPrefix = "t:" });

        descriptor.Resource.Should().BeNull("无 x-resource 时前缀不生效");
        descriptor.Tags.Should().BeEmpty();
    }

    [Fact]
    public void OperationToDescriptor_SchemaSimplificationHandlesMissingTypes()
    {
        const string spec = """
            {
              "paths": {
                "/schema/edge": {
                  "post": {
                    "operationId": "schema.edge",
                    "requestBody": {"content": {"application/json": {"schema": {
                      "properties": {
                        "noType": {"description": "no type"},
                        "badType": {"type": 5},
                        "okType": {"type": "string"}
                      }
                    }}}},
                    "responses": {"200": {"description": "OK"}}
                  }
                }
              }
            }
            """;
        using var document = JsonDocument.Parse(spec);
        var operation = document.RootElement.GetProperty("paths")
            .GetProperty("/schema/edge").GetProperty("post").Clone();

        var descriptor = OpenAPIImporter.OperationToDescriptor("/schema/edge", operation, null);

        descriptor.InputSchema.Should().NotBeNull();
        using var parsed = JsonDocument.Parse(descriptor.InputSchema!);
        var properties = parsed.RootElement.GetProperty("properties");
        properties.GetProperty("noType").GetProperty("type").GetString().Should().Be("object");
        properties.GetProperty("badType").GetProperty("type").GetString().Should().Be("object");
        properties.GetProperty("okType").GetProperty("type").GetString().Should().Be("string");
        parsed.RootElement.TryGetProperty("type", out _).Should().BeFalse("顶层缺 type 时不写入");
    }
}
