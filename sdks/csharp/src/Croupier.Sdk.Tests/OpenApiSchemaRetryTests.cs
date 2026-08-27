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
/// Tests for OpenAPI import, local Draft-07 schema validation, invoker retry
/// and client drain handling.
/// </summary>
public sealed class OpenApiSchemaRetryDrainTests
{
    // -----------------------------------------------------------------------
    // OpenAPI import
    // -----------------------------------------------------------------------

    private const string Spec = """
    {
      "openapi": "3.0.3",
      "info": {"title": "GM API", "version": "1.0.0"},
      "paths": {
        "/players/{id}/ban": {
          "put": {
            "operationId": "player_ban",
            "summary": "Ban player",
            "description": "Bans a player account",
            "tags": ["gm", "risk"],
            "x-resource": "player",
            "x-operation": "ban",
            "x-permission": "player.ban",
            "x-risk": "high",
            "requestBody": {"content": {"application/json": {"schema": {
              "type": "object",
              "required": ["playerId", "reason"],
              "properties": {
                "playerId": {"type": "string", "description": "Player ID"},
                "reason": {"type": "string"}
              }
            }}}},
            "responses": {"200": {"content": {"application/json": {"schema": {
              "type": "object",
              "properties": {"ok": {"type": "boolean"}}
            }}}}}
          }
        },
        "/players/search": {
          "get": {
            "tags": ["query"],
            "responses": {"200": {"content": {"application/json": {"schema": {"type": "array"}}}}}
          }
        }
      }
    }
    """;

    private sealed class RecordingRegistry
    {
        public List<KeyValuePair<FunctionDescriptor, Delegate>> Registered { get; } = new();
    }

    private static (CroupierClient Client, List<FunctionDescriptor> Seen) MakeRecordingClient()
    {
        var seen = new List<FunctionDescriptor>();
        var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        // Intercept registrations through a shim: register into the client but
        // observe the descriptors via the public wrapper below.
        return (client, seen);
    }

    [Fact]
    public void OpenAPIImporter_Helpers_MatchGoSemantics()
    {
        var empty = Parse("{}");
        OpenAPIImporter.DeriveOperationId(empty, "").Should().Be("unknown.function");
        OpenAPIImporter.DeriveOperationId(empty, "/api/players/{id}").Should().Be("api.players.{id}");
        OpenAPIImporter.ToTitleCase("player_ban").Should().Be("Player Ban");
        OpenAPIImporter.ParseRiskLevel("safe").Should().Be("safe");
        OpenAPIImporter.ParseRiskLevel("critical").Should().Be("danger");
        OpenAPIImporter.ParseRiskLevel("bogus").Should().Be("warning");
    }

    [Fact]
    public async Task OpenAPIImporter_RegistersAllOperationsOnClient()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, Spec, null, id => (ctx, payload) => Task.FromResult("{}"));

        registered.Should().Equal("player_ban", "players.search");
    }

    [Fact]
    public async Task OpenAPIImporter_MissingHandlerThrows()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        var action = () => OpenAPIImporter.RegisterFromOpenAPI(client, Spec, null, _ => null);

        action.Should().Throw<InvalidOperationException>()
            .WithMessage("*no handler provided for function: player_ban*");
    }

    [Fact]
    public async Task OpenAPIImporter_ContinueOnErrorSkipsUnhandled()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, Spec,
            new OpenAPIImportOptions { ContinueOnError = true },
            id => id == "players.search" ? (ctx, payload) => Task.FromResult("[]") : null);

        registered.Should().Equal("players.search");
    }

    [Fact]
    public void OpenAPIImporter_RejectsMalformedSpecs()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });
        ((Action)(() => OpenAPIImporter.RegisterFromOpenAPI(client, "{not json", null, _ => null)))
            .Should().Throw<ArgumentException>();
        ((Action)(() => OpenAPIImporter.RegisterFromOpenAPI(client, "{\"openapi\":\"3.0.3\"}", null, _ => null)))
            .Should().Throw<ArgumentException>();
    }

    // -----------------------------------------------------------------------
    // Draft-07 subset validation
    // -----------------------------------------------------------------------

    private static JsonElement Parse(string json)
    {
        using var document = System.Text.Json.JsonDocument.Parse(json);
        return document.RootElement.Clone();
    }

    [Fact]
    public void Validator_AcceptsValidPayloads()
    {
        var schema = Parse("""{"type":"object","required":["a"],"properties":{"a":{"type":"integer"}}}""");
        JsonSchemaValidator.IsValid(schema, Parse("""{"a":1}""")).Should().BeTrue();
    }

    [Fact]
    public void Validator_ReportsTypeRequiredAndNestedErrors()
    {
        var schema = Parse("""{"type":"object","required":["a"],"properties":{"a":{"type":"integer"}}}""");
        var errors = JsonSchemaValidator.Validate(schema, Parse("""{"a":"str"}"""));
        errors.Should().HaveCount(1);
        errors[0].Should().Contain("/a");

        JsonSchemaValidator.Validate(schema, Parse("{}")).Should().NotBeEmpty();
    }

    [Fact]
    public void Validator_EnforcesNumericAndStringConstraints()
    {
        JsonSchemaValidator.Validate(Parse("""{"type":"number","minimum":1,"maximum":10}"""), Parse("0.5"))
            .Should().HaveCount(1);
        JsonSchemaValidator.Validate(Parse("""{"type":"string","minLength":3,"pattern":"^[a-z]+$"}"""), Parse("\"AB\""))
            .Should().HaveCount(2);
        JsonSchemaValidator.Validate(Parse("""{"type":"number","exclusiveMinimum":1}"""), Parse("1"))
            .Should().NotBeEmpty();
        JsonSchemaValidator.Validate(Parse("""{"type":"number","multipleOf":3}"""), Parse("10"))
            .Should().NotBeEmpty();
    }

    [Fact]
    public void Validator_EnforcesEnumConstArraysAndAdditionalProperties()
    {
        JsonSchemaValidator.Validate(Parse("""{"enum":["a","b"]}"""), Parse("\"c\"")).Should().NotBeEmpty();
        JsonSchemaValidator.Validate(Parse("""{"const":7}"""), Parse("8")).Should().NotBeEmpty();
        JsonSchemaValidator.Validate(
            Parse("""{"type":"array","items":{"type":"integer"},"uniqueItems":true,"minItems":2}"""),
            Parse("[1]")).Should().NotBeEmpty();
        JsonSchemaValidator.Validate(
            Parse("""{"type":"object","properties":{"a":{}},"additionalProperties":false}"""),
            Parse("""{"a":1,"b":2}""")).Should().NotBeEmpty();
    }

    [Fact]
    public void Validator_ResolvesLocalRefs()
    {
        var schema = Parse(
            """{"definitions":{"item":{"type":"integer"}},"type":"array","items":{"$ref":"#/definitions/item"}}""");
        JsonSchemaValidator.IsValid(schema, Parse("[1,2]")).Should().BeTrue();
        JsonSchemaValidator.IsValid(schema, Parse("""[1,"x"]""")).Should().BeFalse();
    }

    [Fact]
    public void Validator_ValidatesJsonStrings()
    {
        JsonSchemaValidator.Validate("""{"a":1}""", """{"type":"object","required":["a"]}""")
            .Should().BeEmpty();
        JsonSchemaValidator.Validate("{}", """{"type":"object","required":["a"]}""")
            .Should().NotBeEmpty();
    }

    // -----------------------------------------------------------------------
    // Invoker schema wiring + retry
    // -----------------------------------------------------------------------

    private static CroupierInvoker CreateInvoker(HttpMessageHandler handler, InvokerConfig? config = null)
    {
        var client = new HttpClient(handler) { BaseAddress = new Uri("http://unused.invalid/") };
        return new CroupierInvoker(config ?? new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
        }, client, ownsHttpClient: true);
    }

    private static HttpResponseMessage JsonResponse(string json, HttpStatusCode status = HttpStatusCode.OK) => new(status)
    {
        Content = new StringContent(json, Encoding.UTF8, "application/json"),
    };

    [Fact]
    public async Task Invoker_SetSchema_ValidatesBeforeSending()
    {
        int calls = 0;
        var handler = new CountingHandler(_ => { calls++; return JsonResponse("{\"result\":{}}"); });
        using var invoker = CreateInvoker(handler);
        invoker.SetSchema("fn", """{"type":"object","required":["playerId"],"properties":{"playerId":{"type":"string","minLength":3}}}""");

        await invoker.InvokeAsync("fn", """{"playerId":"abc"}""");
        calls.Should().Be(1);

        var action = () => invoker.InvokeAsync("fn", """{"playerId":"ab"}""");
        await action.Should().ThrowAsync<ArgumentException>()
            .WithMessage("*payload validation failed*");
        calls.Should().Be(1); // no network call for invalid payloads

        var badType = () => invoker.InvokeAsync("fn", """{"playerId":42}""");
        await badType.Should().ThrowAsync<ArgumentException>();
    }

    [Fact]
    public async Task Invoker_ClearSchema_RemovesValidation()
    {
        var handler = new CountingHandler(_ => JsonResponse("{\"result\":{}}"));
        using var invoker = CreateInvoker(handler);
        invoker.SetSchema("fn", """{"type":"object","required":["a"]}""");
        invoker.ClearSchema("fn");

        await invoker.InvokeAsync("fn", "{}");
        handler.Calls.Should().Be(1);
    }

    [Fact]
    public async Task Invoker_RetriesRetryableFailuresUntilSuccess()
    {
        int calls = 0;
        var handler = new CountingHandler(_ =>
        {
            calls++;
            if (calls < 3)
            {
                return JsonResponse("{\"message\":\"flaky\"}", HttpStatusCode.ServiceUnavailable);
            }
            return JsonResponse("{\"result\":{\"ok\":true}}");
        });
        using var invoker = CreateInvoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            Retry = new RetryConfig { MaxAttempts = 3, InitialDelayMs = 1, JitterFactor = 0 },
        });

        var result = await invoker.InvokeAsync("fn", "{}");
        result.Success.Should().BeTrue();
        calls.Should().Be(3);
    }

    [Fact]
    public async Task Invoker_DoesNotRetryClientFailures()
    {
        int calls = 0;
        var handler = new CountingHandler(_ =>
        {
            calls++;
            return JsonResponse("{\"message\":\"missing\"}", HttpStatusCode.NotFound);
        });
        using var invoker = CreateInvoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            Retry = new RetryConfig { MaxAttempts = 5, InitialDelayMs = 1 },
        });

        var result = await invoker.InvokeAsync("fn", "{}");
        result.Success.Should().BeFalse();
        calls.Should().Be(1);
    }

    [Fact]
    public async Task Invoker_PerRequestRetryOverridesConfig()
    {
        int calls = 0;
        var handler = new CountingHandler(_ =>
        {
            calls++;
            return JsonResponse("{\"message\":\"down\"}", HttpStatusCode.BadGateway);
        });
        using var invoker = CreateInvoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            Retry = new RetryConfig { MaxAttempts = 5, InitialDelayMs = 1 },
        });

        var result = await invoker.InvokeAsync("fn", "{}", new InvokeOptions
        {
            Retry = new RetryConfig { Enabled = false },
        });
        result.Success.Should().BeFalse();
        calls.Should().Be(1);
    }

    [Fact]
    public async Task Invoker_StartTaskRetriesOn429()
    {
        int calls = 0;
        var handler = new CountingHandler(_ =>
        {
            calls++;
            if (calls == 1)
            {
                return JsonResponse("{\"message\":\"rate\"}", (HttpStatusCode)429);
            }
            return JsonResponse("{\"taskId\":\"t-9\"}");
        });
        using var invoker = CreateInvoker(handler, new InvokerConfig
        {
            ServerBaseUrl = "http://server.test/api/v1",
            Retry = new RetryConfig { MaxAttempts = 2, InitialDelayMs = 1, JitterFactor = 0 },
        });

        var taskId = await invoker.StartTaskAsync("fn", "{}");
        taskId.Should().Be("t-9");
        calls.Should().Be(2);
    }

    [Fact]
    public void RetryConfig_DelayAndStatusHelpers()
    {
        var retry = new RetryConfig { InitialDelayMs = 100, MaxDelayMs = 250, BackoffMultiplier = 2.0, JitterFactor = 0 };
        retry.DelayMs(10).Should().Be(250); // capped
        retry.IsRetryableStatus(429).Should().BeTrue();
        retry.IsRetryableStatus(503).Should().BeTrue();
        retry.IsRetryableStatus(404).Should().BeFalse();
        new RetryConfig { RetryableStatusCodes = new[] { 418 } }.IsRetryableStatus(418).Should().BeTrue();
    }

    private sealed class CountingHandler(Func<int, HttpResponseMessage> respond) : HttpMessageHandler
    {
        private int count;

        public int Calls => count;

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            count++;
            var index = count;
            return Task.FromResult(respond(index));
        }
    }

    // -----------------------------------------------------------------------
    // Drain handling (end-to-end with the built-in mock agent)
    // -----------------------------------------------------------------------

    [Fact]
    public async Task Client_DrainRequest_AcksWithDrainResponse()
    {
        using var agent = new MockAgent.MockAgentServer();
        agent.Start();
        using var client = new CroupierClient(new ClientConfig
        {
            AgentAddr = agent.Address,
            ServiceId = "drain-service",
            HeartbeatIntervalSeconds = 30,
            AutoReconnect = false,
        });
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.drain", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));
        await client.ConnectAsync();

        var body = await agent.SendUnsupportedInboundAsync(Protocol.MsgProviderDrainRequest);

        // The client must answer with a (parsable, empty) ProviderDrainResponse.
        var response = Croupier.Sdk.V1.ProviderDrainResponse.Parser.ParseFrom(body);
        response.Should().NotBeNull();

        // With no in-flight calls and auto-reconnect disabled, draining clears.
        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(5);
        while (client.IsDraining && DateTime.UtcNow < deadline)
        {
            await Task.Delay(50);
        }
        client.IsDraining.Should().BeFalse();
    }

    [Fact]
    public async Task Client_DrainRejectsNewInvokesWhileInFlightCallsFinish()
    {
        using var agent = new MockAgent.MockAgentServer();
        agent.Start();
        var release = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        using var client = new CroupierClient(new ClientConfig
        {
            AgentAddr = agent.Address,
            ServiceId = "drain-service",
            HeartbeatIntervalSeconds = 30,
            AutoReconnect = false,
        });
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.slow", Version = "1.0.0" },
            async (ctx, payload) =>
            {
                await release.Task;
                return "{}";
            });
        await client.ConnectAsync();

        // Start a slow call, then drain while it is in flight.
        var slowCall = agent.SendInboundInvokeAsync(new Croupier.Sdk.V1.InvokeRequest
        {
            FunctionId = "fn.slow",
            Payload = Google.Protobuf.ByteString.CopyFromUtf8("{}"),
        });
        await Task.Delay(200); // let the handler start
        client.ActiveInboundCalls.Should().BeGreaterThanOrEqualTo(1);

        await agent.SendUnsupportedInboundAsync(Protocol.MsgProviderDrainRequest);
        await Task.Delay(200);
        client.IsDraining.Should().BeTrue();

        // New invocations during drain are rejected with a JSON error payload.
        var rejected = await agent.SendInboundInvokeAsync(new Croupier.Sdk.V1.InvokeRequest
        {
            FunctionId = "fn.slow",
            Payload = Google.Protobuf.ByteString.CopyFromUtf8("{}"),
        });
        rejected.Payload.ToStringUtf8().Should().Contain("draining");

        // Completing the in-flight call lets the drain finish.
        release.SetResult();
        await slowCall;
        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(5);
        while (client.IsDraining && DateTime.UtcNow < deadline)
        {
            await Task.Delay(50);
        }
        client.IsDraining.Should().BeFalse();
        client.ActiveInboundCalls.Should().Be(0);
    }
}
