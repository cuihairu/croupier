using Croupier.Sdk.Models;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// 单线程游戏服兼容：MaxConcurrentCalls=1（串行模式）配置面校验。
/// </summary>
public class MaxConcurrentCallsTests
{
    [Fact]
    public void DefaultIsUnlimited()
    {
        var config = new ClientConfig();
        Assert.Equal(0, config.MaxConcurrentCalls);
    }

    [Fact]
    public void SerialModeIsConfigurable()
    {
        var config = new ClientConfig { MaxConcurrentCalls = 1 };
        Assert.Equal(1, config.MaxConcurrentCalls);
    }
}
