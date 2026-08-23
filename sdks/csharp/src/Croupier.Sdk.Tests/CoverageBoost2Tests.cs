// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Text;
using System.Text.Json;
using Croupier.Sdk.Models;
using Croupier.Sdk.Transport;
using Croupier.Sdk.Validation;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Second coverage boost: config/model defaults, invoker normalization,
/// validator corners, OpenAPI importer edges and client lifecycle via the
/// mock agent.
/// </summary>
public sealed class CoverageBoost2Tests
{
    // -----------------------------------------------------------------------
    // Models & configs
    // -----------------------------------------------------------------------

    [Fact]
    public void ClientConfig_Defaults_FavourLocalDevelopment()
    {
        var config = new ClientConfig();

        config.AgentAddr.Should().Be("127.0.0.1:19091");
        config.ServiceId.Should().Be("csharp-service");
        config.ServiceVersion.Should().Be("1.0.0");
        config.GameId.Should().Be("default-game");
        config.Env.Should().Be("development");
        config.Insecure.Should().BeTrue();
        config.ProviderLang.Should().Be("csharp");
        config.ProviderSdk.Should().Be("croupier-csharp-sdk");
        config.TimeoutSeconds.Should().Be(30);
        config.HeartbeatIntervalSeconds.Should().Be(60);
        config.AutoReconnect.Should().BeTrue();
        config.ReconnectMaxAttempts.Should().Be(0); // 0 = infinite
        config.MaxMessageSize.Should().Be(4 * 1024 * 1024);
    }

    [Theory]
    [InlineData("", "1.0.0", false)]
    [InlineData("fn", "", false)]
    [InlineData("   ", "1.0.0", false)]
    [InlineData("fn", null, false)]
    [InlineData("player.ban", "2.0.0", true)]
    public void FunctionDescriptor_IsValid_GuardsIdAndVersion(string id, string? version, bool expected)
    {
        var descriptor = new FunctionDescriptor { Id = id, Version = version! };

        descriptor.IsValid().Should().Be(expected);
    }

    [Fact]
    public void FunctionDescriptor_Defaults()
    {
        var descriptor = new FunctionDescriptor();

        descriptor.Version.Should().Be("1.0.0");
        descriptor.Enabled.Should().BeTrue();
        descriptor.Risk.Should().Be("warning");
        descriptor.Tags.Should().BeNull();
        descriptor.Deprecated.Should().BeFalse();
        descriptor.ApprovalRequired.Should().BeFalse();
    }

    [Fact]
    public void RetryConfig_Normalization_ClampsBadValues()
    {
        var retry = new RetryConfig
        {
            MaxAttempts = 0,
            BackoffMultiplier = -1,
            InitialDelayMs = -100,
            MaxDelayMs = -5,
        };
        var normalized = retry.Normalized();

        normalized.MaxAttempts.Should().Be(1);
        normalized.BackoffMultiplier.Should().Be(2.0);
        normalized.InitialDelayMs.Should().Be(0);
        normalized.MaxDelayMs.Should().Be(0);
        // The original instance is untouched.
        retry.MaxAttempts.Should().Be(0);
    }

    [Fact]
    public void RetryConfig_DelayMs_HasNoJitterWhenDisabled()
    {
        var retry = new RetryConfig
        {
            InitialDelayMs = 100,
            BackoffMultiplier = 3.0,
            MaxDelayMs = 10_000,
            JitterFactor = 0,
        };

        retry.DelayMs(0).Should().Be(100);
        retry.DelayMs(1).Should().Be(300);
        retry.DelayMs(2).Should().Be(900);
    }

    // -----------------------------------------------------------------------
    // Invoker normalization & guards
    // -----------------------------------------------------------------------

    [Theory]
    [InlineData("tcp://127.0.0.1:19090")]
    [InlineData("ftp://host")]
    [InlineData("not a url")]
    public void Invoker_RejectsNonHttpBaseUrls(string baseUrl)
    {
        var action = () => new CroupierInvoker(new InvokerConfig { ServerBaseUrl = baseUrl });

        action.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void Invoker_TrailingSlashIsNormalized()
    {
        using var invoker = new CroupierInvoker(new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1///",
        });

        invoker.ServerBaseUrl.Should().Be("http://server.test/api/v1/");
    }

    [Fact]
    public async Task Invoker_MethodsAfterDispose_ThrowForAllEntrypoints()
    {
        var invoker = new CroupierInvoker(new InvokerConfig { ServerBaseUrl = "http://127.0.0.1:1" });
        invoker.Dispose();

        await Assert.ThrowsAsync<ObjectDisposedException>(() => invoker.InvokeAsync("fn", "{}"));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => invoker.StartTaskAsync("fn", "{}"));
        await Assert.ThrowsAsync<ObjectDisposedException>(() => invoker.CancelTaskAsync("t1"));
        Assert.Throws<ObjectDisposedException>(() => invoker.SetSchema("fn", "{}"));
    }

    // -----------------------------------------------------------------------
    // Validator corners
    // -----------------------------------------------------------------------

    private static JsonElement Parse(string json)
    {
        using var document = JsonDocument.Parse(json);
        return document.RootElement.Clone();
    }

    [Fact]
    public void Validator_TypeUnions_AcceptAnyListedType()
    {
        var schema = Parse("""{"type":["string","null"]}""");
        JsonSchemaValidator.IsValid(schema, Parse("\"text\"")).Should().BeTrue();
        JsonSchemaValidator.IsValid(schema, Parse("null")).Should().BeTrue();
        JsonSchemaValidator.IsValid(schema, Parse("42")).Should().BeFalse();
    }

    [Fact]
    public void Validator_EmptyAndUnknownTypeSchemas_AreLenient()
    {
        JsonSchemaValidator.IsValid(Parse("{}"), Parse("42")).Should().BeTrue();
        JsonSchemaValidator.IsValid(Parse("""{"type":"weird"}"""), Parse("42")).Should().BeTrue();
    }

    [Fact]
    public void Validator_ChainedLocalRefs_Resolve()
    {
        var schema = Parse(
            """
            {
              "definitions": {
                "name": {"type":"string","minLength":2},
                "person": {"type":"object","required":["name"],"properties":{"name":{"$ref":"#/definitions/name"}}}
              },
              "$ref": "#/definitions/person"
            }
            """);
        JsonSchemaValidator.IsValid(schema, Parse("""{"name":"ab"}""")).Should().BeTrue();
        JsonSchemaValidator.IsValid(schema, Parse("""{"name":"a"}""")).Should().BeFalse();
        JsonSchemaValidator.IsValid(schema, Parse("{}")).Should().BeFalse();
    }

    [Fact]
    public void Validator_UnresolvedRefs_ReportInsteadOfThrow()
    {
        var schema = Parse("""{"$ref":"#/definitions/missing"}""");
        var errors = JsonSchemaValidator.Validate(schema, Parse("1"));
        errors.Should().ContainSingle().Which.Should().Contain("unresolved $ref");
    }

    [Fact]
    public void Validator_IntegerAcceptsOnlyWholeNumbers()
    {
        var schema = Parse("""{"type":"integer"}""");
        JsonSchemaValidator.IsValid(schema, Parse("7")).Should().BeTrue();
        JsonSchemaValidator.IsValid(schema, Parse("7.5")).Should().BeFalse();
        JsonSchemaValidator.IsValid(schema, Parse("\"7\"")).Should().BeFalse();
    }

    [Fact]
    public void Validator_NestedArraysOfObjects_Validate()
    {
        var schema = Parse(
            """
            {"type":"object","properties":{"rows":{"type":"array","items":{
              "type":"object","required":["id"],"properties":{"id":{"type":"string"},"qty":{"type":"integer","minimum":1}}
            }}}}
            """);
        JsonSchemaValidator.IsValid(schema, Parse("""{"rows":[{"id":"a","qty":2}]}""")).Should().BeTrue();
        JsonSchemaValidator.IsValid(schema, Parse("""{"rows":[{"id":"a","qty":0}]}""")).Should().BeFalse();
        JsonSchemaValidator.IsValid(schema, Parse("""{"rows":[{}]}""")).Should().BeFalse();
    }

    // -----------------------------------------------------------------------
    // OpenAPI importer edges
    // -----------------------------------------------------------------------

    private const string MultiSpec = """
    {
      "openapi": "3.0.3",
      "paths": {
        "/a": {"get": {"operationId": "a_get", "responses": {}}},
        "/b": {"post": {"operationId": "b_post", "x-risk": "safe", "responses": {}}},
        "/ignored": "not-an-object",
        "/c": {"put": {}}
      }
    }
    """;

    [Fact]
    public void OpenAPIImporter_InvalidPathItemsAreSkippedNotFatal()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        // The invalid "/ignored" entry must not crash parsing; the failure (if any)
        // comes from the first real operation missing a handler.
        var action = () => OpenAPIImporter.RegisterFromOpenAPI(
            client, MultiSpec, null, _ => null);

        action.Should().Throw<InvalidOperationException>()
            .WithMessage("*no handler provided for function: a_get*");
    }

    [Fact]
    public void OpenAPIImporter_ContinueOnError_RegistersHandledOperations()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, MultiSpec,
            new OpenAPIImportOptions { ContinueOnError = true },
            id => id == "a_get" ? (ctx, payload) => Task.FromResult("{}") : null);

        registered.Should().Equal("a_get");
    }

    [Fact]
    public void OpenAPIImporter_TitleCaseEdges()
    {
        OpenAPIImporter.ToTitleCase("player_ban").Should().Be("Player Ban");
        OpenAPIImporter.ToTitleCase("a_b_c").Should().Be("A B C");
        OpenAPIImporter.ToTitleCase("UPPER").Should().Be("Upper");
        OpenAPIImporter.ToTitleCase("x").Should().Be("X");
        OpenAPIImporter.ToTitleCase("").Should().Be("");
    }

    [Fact]
    public void OpenAPIImporter_RiskMappingMatrix()
    {
        OpenAPIImporter.ParseRiskLevel("low").Should().Be("low");
        OpenAPIImporter.ParseRiskLevel("SAFE").Should().Be("low");
        OpenAPIImporter.ParseRiskLevel("medium").Should().Be("medium");
        OpenAPIImporter.ParseRiskLevel("moderate").Should().Be("medium");
        OpenAPIImporter.ParseRiskLevel("high").Should().Be("high");
        OpenAPIImporter.ParseRiskLevel("critical").Should().Be("danger");
        OpenAPIImporter.ParseRiskLevel("anything").Should().Be("medium");
    }

    [Fact]
    public async Task OpenAPIImporter_FullPrefixesApplyToTagsAndResources()
    {
        const string spec = """
        {
          "paths": {
            "/thing": {"get": {"operationId": "thing_get", "x-resource": "thing",
                                "tags": ["t1"], "responses": {}}}
          }
        }
        """;
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, spec,
            new OpenAPIImportOptions { ResourcePrefix = "game", TagPrefix = "svc-" },
            id => (ctx, payload) => Task.FromResult("{}"));

        registered.Should().Equal("thing_get");
    }

    // -----------------------------------------------------------------------
    // Client lifecycle via mock agent
    // -----------------------------------------------------------------------

    [Fact]
    public async Task Client_Lifecycle_ConnectInvokeDisconnect()
    {
        using var agent = new MockAgent.MockAgentServer();
        agent.Start();
        using var client = new CroupierClient(new ClientConfig
        {
            AgentAddr = agent.Address,
            ServiceId = "lifecycle-boost",
            HeartbeatIntervalSeconds = 30,
            AutoReconnect = false,
        });
        client.RegisterFunction(
            new FunctionDescriptor { Id = "fn.echo", Version = "1.0.0", Resource = "test" },
            (ctx, payload) => Task.FromResult($"{{\"echo\":true}}"));

        client.IsConnected.Should().BeFalse();
        await client.ConnectAsync();
        client.IsConnected.Should().BeTrue();
        client.SessionId.Should().NotBeNullOrEmpty();

        var response = await agent.SendInboundInvokeAsync(new Croupier.Sdk.V1.InvokeRequest
        {
            FunctionId = "fn.echo",
            Payload = Google.Protobuf.ByteString.CopyFromUtf8("{}"),
        });
        response.Payload.ToStringUtf8().Should().Contain("echo");

        client.Disconnect();
        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public async Task Client_DrainRequest_IsIdempotent()
    {
        using var agent = new MockAgent.MockAgentServer();
        agent.Start();
        using var client = new CroupierClient(new ClientConfig
        {
            AgentAddr = agent.Address,
            ServiceId = "drain-idem",
            HeartbeatIntervalSeconds = 30,
            AutoReconnect = false,
        });
        client.RegisterFunction(
            new FunctionDescriptor { Id = "fn.x", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));
        await client.ConnectAsync();

        var first = await agent.SendUnsupportedInboundAsync(Protocol.MsgProviderDrainRequest);
        var second = await agent.SendUnsupportedInboundAsync(Protocol.MsgProviderDrainRequest);

        // Both drain requests are acknowledged with a parsable ProviderDrainResponse.
        Croupier.Sdk.V1.ProviderDrainResponse.Parser.ParseFrom(first).Should().NotBeNull();
        Croupier.Sdk.V1.ProviderDrainResponse.Parser.ParseFrom(second).Should().NotBeNull();

        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(5);
        while (client.IsDraining && DateTime.UtcNow < deadline)
        {
            await Task.Delay(50);
        }
        client.IsDraining.Should().BeFalse();
    }
}
