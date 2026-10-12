---
title: 密码箱 Provider——凭据统一存取抽象与可替换提供商（vault-provider）
---

# 密码箱 Provider（vault-provider）

## 状态

- 状态: **抽象与默认提供商已落码（2026-10-13，todo 4249/4250/4252）**。`internal/platform/vault` 承载 Provider 接口、编译期注册表与两个内置提供商（local 加密文件 / env 只读源）；svc 装配层 `wireVault` 接线，`enabled` 缺省 false=零行为变化。接入点迁移（todo 4251，把现存明文凭据配置逐点改为经密码箱读取）未开始，其前置盘点见 §6。与 [server-status-provider](server-status-provider.md)（Provider 注册表同构先例）、[plugin-mechanism](plugin-mechanism.md)（出站插件机制）互链。

## 1. 背景与定调

用户令（2026-10-11）：persona 新升级已实现密码箱、可作网站密码箱使用；**各项目密码箱应抽象接口、支持替换提供商**。croupier 侧落成三件事：

1. **接口抽象**（todo 4249）：解锁/读/写/列/删除五操作闭集，实现侧自持会话语义（KDF、主口令来源、会话超时收敛在提供商内部，调用面零感知）。
2. **默认实现与注册表**（todo 4250）：`local`（AES-256-GCM + scrypt 加密文件）开箱即用，`env`（只读环境变量源）作容器部署/演示切换的第二实现；编译期工厂注册表（serverstatus/outlet 同构），**切换提供商=只改配置 `vault.provider` 键**。
3. **配置与文档**（todo 4252）：`vault:` 配置段逐字段口径 + 本文档；示例只给环境变量/文件路径引用，不给真实值。

红线（批头即令，测试用例锁死）：

- **密钥不进仓库**：存储文件不入 git；配置示例不含真实凭据。
- **解锁口令与明文凭据不落日志、不出现在错误消息**：`ErrPassphraseMismatch` 等错误闭集消息均为静态文案；单测断言错误消息不回显口令、存储文件字节不含明文。

## 2. Provider 接口（`internal/platform/vault`）

```go
type Provider interface {
    Name() string
    Unlock(ctx) error                       // 解锁会话（实现侧持会话超时；重复调用=续期）
    Locked() bool
    Lock()                                  // 立即锁定（清零会话密钥）
    Read(ctx, path) (string, error)         // 明文只允许内存传递
    Write(ctx, path, value string) error
    List(ctx) ([]EntryMeta, error)          // 路径+更新时间，永不含明文
    Delete(ctx, path) error
}
```

错误闭集（调用方可分支）：`ErrLocked`（未解锁/会话过期）、`ErrNotFound`、`ErrPassphraseMissing`（口令环境变量缺位）、`ErrPassphraseMismatch`（口令不匹配或存储损坏）、`ErrReadOnly`（env 源）、`ErrInvalidPath`（路径非法：空/含 `..`/段为空）。

## 3. 内置提供商

### 3.1 local（默认，加密文件存储）

- **KDF**：scrypt（N=32768, r=8, p=1, 32B 密钥），盐 16B 随机，参数随存储持久化（后续算法升级可辨识）。
- **加密**：AES-256-GCM，逐条随机 nonce；条目密文以凭据路径作 associated data（防密文挪位）。
- **校验块**：`check` 字段存已知常量的密文——Unlock 用派生密钥解校验块验口令，无需存在任何条目；口令错/存储损坏统一报 `ErrPassphraseMismatch`。
- **存储**：单文件 JSON（缺省 `data/vault.json`，0600/目录 0700），全量重写 + tmp+rename 原子落盘；写/删失败回滚内存态保持与磁盘一致。
- **会话**：滑动 TTL（缺省 900s）——每次成功操作续期，空闲超时自动锁定；`Read` 等报 `ErrLocked`，重新 `Unlock` 恢复。启动预检失败不阻断进程（fail-soft，调用方兜底）。

### 3.2 env（只读，环境变量源）

- 凭据路径映射环境变量名：`smtp/password` → `<prefix>SMTP_PASSWORD`（`/` 与 `-` 归并为 `_` 后大写；`smtp_password` 是等价别名）。缺省前缀 `CROUPIER_VAULT_ENV_`。
- 恒解锁、`Write`/`Delete` 报 `ErrReadOnly`；`List` 枚举带前缀变量（只报名不报值，反向映射有损——`_` 不还原为 `/`）。
- 用途：容器编排层管 secret 的部署形态；也是「切换提供商」验收的第二实现。

## 4. 装配（`internal/svc/vault.go`）

`NewServiceContext → wireVault`：读 `vault:` 配置段 → 注册表 `vault.Build(provider, SourceOptions{...})` → 启动解锁预检（失败只 WARN 留痕，不阻断启动）→ `ctx.Vault`。`enabled=false`（缺省）或提供商构建失败 → `ctx.Vault=nil`，接入点行为与现状一致。业务代码只依赖 `vault.Provider` 接口——**提供商切换=只改配置**（验收口径①已在 `TestWireVaultProviderSwitchConfigOnly` 留证：同一取凭据调用面，local/env 两提供商先后交付同一凭据）。

## 5. 配置逐字段

| 字段                                  | 缺省                        | 说明                                                   |
| ------------------------------------- | --------------------------- | ------------------------------------------------------ |
| `vault.enabled`                       | `false`                     | 关=不装配（`ctx.Vault=nil`），零行为变化               |
| `vault.provider`                      | `local`                     | 注册表键（`local` \| `env`；未知键装配失败 WARN 留痕） |
| `vault.sessionTtlSeconds`             | `900`                       | 解锁会话空闲超时（local；滑动续期）                    |
| `vault.providers.local.path`          | `data/vault.json`           | 加密存储文件路径（0600；首次解锁自动创建）             |
| `vault.providers.local.passphraseEnv` | `CROUPIER_VAULT_PASSPHRASE` | 持主口令的环境变量名（口令不落配置文件）               |
| `vault.providers.env.prefix`          | `CROUPIER_VAULT_ENV_`       | 凭据路径 → 环境变量名前缀                              |

## 6. 接入点盘点（todo 4251 输入，实现前先列全）

4251 的验收口径是全有全无（「扫描确认无一处直读明文」），无法半交付；本节为改造的存量清单（2026-10-13 扫描）。

**config 文件面（`internal/config/config.go`）**：

| 接入点                        | 位置                                                   | 现状         |
| ----------------------------- | ------------------------------------------------------ | ------------ |
| Redis 密码                    | `cache.redis.password`（cache/queue/session 三处形态） | 配置文件明文 |
| JWT 密钥                      | `auth.jwtSecret`（含 legacy 兼容段）                   | 配置文件明文 |
| LDAP 绑定口令                 | `auth.ldap.bindPassword`                               | 配置文件明文 |
| OIDC/Gitea/Casdoor 客户端密钥 | `auth.*.clientSecret` ×3                               | 配置文件明文 |
| 钉钉应用密钥                  | `auth.dingtalk.appSecret`                              | 配置文件明文 |
| S3 对象存储密钥               | `storage.s3 accessKey/secretKey`（含 legacy）          | 配置文件明文 |
| 数据库 DSN                    | `database.dataSource`（密码内嵌 DSN 串）               | 配置文件明文 |

**DB settings 面**：通知渠道 secret（钉钉/飞书/webhook `notification.*Secret`、SMTP `smtp.password`）现落 `platform_settings` 表明文——迁移时需决策「密码箱值经 settings 引用」的间接形态。

**Agent 侧**：capture 源库口令 `agents/capture/source` `MySQLConfig.Password`（本地 YAML 明文）——agent 无 server 配置体系，迁移形态（本地密码箱实例 vs 引用 server 密码箱）待 4251 拍板。

改造原则（与既有口径一致）：密钥值从「配置文件明文」迁到「密码箱路径引用」；环境变量引用形态（tokenEnv/secretEnv）视为已达标，不在迁移范围。

## 7. 已知边界（诚实清单）

- **接入点迁移未开始**（todo 4251）：本批交付后 `ctx.Vault` 已可用但无业务调用方——密码箱是「可用的新能力」，不是「已生效的凭据改造」；存量凭据仍按现状读取。
- persona 外部密码箱提供商未实现（接口已为其预留；接入=注册一项工厂 + 配置键，业务代码零改动）。
- 无管理面（API/UI）操作密码箱条目；写入口前为装配层与测试（4251 接入时如需运营写入再扩展）。
- `List` 的 env 源反向映射有损（`_` 不还原 `/`）；local 会话密钥清零是 best effort（语言/OS 层残留不在此控制内）。
- 换主口令=重建存储重录条目（v1 无 re-key 流程）。
