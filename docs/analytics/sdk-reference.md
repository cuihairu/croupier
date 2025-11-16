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

JavaScript（Node/浏览器）
```js
// 计算签名：Base64(HMAC_SHA256(secret, `${ts}\n${nonce}\n${sha256(body)}`))
import crypto from 'node:crypto';
const endpoint = 'https://ingest.example.com/api/ingest/events';
const secret = process.env.ANALYTICS_SECRET;
const body = JSON.stringify([{ event: 'session.start', ts: Date.now(), attrs: { uid: 'u1', game_id: 'demo' } }]);
const ts = Math.floor(Date.now() / 1000).toString();
const nonce = crypto.randomBytes(8).toString('hex');
const bodySha = crypto.createHash('sha256').update(body).digest('hex');
const sig = crypto.createHmac('sha256', secret).update(`${ts}\n${nonce}\n${bodySha}`).digest('base64');
await fetch(endpoint, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', 'X-Timestamp': ts, 'X-Nonce': nonce, 'X-Signature': sig },
  body
});
```

说明
- SDK 做签名、重试、批量；服务端做去重、防重放、鉴权与限流
- 服务端 Traces/Metrics 建议使用 OTel SDK，业务事件用 Analytics SDK
