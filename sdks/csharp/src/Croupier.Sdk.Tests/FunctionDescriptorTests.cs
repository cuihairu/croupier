// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for FunctionDescriptor model
/// </summary>
public class FunctionDescriptorTests
{
    [Fact]
    public void FunctionDescriptor_DefaultValues_ShouldBeCorrect()
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor();

        // Assert
        descriptor.Id.Should().BeEmpty();
        descriptor.Version.Should().Be("1.0.0");
        descriptor.Enabled.Should().BeTrue();
        descriptor.Risk.Should().Be("medium");
        descriptor.Category.Should().BeEmpty();
    }

    [Fact]
    public void FunctionDescriptor_WithFullConfiguration_ShouldHaveCorrectValues()
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor
        {
            Id = "player.profile.get",
            Version = "2.0.0",
            Category = "player",
            Entity = "profile",
            Operation = "get",
            DisplayName = "Get Player Profile",
            Description = "Retrieves the player's profile information",
            Risk = "low",
            Enabled = true,
            InputSchema = "{\"type\":\"object\"}",
            OutputSchema = "{\"type\":\"object\"}",
            Tags = new Dictionary<string, string>
            {
                ["owner"] = "team-player",
                ["deprecated"] = "false"
            }
        };

        // Assert
        descriptor.Id.Should().Be("player.profile.get");
        descriptor.Version.Should().Be("2.0.0");
        descriptor.Category.Should().Be("player");
        descriptor.Entity.Should().Be("profile");
        descriptor.Operation.Should().Be("get");
        descriptor.DisplayName.Should().Be("Get Player Profile");
        descriptor.Description.Should().Be("Retrieves the player's profile information");
        descriptor.Risk.Should().Be("low");
        descriptor.Enabled.Should().BeTrue();
        descriptor.InputSchema.Should().Be("{\"type\":\"object\"}");
        descriptor.OutputSchema.Should().Be("{\"type\":\"object\"}");
        descriptor.Tags.Should().HaveCount(2);
        descriptor.Tags!["owner"].Should().Be("team-player");
    }

    [Theory]
    [InlineData("low")]
    [InlineData("medium")]
    [InlineData("high")]
    [InlineData("critical")]
    public void FunctionDescriptor_Risk_AcceptsValidLevels(string riskLevel)
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor { Risk = riskLevel };

        // Assert
        descriptor.Risk.Should().Be(riskLevel);
    }

    [Fact]
    public void FunctionDescriptor_Tags_CanBeNull()
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor();

        // Assert
        descriptor.Tags.Should().BeNull();
    }

    [Fact]
    public void FunctionDescriptor_Tags_CanBeEmptyDictionary()
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor
        {
            Tags = new Dictionary<string, string>()
        };

        // Assert
        descriptor.Tags.Should().NotBeNull();
        descriptor.Tags.Should().BeEmpty();
    }

    [Theory]
    [InlineData("player.ban", "player", null, "ban")]
    [InlineData("wallet.transfer", "wallet", null, "transfer")]
    [InlineData("player.inventory.add", "player", "inventory", "add")]
    public void FunctionDescriptor_IdComponents_ShouldMatchIdPattern(
        string id, string category, string? entity, string operation)
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor
        {
            Id = id,
            Category = category,
            Entity = entity,
            Operation = operation
        };

        // Assert
        descriptor.Id.Should().Be(id);
        descriptor.Category.Should().Be(category);
        descriptor.Entity.Should().Be(entity);
        descriptor.Operation.Should().Be(operation);
    }

    [Fact]
    public void FunctionDescriptor_DisabledFunction_ShouldHaveEnabledFalse()
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor { Enabled = false };

        // Assert
        descriptor.Enabled.Should().BeFalse();
    }

    #region IsValid Tests

    [Fact]
    public void FunctionDescriptor_IsValid_ReturnsTrueForValidDescriptor()
    {
        // Arrange
        var descriptor = new FunctionDescriptor
        {
            Id = "player.get",
            Category = "player"
        };

        // Act & Assert
        descriptor.IsValid().Should().BeTrue();
    }

    [Fact]
    public void FunctionDescriptor_IsValid_ReturnsFalseForEmptyId()
    {
        // Arrange
        var descriptor = new FunctionDescriptor
        {
            Id = "",
            Category = "player"
        };

        // Act & Assert
        descriptor.IsValid().Should().BeFalse();
    }

    [Fact]
    public void FunctionDescriptor_IsValid_ReturnsFalseForEmptyCategory()
    {
        // Arrange
        var descriptor = new FunctionDescriptor
        {
            Id = "player.get",
            Category = ""
        };

        // Act & Assert
        descriptor.IsValid().Should().BeFalse();
    }

    [Fact]
    public void FunctionDescriptor_IsValid_ReturnsFalseForEmptyVersion()
    {
        // Arrange
        var descriptor = new FunctionDescriptor
        {
            Id = "player.get",
            Category = "player",
            Version = ""
        };

        // Act & Assert
        descriptor.IsValid().Should().BeFalse();
    }

    [Fact]
    public void FunctionDescriptor_IsValid_ReturnsFalseForEmptyRisk()
    {
        // Arrange
        var descriptor = new FunctionDescriptor
        {
            Id = "player.get",
            Category = "player",
            Risk = ""
        };

        // Act & Assert
        descriptor.IsValid().Should().BeFalse();
    }

    #endregion

    #region GetFullName Tests

    [Fact]
    public void FunctionDescriptor_GetFullName_ReturnsCategoryDotId()
    {
        // Arrange
        var descriptor = new FunctionDescriptor
        {
            Id = "get",
            Category = "player"
        };

        // Act
        var fullName = descriptor.GetFullName();

        // Assert
        fullName.Should().Be("player.get");
    }

    [Fact]
    public void FunctionDescriptor_GetFullName_WorksWithComplexId()
    {
        // Arrange
        var descriptor = new FunctionDescriptor
        {
            Id = "profile.details",
            Category = "player"
        };

        // Act
        var fullName = descriptor.GetFullName();

        // Assert
        fullName.Should().Be("player.profile.details");
    }

    #endregion

    #region Schema Tests

    [Fact]
    public void FunctionDescriptor_InputSchema_CanBeSet()
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor
        {
            InputSchema = "{\"type\":\"object\",\"properties\":{\"id\":{\"type\":\"string\"}}}"
        };

        // Assert
        descriptor.InputSchema.Should().Contain("type");
    }

    [Fact]
    public void FunctionDescriptor_OutputSchema_CanBeSet()
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor
        {
            OutputSchema = "{\"type\":\"object\",\"properties\":{\"result\":{\"type\":\"boolean\"}}}"
        };

        // Assert
        descriptor.OutputSchema.Should().Contain("result");
    }

    #endregion
}
