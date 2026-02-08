# Agent OpenAPI 函数注册指南

本指南说明如何在 Croupier Agent 中注册 OpenAPI 函数。

## 📋 目录

- [快速开始](#快速开始)
- [配置文件说明](#配置文件说明)
- [单个 OpenAPI 文件](#单个-openapi-文件)
- [多个 OpenAPI 文件](#多个-openapi-文件)
- [完整示例](#完整示例)
- [验证和调试](#验证和调试)

---

## 🚀 快速开始

### 1. 准备 OpenAPI 文件

将你的 OpenAPI 3.0.3 文件放到 `services/agent/etc/` 目录：

```bash
services/agent/etc/
├── agent.yaml              # Agent 主配置
├── platforms.yaml          # 平台配置（创建这个）
├── openapi.example.yaml    # 示例 OpenAPI 文件
└── my-functions.yaml       # 你的 OpenAPI 文件
```

### 2. 创建配置文件

创建或编辑 `services/agent/etc/platforms.yaml`：

```yaml
platforms:
  my_functions:
    enabled: true
    type: openapi
    config:
      openapi_spec: ./etc/my-functions.yaml
      base_url: http://localhost:8080
      timeout: 30s
```

### 3. 重启 Agent

```bash
# 停止 Agent
killall croupier-agent

# 启动 Agent
./bin/croupier-agent -config ./services/agent/etc/agent.yaml
```

### 4. 验证加载

查看 Agent 日志，确认函数已注册：

```
INFO platform loaded name=my_functions methods=13
INFO registering platform method function=my_functions.player.create
INFO registering platform method function=my_functions.player.get
```

---

## 📝 配置文件说明

### 配置文件位置

```
services/agent/etc/platforms.yaml
```

### 配置结构

```yaml
platforms:
  # 平台名称（自定义，用于生成函数 ID 前缀）
  platform_name:
    enabled: true              # 是否启用
    type: openapi              # 固定为 openapi
    config:
      # OpenAPI 文件配置（3 种方式）
      openapi_spec: "./file.yaml"           # 方式 1: 单个文件
      openapi_specs: ["file1.yaml", ...]   # 方式 2: 多个文件
      methods: [...]                       # 方式 3: 手动定义

      # HTTP 配置
      base_url: http://localhost:8080
      timeout: 30s
      retry_count: 3
      headers:
        X-Custom-Header: value

      # 认证配置
      auth:
        type: bearer  # none, bearer, basic, api_key, custom
        token: "${API_TOKEN}"

      # 速率限制
      rate_limit:
        requests_per_minute: 60
        burst_size: 10

      # 响应转换
      transform:
        success_field: code
        success_value: 0
        data_field: data
        error_field: message
```

---

## 📄 单个 OpenAPI 文件

### 基础配置

```yaml
platforms:
  single_file:
    enabled: true
    type: openapi
    config:
      # 相对路径
      openapi_spec: ./etc/my-functions.yaml

      # 或绝对路径
      # openapi_spec: /path/to/my-functions.yaml

      # 或 HTTP URL
      # openapi_spec: https://example.com/openapi.yaml

      base_url: http://localhost:8080
```

### 支持的文件格式

- ✅ JSON 格式 (`.json`)
- ✅ YAML 格式 (`.yaml`, `.yml`)
- ✅ HTTP/HTTPS URL

### 支持的 OpenAPI 版本

- ✅ OpenAPI 3.0.x
- ✅ OpenAPI 3.1.x
- ✅ Swagger 2.0 (部分支持)

---

## 📚 多个 OpenAPI 文件

### 配置方式

```yaml
platforms:
  multiple_files:
    enabled: true
    type: openapi
    config:
      # 使用 openapi_specs（复数）加载多个文件
      openapi_specs:
        - ./etc/player-functions.yaml
        - ./etc/inventory-functions.yaml
        - ./etc/chat-functions.yaml
        - ../packs/http/openapi.yaml      # 支持相对路径
        - ../packs/player/openapi.yaml

      base_url: http://localhost:8080
```

### 函数合并规则

1. **所有函数都会被合并**到同一个平台
2. **函数 ID 冲突**：后加载的会覆盖先加载的
3. **加载顺序**：按 `openapi_specs` 列表的顺序

### 示例：加载所有 Packs

```yaml
platforms:
  all_packs:
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
      timeout: 30s
      rate_limit:
        requests_per_minute: 100
        burst_size: 20
```

---

## 🎯 完整示例

### 场景 1: 开发环境

```yaml
platforms:
  # 本地测试函数
  dev_functions:
    enabled: true
    type: openapi
    config:
      openapi_spec: ./etc/openapi.example.yaml
      base_url: http://localhost:8080
      timeout: 30s
      retry_count: 3
```

**注册的函数**：
- `dev_functions.player.create`
- `dev_functions.player.get`
- `dev_functions.player.delete`
- ... (共 13 个函数)

### 场景 2: 生产环境

```yaml
platforms:
  # Pack 函数
  game_packs:
    enabled: true
    type: openapi
    config:
      openapi_specs:
        - ../packs/http/openapi.yaml
        - ../packs/player/openapi.yaml
        - ../packs/prom/openapi.yaml
      base_url: http://game-server:8080
      timeout: 30s
      retry_count: 3
      rate_limit:
        requests_per_minute: 200
        burst_size: 50

  # 自定义函数
  custom_functions:
    enabled: true
    type: openapi
    config:
      openapi_specs:
        - ./etc/player-functions.yaml
        - ./etc/inventory-functions.yaml
      base_url: http://game-server:8080
      timeout: 30s
      headers:
        X-Game-ID: "${GAME_ID}"
        X-Env: "${ENV}"
```

**注册的函数**：
- `game_packs.http.generic_invoke`
- `game_packs.player.ban`
- `game_packs.prom.query`
- `custom_functions.player.create`
- `custom_functions.inventory.get`
- ... (合并所有函数)

### 场景 3: 外部 API 集成

```yaml
platforms:
  external_prometheus:
    enabled: true
    type: openapi
    config:
      openapi_spec: https://prometheus.example.com/openapi.json
      base_url: https://prometheus.example.com
      timeout: 10s
      auth:
        type: basic
        username: "${PROM_USER}"
        password: "${PROM_PASSWORD}"

  external_grafana:
    enabled: true
    type: openapi
    config:
      openapi_spec: https://grafana.example.com/api/openapi.yaml
      base_url: https://grafana.example.com
      timeout: 30s
      auth:
        type: bearer
        token: "${GRAFANA_TOKEN}"
```

---

## 🔍 验证和调试

### 1. 检查配置文件

```bash
# 检查 YAML 语法
python3 -c "import yaml; yaml.safe_load(open('services/agent/etc/platforms.yaml'))"

# 或使用 yamllint
yamllint services/agent/etc/platforms.yaml
```

### 2. 验证 OpenAPI 文件

```bash
# 安装 swagger-cli
npm install -g @apidevtools/swagger-cli

# 验证 OpenAPI 文件
swagger-cli validate services/agent/etc/openapi.example.yaml
```

### 3. 启动 Agent 并查看日志

```bash
# 启动 Agent
./bin/croupier-agent -config ./services/agent/etc/agent.yaml

# 查看日志输出
tail -f /var/log/croupier-agent.log
```

**成功日志示例**：
```
INFO  loading platform config from platforms.yaml
INFO  platform loaded name=game_packs methods=5
INFO  registering platform method function=game_packs.http.generic_invoke
INFO  registering platform method function=game_packs.player.ban
INFO  registering platform method function=game_packs.prom.query
```

**失败日志示例**：
```
ERROR failed to init platform name=game_packs error="failed to read OpenAPI spec file: open ../packs/not-found.yaml: no such file or directory"
```

### 4. 检查已注册函数

启动 Agent 后，可以通过 Server API 查询已注册的函数：

```bash
# 查询所有函数
curl http://localhost:8080/api/v1/functions/descriptors

# 查询特定平台的函数
curl http://localhost:8080/api/v1/functions/descriptors | jq '.[] | select(.id | startswith("game_packs"))'
```

### 5. 常见问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| `platform config not found` | platforms.yaml 不存在 | 创建配置文件 |
| `failed to read OpenAPI spec file` | 文件路径错误 | 检查相对/绝对路径 |
| `failed to parse OpenAPI spec` | YAML/JSON 格式错误 | 使用 swagger-cli 验证 |
| `platform disabled` | enabled: false | 设置 enabled: true |
| `no methods discovered` | OpenAPI 文件为空或格式错误 | 检查 paths 定义 |

---

## 📖 进阶用法

### 环境变量替换

配置中支持 `${VAR_NAME}` 格式的环境变量：

```yaml
platforms:
  my_functions:
    enabled: true
    type: openapi
    config:
      base_url: "http://${SERVICE_HOST}:${SERVICE_PORT}"
      headers:
        Authorization: "Bearer ${API_TOKEN}"
        X-Game-ID: "${GAME_ID}"
```

### 响应转换

将非标准 API 响应转换为标准格式：

```yaml
platforms:
  legacy_api:
    enabled: true
    type: openapi
    config:
      openapi_spec: ./etc/legacy-api.yaml
      base_url: http://legacy-api:8080
      transform:
        success_field: code      # 成功字段
        success_value: 0         # 成功值
        data_field: data         # 数据字段
        error_field: message     # 错误字段
```

**转换前**：
```json
{
  "code": 0,
  "message": "success",
  "data": { "player_id": "123" }
}
```

**转换后**：
```json
{
  "success": true,
  "data": { "player_id": "123" },
  "error": null
}
```

### 速率限制

防止过载：

```yaml
platforms:
  rate_limited:
    enabled: true
    type: openapi
    config:
      openapi_spec: ./etc/api.yaml
      base_url: http://api:8080
      rate_limit:
        requests_per_minute: 60  # 每分钟 60 个请求
        burst_size: 10           # 突发 10 个
```

---

## 🔗 相关文档

- [OpenAPI 3.0 规范](https://swagger.io/specification/)
- [配置文件示例](./platforms.example.yaml)
- [OpenAPI 示例文件](./openapi.example.yaml)
- [Agent 配置文档](../../docs/architecture/agent.md)

---

## 💡 最佳实践

1. **文件组织**
   - OpenAPI 文件放在 `services/agent/etc/` 目录
   - 使用清晰的命名：`player-functions.yaml`
   - 或按模块分目录：`etc/functions/player.yaml`

2. **环境分离**
   ```bash
   services/agent/etc/
   ├── platforms.yaml          # 基础配置
   ├── platforms.dev.yaml      # 开发环境覆盖
   ├── platforms.staging.yaml  # 测试环境覆盖
   └── platforms.prod.yaml     # 生产环境覆盖
   ```

3. **版本控制**
   - ✅ 提交 OpenAPI 文件到 Git
   - ❌ 不提交敏感信息（API Token）
   - 使用环境变量替代硬编码

4. **函数命名**
   - 使用描述性的平台名称
   - 保持 operationId 简洁清晰
   - 使用一致的命名约定

5. **测试**
   - 先使用 `openapi.example.yaml` 测试
   - 逐步添加实际函数
   - 验证每个 OpenAPI 文件格式

---

**最后更新**: 2024-02-07
**维护者**: Croupier Team
**反馈**: https://github.com/cuihairu/croupier/issues
