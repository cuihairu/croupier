// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Text;
using System.Text.Json;
using Croupier.Sdk.Configuration;
using Croupier.Sdk.Logging;
using Croupier.Sdk.Models;
using Croupier.Sdk.Threading;
using Croupier.Sdk.Transport;
using Croupier.Sdk.Validation;
using FluentAssertions;
using Microsoft.Extensions.Logging.Abstractions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// 覆盖率补测（第六轮，非 Client 部分）：环境/JSON 配置提供者、日志适配、
/// 协议命名、主线程调度、schema 校验器与 field hints、OpenAPI 导入边界、
/// TCP 传输端口解析与 Invoker 契约细节。
/// </summary>
public sealed class CoverageBoost6MiscTests
{
    // -----------------------------------------------------------------------
    // EnvironmentConfigProvider：全量环境变量与布尔/整数解析分支
    // -----------------------------------------------------------------------

    [Fact]
    public void EnvironmentConfigProvider_ReadsAllVariables()
    {
        // 使用独占前缀，避免与其他并行测试类对 CROUPIER_* 变量的读写竞态。
        const string prefix = "BOOST6_";
        var variables = new Dictionary<string, string>
        {
            [prefix + "AGENT_ADDR"] = "env-agent:1234",
            [prefix + "SERVICE_ID"] = "env-service",
            [prefix + "AGENT_ID"] = "env-agent-id",
            [prefix + "SERVICE_VERSION"] = "9.9.9",
            [prefix + "GAME_ID"] = "env-game",
            [prefix + "ENV"] = "prod",
            [prefix + "CONTROL_ADDR"] = "tcp://control:18780",
            [prefix + "INSECURE"] = "1",
            [prefix + "CERT_FILE"] = "/tmp/cert.pem",
            [prefix + "KEY_FILE"] = "/tmp/key.pem",
            [prefix + "CA_FILE"] = "/tmp/ca.pem",
            [prefix + "SERVER_NAME"] = "agent.internal",
            [prefix + "PROVIDER_LANG"] = "csharp-env",
            [prefix + "PROVIDER_SDK"] = "env-sdk",
            [prefix + "AUTH_TOKEN"] = "env-token",
            [prefix + "TIMEOUT_SECONDS"] = "42",
            [prefix + "HEARTBEAT_INTERVAL_SECONDS"] = "17",
            [prefix + "AUTO_RECONNECT"] = "false",
            [prefix + "RECONNECT_INTERVAL_SECONDS"] = "9",
            [prefix + "RECONNECT_MAX_ATTEMPTS"] = "4",
            [prefix + "MAX_CONCURRENT_MESSAGES"] = "31",
            [prefix + "MAX_MESSAGE_SIZE"] = "2048",
            [prefix + "DISABLE_LOGGING"] = "true",
            [prefix + "DEBUG_LOGGING"] = "true",
            [prefix + "LOG_LEVEL"] = "DEBUG",
            [prefix + "ENABLE_FILE_TRANSFER"] = "true",
            [prefix + "MAX_FILE_SIZE"] = "4096",
        };
        foreach (var pair in variables)
        {
            Environment.SetEnvironmentVariable(pair.Key, pair.Value);
        }

        try
        {
            var config = new EnvironmentConfigProvider(prefix).GetConfig();

            config.AgentAddr.Should().Be("env-agent:1234");
            config.ServiceId.Should().Be("env-service");
            config.AgentId.Should().Be("env-agent-id");
            config.ServiceVersion.Should().Be("9.9.9");
            config.GameId.Should().Be("env-game");
            config.Env.Should().Be("prod");
            config.ControlAddr.Should().Be("tcp://control:18780");
            config.Insecure.Should().BeTrue();
            config.CertFile.Should().Be("/tmp/cert.pem");
            config.KeyFile.Should().Be("/tmp/key.pem");
            config.CaFile.Should().Be("/tmp/ca.pem");
            config.ServerName.Should().Be("agent.internal");
            config.ProviderLang.Should().Be("csharp-env");
            config.ProviderSdk.Should().Be("env-sdk");
            config.AuthToken.Should().Be("env-token");
            config.TimeoutSeconds.Should().Be(42);
            config.HeartbeatIntervalSeconds.Should().Be(17);
            config.AutoReconnect.Should().BeFalse();
            config.ReconnectIntervalSeconds.Should().Be(9);
            config.ReconnectMaxAttempts.Should().Be(4);
            config.MaxConcurrentMessages.Should().Be(31);
            config.MaxMessageSize.Should().Be(2048);
            config.DisableLogging.Should().BeTrue();
            config.DebugLogging.Should().BeTrue();
            config.LogLevel.Should().Be("DEBUG");
            config.EnableFileTransfer.Should().BeTrue();
            config.MaxFileSize.Should().Be(4096);
        }
        finally
        {
            foreach (var key in variables.Keys)
            {
                Environment.SetEnvironmentVariable(key, null);
            }
        }
    }

    [Theory]
    [InlineData("TRUE", true)]
    [InlineData("1", true)]
    [InlineData("0", false)]
    [InlineData("no", false)]
    [InlineData("", true)] // 空值回退默认（INSECURE 默认 true）
    public void EnvironmentConfigProvider_InsecureBoolParsing(string value, bool expected)
    {
        const string prefix = "BOOST6B_";
        Environment.SetEnvironmentVariable(prefix + "INSECURE", value);
        try
        {
            new EnvironmentConfigProvider(prefix).GetConfig().Insecure.Should().Be(expected);
        }
        finally
        {
            Environment.SetEnvironmentVariable(prefix + "INSECURE", null);
        }
    }

    [Fact]
    public void EnvironmentConfigProvider_NonNumericIntegers_FallBackToDefaults()
    {
        const string prefix = "BOOST6C_";
        Environment.SetEnvironmentVariable(prefix + "TIMEOUT_SECONDS", "abc");
        Environment.SetEnvironmentVariable(prefix + "RECONNECT_INTERVAL_SECONDS", "abc");
        try
        {
            var config = new EnvironmentConfigProvider(prefix).GetConfig();
            config.TimeoutSeconds.Should().Be(30);
            config.ReconnectIntervalSeconds.Should().Be(5);
        }
        finally
        {
            Environment.SetEnvironmentVariable(prefix + "TIMEOUT_SECONDS", null);
            Environment.SetEnvironmentVariable(prefix + "RECONNECT_INTERVAL_SECONDS", null);
        }
    }

    // -----------------------------------------------------------------------
    // JsonFileConfigProvider：缺失字段 / 非法数值分支
    // -----------------------------------------------------------------------

    [Fact]
    public void JsonFileConfigProvider_ParsesProvidedFields()
    {
        var path = Path.Combine(Path.GetTempFileName());
        File.WriteAllText(path, """
            {"AgentAddr": "file-host:2000", "ServiceId": "file-svc", "TimeoutSeconds": "15", "Insecure": "true"}
            """);
        try
        {
            var config = new JsonFileConfigProvider(path).GetConfig();
            config.AgentAddr.Should().Be("file-host:2000");
            config.ServiceId.Should().Be("file-svc");
            config.TimeoutSeconds.Should().Be(15);
            config.Insecure.Should().BeTrue();
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void JsonFileConfigProvider_EmptyJson_KeepsDefaults()
    {
        var path = Path.GetTempFileName();
        File.WriteAllText(path, "{}");
        try
        {
            var config = new JsonFileConfigProvider(path).GetConfig();
            config.AgentAddr.Should().Be("127.0.0.1:19091");
            config.TimeoutSeconds.Should().Be(30);
            config.Insecure.Should().BeTrue();
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void JsonFileConfigProvider_InvalidNumericAndBoolValues_Normalize()
    {
        var path = Path.GetTempFileName();
        File.WriteAllText(path, """
            {"TimeoutSeconds": "not-a-number", "Insecure": "yes"}
            """);
        try
        {
            var config = new JsonFileConfigProvider(path).GetConfig();
            config.TimeoutSeconds.Should().Be(0);
            config.Insecure.Should().BeFalse();
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void JsonFileConfigProvider_MissingFile_Throws()
    {
        var action = () => new JsonFileConfigProvider("/nonexistent/config.json").GetConfig();
        action.Should().Throw<FileNotFoundException>();
    }

    // -----------------------------------------------------------------------
    // CroupierLogger 构造守卫
    // -----------------------------------------------------------------------

    [Fact]
    public void CroupierLogger_NullLogger_Throws()
    {
        var action = () => new CroupierLogger(null!);
        action.Should().Throw<ArgumentNullException>();
    }

    [Fact]
    public void CroupierLogger_DelegatesToILogger()
    {
        var logger = NullLogger.Instance;
        var adapter = new CroupierLogger(logger);
        var action = () =>
        {
            adapter.LogDebug("c", "m");
            adapter.LogInfo("c", "m");
            adapter.LogWarning("c", "m");
            adapter.LogError("c", "m");
            adapter.LogError("c", "m", new InvalidOperationException("boom"));
        };
        action.Should().NotThrow();
    }

    // -----------------------------------------------------------------------
    // Protocol.MsgIdString：全量命名与未知回退
    // -----------------------------------------------------------------------

    [Theory]
    [InlineData(Protocol.MsgRegisterRequest, "RegisterRequest")]
    [InlineData(Protocol.MsgRegisterResponse, "RegisterResponse")]
    [InlineData(Protocol.MsgHeartbeatRequest, "HeartbeatRequest")]
    [InlineData(Protocol.MsgHeartbeatResponse, "HeartbeatResponse")]
    [InlineData(Protocol.MsgInvokeRequest, "InvokeRequest")]
    [InlineData(Protocol.MsgInvokeResponse, "InvokeResponse")]
    [InlineData(Protocol.MsgStartTaskRequest, "StartTaskRequest")]
    [InlineData(Protocol.MsgStartTaskResponse, "StartTaskResponse")]
    [InlineData(Protocol.MsgStreamTaskRequest, "StreamTaskRequest")]
    [InlineData(Protocol.MsgTaskEvent, "TaskEvent")]
    [InlineData(Protocol.MsgCancelTaskRequest, "CancelTaskRequest")]
    [InlineData(Protocol.MsgCancelTaskResponse, "CancelTaskResponse")]
    [InlineData(Protocol.MsgProviderConnectRequest, "ProviderConnectRequest")]
    [InlineData(Protocol.MsgProviderConnectResponse, "ProviderConnectResponse")]
    [InlineData(Protocol.MsgProviderHeartbeatRequest, "ProviderHeartbeatRequest")]
    [InlineData(Protocol.MsgProviderHeartbeatResponse, "ProviderHeartbeatResponse")]
    [InlineData(Protocol.MsgProviderDrainRequest, "ProviderDrainRequest")]
    [InlineData(Protocol.MsgProviderDrainResponse, "ProviderDrainResponse")]
    [InlineData(Protocol.MsgGetTaskResultRequest, "GetTaskResultRequest")]
    [InlineData(Protocol.MsgGetTaskResultResponse, "GetTaskResultResponse")]
    public void MsgIdString_MapsKnownIds(int msgId, string expected)
    {
        Protocol.MsgIdString(msgId).Should().Be(expected);
    }

    [Fact]
    public void MsgIdString_UnknownId_UsesHexFallback()
    {
        Protocol.MsgIdString(0x123456).Should().Be("Unknown(0x123456)");
        Protocol.MsgIdString(Protocol.MsgTaskEvent + 0x1000).Should().StartWith("Unknown(0x");
    }

    // -----------------------------------------------------------------------
    // MainThreadDispatcher.IsMainThread 非主线程分支
    // -----------------------------------------------------------------------

    [Fact]
    public async Task MainThreadDispatcher_IsMainThreadFalseOnOtherThreads()
    {
        MainThreadDispatcher.Reset();
        MainThreadDispatcher.Initialize();

        MainThreadDispatcher.Instance.IsMainThread.Should().BeTrue();

        var offMainThread = await Task.Run(() => MainThreadDispatcher.Instance.IsMainThread);
        offMainThread.Should().BeFalse();

        MainThreadDispatcher.Reset();
    }

    [Fact]
    public void MainThreadDispatcher_BeforeInitialize_IsMainThreadFalse()
    {
        MainThreadDispatcher.Reset();
        MainThreadDispatcher.Instance.IsMainThread.Should().BeFalse();
        MainThreadDispatcher.Reset();
    }

    // -----------------------------------------------------------------------
    // JsonSchemaValidator 边界分支
    // -----------------------------------------------------------------------

    private static (JsonElement Schema, JsonElement Value) ParsePair(string schema, string value)
    {
        var schemaElement = JsonDocument.Parse(schema).RootElement.Clone();
        var valueElement = JsonDocument.Parse(value).RootElement.Clone();
        return (schemaElement, valueElement);
    }

    [Fact]
    public void Validator_UndefinedValueKind_FallsBackToTypeName()
    {
        var (schema, _) = ParsePair("{\"type\":\"object\"}", "{}");
        var errors = JsonSchemaValidator.Validate(schema, default);
        errors.Should().NotBeEmpty();
        errors[0].Should().Contain("undefined");
    }

    [Fact]
    public void Validator_RequiredArrayWithNonStringEntries_SkipsThem()
    {
        var (schema, value) = ParsePair(
            "{\"type\":\"object\",\"required\":[42, \"missing\"]}", "{}");
        var errors = JsonSchemaValidator.Validate(schema, value);
        errors.Should().ContainSingle().Which.Should().Contain("missing required property 'missing'");
    }

    [Fact]
    public void Validator_AdditionalPropertiesWithoutPropertiesKeyword_IsSkipped()
    {
        var (schema, value) = ParsePair(
            "{\"type\":\"object\",\"additionalProperties\":false}", "{\"extra\":1}");
        JsonSchemaValidator.Validate(schema, value).Should().BeEmpty();
    }

    [Fact]
    public void Validator_AdditionalPropertiesFalse_RejectsUndeclared()
    {
        var (schema, value) = ParsePair(
            "{\"type\":\"object\",\"properties\":{\"a\":{}},\"additionalProperties\":false}",
            "{\"a\":1,\"extra\":2}");
        var errors = JsonSchemaValidator.Validate(schema, value);
        errors.Should().ContainSingle().Which.Should().Contain("additional property 'extra'");
    }

    [Fact]
    public void Validator_AdditionalPropertiesSchema_ValidatesUndeclared()
    {
        var (schema, value) = ParsePair(
            "{\"type\":\"object\",\"properties\":{\"a\":{}},\"additionalProperties\":{\"type\":\"string\"}}",
            "{\"a\":1,\"extra\":2}");
        var errors = JsonSchemaValidator.Validate(schema, value);
        errors.Should().ContainSingle().Which.Should().Contain("extra");
    }

    [Fact]
    public void Validator_RefThroughMissingPointer_IsUnresolved()
    {
        var (schema, value) = ParsePair(
            "{\"$ref\":\"#/definitions/missing\"}", "1");
        var errors = JsonSchemaValidator.Validate(schema, value);
        errors.Should().ContainSingle().Which.Should().Contain("unresolved $ref '#/definitions/missing'");
    }

    [Fact]
    public void Validator_RefThroughNonObjectSegment_IsUnresolved()
    {
        var (schema, value) = ParsePair(
            "{\"definitions\":{\"name\":\"scalar-string\"},\"$ref\":\"#/definitions/name/child\"}", "1");
        var errors = JsonSchemaValidator.Validate(schema, value);
        errors.Should().ContainSingle().Which.Should().Contain("unresolved $ref");
    }

    // -----------------------------------------------------------------------
    // FieldHints.NormalizeHintKey 边界
    // -----------------------------------------------------------------------

    private static FunctionDescriptor NewDescriptor() => new()
    {
        Id = "fn.hints",
        Version = "1.0.0",
        InputSchema = "{\"type\":\"object\"}",
    };

    private static FunctionDescriptor Hint(string hint) =>
        FieldHints.SetFieldHint(NewDescriptor(), "name", hint, JsonDocument.Parse("\"v\"").RootElement);

    [Theory]
    [InlineData("")]
    [InlineData("   ")]
    [InlineData("ab")] // trim 后长度 < 3
    [InlineData("y-foo")] // 非 x 前缀
    [InlineData("xfoo")] // x 后无 -/_
    public void SetFieldHint_InvalidHint_Throws(string hint)
    {
        var action = () => Hint(hint);
        action.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void SetFieldHint_ValidPrefixVariants_AreAccepted()
    {
        Hint("x-widget").InputSchema.Should().Contain("x-widget");
        Hint("x_widget").InputSchema.Should().Contain("x-widget", "x_ 前缀归一为 x-");
        Hint("  X-Widget ").InputSchema.Should().Contain("x-Widget", "仅首字符归一小写");
    }

    [Fact]
    public void SetFieldHint_EmptyField_Throws()
    {
        var action = () => FieldHints.SetFieldHint(NewDescriptor(), "", "x-widget",
            JsonDocument.Parse("\"v\"").RootElement);
        action.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void SetFieldWidget_EmptyWidget_Throws()
    {
        var action = () => FieldHints.SetFieldWidget(NewDescriptor(), "name", "");
        action.Should().Throw<ArgumentException>();
    }

    // -----------------------------------------------------------------------
    // TCPTransport：端口越界与未连接即释放
    // -----------------------------------------------------------------------

    [Fact]
    public void TcpTransport_PortAboveRange_Throws()
    {
        var action = () => new TCPTransport("127.0.0.1:70000");
        action.Should().Throw<OverflowException>();
    }

    [Fact]
    public void TcpTransport_NegativePort_Throws()
    {
        var action = () => new TCPTransport("127.0.0.1:-1");
        action.Should().Throw<OverflowException>();
    }

    [Fact]
    public void TcpTransport_DisposeBeforeConnect_DoesNotThrow()
    {
        using var transport = new TCPTransport("127.0.0.1:19091");
        var action = () => transport.Dispose();
        action.Should().NotThrow();
    }
}
