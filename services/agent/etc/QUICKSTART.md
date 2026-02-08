# 🚀 Agent OpenAPI 注册 - 快速参考

## 📁 文件结构

```
services/agent/etc/
├── agent.yaml                   # Agent 主配置（已存在）
├── providers.yaml               # ✨ Provider 配置（新建）
├── providers.example.yaml       # 配置示例（参考）
├── openapi.example.yaml         # OpenAPI 示例（完整）
└── README-OPENAPI.md            # 详细文档
```

---

## ⚡ 30 秒快速开始

### 步骤 1: 创建配置文件

```bash
cd /Users/cui/Workspaces/croupier/croupier/services/agent/etc
cp providers.yaml providers.yaml.backup  # 备份原配置
```

### 步骤 2: 编辑 `providers.yaml`

```yaml
providers:
  my_functions:
    enabled: true
    type: openapi
    config:
      openapi_spec: ./etc/openapi.example.yaml  # 你的 OpenAPI 文件
      base_url: http://localhost:8080
      timeout: 30s
```

### 步骤 3: 重启 Agent

```bash
# 停止现有 Agent
pkill croupier-agent

# 启动 Agent
cd /Users/cui/Workspaces/croupier/croupier
./bin/croupier-agent -config ./services/agent/etc/agent.yaml
```

### 步骤 4: 验证

查看日志：
```bash
journalctl -u croupier-agent -f
# 或
tail -f /var/log/croupier-agent.log
```

**期望输出**：
```
INFO  provider loaded name=my_functions methods=13
INFO  registering provider method function=my_functions.player.create
```

---

## 📚 三种配置方式

### 方式 1️⃣: 单个文件

```yaml
providers:
  single:
    enabled: true
    type: openapi
    config:
      openapi_spec: ./my-functions.yaml
      base_url: http://localhost:8080
```

### 方式 2️⃣: 多个文件（推荐）

```yaml
providers:
  multiple:
    enabled: true
    type: openapi
    config:
      openapi_specs:
        - ./player-functions.yaml
        - ./inventory-functions.yaml
        - ./chat-functions.yaml
      base_url: http://localhost:8080
```

### 方式 3️⃣: 手动定义

```yaml
providers:
  manual:
    enabled: true
    type: openapi
    config:
      base_url: http://localhost:8080
      methods:
        - name: my_function
          path: /api/v1/my
          method: POST
          tags: [custom]
```

---

## 🎯 常见场景

### 场景 A: 加载示例函数

```yaml
providers:
  examples:
    enabled: true
    type: openapi
    config:
      openapi_spec: ./etc/openapi.example.yaml
      base_url: http://localhost:8080
```

**注册 13 个示例函数**：
- `examples.player.create`
- `examples.player.get`
- `examples.player.update_profile`
- `examples.player.delete`
- `examples.player.ban_batch`
- ... 等

### 场景 B: 加载所有 Packs

```yaml
providers:
  game_packs:
    enabled: true
    type: openapi
    config:
      openapi_specs:
        - ../packs/http/openapi.yaml
        - ../packs/prom/openapi.yaml
        - ../packs/player/openapi.yaml
        - ../packs/grafana/openapi.yaml
        - ../packs/alertmanager/openapi.yaml
      base_url: http://localhost:8080
```

### 场景 C: 本地 + 远程

```yaml
providers:
  hybrid:
    enabled: true
    type: openapi
    config:
      openapi_specs:
        - ./etc/local-functions.yaml      # 本地文件
        - https://api.example.com/openapi.yaml  # 远程 URL
      base_url: http://localhost:8080
      timeout: 30s
```

---

## 📋 函数 ID 命名规则

注册后的函数 ID 格式：`{provider_name}.{method_name}`

| 平台名称 | operationId/path | 函数 ID |
|---------|-----------------|---------|
| `examples` | `player.create` | `examples.player.create` |
| `game_packs` | `player.ban` | `game_packs.player.ban` |
| `my_funcs` | `/api/player/{id}` (GET) | `my_funcs.getPlayer` |

---

## ⚙️ 配置选项速查

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | false | 是否启用平台 |
| `type` | string | - | 固定为 `openapi` |
| `openapi_spec` | string | - | 单个 OpenAPI 文件路径 |
| `openapi_specs` | []string | - | 多个 OpenAPI 文件路径 |
| `base_url` | string | - | API 基础 URL |
| `timeout` | duration | 30s | HTTP 请求超时 |
| `retry_count` | int | 0 | 重试次数 |
| `headers` | map | - | 默认请求头 |
| `auth.type` | string | none | 认证类型 |
| `auth.token` | string | - | Bearer token |
| `auth.api_key` | object | - | API key 配置 |
| `rate_limit.requests_per_minute` | int | - | 速率限制 |
| `transform.success_field` | string | - | 响应转换 |

---

## 🔍 验证命令

```bash
# 1. 验证 YAML 语法
python3 -c "import yaml; yaml.safe_load(open('services/agent/etc/providers.yaml'))"

# 2. 验证 OpenAPI 文件
swagger-cli validate services/agent/etc/openapi.example.yaml

# 3. 查询已注册函数
curl http://localhost:8080/api/v1/functions/descriptors | jq .

# 4. 查看特定平台函数
curl http://localhost:8080/api/v1/functions/descriptors | \
  jq '.[] | select(.id | startswith("examples"))'

# 5. 查看日志
tail -f /var/log/croupier-agent.log | grep provider
```

---

## 🐛 故障排查

| 问题 | 检查 | 解决方案 |
|------|------|----------|
| `provider config not found` | 文件是否存在 | 创建 `providers.yaml` |
| `failed to read file` | 路径是否正确 | 使用相对路径 `./etc/file.yaml` |
| `no methods discovered` | OpenAPI 格式 | 运行 `swagger-cli validate` |
| `provider disabled` | enabled 字段 | 设置 `enabled: true` |
| 函数未出现 | 查看日志 | 检查错误信息 |

---

## 📖 相关文档

- **完整文档**: [README-OPENAPI.md](./README-OPENAPI.md)
- **配置示例**: [providers.example.yaml](./providers.example.yaml)
- **OpenAPI 示例**: [openapi.example.yaml](./openapi.example.yaml)
- **官方规范**: https://swagger.io/specification/

---

## 💡 最佳实践

✅ **DO**:
- 使用版本控制管理 OpenAPI 文件
- 使用环境变量存储敏感信息
- 使用描述性的平台名称
- 验证 OpenAPI 文件格式
- 查看日志确认加载成功

❌ **DON'T**:
- 硬编码 API Token
- 使用绝对路径（除非必要）
- 忽略 YAML 语法错误
- 在生产环境启用未测试的函数

---

**最后更新**: 2024-02-07
**快速参考**: v1.0
