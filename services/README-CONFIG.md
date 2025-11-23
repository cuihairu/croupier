# Services 目录配置说明

## 📁 目录结构

```
services/
├── .goctl.yaml              # ✅ 统一配置文件
├── gen-logic.sh             # 🚀 统一生成脚本
├── server/                  # 服务端 API
│   └── server.api
├── agent/                   # Agent 服务
│   └── agent.api
├── edge/                    # Edge 服务
├── ingest/                  # Ingest 服务
└── demo/                    # Demo 服务
```

## ⚙️ 统一配置

**配置文件**：`.goctl.yaml`

所有子服务（server, agent, edge, ingest, demo）共享此配置。

**命名风格**：`go_zero`（下划线分隔）

生成的文件示例：
- `admin_user_handler.go` ✅
- `user_profile_logic.go` ✅
- `auth_login_handler.go` ✅

## 🚀 使用方法

### 批量生成所有服务

```bash
cd /Users/cui/Workspaces/croupier/services

# 使用统一脚本生成所有服务的代码
./gen-logic.sh
```

脚本会自动：
1. 查找所有 `.api` 文件
2. 使用 `go_zero` 风格生成代码
3. 应用统一的配置

### 单独生成某个服务

```bash
# 方法 1：在服务目录内
cd /Users/cui/Workspaces/croupier/services/server
goctl api go -api server.api -dir .

# 方法 2：在 services 目录
cd /Users/cui/Workspaces/croupier/services
goctl api go -api server/server.api -dir server
```

goctl 会自动向上查找并使用 `services/.goctl.yaml` 配置。

### 使用不同风格（临时）

```bash
# 临时使用其他风格
./gen-logic.sh --style gozero

# 或者直接指定
goctl api go -api server.api -dir . --style gozero
```

## 📊 配置层级说明

goctl 配置查找顺序（优先级从高到低）：

```
1. services/server/.goctl.yaml     # 子服务专属配置（优先级最高）
2. services/.goctl.yaml            # ✅ 当前使用（统一配置）
3. ~/.goctl/config.yaml            # 用户全局配置
4. /etc/goctl/config.yaml          # 系统全局配置
```

**当前策略**：使用 `services/.goctl.yaml` 统一配置。

## 🎯 配置策略

### 当前策略：统一配置（推荐）✅

```
services/.goctl.yaml    # 所有服务共享
```

**优点**：
- 配置统一，维护简单
- 新增服务自动继承
- 团队规范一致

### 备选策略：分层配置

如果某个服务需要特殊配置，可以在子目录创建：

```bash
# 例如：server 服务需要特殊配置
cat > services/server/.goctl.yaml << EOF
style: goZero  # 覆盖父级配置
EOF
```

子目录配置会覆盖父级配置。

## 🔄 配置变更流程

### 修改全局配置

1. 编辑 `services/.goctl.yaml`
2. 运行 `./gen-logic.sh` 重新生成所有服务
3. 验证生成的文件名格式

### 为特定服务设置不同配置

```bash
# 1. 创建服务专属配置
cat > services/agent/.goctl.yaml << EOF
style: gozero  # agent 使用不同风格
EOF

# 2. 生成代码
cd services/agent
goctl api go -api agent.api -dir .
```

## 📝 命名风格对比

| 风格 | 示例文件名 | 推荐度 |
|------|-----------|--------|
| `go_zero` ✅ | `user_profile_handler.go` | ⭐⭐⭐⭐⭐ 符合Go规范 |
| `gozero` | `userprofilehandler.go` | ⭐⭐⭐ go-zero默认 |
| `goZero` | `userProfileHandler.go` | ⭐ 不符合Go规范 |
| `go-zero` | `user-profile-handler.go` | ⭐⭐ 不推荐 |

## 🛠️ 工具脚本

### gen-logic.sh

统一代码生成脚本，支持批量生成所有服务。

```bash
# 查看帮助
./gen-logic.sh --help

# 使用默认风格（go_zero）
./gen-logic.sh

# 使用其他风格
./gen-logic.sh --style gozero
```

### gen-openapi.sh

生成 OpenAPI/Swagger 文档。

### start-services.sh / stop-services.sh

启动/停止所有服务。

## 🔍 验证配置

### 检查配置文件

```bash
# 查看当前配置
cat services/.goctl.yaml

# 验证配置语法（生成时会自动验证）
goctl api validate -api server/server.api
```

### 验证生成结果

```bash
# 生成代码
./gen-logic.sh

# 检查文件名格式
ls services/server/internal/handler/ | head -5
ls services/agent/internal/handler/ | head -5

# 应该看到下划线风格的文件名
```

## 📚 参考

- goctl 配置文档：https://go-zero.dev/docs/tutorials/cli/config
- API 语法参考：https://go-zero.dev/docs/tutorials/api/define
- go-zero 官网：https://go-zero.dev/

## ⚠️ 注意事项

1. **配置变更后需重新生成**：修改 `.goctl.yaml` 后，需要运行生成脚本才能应用新配置
2. **types.go 会被覆盖**：不要手动修改 `internal/types/types.go`
3. **备份重要代码**：重新生成前建议先提交或备份现有代码
4. **团队同步**：配置变更后通知团队成员，保持代码风格一致
