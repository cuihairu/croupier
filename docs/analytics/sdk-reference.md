---
title: SDK 参考
---
# 客户端上报概览

Go（示例）
```go
client := analytics.NewClient(analytics.Config{
  Endpoint: "https://ingest.example.com",
  Secret:   os.Getenv("ANALYTICS_SECRET"),
})
client.Track(context.Background(), analytics.Event{
  Name: "session.start",
  Ts:   time.Now(),
  Attrs: map[string]any{"uid": "u1", "game_id": "demo"},
})
```

C#（Unity 快速接入，示意）
```csharp
var c = new AnalyticsClient(new Config {
  Endpoint = "https://ingest.example.com",
  Secret = Env.Get("ANALYTICS_SECRET")
});
c.Track("session.start", new { uid = "u1", game_id = "demo" });
```

说明
- SDK 做签名、重试、批量；服务端做去重、防重放、鉴权与限流
- 服务端 Traces/Metrics 建议使用 OTel SDK，业务事件用 Analytics SDK
