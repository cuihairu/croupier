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

using Croupier.Sdk;
using Croupier.Sdk.Configuration;
using Croupier.Sdk.Models;

// 简单的服务示例：注册函数并处理调用

// 从环境变量加载配置
var configProvider = new EnvironmentConfigProvider();
var config = configProvider.GetConfig();

Console.WriteLine("========================================");
Console.WriteLine("Croupier C# SDK - Simple Service Example");
Console.WriteLine("========================================");
Console.WriteLine($"Service ID: {config.ServiceId}");
Console.WriteLine($"Game ID: {config.GameId}");
Console.WriteLine($"Environment: {config.Env}");
Console.WriteLine($"Agent Address: {config.AgentAddr}");
Console.WriteLine();

// 创建客户端
using var client = new CroupierClient(config);

// 注册函数处理器
Console.WriteLine("Registering functions...");

// 玩家信息查询
var playerGetDescriptor = new FunctionDescriptor
{
    Id = "player.get",
    Version = "1.0.0",
    Category = "player",
    Risk = "low",
    Entity = "player",
    Operation = "get",
    DisplayName = "获取玩家信息",
    Description = "根据玩家 ID 获取玩家详细信息"
};

client.RegisterFunction(playerGetDescriptor, async (context, payload) =>
{
    Console.WriteLine($"[player.get] Call: {context.CallId}, Payload: {payload}");

    // 解析请求（简化版，实际应使用 JSON 库）
    // payload: {"player_id": "12345"}

    // 模拟处理
    await Task.Delay(10);

    var response = System.Text.Json.JsonSerializer.Serialize(new
    {
        status = "success",
        player = new
        {
            id = "12345",
            name = "TestPlayer",
            level = 50,
            exp = 125000,
            vip_level = 3,
            last_login = DateTime.UtcNow.ToString("o")
        }
    });

    return response;
});

Console.WriteLine("  ✓ player.get");

// 玩家封禁
var playerBanDescriptor = new FunctionDescriptor
{
    Id = "player.ban",
    Version = "1.0.0",
    Category = "moderation",
    Risk = "high",
    Entity = "player",
    Operation = "ban",
    DisplayName = "封禁玩家",
    Description = "封禁指定玩家账号"
};

client.RegisterFunction(playerBanDescriptor, async (context, payload) =>
{
    Console.WriteLine($"[player.ban] Call: {context.CallId}, Payload: {payload}, User: {context.UserId}");

    // 模拟处理
    await Task.Delay(20);

    var response = System.Text.Json.JsonSerializer.Serialize(new
    {
        status = "success",
        action = "ban",
        timestamp = DateTime.UtcNow.ToString("o"),
        moderator = context.UserId
    });

    return response;
});

Console.WriteLine("  ✓ player.ban");

// 钱包转账
var walletTransferDescriptor = new FunctionDescriptor
{
    Id = "wallet.transfer",
    Version = "1.0.0",
    Category = "economy",
    Risk = "high",
    Entity = "wallet",
    Operation = "transfer",
    DisplayName = "钱包转账",
    Description = "在玩家之间转移游戏货币"
};

client.RegisterFunction(walletTransferDescriptor, async (context, payload) =>
{
    Console.WriteLine($"[wallet.transfer] Call: {context.CallId}");

    // 模拟处理
    await Task.Delay(30);

    var response = System.Text.Json.JsonSerializer.Serialize(new
    {
        status = "success",
        transaction_id = $"tx_{DateTime.UtcNow.Ticks}",
        from = "player_123",
        to = "player_456",
        amount = 100,
        currency = "gold"
    });

    return response;
});

Console.WriteLine("  ✓ wallet.transfer");
Console.WriteLine();

// 连接到 Agent
Console.WriteLine("Connecting to Agent...");
try
{
    await client.ConnectAsync();
    Console.WriteLine($"  ✓ Connected. Local: {client.LocalAddress}");
}
catch (Exception ex)
{
    Console.WriteLine($"  ✗ Connection failed: {ex.Message}");
    Console.WriteLine();
    Console.WriteLine("Make sure Croupier Agent is running at {0}", config.AgentAddr);
    return 1;
}

Console.WriteLine();
Console.WriteLine("========================================");
Console.WriteLine("Service started. Press Ctrl+C to exit.");
Console.WriteLine("========================================");
Console.WriteLine();

// 启动服务
var cts = new CancellationTokenSource();
Console.CancelKeyPress += (sender, e) =>
{
    Console.WriteLine();
    Console.WriteLine("Shutting down...");
    e.Cancel = true;
    cts.Cancel();
};

try
{
    await client.ServeAsync(cts.Token);
}
catch (OperationCanceledException)
{
    // Expected
}
finally
{
    client.Stop();
    client.Disconnect();
}

Console.WriteLine("Service stopped.");
return 0;
