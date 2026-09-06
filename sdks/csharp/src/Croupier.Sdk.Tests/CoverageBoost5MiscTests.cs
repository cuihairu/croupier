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
using Croupier.Sdk.Configuration;
using Croupier.Sdk.Extensions;
using Croupier.Sdk.Models;
using Croupier.Sdk.Tests.MockAgent;
using Croupier.Sdk.Threading;
using Croupier.Sdk.Validation;
using FluentAssertions;
using Microsoft.Extensions.DependencyInjection;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// 杂项低覆盖分支补测：JsonSchemaValidator 边界、FieldHints 归一化、
/// MainThreadDispatcher 异常回调、CroupierClientExtensions 成功路径、
/// ServiceCollectionExtensions 参数校验与 JsonFileConfigProvider 类型转换。
/// </summary>
public sealed class CoverageBoost5MiscTests
{
    #region JsonSchemaValidator

    private static IReadOnlyList<string> Validate(string schemaJson, string payloadJson)
    {
        using var schema = JsonDocument.Parse(schemaJson);
        using var payload = JsonDocument.Parse(payloadJson);
        return JsonSchemaValidator.Validate(schema.RootElement, payload.RootElement);
    }

    [Fact]
    public void Validator_NonObjectSchema_IsAllowed()
    {
        Validate("true", "{}").Should().BeEmpty();
        Validate("\"just-a-string\"", "42").Should().BeEmpty();
    }

    [Fact]
    public void Validator_EmptyTypeArray_IsIgnored()
    {
        Validate("{\"type\":[]}", "\"text\"").Should().BeEmpty();
        Validate("{\"type\":[42, null]}", "\"text\"").Should().BeEmpty();
    }

    [Fact]
    public void Validator_BooleanType_MatchesOnlyBooleans()
    {
        Validate("{\"type\":\"boolean\"}", "true").Should().BeEmpty();
        Validate("{\"type\":\"boolean\"}", "false").Should().BeEmpty();
        Validate("{\"type\":\"boolean\"}", "1").Should().NotBeEmpty();
        Validate("{\"type\":\"boolean\"}", "\"true\"").Should().NotBeEmpty();
    }

    [Fact]
    public void Validator_RefPointerWithEmptySegment_ResolvesTarget()
    {
        // 指针 "#/definitions//name" 的空段应被跳过并解析到 definitions/name。
        const string schemaJson =
            "{\"definitions\":{\"name\":{\"type\":\"string\"}},\"properties\":{\"who\":{\"$ref\":\"#/definitions//name\"}}}";

        Validate(schemaJson, "{\"who\":\"abc\"}").Should().BeEmpty();
        Validate(schemaJson, "{\"who\":123}").Should().NotBeEmpty();
    }

    #endregion

    #region FieldHints

    private static FunctionDescriptor NewDescriptor(string? schema = null) => new()
    {
        Id = "fn.hints",
        Version = "1.0.0",
        InputSchema = schema,
    };

    [Fact]
    public void SetFieldHint_TooShortHint_Throws()
    {
        var action = () => FieldHints.SetFieldHint(NewDescriptor(), "name", "x",
            JsonSerializer.SerializeToElement("slider"));

        action.Should().Throw<ArgumentException>()
            .WithMessage("*must be an x- extension key*");
    }

    [Fact]
    public void SetFieldHint_HintWithoutXPrefix_Throws()
    {
        var action = () => FieldHints.SetFieldHint(NewDescriptor(), "name", "widget",
            JsonSerializer.SerializeToElement("slider"));

        action.Should().Throw<ArgumentException>()
            .WithMessage("*must be an x- extension key*");
    }

    [Fact]
    public void SetFieldHint_NullSchemaLiteral_Throws()
    {
        var action = () => FieldHints.SetFieldHint(NewDescriptor("null"), "name", "x-widget",
            JsonSerializer.SerializeToElement("slider"));

        action.Should().Throw<ArgumentException>()
            .WithMessage("*input schema must be a JSON object*");
    }

    [Fact]
    public void SetFieldHint_SchemaWithoutType_AddsObjectType()
    {
        var updated = FieldHints.SetFieldHint(
            NewDescriptor("{\"properties\":{\"name\":{\"type\":\"string\"}}}"),
            "name", "x-widget", JsonSerializer.SerializeToElement("slider"));

        using var schema = JsonDocument.Parse(updated.InputSchema!);
        schema.RootElement.GetProperty("type").GetString().Should().Be("object");
        schema.RootElement.GetProperty("properties").GetProperty("name")
            .GetProperty("x-widget").GetString().Should().Be("slider");
    }

    #endregion

    #region MainThreadDispatcher exception callback

    public sealed class MainThreadDispatcherErrorTests : IDisposable
    {
        public MainThreadDispatcherErrorTests()
        {
            MainThreadDispatcher.Reset();
            MainThreadDispatcher.Initialize();
        }

        public void Dispose()
        {
            MainThreadDispatcher.Reset();
        }

        [Fact]
        public void Enqueue_OnMainThread_ThrowingCallback_IsCaughtAndLogged()
        {
            var action = () => MainThreadDispatcher.Instance.Enqueue(
                () => throw new InvalidOperationException("callback boom"));

            action.Should().NotThrow();
        }

        [Fact]
        public void Enqueue_OnMainThread_DataCallbackThrowing_IsCaughtAndLogged()
        {
            var action = () => MainThreadDispatcher.Instance.Enqueue<int>(
                _ => throw new InvalidOperationException("data callback boom"), 42);

            action.Should().NotThrow();
        }
    }

    #endregion
}

/// <summary>CroupierClientExtensions 与 DI/配置补测（需要 MockAgent 或文件系统）。</summary>
public sealed class CoverageBoost5ExtensionsAndConfigTests : IDisposable
{
    private readonly MockAgentServer _agent = new();

    public CoverageBoost5ExtensionsAndConfigTests()
    {
        _agent.Start();
        MainThreadDispatcher.Reset();
        MainThreadDispatcher.Initialize();
    }

    public void Dispose()
    {
        MainThreadDispatcher.Reset();
        _agent.DisposeAsync().AsTask().GetAwaiter().GetResult();
    }

    #region CroupierClientExtensions success paths

    [Fact]
    public async Task InvokeOnMainThreadAsync_Success_RunsSuccessCallbackOnMainThread()
    {
        using var client = new CroupierClient(new ClientConfig
        {
            AgentAddr = _agent.Address,
            ServiceId = "ext-service",
            GameId = "g",
            Env = "e",
            AutoReconnect = false,
            HeartbeatIntervalSeconds = 30,
        });
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.ext", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("ext-result"));
        await client.ConnectAsync();

        var callbackResult = "unset";
        await client.InvokeOnMainThreadAsync("fn.ext", "{}", result => callbackResult = result);

        var deadline = DateTime.UtcNow.AddSeconds(5);
        while (callbackResult == "unset" && DateTime.UtcNow < deadline)
        {
            MainThreadDispatcher.Instance.ProcessQueue();
            await Task.Delay(20);
        }
        callbackResult.Should().Be("echo:{}");
    }

    [Fact]
    public async Task ConnectOnMainThreadAsync_Success_RunsSuccessCallbackOnMainThread()
    {
        using var client = new CroupierClient(new ClientConfig
        {
            AgentAddr = _agent.Address,
            ServiceId = "ext-service",
            GameId = "g",
            Env = "e",
            AutoReconnect = false,
            HeartbeatIntervalSeconds = 30,
        });
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.conn", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        var connected = false;
        await client.ConnectOnMainThreadAsync(() => connected = true);

        var deadline = DateTime.UtcNow.AddSeconds(5);
        while (!connected && DateTime.UtcNow < deadline)
        {
            MainThreadDispatcher.Instance.ProcessQueue();
            await Task.Delay(20);
        }
        connected.Should().BeTrue();
    }

    [Fact]
    public async Task ConnectOnMainThreadAsync_Failure_RunsErrorCallbackOnMainThread()
    {
        using var client = new CroupierClient(new ClientConfig
        {
            AgentAddr = "127.0.0.1:1", // 无监听
            ServiceId = "ext-service",
        });
        // 未注册函数：ConnectAsync 必然失败。

        Exception? captured = null;
        await client.ConnectOnMainThreadAsync(
            () => { },
            ex => captured = ex);

        var deadline = DateTime.UtcNow.AddSeconds(5);
        while (captured is null && DateTime.UtcNow < deadline)
        {
            MainThreadDispatcher.Instance.ProcessQueue();
            await Task.Delay(20);
        }
        captured.Should().NotBeNull();
        captured.Should().BeAssignableTo<Exception>();
    }

    #endregion

    #region ServiceCollectionExtensions guards

    [Fact]
    public void AddCroupier_NullServices_Throws()
    {
        IServiceCollection services = null!;

        var action = () => services.AddCroupier((ICroupierConfigProvider)new MemoryConfigProvider(new ClientConfig()));

        action.Should().Throw<ArgumentNullException>();
    }

    [Fact]
    public void AddCroupier_NullConfigProvider_Throws()
    {
        var services = new ServiceCollection();

        var action = () => services.AddCroupier((ICroupierConfigProvider)null!);

        action.Should().Throw<ArgumentNullException>();
    }

    #endregion

    #region JsonFileConfigProvider conversions

    private static string WriteTempConfig(string json)
    {
        var path = Path.Combine(Path.GetTempPath(), $"croupier-config-{Guid.NewGuid():N}.json");
        File.WriteAllText(path, json);
        return path;
    }

    [Fact]
    public void JsonFileConfigProvider_ParsesBoolAndStringProperties()
    {
        var path = WriteTempConfig(
            "{\"ServiceId\": \"file-service\", \"AutoReconnect\": \"true\", \"Insecure\": \"false\", \"TimeoutSeconds\": \"12\"}");
        try
        {
            var provider = new JsonFileConfigProvider(path);

            var config = provider.GetConfig();

            config.ServiceId.Should().Be("file-service");
            config.AutoReconnect.Should().BeTrue();
            config.Insecure.Should().BeFalse();
            config.TimeoutSeconds.Should().Be(12);
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void JsonFileConfigProvider_UnconvertibleProperty_Throws()
    {
        // Headers 是 Dictionary<string,string>：ConvertValue 走默认分支返回原始字符串，
        // SetValue 对不兼容类型抛 ArgumentException。
        var path = WriteTempConfig("{\"Headers\": \"not-a-dictionary\"}");
        try
        {
            var provider = new JsonFileConfigProvider(path);

            var action = () => provider.GetConfig();

            action.Should().Throw<ArgumentException>();
        }
        finally
        {
            File.Delete(path);
        }
    }

    #endregion
}
