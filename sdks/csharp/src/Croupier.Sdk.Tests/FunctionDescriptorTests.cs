// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for FunctionDescriptor capability contract fields.
/// </summary>
public class FunctionDescriptorTests
{
    [Fact]
    public void FunctionDescriptor_DefaultValues_ShouldBeCorrect()
    {
        var descriptor = new FunctionDescriptor();

        descriptor.Id.Should().BeEmpty();
        descriptor.Version.Should().Be("1.0.0");
        descriptor.Enabled.Should().BeTrue();
        descriptor.Risk.Should().Be("warning");
        descriptor.Resource.Should().BeNull();
        descriptor.Operation.Should().BeNull();
        descriptor.Permission.Should().BeNull();
    }

    [Fact]
    public void FunctionDescriptor_WithFullCapabilityContract_ShouldHaveCorrectValues()
    {
        var descriptor = new FunctionDescriptor
        {
            Id = "player.ban",
            Version = "2.0.0",
            Resource = "player",
            Operation = "ban",
            Risk = "danger",
            Permission = "player.ban",
            Summary = "Ban player",
            Description = "Ban a player account",
            OperationId = "banPlayer",
            Deprecated = true,
            Enabled = true,
            InputSchema = "{\"type\":\"object\"}",
            OutputSchema = "{\"type\":\"object\"}",
            Tags = new List<string> { "team-player", "moderation" }
        };

        descriptor.Id.Should().Be("player.ban");
        descriptor.Version.Should().Be("2.0.0");
        descriptor.Resource.Should().Be("player");
        descriptor.Operation.Should().Be("ban");
        descriptor.Risk.Should().Be("danger");
        descriptor.Permission.Should().Be("player.ban");
        descriptor.Summary.Should().Be("Ban player");
        descriptor.Description.Should().Be("Ban a player account");
        descriptor.OperationId.Should().Be("banPlayer");
        descriptor.Deprecated.Should().BeTrue();
        descriptor.Enabled.Should().BeTrue();
        descriptor.InputSchema.Should().Be("{\"type\":\"object\"}");
        descriptor.OutputSchema.Should().Be("{\"type\":\"object\"}");
        descriptor.Tags.Should().HaveCount(2);
        descriptor.Tags.Should().Contain("team-player");
        descriptor.Tags.Should().Contain("moderation");
    }

    [Theory]
    [InlineData("safe")]
    [InlineData("warning")]
    [InlineData("high")]
    [InlineData("danger")]
    public void FunctionDescriptor_Risk_AcceptsCapabilityRiskLevels(string riskLevel)
    {
        var descriptor = new FunctionDescriptor { Risk = riskLevel };

        descriptor.Risk.Should().Be(riskLevel);
    }

    [Fact]
    public void FunctionDescriptor_Tags_CanBeNull()
    {
        var descriptor = new FunctionDescriptor();

        descriptor.Tags.Should().BeNull();
    }

    [Fact]
    public void FunctionDescriptor_Tags_CanBeEmptyList()
    {
        var descriptor = new FunctionDescriptor
        {
            Tags = new List<string>()
        };

        descriptor.Tags.Should().NotBeNull();
        descriptor.Tags.Should().BeEmpty();
    }

    [Theory]
    [InlineData("player.ban", "player", "ban")]
    [InlineData("wallet.transfer", "wallet", "transfer")]
    [InlineData("mail.send", "mail", "send")]
    public void FunctionDescriptor_CapabilityFields_ShouldBeIndependentFromId(
        string id,
        string resource,
        string operation)
    {
        var descriptor = new FunctionDescriptor
        {
            Id = id,
            Resource = resource,
            Operation = operation
        };

        descriptor.Id.Should().Be(id);
        descriptor.Resource.Should().Be(resource);
        descriptor.Operation.Should().Be(operation);
    }

    [Fact]
    public void FunctionDescriptor_DisabledFunction_ShouldHaveEnabledFalse()
    {
        var descriptor = new FunctionDescriptor { Enabled = false };

        descriptor.Enabled.Should().BeFalse();
    }

    [Fact]
    public void FunctionDescriptor_IsValid_ReturnsTrueForIdAndVersion()
    {
        var descriptor = new FunctionDescriptor
        {
            Id = "player.get",
            Version = "1.0.0"
        };

        descriptor.IsValid().Should().BeTrue();
    }

    [Fact]
    public void FunctionDescriptor_IsValid_ReturnsFalseForEmptyId()
    {
        var descriptor = new FunctionDescriptor
        {
            Id = "",
            Version = "1.0.0"
        };

        descriptor.IsValid().Should().BeFalse();
    }

    [Fact]
    public void FunctionDescriptor_IsValid_ReturnsFalseForEmptyVersion()
    {
        var descriptor = new FunctionDescriptor
        {
            Id = "player.get",
            Version = ""
        };

        descriptor.IsValid().Should().BeFalse();
    }

    [Fact]
    public void FunctionDescriptor_InputSchema_CanBeSet()
    {
        var descriptor = new FunctionDescriptor
        {
            InputSchema = "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\"}}}"
        };

        descriptor.InputSchema.Should().Contain("type");
    }

    [Fact]
    public void FunctionDescriptor_OutputSchema_CanBeSet()
    {
        var descriptor = new FunctionDescriptor
        {
            OutputSchema = "{\"type\":\"object\",\"properties\":{\"result\":{\"type\":\"boolean\"}}}"
        };

        descriptor.OutputSchema.Should().Contain("result");
    }
}
