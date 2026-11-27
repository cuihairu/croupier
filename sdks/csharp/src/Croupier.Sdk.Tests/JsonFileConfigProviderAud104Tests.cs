// AUD-104: JsonFileConfigProvider must parse files as standard JSON. The old
// per-property regex truncated string values at the first comma, failed on
// nested objects such as Headers, ignored escapes, and normalized type errors
// silently instead of pointing at the offending line.

using Croupier.Sdk.Configuration;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

public class JsonFileConfigProviderAud104Tests
{
    private static string WriteTempConfig(string json)
    {
        var path = Path.Combine(Path.GetTempPath(), $"croupier-aud104-{Guid.NewGuid():N}.json");
        File.WriteAllText(path, json);
        return path;
    }

    [Fact]
    public void StringValuesWithCommasSurvive()
    {
        var path = WriteTempConfig(
            """{"ServerName": "croupier.internal,alt.internal", "AgentAddr": "10.0.0.1:19091,10.0.0.2:19091"}""");
        try
        {
            var config = new JsonFileConfigProvider(path).GetConfig();

            config.ServerName.Should().Be("croupier.internal,alt.internal");
            config.AgentAddr.Should().Be("10.0.0.1:19091,10.0.0.2:19091");
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void HeadersObjectParsesIntoDictionary()
    {
        var path = WriteTempConfig("""
            {
              "ServiceId": "headers-svc",
              "Headers": {
                "X-Game-Version": "1.0.0",
                "X-Trace-Id": "t,1"
              }
            }
            """);
        try
        {
            var config = new JsonFileConfigProvider(path).GetConfig();

            config.Headers.Should().HaveCount(2);
            config.Headers["X-Game-Version"].Should().Be("1.0.0");
            config.Headers["X-Trace-Id"].Should().Be("t,1");
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void EscapesAndEmptyStringsArePreserved()
    {
        var path = WriteTempConfig(
            """{"ServiceId": "quote\"and\\slash", "ServerName": "", "AuthToken": "Bearer a中b"}""");
        try
        {
            var config = new JsonFileConfigProvider(path).GetConfig();

            config.ServiceId.Should().Be("quote\"and\\slash");
            config.ServerName.Should().BeEmpty();
            config.AuthToken.Should().Be("Bearer a中b");
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void ExplicitFalseBooleansOverrideDefaults()
    {
        var path = WriteTempConfig("""
            {"Insecure": false, "AutoReconnect": false, "DisableLogging": true}
            """);
        try
        {
            var config = new JsonFileConfigProvider(path).GetConfig();

            config.Insecure.Should().BeFalse();
            config.AutoReconnect.Should().BeFalse();
            config.DisableLogging.Should().BeTrue();
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void ExplicitZeroRetryCountSurvives()
    {
        var path = WriteTempConfig("""{"ReconnectMaxAttempts": 0, "TimeoutSeconds": 17}""");
        try
        {
            var config = new JsonFileConfigProvider(path).GetConfig();

            config.ReconnectMaxAttempts.Should().Be(0);
            config.TimeoutSeconds.Should().Be(17);
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void CamelCaseNamesAreAccepted()
    {
        // 超集兼容：PascalCase 依旧支持，camelCase 也不拒绝。
        var path = WriteTempConfig(
            """{"agentAddr": "camel-host:2000", "serviceId": "camel-svc", "timeoutSeconds": 45}""");
        try
        {
            var config = new JsonFileConfigProvider(path).GetConfig();

            config.AgentAddr.Should().Be("camel-host:2000");
            config.ServiceId.Should().Be("camel-svc");
            config.TimeoutSeconds.Should().Be(45);
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public void SyntaxErrorsPointAtTheOffendingLine()
    {
        // STJ 的行号从 0 计数，这里不断言具体行数，只断言消息带文件路径与行列定位。
        var path = WriteTempConfig("""
            {"ServiceId": "svc",
             "AgentAddr": "broken
            """);
        try
        {
            var action = () => new JsonFileConfigProvider(path).GetConfig();

            action.Should().Throw<FormatException>().Which.Message.Should()
                .Contain(path).And.Contain("line").And.Contain("position");
        }
        finally
        {
            File.Delete(path);
        }
    }
}
