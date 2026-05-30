// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for InvokeOptions, FunctionContext, and InvokeResult models
/// </summary>
public class InvokeModelsTests
{
    #region InvokeOptions Tests

    [Fact]
    public void InvokeOptions_DefaultValues_ShouldBeCorrect()
    {
        // Arrange & Act
        var options = new InvokeOptions();

        // Assert
        options.TimeoutSeconds.Should().Be(30);
        options.IdempotencyKey.Should().BeNull();
        options.GameId.Should().BeNull();
        options.Env.Should().BeNull();
        options.Metadata.Should().BeNull();
    }

    [Fact]
    public void InvokeOptions_CustomTimeout_ShouldBeSettable()
    {
        // Arrange & Act
        var options = new InvokeOptions { TimeoutSeconds = 60 };

        // Assert
        options.TimeoutSeconds.Should().Be(60);
    }

    [Fact]
    public void InvokeOptions_IdempotencyKey_ShouldBeSettable()
    {
        // Arrange
        var key = Guid.NewGuid().ToString();

        // Act
        var options = new InvokeOptions { IdempotencyKey = key };

        // Assert
        options.IdempotencyKey.Should().Be(key);
    }

    [Fact]
    public void InvokeOptions_Metadata_ShouldBeSettable()
    {
        // Arrange & Act
        var options = new InvokeOptions
        {
            Metadata = new Dictionary<string, string>
            {
                ["X-Request-Id"] = "12345",
                ["X-Correlation-Id"] = "abc-def"
            }
        };

        // Assert
        options.Metadata.Should().HaveCount(2);
        options.Metadata!["X-Request-Id"].Should().Be("12345");
    }

    [Fact]
    public void InvokeOptions_GameIdAndEnv_ShouldBeSettable()
    {
        // Arrange & Act
        var options = new InvokeOptions
        {
            GameId = "my-game",
            Env = "production"
        };

        // Assert
        options.GameId.Should().Be("my-game");
        options.Env.Should().Be("production");
    }

    [Fact]
    public void InvokeOptions_RequestIdAndUserId_ShouldBeSettable()
    {
        // Arrange & Act
        var options = new InvokeOptions
        {
            RequestId = "req-123",
            UserId = "user-456"
        };

        // Assert
        options.RequestId.Should().Be("req-123");
        options.UserId.Should().Be("user-456");
    }

    #endregion

    #region FunctionContext Tests

    [Fact]
    public void FunctionContext_ShouldContainInvocationDetails()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "player.ban",
            CallId = "call-123",
            GameId = "my-game",
            Env = "production",
            UserId = "user-456"
        };

        // Assert
        context.FunctionId.Should().Be("player.ban");
        context.CallId.Should().Be("call-123");
        context.GameId.Should().Be("my-game");
        context.Env.Should().Be("production");
        context.UserId.Should().Be("user-456");
    }

    [Fact]
    public void FunctionContext_RequiredProperties_MustBeSet()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "wallet.transfer",
            CallId = "call-abc",
            GameId = "game-1",
            Env = "staging"
        };

        // Assert
        context.FunctionId.Should().NotBeNullOrEmpty();
        context.CallId.Should().NotBeNullOrEmpty();
        context.GameId.Should().NotBeNullOrEmpty();
        context.Env.Should().NotBeNullOrEmpty();
    }

    [Fact]
    public void FunctionContext_OptionalProperties_CanBeNull()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "test.func",
            CallId = "call-123",
            GameId = "game-1",
            Env = "test"
        };

        // Assert
        context.UserId.Should().BeNull();
        context.IdempotencyKey.Should().BeNull();
        context.CallerServiceId.Should().BeNull();
    }

    [Fact]
    public void FunctionContext_Timestamp_DefaultsToZero()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "test.func",
            CallId = "call-123",
            GameId = "game-1",
            Env = "test"
        };

        // Assert
        context.Timestamp.Should().Be(0);
    }

    #endregion

    #region InvokeResult Tests

    [Fact]
    public void InvokeResult_Succeeded_ShouldHaveCorrectState()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("{\"status\":\"ok\"}");

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().Be("{\"status\":\"ok\"}");
        result.Error.Should().BeNull();
    }

    [Fact]
    public void InvokeResult_Failed_ShouldHaveCorrectState()
    {
        // Arrange & Act
        var result = InvokeResult.Failed("Connection timeout");

        // Assert
        result.Success.Should().BeFalse();
        result.Data.Should().BeNull();
        result.Error.Should().Be("Connection timeout");
    }

    [Fact]
    public void InvokeResult_Succeeded_WithDuration()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("{\"data\":\"test\"}", 150);

        // Assert
        result.Success.Should().BeTrue();
        result.DurationMs.Should().Be(150);
    }

    [Fact]
    public void InvokeResult_Failed_WithErrorCode()
    {
        // Arrange & Act
        var result = InvokeResult.Failed("Not found", "NOT_FOUND");

        // Assert
        result.Success.Should().BeFalse();
        result.Error.Should().Be("Not found");
        result.ErrorCode.Should().Be("NOT_FOUND");
    }

    [Fact]
    public void InvokeResult_Failed_WithErrorCodeAndDuration()
    {
        // Arrange & Act
        var result = InvokeResult.Failed("Timeout", "TIMEOUT", 30000);

        // Assert
        result.Success.Should().BeFalse();
        result.Error.Should().Be("Timeout");
        result.ErrorCode.Should().Be("TIMEOUT");
        result.DurationMs.Should().Be(30000);
    }

    [Fact]
    public void InvokeResult_SucceededWithEmptyData_ShouldBeValid()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("");

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().BeEmpty();
    }

    [Fact]
    public void InvokeResult_FailedWithEmptyError_ShouldBeValid()
    {
        // Arrange & Act
        var result = InvokeResult.Failed("");

        // Assert
        result.Success.Should().BeFalse();
        result.Error.Should().BeEmpty();
    }

    [Theory]
    [InlineData("{\"player_id\":\"123\",\"name\":\"Test\"}")]
    [InlineData("[]")]
    [InlineData("null")]
    [InlineData("\"simple string\"")]
    public void InvokeResult_Succeeded_AcceptsVariousJsonFormats(string json)
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded(json);

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().Be(json);
    }

    [Theory]
    [InlineData("NOT_FOUND")]
    [InlineData("PERMISSION_DENIED")]
    [InlineData("INTERNAL_ERROR")]
    [InlineData("TIMEOUT")]
    public void InvokeResult_Failed_AcceptsVariousErrorMessages(string errorMessage)
    {
        // Arrange & Act
        var result = InvokeResult.Failed(errorMessage);

        // Assert
        result.Success.Should().BeFalse();
        result.Error.Should().Be(errorMessage);
    }

    [Fact]
    public void InvokeResult_ManualConstruction_RequiresSuccess()
    {
        // Arrange & Act
        var result = new InvokeResult
        {
            Success = true,
            Data = "{\"manual\":true}"
        };

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().Be("{\"manual\":true}");
    }

    #endregion

    #region BatchInvokeRequest Tests

    [Fact]
    public void BatchInvokeRequest_RequiredProperties_MustBeSet()
    {
        // Arrange & Act
        var request = new BatchInvokeRequest
        {
            FunctionId = "wallet.balance",
            Payload = "{\"playerId\":\"123\"}"
        };

        // Assert
        request.FunctionId.Should().Be("wallet.balance");
        request.Payload.Should().Be("{\"playerId\":\"123\"}");
    }

    [Fact]
    public void BatchInvokeRequest_IdempotencyKey_IsOptional()
    {
        // Arrange
        var request = new BatchInvokeRequest
        {
            FunctionId = "test.func",
            Payload = "{}"
        };

        // Assert
        request.IdempotencyKey.Should().BeNull();
    }

    #endregion

    #region JobStatus Tests

    [Fact]
    public void JobStatus_RequiredProperties_MustBeSet()
    {
        // Arrange & Act
        var status = new JobStatus
        {
            JobId = "job-123",
            Status = "running"
        };

        // Assert
        status.JobId.Should().Be("job-123");
        status.Status.Should().Be("running");
    }

    [Fact]
    public void JobStatus_OptionalProperties_HaveDefaults()
    {
        // Arrange
        var status = new JobStatus
        {
            JobId = "job-456",
            Status = "pending"
        };

        // Assert
        status.Progress.Should().Be(0);
        status.Error.Should().BeNull();
        status.Result.Should().BeNull();
    }

    #endregion
}
