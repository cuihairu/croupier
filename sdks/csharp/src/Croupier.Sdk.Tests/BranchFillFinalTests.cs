// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Configuration;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// 覆盖率补测（末轮）：EnvironmentConfigProvider 布尔/整数解析的剩余分支
/// （解析失败回退默认值、大小写不敏感真值、非 0/1 数字串回退假值）。
/// </summary>
public sealed class BranchFillFinalTests
{
    [Fact]
    public void EnvironmentConfigProvider_BoolAndIntParsingEdges()
    {
        // 独占前缀，避免与其他测试类的环境变量读写竞态。
        const string prefix = "BRANCHFILL_";
        Environment.SetEnvironmentVariable(prefix + "TIMEOUT_SECONDS", "not-a-number");
        Environment.SetEnvironmentVariable(prefix + "HEARTBEAT_INTERVAL_SECONDS", "12.5");
        Environment.SetEnvironmentVariable(prefix + "INSECURE", "TRUE");
        Environment.SetEnvironmentVariable(prefix + "AUTO_RECONNECT", "0");
        Environment.SetEnvironmentVariable(prefix + "DISABLE_LOGGING", "Yes");

        try
        {
            var config = new EnvironmentConfigProvider(prefix).GetConfig();

            // 整数解析失败（非数字 / 带小数点）回退默认值
            config.TimeoutSeconds.Should().Be(30);
            config.HeartbeatIntervalSeconds.Should().Be(60);

            // 布尔解析：大小写不敏感 true；非 true/1 的一律视为 false
            config.Insecure.Should().BeTrue();
            config.AutoReconnect.Should().BeFalse();
            config.DisableLogging.Should().BeFalse();
        }
        finally
        {
            Environment.SetEnvironmentVariable(prefix + "TIMEOUT_SECONDS", null);
            Environment.SetEnvironmentVariable(prefix + "HEARTBEAT_INTERVAL_SECONDS", null);
            Environment.SetEnvironmentVariable(prefix + "INSECURE", null);
            Environment.SetEnvironmentVariable(prefix + "AUTO_RECONNECT", null);
            Environment.SetEnvironmentVariable(prefix + "DISABLE_LOGGING", null);
        }
    }
}
