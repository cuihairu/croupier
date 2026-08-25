// Game Demo - 19 functions matching the Go SDK demo.
//
// Covers: player/order lifecycle actions, leaderboard, inventory, and mail.
// Run: cd sdks/csharp && dotnet run --project examples/GameDemo

using Croupier.Sdk;
using Croupier.Sdk.Configuration;
using Croupier.Sdk.Models;
using System.Collections.Concurrent;
using System.Text.Json;

// ==================== Data Models ====================

record PlayerRecord(
    string Id, string Name, int Level, int Vip, long Gold,
    string Status, string Server, string CreatedAt, string UpdatedAt, string LastLoginAt,
    Dictionary<string, object>? Profile = null);

record OrderRecord(
    string Id, string PlayerId, string ProductId, long Amount,
    string Currency, string Status, string Channel, string CreatedAt, string UpdatedAt,
    Dictionary<string, object>? Attributes = null);

record LeaderboardEntry(string PlayerId, string PlayerName, long Score, int Rank, string UpdatedAt);

record ItemRecord(string Id, string TemplateId, string Name, long Quantity, string Rarity, string UpdatedAt);

record MailRecord(
    string Id, string PlayerId, string Title, string Content, string Status,
    Dictionary<string, object>? Reward, string SentAt, string UpdatedAt, string ExpireAt = "");

// ==================== In-Memory Store ====================

class DemoStore
{
    long _playerSeq = 1002, _orderSeq = 3002, _mailSeq = 5002;

    public ConcurrentDictionary<string, PlayerRecord> Players { get; } = new();
    public ConcurrentDictionary<string, OrderRecord> Orders { get; } = new();
    public ConcurrentDictionary<string, LeaderboardEntry> Leaderboard { get; } = new();
    public ConcurrentDictionary<string, ConcurrentDictionary<string, ItemRecord>> Inventories { get; } = new();
    public ConcurrentDictionary<string, List<MailRecord>> Mails { get; } = new();

    public DemoStore()
    {
        var now = DateTime.UtcNow.ToString("o");

        Players["player_1001"] = new PlayerRecord("player_1001", "Alice", 35, 3, 128800,
            "active", "s1", now, now, now,
            new Dictionary<string, object> { ["guild"] = "星海旅团", ["country"] = "CN", ["platform"] = "ios" });

        Players["player_1002"] = new PlayerRecord("player_1002", "Bob", 42, 5, 256000,
            "active", "s2", now, now, now,
            new Dictionary<string, object> { ["guild"] = "苍穹守卫", ["country"] = "US", ["platform"] = "android" });

        Orders["order_3001"] = new OrderRecord("order_3001", "player_1001", "com.croupier.gems.648",
            6480, "CNY", "paid", "appstore", now, now,
            new Dictionary<string, object> { ["region"] = "cn" });

        Orders["order_3002"] = new OrderRecord("order_3002", "player_1002", "battle.pass.s2",
            68, "USD", "pending", "googleplay", now, now);

        Leaderboard["player_1002"] = new LeaderboardEntry("player_1002", "Bob", 98500, 1, now);
        Leaderboard["player_1001"] = new LeaderboardEntry("player_1001", "Alice", 91200, 2, now);

        var inv = new ConcurrentDictionary<string, ItemRecord>();
        inv["gold_coin"] = new ItemRecord("item_gold_coin", "gold_coin", "金币", 128800, "common", now);
        inv["hero_ticket"] = new ItemRecord("item_hero_ticket", "hero_ticket", "英雄招募券", 12, "rare", now);
        Inventories["player_1001"] = inv;

        Mails["player_1001"] = new List<MailRecord>
        {
            new("mail_5001", "player_1001", "开服奖励", "欢迎来到 Croupier Demo World", "unread",
                new Dictionary<string, object> { ["gold"] = 10000, ["item"] = "hero_ticket" },
                now, now)
        };
    }

    public string NextPlayerId() => $"player_{Interlocked.Increment(ref _playerSeq)}";
    public string NextOrderId() => $"order_{Interlocked.Increment(ref _orderSeq)}";
    public string NextMailId() => $"mail_{Interlocked.Increment(ref _mailSeq)}";
}

// ==================== Helpers ====================

static class H
{
    public static string Now() => DateTime.UtcNow.ToString("o");

    public static Dictionary<string, object> Parse(string payload)
    {
        if (string.IsNullOrWhiteSpace(payload)) return new();
        try { return JsonSerializer.Deserialize<Dictionary<string, object>>(payload) ?? new(); }
        catch { return new(); }
    }

    public static string Resp(Dictionary<string, object> data)
    {
        data["timestamp"] = Now();
        return JsonSerializer.Serialize(data);
    }

    public static string Str(Dictionary<string, object> m, params string[] keys)
    {
        foreach (var k in keys)
            if (m.TryGetValue(k, out var v) && v is JsonElement je && je.ValueKind == JsonValueKind.String)
            { var s = je.GetString(); if (!string.IsNullOrWhiteSpace(s)) return s.Trim(); }
            else if (v is string s && !string.IsNullOrWhiteSpace(s)) return s.Trim();
        return "";
    }

    public static long Long(Dictionary<string, object> m, long def, params string[] keys)
    {
        foreach (var k in keys)
            if (m.TryGetValue(k, out var v))
            {
                if (v is JsonElement je)
                {
                    if (je.ValueKind == JsonValueKind.Number && je.TryGetInt64(out var n)) return n;
                    if (je.ValueKind == JsonValueKind.String && long.TryParse(je.GetString(), out var s)) return s;
                }
                else if (v is long l) return l;
                else if (v is int i) return i;
            }
        return def;
    }

    public static int Int(Dictionary<string, object> m, int def, params string[] keys) => (int)Long(m, def, keys);

    public static Dictionary<string, object>? Map(Dictionary<string, object> m, string key)
    {
        if (!m.TryGetValue(key, out var v)) return null;
        if (v is JsonElement je && je.ValueKind == JsonValueKind.Object)
            return JsonSerializer.Deserialize<Dictionary<string, object>>(je.GetRawText());
        return v as Dictionary<string, object>;
    }

    public static string NonEmpty(params string[] vals)
    {
        foreach (var v in vals) if (!string.IsNullOrWhiteSpace(v)) return v.Trim();
        return "";
    }
}

// ==================== Program ====================

class Program
{
    static async Task<int> Main(string[] args)
    {
        var agentAddr = Environment.GetEnvironmentVariable("CROUPIER_AGENT_ADDR") ?? "127.0.0.1:19091";
        var gameId = Environment.GetEnvironmentVariable("CROUPIER_GAME_ID") ?? "demo-game";
        var serviceId = Environment.GetEnvironmentVariable("CROUPIER_SERVICE_ID") ?? "game-demo-service";
        var envName = Environment.GetEnvironmentVariable("CROUPIER_ENV") ?? "development";

        Console.WriteLine($"starting game demo: agent={agentAddr} game={gameId} env={envName} service={serviceId}");

        var config = new ClientConfig
        {
            AgentAddr = agentAddr,
            GameId = gameId,
            Env = envName,
            ServiceId = serviceId,
            ServiceVersion = "1.0.0",
            Insecure = true,
        };

        using var client = new CroupierClient(config);
        var store = new DemoStore();

        var fns = new (string Id, string Risk, string Resource, string Op, FunctionHandlerDelegate Handler)[]
        {
            ("player.create", "warning", "player", "create", async (ctx, payload) => {
                var body = H.Parse(payload);
                var id = H.Str(body, "id", "playerId");
                if (id == "") id = store.NextPlayerId();
                var now = H.Now();
                var r = new PlayerRecord(id, H.NonEmpty(H.Str(body, "name"), $"Player-{id}"),
                    H.Int(body, 1, "level"), H.Int(body, 0, "vip"), H.Long(body, 0, "gold"),
                    H.NonEmpty(H.Str(body, "status"), "active"), H.NonEmpty(H.Str(body, "server"), "s1"),
                    now, now, now, H.Map(body, "profile"));
                store.Players[id] = r;
                return H.Resp(new() { ["status"] = "success", ["action"] = "player.create", ["player"] = r });
            }),
            ("player.get", "safe", "player", "get", async (ctx, payload) => {
                var body = H.Parse(payload);
                var id = H.Str(body, "playerId", "id");
                if (!store.Players.TryGetValue(id, out var r))
                    return H.Resp(new() { ["status"] = "not_found", ["message"] = "player not found" });
                return H.Resp(new() { ["status"] = "success", ["action"] = "player.get", ["player"] = r });
            }),
            ("player.update", "warning", "player", "update", async (ctx, payload) => {
                var body = H.Parse(payload);
                var id = H.Str(body, "playerId", "id");
                if (!store.Players.TryGetValue(id, out var r))
                    return H.Resp(new() { ["status"] = "not_found", ["message"] = "player not found" });
                var name = H.Str(body, "name"); if (name != "") r = r with { Name = name };
                if (body.ContainsKey("level")) r = r with { Level = H.Int(body, r.Level, "level") };
                if (body.ContainsKey("vip")) r = r with { Vip = H.Int(body, r.Vip, "vip") };
                if (body.ContainsKey("gold")) r = r with { Gold = H.Long(body, r.Gold, "gold") };
                var status = H.Str(body, "status"); if (status != "") r = r with { Status = status };
                var server = H.Str(body, "server"); if (server != "") r = r with { Server = server };
                var profile = H.Map(body, "profile"); if (profile != null) r = r with { Profile = profile };
                r = r with { UpdatedAt = H.Now() };
                store.Players[id] = r;
                return H.Resp(new() { ["status"] = "success", ["action"] = "player.update", ["player"] = r });
            }),
            ("player.delete", "danger", "player", "delete", async (ctx, payload) => {
                var body = H.Parse(payload);
                var id = H.Str(body, "playerId", "id");
                store.Players.TryRemove(id, out _); store.Inventories.TryRemove(id, out _);
                store.Mails.TryRemove(id, out _); store.Leaderboard.TryRemove(id, out _);
                return H.Resp(new() { ["status"] = "success", ["action"] = "player.delete", ["playerId"] = id });
            }),
            ("player.list", "safe", "player", "list", async (ctx, payload) => {
                var items = store.Players.Values.OrderBy(p => p.Id).ToList();
                return H.Resp(new() { ["status"] = "success", ["action"] = "player.list", ["items"] = items, ["total"] = items.Count });
            }),

            ("order.create", "warning", "order", "create", async (ctx, payload) => {
                var body = H.Parse(payload);
                var id = H.Str(body, "order_id", "id");
                if (id == "") id = store.NextOrderId();
                var now = H.Now();
                var r = new OrderRecord(id, H.Str(body, "playerId"),
                    H.NonEmpty(H.Str(body, "productId"), "product.demo"),
                    H.Long(body, 0, "amount"), H.NonEmpty(H.Str(body, "currency"), "CNY"),
                    H.NonEmpty(H.Str(body, "status"), "created"), H.NonEmpty(H.Str(body, "channel"), "gm"),
                    now, now, H.Map(body, "attributes"));
                store.Orders[id] = r;
                return H.Resp(new() { ["status"] = "success", ["action"] = "order.create", ["order"] = r });
            }),
            ("order.get", "safe", "order", "get", async (ctx, payload) => {
                var body = H.Parse(payload);
                var id = H.Str(body, "order_id", "id");
                if (!store.Orders.TryGetValue(id, out var r))
                    return H.Resp(new() { ["status"] = "not_found", ["message"] = "order not found" });
                return H.Resp(new() { ["status"] = "success", ["action"] = "order.get", ["order"] = r });
            }),
            ("order.update", "warning", "order", "update", async (ctx, payload) => {
                var body = H.Parse(payload);
                var id = H.Str(body, "order_id", "id");
                if (!store.Orders.TryGetValue(id, out var r))
                    return H.Resp(new() { ["status"] = "not_found", ["message"] = "order not found" });
                var status = H.Str(body, "status"); if (status != "") r = r with { Status = status };
                var channel = H.Str(body, "channel"); if (channel != "") r = r with { Channel = channel };
                if (body.ContainsKey("amount")) r = r with { Amount = H.Long(body, r.Amount, "amount") };
                var attrs = H.Map(body, "attributes"); if (attrs != null) r = r with { Attributes = attrs };
                r = r with { UpdatedAt = H.Now() };
                store.Orders[id] = r;
                return H.Resp(new() { ["status"] = "success", ["action"] = "order.update", ["order"] = r });
            }),
            ("order.delete", "danger", "order", "delete", async (ctx, payload) => {
                var body = H.Parse(payload);
                var id = H.Str(body, "order_id", "id");
                store.Orders.TryRemove(id, out _);
                return H.Resp(new() { ["status"] = "success", ["action"] = "order.delete", ["order_id"] = id });
            }),
            ("order.list", "safe", "order", "list", async (ctx, payload) => {
                var body = H.Parse(payload);
                var pid = H.Str(body, "playerId");
                var items = store.Orders.Values.Where(o => pid == "" || o.PlayerId == pid).OrderBy(o => o.Id).ToList();
                return H.Resp(new() { ["status"] = "success", ["action"] = "order.list", ["items"] = items, ["total"] = items.Count });
            }),

            ("leaderboard.list", "safe", "leaderboard", "list", async (ctx, payload) => {
                var sorted = store.Leaderboard.Values.OrderByDescending(e => e.Score).ToList();
                var ranked = sorted.Select((e, i) => e with { Rank = i + 1 }).ToList();
                return H.Resp(new() { ["status"] = "success", ["action"] = "leaderboard.list", ["items"] = ranked, ["total"] = ranked.Count });
            }),
            ("leaderboard.upsert", "warning", "leaderboard", "upsert", async (ctx, payload) => {
                var body = H.Parse(payload);
                var pid = H.Str(body, "playerId");
                if (pid == "") throw new ArgumentException("player_id is required");
                var pname = pid;
                if (store.Players.TryGetValue(pid, out var p) && p.Name != "") pname = p.Name;
                var e = new LeaderboardEntry(pid, pname, H.Long(body, 0, "score"), 0, H.Now());
                store.Leaderboard[pid] = e;
                return H.Resp(new() { ["status"] = "success", ["action"] = "leaderboard.upsert", ["entry"] = e });
            }),
            ("leaderboard.reset", "danger", "leaderboard", "reset", async (ctx, payload) => {
                store.Leaderboard.Clear();
                return H.Resp(new() { ["status"] = "success", ["action"] = "leaderboard.reset" });
            }),

            ("inventory.list", "safe", "inventory", "list", async (ctx, payload) => {
                var body = H.Parse(payload);
                var pid = H.Str(body, "playerId");
                if (pid == "") throw new ArgumentException("player_id is required");
                var items = store.Inventories.TryGetValue(pid, out var inv)
                    ? inv.Values.OrderBy(i => i.TemplateId).ToList() : new List<ItemRecord>();
                return H.Resp(new() { ["status"] = "success", ["action"] = "inventory.list", ["playerId"] = pid, ["items"] = items });
            }),
            ("inventory.grant", "warning", "inventory", "grant", async (ctx, payload) => {
                var body = H.Parse(payload);
                var pid = H.Str(body, "playerId");
                var tid = H.Str(body, "templateId", "item_id");
                if (pid == "" || tid == "") throw new ArgumentException("player_id and template_id are required");
                var inv = store.Inventories.GetOrAdd(pid, _ => new());
                var qty = H.Long(body, 1, "quantity");
                inv.AddOrUpdate(tid,
                    _ => new ItemRecord($"item_{tid}", tid, H.NonEmpty(H.Str(body, "name"), tid), qty,
                        H.NonEmpty(H.Str(body, "rarity"), "common"), H.Now()),
                    (_, existing) => existing with { Quantity = existing.Quantity + qty, UpdatedAt = H.Now() });
                return H.Resp(new() { ["status"] = "success", ["action"] = "inventory.grant", ["playerId"] = pid, ["item"] = inv[tid] });
            }),
            ("inventory.consume", "warning", "inventory", "consume", async (ctx, payload) => {
                var body = H.Parse(payload);
                var pid = H.Str(body, "playerId");
                var tid = H.Str(body, "templateId", "item_id");
                var qty = H.Long(body, 1, "quantity");
                if (pid == "" || tid == "") throw new ArgumentException("player_id and template_id are required");
                if (!store.Inventories.TryGetValue(pid, out var inv) || !inv.TryGetValue(tid, out var r))
                    return H.Resp(new() { ["status"] = "not_found", ["message"] = "item not found" });
                if (r.Quantity < qty)
                    return H.Resp(new() { ["status"] = "failed", ["message"] = "insufficient quantity" });
                inv[tid] = r with { Quantity = r.Quantity - qty, UpdatedAt = H.Now() };
                return H.Resp(new() { ["status"] = "success", ["action"] = "inventory.consume", ["playerId"] = pid, ["item"] = inv[tid] });
            }),

            ("mail.send", "warning", "mail", "send", async (ctx, payload) => {
                var body = H.Parse(payload);
                var pid = H.Str(body, "playerId");
                if (pid == "") throw new ArgumentException("player_id is required");
                var now = H.Now();
                var r = new MailRecord(store.NextMailId(), pid,
                    H.NonEmpty(H.Str(body, "title"), "系统邮件"),
                    H.NonEmpty(H.Str(body, "content"), "请查收奖励"),
                    "unread", H.Map(body, "reward"), now, now, H.Str(body, "expireAt"));
                store.Mails.AddOrUpdate(pid, _ => new List<MailRecord> { r }, (_, list) => { lock (list) { list.Add(r); } return list; });
                return H.Resp(new() { ["status"] = "success", ["action"] = "mail.send", ["mail"] = r });
            }),
            ("mail.list", "safe", "mail", "list", async (ctx, payload) => {
                var body = H.Parse(payload);
                var pid = H.Str(body, "playerId");
                if (pid == "") throw new ArgumentException("player_id is required");
                List<MailRecord> items = new();
                if (store.Mails.TryGetValue(pid, out var list)) lock (list) { items = list.ToList(); }
                return H.Resp(new() { ["status"] = "success", ["action"] = "mail.list", ["playerId"] = pid, ["items"] = items, ["total"] = items.Count });
            }),
            ("mail.claim", "warning", "mail", "claim", async (ctx, payload) => {
                var body = H.Parse(payload);
                var pid = H.Str(body, "playerId");
                var mid = H.Str(body, "mail_id", "id");
                if (pid == "" || mid == "") throw new ArgumentException("player_id and mail_id are required");
                if (store.Mails.TryGetValue(pid, out var list))
                {
                    lock (list)
                    {
                        var m = list.FirstOrDefault(x => x.Id == mid);
                        if (m != null)
                        {
                            var claimed = m with { Status = "claimed", UpdatedAt = H.Now() };
                            list[list.IndexOf(m)] = claimed;
                            return H.Resp(new() { ["status"] = "success", ["action"] = "mail.claim", ["mail"] = claimed });
                        }
                    }
                }
                return H.Resp(new() { ["status"] = "not_found", ["message"] = "mail not found" });
            }),
        };

        foreach (var (id, risk, resource, op, handler) in fns)
        {
            var desc = new FunctionDescriptor
            {
                Id = id, Version = "1.0.0", Risk = risk,
                Resource = resource, Operation = op, Enabled = true,
            };
            EnrichDescriptor(desc);
            client.RegisterFunction(desc, handler);
            Console.WriteLine($"  registered: {id}");
        }

        Console.WriteLine();
        Console.WriteLine("========================================");
        Console.WriteLine("Service started. Press Ctrl+C to exit.");
        Console.WriteLine("========================================");

        var cts = new CancellationTokenSource();
        Console.CancelKeyPress += (s, e) => { e.Cancel = true; cts.Cancel(); };

        try
        {
            await client.ConnectAsync();
            Console.WriteLine($"  Connected to Agent at {agentAddr}");
            await client.ServeAsync(cts.Token);
        }
        catch (OperationCanceledException) { }
        catch (Exception ex)
        {
            Console.Error.WriteLine($"  Connection failed: {ex.Message}");
            return 1;
        }
        finally
        {
            client.Stop();
            client.Disconnect();
        }

        Console.WriteLine("Service stopped.");
        return 0;
    }

    static void EnrichDescriptor(FunctionDescriptor desc)
    {
        desc.Summary ??= $"{desc.Resource} {desc.Operation}";
        desc.Description ??= $"Demo function {desc.Id} for {desc.Resource} {desc.Operation} action.";
        desc.OperationId ??= desc.Id;
        desc.Tags ??= new List<string> { desc.Resource ?? "", desc.Operation ?? "" };
        var schemas = SchemasFor(desc.Id ?? "");
        desc.InputSchema ??= schemas.Input;
        desc.OutputSchema ??= schemas.Output;
    }

    // Schemas describe the handlers' real wire contract with camelCase JSON
    // keys. snake_case is only allowed inside databases, never on the wire.
    static readonly string SchemaObj = "{\"type\":\"object\"}";
    static readonly string SchemaStr = "{\"type\":\"string\"}";
    static readonly string SchemaInt = "{\"type\":\"integer\"}";
    static readonly string PlayerFields =
        "{\"id\":" + SchemaStr + ",\"name\":" + SchemaStr + ",\"level\":" + SchemaInt + ",\"vip\":" + SchemaInt +
        ",\"gold\":" + SchemaInt + ",\"status\":" + SchemaStr + ",\"server\":" + SchemaStr + ",\"profile\":" + SchemaObj + "}";

    static (string Input, string Output) SchemasFor(string id) => id switch
    {
        "player.create" => (BuildObj("{" + PlayerFields + "}"), BuildObj("{\"player\":" + SchemaObj + "}")),
        "player.get" => (BuildObj("{\"id\":" + SchemaStr + "}", new[] { "id" }), BuildObj("{\"player\":" + SchemaObj + "}")),
        "player.update" => (BuildObj("{" + PlayerFields + "}", new[] { "id" }), BuildObj("{\"player\":" + SchemaObj + "}")),
        "player.delete" => (BuildObj("{\"id\":" + SchemaStr + "}", new[] { "id" }), BuildObj("{\"playerId\":" + SchemaStr + "}")),
        "player.list" => (BuildObj("{\"page\":" + SchemaInt + ",\"pageSize\":" + SchemaInt + "}"),
                          BuildObj("{\"items\":{\"type\":\"array\",\"items\":" + SchemaObj + "},\"total\":" + SchemaInt + "}")),
        _ => (BuildObj("{}"), "{\"type\":\"object\",\"properties\":{\"status\":{\"type\":\"string\"},\"action\":{\"type\":\"string\"}}}"),
    };

    static string BuildObj(string props, string[]? required = null)
    {
        var schema = "{\"type\":\"object\",\"properties\":" + props + "}";
        if (required != null && required.Length > 0)
        {
            schema = schema.Substring(0, schema.Length - 1) + ",\"required\":[\"" + string.Join("\",\"", required) + "\"]}";
        }
        return schema;
    }

}
