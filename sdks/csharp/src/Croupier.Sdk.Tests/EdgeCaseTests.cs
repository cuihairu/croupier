// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using Croupier.Sdk.Transport;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for edge cases and error scenarios
/// </summary>
public class EdgeCaseTests
{
    #region InvokeOptions Edge Cases

    [Fact]
    public void InvokeOptions_ZeroTimeout_IsAllowed()
    {
        // Arrange & Act
        var options = new InvokeOptions { TimeoutSeconds = 0 };

        // Assert
        options.TimeoutSeconds.Should().Be(0);
    }

    [Fact]
    public void InvokeOptions_VeryLargeTimeout_IsAllowed()
    {
        // Arrange & Act
        var options = new InvokeOptions { TimeoutSeconds = 3600 }; // 1 hour

        // Assert
        options.TimeoutSeconds.Should().Be(3600);
    }

    [Fact]
    public void InvokeOptions_EmptyIdempotencyKey_IsAllowed()
    {
        // Arrange & Act
        var options = new InvokeOptions { IdempotencyKey = "" };

        // Assert
        options.IdempotencyKey.Should().Be("");
    }

    [Fact]
    public void InvokeOptions_NullMetadata_IsAllowed()
    {
        // Arrange & Act
        var options = new InvokeOptions();

        // Assert
        options.Metadata.Should().BeNull();
    }

    [Fact]
    public void InvokeOptions_EmptyMetadata_IsAllowed()
    {
        // Arrange & Act
        var options = new InvokeOptions
        {
            Metadata = new Dictionary<string, string>()
        };

        // Assert
        options.Metadata.Should().NotBeNull();
        options.Metadata.Should().BeEmpty();
    }

    [Fact]
    public void InvokeOptions_Metadata_CanHandleManyValues()
    {
        // Arrange & Act
        var options = new InvokeOptions
        {
            Metadata = new Dictionary<string, string>
            {
                ["X-Request-Id"] = Guid.NewGuid().ToString(),
                ["X-Correlation-Id"] = "corr-123",
                ["X-User-Id"] = "user-456",
                ["X-Session-Id"] = "session-789",
                ["X-Trace-Id"] = "trace-101",
                ["Custom-Header-1"] = "value1",
                ["Custom-Header-2"] = "value2",
                ["Custom-Header-3"] = "value3",
                ["Custom-Header-4"] = "value4",
                ["Custom-Header-5"] = "value5"
            }
        };

        // Assert
        options.Metadata.Should().HaveCount(10);
    }

    #endregion

    #region InvokeResult Edge Cases

    [Fact]
    public void InvokeResult_CanBeCreatedWithNullData()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded(null!);

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().BeNull();
        result.Error.Should().BeNull();
    }

    [Fact]
    public void InvokeResult_CanBeCreatedWithEmptyData()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("");

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().Be("");
    }

    [Fact]
    public void InvokeResult_CanBeCreatedWithWhitespaceData()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("   ");

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().Be("   ");
    }

    [Fact]
    public void InvokeResult_Failed_WithNullErrorMessage_IsAllowed()
    {
        // Arrange & Act
        var result = InvokeResult.Failed(null!);

        // Assert
        result.Success.Should().BeFalse();
        result.Error.Should().BeNull();
    }

    [Fact]
    public void InvokeResult_Failed_WithEmptyErrorMessage_IsAllowed()
    {
        // Arrange & Act
        var result = InvokeResult.Failed("");

        // Assert
        result.Success.Should().BeFalse();
        result.Error.Should().Be("");
    }

    [Fact]
    public void InvokeResult_DurationMs_CanBeZero()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("{}");

        // Assert
        result.Success.Should().BeTrue();
        result.DurationMs.Should().BeGreaterOrEqualTo(0);
    }

    [Fact]
    public void InvokeResult_MultipleInstances_AreIndependent()
    {
        // Arrange & Act
        var result1 = InvokeResult.Succeeded("{\"id\":1}");
        var result2 = InvokeResult.Failed("error");

        // Assert
        result1.Success.Should().BeTrue();
        result2.Success.Should().BeFalse();
        result1.Should().NotBeSameAs(result2);
    }

    #endregion

    #region FunctionContext Edge Cases

    [Fact]
    public void FunctionContext_DefaultValues_AreEmptyOrNull()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "test.function",
            CallId = "call-123",
            GameId = "game-123",
            Env = "dev"
        };

        // Assert
        context.FunctionId.Should().Be("test.function");
        context.CallId.Should().Be("call-123");
        context.GameId.Should().Be("game-123");
        context.Env.Should().Be("dev");
        context.UserId.Should().BeNull();
    }

    [Fact]
    public void FunctionContext_CanSetAllProperties()
    {
        // Arrange & Act
        var timestamp = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
        var context = new FunctionContext
        {
            FunctionId = "test.function",
            CallId = Guid.NewGuid().ToString(),
            GameId = "game-123",
            Env = "production",
            UserId = "user-456",
            Timestamp = timestamp
        };

        // Assert
        context.FunctionId.Should().Be("test.function");
        context.CallId.Should().NotBeNullOrEmpty();
        context.GameId.Should().Be("game-123");
        context.Env.Should().Be("production");
        context.UserId.Should().Be("user-456");
        context.Timestamp.Should().Be(timestamp);
    }

    [Fact]
    public void FunctionContext_Timestamp_DefaultsToMinValue()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "test.function",
            CallId = "call-123",
            GameId = "game-123",
            Env = "dev"
        };

        // Assert
        context.Timestamp.Should().Be(0);
    }

    #endregion

    #region BatchInvokeRequest Edge Cases

    [Fact]
    public void BatchInvokeRequest_CanBeCreatedWithMinimalData()
    {
        // Arrange & Act
        var request = new BatchInvokeRequest
        {
            FunctionId = "test.func",
            Payload = "{}"
        };

        // Assert
        request.FunctionId.Should().Be("test.func");
        request.Payload.Should().Be("{}");
    }

    [Fact]
    public void BatchInvokeRequest_EmptyPayload_IsAllowed()
    {
        // Arrange & Act
        var request = new BatchInvokeRequest
        {
            FunctionId = "test.func",
            Payload = ""
        };

        // Assert
        request.Payload.Should().Be("");
    }

    [Fact]
    public void BatchInvokeRequest_NullIdempotencyKey_IsAllowed()
    {
        // Arrange & Act
        var request = new BatchInvokeRequest
        {
            FunctionId = "test.func",
            Payload = "{}",
            IdempotencyKey = null
        };

        // Assert
        request.IdempotencyKey.Should().BeNull();
    }

    [Fact]
    public void BatchInvokeRequest_CanHaveIdempotencyKey()
    {
        // Arrange & Act
        var request = new BatchInvokeRequest
        {
            FunctionId = "test.func",
            Payload = "{}",
            IdempotencyKey = "idemp-123"
        };

        // Assert
        request.IdempotencyKey.Should().Be("idemp-123");
    }

    #endregion

    #region JobStatus Edge Cases

    [Fact]
    public void JobStatus_DefaultValues_AreCorrect()
    {
        // Arrange & Act
        var status = new JobStatus
        {
            JobId = "job-123",
            Status = "pending"
        };

        // Assert
        status.JobId.Should().Be("job-123");
        status.Status.Should().Be("pending");
        status.Progress.Should().Be(0);
        status.Error.Should().BeNull();
        status.Result.Should().BeNull();
    }

    [Fact]
    public void JobStatus_Progress_CanBeZero()
    {
        // Arrange & Act
        var status = new JobStatus
        {
            JobId = "job-123",
            Status = "pending",
            Progress = 0
        };

        // Assert
        status.Progress.Should().Be(0);
    }

    [Fact]
    public void JobStatus_Progress_CanBeHundred()
    {
        // Arrange & Act
        var status = new JobStatus
        {
            JobId = "job-123",
            Status = "completed",
            Progress = 1.0 // 100%
        };

        // Assert
        status.Progress.Should().Be(1.0);
    }

    [Fact]
    public void JobStatus_Progress_CanBePartial()
    {
        // Arrange & Act
        var status = new JobStatus
        {
            JobId = "job-123",
            Status = "running",
            Progress = 0.5 // 50%
        };

        // Assert
        status.Progress.Should().Be(0.5);
    }

    [Fact]
    public void JobStatus_AllFields_CanBeSet()
    {
        // Arrange & Act
        var status = new JobStatus
        {
            JobId = "job-123",
            Status = "running",
            Progress = 0.75,
            Error = "Partial failure",
            Result = "{\"partial\":\"result\"}"
        };

        // Assert
        status.JobId.Should().Be("job-123");
        status.Status.Should().Be("running");
        status.Progress.Should().Be(0.75);
        status.Error.Should().Be("Partial failure");
        status.Result.Should().Be("{\"partial\":\"result\"}");
    }

    #endregion

    #region FunctionDescriptor Edge Cases

    [Fact]
    public void FunctionDescriptor_MinimalValidDescriptor()
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor
        {
            Id = "test.function",
            Version = "1.0.0",
            Category = "test",
            Risk = "low"
        };

        // Assert
        descriptor.Id.Should().Be("test.function");
        descriptor.Version.Should().Be("1.0.0");
        descriptor.Category.Should().Be("test");
        descriptor.Risk.Should().Be("low");
    }

    [Fact]
    public void FunctionDescriptor_AllOptionalFields_CanBeNull()
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor
        {
            Id = "test.function",
            Version = "1.0.0",
            Category = "test",
            Risk = "low",
            Description = null,
            InputSchema = null,
            OutputSchema = null,
            Tags = null
        };

        // Assert
        descriptor.Description.Should().BeNull();
        descriptor.InputSchema.Should().BeNull();
        descriptor.OutputSchema.Should().BeNull();
        descriptor.Tags.Should().BeNull();
    }

    [Fact]
    public void FunctionDescriptor_Version_CanBeComplex()
    {
        // Arrange
        var versions = new[] { "1.0.0", "2.5.3", "10.20.30", "0.0.1", "3.0.0-beta" };

        foreach (var version in versions)
        {
            // Act
            var descriptor = new FunctionDescriptor
            {
                Id = "test.function",
                Version = version,
                Category = "test",
                Risk = "low"
            };

            // Assert
            descriptor.Version.Should().Be(version);
        }
    }

    #endregion

    #region Empty and Null Collection Tests

    [Fact]
    public void InvokeOptions_Metadata_CanHandleNullKeysAndValues()
    {
        // Arrange & Act
        var options = new InvokeOptions
        {
            Metadata = new Dictionary<string, string>
            {
                ["NullKey"] = null!,
                ["EmptyKey"] = ""
            }
        };

        // Assert
        options.Metadata.Should().HaveCount(2);
        options.Metadata["NullKey"].Should().BeNull();
        options.Metadata["EmptyKey"].Should().Be("");
    }

    [Fact]
    public void FunctionContext_CanHaveIdempotencyKey()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "test.func",
            CallId = "call-123",
            GameId = "game-123",
            Env = "dev",
            IdempotencyKey = "idemp-456"
        };

        // Assert
        context.IdempotencyKey.Should().Be("idemp-456");
    }

    #endregion

    #region String Edge Cases

    [Theory]
    [InlineData("")]
    [InlineData(" ")]
    [InlineData("  ")]
    [InlineData("\t")]
    [InlineData("\n")]
    public void InvokeOptions_GameId_AcceptsWhitespaceStrings(string gameId)
    {
        // Arrange & Act
        var options = new InvokeOptions { GameId = gameId };

        // Assert
        options.GameId.Should().Be(gameId);
    }

    [Theory]
    [InlineData("a")]
    [InlineData("abc")]
    [InlineData("very.long.function.name.with.many.dots")]
    [InlineData("function-with-hyphens")]
    [InlineData("function_with_underscores")]
    public void FunctionDescriptor_Id_AcceptsVariousFormats(string id)
    {
        // Arrange & Act
        var descriptor = new FunctionDescriptor
        {
            Id = id,
            Version = "1.0.0",
            Category = "test",
            Risk = "low"
        };

        // Assert
        descriptor.Id.Should().Be(id);
    }

    #endregion
}
