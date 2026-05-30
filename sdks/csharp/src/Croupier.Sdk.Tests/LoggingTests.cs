// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Logging;
using FluentAssertions;
using Microsoft.Extensions.Logging;
using Moq;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for logging implementations
/// </summary>
public class LoggingTests
{
    #region ConsoleCroupierLogger Tests

    [Fact]
    public void ConsoleCroupierLogger_CanBeCreated()
    {
        // Act
        var logger = new ConsoleCroupierLogger();

        // Assert
        logger.Should().NotBeNull();
    }

    [Fact]
    public void ConsoleCroupierLogger_ImplementsICroupierLogger()
    {
        // Assert
        typeof(ConsoleCroupierLogger).Should().Implement<ICroupierLogger>();
    }

    [Fact]
    public void ConsoleCroupierLogger_LogMethods_DoNotThrow()
    {
        // Arrange
        var logger = new ConsoleCroupierLogger();

        // Act & Assert
        var actions = new Action[]
        {
            () => logger.LogDebug("TestComponent", "Debug message"),
            () => logger.LogInfo("TestComponent", "Info message"),
            () => logger.LogWarning("TestComponent", "Warning message"),
            () => logger.LogError("TestComponent", "Error message"),
            () => logger.LogError("TestComponent", "Error with exception", new Exception("Test"))
        };

        foreach (var action in actions)
        {
            action.Should().NotThrow();
        }
    }

    #endregion

    #region CroupierLogger Tests

    [Fact]
    public void CroupierLogger_CanBeCreatedWithMicrosoftLogger()
    {
        // Arrange
        var mockLogger = new Mock<ILogger<CroupierLogger>>();

        // Act
        var logger = new CroupierLogger(mockLogger.Object);

        // Assert
        logger.Should().NotBeNull();
    }

    [Fact]
    public void CroupierLogger_ImplementsICroupierLogger()
    {
        // Assert
        typeof(CroupierLogger).Should().Implement<ICroupierLogger>();
    }

    [Fact]
    public void CroupierLogger_LogDebug_CallsUnderlyingLogger()
    {
        // Arrange
        var mockLogger = new Mock<ILogger<CroupierLogger>>();
        var logger = new CroupierLogger(mockLogger.Object);

        // Act
        logger.LogDebug("TestComponent", "Debug message");

        // Assert
        mockLogger.Verify(
            x => x.Log(
                LogLevel.Debug,
                It.IsAny<EventId>(),
                It.IsAny<It.IsAnyType>(),
                It.IsAny<Exception>(),
                It.IsAny<Func<It.IsAnyType, Exception?, string>>()),
            Times.Once);
    }

    [Fact]
    public void CroupierLogger_LogInfo_CallsUnderlyingLogger()
    {
        // Arrange
        var mockLogger = new Mock<ILogger<CroupierLogger>>();
        var logger = new CroupierLogger(mockLogger.Object);

        // Act
        logger.LogInfo("TestComponent", "Info message");

        // Assert
        mockLogger.Verify(
            x => x.Log(
                LogLevel.Information,
                It.IsAny<EventId>(),
                It.IsAny<It.IsAnyType>(),
                It.IsAny<Exception>(),
                It.IsAny<Func<It.IsAnyType, Exception?, string>>()),
            Times.Once);
    }

    [Fact]
    public void CroupierLogger_LogWarning_CallsUnderlyingLogger()
    {
        // Arrange
        var mockLogger = new Mock<ILogger<CroupierLogger>>();
        var logger = new CroupierLogger(mockLogger.Object);

        // Act
        logger.LogWarning("TestComponent", "Warning message");

        // Assert
        mockLogger.Verify(
            x => x.Log(
                LogLevel.Warning,
                It.IsAny<EventId>(),
                It.IsAny<It.IsAnyType>(),
                It.IsAny<Exception>(),
                It.IsAny<Func<It.IsAnyType, Exception?, string>>()),
            Times.Once);
    }

    [Fact]
    public void CroupierLogger_LogError_CallsUnderlyingLogger()
    {
        // Arrange
        var mockLogger = new Mock<ILogger<CroupierLogger>>();
        var logger = new CroupierLogger(mockLogger.Object);

        // Act
        logger.LogError("TestComponent", "Error message");

        // Assert
        mockLogger.Verify(
            x => x.Log(
                LogLevel.Error,
                It.IsAny<EventId>(),
                It.IsAny<It.IsAnyType>(),
                It.IsAny<Exception>(),
                It.IsAny<Func<It.IsAnyType, Exception?, string>>()),
            Times.Once);
    }

    [Fact]
    public void CroupierLogger_LogErrorWithException_IncludesException()
    {
        // Arrange
        var mockLogger = new Mock<ILogger<CroupierLogger>>();
        var logger = new CroupierLogger(mockLogger.Object);
        var exception = new InvalidOperationException("Test exception");

        // Act
        logger.LogError("TestComponent", "Error with exception", exception);

        // Assert
        mockLogger.Verify(
            x => x.Log(
                LogLevel.Error,
                It.IsAny<EventId>(),
                It.IsAny<It.IsAnyType>(),
                exception,
                It.IsAny<Func<It.IsAnyType, Exception?, string>>()),
            Times.Once);
    }

    #endregion

    #region ICroupierLogger Interface Tests

    [Fact]
    public void ICroupierLogger_MockedImplementation()
    {
        // Arrange
        var mockLogger = new Mock<ICroupierLogger>();

        // Act
        mockLogger.Object.LogDebug("Component", "Message");
        mockLogger.Object.LogInfo("Component", "Message");
        mockLogger.Object.LogWarning("Component", "Message");
        mockLogger.Object.LogError("Component", "Message");
        mockLogger.Object.LogError("Component", "Message", new Exception());

        // Assert
        mockLogger.Verify(l => l.LogDebug("Component", "Message"), Times.Once);
        mockLogger.Verify(l => l.LogInfo("Component", "Message"), Times.Once);
        mockLogger.Verify(l => l.LogWarning("Component", "Message"), Times.Once);
        mockLogger.Verify(l => l.LogError("Component", "Message", null), Times.Once);
        mockLogger.Verify(l => l.LogError("Component", "Message", It.IsAny<Exception>()), Times.Exactly(2));
    }

    #endregion
}
