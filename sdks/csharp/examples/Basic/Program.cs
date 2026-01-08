// Example demonstrates basic usage of the Croupier C# SDK
using System;
using System.Text;
using System.Text.Json;
using System.Threading.Tasks;
using Croupier.SDK.Client;
using Croupier.SDK.Models;

namespace Croupier.Examples;

class Program
{
    static async Task Main(string[] args)
    {
        // Create client configuration
        var config = new ClientConfig
        {
            AgentAddr = "localhost:19090",
            GameId = "example-game",
            Env = "development",
            ServiceId = "csharp-service",
            ServiceVersion = "1.0.0",
            Insecure = true,
            DebugLogging = true
        };

        // Create client
        var client = new CroupierClient(config);

        // Register player ban function using OpenAPI 3.0 compatible fields
        client.RegisterOpenAPIFunction(
            id: "player.ban",
            summary: "Ban a player from the game",
            handler: async (context, payload) =>
            {
                Console.WriteLine($"🔨 Banning player with payload: {Encoding.UTF8.GetString(payload)}");

                // Simulate processing time
                await Task.Delay(100);

                var result = new
                {
                    status = "success",
                    action = "ban",
                    timestamp = DateTime.UtcNow.ToString("o"),
                    message = "Player banned successfully"
                };

                return JsonSerializer.SerializeToUtf8Bytes(result);
            },
            configure: descriptor =>
            {
                // OpenAPI 3.0 tags
                descriptor.Tags = new List<string> { "player", "moderation", "high-risk" };
                descriptor.Description = "Permanently bans a player account from accessing the game server.";
                descriptor.Deprecated = false;
                descriptor.OperationId = "banPlayer";

                // OpenAPI 3.0 requestBody
                descriptor.RequestBody = new RequestBodyDescriptor
                {
                    Description = "Player ban request with reason and duration",
                    Required = true,
                    Content = new Dictionary<string, string>
                    {
                        ["application/json"] = @"
                        {
                            ""type"": ""object"",
                            ""required"": [""player_id"", ""reason""],
                            ""properties"": {
                                ""player_id"": {
                                    ""type"": ""string"",
                                    ""description"": ""Unique player identifier""
                                },
                                ""reason"": {
                                    ""type"": ""string"",
                                    ""description"": ""Reason for the ban""
                                },
                                ""duration_hours"": {
                                    ""type"": ""integer"",
                                    ""description"": ""Ban duration in hours (0 = permanent)"",
                                    ""default"": 0
                                }
                            }
                        }"
                    }
                };

                // OpenAPI 3.0 response
                descriptor.Response = new ResponseDescriptor
                {
                    Description = "Ban operation result",
                    StatusCode = "200",
                    Content = new Dictionary<string, string>
                    {
                        ["application/json"] = @"
                        {
                            ""type"": ""object"",
                            ""properties"": {
                                ""status"": {""type"": ""string""},
                                ""player_id"": {""type"": ""string""},
                                ""banned_at"": {""type"": ""string"", ""format"": ""date-time""}
                            }
                        }"
                    }
                };

                // OpenAPI 3.0 externalDocs
                descriptor.ExternalDocs = new ExternalDocumentation
                {
                    Description = "Player moderation guide",
                    Url = "https://docs.example.com/moderation#banning"
                };

                // UI rendering extensions
                descriptor.DisplayName = new DisplayName
                {
                    Zh = "玩家封禁",
                    En = "Ban Player"
                };

                descriptor.Menu = new MenuConfig
                {
                    Section = "player",
                    Group = "moderation",
                    Icon = "StopOutlined",
                    Order = 1
                };
            }
        );

        // Register item create function with minimal descriptor
        client.RegisterOpenAPIFunction(
            id: "item.create",
            summary: "Create a new game item",
            handler: async (context, payload) =>
            {
                Console.WriteLine($"📦 Creating item with payload: {Encoding.UTF8.GetString(payload)}");

                var result = new
                {
                    status = "success",
                    action = "create",
                    item_id = $"item_{Guid.NewGuid()}",
                    timestamp = DateTime.UtcNow.ToString("o")
                };

                return JsonSerializer.SerializeToUtf8Bytes(result);
            },
            configure: descriptor =>
            {
                descriptor.Tags = new List<string> { "item", "inventory", "low-risk" };
                descriptor.Description = "Creates a new item in the player's inventory";
                descriptor.OperationId = "createItem";
            }
        );

        Console.WriteLine("✅ All functions registered successfully");

        // Connect to agent
        Console.WriteLine("Connecting to Croupier Agent...");
        await client.ConnectAsync();
        Console.WriteLine($"Connected! Local address: {client.LocalAddress}");

        // Keep running until Ctrl+C
        Console.WriteLine("Press Ctrl+C to exit...");
        var cts = new CancellationTokenSource();
        Console.CancelKeyPress += (s, e) =>
        {
            e.Cancel = true;
            cts.Cancel();
        };

        try
        {
            await Task.Delay(Timeout.InfiniteTimeSpan, cts.Token);
        }
        catch (OperationCanceledException)
        {
            Console.WriteLine("\nReceived shutdown signal");
        }
        finally
        {
            await client.DisconnectAsync();
            Console.WriteLine("Example completed");
        }
    }
}
