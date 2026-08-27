// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Text.Json;
using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Descriptor v2 OpenAPI import conversion tests: x-capability / x-execution /
/// x-approval extensions, path fallback IDs and ContinueOnError behavior.
/// </summary>
public sealed class OpenAPIImporterDescriptorV2Tests
{
    private const string V2Spec = """
    {
      "openapi": "3.0.3",
      "info": {"title": "GM API", "version": "1.0.0"},
      "paths": {
        "/players/ban": {
          "post": {
            "operationId": "player.ban",
            "summary": "Ban Player",
            "tags": ["player", "moderation"],
            "x-resource": "player",
            "x-operation": "ban",
            "x-capability": "action",
            "x-execution": "task",
            "x-permission": "player.ban.invoke",
            "x-risk": "high",
            "x-enabled": true,
            "x-approval": {"required": true, "policyKey": "gm.player.ban"},
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
        "/api/players/{id}": {
          "get": {
            "summary": "Get Player",
            "responses": {"200": {"content": {"application/json": {"schema": {
              "type": "object",
              "properties": {"name": {"type": "string"}}
            }}}}}
          }
        },
        "/orders/purge": {
          "post": {
            "operationId": "order.purge",
            "summary": "Purge Orders",
            "x-resource": "order",
            "x-operation": "purge",
            "x-capability": "delete",
            "x-execution": "task",
            "x-risk": "danger",
            "x-approval": {"required": false},
            "responses": {"200": {"description": "Success"}}
          }
        }
      }
    }
    """;

    private static JsonElement OperationAt(string path)
    {
        using var document = JsonDocument.Parse(V2Spec);
        var operation = document.RootElement.GetProperty("paths").GetProperty(path);
        foreach (var method in new[] { "get", "post" })
        {
            if (operation.TryGetProperty(method, out var value))
            {
                return value.Clone();
            }
        }
        throw new InvalidOperationException($"no operation under {path}");
    }

    [Fact]
    public void DescriptorV2_MapsCapabilityExecutionApprovalAndRisk()
    {
        var descriptor = OpenAPIImporter.OperationToDescriptor(
            "/players/ban", OperationAt("/players/ban"), null);

        descriptor.Id.Should().Be("player.ban");
        descriptor.Summary.Should().Be("Ban Player");
        descriptor.Resource.Should().Be("player");
        descriptor.Operation.Should().Be("ban");
        descriptor.Capability.Should().Be("action");
        descriptor.Execution.Should().Be("task");
        descriptor.Permission.Should().Be("player.ban.invoke");
        descriptor.Risk.Should().Be("high");
        descriptor.Enabled.Should().BeTrue();
        descriptor.ApprovalRequired.Should().BeTrue();
        descriptor.ApprovalPolicyKey.Should().Be("gm.player.ban");

        descriptor.InputSchema.Should().NotBeNull();
        using var input = JsonDocument.Parse(descriptor.InputSchema!);
        input.RootElement.GetProperty("type").GetString().Should().Be("object");
        input.RootElement.GetProperty("required").EnumerateArray()
            .Select(item => item.GetString()).Should().Equal("playerId", "reason");
        input.RootElement.GetProperty("properties").GetProperty("playerId")
            .GetProperty("type").GetString().Should().Be("string");

        descriptor.OutputSchema.Should().NotBeNull();
        using var output = JsonDocument.Parse(descriptor.OutputSchema!);
        output.RootElement.GetProperty("properties").GetProperty("ok")
            .GetProperty("type").GetString().Should().Be("boolean");
    }

    [Fact]
    public void DescriptorV2_ApprovalOptionalAndMissingExtensionsDefault()
    {
        var descriptor = OpenAPIImporter.OperationToDescriptor(
            "/orders/purge", OperationAt("/orders/purge"), null);

        descriptor.ApprovalRequired.Should().BeFalse();
        descriptor.ApprovalPolicyKey.Should().BeNull();
        descriptor.Capability.Should().Be("delete");
        descriptor.Execution.Should().Be("task");
        descriptor.Risk.Should().Be("danger");
        descriptor.OutputSchema.Should().BeNull();
    }

    [Fact]
    public void DescriptorV2_PathFallbackDerivesDottedId()
    {
        var descriptor = OpenAPIImporter.OperationToDescriptor(
            "/api/players/{id}", OperationAt("/api/players/{id}"), null);

        descriptor.Id.Should().Be("api.players.{id}");
        descriptor.Summary.Should().Be("Get Player");
        descriptor.Risk.Should().Be("warning");
        descriptor.Capability.Should().BeNull();
        descriptor.Execution.Should().BeNull();
    }

    [Fact]
    public void DescriptorV2_OptionsApplyPrefixes()
    {
        var descriptor = OpenAPIImporter.OperationToDescriptor(
            "/players/ban",
            OperationAt("/players/ban"),
            new OpenAPIImportOptions { ResourcePrefix = "demo", TagPrefix = "v2:" });

        descriptor.Resource.Should().Be("demo.player");
        descriptor.Tags.Should().Equal("v2:player", "v2:moderation");
    }

    [Fact]
    public void RegisterFromOpenAPI_ImportsV2FieldsEndToEnd()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });

        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, V2Spec,
            new OpenAPIImportOptions { ContinueOnError = true },
            id => id == "player.ban" ? (ctx, payload) => Task.FromResult("{}") : null);

        registered.Should().Equal("player.ban");
    }

    [Fact]
    public void RegisterFromOpenAPI_MissingHandlerWithoutContinueOnErrorThrows()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });

        var action = () => OpenAPIImporter.RegisterFromOpenAPI(
            client, V2Spec, null, _ => null);

        action.Should().Throw<InvalidOperationException>()
            .WithMessage("*no handler provided for function: player.ban*");
    }

    [Fact]
    public void RegisterFromOpenAPI_WithHandlersDictionaryRegistersAll()
    {
        using var client = new CroupierClient(new ClientConfig { AgentAddr = "127.0.0.1:1" });

        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, V2Spec, null,
            new Dictionary<string, FunctionHandlerDelegate>
            {
                ["player.ban"] = (ctx, payload) => Task.FromResult("{}"),
                ["api.players.{id}"] = (ctx, payload) => Task.FromResult("{}"),
                ["order.purge"] = (ctx, payload) => Task.FromResult("{}"),
            });

        registered.Should().Equal("player.ban", "api.players.{id}", "order.purge");
    }
}
