# 远程访问（网页 RDP/SSH）方案设计

面向 Croupier 平台，将“服务器的登录与操作”纳入统一入口并留痕审计的可行方案整理。遵循 KISS/YAGNI：本设计先给出稳妥选型与最小闭环，后续按优先级扩展。

## 目标与范围
- 通过浏览器访问服务器的 RDP/SSH（必要时 VNC/KVM），无需客户端插件
- 接入平台 RBAC/审批流，做到“谁在何时访问了哪台机器”的可追踪
- 支持会话录制（桌面/终端）与操作日志留存，安全受控（禁用高风险重定向）
- 兼容多网络环境（内网/DMZ/公网），可水平扩展

## 方案选型（结论）
- 优先：Apache Guacamole（开源、成熟、无 Agent，WebSocket + Canvas 前端，guacd 作为协议网关）
  - 协议：RDP、SSH、VNC
  - 审计：会话录屏（.guac）、SSH 命令录制（typescript）、连接历史入库
  - 集成：数据库/LDAP/OIDC/2FA；REST API 管理连接和用户
- 备选/增强：MeshCentral（Node.js + Agent）
  - 优点：穿透 NAT、资产管理、广泛审计与事件，多端控制能力强
  - 代价：需部署 Agent、引入与运维成本更高

推荐：以 Guacamole 为基础能力，后续按需补充 MeshCentral 作为“有 Agent 的深度管控”补充（双栈）。

## 推荐架构与拓扑
- 控制面：Croupier（内网）统一 RBAC/审批/审计 → 反代/单点登录到 Guacamole Web（DMZ/内网）
- 协议面：浏览器 → WebSocket → Guacamole Web → guacd → 目标 RDP/SSH 主机（guacd 与目标机同域/隧道可达）
- 存储与审计：
  - 会话历史：Guacamole DB → 周期同步/直连查询写入 ClickHouse（统一审计视图）
  - 会话录制：文件存储（NAS/对象存储），索引与元数据入库，访问经审批与临时凭证

## 能力映射与安全控制
- 认证/授权：
  - 首选 OIDC/反向代理头部认证（由 Croupier SSO 透传），开启 2FA（TOTP）
  - Croupier 侧按“主机/环境/标签”做授权，调用 Guacamole REST API 动态创建/启用连接
- 审计留痕：
  - 连接历史：用户、时间、目标、时长、来源 IP
  - 录制：RDP 录屏（.guac）、SSH typescript；支持命名模板与集中存放
- 限制策略：
  - 禁用非必要通道（文件、打印、剪贴板）
  - 仅 HTTPS 暴露，WAF 与速率限制；外网访问必须通过审批

## 与 Croupier 的集成设计（最小闭环）
1) 身份与入口
   - 前端：在 Web 中以 IFrame/反代路径嵌入 Guacamole 控制台（/ops/remote）
   - 单点：由反向代理注入登录标识（OIDC/OAuth2 Proxy），Guacamole 侧启用对应模块
2) 授权与审批
   - Croupier 资源模型：Host（主机）、HostGroup（生产/预发/测试）、Env（dev/stage/prod）
   - 高危访问（生产）走审批流，审批通过后生成“临时连接/短期令牌”，到期自动失效
3) 审计归集
   - 连接历史同步至 ClickHouse：任务周期拉取或 DB 视图直连
   - 录制文件索引入库：路径、时长、操作者、目标；文件存储以生命周期策略管理
4) 安全策略
   - 默认禁用 RDP 文件/打印/剪贴板；SSH 禁用 SFTP（按需启用白名单）
   - 审计员角色可查看录制；普通用户仅能发起连接不可访问他人录制

## Guacamole 关键配置（示例）
- RDP 连接参数：
  - `recording-path=/var/recordings/rdp`
  - `recording-name=\${GUAC_USERNAME}-\${GUAC_DATE}.guac`
  - `create-recording-path=true`
  - `disable-clipboard=true, enable-audio=false, enable-drive=false`（按需）
- SSH 连接参数：
  - `typescript-path=/var/recordings/ssh`
  - `typescript-name=\${GUAC_USERNAME}-\${GUAC_DATE}`
  - `create-typescript-path=true`
- 认证与 SSO：
  - 数据库/MySQL/Postgres
  - OIDC/TOTP 模块（与反向代理/OAuth2 Proxy 配合）

## 最小化部署骨架（示意 Compose）
```yaml
services:
  guacd:
    image: guacamole/guacd:1.5.5
    restart: unless-stopped

  db:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
    volumes:
      - guac-mysql:/var/lib/mysql
    restart: unless-stopped

  guacamole:
    image: guacamole/guacamole:1.5.5
    depends_on: [guacd, db]
    environment:
      MYSQL_HOSTNAME: db
      MYSQL_DATABASE: guacamole_db
      MYSQL_USER: guacamole_user
      MYSQL_PASSWORD: guacpass
      GUACD_HOSTNAME: guacd
    ports:
      - "8081:8080"
    restart: unless-stopped

volumes:
  guac-mysql: {}
```
注：首次部署需执行官方 initdb.sql 初始化数据库；生产环境放置在反向代理后并启用 HTTPS、WAF、限流与 OIDC/2FA。

## MeshCentral 作为补充（何时考虑）
- 场景：需要穿透 NAT/零信任、设备资产管理、远程电源/KVM，更强的主机侧审计
- 代价：需要安装 Agent、更多的服务端维护与升级
- 可与 Guacamole 并存：有 Agent 的主机走 MeshCentral，无 Agent 的主机走 Guacamole

## 里程碑规划（建议）
- M1（PoC 1-2 天）：起 Guacamole（录制开启）→ 反代 HTTPS → 测试 RDP/SSH 连接与录制 → 导出连接历史
- M2（集成 3-5 天）：Croupier 中增“远程访问”资源与审批 → 适配 Guacamole REST（临时连接/令牌）→ 审计元数据入 ClickHouse 与页面查询
- M3（生产化 3-7 天）：对象存储/加密/生命周期、审计员权限、黑白名单策略、WAF/限流、告警与容量监控
- M4（可选）：MeshCentral 双栈、敏感操作水印、录制检索（OCR/全文索引）

## 风险与边界
- 录制体积与保留周期：需容量规划（对象存储/压缩/归档）
- 安全合规：录屏涉及敏感信息，严格分权与审批；访问录制应有审计
- 网络可达性：guacd 到目标主机需同域或经隧道；零信任/SDP 可引入但复杂度上升

## 结论
- 近期以 Guacamole 为“网页 RDP/SSH 网关 + 审计录制”的基础能力，快速纳管服务器访问
- 通过 Croupier 统一入口与 RBAC/审批/审计，打通最小闭环
- 需要更深控时再引入 MeshCentral（双栈），逐步完善
