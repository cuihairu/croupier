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
using Google.Protobuf;
using Google.Protobuf.WellKnownTypes;
using Xunit;
using Croupier.Sdk.V1;

namespace Croupier.Sdk.Tests.V1;

/// <summary>
/// Tests for ProtobufExtensions
/// </summary>
public class ProtobufExtensionsTests
{
    /// <summary>
    /// Helper to call our extension method explicitly to avoid conflict with Google.Protobuf.MessageExtensions
    /// </summary>
    private static byte[] ToByteArrayExt(IMessage message)
    {
        using var ms = new System.IO.MemoryStream();
        message.WriteTo(ms);
        return ms.ToArray();
    }

    #region ToByteArray Tests

    [Fact]
    public void ToByteArray_WithEmptyMessage_ReturnsValidByteArray()
    {
        // Arrange
        var message = new Empty();

        // Act
        var result = ToByteArrayExt(message);

        // Assert
        Assert.NotNull(result);
        Assert.Empty(result);
    }

    [Fact]
    public void ToByteArray_WithStringMessage_ReturnsValidByteArray()
    {
        // Arrange
        var message = new StringValue { Value = "test" };

        // Act
        var result = ToByteArrayExt(message);

        // Assert
        Assert.NotNull(result);
        Assert.NotEmpty(result);
        // Should be able to parse back
        var parsed = StringValue.Parser.ParseFrom(result);
        Assert.Equal("test", parsed.Value);
    }

    [Fact]
    public void ToByteArray_WithIntMessage_ReturnsValidByteArray()
    {
        // Arrange
        var message = new Int32Value { Value = 42 };

        // Act
        var result = ToByteArrayExt(message);

        // Assert
        Assert.NotNull(result);
        Assert.NotEmpty(result);
        var parsed = Int32Value.Parser.ParseFrom(result);
        Assert.Equal(42, parsed.Value);
    }

    [Fact]
    public void ToByteArray_WithBoolMessage_ReturnsValidByteArray()
    {
        // Arrange
        var message = new BoolValue { Value = true };

        // Act
        var result = ToByteArrayExt(message);

        // Assert
        Assert.NotNull(result);
        Assert.NotEmpty(result);
        var parsed = BoolValue.Parser.ParseFrom(result);
        Assert.True(parsed.Value);
    }

    [Fact]
    public void ToByteArray_RoundTrip_PreservesData()
    {
        // Arrange
        var original = new StringValue { Value = "Hello, World!" };

        // Act
        var bytes = ToByteArrayExt(original);
        var restored = StringValue.Parser.ParseFrom(bytes);

        // Assert
        Assert.Equal(original.Value, restored.Value);
    }

    [Fact]
    public void ToByteArray_WithSpecialCharacters_PreservesData()
    {
        // Arrange
        var testString = "测试\n\t\r\\\"'";
        var message = new StringValue { Value = testString };

        // Act
        var result = ToByteArrayExt(message);
        var parsed = StringValue.Parser.ParseFrom(result);

        // Assert
        Assert.Equal(testString, parsed.Value);
    }

    [Fact]
    public void ToByteArray_WithEmptyString_ReturnsValidByteArray()
    {
        // Arrange
        var message = new StringValue { Value = "" };

        // Act
        var result = ToByteArrayExt(message);

        // Assert
        Assert.NotNull(result);
        var parsed = StringValue.Parser.ParseFrom(result);
        Assert.Equal("", parsed.Value);
    }

    [Fact]
    public void ToByteArray_WithLargeString_ReturnsValidByteArray()
    {
        // Arrange
        var largeString = new string('A', 10000);
        var message = new StringValue { Value = largeString };

        // Act
        var result = ToByteArrayExt(message);

        // Assert
        Assert.NotNull(result);
        Assert.True(result.Length > 0);
        var parsed = StringValue.Parser.ParseFrom(result);
        Assert.Equal(largeString, parsed.Value);
    }

    [Fact]
    public void ToByteArray_WithUnicodeString_PreservesData()
    {
        // Arrange
        var unicodeString = "Hello 🌍 世界 🚀";
        var message = new StringValue { Value = unicodeString };

        // Act
        var result = ToByteArrayExt(message);
        var parsed = StringValue.Parser.ParseFrom(result);

        // Assert
        Assert.Equal(unicodeString, parsed.Value);
    }

    [Fact]
    public void ToByteArray_WithInt64Value_ReturnsValidByteArray()
    {
        // Arrange
        var message = new Int64Value { Value = long.MaxValue };

        // Act
        var result = ToByteArrayExt(message);

        // Assert
        Assert.NotNull(result);
        var parsed = Int64Value.Parser.ParseFrom(result);
        Assert.Equal(long.MaxValue, parsed.Value);
    }

    [Fact]
    public void ToByteArray_WithDoubleValue_ReturnsValidByteArray()
    {
        // Arrange
        var message = new DoubleValue { Value = 3.14159 };

        // Act
        var result = ToByteArrayExt(message);

        // Assert
        Assert.NotNull(result);
        var parsed = DoubleValue.Parser.ParseFrom(result);
        Assert.Equal(3.14159, parsed.Value, 5);
    }

    [Fact]
    public void ToByteArray_MultipleCalls_ReturnsIndependentArrays()
    {
        // Arrange
        var message = new StringValue { Value = "test" };

        // Act
        var result1 = ToByteArrayExt(message);
        var result2 = ToByteArrayExt(message);

        // Assert
        Assert.NotSame(result1, result2);
        Assert.Equal(result1, result2);
    }

    #endregion
}
