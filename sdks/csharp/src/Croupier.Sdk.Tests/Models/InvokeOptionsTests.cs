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

using System;
using System.Collections.Generic;
using Xunit;
using FluentAssertions;
using Croupier.Sdk.Models;

namespace Croupier.Sdk.Tests.Models;

/// <summary>
/// Tests for InvokeOptions, FunctionContext, and InvokeResult
/// </summary>
public class InvokeOptionsTests
{
    #region InvokeOptions Tests

    [Fact]
    public void InvokeOptions_DefaultValues_AreCorrect()
    {
        // Arrange & Act
        var options = new InvokeOptions();

        // Assert
        options.GameId.Should().BeNull();
        options.Env.Should().BeNull();
        options.TimeoutSeconds.Should().Be(30);
        options.IdempotencyKey.Should().BeNull();
        options.RequestId.Should().BeNull();
        options.UserId.Should().BeNull();
        options.Metadata.Should().BeNull();
    }

    [Fact]
    public void InvokeOptions_CanSetGameId()
    {
        // Arrange & Act
        var options = new InvokeOptions { GameId = "game1" };

        // Assert
        options.GameId.Should().Be("game1");
    }

    [Fact]
    public void InvokeOptions_CanSetEnv()
    {
        // Arrange & Act
        var options = new InvokeOptions { Env = "production" };

        // Assert
        options.Env.Should().Be("production");
    }

    [Fact]
    public void InvokeOptions_CanSetTimeoutSeconds()
    {
        // Arrange & Act
        var options = new InvokeOptions { TimeoutSeconds = 60 };

        // Assert
        options.TimeoutSeconds.Should().Be(60);
    }

    [Fact]
    public void InvokeOptions_CanSetIdempotencyKey()
    {
        // Arrange & Act
        var options = new InvokeOptions { IdempotencyKey = "key-123" };

        // Assert
        options.IdempotencyKey.Should().Be("key-123");
    }

    [Fact]
    public void InvokeOptions_CanSetRequestId()
    {
        // Arrange & Act
        var options = new InvokeOptions { RequestId = "req-456" };

        // Assert
        options.RequestId.Should().Be("req-456");
    }

    [Fact]
    public void InvokeOptions_CanSetUserId()
    {
        // Arrange & Act
        var options = new InvokeOptions { UserId = "user-789" };

        // Assert
        options.UserId.Should().Be("user-789");
    }

    [Fact]
    public void InvokeOptions_CanSetMetadata()
    {
        // Arrange
        var metadata = new Dictionary<string, string>
        {
            ["key1"] = "value1",
            ["key2"] = "value2"
        };

        // Act
        var options = new InvokeOptions { Metadata = metadata };

        // Assert
        options.Metadata.Should().NotBeNull();
        options.Metadata.Should().HaveCount(2);
        options.Metadata["key1"].Should().Be("value1");
        options.Metadata["key2"].Should().Be("value2");
    }

    [Fact]
    public void InvokeOptions_CanSetAllProperties()
    {
        // Arrange
        var metadata = new Dictionary<string, string> { ["trace"] = "true" };

        // Act
        var options = new InvokeOptions
        {
            GameId = "game1",
            Env = "dev",
            TimeoutSeconds = 120,
            IdempotencyKey = "idemp-key",
            RequestId = "req-id",
            UserId = "user-id",
            Metadata = metadata
        };

        // Assert
        options.GameId.Should().Be("game1");
        options.Env.Should().Be("dev");
        options.TimeoutSeconds.Should().Be(120);
        options.IdempotencyKey.Should().Be("idemp-key");
        options.RequestId.Should().Be("req-id");
        options.UserId.Should().Be("user-id");
        options.Metadata.Should().HaveCount(1);
        options.Metadata["trace"].Should().Be("true");
    }

    [Fact]
    public void InvokeOptions_WithEmptyStringProperties_WorksCorrectly()
    {
        // Arrange & Act
        var options = new InvokeOptions
        {
            GameId = "",
            Env = "",
            IdempotencyKey = "",
            RequestId = "",
            UserId = ""
        };

        // Assert
        options.GameId.Should().Be("");
        options.Env.Should().Be("");
        options.IdempotencyKey.Should().Be("");
        options.RequestId.Should().Be("");
        options.UserId.Should().Be("");
    }

    [Fact]
    public void InvokeOptions_Metadata_IsMutable()
    {
        // Arrange
        var options = new InvokeOptions
        {
            Metadata = new Dictionary<string, string>()
        };

        // Act
        options.Metadata["newKey"] = "newValue";

        // Assert
        options.Metadata.Should().HaveCount(1);
        options.Metadata["newKey"].Should().Be("newValue");
    }

    [Fact]
    public void InvokeOptions_TimeoutZero_IsAllowed()
    {
        // Arrange & Act
        var options = new InvokeOptions { TimeoutSeconds = 0 };

        // Assert
        options.TimeoutSeconds.Should().Be(0);
    }

    [Fact]
    public void InvokeOptions_NegativeTimeout_IsAllowed()
    {
        // Arrange & Act
        var options = new InvokeOptions { TimeoutSeconds = -1 };

        // Assert
        options.TimeoutSeconds.Should().Be(-1);
    }

    #endregion

    #region FunctionContext Tests

    [Fact]
    public void FunctionContext_WithRequiredProperties_Works()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "func1",
            CallId = "call1",
            GameId = "game1",
            Env = "prod",
            Timestamp = 1234567890
        };

        // Assert
        context.FunctionId.Should().Be("func1");
        context.CallId.Should().Be("call1");
        context.GameId.Should().Be("game1");
        context.Env.Should().Be("prod");
        context.Timestamp.Should().Be(1234567890);
    }

    [Fact]
    public void FunctionContext_CanSetUserId()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "func1",
            CallId = "call1",
            GameId = "game1",
            Env = "prod",
            Timestamp = 0,
            UserId = "user123"
        };

        // Assert
        context.UserId.Should().Be("user123");
    }

    [Fact]
    public void FunctionContext_CanSetIdempotencyKey()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "func1",
            CallId = "call1",
            GameId = "game1",
            Env = "prod",
            Timestamp = 0,
            IdempotencyKey = "idemp-key"
        };

        // Assert
        context.IdempotencyKey.Should().Be("idemp-key");
    }

    [Fact]
    public void FunctionContext_CanSetCallerServiceId()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "func1",
            CallId = "call1",
            GameId = "game1",
            Env = "prod",
            Timestamp = 0,
            CallerServiceId = "service123"
        };

        // Assert
        context.CallerServiceId.Should().Be("service123");
    }

    [Fact]
    public void FunctionContext_WithAllProperties_Works()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "player.create",
            CallId = "call-uuid-123",
            GameId = "mygame",
            Env = "staging",
            UserId = "player-456",
            Timestamp = 1704067200000,
            IdempotencyKey = "idemp-789",
            CallerServiceId = "api-server"
        };

        // Assert
        context.FunctionId.Should().Be("player.create");
        context.CallId.Should().Be("call-uuid-123");
        context.GameId.Should().Be("mygame");
        context.Env.Should().Be("staging");
        context.UserId.Should().Be("player-456");
        context.Timestamp.Should().Be(1704067200000);
        context.IdempotencyKey.Should().Be("idemp-789");
        context.CallerServiceId.Should().Be("api-server");
    }

    #endregion

    #region InvokeResult Tests

    [Fact]
    public void InvokeResult_Succeeded_CreatesSuccessResult()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("success data", 100);

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().Be("success data");
        result.Error.Should().BeNull();
        result.ErrorCode.Should().BeNull();
        result.DurationMs.Should().Be(100);
    }

    [Fact]
    public void InvokeResult_Succeeded_WithoutDuration_CreatesSuccessResult()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("data");

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().Be("data");
        result.DurationMs.Should().Be(0);
    }

    [Fact]
    public void InvokeResult_Failed_CreatesFailureResult()
    {
        // Arrange & Act
        var result = InvokeResult.Failed("error message", "ERR_CODE", 250);

        // Assert
        result.Success.Should().BeFalse();
        result.Error.Should().Be("error message");
        result.ErrorCode.Should().Be("ERR_CODE");
        result.Data.Should().BeNull();
        result.DurationMs.Should().Be(250);
    }

    [Fact]
    public void InvokeResult_Failed_WithoutErrorCode_CreatesFailureResult()
    {
        // Arrange & Act
        var result = InvokeResult.Failed("error");

        // Assert
        result.Success.Should().BeFalse();
        result.Error.Should().Be("error");
        result.ErrorCode.Should().BeNull();
        result.DurationMs.Should().Be(0);
    }

    [Fact]
    public void InvokeResult_Failed_WithoutDuration_CreatesFailureResult()
    {
        // Arrange & Act
        var result = InvokeResult.Failed("error", "ERR_500");

        // Assert
        result.Success.Should().BeFalse();
        result.Error.Should().Be("error");
        result.ErrorCode.Should().Be("ERR_500");
        result.DurationMs.Should().Be(0);
    }

    [Fact]
    public void InvokeResult_SuccessAndFailure_AreMutuallyExclusive()
    {
        // Arrange
        var success = InvokeResult.Succeeded("data");
        var failure = InvokeResult.Failed("error");

        // Assert
        success.Success.Should().BeTrue();
        failure.Success.Should().BeFalse();
    }

    [Fact]
    public void InvokeResult_WithEmptyData_Works()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("");

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().Be("");
    }

    [Fact]
    public void InvokeResult_WithLargeData_Works()
    {
        // Arrange
        var largeData = new string('X', 10000);

        // Act
        var result = InvokeResult.Succeeded(largeData);

        // Assert
        result.Success.Should().BeTrue();
        result.Data.Should().HaveLength(10000);
    }

    [Fact]
    public void InvokeResult_WithNullError_MakesNull()
    {
        // Arrange & Act
        var result = InvokeResult.Failed(null!);

        // Assert
        result.Error.Should().BeNull();
    }

    [Fact]
    public void InvokeResult_Duration_CanBeLarge()
    {
        // Arrange & Act
        var result = InvokeResult.Succeeded("data", 3600000); // 1 hour in ms

        // Assert
        result.DurationMs.Should().Be(3600000);
    }

    [Fact]
    public void InvokeResult_Duration_CanBeNegative()
    {
        // Arrange & Act
        var result = InvokeResult.Failed("error", durationMs: -1);

        // Assert
        result.DurationMs.Should().Be(-1);
    }

    #endregion

    #region Edge Cases Tests

    [Fact]
    public void InvokeOptions_NullMetadata_Works()
    {
        // Arrange & Act
        var options = new InvokeOptions { Metadata = null };

        // Assert
        options.Metadata.Should().BeNull();
    }

    [Fact]
    public void InvokeResult_WithSpecialCharacters_Works()
    {
        // Arrange
        var specialData = "{\"key\":\"value\",\"array\":[1,2,3]}";
        var specialError = "Error:测试\n\t";

        // Act
        var success = InvokeResult.Succeeded(specialData);
        var failure = InvokeResult.Failed(specialError);

        // Assert
        success.Data.Should().Be(specialData);
        failure.Error.Should().Be(specialError);
    }

    [Fact]
    public void FunctionContext_WithEmptyStrings_Works()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "",
            CallId = "",
            GameId = "",
            Env = "",
            Timestamp = 0
        };

        // Assert
        context.FunctionId.Should().Be("");
        context.CallId.Should().Be("");
        context.GameId.Should().Be("");
        context.Env.Should().Be("");
    }

    [Fact]
    public void FunctionContext_Timestamp_CanBeZero()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "func",
            CallId = "call",
            GameId = "game",
            Env = "env",
            Timestamp = 0
        };

        // Assert
        context.Timestamp.Should().Be(0);
    }

    [Fact]
    public void InvokeResult_MultipleInstances_AreIndependent()
    {
        // Arrange & Act
        var result1 = InvokeResult.Succeeded("data1");
        var result2 = InvokeResult.Succeeded("data2");

        // Assert
        result1.Data.Should().Be("data1");
        result2.Data.Should().Be("data2");
        result1.Should().NotBeSameAs(result2);
    }

    #endregion
}
