<template><div><h1 id="deployment" tabindex="-1"><a class="header-anchor" href="#deployment"><span>Deployment</span></a></h1>
<ul>
<li>Server runs in DMZ/public with mTLS on :443 (configurable via --config or env CROUPIER_SERVER_*)</li>
<li>Agent runs in game private networks and dials out to Server (--config or env CROUPIER_AGENT_*)</li>
<li>Edge (optional) can be started with <code v-pre>croupier edge</code> to relay between Server and Agents</li>
<li>Game servers connect to local Agent</li>
</ul>
<p>Status: skeleton.</p>
<h2 id="geoip-ip2location-可选" tabindex="-1"><a class="header-anchor" href="#geoip-ip2location-可选"><span>GeoIP / IP2Location（可选）</span></a></h2>
<p>若希望在日志/审计/审批等页面展示“属地”（国家/省/市），可启用以下任一方案：</p>
<ol>
<li>离线库（推荐）</li>
</ol>
<ul>
<li>下载 IP2Location LITE DB（免费）：
<ul>
<li>IPv4：IP2LOCATION-LITE-DB3.BIN</li>
<li>IPv6：IP2LOCATION-LITE-DB3.IPV6.BIN</li>
</ul>
</li>
<li>放置到 Server 工作目录的 <code v-pre>configs/</code> 下，文件名保持一致；或用环境变量显式指定：
<ul>
<li>IP2LOCATION_BIN_PATH=/abs/path/IP2LOCATION-LITE-DB3.BIN</li>
<li>IP2LOCATION_BIN_PATH_V6=/abs/path/IP2LOCATION-LITE-DB3.IPV6.BIN</li>
</ul>
</li>
<li>Server 运行时会自动探测并启用；不存在时自动跳过。</li>
</ul>
<ol start="2">
<li>在线 HTTP 解析</li>
</ol>
<ul>
<li>配置环境变量：
<ul>
<li>GEOIP_HTTP_URL：例如 <code v-pre>https://your-geo.example.com/lookup?ip={{ip}}</code></li>
<li>GEOIP_TIMEOUT_MS：HTTP 调用超时，默认 1500</li>
</ul>
</li>
<li>响应 JSON 可包含 <code v-pre>country/country_name</code>、<code v-pre>region/region_name/province/state</code>、<code v-pre>city</code> 中的一种或多种字段。</li>
</ul>
<p>内网/本地地址不会进行查询：</p>
<ul>
<li>127.0.0.1/::1 → “本地”；10/172.16–31/192.168/169.254、fc00::/7、fe80::/10 → “局域网”。</li>
</ul>
<p>验证：</p>
<ul>
<li>登录后台后查看“登录日志”的“属地”列，或请求 <code v-pre>/api/audit?kinds=login</code> 查看 <code v-pre>meta.ip_region</code>。</li>
</ul>
</div></template>


