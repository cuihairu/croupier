# Deployment

- Server runs in DMZ/public with mTLS on :443 (configurable via --config or env CROUPIER_SERVER_*)
- Agent runs in game private networks and dials out to Server (--config or env CROUPIER_AGENT_*)
- Edge (optional) can be started with `croupier edge` to relay between Server and Agents
- Game servers connect to local Agent

Status: skeleton.

## GeoIP / IP2Location（可选）

若希望在日志/审计/审批等页面展示“属地”（国家/省/市），可启用以下任一方案：

1) 离线库（推荐）
- 下载 IP2Location LITE DB（免费）：
  - IPv4：IP2LOCATION-LITE-DB3.BIN
  - IPv6：IP2LOCATION-LITE-DB3.IPV6.BIN
- 放置到 Server 工作目录的 `configs/` 下，文件名保持一致；或用环境变量显式指定：
  - IP2LOCATION_BIN_PATH=/abs/path/IP2LOCATION-LITE-DB3.BIN
  - IP2LOCATION_BIN_PATH_V6=/abs/path/IP2LOCATION-LITE-DB3.IPV6.BIN
- Server 运行时会自动探测并启用；不存在时自动跳过。

2) 在线 HTTP 解析
- 配置环境变量：
  - GEOIP_HTTP_URL：例如 `https://your-geo.example.com/lookup?ip={{ip}}`
  - GEOIP_TIMEOUT_MS：HTTP 调用超时，默认 1500
- 响应 JSON 可包含 `country/country_name`、`region/region_name/province/state`、`city` 中的一种或多种字段。

内网/本地地址不会进行查询：
- 127.0.0.1/::1 → “本地”；10/172.16–31/192.168/169.254、fc00::/7、fe80::/10 → “局域网”。

验证：
- 登录后台后查看“登录日志”的“属地”列，或请求 `/api/audit?kinds=login` 查看 `meta.ip_region`。
